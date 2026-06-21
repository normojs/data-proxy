package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const (
	enterpriseGovernanceAuditActionQueueAdmission = "enterprise_governance.queue_admission"
	enterpriseGovernanceAuditActionQueueRecovery  = "enterprise_governance.queue_admission.recover"
	enterpriseQueueStatusQueued                   = model.EnterpriseGovernanceQueueAdmissionStatusQueued
	enterpriseQueueStatusAdmitted                 = model.EnterpriseGovernanceQueueAdmissionStatusAdmitted
	enterpriseQueueStatusReleased                 = model.EnterpriseGovernanceQueueAdmissionStatusReleased
	enterpriseQueueStatusTimeout                  = model.EnterpriseGovernanceQueueAdmissionStatusTimeout
	enterpriseQueueStatusCanceled                 = model.EnterpriseGovernanceQueueAdmissionStatusCanceled
	enterpriseQueueStatusRetryPending             = model.EnterpriseGovernanceQueueAdmissionStatusRetryPending
	enterpriseQueueStatusReplayProcessing         = model.EnterpriseGovernanceQueueAdmissionStatusReplayProcessing
	enterpriseQueueStatusHeader                   = "X-Data-Proxy-Enterprise-Queue-Status"
	enterpriseQueueWaitMsHeader                   = "X-Data-Proxy-Enterprise-Queue-Wait-Ms"
	enterpriseQueueTimeoutMsHeader                = "X-Data-Proxy-Enterprise-Queue-Timeout-Ms"
	enterpriseQueueRequestPayloadMaxBytes         = 32 * 1024
)

var (
	enterprisePolicyQueueMaxConcurrent = 1
	enterprisePolicyQueueTimeout       = 30 * time.Second
	enterprisePolicyQueueAdmittedStale = time.Hour
	enterprisePolicyQueues             sync.Map
	enterprisePolicyQueueCancelers     sync.Map
)

var ErrEnterpriseGovernanceQueueTimeout = errors.New("enterprise governance queue timeout")
var ErrEnterpriseGovernanceQueueCanceled = errors.New("enterprise governance queue canceled")
var ErrEnterpriseGovernanceQueueAdmissionNotCancelable = errors.New("enterprise governance queue admission is not queued on this node")
var ErrEnterpriseGovernanceQueueAdmissionNotRetryable = errors.New("enterprise governance queue admission is not retryable")

type EnterpriseGovernanceQueueResult struct {
	Applied   bool
	Status    string
	WaitMs    int64
	TimeoutMs int64
}

type enterprisePolicyQueue struct {
	slots chan struct{}
}

type EnterpriseGovernanceQueueRequestPayload struct {
	Method                string `json:"method"`
	Path                  string `json:"path"`
	RawQuery              string `json:"raw_query,omitempty"`
	ContentType           string `json:"content_type,omitempty"`
	ContentLength         int64  `json:"content_length"`
	Body                  string `json:"body,omitempty"`
	BodyCapturedBytes     int    `json:"body_captured_bytes"`
	BodyCaptureLimitBytes int    `json:"body_capture_limit_bytes"`
	BodyTruncated         bool   `json:"body_truncated"`
	Model                 string `json:"model,omitempty"`
	RelayMode             int    `json:"relay_mode"`
	ChannelId             int    `json:"channel_id"`
}

type enterpriseQueueRequestPayload = EnterpriseGovernanceQueueRequestPayload

type enterpriseQueueRequestBodyRestore struct {
	reader io.Reader
	closer io.Closer
}

func (body *enterpriseQueueRequestBodyRestore) Read(p []byte) (int, error) {
	return body.reader.Read(p)
}

func (body *enterpriseQueueRequestBodyRestore) Close() error {
	if body == nil || body.closer == nil {
		return nil
	}
	return body.closer.Close()
}

