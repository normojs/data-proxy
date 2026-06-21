package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	requestBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"queue me"}]}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?stream=false", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Authorization", "Bearer queue-payload-secret")
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
	var requestPayload enterpriseQueueRequestPayload
	require.NoError(t, common.UnmarshalJsonStr(admission.RequestPayloadJson, &requestPayload))
	assert.Equal(t, http.MethodPost, requestPayload.Method)
	assert.Equal(t, "/v1/chat/completions", requestPayload.Path)
	assert.Equal(t, "stream=false", requestPayload.RawQuery)
	assert.Equal(t, "application/json", requestPayload.ContentType)
	assert.Equal(t, requestBody, requestPayload.Body)
	assert.Equal(t, len(requestBody), requestPayload.BodyCapturedBytes)
	assert.False(t, requestPayload.BodyTruncated)
	assert.Equal(t, enterpriseQueueRequestPayloadMaxBytes, requestPayload.BodyCaptureLimitBytes)
	assert.Equal(t, "gpt-4o", requestPayload.Model)
	assert.EqualValues(t, relayconstant.RelayModeChatCompletions, requestPayload.RelayMode)
	assert.EqualValues(t, 701, requestPayload.ChannelId)
	assert.NotContains(t, admission.RequestPayloadJson, "queue-payload-secret")
	restoredBody, err := io.ReadAll(ctx.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, requestBody, string(restoredBody))
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
	assert.Equal(t, ErrEnterpriseGovernanceQueueTimeout.Error(), admission.LastError)
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
	assert.NotEmpty(t, admission.LastError)
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
	assert.Equal(t, ErrEnterpriseGovernanceQueueCanceled.Error(), admission.LastError)

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
		{
			RequestId:      "req-enterprise-queue-stale-replay",
			EnterpriseId:   enterprise.Id,
			UserId:         1037,
			TokenId:        127,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusReplayProcessing,
			UserMessageKey: "enterprise_governance.queue_replay_processing",
			CreatedAt:      now - 20*60,
			UpdatedAt:      now - 11*60,
		},
	}).Error)

	stats, err := RecoverStaleEnterpriseGovernanceQueueAdmissions(now, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 3, stats.Scanned)
	assert.EqualValues(t, 2, stats.TimedOut)
	assert.EqualValues(t, 1, stats.Canceled)
	assert.Zero(t, stats.Errors)

	var queued model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-stale-queued").First(&queued).Error)
	assert.Equal(t, enterpriseQueueStatusTimeout, queued.Status)
	assert.EqualValues(t, 120000, queued.WaitMs)
	assert.Equal(t, "enterprise_governance.queue_timeout", queued.UserMessageKey)
	assert.Equal(t, "enterprise governance queue admission recovered as timeout", queued.LastError)

	var admitted model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-stale-admitted").First(&admitted).Error)
	assert.Equal(t, enterpriseQueueStatusCanceled, admitted.Status)
	assert.EqualValues(t, now, admitted.CanceledAt)
	assert.EqualValues(t, 2*60*60*1000, admitted.RunMs)
	assert.Equal(t, "enterprise_governance.queue_canceled", admitted.UserMessageKey)
	assert.Equal(t, "enterprise governance admitted queue admission recovered as canceled", admitted.LastError)

	var fresh model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-fresh").First(&fresh).Error)
	assert.Equal(t, enterpriseQueueStatusQueued, fresh.Status)

	var replay model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-stale-replay").First(&replay).Error)
	assert.Equal(t, enterpriseQueueStatusTimeout, replay.Status)
	assert.Equal(t, "enterprise_governance.queue_timeout", replay.UserMessageKey)
	assert.Equal(t, "enterprise governance queue replay recovered as timeout", replay.LastError)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("action = ?", enterpriseGovernanceAuditActionQueueRecovery).
		Count(&auditCount).Error)
	assert.EqualValues(t, 3, auditCount)
}

