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

func TestApplyEnterpriseGovernanceAnomalyThrottleRequestSpikeAuditsAndProtects(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseAnomalyThrottleForTest(t)
	gin.SetMode(gin.TestMode)

	enterprise, relayInfo := prepareEnterpriseAnomalyThrottleRequest(t, "req-enterprise-anomaly-spike", 1040, 130)
	now := time.Now()
	seedEnterpriseAnomalyUsageAttributions(t, enterprise.Id, 1040, []time.Time{
		now.Add(-15 * time.Minute),
		now.Add(-4 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-2 * time.Minute),
		now.Add(-time.Minute),
	}, 10)

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-spike", 731)
	result, err := ApplyEnterpriseGovernanceAnomalyThrottle(ctx, relayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceAnomalyThrottled))
	assert.True(t, result.Applied)
	assert.Equal(t, enterpriseAnomalyStatusThrottled, result.Status)
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, result.Reason)
	assert.Equal(t, enterpriseAnomalyStatusThrottled, ctx.Writer.Header().Get(enterpriseAnomalyStatusHeader))
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, ctx.Writer.Header().Get(enterpriseAnomalyReasonHeader))
	assert.Equal(t, strconv.FormatInt(result.ProtectedUntil, 10), ctx.Writer.Header().Get(enterpriseAnomalyProtectedUntilHeader))

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-anomaly-spike", enterpriseGovernanceAuditActionAnomalyThrottle).
		First(&audit).Error)
	assert.Equal(t, enterprise.Id, audit.TargetId)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseAnomalyStatusThrottled, auditAfter["anomaly_status"])
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, auditAfter["anomaly_reason"])
	assert.Equal(t, "enterprise_governance.anomaly_throttled", auditAfter["user_message_key"])
	assert.Equal(t, "enterprise_governance_anomaly_throttled", auditAfter["error_code"])
	triggers, ok := auditAfter["anomaly_triggers"].([]any)
	require.True(t, ok)
	require.Len(t, triggers, 1)
	trigger, ok := triggers[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, trigger["reason"])

	var protection model.EnterpriseGovernanceAnomalyProtection
	require.NoError(t, model.DB.Where("enterprise_id = ? AND protection_key = ? AND status = ?", enterprise.Id, enterpriseAnomalyProtectionKey(&EnterpriseContext{EnterpriseId: enterprise.Id}), model.EnterpriseGovernanceAnomalyProtectionStatusActive).
		First(&protection).Error)
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, protection.Reason)
	assert.Equal(t, result.ProtectedUntil, protection.ProtectedUntil)
	assert.NotEmpty(t, protection.PayloadJson)

	protectedRelayInfo := *relayInfo
	protectedRelayInfo.RequestId = "req-enterprise-anomaly-protected"
	protectedCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-protected", 732)
	protected, err := ApplyEnterpriseGovernanceAnomalyThrottle(protectedCtx, &protectedRelayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceAnomalyThrottled))
	assert.True(t, protected.Applied)
	assert.Equal(t, result.ProtectedUntil, protected.ProtectedUntil)
}

func TestApplyEnterpriseGovernanceAnomalyThrottleRestoresProtectionFromDB(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseAnomalyThrottleForTest(t)
	gin.SetMode(gin.TestMode)

	enterprise, relayInfo := prepareEnterpriseAnomalyThrottleRequest(t, "req-enterprise-anomaly-db-restore", 1043, 133)
	now := time.Now()
	seedEnterpriseAnomalyUsageAttributions(t, enterprise.Id, 1043, []time.Time{
		now.Add(-15 * time.Minute),
		now.Add(-4 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-2 * time.Minute),
		now.Add(-time.Minute),
	}, 10)

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-db-restore", 735)
	result, err := ApplyEnterpriseGovernanceAnomalyThrottle(ctx, relayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceAnomalyThrottled))
	require.True(t, result.Applied)

	enterpriseAnomalyProtections = sync.Map{}
	restoredRelayInfo := *relayInfo
	restoredRelayInfo.RequestId = "req-enterprise-anomaly-db-restored"
	restoredCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-db-restored", 736)
	restored, err := ApplyEnterpriseGovernanceAnomalyThrottle(restoredCtx, &restoredRelayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceAnomalyThrottled))
	assert.Equal(t, result.ProtectedUntil, restored.ProtectedUntil)
	assert.Equal(t, enterpriseAnomalyReasonRequestSpike, restored.Reason)
	assert.Equal(t, enterpriseAnomalyStatusThrottled, restoredCtx.Writer.Header().Get(enterpriseAnomalyStatusHeader))
}

