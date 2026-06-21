package service

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	enterpriseGovernanceAuditActionSharedPoolReserve = "enterprise_governance.shared_pool_reserve"
	enterpriseGovernanceAuditActionSharedPoolSettle  = "enterprise_governance.shared_pool_settle"
	enterpriseGovernanceAuditActionSharedPoolRefund  = "enterprise_governance.shared_pool_refund"
	enterpriseSharedPoolStatusReserved               = "reserved"
	enterpriseSharedPoolStatusInsufficient           = "insufficient"
	enterpriseSharedPoolStatusHeader                 = "X-Data-Proxy-Enterprise-Shared-Pool-Status"
	enterpriseSharedPoolBorrowedQuotaHeader          = "X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Quota"
	enterpriseSharedPoolBorrowedRequestsHeader       = "X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Requests"
	enterpriseSharedPoolRemainingQuotaHeader         = "X-Data-Proxy-Enterprise-Shared-Pool-Remaining-Quota"
	enterpriseSharedPoolRemainingRequestsHeader      = "X-Data-Proxy-Enterprise-Shared-Pool-Remaining-Requests"
)

var ErrEnterpriseGovernanceSharedPoolInsufficient = errors.New("enterprise governance shared pool capacity insufficient")

type EnterpriseGovernanceSharedPoolResult struct {
	Applied              bool
	Status               string
	BorrowedQuota        int64
	BorrowedRequestCount int64
	RemainingQuota       int64
	RemainingRequests    int64
	Records              []EnterpriseGovernanceSharedPoolBorrowRecord
}

type EnterpriseGovernanceSharedPoolBorrowRecord struct {
	PolicyId             int    `json:"policy_id"`
	PoolId               int64  `json:"pool_id,omitempty"`
	BorrowId             int64  `json:"borrow_id,omitempty"`
	Metric               string `json:"metric"`
	BorrowedValue        int64  `json:"borrowed_value"`
	SettledBorrowedValue int64  `json:"settled_borrowed_value,omitempty"`
	ReturnedValue        int64  `json:"returned_value,omitempty"`
	LimitValue           int64  `json:"limit_value"`
	UsedValue            int64  `json:"used_value"`
	ReservedValue        int64  `json:"reserved_value"`
	RequestedValue       int64  `json:"requested_value"`
	PeriodStart          int64  `json:"period_start"`
	PeriodEnd            int64  `json:"period_end"`
	PoolCapacityValue    int64  `json:"pool_capacity_value,omitempty"`
	PoolUsedValue        int64  `json:"pool_used_value,omitempty"`
	PoolReservedValue    int64  `json:"pool_reserved_value,omitempty"`
	PoolRemainingValue   int64  `json:"pool_remaining_value,omitempty"`
	Status               string `json:"status,omitempty"`
}

type EnterpriseGovernanceSharedPoolReservation struct {
	RequestId    string
	EnterpriseId int
	UserId       int
	OrgUnitId    int
	ProjectId    int
	Records      []EnterpriseGovernanceSharedPoolBorrowRecord
}

