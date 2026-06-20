package service

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	enterpriseGovernanceAuditActionDryRunReject = "enterprise_governance.dry_run_reject"
	enterpriseGovernanceAuditActionHardReject   = "enterprise_governance.hard_limit_reject"
	enterpriseGovernanceAuditActionPolicyAction = "enterprise_governance.policy_action"
	enterpriseUsageAttributionStatusSuccess     = "success"
	enterpriseProjectIdHeader                   = "X-Data-Proxy-Project-ID"
	enterprisePolicyActionsHeader               = "X-Data-Proxy-Enterprise-Policy-Actions"
	enterprisePolicyActionHintHeader            = "X-Data-Proxy-Enterprise-Policy-Action-Hint"
	enterpriseFallbackModelHeader               = "X-Data-Proxy-Enterprise-Fallback-Model"
)

func PreCheckEnterpriseGovernance(c *gin.Context, relayInfo *relaycommon.RelayInfo, estimatedQuota int) *types.NewAPIError {
	if !common.EnterpriseGovernanceEnabled || relayInfo == nil {
		return nil
	}
	enterpriseCtx, err := resolveEnterpriseContextFromRelay(c, relayInfo)
	if err != nil {
		if isEnterpriseProjectContextError(err) {
			return enterpriseGovernanceProjectError(err)
		}
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled {
		return nil
	}

	req := PolicyEvaluationRequest{
		EnterpriseContext: enterpriseCtx,
		ModelName:         relayInfo.OriginModelName,
		Ability:           enterpriseAbilityFromRelayMode(relayInfo.RelayMode),
		IsPlayground:      relayInfo.IsPlayground,
		ChannelId:         enterpriseChannelIdFromRelay(c, relayInfo),
		Estimated: UsageAmount{
			RequestCount: 1,
			Quota:        int64(estimatedQuota),
		},
		RequestId: enterpriseRequestIdFromRelay(c, relayInfo),
	}
	decision, reservation, err := evaluateEnterpriseGovernancePreCheck(req)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	common.SetContextKey(c, constant.ContextKeyEnterpriseGovernanceContext, enterpriseCtx)
	common.SetContextKey(c, constant.ContextKeyEnterpriseGovernanceDecision, decision)
	if reservation != nil {
		common.SetContextKey(c, constant.ContextKeyEnterpriseGovernanceReserve, reservation)
	}

	if len(decision.ActionObservations) > 0 {
		recordEnterpriseGovernancePolicyActionAudit(c, enterpriseCtx, req, decision)
		setEnterpriseGovernancePolicyActionHeaders(c, decision)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance policy action observed: %s", enterprisePolicyActionLogSummary(decision.ActionObservations)))
	}
	if decision.DryRun && decision.WouldReject {
		recordEnterpriseGovernanceRejectAudit(c, enterpriseCtx, req, decision)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance dry-run would reject request: %s", decision.DenyReason))
		return nil
	}
	if !decision.Allowed {
		message, errorCode := enterpriseGovernanceUserFacingError(c, decision)
		recordEnterpriseGovernanceRejectAudit(c, enterpriseCtx, req, decision)
		if decision.DenyReason != "" {
			logger.LogWarn(c, fmt.Sprintf("enterprise governance hard-limit rejected request: %s", decision.DenyReason))
		}
		return types.NewErrorWithStatusCode(
			fmt.Errorf("%s", message),
			errorCode,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return nil
}

func SettleEnterpriseGovernanceUsage(ctx *gin.Context, actual UsageAmount) error {
	reservation, ok := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	if !ok || reservation == nil {
		return nil
	}
	if err := SettleEnterpriseReservation(reservation, actual); err != nil {
		return err
	}
	clearEnterpriseGovernanceReservation(ctx)
	return nil
}

func RefundEnterpriseGovernanceReservation(ctx *gin.Context) {
	reservation, ok := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	if !ok || reservation == nil {
		return
	}
	if err := RefundEnterpriseReservation(reservation); err != nil {
		logger.LogError(ctx, "error refunding enterprise governance reservation: "+err.Error())
		return
	}
	clearEnterpriseGovernanceReservation(ctx)
}

func RecordEnterpriseTextUsageAttribution(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, summary textQuotaSummary) error {
	if !common.EnterpriseGovernanceEnabled || ctx == nil || relayInfo == nil {
		return nil
	}
	enterpriseCtx, ok := common.GetContextKeyType[*EnterpriseContext](ctx, constant.ContextKeyEnterpriseGovernanceContext)
	if !ok || enterpriseCtx == nil {
		var err error
		enterpriseCtx, err = resolveEnterpriseContextFromRelay(ctx, relayInfo)
		if err != nil {
			return err
		}
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled || enterpriseCtx.EnterpriseId <= 0 {
		return nil
	}

	decision, _ := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	policyGroupIds := cloneIntSlice(enterpriseCtx.PolicyGroupIds)
	policyIds := cloneIntSlice(decision.MatchedPolicyIds)
	policyGroupIdsJson, err := common.Marshal(policyGroupIds)
	if err != nil {
		return err
	}
	policyIdsJson, err := common.Marshal(policyIds)
	if err != nil {
		return err
	}

	modelName := summary.ModelName
	if modelName == "" {
		modelName = relayInfo.OriginModelName
	}
	attribution := model.EnterpriseUsageAttribution{
		RequestId:          enterpriseRequestIdFromRelay(ctx, relayInfo),
		UserId:             relayInfo.UserId,
		TokenId:            relayInfo.TokenId,
		EnterpriseId:       enterpriseCtx.EnterpriseId,
		OrgUnitId:          enterpriseCtx.PrimaryOrgUnitId,
		ProjectId:          enterpriseCtx.ProjectId,
		PolicyGroupIdsJson: string(policyGroupIdsJson),
		PolicyIdsJson:      string(policyIdsJson),
		ModelName:          modelName,
		ChannelId:          enterpriseChannelIdFromRelay(ctx, relayInfo),
		PromptTokens:       summary.PromptTokens,
		CompletionTokens:   summary.CompletionTokens,
		TotalTokens:        summary.TotalTokens,
		Quota:              summary.Quota,
		Status:             enterpriseUsageAttributionStatusSuccess,
	}
	return model.DB.Create(&attribution).Error
}

func resolveEnterpriseContextFromRelay(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*EnterpriseContext, error) {
	if relayInfo == nil {
		return nil, errors.New("relay info is nil")
	}
	requestedProjectId, err := enterpriseRequestedProjectId(c)
	if err != nil {
		return nil, err
	}
	tokenDefaultProjectId := 0
	if c != nil {
		tokenDefaultProjectId = common.GetContextKeyInt(c, constant.ContextKeyTokenDefaultProjectId)
	}
	return ResolveEnterpriseContextWithProject(relayInfo.UserId, relayInfo.TokenId, tokenDefaultProjectId, requestedProjectId)
}

func enterpriseRequestedProjectId(c *gin.Context) (int, error) {
	if c == nil {
		return 0, nil
	}
	raw := strings.TrimSpace(c.GetHeader(enterpriseProjectIdHeader))
	if raw == "" {
		return 0, nil
	}
	projectId, err := strconv.Atoi(raw)
	if err != nil || projectId <= 0 {
		return 0, EnterpriseProjectContextError{Message: fmt.Sprintf("%s 必须是正整数", enterpriseProjectIdHeader)}
	}
	return projectId, nil
}

func isEnterpriseProjectContextError(err error) bool {
	var projectErr EnterpriseProjectContextError
	return errors.As(err, &projectErr)
}

func enterpriseGovernanceProjectError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	return types.NewErrorWithStatusCode(
		err,
		types.ErrorCodeEnterpriseGovernanceQuotaExceeded,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func evaluateEnterpriseGovernancePreCheck(req PolicyEvaluationRequest) (PolicyDecision, *Reservation, error) {
	decision := PolicyDecision{Allowed: true}
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return decision, nil, nil
	}
	return EvaluateEnterprisePolicies(req)
}

func enterpriseGovernanceUserFacingError(c *gin.Context, decision PolicyDecision) (string, types.ErrorCode) {
	messageKey, errorCode := enterpriseGovernanceUserFacingMessageKey(decision)
	return common.TranslateMessage(c, messageKey), errorCode
}

func enterpriseGovernanceUserFacingMessageKey(decision PolicyDecision) (string, types.ErrorCode) {
	messageKey := "enterprise_governance.quota_exceeded"
	errorCode := types.ErrorCodeEnterpriseGovernanceQuotaExceeded
	var modelErr EnterpriseModelNotAllowedError
	if errors.As(decision.DenyError, &modelErr) {
		return "enterprise_governance.model_not_allowed", types.ErrorCodeEnterpriseGovernanceModelNotAllowed
	}
	var quotaErr EnterpriseQuotaExceededError
	if errors.As(decision.DenyError, &quotaErr) {
		switch quotaErr.TargetType {
		case model.PolicyTargetOrgUnit:
			messageKey = "enterprise_governance.org_quota_exceeded"
			errorCode = types.ErrorCodeEnterpriseGovernanceOrgQuotaExceeded
		case model.PolicyTargetPolicyGroup:
			messageKey = "enterprise_governance.group_quota_exceeded"
			errorCode = types.ErrorCodeEnterpriseGovernanceGroupQuotaExceeded
		case model.PolicyTargetUser:
			messageKey = "enterprise_governance.user_quota_exceeded"
			errorCode = types.ErrorCodeEnterpriseGovernanceUserQuotaExceeded
		}
	}
	return messageKey, errorCode
}

func recordEnterpriseGovernanceRejectAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, req PolicyEvaluationRequest, decision PolicyDecision) {
	if enterpriseCtx == nil || !decision.WouldReject {
		return
	}
	targetId := firstEnterprisePolicyId(decision.CounterPolicyIds)
	if targetId == 0 {
		targetId = firstEnterprisePolicyId(decision.MatchedPolicyIds)
	}
	action := enterpriseGovernanceAuditActionHardReject
	if decision.DryRun {
		action = enterpriseGovernanceAuditActionDryRunReject
	}
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterpriseCtx.EnterpriseId,
		ActorUserId:  enterpriseCtx.UserId,
		Action:       action,
		TargetType:   "quota_policy",
		TargetId:     targetId,
		After:        enterpriseGovernanceRejectAuditPayload(enterpriseCtx, req, decision),
		RequestId:    req.RequestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance reject audit: "+err.Error())
	}
}

func enterpriseGovernanceRejectAuditPayload(enterpriseCtx *EnterpriseContext, req PolicyEvaluationRequest, decision PolicyDecision) map[string]any {
	userMessageKey, errorCode := enterpriseGovernanceUserFacingMessageKey(decision)
	payload := map[string]any{
		"request_id":         req.RequestId,
		"model":              req.ModelName,
		"ability":            req.Ability,
		"channel_id":         req.ChannelId,
		"token_id":           enterpriseCtx.TokenId,
		"org_unit_id":        enterpriseCtx.PrimaryOrgUnitId,
		"project_id":         enterpriseCtx.ProjectId,
		"policy_group_ids":   cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"matched_policy_ids": cloneIntSlice(decision.MatchedPolicyIds),
		"counter_policy_ids": cloneIntSlice(decision.CounterPolicyIds),
		"policy_actions":     cloneEnterprisePolicyActionObservations(decision.ActionObservations),
		"estimated_quota":    req.Estimated.Quota,
		"request_count":      req.Estimated.RequestCount,
		"deny_reason":        decision.DenyReason,
		"user_message_key":   userMessageKey,
		"error_code":         string(errorCode),
		"dry_run":            decision.DryRun,
	}
	var quotaErr EnterpriseQuotaExceededError
	if errors.As(decision.DenyError, &quotaErr) {
		payload["deny_type"] = "quota_exceeded"
		payload["policy_id"] = quotaErr.PolicyId
		payload["target_type"] = quotaErr.TargetType
		payload["target_id"] = quotaErr.TargetId
		payload["metric"] = quotaErr.Metric
		payload["limit_value"] = quotaErr.LimitValue
		payload["used_value"] = quotaErr.UsedValue
		payload["reserved_value"] = quotaErr.ReservedValue
		payload["requested_value"] = quotaErr.RequestedValue
		payload["period_start"] = quotaErr.PeriodStart
		payload["period_end"] = quotaErr.PeriodEnd
		payload["remaining_before_request"] = quotaErr.LimitValue - quotaErr.UsedValue - quotaErr.ReservedValue
		payload["suggested_limit_value"] = quotaErr.UsedValue + quotaErr.ReservedValue + quotaErr.RequestedValue
		return payload
	}
	var modelErr EnterpriseModelNotAllowedError
	if errors.As(decision.DenyError, &modelErr) {
		payload["deny_type"] = "model_not_allowed"
		payload["policy_ids"] = cloneIntSlice(modelErr.PolicyIds)
		payload["allowed_models"] = append([]string(nil), modelErr.AllowedModels...)
		return payload
	}
	payload["deny_type"] = "unknown"
	return payload
}

func recordEnterpriseGovernancePolicyActionAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, req PolicyEvaluationRequest, decision PolicyDecision) {
	if enterpriseCtx == nil || len(decision.ActionObservations) == 0 {
		return
	}
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterpriseCtx.EnterpriseId,
		ActorUserId:  enterpriseCtx.UserId,
		Action:       enterpriseGovernanceAuditActionPolicyAction,
		TargetType:   "quota_policy",
		TargetId:     firstEnterprisePolicyActionObservationId(decision.ActionObservations),
		After:        enterpriseGovernancePolicyActionAuditPayload(enterpriseCtx, req, decision),
		RequestId:    req.RequestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance policy action audit: "+err.Error())
	}
}

