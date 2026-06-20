package service

import (
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
