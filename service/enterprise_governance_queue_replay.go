package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	EnterpriseGovernanceQueueReplayDefaultBatchSize = 20
	EnterpriseGovernanceQueueReplayMaxBatchSize     = 200

	enterpriseGovernanceAuditActionQueueReplay    = "enterprise_governance.queue_admission.replay"
	enterpriseGovernanceQueueReplayTickInterval   = 15 * time.Second
	enterpriseGovernanceQueueReplayRequestTimeout = 10 * time.Minute
)

var (
	ErrEnterpriseGovernanceQueueReplayExecutorNotConfigured = errors.New("enterprise governance queue replay executor is not configured")
	ErrEnterpriseGovernanceQueueReplayPayloadMissing        = errors.New("enterprise governance queue replay payload is missing")
	ErrEnterpriseGovernanceQueueReplayPayloadTruncated      = errors.New("enterprise governance queue replay payload body is truncated")
	ErrEnterpriseGovernanceQueueReplayPayloadUnsupported    = errors.New("enterprise governance queue replay payload is unsupported")
	ErrEnterpriseGovernanceQueueReplayTokenInvalid          = errors.New("enterprise governance queue replay token is invalid")

	enterpriseGovernanceQueueReplayOnce     sync.Once
	enterpriseGovernanceQueueReplayRunning  atomic.Bool
	enterpriseGovernanceQueueReplayExecutor EnterpriseGovernanceQueueReplayExecutor
)

type EnterpriseGovernanceQueueReplayExecutor func(context.Context, EnterpriseGovernanceQueueReplayRequest) EnterpriseGovernanceQueueReplayResult

type EnterpriseGovernanceQueueReplayRequest struct {
	Admission   model.EnterpriseGovernanceQueueAdmission
	Payload     EnterpriseGovernanceQueueRequestPayload
	Method      string
	Path        string
	RawQuery    string
	ContentType string
	Body        []byte
	TokenKey    string
}

type EnterpriseGovernanceQueueReplayResult struct {
	StatusCode int
	Error      error
	DurationMs int64
}

type EnterpriseGovernanceQueueReplayStats struct {
	Scanned    int   `json:"scanned"`
	Claimed    int   `json:"claimed"`
	Replayed   int   `json:"replayed"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped"`
	Errors     int   `json:"errors"`
	DurationMs int64 `json:"duration_ms"`
}

func StartEnterpriseGovernanceQueueReplayTask(executor EnterpriseGovernanceQueueReplayExecutor) {
	if executor == nil {
		logger.LogWarn(context.Background(), ErrEnterpriseGovernanceQueueReplayExecutorNotConfigured.Error())
		return
	}
	enterpriseGovernanceQueueReplayExecutor = executor
	enterpriseGovernanceQueueReplayOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise governance queue replay task started: tick=%s", enterpriseGovernanceQueueReplayTickInterval))
			ticker := time.NewTicker(enterpriseGovernanceQueueReplayTickInterval)
			defer ticker.Stop()

			runEnterpriseGovernanceQueueReplayOnce()
			for range ticker.C {
				runEnterpriseGovernanceQueueReplayOnce()
			}
		})
	})
}

func runEnterpriseGovernanceQueueReplayOnce() {
	if !enterpriseGovernanceQueueReplayRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseGovernanceQueueReplayRunning.Store(false)

	ctx := context.Background()
	totalStats := EnterpriseGovernanceQueueReplayStats{}
	for {
		stats, err := ProcessEnterpriseGovernanceQueueReplayBatchWithStats(EnterpriseGovernanceQueueReplayDefaultBatchSize, enterpriseGovernanceQueueReplayExecutor)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("enterprise governance queue replay task failed: %v", err))
			return
		}
		totalStats.Scanned += stats.Scanned
		totalStats.Claimed += stats.Claimed
		totalStats.Replayed += stats.Replayed
		totalStats.Failed += stats.Failed
		totalStats.Skipped += stats.Skipped
		totalStats.Errors += stats.Errors
		totalStats.DurationMs += stats.DurationMs
		if stats.Claimed < EnterpriseGovernanceQueueReplayDefaultBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalStats.Claimed > 0 {
		logger.LogDebug(ctx, "enterprise governance queue replay task: scanned=%d, claimed=%d, replayed=%d, failed=%d, skipped=%d, errors=%d, duration_ms=%d", totalStats.Scanned, totalStats.Claimed, totalStats.Replayed, totalStats.Failed, totalStats.Skipped, totalStats.Errors, totalStats.DurationMs)
	}
}

func ProcessEnterpriseGovernanceQueueReplayBatchWithStats(batchSize int, executor EnterpriseGovernanceQueueReplayExecutor) (EnterpriseGovernanceQueueReplayStats, error) {
	return processEnterpriseGovernanceQueueReplayBatchWithStats(context.Background(), common.GetTimestamp(), batchSize, executor)
}