func ApplyEnterpriseGovernanceQueue(c *gin.Context, relayInfo *relaycommon.RelayInfo) (EnterpriseGovernanceQueueResult, func(), error) {
	result := EnterpriseGovernanceQueueResult{}
	if !common.EnterpriseGovernanceEnabled || c == nil || relayInfo == nil {
		return result, nil, nil
	}
	decision, ok := common.GetContextKeyType[PolicyDecision](c, constant.ContextKeyEnterpriseGovernanceDecision)
	if !ok || !hasEnterprisePolicyQueueAction(decision.ActionObservations) {
		return result, nil, nil
	}
	enterpriseCtx, ok := common.GetContextKeyType[*EnterpriseContext](c, constant.ContextKeyEnterpriseGovernanceContext)
	if !ok || enterpriseCtx == nil {
		var err error
		enterpriseCtx, err = resolveEnterpriseContextFromRelay(c, relayInfo)
		if err != nil {
			return result, nil, err
		}
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled {
		return result, nil, nil
	}

	result.Applied = true
	result.TimeoutMs = durationMillis(enterprisePolicyQueueTimeout)
	result.Status = enterpriseQueueStatusQueued
	queueKey := enterprisePolicyQueueKey(enterpriseCtx, relayInfo)
	queue := getEnterprisePolicyQueue(queueKey)
	start := time.Now()
	admission := createEnterpriseGovernanceQueueAdmission(c, enterpriseCtx, relayInfo, decision, result, queueKey)
	cancelRegistration := registerEnterpriseGovernanceQueueCancellation(admission)
	if cancelRegistration != nil {
		defer cancelRegistration.Unregister()
	}
	var queueCanceled <-chan struct{}
	if cancelRegistration != nil {
		queueCanceled = cancelRegistration.Done()
	}
	timer := time.NewTimer(enterprisePolicyQueueTimeout)
	defer timer.Stop()
	var requestDone <-chan struct{}
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	select {
	case queue.slots <- struct{}{}:
		admittedAt := time.Now()
		result.Status = enterpriseQueueStatusAdmitted
		result.WaitMs = durationMillis(admittedAt.Sub(start))
		setEnterpriseQueueHeaders(c, result)
		updateEnterpriseGovernanceQueueAdmission(c, admission, result, map[string]any{
			"admitted_at": admittedAt.Unix(),
		})
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		var once sync.Once
		release := func() {
			once.Do(func() {
				<-queue.slots
				releasedAt := time.Now()
				finalResult := result
				finalResult.Status = enterpriseQueueStatusReleased
				updates := map[string]any{
					"released_at": releasedAt.Unix(),
					"run_ms":      durationMillis(releasedAt.Sub(admittedAt)),
				}
				if c.Request != nil && c.Request.Context().Err() != nil {
					finalResult.Status = enterpriseQueueStatusCanceled
					updates["canceled_at"] = releasedAt.Unix()
					updates["last_error"] = c.Request.Context().Err().Error()
				}
				updateEnterpriseGovernanceQueueAdmission(c, admission, finalResult, updates)
			})
		}
		return result, release, nil
	case <-timer.C:
		now := time.Now()
		result.Status = enterpriseQueueStatusTimeout
		result.WaitMs = durationMillis(now.Sub(start))
		setEnterpriseQueueHeaders(c, result)
		updateEnterpriseGovernanceQueueAdmission(c, admission, result, map[string]any{
			"last_error": ErrEnterpriseGovernanceQueueTimeout.Error(),
		})
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance queue timeout after %dms", result.WaitMs))
		return result, nil, ErrEnterpriseGovernanceQueueTimeout
	case <-requestDone:
		now := time.Now()
		result.Status = enterpriseQueueStatusCanceled
		result.WaitMs = durationMillis(now.Sub(start))
		setEnterpriseQueueHeaders(c, result)
		err := context.Canceled
		if c.Request != nil && c.Request.Context().Err() != nil {
			err = c.Request.Context().Err()
		}
		updateEnterpriseGovernanceQueueAdmission(c, admission, result, map[string]any{
			"canceled_at": now.Unix(),
			"last_error":  err.Error(),
		})
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		return result, nil, err
	case <-queueCanceled:
		now := time.Now()
		result.Status = enterpriseQueueStatusCanceled
		result.WaitMs = durationMillis(now.Sub(start))
		setEnterpriseQueueHeaders(c, result)
		updateEnterpriseGovernanceQueueAdmission(c, admission, result, map[string]any{
			"canceled_at": now.Unix(),
			"last_error":  ErrEnterpriseGovernanceQueueCanceled.Error(),
		})
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		return result, nil, ErrEnterpriseGovernanceQueueCanceled
	}
}

func hasEnterprisePolicyQueueAction(observations []PolicyActionObservation) bool {
	for _, observation := range observations {
		if observation.Action == model.PolicyActionQueue {
			return true
		}
	}
	return false
}

func getEnterprisePolicyQueue(key string) *enterprisePolicyQueue {
	if queue, ok := enterprisePolicyQueues.Load(key); ok {
		return queue.(*enterprisePolicyQueue)
	}
	maxConcurrent := enterprisePolicyQueueMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	queue := &enterprisePolicyQueue{
		slots: make(chan struct{}, maxConcurrent),
	}
	actual, _ := enterprisePolicyQueues.LoadOrStore(key, queue)
	return actual.(*enterprisePolicyQueue)
}

func enterprisePolicyQueueKey(enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo) string {
	if enterpriseCtx != nil {
		if enterpriseCtx.EnterpriseId > 0 {
			return fmt.Sprintf("enterprise:%d", enterpriseCtx.EnterpriseId)
		}
		if enterpriseCtx.TokenId > 0 {
			return fmt.Sprintf("token:%d", enterpriseCtx.TokenId)
		}
		if enterpriseCtx.UserId > 0 {
			return fmt.Sprintf("user:%d", enterpriseCtx.UserId)
		}
	}
	if relayInfo != nil {
		if relayInfo.TokenId > 0 {
			return fmt.Sprintf("token:%d", relayInfo.TokenId)
		}
		if relayInfo.UserId > 0 {
			return fmt.Sprintf("user:%d", relayInfo.UserId)
		}
	}
	return "global"
}

func setEnterpriseQueueHeaders(c *gin.Context, result EnterpriseGovernanceQueueResult) {
	if c == nil || !result.Applied {
		return
	}
	c.Header(enterpriseQueueStatusHeader, result.Status)
	c.Header(enterpriseQueueWaitMsHeader, strconv.FormatInt(result.WaitMs, 10))
	c.Header(enterpriseQueueTimeoutMsHeader, strconv.FormatInt(result.TimeoutMs, 10))
}

func recordEnterpriseGovernanceQueueAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceQueueResult) {
	if enterpriseCtx == nil || !result.Applied {
		return
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ActorUserId:    enterpriseCtx.UserId,
		Action:         enterpriseGovernanceAuditActionQueueAdmission,
		TargetType:     "quota_policy",
		TargetId:       firstEnterpriseQueuePolicyActionObservationId(decision.ActionObservations),
		ScopeUserId:    enterpriseCtx.UserId,
		ScopeOrgUnitId: enterpriseCtx.PrimaryOrgUnitId,
		ScopeProjectId: enterpriseCtx.ProjectId,
		After:          enterpriseGovernanceQueueAuditPayload(c, enterpriseCtx, relayInfo, decision, result, requestId),
		RequestId:      requestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance queue audit: "+err.Error())
	}
}

