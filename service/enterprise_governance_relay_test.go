package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterprisePolicyCheckDoesNotCreateCounter(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "dry-run request limit",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   1,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	err = CheckEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: &EnterpriseContext{
			Enabled:      true,
			EnterpriseId: enterprise.Id,
			UserId:       1006,
		},
		Estimated: UsageAmount{RequestCount: 2},
		Now:       time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	}, []model.EnterpriseQuotaPolicy{policy})

	require.Error(t, err)
	var counterCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaCounter{}).Count(&counterCount).Error)
	assert.EqualValues(t, 0, counterCount)
}

func TestPreCheckEnterpriseGovernanceDryRunAuditsAndAttribution(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)
	common.EnterpriseGovernanceDryRunEnabled = true

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1007,
		Username: "frank",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "relay quota limit",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-dry-run")
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 321)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1007,
		TokenId:         88,
		RequestId:       "req-enterprise-dry-run",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.Nil(t, apiErr)
	decision, ok := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	assert.True(t, decision.DryRun)
	assert.True(t, decision.WouldReject)
	assert.Equal(t, []int{policy.Id}, decision.MatchedPolicyIds)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ?", enterpriseGovernanceAuditActionDryRunReject).First(&audit).Error)
	assert.Equal(t, enterprise.Id, audit.EnterpriseId)
	assert.Equal(t, 1007, audit.ActorUserId)
	assert.Equal(t, policy.Id, audit.TargetId)
	assert.Equal(t, "req-enterprise-dry-run", audit.RequestId)

	err = RecordEnterpriseTextUsageAttribution(ctx, relayInfo, textQuotaSummary{
		ModelName:        "gpt-4o",
		PromptTokens:     11,
		CompletionTokens: 7,
		TotalTokens:      18,
		Quota:            20,
	})
	require.NoError(t, err)

	var attribution model.EnterpriseUsageAttribution
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-dry-run").First(&attribution).Error)
	assert.Equal(t, enterprise.Id, attribution.EnterpriseId)
	assert.Equal(t, 1007, attribution.UserId)
	assert.Equal(t, 88, attribution.TokenId)
	assert.Equal(t, 321, attribution.ChannelId)
	assert.Equal(t, "success", attribution.Status)
	assert.Equal(t, 20, attribution.Quota)
	assert.JSONEq(t, "[]", attribution.PolicyGroupIdsJson)
	assert.JSONEq(t, fmt.Sprintf("[%d]", policy.Id), attribution.PolicyIdsJson)
}

func TestPreCheckEnterpriseGovernanceHardLimitReservesAndSettles(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1008,
		Username: "grace",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "hard quota limit",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-hard-settle")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1008,
		TokenId:         89,
		RequestId:       "req-enterprise-hard-settle",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.Nil(t, apiErr)
	reservation, ok := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	require.True(t, ok)
	require.NotNil(t, reservation)

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 20, counter.ReservedValue)
	assert.EqualValues(t, 0, counter.UsedValue)

	require.NoError(t, SettleEnterpriseGovernanceUsage(ctx, UsageAmount{RequestCount: 1, Quota: 13}))
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 0, counter.ReservedValue)
	assert.EqualValues(t, 13, counter.UsedValue)
	reservation, _ = common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
}

func TestPreCheckEnterpriseGovernanceHardLimitRejectsWithEnterpriseErrorCode(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1010,
		Username: "ivan",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "reject quota limit",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-hard-reject")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1010,
		TokenId:         91,
		RequestId:       "req-enterprise-hard-reject",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeEnterpriseGovernanceQuotaExceeded, apiErr.GetErrorCode())
	assert.Equal(t, "enterprise_governance.quota_exceeded", apiErr.Error())
	assertEnterpriseGovernanceUserErrorIsRedacted(t, apiErr)
	internalReason := decisionInternalReasonForTest(t, ctx)
	assert.Contains(t, internalReason, fmt.Sprintf("policy_id=%d", policy.Id))
	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-hard-reject", enterpriseGovernanceAuditActionHardReject).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, "enterprise_governance.quota_exceeded", auditAfter["user_message_key"])
	assert.Equal(t, string(types.ErrorCodeEnterpriseGovernanceQuotaExceeded), auditAfter["error_code"])
	assert.Equal(t, "quota_exceeded", auditAfter["deny_type"])
	assert.EqualValues(t, policy.Id, auditAfter["policy_id"])
	assert.EqualValues(t, 10, auditAfter["limit_value"])
	assert.EqualValues(t, 20, auditAfter["requested_value"])
	reservation, _ := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
	var counterCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaCounter{}).Where("policy_id = ?", policy.Id).Count(&counterCount).Error)
	assert.EqualValues(t, 0, counterCount)
}