func enterpriseGovernancePolicyActionAuditPayload(enterpriseCtx *EnterpriseContext, req PolicyEvaluationRequest, decision PolicyDecision) map[string]any {
	return map[string]any{
		"request_id":         req.RequestId,
		"model":              req.ModelName,
		"ability":            req.Ability,
		"channel_id":         req.ChannelId,
		"token_id":           enterpriseCtx.TokenId,
		"org_unit_id":        enterpriseCtx.PrimaryOrgUnitId,
		"project_id":         enterpriseCtx.ProjectId,
		"policy_group_ids":   cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"matched_policy_ids": cloneIntSlice(decision.MatchedPolicyIds),
		"counter_policy_ids": cloneIntSlice(decision.CounterPolicyIds),
		"policy_actions":     cloneEnterprisePolicyActionObservations(decision.ActionObservations),
		"estimated_quota":    req.Estimated.Quota,
		"request_count":      req.Estimated.RequestCount,
		"user_message_key":   "enterprise_governance.policy_action_observed",
		"dry_run":            decision.DryRun,
	}
}

func setEnterpriseGovernancePolicyActionHeaders(c *gin.Context, decision PolicyDecision) {
	if c == nil || len(decision.ActionObservations) == 0 {
		return
	}
	c.Header(enterprisePolicyActionsHeader, strings.Join(uniqueEnterprisePolicyActionNames(decision.ActionObservations), ","))
	c.Header(enterprisePolicyActionHintHeader, "enterprise_governance.policy_action_observed")
	if fallbackModel := firstEnterprisePolicyFallbackModel(decision.ActionObservations); fallbackModel != "" {
		c.Header(enterpriseFallbackModelHeader, fallbackModel)
	}
}

