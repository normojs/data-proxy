package service

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyEnterpriseGovernanceQueueAdmitsAuditsAndReleases(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 50*time.Millisecond)
	gin.SetMode(gin.TestMode)

	relayInfo, policy := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-admit", 1030, 120)
	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-admit", 701)
	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))

	result, release, err := ApplyEnterpriseGovernanceQueue(ctx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, release)
	defer release()
	assert.True(t, result.Applied)
	assert.Equal(t, enterpriseQueueStatusAdmitted, result.Status)
	assert.Equal(t, "admitted", ctx.Writer.Header().Get(enterpriseQueueStatusHeader))
	assert.Equal(t, strconv.FormatInt(result.TimeoutMs, 10), ctx.Writer.Header().Get(enterpriseQueueTimeoutMsHeader))

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-admit", enterpriseGovernanceAuditActionQueueAdmission).
		First(&audit).Error)
	assert.Equal(t, policy.Id, audit.TargetId)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseQueueStatusAdmitted, auditAfter["queue_status"])
	assert.Equal(t, "gpt-4o", auditAfter["model"])
	assert.EqualValues(t, 701, auditAfter["channel_id"])
	assert.Equal(t, "enterprise_governance.policy_action_observed", auditAfter["user_message_key"])
	actions, ok := auditAfter["policy_actions"].([]any)
	require.True(t, ok)
	require.Len(t, actions, 1)
	action, ok := actions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.PolicyActionQueue, action["action"])
	assert.Equal(t, "quota_exceeded", action["trigger"])

	release()
	secondCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-admit-second", 702)
	secondRelayInfo := *relayInfo
	secondRelayInfo.RequestId = "req-enterprise-queue-admit-second"
	require.Nil(t, PreCheckEnterpriseGovernance(secondCtx, &secondRelayInfo, 20))
	secondResult, secondRelease, err := ApplyEnterpriseGovernanceQueue(secondCtx, &secondRelayInfo)
	require.NoError(t, err)
	require.NotNil(t, secondRelease)
	defer secondRelease()
	assert.Equal(t, enterpriseQueueStatusAdmitted, secondResult.Status)
}

func TestApplyEnterpriseGovernanceQueueTimeoutAudits(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 10*time.Millisecond)
	gin.SetMode(gin.TestMode)

	relayInfo, policy := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-held", 1031, 121)
	heldCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-held", 711)
	require.Nil(t, PreCheckEnterpriseGovernance(heldCtx, relayInfo, 20))
	heldResult, heldRelease, err := ApplyEnterpriseGovernanceQueue(heldCtx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, heldRelease)
	defer heldRelease()
	require.Equal(t, enterpriseQueueStatusAdmitted, heldResult.Status)

	waitingRelayInfo := *relayInfo
	waitingRelayInfo.RequestId = "req-enterprise-queue-timeout"
	waitingCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-timeout", 712)
	require.Nil(t, PreCheckEnterpriseGovernance(waitingCtx, &waitingRelayInfo, 20))
	result, release, err := ApplyEnterpriseGovernanceQueue(waitingCtx, &waitingRelayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceQueueTimeout))
	assert.Nil(t, release)
	assert.True(t, result.Applied)
	assert.Equal(t, enterpriseQueueStatusTimeout, result.Status)
	assert.Equal(t, "timeout", waitingCtx.Writer.Header().Get(enterpriseQueueStatusHeader))
	assert.Equal(t, "10", waitingCtx.Writer.Header().Get(enterpriseQueueTimeoutMsHeader))
	assert.NotEmpty(t, waitingCtx.Writer.Header().Get(enterpriseQueueWaitMsHeader))

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-timeout", enterpriseGovernanceAuditActionQueueAdmission).
		First(&audit).Error)
	assert.Equal(t, policy.Id, audit.TargetId)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseQueueStatusTimeout, auditAfter["queue_status"])
	assert.EqualValues(t, 10, auditAfter["timeout_ms"])
	assert.Equal(t, "enterprise_governance.queue_timeout", auditAfter["user_message_key"])
}

func prepareEnterpriseGovernanceQueueRequest(t *testing.T, requestId string, userId int, tokenId int) (*relaycommon.RelayInfo, model.EnterpriseQuotaPolicy) {
	t.Helper()
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: requestId,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         requestId + " queue policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionQueue,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	return &relaycommon.RelayInfo{
		UserId:          userId,
		TokenId:         tokenId,
		RequestId:       requestId,
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}, policy
}

func resetEnterpriseGovernanceQueueForTest(t *testing.T, maxConcurrent int, timeout time.Duration) {
	t.Helper()
	originalMaxConcurrent := enterprisePolicyQueueMaxConcurrent
	originalTimeout := enterprisePolicyQueueTimeout
	enterprisePolicyQueueMaxConcurrent = maxConcurrent
	enterprisePolicyQueueTimeout = timeout
	enterprisePolicyQueues = syncMapForEnterpriseGovernanceQueueTest()
	t.Cleanup(func() {
		enterprisePolicyQueueMaxConcurrent = originalMaxConcurrent
		enterprisePolicyQueueTimeout = originalTimeout
		enterprisePolicyQueues = syncMapForEnterpriseGovernanceQueueTest()
	})
}

func syncMapForEnterpriseGovernanceQueueTest() sync.Map {
	return sync.Map{}
}
