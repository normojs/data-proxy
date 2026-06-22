package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	enterpriseQuotaCounterRedisReserveScript = `
local used = tonumber(redis.call("HGET", KEYS[1], "used")) or 0
local reserved = tonumber(redis.call("HGET", KEYS[1], "reserved")) or 0
local amount = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local seed_used = tonumber(ARGV[4]) or 0
local seed_reserved = tonumber(ARGV[5]) or 0
if redis.call("HGET", KEYS[1], "initialized") ~= "1" then
  redis.call("HSET", KEYS[1], "used", seed_used, "reserved", seed_reserved, "initialized", "1")
  used = seed_used
  reserved = seed_reserved
end
if used + reserved + amount > limit then
  redis.call("HSET", KEYS[1], "limit", limit)
  if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
  return {0, used, reserved}
end
reserved = reserved + amount
redis.call("HSET", KEYS[1], "used", used, "reserved", reserved, "limit", limit)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {1, used, reserved}
`
	enterpriseQuotaCounterRedisSettleScript = `
local used = tonumber(redis.call("HGET", KEYS[1], "used")) or 0
local reserved = tonumber(redis.call("HGET", KEYS[1], "reserved")) or 0
local reserved_delta = tonumber(ARGV[1])
local actual_delta = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
reserved = reserved - reserved_delta
if reserved < 0 then reserved = 0 end
used = used + actual_delta
redis.call("HSET", KEYS[1], "used", used, "reserved", reserved)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {used, reserved}
`
	enterpriseQuotaCounterRedisRefundScript = `
local used = tonumber(redis.call("HGET", KEYS[1], "used")) or 0
local reserved = tonumber(redis.call("HGET", KEYS[1], "reserved")) or 0
local amount = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
reserved = reserved - amount
if reserved < 0 then reserved = 0 end
redis.call("HSET", KEYS[1], "used", used, "reserved", reserved)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {used, reserved}
`
)