func TestPreCheckEnterpriseGovernanceHardLimitRejectsWithLocalizedOrgQuotaMessage(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)
	originalTranslate := common.TranslateMessage
	common.TranslateMessage = func(c *gin.Context, key string, args ...map[string]any) string {
		return "translated:" + key
	}
	t.Cleanup(func() {
		common.TranslateMessage = originalTranslate
	})

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1012,
		Username: "judy",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	orgUnit := model.EnterpriseOrgUnit{
		Id:           501,
		EnterpriseId: enterprise.Id,
		ParentId:     0,
		Name:         "Finance",
		Slug:         "finance",
		Path:         "/501/",
		Depth:        1,
		Status:       model.OrgUnitStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&orgUnit).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       1012,
		OrgUnitId:    orgUnit.Id,
		IsPrimary:    true,
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "org quota limit",
		TargetType:   model.PolicyTargetOrgUnit,
		TargetId:     orgUnit.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-org-hard-reject")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1012,
		TokenId:         93,
		RequestId:       "req-enterprise-org-hard-reject",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeEnterpriseGovernanceOrgQuotaExceeded, apiErr.GetErrorCode())
	assert.Equal(t, "translated:enterprise_governance.org_quota_exceeded", apiErr.Error())
	assertEnterpriseGovernanceUserErrorIsRedacted(t, apiErr)
	internalReason := decisionInternalReasonForTest(t, ctx)
	assert.Contains(t, internalReason, fmt.Sprintf("policy_id=%d", policy.Id))
	reservation, _ := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
}

func TestPreCheckEnterpriseGovernanceHardLimitRejectsWithModelNotAllowedCode(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1013,
		Username: "mallory",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "model allow list",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeSpecific,
		ModelsJson:   `["gpt-4o"]`,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-model-hard-reject")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1013,
		TokenId:         94,
		RequestId:       "req-enterprise-model-hard-reject",
		OriginModelName: "claude-sonnet-4",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 1)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeEnterpriseGovernanceModelNotAllowed, apiErr.GetErrorCode())
	assert.Equal(t, "enterprise_governance.model_not_allowed", apiErr.Error())
	assertEnterpriseGovernanceUserErrorIsRedacted(t, apiErr)
	internalReason := decisionInternalReasonForTest(t, ctx)
	assert.Contains(t, internalReason, fmt.Sprintf("policy_ids=%d", policy.Id))
	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-model-hard-reject", enterpriseGovernanceAuditActionHardReject).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, "enterprise_governance.model_not_allowed", auditAfter["user_message_key"])
	assert.Equal(t, string(types.ErrorCodeEnterpriseGovernanceModelNotAllowed), auditAfter["error_code"])
	assert.Equal(t, "model_not_allowed", auditAfter["deny_type"])
	assert.Contains(t, auditAfter["allowed_models"], "gpt-4o")
	reservation, _ := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
	var counterCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaCounter{}).Where("policy_id = ?", policy.Id).Count(&counterCount).Error)
	assert.EqualValues(t, 0, counterCount)
}

func TestPreCheckEnterpriseGovernanceDryRunReturnsQueryErrorForInvalidModelScope(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)
	common.EnterpriseGovernanceDryRunEnabled = true

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1015,
		Username: "peggy",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "invalid model scope",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Where("id = ?", policy.Id).
		Update("model_scope", "legacy").Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-invalid-model-scope")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1015,
		TokenId:         96,
		RequestId:       "req-enterprise-invalid-model-scope",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 1)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeQueryDataError, apiErr.GetErrorCode())
	assert.Contains(t, apiErr.Error(), "unsupported enterprise policy model scope")
	_, ok := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	assert.False(t, ok)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("request_id = ?", "req-enterprise-invalid-model-scope").
		Count(&auditCount).Error)
	assert.EqualValues(t, 0, auditCount)
}

