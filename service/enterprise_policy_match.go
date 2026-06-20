package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type UsageAmount struct {
	RequestCount     int64
	Quota            int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

type PolicyEvaluationRequest struct {
	EnterpriseContext *EnterpriseContext
	ModelName         string
	Ability           string
	IsPlayground      bool
	ChannelId         int
	Estimated         UsageAmount
	RequestId         string
	Now               time.Time
}

type PolicyDecision struct {
	Allowed            bool
	DryRun             bool
	DenyReason         string
	DenyError          error
	WouldReject        bool
	MatchedPolicyIds   []int
	CounterPolicyIds   []int
	ActionObservations []PolicyActionObservation
}

type PolicyActionObservation struct {
	PolicyId      int      `json:"policy_id"`
	Action        string   `json:"action"`
	Trigger       string   `json:"trigger"`
	Reason        string   `json:"reason"`
	FallbackModel string   `json:"fallback_model,omitempty"`
	AllowedModels []string `json:"allowed_models,omitempty"`
}

type EnterpriseModelNotAllowedError struct {
	ModelName     string
	PolicyIds     []int
	AllowedModels []string
}

func (e EnterpriseModelNotAllowedError) Error() string {
	return fmt.Sprintf("enterprise governance model not allowed: model=%s policy_ids=%s allowed_models=%s", e.ModelName, joinEnterprisePolicyIds(e.PolicyIds), strings.Join(e.AllowedModels, ","))
}

func EvaluateEnterprisePolicies(req PolicyEvaluationRequest) (PolicyDecision, *Reservation, error) {
	decision := PolicyDecision{Allowed: true}
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return decision, nil, nil
	}
	decision.DryRun = req.EnterpriseContext.DryRun
	policies, err := MatchEnterprisePolicies(req)
	if err != nil {
		return decision, nil, err
	}
	for _, policy := range policies {
		decision.MatchedPolicyIds = append(decision.MatchedPolicyIds, policy.Id)
		if isEnterpriseCounterPolicy(policy) {
			decision.CounterPolicyIds = append(decision.CounterPolicyIds, policy.Id)
		}
	}
	blockingPolicies, actionPolicies := splitEnterprisePoliciesByAction(policies)
	if err := CheckEnterpriseModelPermission(req, blockingPolicies); err != nil {
		if isEnterpriseModelNotAllowedError(err) {
			markEnterprisePolicyDecisionDenied(&decision, err)
			return decision, nil, nil
		}
		return decision, nil, err
	}
	actionObservations, err := EvaluateEnterprisePolicyActions(req, actionPolicies)
	if err != nil {
		return decision, nil, err
	}
	decision.ActionObservations = actionObservations
	if decision.DryRun {
		if err := CheckEnterpriseQuota(req, blockingPolicies); err != nil {
			markEnterprisePolicyDecisionDenied(&decision, err)
		}
		return decision, nil, nil
	}
	reservation, err := ReserveEnterpriseQuota(req, blockingPolicies)
	if err != nil {
		markEnterprisePolicyDecisionDenied(&decision, err)
		return decision, nil, nil
	}
	actionReservation, err := ReserveEnterpriseQuotaObservation(req, actionPolicies)
	if err != nil {
		if reservation != nil {
			_ = RefundEnterpriseReservation(reservation)
		}
		return decision, nil, err
	}
	return decision, MergeEnterpriseReservations(reservation, actionReservation), nil
}

func MatchEnterprisePolicies(req PolicyEvaluationRequest) ([]model.EnterpriseQuotaPolicy, error) {
	if req.EnterpriseContext == nil || !req.EnterpriseContext.Enabled {
		return nil, nil
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	candidates, err := loadEnterprisePolicyCandidates(req.EnterpriseContext, now)
	if err != nil {
		return nil, err
	}
	input := enterprisePolicyCELInputFromRequest(req)
	matched := make([]model.EnterpriseQuotaPolicy, 0, len(candidates))
	for _, policy := range candidates {
		ok, err := EvaluatePolicyCondition(policy, input)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, policy)
		}
	}
	return matched, nil
}

func ListRequestableEnterpriseQuotaPolicies(ctx *EnterpriseContext, now time.Time) ([]model.EnterpriseQuotaPolicy, error) {
	if ctx == nil || !ctx.Enabled {
		return nil, nil
	}
	policies, err := MatchEnterprisePolicies(PolicyEvaluationRequest{EnterpriseContext: ctx, Now: now})
	if err != nil {
		return nil, err
	}
	requestable := make([]model.EnterpriseQuotaPolicy, 0, len(policies))
	for _, policy := range policies {
		if isEnterpriseCounterPolicy(policy) {
			requestable = append(requestable, policy)
		}
	}
	return requestable, nil
}

