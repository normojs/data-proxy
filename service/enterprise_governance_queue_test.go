package service

import (
	"context"
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

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-admit").First(&admission).Error)
	assert.Equal(t, policy.Id, admission.PolicyId)
	assert.Equal(t, enterpriseQueueStatusAdmitted, admission.Status)
	assert.Equal(t, "enterprise:"+strconv.Itoa(policy.EnterpriseId), admission.QueueKey)
	assert.Equal(t, "gpt-4o", admission.ModelName)
	assert.EqualValues(t, 701, admission.ChannelId)
	assert.EqualValues(t, relayconstant.RelayModeChatCompletions, admission.RelayMode)
	assert.Equal(t, "enterprise_governance.policy_action_observed", admission.UserMessageKey)
	assert.Greater(t, admission.AdmittedAt, int64(0))
	assert.Zero(t, admission.ReleasedAt)
	assert.Equal(t, strconv.FormatInt(result.TimeoutMs, 10), ctx.Writer.Header().Get(enterpriseQueueTimeoutMsHeader))
	var admissionPolicyActions []PolicyActionObservation
	require.NoError(t, common.Unmarshal([]byte(admission.PolicyActionsJson), &admissionPolicyActions))
	require.Len(t, admissionPolicyActions, 1)
	assert.Equal(t, model.PolicyActionQueue, admissionPolicyActions[0].Action)

	release()
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-admit").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusReleased, admission.Status)
	assert.Greater(t, admission.ReleasedAt, int64(0))
	assert.GreaterOrEqual(t, admission.RunMs, int64(0))
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

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-timeout").First(&admission).Error)
	assert.Equal(t, policy.Id, admission.PolicyId)
	assert.Equal(t, enterpriseQueueStatusTimeout, admission.Status)
	assert.EqualValues(t, 10, admission.TimeoutMs)
	assert.GreaterOrEqual(t, admission.WaitMs, int64(10))
	assert.Equal(t, "enterprise_governance.queue_timeout", admission.UserMessageKey)
	assert.Zero(t, admission.AdmittedAt)
	assert.Zero(t, admission.ReleasedAt)
	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).Where("request_id = ?", "req-enterprise-queue-timeout").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestApplyEnterpriseGovernanceQueueCancelUpdatesQueuedAdmission(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 200*time.Millisecond)
	gin.SetMode(gin.TestMode)

	relayInfo, _ := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-held-cancel", 1032, 122)
	heldCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-held-cancel", 721)
	require.Nil(t, PreCheckEnterpriseGovernance(heldCtx, relayInfo, 20))
	heldResult, heldRelease, err := ApplyEnterpriseGovernanceQueue(heldCtx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, heldRelease)
	defer heldRelease()
	require.Equal(t, enterpriseQueueStatusAdmitted, heldResult.Status)

	waitingRelayInfo := *relayInfo
	waitingRelayInfo.RequestId = "req-enterprise-queue-canceled"
	waitingCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-canceled", 722)
	ctxWithCancel, cancel := context.WithCancel(waitingCtx.Request.Context())
	waitingCtx.Request = waitingCtx.Request.WithContext(ctxWithCancel)
	require.Nil(t, PreCheckEnterpriseGovernance(waitingCtx, &waitingRelayInfo, 20))

	type queueOutcome struct {
		result  EnterpriseGovernanceQueueResult
		release func()
		err     error
	}
	done := make(chan queueOutcome, 1)
	go func() {
		result, release, err := ApplyEnterpriseGovernanceQueue(waitingCtx, &waitingRelayInfo)
		done <- queueOutcome{result: result, release: release, err: err}
	}()

	require.Eventually(t, func() bool {
		var admission model.EnterpriseGovernanceQueueAdmission
		err := model.DB.Where("request_id = ?", "req-enterprise-queue-canceled").First(&admission).Error
		return err == nil && admission.Status == enterpriseQueueStatusQueued
	}, time.Second, 5*time.Millisecond)
	cancel()

	var outcome queueOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queue cancellation")
	}
	require.ErrorIs(t, outcome.err, context.Canceled)
	assert.Nil(t, outcome.release)
	assert.Equal(t, enterpriseQueueStatusCanceled, outcome.result.Status)
	assert.Equal(t, "canceled", waitingCtx.Writer.Header().Get(enterpriseQueueStatusHeader))

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-canceled").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusCanceled, admission.Status)
	assert.Greater(t, admission.CanceledAt, int64(0))
	assert.Zero(t, admission.AdmittedAt)
	assert.Zero(t, admission.ReleasedAt)
	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).Where("request_id = ?", "req-enterprise-queue-canceled").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestCancelEnterpriseGovernanceQueuedAdmissionCancelsWaitingRequest(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 200*time.Millisecond)
	gin.SetMode(gin.TestMode)

	relayInfo, _ := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-held-admin-cancel", 1033, 123)
	heldCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-held-admin-cancel", 731)
	require.Nil(t, PreCheckEnterpriseGovernance(heldCtx, relayInfo, 20))
	heldResult, heldRelease, err := ApplyEnterpriseGovernanceQueue(heldCtx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, heldRelease)
	defer heldRelease()
	require.Equal(t, enterpriseQueueStatusAdmitted, heldResult.Status)

	waitingRelayInfo := *relayInfo
	waitingRelayInfo.RequestId = "req-enterprise-queue-admin-canceled"
	waitingCtx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-admin-canceled", 732)
	require.Nil(t, PreCheckEnterpriseGovernance(waitingCtx, &waitingRelayInfo, 20))

	type queueOutcome struct {
		result  EnterpriseGovernanceQueueResult
		release func()
		err     error
	}
	done := make(chan queueOutcome, 1)
	go func() {
		result, release, err := ApplyEnterpriseGovernanceQueue(waitingCtx, &waitingRelayInfo)
		done <- queueOutcome{result: result, release: release, err: err}
	}()

	var admission model.EnterpriseGovernanceQueueAdmission
	require.Eventually(t, func() bool {
		err := model.DB.Where("request_id = ?", "req-enterprise-queue-admin-canceled").First(&admission).Error
		return err == nil && admission.Status == enterpriseQueueStatusQueued
	}, time.Second, 5*time.Millisecond)

	after, err := CancelEnterpriseGovernanceQueuedAdmission(admission)
	require.NoError(t, err)
	assert.Equal(t, enterpriseQueueStatusCanceled, after.Status)
	assert.Equal(t, "enterprise_governance.queue_canceled", after.UserMessageKey)

	var outcome queueOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for admin queue cancellation")
	}
	require.ErrorIs(t, outcome.err, ErrEnterpriseGovernanceQueueCanceled)
	assert.Nil(t, outcome.release)
	assert.Equal(t, enterpriseQueueStatusCanceled, outcome.result.Status)
	assert.Equal(t, "canceled", waitingCtx.Writer.Header().Get(enterpriseQueueStatusHeader))

	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-admin-canceled").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusCanceled, admission.Status)
	assert.Greater(t, admission.CanceledAt, int64(0))
	assert.Equal(t, "enterprise_governance.queue_canceled", admission.UserMessageKey)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-admin-canceled", enterpriseGovernanceAuditActionQueueAdmission).
		First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	assert.Equal(t, enterpriseQueueStatusCanceled, auditAfter["queue_status"])
	assert.Equal(t, "enterprise_governance.queue_canceled", auditAfter["user_message_key"])
}