func TestPreCheckEnterpriseGovernanceHardLimitRejectsWithUserQuotaCode(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1014,
		Username: "oscar",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "user quota limit",
		TargetType:   model.PolicyTargetUser,
		TargetId:     1014,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-user-hard-reject")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1014,
		TokenId:         95,
		RequestId:       "req-enterprise-user-hard-reject",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeEnterpriseGovernanceUserQuotaExceeded, apiErr.GetErrorCode())
	assert.Equal(t, "enterprise_governance.user_quota_exceeded", apiErr.Error())
	assertEnterpriseGovernanceUserErrorIsRedacted(t, apiErr)
	internalReason := decisionInternalReasonForTest(t, ctx)
	assert.Contains(t, internalReason, fmt.Sprintf("policy_id=%d", policy.Id))
	reservation, _ := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
}

func TestRefundEnterpriseGovernanceReservationReleasesPrecheckReserve(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1009,
		Username: "heidi",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "refund quota limit",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-enterprise-hard-refund")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1009,
		TokenId:         90,
		RequestId:       "req-enterprise-hard-refund",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	apiErr := PreCheckEnterpriseGovernance(ctx, relayInfo, 20)
	require.Nil(t, apiErr)
	RefundEnterpriseGovernanceReservation(ctx)

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 0, counter.ReservedValue)
	assert.EqualValues(t, 0, counter.UsedValue)
	reservation, _ := common.GetContextKeyType[*Reservation](ctx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
}

