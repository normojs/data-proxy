package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
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

func TestCleanupEnterpriseGovernanceQueuePayloadsDeletesOnlySafeRows(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000100)
	ttlSeconds := int64(3600)
	cutoff := now - ttlSeconds
	admissions := []model.EnterpriseGovernanceQueueAdmission{
		{
			RequestId:      "req-queue-payload-cleanup-released",
			EnterpriseId:   enterprise.Id,
			UserId:         1038,
			TokenId:        128,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusReleased,
			ReleasedAt:     cutoff - 10,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      cutoff - 120,
			UpdatedAt:      cutoff - 10,
		},
		{
			RequestId:      "req-queue-payload-cleanup-retry",
			EnterpriseId:   enterprise.Id,
			UserId:         1039,
			TokenId:        129,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusRetryPending,
			RetryCount:     1,
			NextRetryAt:    now,
			UserMessageKey: "enterprise_governance.queue_retry_pending",
			CreatedAt:      cutoff - 120,
			UpdatedAt:      cutoff - 120,
		},
		{
			RequestId:      "req-queue-payload-cleanup-fresh-release",
			EnterpriseId:   enterprise.Id,
			UserId:         1040,
			TokenId:        130,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusReleased,
			ReleasedAt:     cutoff + 10,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      cutoff - 120,
			UpdatedAt:      cutoff + 10,
		},
		{
			RequestId:      "req-queue-payload-cleanup-timeout",
			EnterpriseId:   enterprise.Id,
			UserId:         1041,
			TokenId:        131,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
			UserMessageKey: "enterprise_governance.queue_timeout",
			CreatedAt:      cutoff - 120,
			UpdatedAt:      cutoff - 120,
		},
	}
	require.NoError(t, model.DB.Create(&admissions).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceQueuePayload{
		{
			AdmissionId:   admissions[0].Id,
			RequestId:     admissions[0].RequestId,
			EnterpriseId:  enterprise.Id,
			UserId:        admissions[0].UserId,
			TokenId:       admissions[0].TokenId,
			ContentType:   "application/json",
			ContentLength: 10,
			Body:          []byte("released-1"),
			BodyBytes:     10,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("released-1")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff - 120,
			UpdatedAt:     cutoff - 120,
		},
		{
			AdmissionId:   admissions[1].Id,
			RequestId:     admissions[1].RequestId,
			EnterpriseId:  enterprise.Id,
			UserId:        admissions[1].UserId,
			TokenId:       admissions[1].TokenId,
			ContentType:   "application/json",
			ContentLength: 10,
			Body:          []byte("retry-keep"),
			BodyBytes:     10,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("retry-keep")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff - 120,
			UpdatedAt:     cutoff - 120,
		},
		{
			AdmissionId:   admissions[2].Id,
			RequestId:     admissions[2].RequestId,
			EnterpriseId:  enterprise.Id,
			UserId:        admissions[2].UserId,
			TokenId:       admissions[2].TokenId,
			ContentType:   "application/json",
			ContentLength: 12,
			Body:          []byte("fresh-release"),
			BodyBytes:     12,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("fresh-release")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff - 120,
			UpdatedAt:     cutoff - 120,
		},
		{
			AdmissionId:   admissions[3].Id,
			RequestId:     admissions[3].RequestId,
			EnterpriseId:  enterprise.Id,
			UserId:        admissions[3].UserId,
			TokenId:       admissions[3].TokenId,
			ContentType:   "application/json",
			ContentLength: 12,
			Body:          []byte("timeout-keep"),
			BodyBytes:     12,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("timeout-keep")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff - 120,
			UpdatedAt:     cutoff - 120,
		},
		{
			AdmissionId:   999999,
			RequestId:     "req-queue-payload-cleanup-orphan",
			EnterpriseId:  enterprise.Id,
			UserId:        1042,
			TokenId:       132,
			ContentType:   "application/json",
			ContentLength: 20,
			Body:          []byte("orphaned-payload-body"),
			BodyBytes:     20,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("orphaned-payload-body")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff - 120,
			UpdatedAt:     cutoff - 120,
		},
		{
			AdmissionId:   admissions[0].Id,
			RequestId:     "req-queue-payload-cleanup-new-payload",
			EnterpriseId:  enterprise.Id,
			UserId:        admissions[0].UserId,
			TokenId:       admissions[0].TokenId,
			ContentType:   "application/json",
			ContentLength: 12,
			Body:          []byte("new-payload"),
			BodyBytes:     12,
			SHA256:        enterpriseGovernanceQueueBodySHA256([]byte("new-payload")),
			StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
			CreatedAt:     cutoff + 10,
			UpdatedAt:     cutoff + 10,
		},
	}).Error)

	preview, err := CleanupEnterpriseGovernanceQueuePayloads(now, ttlSeconds, 10, true)
	require.NoError(t, err)
	assert.True(t, preview.DryRun)
	assert.EqualValues(t, 5, preview.Scanned)
	assert.EqualValues(t, 2, preview.Deleted)
	assert.EqualValues(t, 1, preview.DeletedReleased)
	assert.EqualValues(t, 1, preview.DeletedOrphaned)
	assert.EqualValues(t, 30, preview.DeletedBytes)
	assert.EqualValues(t, 3, preview.Skipped)

	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueuePayload{}).Count(&count).Error)
	assert.EqualValues(t, 6, count)

	cleaned, err := CleanupEnterpriseGovernanceQueuePayloads(now, ttlSeconds, 10, false)
	require.NoError(t, err)
	assert.False(t, cleaned.DryRun)
	assert.EqualValues(t, 5, cleaned.Scanned)
	assert.EqualValues(t, 2, cleaned.Deleted)
	assert.EqualValues(t, 30, cleaned.DeletedBytes)
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueuePayload{}).Count(&count).Error)
	assert.EqualValues(t, 4, count)

	for requestId, expected := range map[string]int64{
		"req-queue-payload-cleanup-released":      0,
		"req-queue-payload-cleanup-orphan":        0,
		"req-queue-payload-cleanup-retry":         1,
		"req-queue-payload-cleanup-fresh-release": 1,
		"req-queue-payload-cleanup-timeout":       1,
		"req-queue-payload-cleanup-new-payload":   1,
	} {
		require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueuePayload{}).
			Where("request_id = ?", requestId).
			Count(&count).Error)
		assert.EqualValues(t, expected, count, requestId)
	}
}