func createEnterpriseGovernanceQueueAdmission(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceQueueResult, queueKey string) *model.EnterpriseGovernanceQueueAdmission {
	if enterpriseCtx == nil || !result.Applied {
		return nil
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	policyIdsJson, err := common.Marshal(cloneIntSlice(decision.MatchedPolicyIds))
	if err != nil {
		logger.LogError(c, "error marshaling enterprise governance queue policy ids: "+err.Error())
		return nil
	}
	policyGroupIdsJson, err := common.Marshal(cloneIntSlice(enterpriseCtx.PolicyGroupIds))
	if err != nil {
		logger.LogError(c, "error marshaling enterprise governance queue policy group ids: "+err.Error())
		return nil
	}
	policyActionsJson, err := common.Marshal(cloneEnterprisePolicyActionObservations(decision.ActionObservations))
	if err != nil {
		logger.LogError(c, "error marshaling enterprise governance queue policy actions: "+err.Error())
		return nil
	}
	requestPayloadJson, err := enterpriseGovernanceQueueRequestPayloadJson(c, relayInfo)
	if err != nil {
		logger.LogWarn(c, "error capturing enterprise governance queue request payload: "+err.Error())
	}
	modelName := ""
	relayMode := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		relayMode = relayInfo.RelayMode
	}
	row := model.EnterpriseGovernanceQueueAdmission{
		RequestId:          requestId,
		EnterpriseId:       enterpriseCtx.EnterpriseId,
		UserId:             enterpriseCtx.UserId,
		TokenId:            enterpriseCtx.TokenId,
		OrgUnitId:          enterpriseCtx.PrimaryOrgUnitId,
		ProjectId:          enterpriseCtx.ProjectId,
		PolicyId:           firstEnterpriseQueuePolicyActionObservationId(decision.ActionObservations),
		PolicyIdsJson:      string(policyIdsJson),
		PolicyGroupIdsJson: string(policyGroupIdsJson),
		ModelName:          modelName,
		ChannelId:          enterpriseChannelIdFromRelay(c, relayInfo),
		RelayMode:          relayMode,
		QueueKey:           queueKey,
		Status:             result.Status,
		WaitMs:             result.WaitMs,
		TimeoutMs:          result.TimeoutMs,
		DryRun:             decision.DryRun,
		PolicyActionsJson:  string(policyActionsJson),
		RequestPayloadJson: requestPayloadJson,
		UserMessageKey:     enterpriseQueueUserMessageKey(result.Status),
	}
	if err := model.DB.Create(&row).Error; err != nil {
		logger.LogError(c, "error recording enterprise governance queue admission: "+err.Error())
		return nil
	}
	return &row
}