func enterprisePolicyActionLogSummary(observations []PolicyActionObservation) string {
	parts := make([]string, 0, len(observations))
	for _, observation := range observations {
		parts = append(parts, fmt.Sprintf("policy_id=%d action=%s trigger=%s", observation.PolicyId, observation.Action, observation.Trigger))
	}
	return strings.Join(parts, "; ")
}

func enterpriseAbilityFromRelayMode(relayMode int) string {
	switch relayMode {
	case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeCompletions:
		return "chat"
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		return "responses"
	case relayconstant.RelayModeEmbeddings:
		return "embedding"
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return "image"
	case relayconstant.RelayModeAudioSpeech, relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		return "audio"
	case relayconstant.RelayModeRerank:
		return "rerank"
	case relayconstant.RelayModeRealtime:
		return "realtime"
	default:
		return "unknown"
	}
}

func enterpriseRequestIdFromRelay(c *gin.Context, relayInfo *relaycommon.RelayInfo) string {
	if relayInfo != nil && relayInfo.RequestId != "" {
		return relayInfo.RequestId
	}
	if c == nil {
		return ""
	}
	if requestId := c.GetString(common.RequestIdKey); requestId != "" {
		return requestId
	}
	return c.GetHeader(common.RequestIdKey)
}

func enterpriseChannelIdFromRelay(c *gin.Context, relayInfo *relaycommon.RelayInfo) int {
	if relayInfo != nil && relayInfo.ChannelMeta != nil && relayInfo.ChannelId > 0 {
		return relayInfo.ChannelId
	}
	if c == nil {
		return 0
	}
	return common.GetContextKeyInt(c, constant.ContextKeyChannelId)
}