func TestCleanupEnterpriseGovernanceQueuePayloadsDeletesObjectStorage(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER", "local")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_DIR", t.TempDir())
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000150)
	ttlSeconds := int64(3600)
	cutoff := now - ttlSeconds
	admission := model.EnterpriseGovernanceQueueAdmission{
		RequestId:      "req-queue-payload-cleanup-object",
		EnterpriseId:   enterprise.Id,
		UserId:         1050,
		TokenId:        150,
		QueueKey:       "enterprise:1",
		Status:         model.EnterpriseGovernanceQueueAdmissionStatusReleased,
		ReleasedAt:     cutoff - 10,
		UserMessageKey: "enterprise_governance.policy_action_observed",
		CreatedAt:      cutoff - 120,
		UpdatedAt:      cutoff - 10,
	}
	require.NoError(t, model.DB.Create(&admission).Error)
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"object cleanup"}]}`)
	bodySHA256 := enterpriseGovernanceQueueBodySHA256(body)
	row := model.EnterpriseGovernanceQueuePayload{
		AdmissionId:   admission.Id,
		RequestId:     admission.RequestId,
		EnterpriseId:  admission.EnterpriseId,
		UserId:        admission.UserId,
		TokenId:       admission.TokenId,
		ContentType:   "application/json",
		ContentLength: int64(len(body)),
		BodyBytes:     int64(len(body)),
		SHA256:        bodySHA256,
		StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageObject,
		CreatedAt:     cutoff - 120,
		UpdatedAt:     cutoff - 120,
	}
	object, err := SaveEnterpriseGovernanceQueuePayloadObject(context.Background(), row, body)
	require.NoError(t, err)
	row.ObjectId = object.Id
	row.Provider = object.Provider
	row.StorageKey = object.StorageKey
	require.NoError(t, model.DB.Create(&row).Error)

	preview, err := CleanupEnterpriseGovernanceQueuePayloads(now, ttlSeconds, 10, true)
	require.NoError(t, err)
	assert.EqualValues(t, 1, preview.Deleted)
	assert.EqualValues(t, 1, preview.DeletedObjects)
	_, loadedBody, err := LoadEnterpriseGovernanceQueuePayloadObject(context.Background(), object.Id)
	require.NoError(t, err)
	assert.Equal(t, body, loadedBody)

	cleaned, err := CleanupEnterpriseGovernanceQueuePayloads(now, ttlSeconds, 10, false)
	require.NoError(t, err)
	assert.EqualValues(t, 1, cleaned.Deleted)
	assert.EqualValues(t, 1, cleaned.DeletedObjects)
	_, _, err = LoadEnterpriseGovernanceQueuePayloadObject(context.Background(), object.Id)
	require.Error(t, err)

	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueuePayload{}).Where("id = ?", row.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestEnterpriseGovernanceQueuePayloadObjectStoreS3SaveLoadDelete(t *testing.T) {
	server, objects := newEnterpriseQueuePayloadFakeS3Server(t)
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER", "s3")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT", server.URL)
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_BUCKET", "bucket")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_REGION", "us-east-1")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ACCESS_KEY", "access-key")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_SECRET_KEY", "secret-key")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_KEY_PREFIX", "queue-test-prefix")

	body := []byte("queue payload s3 body")
	row := model.EnterpriseGovernanceQueuePayload{
		AdmissionId:   123,
		RequestId:     "req-queue-payload-s3",
		EnterpriseId:  1,
		UserId:        2,
		TokenId:       3,
		ContentType:   "application/json",
		ContentLength: int64(len(body)),
		BodyBytes:     int64(len(body)),
		SHA256:        enterpriseGovernanceQueueBodySHA256(body),
	}
	object, err := SaveEnterpriseGovernanceQueuePayloadObject(context.Background(), row, body)
	require.NoError(t, err)
	assert.Equal(t, "s3", object.Provider)
	assert.Equal(t, "queue-test-prefix/"+object.Id[:2]+"/"+object.Id, object.StorageKey)
	assert.True(t, objects.exists(object.StorageKey+"/body.bin"))
	assert.True(t, objects.exists(object.StorageKey+"/metadata.json"))

	loaded, loadedBody, err := LoadEnterpriseGovernanceQueuePayloadObject(context.Background(), object.Id)
	require.NoError(t, err)
	assert.Equal(t, object.Id, loaded.Id)
	assert.Equal(t, row.AdmissionId, loaded.AdmissionId)
	assert.Equal(t, body, loadedBody)

	require.NoError(t, DeleteEnterpriseGovernanceQueuePayloadObject(context.Background(), object.Id))
	assert.False(t, objects.exists(object.StorageKey+"/body.bin"))
	assert.False(t, objects.exists(object.StorageKey+"/metadata.json"))
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

func TestEnterpriseGovernanceQueuePersistsLargePayloadAndRestoresBody(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 50*time.Millisecond)
	gin.SetMode(gin.TestMode)

	relayInfo, _ := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-large-payload", 1043, 143)
	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-large-payload", 733)
	requestBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("a", enterpriseQueueRequestPayloadMaxBytes+512) + `"}]}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))

	result, release, err := ApplyEnterpriseGovernanceQueue(ctx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, release)
	defer release()
	assert.Equal(t, enterpriseQueueStatusAdmitted, result.Status)

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-large-payload").First(&admission).Error)

	var payload enterpriseQueueRequestPayload
	require.NoError(t, common.UnmarshalJsonStr(admission.RequestPayloadJson, &payload))
	assert.False(t, payload.BodyTruncated)
	assert.Equal(t, enterpriseQueueRequestBodyStorageDB, payload.BodyStorage)
	assert.Greater(t, payload.PayloadId, int64(0))
	assert.Equal(t, enterpriseQueueRequestPayloadMaxBytes, payload.BodyCapturedBytes)
	assert.Equal(t, requestBody[:enterpriseQueueRequestPayloadMaxBytes], payload.Body)
	assert.EqualValues(t, len(requestBody), payload.ContentLength)
	assert.EqualValues(t, len(requestBody), payload.BodyBytes)
	assert.Equal(t, enterpriseGovernanceQueueBodySHA256([]byte(requestBody)), payload.BodySHA256)

	var durablePayload model.EnterpriseGovernanceQueuePayload
	require.NoError(t, model.DB.Where("id = ?", payload.PayloadId).First(&durablePayload).Error)
	assert.Equal(t, admission.Id, durablePayload.AdmissionId)
	assert.Equal(t, admission.RequestId, durablePayload.RequestId)
	assert.Equal(t, enterpriseQueueRequestBodyStorageDB, durablePayload.StorageKind)
	assert.EqualValues(t, len(requestBody), durablePayload.BodyBytes)
	assert.Equal(t, enterpriseGovernanceQueueBodySHA256([]byte(requestBody)), durablePayload.SHA256)
	assert.Equal(t, requestBody, string(durablePayload.Body))

	restoredBody, err := io.ReadAll(ctx.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, requestBody, string(restoredBody))
}