func enterpriseGovernanceQueueRequestPayloadJson(c *gin.Context, relayInfo *relaycommon.RelayInfo) (string, error) {
	if c == nil || c.Request == nil {
		return "", nil
	}
	request := c.Request
	path := ""
	rawQuery := ""
	if request.URL != nil {
		path = request.URL.Path
		rawQuery = request.URL.RawQuery
	}
	modelName := ""
	relayMode := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		relayMode = relayInfo.RelayMode
		if path == "" {
			path = relayInfo.RequestURLPath
		}
	}
	body, truncated, err := captureEnterpriseGovernanceQueueRequestBody(request)
	if err != nil {
		return "", err
	}
	payload := enterpriseQueueRequestPayload{
		Method:                request.Method,
		Path:                  path,
		RawQuery:              rawQuery,
		ContentType:           request.Header.Get("Content-Type"),
		ContentLength:         request.ContentLength,
		Body:                  string(body),
		BodyCapturedBytes:     len(body),
		BodyCaptureLimitBytes: enterpriseQueueRequestPayloadMaxBytes,
		BodyTruncated:         truncated,
		Model:                 modelName,
		RelayMode:             relayMode,
		ChannelId:             enterpriseChannelIdFromRelay(c, relayInfo),
	}
	payloadJson, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(payloadJson), nil
}

func captureEnterpriseGovernanceQueueRequestBody(request *http.Request) ([]byte, bool, error) {
	if request == nil || request.Body == nil {
		return nil, false, nil
	}
	originalBody := request.Body
	capturedWithMarker, err := io.ReadAll(io.LimitReader(originalBody, enterpriseQueueRequestPayloadMaxBytes+1))
	request.Body = &enterpriseQueueRequestBodyRestore{
		reader: io.MultiReader(bytes.NewReader(capturedWithMarker), originalBody),
		closer: originalBody,
	}
	if err != nil {
		return nil, false, err
	}
	if len(capturedWithMarker) > enterpriseQueueRequestPayloadMaxBytes {
		return capturedWithMarker[:enterpriseQueueRequestPayloadMaxBytes], true, nil
	}
	return capturedWithMarker, false, nil
}

func updateEnterpriseGovernanceQueueAdmission(c *gin.Context, admission *model.EnterpriseGovernanceQueueAdmission, result EnterpriseGovernanceQueueResult, updates map[string]any) {
	if admission == nil || admission.Id <= 0 || !result.Applied {
		return
	}
	now := time.Now().Unix()
	values := map[string]any{
		"status":           result.Status,
		"wait_ms":          result.WaitMs,
		"timeout_ms":       result.TimeoutMs,
		"user_message_key": enterpriseQueueUserMessageKey(result.Status),
		"updated_at":       now,
	}
	for key, value := range updates {
		values[key] = value
	}
	if err := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ?", admission.Id).
		Updates(values).Error; err != nil {
		logger.LogError(c, "error updating enterprise governance queue admission: "+err.Error())
	}
}

func enterpriseQueueUserMessageKey(status string) string {
	if status == enterpriseQueueStatusRetryPending {
		return "enterprise_governance.queue_retry_pending"
	}
	if status == enterpriseQueueStatusReplayProcessing {
		return "enterprise_governance.queue_replay_processing"
	}
	if status == enterpriseQueueStatusTimeout {
		return "enterprise_governance.queue_timeout"
	}
	if status == enterpriseQueueStatusCanceled {
		return "enterprise_governance.queue_canceled"
	}
	return "enterprise_governance.policy_action_observed"
}

func isEnterpriseGovernanceQueueAdmissionRetryable(status string) bool {
	switch status {
	case enterpriseQueueStatusTimeout, enterpriseQueueStatusCanceled:
		return true
	default:
		return false
	}
}

func enterpriseGovernanceQueueRetryableStatuses() []string {
	return []string{enterpriseQueueStatusTimeout, enterpriseQueueStatusCanceled}
}

type enterpriseQueueCancelRegistration struct {
	id   int64
	done chan struct{}
	once sync.Once
}