func TestEnterpriseGovernanceRolloutRunbookR0ToR3(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	const userId = 1020
	const tokenId = 92
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: "rollout-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          userId,
		TokenId:         tokenId,
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	// R0: 发布代码但保持默认关闭，relay 前置检查和成功归集都不产生治理副作用。
	common.EnterpriseGovernanceEnabled = false
	common.EnterpriseGovernanceDryRunEnabled = false
	r0Ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-rollout-r0", 401)
	relayInfo.RequestId = "req-enterprise-rollout-r0"
	require.Nil(t, PreCheckEnterpriseGovernance(r0Ctx, relayInfo, 10))
	require.NoError(t, RecordEnterpriseTextUsageAttribution(r0Ctx, relayInfo, textQuotaSummary{
		ModelName: "gpt-4o",
		Quota:     10,
	}))
	assertEnterpriseGovernanceTableCount(t, &model.EnterpriseQuotaCounter{}, 0)
	assertEnterpriseGovernanceTableCount(t, &model.EnterpriseUsageAttribution{}, 0)
	assertEnterpriseGovernanceTableCount(t, &model.EnterpriseAuditLog{}, 0)

	// R1: 初始化默认企业、组织、成员、分组和第一条策略，确认审计与查询底座可用。
	common.EnterpriseGovernanceEnabled = true
	orgUnit := model.EnterpriseOrgUnit{
		EnterpriseId: enterprise.Id,
		ParentId:     0,
		Name:         "Runbook Engineering",
		Slug:         "runbook-engineering",
		Depth:        1,
		Status:       model.OrgUnitStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&orgUnit).Error)
	orgUnit.Path = fmt.Sprintf("/%d/", orgUnit.Id)
	require.NoError(t, model.DB.Save(&orgUnit).Error)
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  userId,
		Action:       "org_unit.create",
		TargetType:   "org_unit",
		TargetId:     orgUnit.Id,
		After:        orgUnit,
		RequestId:    "req-enterprise-rollout-r1",
	}))
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       userId,
		OrgUnitId:    orgUnit.Id,
		IsPrimary:    true,
	}).Error)
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  userId,
		Action:       "member.update_org_unit",
		TargetType:   "user",
		TargetId:     userId,
		After:        gin.H{"org_unit_id": orgUnit.Id},
		RequestId:    "req-enterprise-rollout-r1",
	}))
	group := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		Name:         "Runbook Pilot",
		Slug:         "runbook-pilot",
		Status:       model.PolicyGroupStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&group).Error)
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  userId,
		Action:       "policy_group.create",
		TargetType:   "policy_group",
		TargetId:     group.Id,
		After:        group,
		RequestId:    "req-enterprise-rollout-r1",
	}))
	require.NoError(t, model.DB.Create(&model.EnterprisePolicyGroupMember{
		EnterpriseId:  enterprise.Id,
		PolicyGroupId: group.Id,
		UserId:        userId,
	}).Error)
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  userId,
		Action:       "policy_group.members.add",
		TargetType:   "policy_group",
		TargetId:     group.Id,
		After:        gin.H{"user_ids": []int{userId}},
		RequestId:    "req-enterprise-rollout-r1",
	}))
	baselinePolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "Runbook baseline quota",
		TargetType:   model.PolicyTargetPolicyGroup,
		TargetId:     group.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   1000,
		ModelScope:   model.PolicyModelScopeAll,
		Priority:     0,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  userId,
		Action:       "quota_policy.create",
		TargetType:   "quota_policy",
		TargetId:     baselinePolicy.Id,
		After:        baselinePolicy,
		RequestId:    "req-enterprise-rollout-r1",
	}))
	enterpriseCtx, err := ResolveEnterpriseContext(userId, tokenId)
	require.NoError(t, err)
	assert.Equal(t, orgUnit.Id, enterpriseCtx.PrimaryOrgUnitId)
	assert.Equal(t, []int{orgUnit.Id}, enterpriseCtx.OrgUnitIds)
	assert.Equal(t, []int{group.Id}, enterpriseCtx.PolicyGroupIds)
	var r1AuditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("request_id = ?", "req-enterprise-rollout-r1").
		Count(&r1AuditCount).Error)
	assert.EqualValues(t, 5, r1AuditCount)

	// R2: dry-run 观测，超低额度只记录 would reject，不拒绝、不写 counter，但保留成功请求归集。
	common.EnterpriseGovernanceDryRunEnabled = true
	dryRunPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "Runbook dry-run quota",
		TargetType:   model.PolicyTargetPolicyGroup,
		TargetId:     group.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   5,
		ModelScope:   model.PolicyModelScopeAll,
		Priority:     50,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	r2Ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-rollout-r2", 402)
	relayInfo.RequestId = "req-enterprise-rollout-r2"
	require.Nil(t, PreCheckEnterpriseGovernance(r2Ctx, relayInfo, 10))
	decision, ok := common.GetContextKeyType[PolicyDecision](r2Ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	assert.True(t, decision.DryRun)
	assert.True(t, decision.WouldReject)
	assert.Contains(t, decision.MatchedPolicyIds, dryRunPolicy.Id)
	require.NoError(t, RecordEnterpriseTextUsageAttribution(r2Ctx, relayInfo, textQuotaSummary{
		ModelName:        "gpt-4o",
		PromptTokens:     3,
		CompletionTokens: 2,
		TotalTokens:      5,
		Quota:            10,
	}))
	assertEnterpriseGovernanceTableCount(t, &model.EnterpriseQuotaCounter{}, 0)
	var r2Audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-rollout-r2", enterpriseGovernanceAuditActionDryRunReject).
		First(&r2Audit).Error)
	assert.Equal(t, dryRunPolicy.Id, r2Audit.TargetId)
	r2AuditAfter := enterpriseAuditAfterForTest(t, r2Audit)
	assert.Equal(t, "quota_exceeded", r2AuditAfter["deny_type"])
	assert.EqualValues(t, dryRunPolicy.Id, r2AuditAfter["policy_id"])
	assert.EqualValues(t, 5, r2AuditAfter["limit_value"])
	assert.EqualValues(t, 0, r2AuditAfter["used_value"])
	assert.EqualValues(t, 0, r2AuditAfter["reserved_value"])
	assert.EqualValues(t, 10, r2AuditAfter["requested_value"])
	assert.EqualValues(t, 10, r2AuditAfter["suggested_limit_value"])
	assert.Equal(t, "enterprise_governance.group_quota_exceeded", r2AuditAfter["user_message_key"])
	assert.Equal(t, string(types.ErrorCodeEnterpriseGovernanceGroupQuotaExceeded), r2AuditAfter["error_code"])
	assert.Equal(t, true, r2AuditAfter["dry_run"])
	var r2Attribution model.EnterpriseUsageAttribution
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-rollout-r2").First(&r2Attribution).Error)
	assert.Equal(t, userId, r2Attribution.UserId)
	assert.Equal(t, orgUnit.Id, r2Attribution.OrgUnitId)
	assert.JSONEq(t, fmt.Sprintf("[%d]", group.Id), r2Attribution.PolicyGroupIdsJson)

	// R3: 小范围硬限制，第一次请求成功结算，第二次请求 403 且不会产生上游归集。
	common.EnterpriseGovernanceDryRunEnabled = false
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Where("id = ?", dryRunPolicy.Id).
		Update("status", model.QuotaPolicyStatusDisabled).Error)
	hardPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "Runbook hard request count",
		TargetType:   model.PolicyTargetPolicyGroup,
		TargetId:     group.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   1,
		ModelScope:   model.PolicyModelScopeAll,
		Priority:     100,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	r3AllowCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-rollout-r3-allow", 403)
	relayInfo.RequestId = "req-enterprise-rollout-r3-allow"
	require.Nil(t, PreCheckEnterpriseGovernance(r3AllowCtx, relayInfo, 1))
	require.NoError(t, SettleEnterpriseGovernanceUsage(r3AllowCtx, UsageAmount{RequestCount: 1, Quota: 1}))
	require.NoError(t, RecordEnterpriseTextUsageAttribution(r3AllowCtx, relayInfo, textQuotaSummary{
		ModelName:        "gpt-4o",
		TotalTokens:      1,
		Quota:            1,
		PromptTokens:     1,
		CompletionTokens: 0,
	}))
	var hardCounter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", hardPolicy.Id).First(&hardCounter).Error)
	assert.EqualValues(t, 1, hardCounter.UsedValue)
	assert.EqualValues(t, 0, hardCounter.ReservedValue)

	r3RejectCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-rollout-r3-reject", 404)
	relayInfo.RequestId = "req-enterprise-rollout-r3-reject"
	apiErr := PreCheckEnterpriseGovernance(r3RejectCtx, relayInfo, 1)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeEnterpriseGovernanceGroupQuotaExceeded, apiErr.GetErrorCode())
	assert.Equal(t, "enterprise_governance.group_quota_exceeded", apiErr.Error())
	assertEnterpriseGovernanceUserErrorIsRedacted(t, apiErr)
	reservation, _ := common.GetContextKeyType[*Reservation](r3RejectCtx, constant.ContextKeyEnterpriseGovernanceReserve)
	assert.Nil(t, reservation)
	require.NoError(t, model.DB.Where("policy_id = ?", hardPolicy.Id).First(&hardCounter).Error)
	assert.EqualValues(t, 1, hardCounter.UsedValue)
	assert.EqualValues(t, 0, hardCounter.ReservedValue)
	var rejectedAttributionCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseUsageAttribution{}).
		Where("request_id = ?", "req-enterprise-rollout-r3-reject").
		Count(&rejectedAttributionCount).Error)
	assert.EqualValues(t, 0, rejectedAttributionCount)
	var r3RejectAudit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-rollout-r3-reject", enterpriseGovernanceAuditActionHardReject).
		First(&r3RejectAudit).Error)
	assert.Equal(t, hardPolicy.Id, r3RejectAudit.TargetId)
	r3AuditAfter := enterpriseAuditAfterForTest(t, r3RejectAudit)
	assert.Equal(t, "quota_exceeded", r3AuditAfter["deny_type"])
	assert.EqualValues(t, hardPolicy.Id, r3AuditAfter["policy_id"])
	assert.EqualValues(t, 1, r3AuditAfter["limit_value"])
	assert.EqualValues(t, 1, r3AuditAfter["used_value"])
	assert.EqualValues(t, 0, r3AuditAfter["reserved_value"])
	assert.EqualValues(t, 1, r3AuditAfter["requested_value"])
	assert.EqualValues(t, 2, r3AuditAfter["suggested_limit_value"])
	assert.Equal(t, "enterprise_governance.group_quota_exceeded", r3AuditAfter["user_message_key"])
	assert.Equal(t, string(types.ErrorCodeEnterpriseGovernanceGroupQuotaExceeded), r3AuditAfter["error_code"])
	assert.Equal(t, false, r3AuditAfter["dry_run"])
}