func TestRetryEnterpriseGovernanceQueueAdmissionMarksRetryPending(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000100)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceQueueAdmission{
		{
			RequestId:          "req-enterprise-queue-retry-timeout",
			EnterpriseId:       enterprise.Id,
			UserId:             1037,
			TokenId:            127,
			QueueKey:           "enterprise:1",
			Status:             model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
			TimeoutMs:          30000,
			RequestPayloadJson: `{"method":"POST","path":"/v1/chat/completions"}`,
			LastError:          ErrEnterpriseGovernanceQueueTimeout.Error(),
			UserMessageKey:     "enterprise_governance.queue_timeout",
			CreatedAt:          now - 60,
			UpdatedAt:          now - 60,
		},
		{
			RequestId:      "req-enterprise-queue-retry-released",
			EnterpriseId:   enterprise.Id,
			UserId:         1038,
			TokenId:        128,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusReleased,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      now - 30,
			UpdatedAt:      now - 30,
		},
	}).Error)

	var timeoutAdmission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-retry-timeout").First(&timeoutAdmission).Error)
	retry, err := RetryEnterpriseGovernanceQueueAdmission(timeoutAdmission, now)
	require.NoError(t, err)
	assert.Equal(t, enterpriseQueueStatusRetryPending, retry.Status)
	assert.EqualValues(t, 1, retry.RetryCount)
	assert.EqualValues(t, now, retry.NextRetryAt)
	assert.Empty(t, retry.LastError)
	assert.Equal(t, "enterprise_governance.queue_retry_pending", retry.UserMessageKey)

	var releasedAdmission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-retry-released").First(&releasedAdmission).Error)
	_, err = RetryEnterpriseGovernanceQueueAdmission(releasedAdmission, now)
	require.ErrorIs(t, err, ErrEnterpriseGovernanceQueueAdmissionNotRetryable)
}

func TestEnterpriseGovernanceQueuePayloadSnapshotTruncatesAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	requestBody := strings.Repeat("a", enterpriseQueueRequestPayloadMaxBytes+512)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 733)
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}

	payloadJson, err := enterpriseGovernanceQueueRequestPayloadJson(ctx, relayInfo)
	require.NoError(t, err)

	var payload enterpriseQueueRequestPayload
	require.NoError(t, common.UnmarshalJsonStr(payloadJson, &payload))
	assert.True(t, payload.BodyTruncated)
	assert.Equal(t, enterpriseQueueRequestPayloadMaxBytes, payload.BodyCapturedBytes)
	assert.Equal(t, strings.Repeat("a", enterpriseQueueRequestPayloadMaxBytes), payload.Body)
	assert.EqualValues(t, len(requestBody), payload.ContentLength)
	restoredBody, err := io.ReadAll(ctx.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, requestBody, string(restoredBody))
}

func TestProcessEnterpriseGovernanceQueueReplayBatchReplaysDueAdmission(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000200)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1040,
		Username: "queue-replay-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:           140,
		UserId:       1040,
		Key:          "queue-replay-token",
		Status:       common.TokenStatusEnabled,
		RemainQuota:  1000,
		ExpiredTime:  -1,
		Name:         "Queue Replay Token",
		CreatedTime:  now - 100,
		AccessedTime: now - 100,
	}).Error)
	payload := EnterpriseGovernanceQueueRequestPayload{
		Method:                http.MethodPost,
		Path:                  "/v1/chat/completions",
		RawQuery:              "stream=false",
		ContentType:           "application/json",
		Body:                  `{"model":"gpt-4o","messages":[{"role":"user","content":"again"}]}`,
		BodyCapturedBytes:     66,
		BodyCaptureLimitBytes: enterpriseQueueRequestPayloadMaxBytes,
		Model:                 "gpt-4o",
		RelayMode:             relayconstant.RelayModeChatCompletions,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceQueueAdmission{
		RequestId:          "req-enterprise-queue-replay-success",
		EnterpriseId:       enterprise.Id,
		UserId:             1040,
		TokenId:            140,
		QueueKey:           "enterprise:1",
		Status:             model.EnterpriseGovernanceQueueAdmissionStatusRetryPending,
		RetryCount:         1,
		NextRetryAt:        now,
		RequestPayloadJson: string(payloadJson),
		UserMessageKey:     "enterprise_governance.queue_retry_pending",
		CreatedAt:          now - 60,
		UpdatedAt:          now - 60,
	}).Error)

	var gotRequest EnterpriseGovernanceQueueReplayRequest
	stats, err := processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), now, 10, func(ctx context.Context, request EnterpriseGovernanceQueueReplayRequest) EnterpriseGovernanceQueueReplayResult {
		gotRequest = request
		return EnterpriseGovernanceQueueReplayResult{StatusCode: http.StatusOK, DurationMs: 12}
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.Scanned)
	assert.EqualValues(t, 1, stats.Claimed)
	assert.EqualValues(t, 1, stats.Replayed)
	assert.Zero(t, stats.Failed)
	assert.Equal(t, "/v1/chat/completions", gotRequest.Path)
	assert.Equal(t, "stream=false", gotRequest.RawQuery)
	assert.Equal(t, "queue-replay-token", gotRequest.TokenKey)
	assert.Contains(t, string(gotRequest.Body), "again")

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-replay-success").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusReleased, admission.Status)
	assert.EqualValues(t, now, admission.AdmittedAt)
	assert.EqualValues(t, now, admission.ReleasedAt)
	assert.EqualValues(t, 12, admission.RunMs)
	assert.Empty(t, admission.LastError)
	assert.Equal(t, "enterprise_governance.policy_action_observed", admission.UserMessageKey)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-replay-success", enterpriseGovernanceAuditActionQueueReplay).First(&audit).Error)
	auditAfter := enterpriseAuditAfterForTest(t, audit)
	replay, ok := auditAfter["replay"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, replay["success"])
	assert.EqualValues(t, http.StatusOK, replay["status_code"])
	assert.Equal(t, "/v1/chat/completions", replay["path"])
	assert.NotContains(t, audit.AfterJson, "queue-replay-token")
}