func TestEnterpriseGovernanceQueuePersistsLargePayloadToObjectStore(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 50*time.Millisecond)
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER", "local")
	t.Setenv("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_DIR", t.TempDir())
	gin.SetMode(gin.TestMode)

	relayInfo, _ := prepareEnterpriseGovernanceQueueRequest(t, "req-enterprise-queue-object-payload", 1046, 146)
	ctx := newEnterpriseGovernanceRelayTestContext(t, "req-enterprise-queue-object-payload", 736)
	requestBody := `{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("o", enterpriseQueueRequestPayloadMaxBytes+512) + `"}]}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	require.Nil(t, PreCheckEnterpriseGovernance(ctx, relayInfo, 20))

	result, release, err := ApplyEnterpriseGovernanceQueue(ctx, relayInfo)
	require.NoError(t, err)
	require.NotNil(t, release)
	defer release()
	assert.Equal(t, enterpriseQueueStatusAdmitted, result.Status)

	var admission model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-object-payload").First(&admission).Error)
	var payload enterpriseQueueRequestPayload
	require.NoError(t, common.UnmarshalJsonStr(admission.RequestPayloadJson, &payload))
	assert.Equal(t, enterpriseQueueRequestBodyStorageObject, payload.BodyStorage)
	assert.Greater(t, payload.PayloadId, int64(0))
	assert.Equal(t, enterpriseGovernanceQueueBodySHA256([]byte(requestBody)), payload.BodySHA256)

	var durablePayload model.EnterpriseGovernanceQueuePayload
	require.NoError(t, model.DB.Where("id = ?", payload.PayloadId).First(&durablePayload).Error)
	assert.Equal(t, model.EnterpriseGovernanceQueuePayloadStorageObject, durablePayload.StorageKind)
	assert.Empty(t, durablePayload.Body)
	assert.NotEmpty(t, durablePayload.ObjectId)
	assert.Equal(t, "local", durablePayload.Provider)
	assert.Equal(t, durablePayload.ObjectId, durablePayload.StorageKey)
	assert.EqualValues(t, len(requestBody), durablePayload.BodyBytes)

	object, body, err := LoadEnterpriseGovernanceQueuePayloadObject(context.Background(), durablePayload.ObjectId)
	require.NoError(t, err)
	assert.Equal(t, durablePayload.ObjectId, object.Id)
	assert.Equal(t, admission.Id, object.AdmissionId)
	assert.Equal(t, []byte(requestBody), body)

	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.Token{
		Id:           146,
		UserId:       1046,
		Key:          "queue-replay-object-token",
		Status:       common.TokenStatusEnabled,
		RemainQuota:  1000,
		ExpiredTime:  -1,
		Name:         "Queue Replay Object Token",
		CreatedTime:  now - 100,
		AccessedTime: now - 100,
	}).Error)
	admission.Status = model.EnterpriseGovernanceQueueAdmissionStatusRetryPending
	admission.RetryCount = 1
	admission.NextRetryAt = now
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ?", admission.Id).
		Updates(map[string]any{
			"status":        admission.Status,
			"retry_count":   admission.RetryCount,
			"next_retry_at": admission.NextRetryAt,
		}).Error)
	request, err := BuildEnterpriseGovernanceQueueReplayRequest(admission)
	require.NoError(t, err)
	assert.Equal(t, enterpriseQueueRequestBodyStorageObject, request.Payload.BodyStorage)
	assert.Equal(t, []byte(requestBody), request.Body)
	assert.Equal(t, "queue-replay-object-token", request.TokenKey)
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

