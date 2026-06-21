package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseSharedPoolBorrowedValue(t *testing.T) {
	tests := []struct {
		name      string
		limit     int64
		used      int64
		reserved  int64
		requested int64
		want      int64
	}{
		{name: "no borrow below limit", limit: 10, requested: 5, want: 0},
		{name: "partial borrow", limit: 10, used: 8, requested: 5, want: 3},
		{name: "full borrow when already over limit", limit: 10, used: 12, requested: 5, want: 5},
		{name: "full borrow for large request", limit: 10, requested: 20, want: 10},
		{name: "ignore empty request", limit: 10, used: 12, requested: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, enterpriseSharedPoolBorrowedValue(tt.limit, tt.used, tt.reserved, tt.requested))
		})
	}
}

func TestApplyEnterpriseGovernanceSharedPoolAuditsBorrow(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1032,
		Username: "req-enterprise-shared-pool",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool soft quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-shared-pool", 721)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1032,
		TokenId:         122,
		RequestId:       "req-enterprise-shared-pool",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))
	decision, ok := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	require.Len(t, decision.ActionObservations, 1)
	observation := decision.ActionObservations[0]
	assert.Equal(t, model.PolicyActionSharedPool, observation.Action)
	assert.Equal(t, model.PolicyMetricQuota, observation.Metric)
	assert.EqualValues(t, 20, observation.RequestedValue)
	assert.EqualValues(t, 10, observation.BorrowedValue)

	result, err := ApplyEnterpriseGovernanceSharedPool(ctx, relayInfo)
	require.NoError(t, err)
	require.True(t, result.Applied)
	assert.Equal(t, enterpriseSharedPoolStatusReserved, result.Status)
	assert.EqualValues(t, 10, result.BorrowedQuota)
	assert.EqualValues(t, 0, result.BorrowedRequestCount)
	assert.Equal(t, enterpriseSharedPoolStatusReserved, ctx.Writer.Header().Get(enterpriseSharedPoolStatusHeader))
	assert.Equal(t, "10", ctx.Writer.Header().Get(enterpriseSharedPoolBorrowedQuotaHeader))
	assert.Equal(t, "0", ctx.Writer.Header().Get(enterpriseSharedPoolBorrowedRequestsHeader))
	assert.Equal(t, "0", ctx.Writer.Header().Get(enterpriseSharedPoolRemainingQuotaHeader))
	assert.Equal(t, "0", ctx.Writer.Header().Get(enterpriseSharedPoolRemainingRequestsHeader))

	var pool model.EnterpriseGovernanceSharedPool
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&pool).Error)
	assert.EqualValues(t, 10, pool.CapacityValue)
	assert.EqualValues(t, 10, pool.ReservedValue)
	assert.EqualValues(t, 0, pool.UsedValue)

	var borrow model.EnterpriseGovernanceSharedPoolBorrow
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-shared-pool").First(&borrow).Error)
	assert.Equal(t, pool.Id, borrow.PoolId)
	assert.Equal(t, policy.Id, borrow.PolicyId)
	assert.Equal(t, model.EnterpriseGovernanceSharedPoolBorrowStatusReserved, borrow.Status)
	assert.EqualValues(t, 10, borrow.ReservedBorrowedValue)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-shared-pool", enterpriseGovernanceAuditActionSharedPoolReserve).
		First(&audit).Error)
	assert.Equal(t, policy.Id, audit.TargetId)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseSharedPoolStatusReserved, auditAfter["shared_pool_status"])
	assert.EqualValues(t, 10, auditAfter["borrowed_quota"])
	assert.EqualValues(t, 0, auditAfter["borrowed_request_count"])
	assert.Equal(t, "enterprise_governance.shared_pool_reserved", auditAfter["user_message_key"])
	records, ok := auditAfter["shared_pool_borrow_rows"].([]any)
	require.True(t, ok)
	require.Len(t, records, 1)
	record, ok := records[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, policy.Id, record["policy_id"])
	assert.Equal(t, model.PolicyMetricQuota, record["metric"])
	assert.EqualValues(t, 10, record["borrowed_value"])
	actions, ok := auditAfter["policy_actions"].([]any)
	require.True(t, ok)
	require.Len(t, actions, 1)
	action, ok := actions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.PolicyActionSharedPool, action["action"])
	assert.EqualValues(t, 10, action["borrowed_value"])
}

func TestEnterpriseGovernanceSharedPoolHeadersExposeRemainingCapacity(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1035,
		Username: "req-enterprise-shared-pool-remaining",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool remaining quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-shared-pool-remaining", 724)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1035,
		TokenId:         125,
		RequestId:       "req-enterprise-shared-pool-remaining",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 15))
	result, err := ApplyEnterpriseGovernanceSharedPool(ctx, relayInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.BorrowedQuota)
	assert.EqualValues(t, 5, result.RemainingQuota)
	assert.Equal(t, "5", ctx.Writer.Header().Get(enterpriseSharedPoolBorrowedQuotaHeader))
	assert.Equal(t, "5", ctx.Writer.Header().Get(enterpriseSharedPoolRemainingQuotaHeader))

	var pool model.EnterpriseGovernanceSharedPool
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&pool).Error)
	assert.EqualValues(t, 5, pool.ReservedValue)
	assert.EqualValues(t, 5, pool.CapacityValue-pool.UsedValue-pool.ReservedValue)
}