func registerEnterpriseGovernanceQueueCancellation(admission *model.EnterpriseGovernanceQueueAdmission) *enterpriseQueueCancelRegistration {
	if admission == nil || admission.Id <= 0 {
		return nil
	}
	registration := &enterpriseQueueCancelRegistration{
		id:   admission.Id,
		done: make(chan struct{}),
	}
	enterprisePolicyQueueCancelers.Store(admission.Id, registration)
	return registration
}

func (registration *enterpriseQueueCancelRegistration) Done() <-chan struct{} {
	if registration == nil {
		return nil
	}
	return registration.done
}

func (registration *enterpriseQueueCancelRegistration) Cancel() {
	if registration == nil {
		return
	}
	registration.once.Do(func() {
		close(registration.done)
	})
}

func (registration *enterpriseQueueCancelRegistration) Unregister() {
	if registration == nil {
		return
	}
	enterprisePolicyQueueCancelers.Delete(registration.id)
}

func CancelEnterpriseGovernanceQueuedAdmission(admission model.EnterpriseGovernanceQueueAdmission) (model.EnterpriseGovernanceQueueAdmission, error) {
	if admission.Id <= 0 || admission.Status != enterpriseQueueStatusQueued {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotCancelable
	}
	value, ok := enterprisePolicyQueueCancelers.Load(admission.Id)
	if !ok {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotCancelable
	}
	registration, ok := value.(*enterpriseQueueCancelRegistration)
	if !ok || registration == nil {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotCancelable
	}
	now := time.Now().Unix()
	result := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ? AND status = ?", admission.Id, enterpriseQueueStatusQueued).
		Updates(map[string]any{
			"status":           enterpriseQueueStatusCanceled,
			"canceled_at":      now,
			"last_error":       ErrEnterpriseGovernanceQueueCanceled.Error(),
			"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusCanceled),
			"updated_at":       now,
		})
	if result.Error != nil {
		return admission, result.Error
	}
	if result.RowsAffected == 0 {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotCancelable
	}
	registration.Cancel()
	var after model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.Where("id = ?", admission.Id).First(&after).Error; err != nil {
		return admission, err
	}
	return after, nil
}

func RetryEnterpriseGovernanceQueueAdmission(admission model.EnterpriseGovernanceQueueAdmission, now int64) (model.EnterpriseGovernanceQueueAdmission, error) {
	if admission.Id <= 0 || !isEnterpriseGovernanceQueueAdmissionRetryable(admission.Status) {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotRetryable
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	result := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ? AND status IN ?", admission.Id, enterpriseGovernanceQueueRetryableStatuses()).
		Updates(map[string]any{
			"status":           enterpriseQueueStatusRetryPending,
			"retry_count":      admission.RetryCount + 1,
			"next_retry_at":    now,
			"last_error":       "",
			"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusRetryPending),
			"updated_at":       now,
		})
	if result.Error != nil {
		return admission, result.Error
	}
	if result.RowsAffected == 0 {
		return admission, ErrEnterpriseGovernanceQueueAdmissionNotRetryable
	}
	var after model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.Where("id = ?", admission.Id).First(&after).Error; err != nil {
		return admission, err
	}
	return after, nil
}

func enterpriseGovernanceQueueAuditPayload(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceQueueResult, requestId string) map[string]any {
	modelName := ""
	channelId := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		channelId = enterpriseChannelIdFromRelay(c, relayInfo)
	}
	return map[string]any{
		"request_id":         requestId,
		"model":              modelName,
		"channel_id":         channelId,
		"token_id":           enterpriseCtx.TokenId,
		"org_unit_id":        enterpriseCtx.PrimaryOrgUnitId,
		"project_id":         enterpriseCtx.ProjectId,
		"policy_group_ids":   cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"matched_policy_ids": cloneIntSlice(decision.MatchedPolicyIds),
		"counter_policy_ids": cloneIntSlice(decision.CounterPolicyIds),
		"policy_actions":     cloneEnterprisePolicyActionObservations(decision.ActionObservations),
		"queue_status":       result.Status,
		"wait_ms":            result.WaitMs,
		"timeout_ms":         result.TimeoutMs,
		"user_message_key":   enterpriseQueueUserMessageKey(result.Status),
		"dry_run":            decision.DryRun,
	}
}

func firstEnterpriseQueuePolicyActionObservationId(observations []PolicyActionObservation) int {
	for _, observation := range observations {
		if observation.Action == model.PolicyActionQueue {
			return observation.PolicyId
		}
	}
	return firstEnterprisePolicyActionObservationId(observations)
}

func durationMillis(duration time.Duration) int64 {
	return int64(duration / time.Millisecond)
}
