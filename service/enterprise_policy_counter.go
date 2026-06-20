package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Reservation struct {
	RequestId       string
	EnterpriseId    int
	UserId          int
	PolicyIds       []int
	CounterIds      map[int]int
	ReservedAmounts map[int]UsageAmount
}

type EnterpriseQuotaExceededError struct {
	PolicyId       int
	TargetType     string
	TargetId       int
	Metric         string
	LimitValue     int64
	UsedValue      int64
	ReservedValue  int64
	RequestedValue int64
	PeriodStart    int64
	PeriodEnd      int64
}

func (e EnterpriseQuotaExceededError) Error() string {
	return fmt.Sprintf("enterprise governance quota exceeded: policy_id=%d target_type=%s target_id=%d metric=%s used=%d reserved=%d requested=%d limit=%d period_start=%d period_end=%d", e.PolicyId, e.TargetType, e.TargetId, e.Metric, e.UsedValue, e.ReservedValue, e.RequestedValue, e.LimitValue, e.PeriodStart, e.PeriodEnd)
}

func ReserveEnterpriseQuota(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return nil, nil
	}
	reservation := &Reservation{
		RequestId:       req.RequestId,
		EnterpriseId:    req.EnterpriseContext.EnterpriseId,
		UserId:          req.EnterpriseContext.UserId,
		CounterIds:      map[int]int{},
		ReservedAmounts: map[int]UsageAmount{},
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, policy := range policies {
			amount := amountForEnterprisePolicyMetric(policy.Metric, req.Estimated)
			if amount <= 0 {
				continue
			}
			start, end, err := ResolveEnterprisePolicyPeriod(policy, now)
			if err != nil {
				return err
			}
			counter, err := lockEnterpriseQuotaCounter(tx, policy, start, end)
			if err != nil {
				return err
			}
			effectiveLimit, err := enterpriseEffectivePolicyLimit(tx, policy, start, end, now)
			if err != nil {
				return err
			}
			if counter.UsedValue+counter.ReservedValue+amount > effectiveLimit {
				return enterpriseQuotaExceededErrorForPolicy(policy, *counter, amount, effectiveLimit, start, end)
			}
			counter.ReservedValue += amount
			if err := tx.Save(counter).Error; err != nil {
				return err
			}
			reservation.PolicyIds = append(reservation.PolicyIds, policy.Id)
			reservation.CounterIds[policy.Id] = counter.Id
			reservation.ReservedAmounts[policy.Id] = usageAmountForMetric(policy.Metric, amount)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(reservation.PolicyIds) == 0 {
		return nil, nil
	}
	return reservation, nil
}

func CheckEnterpriseQuota(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) error {
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return nil
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	for _, policy := range policies {
		amount := amountForEnterprisePolicyMetric(policy.Metric, req.Estimated)
		if amount <= 0 {
			continue
		}
		start, end, err := ResolveEnterprisePolicyPeriod(policy, now)
		if err != nil {
			return err
		}
		var counter model.EnterpriseQuotaCounter
		err = model.DB.Where("policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period_start = ?",
			policy.Id, policy.TargetType, policy.TargetId, policy.Metric, start.Unix()).
			First(&counter).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		effectiveLimit, err := enterpriseEffectivePolicyLimit(model.DB, policy, start, end, now)
		if err != nil {
			return err
		}
		if counter.UsedValue+counter.ReservedValue+amount > effectiveLimit {
			return enterpriseQuotaExceededErrorForPolicy(policy, counter, amount, effectiveLimit, start, end)
		}
	}
	return nil
}

func enterpriseQuotaExceededErrorForPolicy(policy model.EnterpriseQuotaPolicy, counter model.EnterpriseQuotaCounter, requested int64, effectiveLimit int64, start time.Time, end time.Time) EnterpriseQuotaExceededError {
	return EnterpriseQuotaExceededError{
		PolicyId:       policy.Id,
		TargetType:     policy.TargetType,
		TargetId:       policy.TargetId,
		Metric:         policy.Metric,
		LimitValue:     effectiveLimit,
		UsedValue:      counter.UsedValue,
		ReservedValue:  counter.ReservedValue,
		RequestedValue: requested,
		PeriodStart:    start.Unix(),
		PeriodEnd:      end.Unix(),
	}
}

func enterpriseEffectivePolicyLimit(db *gorm.DB, policy model.EnterpriseQuotaPolicy, periodStart time.Time, periodEnd time.Time, now time.Time) (int64, error) {
	var extra int64
	err := db.Model(&model.EnterpriseQuotaRequest{}).
		Where("enterprise_id = ? AND policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period = ?", policy.EnterpriseId, policy.Id, policy.TargetType, policy.TargetId, policy.Metric, policy.Period).
		Where("status = ?", model.EnterpriseQuotaRequestStatusApproved).
		Where("effective_at <= ? AND expires_at > ?", now.Unix(), now.Unix()).
		Where("effective_at < ? AND expires_at > ?", periodEnd.Unix(), periodStart.Unix()).
		Select("COALESCE(SUM(limit_delta), 0)").
		Scan(&extra).Error
	if err != nil {
		return 0, err
	}
	return policy.LimitValue + extra, nil
}

func SettleEnterpriseReservation(reservation *Reservation, actual UsageAmount) error {
	if reservation == nil || len(reservation.PolicyIds) == 0 {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, policyId := range reservation.PolicyIds {
			reserved := reservation.ReservedAmounts[policyId]
			reservedValue := reserved.RequestCount + reserved.Quota
			if reservedValue <= 0 {
				continue
			}
			actualValue := actualAmountForReservedMetric(reserved, actual)
			if actualValue <= 0 {
				actualValue = reservedValue
			}
			counter, err := lockEnterpriseQuotaCounterByReservation(tx, reservation, policyId)
			if err != nil {
				return err
			}
			counter.ReservedValue -= reservedValue
			if counter.ReservedValue < 0 {
				counter.ReservedValue = 0
			}
			counter.UsedValue += actualValue
			if err := tx.Save(counter).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func RefundEnterpriseReservation(reservation *Reservation) error {
	if reservation == nil || len(reservation.PolicyIds) == 0 {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, policyId := range reservation.PolicyIds {
			amount := reservation.ReservedAmounts[policyId]
			delta := amount.RequestCount + amount.Quota
			if delta <= 0 {
				continue
			}
			counter, err := lockEnterpriseQuotaCounterByReservation(tx, reservation, policyId)
			if err != nil {
				return err
			}
			counter.ReservedValue -= delta
			if counter.ReservedValue < 0 {
				counter.ReservedValue = 0
			}
			if err := tx.Save(counter).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ResolveEnterprisePolicyPeriod(policy model.EnterpriseQuotaPolicy, now time.Time) (time.Time, time.Time, error) {
	timezone := policy.Timezone
	if timezone == "" {
		timezone = model.DefaultEnterpriseTimezone
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := now.In(location)
	switch policy.Period {
	case model.PolicyPeriodDay:
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		return start, start.AddDate(0, 0, 1), nil
	case model.PolicyPeriodMonth:
		start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		return start, start.AddDate(0, 1, 0), nil
	default:
		return time.Time{}, time.Time{}, errors.New("不支持的策略周期")
	}
}

func lockEnterpriseQuotaCounter(tx *gorm.DB, policy model.EnterpriseQuotaPolicy, start time.Time, end time.Time) (*model.EnterpriseQuotaCounter, error) {
	var counter model.EnterpriseQuotaCounter
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period_start = ?",
			policy.Id, policy.TargetType, policy.TargetId, policy.Metric, start.Unix()).
		First(&counter).Error
	if err == nil {
		return &counter, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	counter = model.EnterpriseQuotaCounter{
		EnterpriseId: policy.EnterpriseId,
		PolicyId:     policy.Id,
		TargetType:   policy.TargetType,
		TargetId:     policy.TargetId,
		Metric:       policy.Metric,
		PeriodStart:  start.Unix(),
		PeriodEnd:    end.Unix(),
	}
	if err := tx.Create(&counter).Error; err != nil {
		return nil, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&counter, counter.Id).Error; err != nil {
		return nil, err
	}
	return &counter, nil
}

func lockEnterpriseQuotaCounterByReservation(tx *gorm.DB, reservation *Reservation, policyId int) (*model.EnterpriseQuotaCounter, error) {
	var counter model.EnterpriseQuotaCounter
	if counterId := reservation.CounterIds[policyId]; counterId > 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&counter, counterId).Error; err != nil {
			return nil, err
		}
		return &counter, nil
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("enterprise_id = ? AND policy_id = ?", reservation.EnterpriseId, policyId).
		Order("period_start desc").
		First(&counter).Error; err != nil {
		return nil, err
	}
	return &counter, nil
}

func amountForEnterprisePolicyMetric(metric string, usage UsageAmount) int64 {
	switch metric {
	case model.PolicyMetricRequestCount:
		if usage.RequestCount > 0 {
			return usage.RequestCount
		}
		return 1
	case model.PolicyMetricQuota:
		return usage.Quota
	default:
		return 0
	}
}

func actualAmountForReservedMetric(reserved UsageAmount, actual UsageAmount) int64 {
	if reserved.RequestCount > 0 {
		return actual.RequestCount
	}
	if reserved.Quota > 0 {
		return actual.Quota
	}
	return 0
}

func usageAmountForMetric(metric string, amount int64) UsageAmount {
	switch metric {
	case model.PolicyMetricRequestCount:
		return UsageAmount{RequestCount: amount}
	case model.PolicyMetricQuota:
		return UsageAmount{Quota: amount}
	default:
		return UsageAmount{}
	}
}