func TestProcessEnterpriseGovernanceQueueReplayBatchFailsTruncatedPayload(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000300)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1041,
		Username: "queue-replay-truncated-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          141,
		UserId:      1041,
		Key:         "queue-replay-truncated-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
		ExpiredTime: -1,
		Name:        "Queue Replay Truncated Token",
	}).Error)
	payload := EnterpriseGovernanceQueueRequestPayload{
		Method:        http.MethodPost,
		Path:          "/v1/chat/completions",
		ContentType:   "application/json",
		Body:          strings.Repeat("a", enterpriseQueueRequestPayloadMaxBytes),
		BodyTruncated: true,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceQueueAdmission{
		RequestId:          "req-enterprise-queue-replay-truncated",
		EnterpriseId:       enterprise.Id,
		UserId:             1041,
		TokenId:            141,
		QueueKey:           "enterprise:1",
		Status:             model.EnterpriseGovernanceQueueAdmissionStatusRetryPending,
		RetryCount:         1,
		NextRetryAt:        now,
		RequestPayloadJson: string(payloadJson),
		UserMessageKey:     "enterprise_governance.queue_retry_pending",
		CreatedAt:          now - 60,
		UpdatedAt:          now - 60,
	}).Error)

	called := false
	stats, err := processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), now, 10, func(ctx context.Context, request EnterpriseGovernanceQueueReplayRequest) EnterpriseGovernanceQueueReplayResult {
		called = true
		return EnterpriseGovernanceQueueReplayResult{StatusCode: http.StatusOK}
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.EqualValues(t, 1, stats.Claimed)
	assert.EqualValues(t, 1, stats.Failed)
	assert.Zero(t, stats.Replayed)

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-replay-truncated").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusTimeout, admission.Status)
	assert.Contains(t, admission.LastError, ErrEnterpriseGovernanceQueueReplayPayloadTruncated.Error())
	assert.Equal(t, "enterprise_governance.queue_timeout", admission.UserMessageKey)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-replay-truncated", enterpriseGovernanceAuditActionQueueReplay).First(&audit).Error)
	assert.Contains(t, audit.AfterJson, ErrEnterpriseGovernanceQueueReplayPayloadTruncated.Error())
	assert.NotContains(t, audit.AfterJson, "queue-replay-truncated-token")
}

func TestProcessEnterpriseGovernanceQueueReplayBatchSkipsFutureAdmission(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000400)
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceQueueAdmission{
		RequestId:      "req-enterprise-queue-replay-future",
		EnterpriseId:   enterprise.Id,
		UserId:         1042,
		TokenId:        142,
		QueueKey:       "enterprise:1",
		Status:         model.EnterpriseGovernanceQueueAdmissionStatusRetryPending,
		RetryCount:     1,
		NextRetryAt:    now + 60,
		UserMessageKey: "enterprise_governance.queue_retry_pending",
		CreatedAt:      now - 60,
		UpdatedAt:      now - 60,
	}).Error)

	called := false
	stats, err := processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), now, 10, func(ctx context.Context, request EnterpriseGovernanceQueueReplayRequest) EnterpriseGovernanceQueueReplayResult {
		called = true
		return EnterpriseGovernanceQueueReplayResult{StatusCode: http.StatusOK}
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.Zero(t, stats.Scanned)
	assert.Zero(t, stats.Claimed)

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-replay-future").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusRetryPending, admission.Status)
}

func TestProcessEnterpriseGovernanceQueueReplayBatchRequiresExecutor(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	_, err := processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), 1700000500, 10, nil)
	require.ErrorIs(t, err, ErrEnterpriseGovernanceQueueReplayExecutorNotConfigured)
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
