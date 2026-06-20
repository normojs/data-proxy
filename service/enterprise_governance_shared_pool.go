package service

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const (
	enterpriseGovernanceAuditActionSharedPoolReserve = "enterprise_governance.shared_pool_reserve"
	enterpriseSharedPoolStatusReserved               = "reserved"
	enterpriseSharedPoolStatusHeader                 = "X-Data-Proxy-Enterprise-Shared-Pool-Status"
	enterpriseSharedPoolBorrowedQuotaHeader          = "X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Quota"
	enterpriseSharedPoolBorrowedRequestsHeader       = "X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Requests"
)

type EnterpriseGovernanceSharedPoolResult struct {
	Applied              bool
	Status               string
	BorrowedQuota        int64
	BorrowedRequestCount int64
	Records              []EnterpriseGovernanceSharedPoolBorrowRecord
}

type EnterpriseGovernanceSharedPoolBorrowRecord struct {
	PolicyId       int    `json:"policy_id"`
	Metric         string `json:"metric"`
	BorrowedValue  int64  `json:"borrowed_value"`
	LimitValue     int64  `json:"limit_value"`
	UsedValue      int64  `json:"used_value"`
	ReservedValue  int64  `json:"reserved_value"`
	RequestedValue int64  `json:"requested_value"`
	PeriodStart    int64  `json:"period_start"`
	PeriodEnd      int64  `json:"period_end"`
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

	result.Applied = true
	result.Status = enterpriseSharedPoolStatusReserved
	result.Records = records
	for _, record := range records {
		switch record.Metric {
		case model.PolicyMetricRequestCount:
			result.BorrowedRequestCount += record.BorrowedValue
		case model.PolicyMetricQuota:
			result.BorrowedQuota += record.BorrowedValue
		}
	}
	setEnterpriseSharedPoolHeaders(c, result)
	recordEnterpriseGovernanceSharedPoolAudit(c, enterpriseCtx, relayInfo, decision, result)
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
}

func recordEnterpriseGovernanceSharedPoolAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceSharedPoolResult) {
	if enterpriseCtx == nil || !result.Applied {
		return
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterpriseCtx.EnterpriseId,
		ActorUserId:  enterpriseCtx.UserId,
		Action:       enterpriseGovernanceAuditActionSharedPoolReserve,
		TargetType:   "quota_policy",
		TargetId:     firstEnterpriseSharedPoolPolicyActionObservationId(decision.ActionObservations),
		After:        enterpriseGovernanceSharedPoolAuditPayload(c, enterpriseCtx, relayInfo, decision, result, requestId),
		RequestId:    requestId,
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
		"shared_pool_borrow_rows": result.Records,
		"user_message_key":        "enterprise_governance.shared_pool_reserved",
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