func TestRecoverStaleEnterpriseGovernanceQueueAdmissionsMarksStaleRows(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000000)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceQueueAdmission{
		{
			RequestId:      "req-enterprise-queue-stale-queued",
			EnterpriseId:   enterprise.Id,
			UserId:         1034,
			TokenId:        124,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusQueued,
			TimeoutMs:      30000,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      now - 120,
			UpdatedAt:      now - 120,
		},
		{
			RequestId:      "req-enterprise-queue-stale-admitted",
			EnterpriseId:   enterprise.Id,
			UserId:         1035,
			TokenId:        125,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusAdmitted,
			TimeoutMs:      30000,
			AdmittedAt:     now - 2*60*60,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      now - 2*60*60,
			UpdatedAt:      now - 2*60*60,
		},
		{
			RequestId:      "req-enterprise-queue-fresh",
			EnterpriseId:   enterprise.Id,
			UserId:         1036,
			TokenId:        126,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusQueued,
			TimeoutMs:      30000,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}).Error)

	stats, err := RecoverStaleEnterpriseGovernanceQueueAdmissions(now, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 2, stats.Scanned)
	assert.EqualValues(t, 1, stats.TimedOut)
	assert.EqualValues(t, 1, stats.Canceled)
	assert.Zero(t, stats.Errors)

	var queued model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-stale-queued").First(&queued).Error)
	assert.Equal(t, enterpriseQueueStatusTimeout, queued.Status)
	assert.EqualValues(t, 120000, queued.WaitMs)
	assert.Equal(t, "enterprise_governance.queue_timeout", queued.UserMessageKey)

	var admitted model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-stale-admitted").First(&admitted).Error)
	assert.Equal(t, enterpriseQueueStatusCanceled, admitted.Status)
	assert.EqualValues(t, now, admitted.CanceledAt)
	assert.EqualValues(t, 2*60*60*1000, admitted.RunMs)
	assert.Equal(t, "enterprise_governance.queue_canceled", admitted.UserMessageKey)

	var fresh model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-fresh").First(&fresh).Error)
	assert.Equal(t, enterpriseQueueStatusQueued, fresh.Status)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("action = ?", enterpriseGovernanceAuditActionQueueRecovery).
		Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount)
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
	originalAdmittedStale := enterprisePolicyQueueAdmittedStale
	enterprisePolicyQueueMaxConcurrent = maxConcurrent
	enterprisePolicyQueueTimeout = timeout
	enterprisePolicyQueueAdmittedStale = time.Hour
	enterprisePolicyQueues = syncMapForEnterpriseGovernanceQueueTest()
	enterprisePolicyQueueCancelers = syncMapForEnterpriseGovernanceQueueTest()
	t.Cleanup(func() {
		enterprisePolicyQueueMaxConcurrent = originalMaxConcurrent
		enterprisePolicyQueueTimeout = originalTimeout
		enterprisePolicyQueueAdmittedStale = originalAdmittedStale
		enterprisePolicyQueues = syncMapForEnterpriseGovernanceQueueTest()
		enterprisePolicyQueueCancelers = syncMapForEnterpriseGovernanceQueueTest()
	})
}

func syncMapForEnterpriseGovernanceQueueTest() sync.Map {
	return sync.Map{}
}