func TestProcessEnterpriseGovernanceQueueReplayBatchReplaysDurableLargePayload(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000250)
	createEnterpriseGovernanceQueueReplayToken(t, 1043, 143, "queue-replay-large-token", now)
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("b", enterpriseQueueRequestPayloadMaxBytes+1024) + `"}]}`)
	admission := createEnterpriseGovernanceQueueReplayAdmissionWithPayload(t, enterprise.Id, 1043, 143, "req-enterprise-queue-replay-large", now, "/v1/chat/completions", "application/json", body)

	var gotRequest EnterpriseGovernanceQueueReplayRequest
	stats, err := processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), now, 10, func(ctx context.Context, request EnterpriseGovernanceQueueReplayRequest) EnterpriseGovernanceQueueReplayResult {
		gotRequest = request
		return EnterpriseGovernanceQueueReplayResult{StatusCode: http.StatusOK, DurationMs: 21}
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.Scanned)
	assert.EqualValues(t, 1, stats.Claimed)
	assert.EqualValues(t, 1, stats.Replayed)
	assert.Zero(t, stats.Failed)
	assert.Equal(t, admission.Id, gotRequest.Admission.Id)
	assert.Equal(t, enterpriseQueueRequestBodyStorageDB, gotRequest.Payload.BodyStorage)
	assert.Equal(t, body, gotRequest.Body)
	assert.Equal(t, "queue-replay-large-token", gotRequest.TokenKey)

	require.NoError(t, model.DB.Where("id = ?", admission.Id).First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusReleased, admission.Status)
	assert.EqualValues(t, 21, admission.RunMs)
	assert.Empty(t, admission.LastError)
}