func firstEnterprisePolicyId(ids []int) int {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func firstEnterprisePolicyActionObservationId(observations []PolicyActionObservation) int {
	if len(observations) == 0 {
		return 0
	}
	return observations[0].PolicyId
}

func uniqueEnterprisePolicyActionNames(observations []PolicyActionObservation) []string {
	names := make([]string, 0, len(observations))
	seen := map[string]struct{}{}
	for _, observation := range observations {
		action := strings.TrimSpace(observation.Action)
		if action == "" {
			continue
		}
		if _, ok := seen[action]; ok {
			continue
		}
		seen[action] = struct{}{}
		names = append(names, action)
	}
	return names
}

func firstEnterprisePolicyFallbackModel(observations []PolicyActionObservation) string {
	for _, observation := range observations {
		if observation.FallbackModel != "" {
			return observation.FallbackModel
		}
	}
	return ""
}

func cloneIntSlice(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	return append([]int(nil), values...)
}

func cloneEnterprisePolicyActionObservations(values []PolicyActionObservation) []PolicyActionObservation {
	if len(values) == 0 {
		return []PolicyActionObservation{}
	}
	return append([]PolicyActionObservation(nil), values...)
}

func clearEnterpriseGovernanceReservation(ctx *gin.Context) {
	if ctx == nil {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyEnterpriseGovernanceReserve, (*Reservation)(nil))
}