func IsEnterpriseQuotaPolicyRequestable(ctx *EnterpriseContext, policyId int, now time.Time) (model.EnterpriseQuotaPolicy, bool, error) {
	policies, err := ListRequestableEnterpriseQuotaPolicies(ctx, now)
	if err != nil {
		return model.EnterpriseQuotaPolicy{}, false, err
	}
	for _, policy := range policies {
		if policy.Id == policyId {
			return policy, true, nil
		}
	}
	return model.EnterpriseQuotaPolicy{}, false, nil
}

func CheckEnterpriseModelPermission(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) error {
	var allowedModels map[string]struct{}
	var policyIds []int
	for _, policy := range policies {
		models, constrained, err := enterprisePolicySpecificModelSet(policy)
		if err != nil {
			return err
		}
		if !constrained {
			continue
		}
		policyIds = append(policyIds, policy.Id)
		if allowedModels == nil {
			allowedModels = models
			continue
		}
		for modelName := range allowedModels {
			if _, ok := models[modelName]; !ok {
				delete(allowedModels, modelName)
			}
		}
	}
	if allowedModels == nil {
		return nil
	}
	modelName := strings.TrimSpace(req.ModelName)
	if _, ok := allowedModels[modelName]; ok {
		return nil
	}
	return EnterpriseModelNotAllowedError{
		ModelName:     modelName,
		PolicyIds:     append([]int(nil), policyIds...),
		AllowedModels: sortedEnterpriseModelSet(allowedModels),
	}
}

func loadEnterprisePolicyCandidates(ctx *EnterpriseContext, now time.Time) ([]model.EnterpriseQuotaPolicy, error) {
	if ctx.EnterpriseId <= 0 {
		return nil, errors.New("企业上下文缺少企业 ID")
	}
	query := model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Where("enterprise_id = ? AND status = ?", ctx.EnterpriseId, model.QuotaPolicyStatusEnabled).
		Where("(effective_at = 0 OR effective_at <= ?) AND (expires_at = 0 OR expires_at > ?)", now.Unix(), now.Unix())

	targetWhere := model.DB.Where("target_type = ? AND target_id = ?", model.PolicyTargetEnterprise, ctx.EnterpriseId).
		Or("target_type = ? AND target_id = ?", model.PolicyTargetUser, ctx.UserId)
	if len(ctx.OrgUnitIds) > 0 {
		targetWhere = targetWhere.Or("target_type = ? AND target_id IN ?", model.PolicyTargetOrgUnit, ctx.OrgUnitIds)
	}
	if len(ctx.PolicyGroupIds) > 0 {
		targetWhere = targetWhere.Or("target_type = ? AND target_id IN ?", model.PolicyTargetPolicyGroup, ctx.PolicyGroupIds)
	}
	if ctx.ProjectId > 0 {
		targetWhere = targetWhere.Or("target_type = ? AND target_id = ?", model.PolicyTargetProject, ctx.ProjectId)
	}

	var policies []model.EnterpriseQuotaPolicy
	if err := query.Where(targetWhere).Order("priority desc, id asc").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func enterprisePolicySpecificModelSet(policy model.EnterpriseQuotaPolicy) (map[string]struct{}, bool, error) {
	if strings.TrimSpace(policy.ModelScope) == "" || policy.ModelScope == model.PolicyModelScopeAll {
		return nil, false, nil
	}
	if policy.ModelScope != model.PolicyModelScopeSpecific {
		return nil, false, fmt.Errorf("unsupported enterprise policy model scope: policy_id=%d model_scope=%s", policy.Id, policy.ModelScope)
	}
	var models []string
	if err := common.UnmarshalJsonStr(policy.ModelsJson, &models); err != nil {
		return nil, false, fmt.Errorf("invalid enterprise policy model scope: policy_id=%d: %w", policy.Id, err)
	}
	result := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			result[modelName] = struct{}{}
		}
	}
	return result, true, nil
}