func ApplyEnterpriseGovernanceSharedPool(c *gin.Context, relayInfo *relaycommon.RelayInfo) (EnterpriseGovernanceSharedPoolResult, error) {
	result := EnterpriseGovernanceSharedPoolResult{}
	if !common.EnterpriseGovernanceEnabled || c == nil || relayInfo == nil {
		return result, nil
	}
	decision, ok := common.GetContextKeyType[PolicyDecision](c, constant.ContextKeyEnterpriseGovernanceDecision)
	if !ok {
		return result, nil
	}
	records := enterpriseGovernanceSharedPoolBorrowRecords(decision.ActionObservations)
	if len(records) == 0 {
		return result, nil
	}
	enterpriseCtx, ok := common.GetContextKeyType[*EnterpriseContext](c, constant.ContextKeyEnterpriseGovernanceContext)
	if !ok || enterpriseCtx == nil {
		var err error
		enterpriseCtx, err = resolveEnterpriseContextFromRelay(c, relayInfo)
		if err != nil {
			return result, err
		}
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled {
		return result, nil
	}

	var reservation *EnterpriseGovernanceSharedPoolReservation
	var err error
	result, reservation, err = reserveEnterpriseGovernanceSharedPool(c, enterpriseCtx, relayInfo, decision, records)
	if err != nil && !errors.Is(err, ErrEnterpriseGovernanceSharedPoolInsufficient) {
		return result, err
	}
	setEnterpriseSharedPoolHeaders(c, result)
	recordEnterpriseGovernanceSharedPoolAudit(c, enterpriseCtx, relayInfo, decision, result)
	if err != nil {
		return result, err
	}
	if reservation != nil {
		common.SetContextKey(c, constant.ContextKeyEnterpriseSharedPoolReserve, reservation)
	}
	logger.LogWarn(c, fmt.Sprintf("enterprise governance shared pool reserved: borrowed_quota=%d borrowed_requests=%d", result.BorrowedQuota, result.BorrowedRequestCount))
	return result, nil
}

func enterpriseGovernanceSharedPoolBorrowRecords(observations []PolicyActionObservation) []EnterpriseGovernanceSharedPoolBorrowRecord {
	records := make([]EnterpriseGovernanceSharedPoolBorrowRecord, 0)
	for _, observation := range observations {
		if observation.Action != model.PolicyActionSharedPool || observation.Trigger != "quota_exceeded" {
			continue
		}
		borrowed := observation.BorrowedValue
		if borrowed <= 0 {
			borrowed = enterpriseSharedPoolBorrowedValue(observation.LimitValue, observation.UsedValue, observation.ReservedValue, observation.RequestedValue)
		}
		if borrowed <= 0 {
			continue
		}
		records = append(records, EnterpriseGovernanceSharedPoolBorrowRecord{
			PolicyId:       observation.PolicyId,
			Metric:         observation.Metric,
			BorrowedValue:  borrowed,
			LimitValue:     observation.LimitValue,
			UsedValue:      observation.UsedValue,
			ReservedValue:  observation.ReservedValue,
			RequestedValue: observation.RequestedValue,
			PeriodStart:    observation.PeriodStart,
			PeriodEnd:      observation.PeriodEnd,
		})
	}
	return records
}

func reserveEnterpriseGovernanceSharedPool(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, records []EnterpriseGovernanceSharedPoolBorrowRecord) (EnterpriseGovernanceSharedPoolResult, *EnterpriseGovernanceSharedPoolReservation, error) {
	result := EnterpriseGovernanceSharedPoolResult{
		Applied: true,
		Status:  enterpriseSharedPoolStatusReserved,
		Records: cloneEnterpriseGovernanceSharedPoolBorrowRecords(records),
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	modelName := ""
	relayMode := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		relayMode = relayInfo.RelayMode
	}
	policyGroupIdsJson, err := common.Marshal(cloneIntSlice(enterpriseCtx.PolicyGroupIds))
	if err != nil {
		return result, nil, err
	}
	policyActionsJson, err := common.Marshal(cloneEnterprisePolicyActionObservations(decision.ActionObservations))
	if err != nil {
		return result, nil, err
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		pools := make([]model.EnterpriseGovernanceSharedPool, len(result.Records))
		for i := range result.Records {
			record := &result.Records[i]
			capacity := enterpriseSharedPoolCapacityValue(*record)
			pool, err := lockEnterpriseGovernanceSharedPool(tx, enterpriseCtx.EnterpriseId, *record, capacity)
			if err != nil {
				return err
			}
			record.PoolId = pool.Id
			record.PoolCapacityValue = pool.CapacityValue
			record.PoolUsedValue = pool.UsedValue
			record.PoolReservedValue = pool.ReservedValue
			record.PoolRemainingValue = enterpriseSharedPoolRemainingValue(*pool)
			record.Status = enterpriseSharedPoolStatusReserved
			addEnterpriseSharedPoolRemainingTotals(&result, *record)
			if record.PoolRemainingValue < record.BorrowedValue {
				record.Status = enterpriseSharedPoolStatusInsufficient
				result.Status = enterpriseSharedPoolStatusInsufficient
				return ErrEnterpriseGovernanceSharedPoolInsufficient
			}
			pools[i] = *pool
		}

		result.RemainingQuota = 0
		result.RemainingRequests = 0
		for i := range result.Records {
			record := &result.Records[i]
			pool := pools[i]
			pool.ReservedValue += record.BorrowedValue
			if err := tx.Save(&pool).Error; err != nil {
				return err
			}
			record.PoolReservedValue = pool.ReservedValue
			record.PoolRemainingValue = enterpriseSharedPoolRemainingValue(pool)
			addEnterpriseSharedPoolBorrowTotals(&result, *record)
			addEnterpriseSharedPoolRemainingTotals(&result, *record)
			borrow := model.EnterpriseGovernanceSharedPoolBorrow{
				RequestId:             requestId,
				PoolId:                pool.Id,
				EnterpriseId:          enterpriseCtx.EnterpriseId,
				UserId:                enterpriseCtx.UserId,
				TokenId:               enterpriseCtx.TokenId,
				OrgUnitId:             enterpriseCtx.PrimaryOrgUnitId,
				ProjectId:             enterpriseCtx.ProjectId,
				PolicyId:              record.PolicyId,
				PolicyGroupIdsJson:    string(policyGroupIdsJson),
				ModelName:             modelName,
				ChannelId:             enterpriseChannelIdFromRelay(c, relayInfo),
				RelayMode:             relayMode,
				Metric:                record.Metric,
				CapacityValue:         pool.CapacityValue,
				ReservedBorrowedValue: record.BorrowedValue,
				PeriodStart:           record.PeriodStart,
				PeriodEnd:             record.PeriodEnd,
				Status:                model.EnterpriseGovernanceSharedPoolBorrowStatusReserved,
				DryRun:                decision.DryRun,
				PolicyActionsJson:     string(policyActionsJson),
				UserMessageKey:        "enterprise_governance.shared_pool_reserved",
			}
			if err := tx.Create(&borrow).Error; err != nil {
				return err
			}
			record.BorrowId = borrow.Id
		}
		return nil
	})
	if err != nil {
		return result, nil, err
	}
	reservation := &EnterpriseGovernanceSharedPoolReservation{
		RequestId:    requestId,
		EnterpriseId: enterpriseCtx.EnterpriseId,
		UserId:       enterpriseCtx.UserId,
		OrgUnitId:    enterpriseCtx.PrimaryOrgUnitId,
		ProjectId:    enterpriseCtx.ProjectId,
		Records:      cloneEnterpriseGovernanceSharedPoolBorrowRecords(result.Records),
	}
	return result, reservation, nil
}