func TestEnterpriseGovernanceProjectAttributionAndHeaderOverride(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	const userId = 1030
	const tokenId = 930
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: "project-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	orgUnit := model.EnterpriseOrgUnit{
		EnterpriseId: enterprise.Id,
		Name:         "Project Engineering",
		Slug:         "project-engineering",
		Depth:        1,
		Status:       model.OrgUnitStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&orgUnit).Error)
	orgUnit.Path = fmt.Sprintf("/%d/", orgUnit.Id)
	require.NoError(t, model.DB.Save(&orgUnit).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       userId,
		OrgUnitId:    orgUnit.Id,
		IsPrimary:    true,
	}).Error)
	defaultProject := createEnterpriseProjectForRelayTest(t, enterprise.Id, orgUnit.Id, "Default Cost Center", "default-cost-center")
	overrideProject := createEnterpriseProjectForRelayTest(t, enterprise.Id, orgUnit.Id, "Override Cost Center", "override-cost-center")
	outsideProject := createEnterpriseProjectForRelayTest(t, enterprise.Id, orgUnit.Id+10000, "Outside Cost Center", "outside-cost-center")
	projectPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "override project request limit",
		TargetType:   model.PolicyTargetProject,
		TargetId:     overrideProject.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   1,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	relayInfo := &relaycommon.RelayInfo{
		UserId:          userId,
		TokenId:         tokenId,
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	defaultCtx := newEnterpriseGovernanceRelayTestContext(t, "req-project-default", 501)
	common.SetContextKey(defaultCtx, constant.ContextKeyTokenDefaultProjectId, defaultProject.Id)
	relayInfo.RequestId = "req-project-default"
	require.Nil(t, PreCheckEnterpriseGovernance(defaultCtx, relayInfo, 1))
	require.NoError(t, RecordEnterpriseTextUsageAttribution(defaultCtx, relayInfo, textQuotaSummary{ModelName: "gpt-4o", Quota: 1}))
	var defaultAttribution model.EnterpriseUsageAttribution
	require.NoError(t, model.DB.Where("request_id = ?", "req-project-default").First(&defaultAttribution).Error)
	assert.Equal(t, defaultProject.Id, defaultAttribution.ProjectId)

	overrideCtx := newEnterpriseGovernanceRelayTestContext(t, "req-project-override", 502)
	common.SetContextKey(overrideCtx, constant.ContextKeyTokenDefaultProjectId, defaultProject.Id)
	overrideCtx.Request.Header.Set(enterpriseProjectIdHeader, fmt.Sprintf("%d", overrideProject.Id))
	relayInfo.RequestId = "req-project-override"
	require.Nil(t, PreCheckEnterpriseGovernance(overrideCtx, relayInfo, 1))
	decision, ok := common.GetContextKeyType[PolicyDecision](overrideCtx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	assert.Contains(t, decision.MatchedPolicyIds, projectPolicy.Id)
	require.NoError(t, SettleEnterpriseGovernanceUsage(overrideCtx, UsageAmount{RequestCount: 1}))
	require.NoError(t, RecordEnterpriseTextUsageAttribution(overrideCtx, relayInfo, textQuotaSummary{ModelName: "gpt-4o", Quota: 1}))
	var overrideAttribution model.EnterpriseUsageAttribution
	require.NoError(t, model.DB.Where("request_id = ?", "req-project-override").First(&overrideAttribution).Error)
	assert.Equal(t, overrideProject.Id, overrideAttribution.ProjectId)

	invalidCtx := newEnterpriseGovernanceRelayTestContext(t, "req-project-invalid", 503)
	invalidCtx.Request.Header.Set(enterpriseProjectIdHeader, fmt.Sprintf("%d", outsideProject.Id))
	relayInfo.RequestId = "req-project-invalid"
	apiErr := PreCheckEnterpriseGovernance(invalidCtx, relayInfo, 1)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Contains(t, apiErr.Error(), "不能使用请求项目")
	var invalidCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseUsageAttribution{}).Where("request_id = ?", "req-project-invalid").Count(&invalidCount).Error)
	assert.EqualValues(t, 0, invalidCount)
}