func enterprisePolicyCELInputFromRequest(req PolicyEvaluationRequest) EnterprisePolicyCELInput {
	ctx := req.EnterpriseContext
	if ctx == nil {
		ctx = &EnterpriseContext{}
	}
	return EnterprisePolicyCELInput{
		User: EnterprisePolicyCELUser{
			Id:           ctx.UserId,
			RuntimeGroup: ctx.RuntimeGroup,
			Role:         ctx.Role,
		},
		Org: EnterprisePolicyCELOrg{
			EnterpriseId:   ctx.EnterpriseId,
			OrgUnitId:      ctx.PrimaryOrgUnitId,
			OrgUnitPathIds: ctx.OrgUnitIds,
			PolicyGroupIds: ctx.PolicyGroupIds,
			ProjectId:      ctx.ProjectId,
		},
		Request: EnterprisePolicyCELRequest{
			Model:        req.ModelName,
			Ability:      req.Ability,
			IsPlayground: req.IsPlayground,
			ChannelId:    req.ChannelId,
		},
		Token: EnterprisePolicyCELToken{Id: ctx.TokenId},
	}
}

func isEnterpriseCounterPolicy(policy model.EnterpriseQuotaPolicy) bool {
	return policy.Metric == model.PolicyMetricRequestCount || policy.Metric == model.PolicyMetricQuota
}

func splitEnterprisePoliciesByAction(policies []model.EnterpriseQuotaPolicy) ([]model.EnterpriseQuotaPolicy, []model.EnterpriseQuotaPolicy) {
	blocking := make([]model.EnterpriseQuotaPolicy, 0, len(policies))
	action := make([]model.EnterpriseQuotaPolicy, 0, len(policies))
	for _, policy := range policies {
		if model.IsEnterpriseQuotaPolicyBlockingAction(normalizedEnterprisePolicyAction(policy)) {
			blocking = append(blocking, policy)
			continue
		}
		action = append(action, policy)
	}
	return blocking, action
}

func EvaluateEnterprisePolicyActions(req PolicyEvaluationRequest, policies []model.EnterpriseQuotaPolicy) ([]PolicyActionObservation, error) {
	observations := make([]PolicyActionObservation, 0)
	for _, policy := range policies {
		action := normalizedEnterprisePolicyAction(policy)
		if model.IsEnterpriseQuotaPolicyBlockingAction(action) {
			continue
		}
		if err := CheckEnterpriseModelPermission(req, []model.EnterpriseQuotaPolicy{policy}); err != nil {
			if !isEnterpriseModelNotAllowedError(err) {
				return nil, err
			}
			observation := policyActionObservation(policy, "model_not_allowed", err)
			var modelErr EnterpriseModelNotAllowedError
			if errors.As(err, &modelErr) {
				observation.AllowedModels = append([]string(nil), modelErr.AllowedModels...)
				if action == model.PolicyActionFallbackModel && len(modelErr.AllowedModels) > 0 {
					observation.FallbackModel = modelErr.AllowedModels[0]
				}
			}
			observations = append(observations, observation)
		}
		if err := CheckEnterpriseQuota(req, []model.EnterpriseQuotaPolicy{policy}); err != nil {
			if !isEnterpriseQuotaExceededError(err) {
				return nil, err
			}
			observations = append(observations, policyActionObservation(policy, "quota_exceeded", err))
		}
	}
	return observations, nil
}

func policyActionObservation(policy model.EnterpriseQuotaPolicy, trigger string, err error) PolicyActionObservation {
	return PolicyActionObservation{
		PolicyId: policy.Id,
		Action:   normalizedEnterprisePolicyAction(policy),
		Trigger:  trigger,
		Reason:   err.Error(),
	}
}

func normalizedEnterprisePolicyAction(policy model.EnterpriseQuotaPolicy) string {
	action := strings.TrimSpace(policy.Action)
	if action == "" {
		return model.PolicyActionReject
	}
	if !model.IsEnterpriseQuotaPolicyAction(action) {
		return model.PolicyActionReject
	}
	return action
}

func markEnterprisePolicyDecisionDenied(decision *PolicyDecision, err error) {
	if decision == nil || err == nil {
		return
	}
	decision.WouldReject = true
	decision.DenyReason = err.Error()
	decision.DenyError = err
	if !decision.DryRun {
		decision.Allowed = false
	}
}

func sortedEnterpriseModelSet(models map[string]struct{}) []string {
	result := make([]string, 0, len(models))
	for modelName := range models {
		result = append(result, modelName)
	}
	sort.Strings(result)
	return result
}

func joinEnterprisePolicyIds(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

func isEnterpriseModelNotAllowedError(err error) bool {
	var modelErr EnterpriseModelNotAllowedError
	return errors.As(err, &modelErr)
}
