package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
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
	enterpriseQueueReplayPayloadMaxBytes          = 32 * 1024 * 1024
	enterpriseQueueRequestBodyStorageInline       = "inline"
	enterpriseQueueRequestBodyStorageDB           = model.EnterpriseGovernanceQueuePayloadStorageDB
	enterpriseQueueRequestBodyStorageObject       = model.EnterpriseGovernanceQueuePayloadStorageObject
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
	BodyBytes             int64  `json:"body_bytes,omitempty"`
	BodyCapturedBytes     int    `json:"body_captured_bytes"`
	BodyCaptureLimitBytes int    `json:"body_capture_limit_bytes"`
	BodyTruncated         bool   `json:"body_truncated"`
	BodyStorage           string `json:"body_storage,omitempty"`
	PayloadId             int64  `json:"payload_id,omitempty"`
	BodySHA256            string `json:"body_sha256,omitempty"`
	Model                 string `json:"model,omitempty"`
	RelayMode             int    `json:"relay_mode"`
	ChannelId             int    `json:"channel_id"`
}

type enterpriseQueueRequestPayload = EnterpriseGovernanceQueueRequestPayload

type enterpriseQueueRequestBodyRestore struct {
	reader io.Reader
	closer io.Closer
}

type enterpriseQueueCapturedRequestPayload struct {
	Payload       enterpriseQueueRequestPayload
	Body          []byte
	ShouldPersist bool
}

type enterpriseQueueCapturedBody struct {
	Body              []byte
	FullBodyAvailable bool
	Truncated         bool
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
	capturedPayload, err := captureEnterpriseGovernanceQueueRequestPayload(c, relayInfo)
	if err != nil {
		logger.LogWarn(c, "error capturing enterprise governance queue request payload: "+err.Error())
	}
	requestPayloadJson := ""
	if err == nil {
		requestPayloadJson, err = enterpriseGovernanceQueueRequestPayloadJSON(capturedPayload.Payload)
		if err != nil {
			logger.LogWarn(c, "error marshaling enterprise governance queue request payload: "+err.Error())
		}
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
		Priority:           enterpriseGovernanceQueueAdmissionPriority(decision),
		UserMessageKey:     enterpriseQueueUserMessageKey(result.Status),
	}
	if err := model.DB.Create(&row).Error; err != nil {
		logger.LogError(c, "error recording enterprise governance queue admission: "+err.Error())
		return nil
	}
	if capturedPayload.ShouldPersist {
		persistEnterpriseGovernanceQueueRequestPayload(c, &row, capturedPayload)
	}
	return &row
}

func enterpriseGovernanceQueueRequestPayloadJson(c *gin.Context, relayInfo *relaycommon.RelayInfo) (string, error) {
	captured, err := captureEnterpriseGovernanceQueueRequestPayload(c, relayInfo)
	if err != nil {
		return "", err
	}
	return enterpriseGovernanceQueueRequestPayloadJSON(captured.Payload)
}

func enterpriseGovernanceQueueRequestPayloadJSON(payload enterpriseQueueRequestPayload) (string, error) {
	payloadJson, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(payloadJson), nil
}