func decisionInternalReasonForTest(t *testing.T, ctx *gin.Context) string {
	t.Helper()
	decision, ok := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	require.NotEmpty(t, strings.TrimSpace(decision.DenyReason))
	return decision.DenyReason
}

func assertEnterpriseGovernanceUserErrorIsRedacted(t *testing.T, apiErr *types.NewAPIError) {
	t.Helper()
	require.NotNil(t, apiErr)
	message := apiErr.Error()
	for _, internalField := range []string{
		"policy_id",
		"policy_ids",
		"target_id",
		"used=",
		"reserved=",
		"requested=",
		"limit=",
		"period_start",
		"period_end",
		"allowed_models",
	} {
		assert.NotContains(t, message, internalField)
	}
}

func newEnterpriseGovernanceRelayTestContext(t *testing.T, requestId string, channelId int) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, requestId)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelId)
	return ctx
}

func createEnterpriseProjectForRelayTest(t *testing.T, enterpriseId int, orgUnitId int, name string, slug string) model.EnterpriseProject {
	t.Helper()
	project := model.EnterpriseProject{
		EnterpriseId: enterpriseId,
		Name:         name,
		Slug:         slug,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&project).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseProjectOrgUnit{
		EnterpriseId: enterpriseId,
		ProjectId:    project.Id,
		OrgUnitId:    orgUnitId,
	}).Error)
	return project
}

func assertEnterpriseGovernanceTableCount(t *testing.T, table any, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(table).Count(&count).Error)
	assert.EqualValues(t, expected, count)
}

func enterpriseAuditAfterForTest(t *testing.T, log model.EnterpriseAuditLog) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(log.AfterJson), &payload))
	return payload
}