func enterprisePolicySharedPoolBorrowedValue(quotaErr EnterpriseQuotaExceededError) int64 {
	return enterpriseSharedPoolBorrowedValue(quotaErr.LimitValue, quotaErr.UsedValue, quotaErr.ReservedValue, quotaErr.RequestedValue)
}

func enterpriseSharedPoolBorrowedValue(limit int64, used int64, reserved int64, requested int64) int64 {
	if requested <= 0 {
		return 0
	}
	overBefore := used + reserved - limit
	if overBefore < 0 {
		overBefore = 0
	}
	overAfter := used + reserved + requested - limit
	if overAfter < 0 {
		overAfter = 0
	}
	borrowed := overAfter - overBefore
	if borrowed > requested {
		return requested
	}
	return borrowed
}

func setEnterpriseSharedPoolHeaders(c *gin.Context, result EnterpriseGovernanceSharedPoolResult) {
	if c == nil || !result.Applied {
		return
	}
	c.Header(enterpriseSharedPoolStatusHeader, result.Status)
	c.Header(enterpriseSharedPoolBorrowedQuotaHeader, strconv.FormatInt(result.BorrowedQuota, 10))
	c.Header(enterpriseSharedPoolBorrowedRequestsHeader, strconv.FormatInt(result.BorrowedRequestCount, 10))
	c.Header(enterpriseSharedPoolRemainingQuotaHeader, strconv.FormatInt(result.RemainingQuota, 10))
	c.Header(enterpriseSharedPoolRemainingRequestsHeader, strconv.FormatInt(result.RemainingRequests, 10))
}