func captureEnterpriseGovernanceQueueRequestPayload(c *gin.Context, relayInfo *relaycommon.RelayInfo) (enterpriseQueueCapturedRequestPayload, error) {
	captured := enterpriseQueueCapturedRequestPayload{}
	if c == nil || c.Request == nil {
		return captured, nil
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
	bodyCapture, err := captureEnterpriseGovernanceQueueRequestBody(c, request)
	if err != nil {
		return captured, err
	}
	body := bodyCapture.Body
	bodyBytes := int64(len(body))
	if request.ContentLength > bodyBytes {
		bodyBytes = request.ContentLength
	}
	bodySnapshot := body
	if len(bodySnapshot) > enterpriseQueueRequestPayloadMaxBytes {
		bodySnapshot = bodySnapshot[:enterpriseQueueRequestPayloadMaxBytes]
	}
	contentType := request.Header.Get("Content-Type")
	shouldPersist := bodyCapture.FullBodyAvailable &&
		len(body) > 0 &&
		(len(body) > enterpriseQueueRequestPayloadMaxBytes || isEnterpriseGovernanceQueueMultipartContentType(contentType))
	bodyText := ""
	if !shouldPersist || isEnterpriseGovernanceQueueTextPreviewContentType(contentType) {
		bodyText = string(bodySnapshot)
	}
	bodyStorage := enterpriseQueueRequestBodyStorageInline
	if shouldPersist {
		bodyStorage = enterpriseQueueRequestBodyStorageDB
	}
	bodySHA256 := ""
	if bodyCapture.FullBodyAvailable {
		bodySHA256 = enterpriseGovernanceQueueBodySHA256(body)
	}
	payload := enterpriseQueueRequestPayload{
		Method:                request.Method,
		Path:                  path,
		RawQuery:              rawQuery,
		ContentType:           contentType,
		ContentLength:         request.ContentLength,
		Body:                  bodyText,
		BodyBytes:             bodyBytes,
		BodyCapturedBytes:     len(bodySnapshot),
		BodyCaptureLimitBytes: enterpriseQueueRequestPayloadMaxBytes,
		BodyTruncated:         bodyCapture.Truncated,
		BodyStorage:           bodyStorage,
		BodySHA256:            bodySHA256,
		Model:                 modelName,
		RelayMode:             relayMode,
		ChannelId:             enterpriseChannelIdFromRelay(c, relayInfo),
	}
	captured.Payload = payload
	captured.Body = body
	captured.ShouldPersist = shouldPersist
	return captured, nil
}

func captureEnterpriseGovernanceQueueRequestBody(c *gin.Context, request *http.Request) (enterpriseQueueCapturedBody, error) {
	result := enterpriseQueueCapturedBody{FullBodyAvailable: true}
	if request == nil || request.Body == nil {
		return result, nil
	}
	if c != nil {
		if storageValue, exists := c.Get(common.KeyBodyStorage); exists && storageValue != nil {
			if storage, ok := storageValue.(common.BodyStorage); ok {
				return captureEnterpriseGovernanceQueueRequestBodyFromStorage(request, storage)
			}
		}
	}
	originalBody := request.Body
	capturedWithMarker, err := io.ReadAll(io.LimitReader(originalBody, enterpriseQueueReplayPayloadMaxBytes+1))
	request.Body = &enterpriseQueueRequestBodyRestore{
		reader: io.MultiReader(bytes.NewReader(capturedWithMarker), originalBody),
		closer: originalBody,
	}
	if err != nil {
		return result, err
	}
	if len(capturedWithMarker) > enterpriseQueueReplayPayloadMaxBytes {
		result.FullBodyAvailable = false
		result.Truncated = true
		result.Body = capturedWithMarker[:enterpriseQueueRequestPayloadMaxBytes]
		return result, nil
	}
	result.Body = capturedWithMarker
	return result, nil
}

func captureEnterpriseGovernanceQueueRequestBodyFromStorage(request *http.Request, storage common.BodyStorage) (enterpriseQueueCapturedBody, error) {
	result := enterpriseQueueCapturedBody{FullBodyAvailable: true}
	if storage.Size() > enterpriseQueueReplayPayloadMaxBytes {
		result.FullBodyAvailable = false
		result.Truncated = true
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return result, err
		}
		body, err := io.ReadAll(io.LimitReader(storage, enterpriseQueueRequestPayloadMaxBytes))
		if err != nil {
			return result, err
		}
		result.Body = body
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return result, err
		}
		request.Body = io.NopCloser(storage)
		return result, nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return result, err
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	request.Body = io.NopCloser(storage)
	result.Body = body
	return result, nil
}