func TestEnterpriseGovernanceSharedPoolUsesConfiguredCapacity(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1036,
		Username: "req-enterprise-shared-pool-config",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool configured capacity",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceSharedPoolConfig{
		EnterpriseId:  enterprise.Id,
		PolicyId:      policy.Id,
		Metric:        model.PolicyMetricQuota,
		CapacityValue: 30,
		Status:        model.EnterpriseGovernanceSharedPoolConfigStatusEnabled,
	}).Error)

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-shared-pool-config", 725)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1036,
		TokenId:         126,
		RequestId:       "req-enterprise-shared-pool-config",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 15))
	result, err := ApplyEnterpriseGovernanceSharedPool(ctx, relayInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 5, result.BorrowedQuota)
	assert.EqualValues(t, 25, result.RemainingQuota)
	assert.Equal(t, "25", ctx.Writer.Header().Get(enterpriseSharedPoolRemainingQuotaHeader))

	var pool model.EnterpriseGovernanceSharedPool
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&pool).Error)
	assert.EqualValues(t, 30, pool.CapacityValue)
	assert.EqualValues(t, 5, pool.ReservedValue)
}

func TestEnterpriseGovernanceSharedPoolSettlesAndReturnsUnusedBorrow(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1033,
		Username: "req-enterprise-shared-pool-settle",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool settle quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-shared-pool-settle", 722)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1033,
		TokenId:         123,
		RequestId:       "req-enterprise-shared-pool-settle",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))
	result, err := ApplyEnterpriseGovernanceSharedPool(ctx, relayInfo)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NoError(t, SettleEnterpriseGovernanceUsage(ctx, UsageAmount{Quota: 13}))

	var pool model.EnterpriseGovernanceSharedPool
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&pool).Error)
	assert.EqualValues(t, 10, pool.CapacityValue)
	assert.EqualValues(t, 0, pool.ReservedValue)
	assert.EqualValues(t, 3, pool.UsedValue)

	var borrow model.EnterpriseGovernanceSharedPoolBorrow
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-shared-pool-settle").First(&borrow).Error)
	assert.Equal(t, model.EnterpriseGovernanceSharedPoolBorrowStatusSettled, borrow.Status)
	assert.EqualValues(t, 10, borrow.ReservedBorrowedValue)
	assert.EqualValues(t, 3, borrow.SettledBorrowedValue)
	assert.EqualValues(t, 7, borrow.ReturnedValue)

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 0, counter.ReservedValue)
	assert.EqualValues(t, 13, counter.UsedValue)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-shared-pool-settle", enterpriseGovernanceAuditActionSharedPoolSettle).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, "settled", auditAfter["shared_pool_status"])
	assert.EqualValues(t, 10, auditAfter["borrowed_quota"])
	assert.EqualValues(t, 3, auditAfter["settled_quota"])
	assert.EqualValues(t, 7, auditAfter["returned_quota"])
}

func TestEnterpriseGovernanceSharedPoolInsufficientCapacityAuditsAndBlocks(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1034,
		Username: "req-enterprise-shared-pool-insufficient",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool tight quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-shared-pool-insufficient", 723)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          1034,
		TokenId:         124,
		RequestId:       "req-enterprise-shared-pool-insufficient",
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))
	decision, ok := common.GetContextKeyType[PolicyDecision](ctx, constant.ContextKeyEnterpriseGovernanceDecision)
	require.True(t, ok)
	require.Len(t, decision.ActionObservations, 1)
	observation := decision.ActionObservations[0]
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceSharedPool{
		EnterpriseId:  enterprise.Id,
		PolicyId:      policy.Id,
		Metric:        model.PolicyMetricQuota,
		PeriodStart:   observation.PeriodStart,
		PeriodEnd:     observation.PeriodEnd,
		CapacityValue: observation.LimitValue,
		UsedValue:     9,
		ReservedValue: 0,
	}).Error)

	result, err := ApplyEnterpriseGovernanceSharedPool(ctx, relayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceSharedPoolInsufficient))
	assert.True(t, result.Applied)
	assert.Equal(t, enterpriseSharedPoolStatusInsufficient, result.Status)
	assert.Equal(t, enterpriseSharedPoolStatusInsufficient, ctx.Writer.Header().Get(enterpriseSharedPoolStatusHeader))
	assert.Equal(t, "1", ctx.Writer.Header().Get(enterpriseSharedPoolRemainingQuotaHeader))

	var borrowCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceSharedPoolBorrow{}).Where("request_id = ?", "req-enterprise-shared-pool-insufficient").Count(&borrowCount).Error)
	assert.EqualValues(t, 0, borrowCount)

	var pool model.EnterpriseGovernanceSharedPool
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&pool).Error)
	assert.EqualValues(t, 9, pool.UsedValue)
	assert.EqualValues(t, 0, pool.ReservedValue)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-shared-pool-insufficient", enterpriseGovernanceAuditActionSharedPoolReserve).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseSharedPoolStatusInsufficient, auditAfter["shared_pool_status"])
	assert.Equal(t, "enterprise_governance.shared_pool_insufficient", auditAfter["user_message_key"])
}