func SettleEnterpriseGovernanceSharedPool(ctx *gin.Context, actual UsageAmount) error {
	reservation, ok := common.GetContextKeyType[*EnterpriseGovernanceSharedPoolReservation](ctx, constant.ContextKeyEnterpriseSharedPoolReserve)
	if !ok || reservation == nil || len(reservation.Records) == 0 {
		return nil
	}
	records := cloneEnterpriseGovernanceSharedPoolBorrowRecords(reservation.Records)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for i := range records {
			record := &records[i]
			settled := enterpriseSharedPoolSettledBorrowedValue(*record, actual)
			returned := record.BorrowedValue - settled
			if returned < 0 {
				returned = 0
			}
			pool, err := lockEnterpriseGovernanceSharedPoolForRecord(tx, *record)
			if err != nil {
				return err
			}
			pool.ReservedValue -= record.BorrowedValue
			if pool.ReservedValue < 0 {
				pool.ReservedValue = 0
			}
			pool.UsedValue += settled
			if err := tx.Save(pool).Error; err != nil {
				return err
			}
			record.SettledBorrowedValue = settled
			record.ReturnedValue = returned
			record.PoolUsedValue = pool.UsedValue
			record.PoolReservedValue = pool.ReservedValue
			record.PoolRemainingValue = enterpriseSharedPoolRemainingValue(*pool)
			record.Status = model.EnterpriseGovernanceSharedPoolBorrowStatusSettled
			if record.BorrowId > 0 {
				if err := tx.Model(&model.EnterpriseGovernanceSharedPoolBorrow{}).
					Where("id = ?", record.BorrowId).
					Updates(map[string]any{
						"settled_borrowed_value": settled,
						"returned_value":         returned,
						"status":                 model.EnterpriseGovernanceSharedPoolBorrowStatusSettled,
						"user_message_key":       "enterprise_governance.shared_pool_settled",
					}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	recordEnterpriseGovernanceSharedPoolLifecycleAudit(ctx, reservation, enterpriseGovernanceAuditActionSharedPoolSettle, "settled", records)
	clearEnterpriseGovernanceSharedPoolReservation(ctx)
	return nil
}

func RefundEnterpriseGovernanceSharedPool(ctx *gin.Context) error {
	reservation, ok := common.GetContextKeyType[*EnterpriseGovernanceSharedPoolReservation](ctx, constant.ContextKeyEnterpriseSharedPoolReserve)
	if !ok || reservation == nil || len(reservation.Records) == 0 {
		return nil
	}
	records := cloneEnterpriseGovernanceSharedPoolBorrowRecords(reservation.Records)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for i := range records {
			record := &records[i]
			pool, err := lockEnterpriseGovernanceSharedPoolForRecord(tx, *record)
			if err != nil {
				return err
			}
			pool.ReservedValue -= record.BorrowedValue
			if pool.ReservedValue < 0 {
				pool.ReservedValue = 0
			}
			if err := tx.Save(pool).Error; err != nil {
				return err
			}
			record.SettledBorrowedValue = 0
			record.ReturnedValue = record.BorrowedValue
			record.PoolUsedValue = pool.UsedValue
			record.PoolReservedValue = pool.ReservedValue
			record.PoolRemainingValue = enterpriseSharedPoolRemainingValue(*pool)
			record.Status = model.EnterpriseGovernanceSharedPoolBorrowStatusRefunded
			if record.BorrowId > 0 {
				if err := tx.Model(&model.EnterpriseGovernanceSharedPoolBorrow{}).
					Where("id = ?", record.BorrowId).
					Updates(map[string]any{
						"settled_borrowed_value": int64(0),
						"returned_value":         record.BorrowedValue,
						"status":                 model.EnterpriseGovernanceSharedPoolBorrowStatusRefunded,
						"user_message_key":       "enterprise_governance.shared_pool_refunded",
					}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	recordEnterpriseGovernanceSharedPoolLifecycleAudit(ctx, reservation, enterpriseGovernanceAuditActionSharedPoolRefund, "refunded", records)
	clearEnterpriseGovernanceSharedPoolReservation(ctx)
	return nil
}

func recordEnterpriseGovernanceSharedPoolAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceSharedPoolResult) {
	if enterpriseCtx == nil || !result.Applied {
		return
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ActorUserId:    enterpriseCtx.UserId,
		Action:         enterpriseGovernanceAuditActionSharedPoolReserve,
		TargetType:     "quota_policy",
		TargetId:       firstEnterpriseSharedPoolPolicyActionObservationId(decision.ActionObservations),
		ScopeUserId:    enterpriseCtx.UserId,
		ScopeOrgUnitId: enterpriseCtx.PrimaryOrgUnitId,
		ScopeProjectId: enterpriseCtx.ProjectId,
		After:          enterpriseGovernanceSharedPoolAuditPayload(c, enterpriseCtx, relayInfo, decision, result, requestId),
		RequestId:      requestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance shared pool audit: "+err.Error())
	}
}

func enterpriseGovernanceSharedPoolAuditPayload(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceSharedPoolResult, requestId string) map[string]any {
	modelName := ""
	channelId := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		channelId = enterpriseChannelIdFromRelay(c, relayInfo)
	}
	return map[string]any{
		"request_id":              requestId,
		"model":                   modelName,
		"channel_id":              channelId,
		"token_id":                enterpriseCtx.TokenId,
		"org_unit_id":             enterpriseCtx.PrimaryOrgUnitId,
		"project_id":              enterpriseCtx.ProjectId,
		"policy_group_ids":        cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"matched_policy_ids":      cloneIntSlice(decision.MatchedPolicyIds),
		"counter_policy_ids":      cloneIntSlice(decision.CounterPolicyIds),
		"policy_actions":          cloneEnterprisePolicyActionObservations(decision.ActionObservations),
		"shared_pool_status":      result.Status,
		"borrowed_quota":          result.BorrowedQuota,
		"borrowed_request_count":  result.BorrowedRequestCount,
		"remaining_quota":         result.RemainingQuota,
		"remaining_request_count": result.RemainingRequests,
		"shared_pool_borrow_rows": result.Records,
		"user_message_key":        enterpriseSharedPoolUserMessageKey(result.Status),
		"dry_run":                 decision.DryRun,
	}
}

func firstEnterpriseSharedPoolPolicyActionObservationId(observations []PolicyActionObservation) int {
	for _, observation := range observations {
		if observation.Action == model.PolicyActionSharedPool {
			return observation.PolicyId
		}
	}
	return firstEnterprisePolicyActionObservationId(observations)
}

func lockEnterpriseGovernanceSharedPool(tx *gorm.DB, enterpriseId int, record EnterpriseGovernanceSharedPoolBorrowRecord, capacity int64) (*model.EnterpriseGovernanceSharedPool, error) {
	var pool model.EnterpriseGovernanceSharedPool
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("enterprise_id = ? AND policy_id = ? AND metric = ? AND period_start = ?", enterpriseId, record.PolicyId, record.Metric, record.PeriodStart).
		First(&pool).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pool = model.EnterpriseGovernanceSharedPool{
			EnterpriseId:  enterpriseId,
			PolicyId:      record.PolicyId,
			Metric:        record.Metric,
			PeriodStart:   record.PeriodStart,
			PeriodEnd:     record.PeriodEnd,
			CapacityValue: capacity,
			UsedValue:     0,
			ReservedValue: 0,
		}
		if err := tx.Create(&pool).Error; err != nil {
			return nil, err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool, pool.Id).Error; err != nil {
			return nil, err
		}
		return &pool, nil
	}
	if err != nil {
		return nil, err
	}
	if pool.CapacityValue != capacity || pool.PeriodEnd != record.PeriodEnd {
		pool.CapacityValue = capacity
		pool.PeriodEnd = record.PeriodEnd
		if err := tx.Save(&pool).Error; err != nil {
			return nil, err
		}
	}
	return &pool, nil
}

func lockEnterpriseGovernanceSharedPoolForRecord(tx *gorm.DB, record EnterpriseGovernanceSharedPoolBorrowRecord) (*model.EnterpriseGovernanceSharedPool, error) {
	var pool model.EnterpriseGovernanceSharedPool
	if record.PoolId > 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool, record.PoolId).Error; err != nil {
			return nil, err
		}
		return &pool, nil
	}
	return nil, errors.New("enterprise shared pool reservation missing pool id")
}

func enterpriseSharedPoolCapacityValue(record EnterpriseGovernanceSharedPoolBorrowRecord) int64 {
	if record.LimitValue > 0 {
		return record.LimitValue
	}
	return record.BorrowedValue
}

func enterpriseSharedPoolRemainingValue(pool model.EnterpriseGovernanceSharedPool) int64 {
	remaining := pool.CapacityValue - pool.UsedValue - pool.ReservedValue
	if remaining < 0 {
		return 0
	}
	return remaining
}

func enterpriseSharedPoolSettledBorrowedValue(record EnterpriseGovernanceSharedPoolBorrowRecord, actual UsageAmount) int64 {
	actualValue := amountForEnterprisePolicyMetric(record.Metric, actual)
	if record.Metric == model.PolicyMetricQuota && actualValue <= 0 {
		actualValue = record.RequestedValue
	}
	borrowed := enterpriseSharedPoolBorrowedValue(record.LimitValue, record.UsedValue, record.ReservedValue, actualValue)
	if borrowed > record.BorrowedValue {
		return record.BorrowedValue
	}
	if borrowed < 0 {
		return 0
	}
	return borrowed
}

func addEnterpriseSharedPoolBorrowTotals(result *EnterpriseGovernanceSharedPoolResult, record EnterpriseGovernanceSharedPoolBorrowRecord) {
	switch record.Metric {
	case model.PolicyMetricRequestCount:
		result.BorrowedRequestCount += record.BorrowedValue
	case model.PolicyMetricQuota:
		result.BorrowedQuota += record.BorrowedValue
	}
}

func addEnterpriseSharedPoolRemainingTotals(result *EnterpriseGovernanceSharedPoolResult, record EnterpriseGovernanceSharedPoolBorrowRecord) {
	switch record.Metric {
	case model.PolicyMetricRequestCount:
		result.RemainingRequests += record.PoolRemainingValue
	case model.PolicyMetricQuota:
		result.RemainingQuota += record.PoolRemainingValue
	}
}

func enterpriseSharedPoolUserMessageKey(status string) string {
	if status == enterpriseSharedPoolStatusInsufficient {
		return "enterprise_governance.shared_pool_insufficient"
	}
	return "enterprise_governance.shared_pool_reserved"
}

func recordEnterpriseGovernanceSharedPoolLifecycleAudit(ctx *gin.Context, reservation *EnterpriseGovernanceSharedPoolReservation, action string, status string, records []EnterpriseGovernanceSharedPoolBorrowRecord) {
	if reservation == nil || len(records) == 0 {
		return
	}
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   reservation.EnterpriseId,
		ActorUserId:    reservation.UserId,
		Action:         action,
		TargetType:     "quota_policy",
		TargetId:       records[0].PolicyId,
		ScopeUserId:    reservation.UserId,
		ScopeOrgUnitId: reservation.OrgUnitId,
		ScopeProjectId: reservation.ProjectId,
		After: map[string]any{
			"request_id":              reservation.RequestId,
			"shared_pool_status":      status,
			"borrowed_quota":          enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricQuota, "borrowed"),
			"borrowed_request_count":  enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricRequestCount, "borrowed"),
			"settled_quota":           enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricQuota, "settled"),
			"settled_request_count":   enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricRequestCount, "settled"),
			"returned_quota":          enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricQuota, "returned"),
			"returned_request_count":  enterpriseSharedPoolRecordMetricTotal(records, model.PolicyMetricRequestCount, "returned"),
			"shared_pool_borrow_rows": records,
			"user_message_key":        "enterprise_governance.shared_pool_" + status,
		},
		RequestId: reservation.RequestId,
	})
	if err != nil {
		logger.LogError(ctx, "error recording enterprise governance shared pool lifecycle audit: "+err.Error())
	}
}

func enterpriseSharedPoolRecordMetricTotal(records []EnterpriseGovernanceSharedPoolBorrowRecord, metric string, field string) int64 {
	var total int64
	for _, record := range records {
		if record.Metric != metric {
			continue
		}
		switch field {
		case "borrowed":
			total += record.BorrowedValue
		case "settled":
			total += record.SettledBorrowedValue
		case "returned":
			total += record.ReturnedValue
		}
	}
	return total
}

func cloneEnterpriseGovernanceSharedPoolBorrowRecords(records []EnterpriseGovernanceSharedPoolBorrowRecord) []EnterpriseGovernanceSharedPoolBorrowRecord {
	if len(records) == 0 {
		return []EnterpriseGovernanceSharedPoolBorrowRecord{}
	}
	return append([]EnterpriseGovernanceSharedPoolBorrowRecord(nil), records...)
}

func clearEnterpriseGovernanceSharedPoolReservation(ctx *gin.Context) {
	if ctx == nil {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyEnterpriseSharedPoolReserve, (*EnterpriseGovernanceSharedPoolReservation)(nil))
}