func persistEnterpriseGovernanceQueueRequestPayload(c *gin.Context, admission *model.EnterpriseGovernanceQueueAdmission, captured enterpriseQueueCapturedRequestPayload) {
	if admission == nil || admission.Id <= 0 || !captured.ShouldPersist {
		return
	}
	body := append([]byte(nil), captured.Body...)
	payload := captured.Payload
	if payload.BodySHA256 == "" {
		payload.BodySHA256 = enterpriseGovernanceQueueBodySHA256(body)
	}
	row := model.EnterpriseGovernanceQueuePayload{
		AdmissionId:   admission.Id,
		RequestId:     admission.RequestId,
		EnterpriseId:  admission.EnterpriseId,
		UserId:        admission.UserId,
		TokenId:       admission.TokenId,
		ContentType:   payload.ContentType,
		ContentLength: payload.ContentLength,
		Body:          body,
		BodyBytes:     int64(len(body)),
		SHA256:        payload.BodySHA256,
		StorageKind:   model.EnterpriseGovernanceQueuePayloadStorageDB,
	}
	if EnterpriseGovernanceQueuePayloadObjectStorageEnabled() {
		object, err := SaveEnterpriseGovernanceQueuePayloadObject(context.Background(), row, body)
		if err != nil {
			logger.LogWarn(c, "error storing enterprise governance queue request payload object, falling back to db: "+err.Error())
		} else {
			row.Body = []byte{}
			row.StorageKind = model.EnterpriseGovernanceQueuePayloadStorageObject
			row.ObjectId = object.Id
			row.Provider = object.Provider
			row.StorageKey = object.StorageKey
		}
	}
	if err := model.DB.Create(&row).Error; err != nil {
		if row.StorageKind == model.EnterpriseGovernanceQueuePayloadStorageObject && row.ObjectId != "" {
			if deleteErr := DeleteEnterpriseGovernanceQueuePayloadObjectFromRegistry(context.Background(), row.Provider, row.ObjectId); deleteErr != nil {
				logger.LogWarn(c, "error deleting orphaned enterprise governance queue request payload object after db failure: "+deleteErr.Error())
			}
		}
		logger.LogWarn(c, "error persisting enterprise governance queue request payload: "+err.Error())
		return
	}
	payload.PayloadId = row.Id
	payload.BodyStorage = row.StorageKind
	payload.BodyBytes = int64(len(body))
	payload.BodyTruncated = false
	payloadJson, err := enterpriseGovernanceQueueRequestPayloadJSON(payload)
	if err != nil {
		logger.LogWarn(c, "error marshaling enterprise governance queue persisted request payload: "+err.Error())
		return
	}
	if err := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ?", admission.Id).
		Update("request_payload_json", payloadJson).Error; err != nil {
		logger.LogWarn(c, "error updating enterprise governance queue persisted request payload reference: "+err.Error())
		return
	}
	admission.RequestPayloadJson = payloadJson
}

func enterpriseGovernanceQueueBodySHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func isEnterpriseGovernanceQueueMultipartContentType(contentType string) bool {
	mediaType, _ := enterpriseGovernanceQueueMediaType(contentType)
	return mediaType == "multipart/form-data"
}

func isEnterpriseGovernanceQueueTextPreviewContentType(contentType string) bool {
	mediaType, _ := enterpriseGovernanceQueueMediaType(contentType)
	return mediaType == "" ||
		mediaType == "application/json" ||
		mediaType == "application/x-ndjson" ||
		mediaType == "application/x-www-form-urlencoded" ||
		mediaType == "text/json" ||
		strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json")
}

func enterpriseGovernanceQueueMediaType(contentType string) (string, map[string]string) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "", nil
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
		params = nil
	}
	return strings.ToLower(mediaType), params
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

func enterpriseGovernanceQueueAdmissionPriority(decision PolicyDecision) int {
	policyId := firstEnterpriseQueuePolicyActionObservationId(decision.ActionObservations)
	if policyId <= 0 {
		return 0
	}
	var policy model.EnterpriseQuotaPolicy
	if err := model.DB.Select("id, priority").Where("id = ?", policyId).First(&policy).Error; err != nil {
		return 0
	}
	return policy.Priority
}

func durationMillis(duration time.Duration) int64 {
	return int64(duration / time.Millisecond)
}