func TestBuildEnterpriseGovernanceQueueReplayRequestAcceptsDurableMultipartPayload(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000260)
	createEnterpriseGovernanceQueueReplayToken(t, 1044, 144, "queue-replay-multipart-token", now)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "whisper-1"))
	part, err := writer.CreateFormFile("file", "sample.wav")
	require.NoError(t, err)
	_, err = part.Write([]byte(strings.Repeat("wave", 256)))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	admission := createEnterpriseGovernanceQueueReplayAdmissionWithPayload(t, enterprise.Id, 1044, 144, "req-enterprise-queue-replay-multipart", now, "/v1/audio/transcriptions", writer.FormDataContentType(), body.Bytes())
	request, err := BuildEnterpriseGovernanceQueueReplayRequest(admission)
	require.NoError(t, err)
	assert.Equal(t, "/v1/audio/transcriptions", request.Path)
	assert.Equal(t, writer.FormDataContentType(), request.ContentType)
	assert.Equal(t, body.Bytes(), request.Body)
	assert.Equal(t, "queue-replay-multipart-token", request.TokenKey)
}

func TestProcessEnterpriseGovernanceQueueReplayBatchFailsMissingDurablePayload(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	resetEnterpriseGovernanceQueueForTest(t, 1, 30*time.Second)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1700000270)
	payload := EnterpriseGovernanceQueueRequestPayload{
		Method:                http.MethodPost,
		Path:                  "/v1/chat/completions",
		ContentType:           "application/json",
		BodyCapturedBytes:     enterpriseQueueRequestPayloadMaxBytes,
		BodyCaptureLimitBytes: enterpriseQueueRequestPayloadMaxBytes,
		BodyStorage:           enterpriseQueueRequestBodyStorageDB,
		PayloadId:             999999,
		BodySHA256:            enterpriseGovernanceQueueBodySHA256([]byte("missing")),
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseGovernanceQueueAdmission{
		RequestId:          "req-enterprise-queue-replay-missing-payload",
		EnterpriseId:       enterprise.Id,
		UserId:             1045,
		TokenId:            145,
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
	require.NoError(t, model.DB.Where("request_id = ?", "req-enterprise-queue-replay-missing-payload").First(&admission).Error)
	assert.Equal(t, enterpriseQueueStatusTimeout, admission.Status)
	assert.Contains(t, admission.LastError, ErrEnterpriseGovernanceQueueReplayPayloadMissing.Error())
	assert.Contains(t, admission.LastError, "durable payload 999999")

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("request_id = ? AND action = ?", "req-enterprise-queue-replay-missing-payload", enterpriseGovernanceAuditActionQueueReplay).First(&audit).Error)
	assert.Contains(t, audit.AfterJson, ErrEnterpriseGovernanceQueueReplayPayloadMissing.Error())
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

func createEnterpriseGovernanceQueueReplayToken(t *testing.T, userId int, tokenId int, key string, now int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: key + "-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:           tokenId,
		UserId:       userId,
		Key:          key,
		Status:       common.TokenStatusEnabled,
		RemainQuota:  1000,
		ExpiredTime:  -1,
		Name:         key,
		CreatedTime:  now - 100,
		AccessedTime: now - 100,
	}).Error)
}