func TestApplyEnterpriseGovernanceAnomalyThrottleFailureRate(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseAnomalyThrottleForTest(t)
	enterpriseAnomalyThrottleMinCurrentRequests = 1000
	enterpriseAnomalyThrottleMinCurrentQuota = 1000000
	enterpriseAnomalyThrottleMinFailureRequests = 3
	enterpriseAnomalyThrottleMinFailures = 2
	enterpriseAnomalyThrottleFailureRate = 0.5
	gin.SetMode(gin.TestMode)

	_, relayInfo := prepareEnterpriseAnomalyThrottleRequest(t, "req-enterprise-anomaly-failure", 1041, 131)
	now := common.GetTimestamp()
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 1041, CreatedAt: now - 60, Type: model.LogTypeConsume, ModelName: "gpt-4o", RequestId: "req-ok"},
		{UserId: 1041, CreatedAt: now - 50, Type: model.LogTypeError, ModelName: "gpt-4o", RequestId: "req-error-1"},
		{UserId: 1041, CreatedAt: now - 40, Type: model.LogTypeError, ModelName: "gpt-4o", RequestId: "req-error-2"},
	}).Error)

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-failure", 733)
	result, err := ApplyEnterpriseGovernanceAnomalyThrottle(ctx, relayInfo)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEnterpriseGovernanceAnomalyThrottled))
	assert.Equal(t, enterpriseAnomalyReasonFailureRate, result.Reason)
	assert.EqualValues(t, 3, result.Current.RequestCount)
	assert.EqualValues(t, 2, result.Current.ErrorCount)
}

func TestApplyEnterpriseGovernanceAnomalyThrottleDryRunOnlyAudits(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseAnomalyThrottleForTest(t)
	common.EnterpriseGovernanceDryRunEnabled = true
	gin.SetMode(gin.TestMode)

	enterprise, relayInfo := prepareEnterpriseAnomalyThrottleRequest(t, "req-enterprise-anomaly-dry-run", 1042, 132)
	now := time.Now()
	seedEnterpriseAnomalyUsageAttributions(t, enterprise.Id, 1042, []time.Time{
		now.Add(-15 * time.Minute),
		now.Add(-4 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-2 * time.Minute),
		now.Add(-time.Minute),
	}, 10)

	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-anomaly-dry-run", 734)
	result, err := ApplyEnterpriseGovernanceAnomalyThrottle(ctx, relayInfo)
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Equal(t, enterpriseAnomalyStatusWouldThrottle, result.Status)
	assert.True(t, result.DryRun)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-anomaly-dry-run", enterpriseGovernanceAuditActionAnomalyThrottle).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseAnomalyStatusWouldThrottle, auditAfter["anomaly_status"])
	assert.Equal(t, true, auditAfter["dry_run"])
}

func prepareEnterpriseAnomalyThrottleRequest(t *testing.T, requestId string, userId int, tokenId int) (*model.Enterprise, *relaycommon.RelayInfo) {
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
	return enterprise, &relaycommon.RelayInfo{
		UserId:          userId,
		TokenId:         tokenId,
		RequestId:       requestId,
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}
}