func processEnterpriseGovernanceQueueReplayBatchWithStats(ctx context.Context, now int64, batchSize int, executor EnterpriseGovernanceQueueReplayExecutor) (EnterpriseGovernanceQueueReplayStats, error) {
	stats := EnterpriseGovernanceQueueReplayStats{}
	if executor == nil {
		return stats, ErrEnterpriseGovernanceQueueReplayExecutorNotConfigured
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	batchSize = normalizeEnterpriseGovernanceQueueReplayBatchSize(batchSize)
	startedAt := time.Now()

	var rows []model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.
		Where("status = ? AND (next_retry_at = 0 OR next_retry_at <= ?)", enterpriseQueueStatusRetryPending, now).
		Order("next_retry_at asc, created_at asc, id asc").
		Limit(batchSize).
		Find(&rows).Error; err != nil {
		return stats, err
	}

	for _, row := range rows {
		stats.Scanned++
		before := row
		claimed, claimedOK, err := claimEnterpriseGovernanceQueueReplay(row, now)
		if err != nil {
			stats.Errors++
			return stats, err
		}
		if !claimedOK {
			stats.Skipped++
			continue
		}
		stats.Claimed++

		replayRequest, err := BuildEnterpriseGovernanceQueueReplayRequest(claimed)
		if err != nil {
			after, finishErr := finishEnterpriseGovernanceQueueReplayFailure(before, claimed, now, err, EnterpriseGovernanceQueueReplayResult{})
			if finishErr != nil {
				stats.Errors++
				return stats, finishErr
			}
			if auditErr := recordEnterpriseGovernanceQueueReplayAudit(before, after, nil, EnterpriseGovernanceQueueReplayResult{}, err); auditErr != nil {
				stats.Errors++
				return stats, auditErr
			}
			stats.Failed++
			continue
		}

		requestCtx, cancel := context.WithTimeout(ctx, enterpriseGovernanceQueueReplayRequestTimeout)
		executedAt := time.Now()
		result := executor(requestCtx, replayRequest)
		cancel()
		if result.DurationMs <= 0 {
			result.DurationMs = durationMillis(time.Since(executedAt))
		}

		if result.Error != nil || result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusBadRequest {
			err := enterpriseGovernanceQueueReplayResultError(result)
			after, finishErr := finishEnterpriseGovernanceQueueReplayFailure(before, claimed, now, err, result)
			if finishErr != nil {
				stats.Errors++
				return stats, finishErr
			}
			if auditErr := recordEnterpriseGovernanceQueueReplayAudit(before, after, &replayRequest, result, err); auditErr != nil {
				stats.Errors++
				return stats, auditErr
			}
			stats.Failed++
			continue
		}

		after, err := finishEnterpriseGovernanceQueueReplaySuccess(before, claimed, now, result)
		if err != nil {
			stats.Errors++
			return stats, err
		}
		if auditErr := recordEnterpriseGovernanceQueueReplayAudit(before, after, &replayRequest, result, nil); auditErr != nil {
			stats.Errors++
			return stats, auditErr
		}
		stats.Replayed++
	}

	stats.DurationMs = durationMillis(time.Since(startedAt))
	return stats, nil
}

func BuildEnterpriseGovernanceQueueReplayRequest(admission model.EnterpriseGovernanceQueueAdmission) (EnterpriseGovernanceQueueReplayRequest, error) {
	if strings.TrimSpace(admission.RequestPayloadJson) == "" {
		return EnterpriseGovernanceQueueReplayRequest{}, ErrEnterpriseGovernanceQueueReplayPayloadMissing
	}
	var payload EnterpriseGovernanceQueueRequestPayload
	if err := common.UnmarshalJsonStr(admission.RequestPayloadJson, &payload); err != nil {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: %v", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported, err)
	}
	method := strings.ToUpper(strings.TrimSpace(payload.Method))
	if method != http.MethodPost {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: unsupported method %q", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported, payload.Method)
	}
	path := strings.TrimSpace(payload.Path)
	if !isEnterpriseGovernanceQueueReplayPathSupported(path) {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: unsupported path %q", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported, path)
	}
	body, durableBody, err := enterpriseGovernanceQueueReplayBody(admission, payload)
	if err != nil {
		return EnterpriseGovernanceQueueReplayRequest{}, err
	}
	if !isEnterpriseGovernanceQueueReplayContentTypeSupported(payload.ContentType, durableBody) {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: unsupported content type %q", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported, payload.ContentType)
	}
	token, err := model.GetTokenById(admission.TokenId)
	if err != nil {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: %v", ErrEnterpriseGovernanceQueueReplayTokenInvalid, err)
	}
	if admission.UserId > 0 && token.UserId != admission.UserId {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: token user mismatch", ErrEnterpriseGovernanceQueueReplayTokenInvalid)
	}
	if _, err := model.ValidateUserToken(token.Key); err != nil {
		return EnterpriseGovernanceQueueReplayRequest{}, fmt.Errorf("%w: %v", ErrEnterpriseGovernanceQueueReplayTokenInvalid, err)
	}
	return EnterpriseGovernanceQueueReplayRequest{
		Admission:   admission,
		Payload:     payload,
		Method:      method,
		Path:        path,
		RawQuery:    payload.RawQuery,
		ContentType: payload.ContentType,
		Body:        body,
		TokenKey:    token.Key,
	}, nil
}

func enterpriseGovernanceQueueReplayBody(admission model.EnterpriseGovernanceQueueAdmission, payload EnterpriseGovernanceQueueRequestPayload) ([]byte, bool, error) {
	if payload.PayloadId > 0 || payload.BodyStorage == enterpriseQueueRequestBodyStorageDB {
		if payload.PayloadId <= 0 {
			return nil, true, fmt.Errorf("%w: missing payload id", ErrEnterpriseGovernanceQueueReplayPayloadMissing)
		}
		var row model.EnterpriseGovernanceQueuePayload
		if err := model.DB.Where("id = ?", payload.PayloadId).First(&row).Error; err != nil {
			return nil, true, fmt.Errorf("%w: durable payload %d: %v", ErrEnterpriseGovernanceQueueReplayPayloadMissing, payload.PayloadId, err)
		}
		if row.AdmissionId != admission.Id {
			return nil, true, fmt.Errorf("%w: durable payload admission mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		if row.RequestId != "" && admission.RequestId != "" && row.RequestId != admission.RequestId {
			return nil, true, fmt.Errorf("%w: durable payload request mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		if row.EnterpriseId != 0 && row.EnterpriseId != admission.EnterpriseId {
			return nil, true, fmt.Errorf("%w: durable payload enterprise mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		if row.UserId != 0 && row.UserId != admission.UserId {
			return nil, true, fmt.Errorf("%w: durable payload user mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		if row.TokenId != 0 && row.TokenId != admission.TokenId {
			return nil, true, fmt.Errorf("%w: durable payload token mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		body := append([]byte(nil), row.Body...)
		if row.BodyBytes > 0 && row.BodyBytes != int64(len(body)) {
			return nil, true, fmt.Errorf("%w: durable payload byte length mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		expectedSHA256 := strings.TrimSpace(payload.BodySHA256)
		if expectedSHA256 == "" {
			expectedSHA256 = strings.TrimSpace(row.SHA256)
		}
		if expectedSHA256 != "" && !strings.EqualFold(enterpriseGovernanceQueueBodySHA256(body), expectedSHA256) {
			return nil, true, fmt.Errorf("%w: durable payload sha256 mismatch", ErrEnterpriseGovernanceQueueReplayPayloadUnsupported)
		}
		return body, true, nil
	}
	if payload.BodyTruncated {
		return nil, false, ErrEnterpriseGovernanceQueueReplayPayloadTruncated
	}
	return []byte(payload.Body), false, nil
}

func claimEnterpriseGovernanceQueueReplay(row model.EnterpriseGovernanceQueueAdmission, now int64) (model.EnterpriseGovernanceQueueAdmission, bool, error) {
	result := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ? AND status = ? AND (next_retry_at = 0 OR next_retry_at <= ?)", row.Id, enterpriseQueueStatusRetryPending, now).
		Updates(map[string]any{
			"status":           enterpriseQueueStatusReplayProcessing,
			"last_error":       "",
			"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusReplayProcessing),
			"updated_at":       now,
		})
	if result.Error != nil {
		return row, false, result.Error
	}
	if result.RowsAffected == 0 {
		return row, false, nil
	}
	var claimed model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.Where("id = ?", row.Id).First(&claimed).Error; err != nil {
		return row, false, err
	}
	return claimed, true, nil
}

func finishEnterpriseGovernanceQueueReplaySuccess(before model.EnterpriseGovernanceQueueAdmission, claimed model.EnterpriseGovernanceQueueAdmission, now int64, result EnterpriseGovernanceQueueReplayResult) (model.EnterpriseGovernanceQueueAdmission, error) {
	if result.DurationMs < 0 {
		result.DurationMs = 0
	}
	updates := map[string]any{
		"status":           enterpriseQueueStatusReleased,
		"admitted_at":      now,
		"released_at":      now,
		"run_ms":           result.DurationMs,
		"next_retry_at":    int64(0),
		"last_error":       "",
		"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusReleased),
		"updated_at":       now,
	}
	return finishEnterpriseGovernanceQueueReplay(before, claimed, updates)
}

func finishEnterpriseGovernanceQueueReplayFailure(before model.EnterpriseGovernanceQueueAdmission, claimed model.EnterpriseGovernanceQueueAdmission, now int64, replayErr error, result EnterpriseGovernanceQueueReplayResult) (model.EnterpriseGovernanceQueueAdmission, error) {
	lastError := ""
	if replayErr != nil {
		lastError = replayErr.Error()
	}
	if result.DurationMs < 0 {
		result.DurationMs = 0
	}
	updates := map[string]any{
		"status":           enterpriseQueueStatusTimeout,
		"run_ms":           result.DurationMs,
		"next_retry_at":    int64(0),
		"last_error":       lastError,
		"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusTimeout),
		"updated_at":       now,
	}
	return finishEnterpriseGovernanceQueueReplay(before, claimed, updates)
}

func finishEnterpriseGovernanceQueueReplay(before model.EnterpriseGovernanceQueueAdmission, claimed model.EnterpriseGovernanceQueueAdmission, updates map[string]any) (model.EnterpriseGovernanceQueueAdmission, error) {
	result := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
		Where("id = ? AND status = ?", claimed.Id, enterpriseQueueStatusReplayProcessing).
		Updates(updates)
	if result.Error != nil {
		return before, result.Error
	}
	if result.RowsAffected == 0 {
		return before, fmt.Errorf("enterprise governance queue replay admission %d is no longer processing", claimed.Id)
	}
	var after model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.Where("id = ?", claimed.Id).First(&after).Error; err != nil {
		return before, err
	}
	return after, nil
}

func recordEnterpriseGovernanceQueueReplayAudit(before, after model.EnterpriseGovernanceQueueAdmission, replayRequest *EnterpriseGovernanceQueueReplayRequest, result EnterpriseGovernanceQueueReplayResult, replayErr error) error {
	replay := map[string]any{
		"status_code": result.StatusCode,
		"duration_ms": result.DurationMs,
		"success":     replayErr == nil,
	}
	if replayErr != nil {
		replay["error"] = replayErr.Error()
	}
	if replayRequest != nil {
		replay["method"] = replayRequest.Method
		replay["path"] = replayRequest.Path
		replay["raw_query"] = replayRequest.RawQuery
		replay["content_type"] = replayRequest.ContentType
		replay["body_bytes"] = len(replayRequest.Body)
	}
	return model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   after.EnterpriseId,
		Action:         enterpriseGovernanceAuditActionQueueReplay,
		TargetType:     "enterprise_governance_queue_admission",
		TargetId:       int(after.Id),
		ScopeUserId:    after.UserId,
		ScopeOrgUnitId: after.OrgUnitId,
		ScopeProjectId: after.ProjectId,
		Before:         before,
		After: map[string]any{
			"admission": after,
			"replay":    replay,
		},
		RequestId: after.RequestId,
	})
}

func enterpriseGovernanceQueueReplayResultError(result EnterpriseGovernanceQueueReplayResult) error {
	if result.Error != nil {
		return result.Error
	}
	if result.StatusCode == 0 {
		return errors.New("enterprise governance queue replay returned no status code")
	}
	return fmt.Errorf("enterprise governance queue replay returned status %d", result.StatusCode)
}

func isEnterpriseGovernanceQueueReplayPathSupported(path string) bool {
	if path == "" || !strings.HasPrefix(path, "/") {
		return false
	}
	switch path {
	case "/v1/messages",
		"/v1/edits",
		"/v1/completions",
		"/v1/chat/completions",
		"/v1/responses",
		"/v1/responses/compact",
		"/v1/embeddings",
		"/v1/rerank",
		"/v1/moderations",
		"/v1/images/edits",
		"/v1/images/generations",
		"/v1/audio/transcriptions",
		"/v1/audio/translations",
		"/v1/audio/speech":
		return true
	default:
		return strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1beta/openai/models/")
	}
}

func isEnterpriseGovernanceQueueReplayContentTypeSupported(contentType string, durableBody bool) bool {
	mediaType, params := enterpriseGovernanceQueueMediaType(contentType)
	if mediaType == "" {
		return true
	}
	if mediaType == "multipart/form-data" {
		return durableBody && strings.TrimSpace(params["boundary"]) != ""
	}
	return mediaType == "application/json" ||
		mediaType == "application/x-ndjson" ||
		mediaType == "text/json" ||
		strings.HasSuffix(mediaType, "+json")
}

func normalizeEnterpriseGovernanceQueueReplayBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return EnterpriseGovernanceQueueReplayDefaultBatchSize
	}
	if batchSize > EnterpriseGovernanceQueueReplayMaxBatchSize {
		return EnterpriseGovernanceQueueReplayMaxBatchSize
	}
	return batchSize
}