func createEnterpriseGovernanceQueueReplayAdmissionWithPayload(t *testing.T, enterpriseId int, userId int, tokenId int, requestId string, now int64, path string, contentType string, body []byte) model.EnterpriseGovernanceQueueAdmission {
	t.Helper()
	admission := model.EnterpriseGovernanceQueueAdmission{
		RequestId:      requestId,
		EnterpriseId:   enterpriseId,
		UserId:         userId,
		TokenId:        tokenId,
		QueueKey:       "enterprise:1",
		Status:         model.EnterpriseGovernanceQueueAdmissionStatusRetryPending,
		RetryCount:     1,
		NextRetryAt:    now,
		UserMessageKey: "enterprise_governance.queue_retry_pending",
		CreatedAt:      now - 60,
		UpdatedAt:      now - 60,
	}
	require.NoError(t, model.DB.Create(&admission).Error)

	bodySnapshot := body
	if len(bodySnapshot) > enterpriseQueueRequestPayloadMaxBytes {
		bodySnapshot = bodySnapshot[:enterpriseQueueRequestPayloadMaxBytes]
	}
	bodyText := ""
	if isEnterpriseGovernanceQueueTextPreviewContentType(contentType) {
		bodyText = string(bodySnapshot)
	}
	bodySHA256 := enterpriseGovernanceQueueBodySHA256(body)
	payloadRow := model.EnterpriseGovernanceQueuePayload{
		AdmissionId:   admission.Id,
		RequestId:     admission.RequestId,
		EnterpriseId:  enterpriseId,
		UserId:        userId,
		TokenId:       tokenId,
		ContentType:   contentType,
		ContentLength: int64(len(body)),
		Body:          append([]byte(nil), body...),
		BodyBytes:     int64(len(body)),
		SHA256:        bodySHA256,
		StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
	}
	require.NoError(t, model.DB.Create(&payloadRow).Error)

	payload := EnterpriseGovernanceQueueRequestPayload{
		Method:                http.MethodPost,
		Path:                  path,
		ContentType:           contentType,
		ContentLength:         int64(len(body)),
		Body:                  bodyText,
		BodyBytes:             int64(len(body)),
		BodyCapturedBytes:     len(bodySnapshot),
		BodyCaptureLimitBytes: enterpriseQueueRequestPayloadMaxBytes,
		BodyStorage:           enterpriseQueueRequestBodyStorageDB,
		PayloadId:             payloadRow.Id,
		BodySHA256:            bodySHA256,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ?", admission.Id).
		Update("request_payload_json", string(payloadJson)).Error)
	admission.RequestPayloadJson = string(payloadJson)
	return admission
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

type enterpriseQueuePayloadFakeS3Objects struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newEnterpriseQueuePayloadFakeS3Server(t *testing.T) (*httptest.Server, *enterpriseQueuePayloadFakeS3Objects) {
	t.Helper()
	state := &enterpriseQueuePayloadFakeS3Objects{objects: map[string][]byte{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.Header.Get("Authorization"))
		bucket, key, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if !ok {
			bucket = strings.TrimPrefix(r.URL.Path, "/")
		}
		require.Equal(t, "bucket", bucket)
		if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
			state.writeList(w, r.URL.Query().Get("prefix"))
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			state.set(key, body)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			body, ok := state.get(key)
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		case http.MethodDelete:
			state.delete(key)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	return server, state
}

func (f *enterpriseQueuePayloadFakeS3Objects) set(key string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = append([]byte(nil), content...)
}

func (f *enterpriseQueuePayloadFakeS3Objects) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.objects[key]
	return append([]byte(nil), content...), ok
}

func (f *enterpriseQueuePayloadFakeS3Objects) delete(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
}

func (f *enterpriseQueuePayloadFakeS3Objects) exists(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *enterpriseQueuePayloadFakeS3Objects) writeList(w http.ResponseWriter, prefix string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.objects))
	for key := range f.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	for _, key := range keys {
		_, _ = fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>%d</Size></Contents>", key, len(f.objects[key]))
	}
	_, _ = fmt.Fprint(w, `<IsTruncated>false</IsTruncated></ListBucketResult>`)
}