func seedEnterpriseAnomalyUsageAttributions(t *testing.T, enterpriseId int, userId int, timestamps []time.Time, quota int) {
	t.Helper()
	rows := make([]model.EnterpriseUsageAttribution, 0, len(timestamps))
	for index, timestamp := range timestamps {
		rows = append(rows, model.EnterpriseUsageAttribution{
			RequestId:    "req-anomaly-seed-" + strconv.Itoa(index),
			UserId:       userId,
			EnterpriseId: enterpriseId,
			ModelName:    "gpt-4o",
			Quota:        quota,
			Status:       enterpriseUsageAttributionStatusSuccess,
			CreatedAt:    timestamp.Unix(),
		})
	}
	require.NoError(t, model.DB.Create(&rows).Error)
}

func resetEnterpriseAnomalyThrottleForTest(t *testing.T) {
	t.Helper()
	originalEnabled := enterpriseAnomalyThrottleEnabled
	originalCurrentWindow := enterpriseAnomalyThrottleCurrentWindow
	originalBaselineWindow := enterpriseAnomalyThrottleBaselineWindow
	originalCooldown := enterpriseAnomalyThrottleCooldown
	originalMinCurrentRequests := enterpriseAnomalyThrottleMinCurrentRequests
	originalMinBaselineRequests := enterpriseAnomalyThrottleMinBaselineRequests
	originalRequestSpikeRatio := enterpriseAnomalyThrottleRequestSpikeRatio
	originalMinCurrentQuota := enterpriseAnomalyThrottleMinCurrentQuota
	originalMinBaselineQuota := enterpriseAnomalyThrottleMinBaselineQuota
	originalCostSpikeRatio := enterpriseAnomalyThrottleCostSpikeRatio
	originalMinFailureRequests := enterpriseAnomalyThrottleMinFailureRequests
	originalMinFailures := enterpriseAnomalyThrottleMinFailures
	originalFailureRate := enterpriseAnomalyThrottleFailureRate
	enterpriseAnomalyThrottleEnabled = true
	enterpriseAnomalyThrottleCurrentWindow = 10 * time.Minute
	enterpriseAnomalyThrottleBaselineWindow = 10 * time.Minute
	enterpriseAnomalyThrottleCooldown = time.Minute
	enterpriseAnomalyThrottleMinCurrentRequests = 4
	enterpriseAnomalyThrottleMinBaselineRequests = 1
	enterpriseAnomalyThrottleRequestSpikeRatio = 2
	enterpriseAnomalyThrottleMinCurrentQuota = 1000000
	enterpriseAnomalyThrottleMinBaselineQuota = 1000000
	enterpriseAnomalyThrottleCostSpikeRatio = 2
	enterpriseAnomalyThrottleMinFailureRequests = 1000
	enterpriseAnomalyThrottleMinFailures = 1000
	enterpriseAnomalyThrottleFailureRate = 0.5
	enterpriseAnomalyProtections = sync.Map{}
	t.Cleanup(func() {
		enterpriseAnomalyThrottleEnabled = originalEnabled
		enterpriseAnomalyThrottleCurrentWindow = originalCurrentWindow
		enterpriseAnomalyThrottleBaselineWindow = originalBaselineWindow
		enterpriseAnomalyThrottleCooldown = originalCooldown
		enterpriseAnomalyThrottleMinCurrentRequests = originalMinCurrentRequests
		enterpriseAnomalyThrottleMinBaselineRequests = originalMinBaselineRequests
		enterpriseAnomalyThrottleRequestSpikeRatio = originalRequestSpikeRatio
		enterpriseAnomalyThrottleMinCurrentQuota = originalMinCurrentQuota
		enterpriseAnomalyThrottleMinBaselineQuota = originalMinBaselineQuota
		enterpriseAnomalyThrottleCostSpikeRatio = originalCostSpikeRatio
		enterpriseAnomalyThrottleMinFailureRequests = originalMinFailureRequests
		enterpriseAnomalyThrottleMinFailures = originalMinFailures
		enterpriseAnomalyThrottleFailureRate = originalFailureRate
		enterpriseAnomalyProtections = sync.Map{}
	})
}