type Reservation struct {
	RequestId        string
	EnterpriseId     int
	UserId           int
	PolicyIds        []int
	CounterIds       map[int]int
	ReservedAmounts  map[int]UsageAmount
	RedisCounterKeys map[int]string
	RedisCounterTTLs map[int]time.Duration
	RedisCounterUsed bool
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

type enterpriseQuotaCounterSnapshot struct {
	UsedValue     int64
	ReservedValue int64
}

type enterpriseQuotaAtomicCounter interface {
	Enabled() bool
	Reserve(ctx context.Context, key string, amount int64, limit int64, ttl time.Duration, seed enterpriseQuotaCounterSnapshot) (enterpriseQuotaCounterSnapshot, bool, error)
	Settle(ctx context.Context, key string, reserved int64, actual int64, ttl time.Duration) error
	Refund(ctx context.Context, key string, amount int64, ttl time.Duration) error
	Snapshot(ctx context.Context, key string) (enterpriseQuotaCounterSnapshot, bool, error)
	SetSnapshot(ctx context.Context, key string, snapshot enterpriseQuotaCounterSnapshot, ttl time.Duration) error
	ScanKeys(ctx context.Context, prefix string, limit int) ([]string, bool, error)
}

type enterpriseQuotaRedisReservation struct {
	Policy model.EnterpriseQuotaPolicy
	Amount int64
	Start  time.Time
	End    time.Time
	Key    string
}

type redisEnterpriseQuotaAtomicCounter struct{}

var enterpriseQuotaCounterBackend enterpriseQuotaAtomicCounter = redisEnterpriseQuotaAtomicCounter{}

func ReserveEnterpriseQuota(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
	if enterpriseQuotaAtomicCounterEnabled() {
		reservation, err := reserveEnterpriseQuotaWithRedis(req, policies)
		if err == nil || isEnterpriseQuotaExceededError(err) {
			return reservation, err
		}
		common.SysLog("enterprise quota redis counter failed, fallback to DB counter: " + err.Error())
	}
	return reserveEnterpriseQuotaWithDB(req, policies)
}

func ReserveEnterpriseQuotaObservation(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
	return reserveEnterpriseQuotaObservationWithDB(req, policies)
}

func MergeEnterpriseReservations(primary *Reservation, extra *Reservation) *Reservation {
	if primary == nil {
		return extra
	}
	if extra == nil {
		return primary
	}
	primary.PolicyIds = append(primary.PolicyIds, extra.PolicyIds...)
	if primary.CounterIds == nil {
		primary.CounterIds = map[int]int{}
	}
	for policyId, counterId := range extra.CounterIds {
		primary.CounterIds[policyId] = counterId
	}
	if primary.ReservedAmounts == nil {
		primary.ReservedAmounts = map[int]UsageAmount{}
	}
	for policyId, amount := range extra.ReservedAmounts {
		primary.ReservedAmounts[policyId] = amount
	}
	if len(extra.RedisCounterKeys) > 0 {
		if primary.RedisCounterKeys == nil {
			primary.RedisCounterKeys = map[int]string{}
		}
		for policyId, key := range extra.RedisCounterKeys {
			primary.RedisCounterKeys[policyId] = key
		}
	}
	if len(extra.RedisCounterTTLs) > 0 {
		if primary.RedisCounterTTLs == nil {
			primary.RedisCounterTTLs = map[int]time.Duration{}
		}
		for policyId, ttl := range extra.RedisCounterTTLs {
			primary.RedisCounterTTLs[policyId] = ttl
		}
	}
	primary.RedisCounterUsed = primary.RedisCounterUsed || extra.RedisCounterUsed
	return primary
}

func reserveEnterpriseQuotaWithDB(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
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

func reserveEnterpriseQuotaObservationWithDB(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
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

func reserveEnterpriseQuotaWithRedis(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) (*Reservation, error) {
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return nil, nil
	}
	reservation := &Reservation{
		RequestId:        req.RequestId,
		EnterpriseId:     req.EnterpriseContext.EnterpriseId,
		UserId:           req.EnterpriseContext.UserId,
		CounterIds:       map[int]int{},
		ReservedAmounts:  map[int]UsageAmount{},
		RedisCounterKeys: map[int]string{},
		RedisCounterTTLs: map[int]time.Duration{},
		RedisCounterUsed: true,
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	redisReservations := make([]enterpriseQuotaRedisReservation, 0, len(policies))
	ctx := context.Background()
	for _, policy := range policies {
		amount := amountForEnterprisePolicyMetric(policy.Metric, req.Estimated)
		if amount <= 0 {
			continue
		}
		start, end, err := ResolveEnterprisePolicyPeriod(policy, now)
		if err != nil {
			refundRedisQuotaReservations(ctx, redisReservations)
			return nil, err
		}
		effectiveLimit, err := enterpriseEffectivePolicyLimit(model.DB, policy, start, end, now)
		if err != nil {
			refundRedisQuotaReservations(ctx, redisReservations)
			return nil, err
		}
		seed, err := enterpriseQuotaCounterDBSnapshot(model.DB, policy, start)
		if err != nil {
			refundRedisQuotaReservations(ctx, redisReservations)
			return nil, err
		}
		key := enterpriseQuotaCounterRedisKey(policy, start)
		ttl := enterpriseQuotaCounterRedisTTL(end)
		snapshot, allowed, err := enterpriseQuotaCounterBackend.Reserve(ctx, key, amount, effectiveLimit, ttl, seed)
		if err != nil {
			refundRedisQuotaReservations(ctx, redisReservations)
			return nil, err
		}
		if !allowed {
			refundRedisQuotaReservations(ctx, redisReservations)
			return nil, enterpriseQuotaExceededErrorForPolicyWithSnapshot(policy, snapshot, amount, effectiveLimit, start, end)
		}
		redisReservations = append(redisReservations, enterpriseQuotaRedisReservation{Policy: policy, Amount: amount, Start: start, End: end, Key: key})
		reservation.PolicyIds = append(reservation.PolicyIds, policy.Id)
		reservation.RedisCounterKeys[policy.Id] = key
		reservation.RedisCounterTTLs[policy.Id] = ttl
		reservation.ReservedAmounts[policy.Id] = usageAmountForMetric(policy.Metric, amount)
	}
	if len(redisReservations) == 0 {
		return nil, nil
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range redisReservations {
			counter, err := lockEnterpriseQuotaCounter(tx, item.Policy, item.Start, item.End)
			if err != nil {
				return err
			}
			counter.ReservedValue += item.Amount
			if err := tx.Save(counter).Error; err != nil {
				return err
			}
			reservation.CounterIds[item.Policy.Id] = counter.Id
		}
		return nil
	}); err != nil {
		refundRedisQuotaReservations(ctx, redisReservations)
		return nil, err
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

func enterpriseQuotaExceededErrorForPolicyWithSnapshot(policy model.EnterpriseQuotaPolicy, snapshot enterpriseQuotaCounterSnapshot, requested int64, effectiveLimit int64, start time.Time, end time.Time) EnterpriseQuotaExceededError {
	return EnterpriseQuotaExceededError{
		PolicyId:       policy.Id,
		TargetType:     policy.TargetType,
		TargetId:       policy.TargetId,
		Metric:         policy.Metric,
		LimitValue:     effectiveLimit,
		UsedValue:      snapshot.UsedValue,
		ReservedValue:  snapshot.ReservedValue,
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

func enterpriseQuotaCounterDBSnapshot(db *gorm.DB, policy model.EnterpriseQuotaPolicy, periodStart time.Time) (enterpriseQuotaCounterSnapshot, error) {
	var counter model.EnterpriseQuotaCounter
	err := db.Where("policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period_start = ?",
		policy.Id, policy.TargetType, policy.TargetId, policy.Metric, periodStart.Unix()).
		First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return enterpriseQuotaCounterSnapshot{}, nil
	}
	if err != nil {
		return enterpriseQuotaCounterSnapshot{}, err
	}
	return enterpriseQuotaCounterSnapshot{UsedValue: counter.UsedValue, ReservedValue: counter.ReservedValue}, nil
}

func SettleEnterpriseReservation(reservation *Reservation, actual UsageAmount) error {
	if reservation == nil || len(reservation.PolicyIds) == 0 {
		return nil
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, policyId := range reservation.PolicyIds {
			reserved := reservation.ReservedAmounts[policyId]
			reservedValue := reservedUsageAmountValue(reserved)
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
	if err == nil {
		settleRedisEnterpriseReservation(reservation, actual)
	}
	return err
}

func RefundEnterpriseReservation(reservation *Reservation) error {
	if reservation == nil || len(reservation.PolicyIds) == 0 {
		return nil
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, policyId := range reservation.PolicyIds {
			amount := reservation.ReservedAmounts[policyId]
			delta := reservedUsageAmountValue(amount)
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
	if err == nil {
		refundRedisEnterpriseReservation(reservation)
	}
	return err
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

func enterpriseQuotaAtomicCounterEnabled() bool {
	return common.EnterpriseQuotaRedisCounterEnabled && enterpriseQuotaCounterBackendAvailable()
}

func enterpriseQuotaCounterBackendAvailable() bool {
	return enterpriseQuotaCounterBackend != nil && enterpriseQuotaCounterBackend.Enabled()
}

func (redisEnterpriseQuotaAtomicCounter) Enabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

func (redisEnterpriseQuotaAtomicCounter) Reserve(ctx context.Context, key string, amount int64, limit int64, ttl time.Duration, seed enterpriseQuotaCounterSnapshot) (enterpriseQuotaCounterSnapshot, bool, error) {
	values, err := evalEnterpriseQuotaCounterScript(ctx, enterpriseQuotaCounterRedisReserveScript, key, amount, limit, int64(ttl.Seconds()), seed.UsedValue, seed.ReservedValue)
	if err != nil {
		return enterpriseQuotaCounterSnapshot{}, false, err
	}
	if len(values) < 3 {
		return enterpriseQuotaCounterSnapshot{}, false, errors.New("invalid redis quota reserve response")
	}
	return enterpriseQuotaCounterSnapshot{
		UsedValue:     values[1],
		ReservedValue: values[2],
	}, values[0] == 1, nil
}

func (redisEnterpriseQuotaAtomicCounter) Settle(ctx context.Context, key string, reserved int64, actual int64, ttl time.Duration) error {
	_, err := evalEnterpriseQuotaCounterScript(ctx, enterpriseQuotaCounterRedisSettleScript, key, reserved, actual, int64(ttl.Seconds()))
	return err
}

func (redisEnterpriseQuotaAtomicCounter) Refund(ctx context.Context, key string, amount int64, ttl time.Duration) error {
	_, err := evalEnterpriseQuotaCounterScript(ctx, enterpriseQuotaCounterRedisRefundScript, key, amount, int64(ttl.Seconds()))
	return err
}

func (redisEnterpriseQuotaAtomicCounter) Snapshot(ctx context.Context, key string) (enterpriseQuotaCounterSnapshot, bool, error) {
	values, err := common.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		return enterpriseQuotaCounterSnapshot{}, false, err
	}
	if len(values) == 0 {
		return enterpriseQuotaCounterSnapshot{}, false, nil
	}
	used, err := parseEnterpriseQuotaCounterRedisField(values, "used")
	if err != nil {
		return enterpriseQuotaCounterSnapshot{}, false, err
	}
	reserved, err := parseEnterpriseQuotaCounterRedisField(values, "reserved")
	if err != nil {
		return enterpriseQuotaCounterSnapshot{}, false, err
	}
	return enterpriseQuotaCounterSnapshot{UsedValue: used, ReservedValue: reserved}, true, nil
}

func (redisEnterpriseQuotaAtomicCounter) SetSnapshot(ctx context.Context, key string, snapshot enterpriseQuotaCounterSnapshot, ttl time.Duration) error {
	txn := common.RDB.TxPipeline()
	txn.HSet(ctx, key, map[string]any{
		"used":        snapshot.UsedValue,
		"reserved":    snapshot.ReservedValue,
		"initialized": "1",
	})
	if ttl > 0 {
		txn.Expire(ctx, key, ttl)
	}
	_, err := txn.Exec(ctx)
	return err
}

func (redisEnterpriseQuotaAtomicCounter) ScanKeys(ctx context.Context, prefix string, limit int) ([]string, bool, error) {
	if limit <= 0 {
		return nil, false, nil
	}
	var cursor uint64
	keys := make([]string, 0, limit)
	for {
		batch, nextCursor, err := common.RDB.Scan(ctx, cursor, prefix+"*", int64(limit)).Result()
		if err != nil {
			return nil, false, err
		}
		keys = append(keys, batch...)
		if len(keys) > limit {
			return keys[:limit], true, nil
		}
		if nextCursor == 0 {
			return keys, false, nil
		}
		cursor = nextCursor
	}
}

func parseEnterpriseQuotaCounterRedisField(values map[string]string, field string) (int64, error) {
	raw := values[field]
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid redis quota counter field %s=%q: %w", field, raw, err)
	}
	return value, nil
}

func evalEnterpriseQuotaCounterScript(ctx context.Context, script string, key string, args ...any) ([]int64, error) {
	result, err := common.RDB.Eval(ctx, script, []string{key}, args...).Result()
	if err != nil {
		return nil, err
	}
	rawValues, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected redis quota script result: %T", result)
	}
	values := make([]int64, 0, len(rawValues))
	for _, raw := range rawValues {
		switch value := raw.(type) {
		case int64:
			values = append(values, value)
		case int:
			values = append(values, int64(value))
		case string:
			var parsed int64
			if _, err := fmt.Sscan(value, &parsed); err != nil {
				return nil, err
			}
			values = append(values, parsed)
		default:
			return nil, fmt.Errorf("unexpected redis quota script value: %T", raw)
		}
	}
	return values, nil
}

func enterpriseQuotaCounterRedisKey(policy model.EnterpriseQuotaPolicy, start time.Time) string {
	return fmt.Sprintf("%s:%d:%d:%s:%d:%s:%d", enterpriseQuotaCounterRedisKeyPrefix(), policy.EnterpriseId, policy.Id, policy.TargetType, policy.TargetId, policy.Metric, start.Unix())
}

func enterpriseQuotaCounterRedisKeyForCounter(counter model.EnterpriseQuotaCounter) string {
	return fmt.Sprintf("%s:%d:%d:%s:%d:%s:%d", enterpriseQuotaCounterRedisKeyPrefix(), counter.EnterpriseId, counter.PolicyId, counter.TargetType, counter.TargetId, counter.Metric, counter.PeriodStart)
}

func enterpriseQuotaCounterRedisKeyPrefix() string {
	return "enterprise_quota_counter:v1"
}

func enterpriseQuotaCounterRedisScanPrefix(enterpriseId int) string {
	if enterpriseId > 0 {
		return fmt.Sprintf("%s:%d:", enterpriseQuotaCounterRedisKeyPrefix(), enterpriseId)
	}
	return enterpriseQuotaCounterRedisKeyPrefix() + ":"
}

func enterpriseQuotaCounterRedisTTL(periodEnd time.Time) time.Duration {
	ttl := time.Until(periodEnd) + 48*time.Hour
	if ttl < time.Hour {
		return time.Hour
	}
	return ttl
}

func refundRedisQuotaReservations(ctx context.Context, reservations []enterpriseQuotaRedisReservation) {
	if !enterpriseQuotaCounterBackendAvailable() {
		return
	}
	for _, reservation := range reservations {
		if err := enterpriseQuotaCounterBackend.Refund(ctx, reservation.Key, reservation.Amount, enterpriseQuotaCounterRedisTTL(reservation.End)); err != nil {
			common.SysLog("enterprise quota redis reservation rollback failed: " + err.Error())
		}
	}
}

func settleRedisEnterpriseReservation(reservation *Reservation, actual UsageAmount) {
	if reservation == nil || !reservation.RedisCounterUsed || !enterpriseQuotaCounterBackendAvailable() {
		return
	}
	ctx := context.Background()
	for _, policyId := range reservation.PolicyIds {
		key := reservation.RedisCounterKeys[policyId]
		if key == "" {
			continue
		}
		reserved := reservation.ReservedAmounts[policyId]
		reservedValue := reservedUsageAmountValue(reserved)
		if reservedValue <= 0 {
			continue
		}
		actualValue := actualAmountForReservedMetric(reserved, actual)
		if actualValue <= 0 {
			actualValue = reservedValue
		}
		ttl := reservation.RedisCounterTTLs[policyId]
		if ttl <= 0 {
			ttl = time.Hour
		}
		if err := enterpriseQuotaCounterBackend.Settle(ctx, key, reservedValue, actualValue, ttl); err != nil {
			common.SysLog("enterprise quota redis settle failed: " + err.Error())
		}
	}
}

func refundRedisEnterpriseReservation(reservation *Reservation) {
	if reservation == nil || !reservation.RedisCounterUsed || !enterpriseQuotaCounterBackendAvailable() {
		return
	}
	ctx := context.Background()
	for _, policyId := range reservation.PolicyIds {
		key := reservation.RedisCounterKeys[policyId]
		if key == "" {
			continue
		}
		amount := reservation.ReservedAmounts[policyId]
		delta := reservedUsageAmountValue(amount)
		if delta <= 0 {
			continue
		}
		ttl := reservation.RedisCounterTTLs[policyId]
		if ttl <= 0 {
			ttl = time.Hour
		}
		if err := enterpriseQuotaCounterBackend.Refund(ctx, key, delta, ttl); err != nil {
			common.SysLog("enterprise quota redis refund failed: " + err.Error())
		}
	}
}

func isEnterpriseQuotaExceededError(err error) bool {
	var quotaErr EnterpriseQuotaExceededError
	return errors.As(err, &quotaErr)
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
	case model.PolicyMetricPromptTokens:
		return usage.PromptTokens
	case model.PolicyMetricCompletionTokens:
		return usage.CompletionTokens
	case model.PolicyMetricTotalTokens:
		if usage.TotalTokens > 0 {
			return usage.TotalTokens
		}
		return usage.PromptTokens + usage.CompletionTokens
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
	if reserved.PromptTokens > 0 {
		return actual.PromptTokens
	}
	if reserved.CompletionTokens > 0 {
		return actual.CompletionTokens
	}
	if reserved.TotalTokens > 0 {
		if actual.TotalTokens > 0 {
			return actual.TotalTokens
		}
		return actual.PromptTokens + actual.CompletionTokens
	}
	return 0
}

func reservedUsageAmountValue(amount UsageAmount) int64 {
	switch {
	case amount.RequestCount > 0:
		return amount.RequestCount
	case amount.Quota > 0:
		return amount.Quota
	case amount.PromptTokens > 0:
		return amount.PromptTokens
	case amount.CompletionTokens > 0:
		return amount.CompletionTokens
	case amount.TotalTokens > 0:
		return amount.TotalTokens
	default:
		return 0
	}
}

func usageAmountForMetric(metric string, amount int64) UsageAmount {
	switch metric {
	case model.PolicyMetricRequestCount:
		return UsageAmount{RequestCount: amount}
	case model.PolicyMetricQuota:
		return UsageAmount{Quota: amount}
	case model.PolicyMetricPromptTokens:
		return UsageAmount{PromptTokens: amount}
	case model.PolicyMetricCompletionTokens:
		return UsageAmount{CompletionTokens: amount}
	case model.PolicyMetricTotalTokens:
		return UsageAmount{TotalTokens: amount}
	default:
		return UsageAmount{}
	}
}
