package service

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/klauspost/compress/zstd"
	"gorm.io/gorm"
)

const (
	requestCaptureBundleMasterKeyEnv = "CAPTURE_BUNDLE_MASTER_KEY"
	requestCaptureBundleKeyIdEnv     = "CAPTURE_BUNDLE_KEY_ID"
	requestCaptureBundleMagic        = "DPCE1"
	requestCaptureBundleCompression  = "zstd"
	requestCaptureBundleEncryption   = "AES-256-GCM"
	requestCaptureBundleContentType  = "application/x-data-proxy-capture-bundle"
)

var ErrRequestCaptureBundleDecodedTooLarge = errors.New("request capture bundle decoded body exceeds limit")

type RequestCaptureFinalizeOptions struct {
	SessionDir string
}

type RequestCaptureFinalizeResult struct {
	Manifest        RequestCaptureSpoolManifest  `json:"manifest"`
	Object          RequestCaptureObject         `json:"object"`
	Artifact        model.RequestCaptureArtifact `json:"artifact"`
	TarBytes        int64                        `json:"tar_bytes"`
	CompressedBytes int64                        `json:"compressed_bytes"`
	EncryptedBytes  int64                        `json:"encrypted_bytes"`
}

type RequestCaptureFinalizerWorkerOptions struct {
	SpoolDir         string
	Limit            int
	RemoveOnSuccess  bool
	RetryBaseSeconds int
	RetryMaxSeconds  int
	Now              func() int64
}

type RequestCaptureFinalizerWorkerSummary struct {
	Scanned   int      `json:"scanned"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

type RequestCaptureSpoolRecoveryOptions struct {
	SpoolDir           string
	ActiveStaleSeconds int64
	Now                func() int64
}

type RequestCaptureSpoolRecoverySummary struct {
	ActiveRecovered int      `json:"active_recovered"`
	FinalizeSynced  int      `json:"finalize_synced"`
	FailedSynced    int      `json:"failed_synced"`
	Skipped         int      `json:"skipped"`
	Errors          []string `json:"errors,omitempty"`
}

func FinalizeRequestCaptureSpoolSession(ctx context.Context, options RequestCaptureFinalizeOptions) (RequestCaptureFinalizeResult, error) {
	sessionDir := strings.TrimSpace(options.SessionDir)
	if sessionDir == "" {
		return RequestCaptureFinalizeResult{}, errors.New("request capture finalizer session dir is required")
	}
	manifest, err := readRequestCaptureSpoolManifest(sessionDir)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	if manifest.Status != requestCaptureSpoolStatusFinalize {
		return RequestCaptureFinalizeResult{}, fmt.Errorf("request capture finalizer expected finalize status, got %q", manifest.Status)
	}
	tarBytes, err := buildRequestCaptureTarBundle(sessionDir)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	compressedBytes, err := compressRequestCaptureBundleZstd(tarBytes)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	key, keyId, err := requestCaptureBundleMasterKeyFromEnv()
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	encryptedBytes, err := encryptRequestCaptureBundle(compressedBytes, key)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	object, err := SaveRequestCaptureObject(ctx, RequestCaptureObject{
		RequestId:   manifest.RequestId,
		Kind:        model.RequestCaptureArtifactKindRawBundle,
		ContentType: requestCaptureBundleContentType,
		CreatedAt:   manifest.CreatedAt,
	}, encryptedBytes)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	manifestJson, _ := json.Marshal(manifest)
	now := common.GetTimestamp()
	artifact := model.RequestCaptureArtifact{
		RequestId:           manifest.RequestId,
		Kind:                model.RequestCaptureArtifactKindRawBundle,
		Status:              model.RequestCaptureArtifactStatusAvailable,
		Provider:            object.Provider,
		Bucket:              object.Bucket,
		StorageKey:          object.StorageKey,
		ContentType:         requestCaptureBundleContentType,
		Compression:         requestCaptureBundleCompression,
		EncryptionAlgorithm: requestCaptureBundleEncryption,
		EncryptionKeyId:     keyId,
		SHA256:              object.SHA256,
		SizeBytes:           object.BodyBytes,
		ManifestJson:        string(manifestJson),
		UploadedAt:          now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return RequestCaptureFinalizeResult{
		Manifest:        manifest,
		Object:          object,
		Artifact:        artifact,
		TarBytes:        int64(len(tarBytes)),
		CompressedBytes: int64(len(compressedBytes)),
		EncryptedBytes:  int64(len(encryptedBytes)),
	}, nil
}

func FinalizeAndPersistRequestCaptureSpoolSession(ctx context.Context, options RequestCaptureFinalizeOptions) (RequestCaptureFinalizeResult, error) {
	result, err := FinalizeRequestCaptureSpoolSession(ctx, options)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	artifact, err := PersistRequestCaptureFinalizeResult(ctx, result)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	result.Artifact = artifact
	return result, nil
}

func FinalizePendingRequestCaptureSpool(ctx context.Context, options RequestCaptureFinalizerWorkerOptions) (RequestCaptureFinalizerWorkerSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spoolDir := strings.TrimSpace(options.SpoolDir)
	if spoolDir == "" {
		spoolDir = requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	finalizeRoot := filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize)
	entries, err := os.ReadDir(finalizeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return RequestCaptureFinalizerWorkerSummary{}, nil
		}
		return RequestCaptureFinalizerWorkerSummary{}, err
	}
	now := requestCaptureFinalizerNow(options)
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	summary := RequestCaptureFinalizerWorkerSummary{}
	for _, entry := range entries {
		if summary.Scanned >= limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if !entry.IsDir() {
			summary.Skipped++
			continue
		}
		summary.Scanned++
		sessionDir := filepath.Join(finalizeRoot, entry.Name())
		manifest, err := readRequestCaptureSpoolManifest(sessionDir)
		if err != nil {
			summary.Failed++
			target, failedManifest, quarantineErr := quarantineUnreadableRequestCaptureSpoolSession(ctx, sessionDir, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed), entry.Name(), err, now)
			if quarantineErr != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; quarantine: %s", entry.Name(), err.Error(), quarantineErr.Error()))
				continue
			}
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; moved to failed spool", entry.Name(), err.Error()))
			syncRequestCaptureRecordFromSpoolPaths(ctx, failedManifest, sessionDir, target, model.RequestCaptureStatusFailed, failedManifest.Error, now)
			continue
		}
		decision, err := requestCaptureFinalizeDecision(ctx, manifest.RequestId, now)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s decision: %s", entry.Name(), err.Error()))
			continue
		}
		if decision.skip {
			summary.Skipped++
			if decision.cleanup && options.RemoveOnSuccess {
				if err := os.RemoveAll(sessionDir); err != nil {
					summary.Failed++
					summary.Errors = append(summary.Errors, fmt.Sprintf("%s cleanup: %s", entry.Name(), err.Error()))
				}
			}
			continue
		}
		if _, err := FinalizeAndPersistRequestCaptureSpoolSession(ctx, RequestCaptureFinalizeOptions{SessionDir: sessionDir}); err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %s", entry.Name(), err.Error()))
			recordRequestCaptureFinalizeFailure(ctx, manifest.RequestId, err, options, now)
			continue
		}
		summary.Succeeded++
		if options.RemoveOnSuccess {
			if err := os.RemoveAll(sessionDir); err != nil {
				summary.Failed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s cleanup: %s", entry.Name(), err.Error()))
			}
		}
	}
	return summary, nil
}

func PersistRequestCaptureFinalizeResult(ctx context.Context, result RequestCaptureFinalizeResult) (model.RequestCaptureArtifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return model.RequestCaptureArtifact{}, errors.New("database is not initialized")
	}
	if strings.TrimSpace(result.Manifest.RequestId) == "" {
		return model.RequestCaptureArtifact{}, errors.New("request capture finalize result request id is empty")
	}
	artifact := result.Artifact
	if strings.TrimSpace(artifact.RequestId) == "" {
		artifact.RequestId = result.Manifest.RequestId
	}
	if artifact.Status == "" {
		artifact.Status = model.RequestCaptureArtifactStatusAvailable
	}
	if artifact.Kind == "" {
		artifact.Kind = model.RequestCaptureArtifactKindRawBundle
	}
	now := common.GetTimestamp()
	if artifact.UploadedAt == 0 {
		artifact.UploadedAt = now
	}
	return artifact, model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.RequestCaptureRecord
		recordFound := false
		err := tx.Where("request_id = ?", result.Manifest.RequestId).Order("id desc").First(&record).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			recordFound = true
			artifact.CaptureId = record.Id
			if artifact.ExpiresAt == 0 && record.ExpiresAt > 0 {
				artifact.ExpiresAt = record.ExpiresAt
			}
		}
		if artifact.ExpiresAt == 0 {
			artifact.ExpiresAt = RequestCaptureExpiryFromNow(now)
		}
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		if recordFound {
			updates := map[string]interface{}{
				"capture_status":    model.RequestCaptureStatusUploaded,
				"finalized_at":      artifact.UploadedAt,
				"total_bytes":       artifact.SizeBytes,
				"has_error":         false,
				"error_code":        "",
				"last_error":        "",
				"finalize_attempts": 0,
				"next_finalize_at":  0,
			}
			if err := tx.Model(&model.RequestCaptureRecord{}).Where("id = ?", record.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func RecoverStaleRequestCaptureSpool(ctx context.Context, options RequestCaptureSpoolRecoveryOptions) (RequestCaptureSpoolRecoverySummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spoolDir := strings.TrimSpace(options.SpoolDir)
	if spoolDir == "" {
		spoolDir = requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir)
	}
	now := requestCaptureSpoolRecoveryNow(options)
	summary := RequestCaptureSpoolRecoverySummary{}
	if err := recoverStaleRequestCaptureActiveSpool(ctx, spoolDir, now, options.ActiveStaleSeconds, &summary); err != nil {
		return summary, err
	}
	if err := syncRequestCaptureSpoolStatusDir(ctx, filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize), model.RequestCaptureStatusFinalizing, now, &summary); err != nil {
		return summary, err
	}
	if err := syncRequestCaptureSpoolStatusDir(ctx, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed), model.RequestCaptureStatusFailed, now, &summary); err != nil {
		return summary, err
	}
	return summary, nil
}

type requestCaptureFinalizeDecisionResult struct {
	skip    bool
	cleanup bool
}

func requestCaptureFinalizeDecision(ctx context.Context, requestId string, now int64) (requestCaptureFinalizeDecisionResult, error) {
	requestId = strings.TrimSpace(requestId)
	if model.DB == nil || requestId == "" {
		return requestCaptureFinalizeDecisionResult{}, nil
	}
	var record model.RequestCaptureRecord
	err := model.DB.WithContext(ctx).Where("request_id = ?", requestId).Order("id desc").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requestCaptureFinalizeDecisionResult{}, nil
	}
	if err != nil {
		return requestCaptureFinalizeDecisionResult{}, err
	}
	switch record.CaptureStatus {
	case model.RequestCaptureStatusUploaded, model.RequestCaptureStatusDeleted, model.RequestCaptureStatusExpired:
		return requestCaptureFinalizeDecisionResult{skip: true, cleanup: true}, nil
	}
	if record.NextFinalizeAt > 0 && record.NextFinalizeAt > now {
		return requestCaptureFinalizeDecisionResult{skip: true}, nil
	}
	return requestCaptureFinalizeDecisionResult{}, nil
}

func recordRequestCaptureFinalizeFailure(ctx context.Context, requestId string, cause error, options RequestCaptureFinalizerWorkerOptions, now int64) {
	requestId = strings.TrimSpace(requestId)
	if model.DB == nil || requestId == "" || cause == nil {
		return
	}
	var record model.RequestCaptureRecord
	err := model.DB.WithContext(ctx).Where("request_id = ?", requestId).Order("id desc").First(&record).Error
	if err != nil {
		return
	}
	attempts := record.FinalizeAttempts + 1
	delay := requestCaptureFinalizeRetryDelaySeconds(attempts, options.RetryBaseSeconds, options.RetryMaxSeconds)
	updates := map[string]interface{}{
		"capture_status":    model.RequestCaptureStatusFinalizing,
		"has_error":         true,
		"error_code":        "request_capture_finalize_failed",
		"last_error":        truncateRequestCaptureError(cause.Error()),
		"finalize_attempts": attempts,
		"next_finalize_at":  now + delay,
	}
	_ = model.DB.WithContext(ctx).Model(&model.RequestCaptureRecord{}).Where("id = ?", record.Id).Updates(updates).Error
}

func requestCaptureFinalizeRetryDelaySeconds(attempts int, baseSeconds int, maxSeconds int) int64 {
	if attempts <= 0 {
		attempts = 1
	}
	if baseSeconds <= 0 {
		baseSeconds = 60
	}
	if maxSeconds <= 0 {
		maxSeconds = 3600
	}
	delay := int64(baseSeconds)
	maxDelay := int64(maxSeconds)
	for i := 1; i < attempts; i++ {
		if delay >= maxDelay {
			return maxDelay
		}
		delay *= 2
		if delay > maxDelay {
			return maxDelay
		}
	}
	return delay
}

func requestCaptureFinalizerNow(options RequestCaptureFinalizerWorkerOptions) int64 {
	if options.Now != nil {
		return options.Now()
	}
	return common.GetTimestamp()
}

func requestCaptureSpoolRecoveryNow(options RequestCaptureSpoolRecoveryOptions) int64 {
	if options.Now != nil {
		return options.Now()
	}
	return common.GetTimestamp()
}

func recoverStaleRequestCaptureActiveSpool(ctx context.Context, spoolDir string, now int64, activeStaleSeconds int64, summary *RequestCaptureSpoolRecoverySummary) error {
	activeRoot := filepath.Join(spoolDir, requestCaptureSpoolStatusActive)
	entries, err := os.ReadDir(activeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			summary.Skipped++
			continue
		}
		sessionDir := filepath.Join(activeRoot, entry.Name())
		manifest, err := readRequestCaptureSpoolManifest(sessionDir)
		if err != nil {
			target, failedManifest, quarantineErr := quarantineUnreadableRequestCaptureSpoolSession(ctx, sessionDir, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed), entry.Name(), err, now)
			if quarantineErr != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; quarantine: %s", entry.Name(), err.Error(), quarantineErr.Error()))
				continue
			}
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; moved to failed spool", entry.Name(), err.Error()))
			summary.ActiveRecovered++
			syncRequestCaptureRecordFromSpoolPaths(ctx, failedManifest, sessionDir, target, model.RequestCaptureStatusFailed, failedManifest.Error, now)
			continue
		}
		updatedAt := manifest.UpdatedAt
		if updatedAt == 0 {
			updatedAt = manifest.CreatedAt
		}
		if activeStaleSeconds > 0 && updatedAt > 0 && updatedAt+activeStaleSeconds > now {
			summary.Skipped++
			continue
		}
		_ = refreshRequestCaptureSpoolManifestArtifacts(sessionDir, &manifest)
		manifest.Status = requestCaptureSpoolStatusFailed
		manifest.Error = "recovered stale active request capture session after service restart"
		manifest.UpdatedAt = now
		manifest.FinishedAt = now
		if err := writeRequestCaptureSpoolManifest(sessionDir, manifest); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s write manifest: %s", entry.Name(), err.Error()))
			continue
		}
		target, err := requestCaptureMoveSessionDir(sessionDir, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed))
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s move failed: %s", entry.Name(), err.Error()))
			continue
		}
		summary.ActiveRecovered++
		syncRequestCaptureRecordFromSpool(ctx, manifest, target, model.RequestCaptureStatusFailed, manifest.Error, now)
	}
	return nil
}

func syncRequestCaptureSpoolStatusDir(ctx context.Context, root string, captureStatus string, now int64, summary *RequestCaptureSpoolRecoverySummary) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			summary.Skipped++
			continue
		}
		sessionDir := filepath.Join(root, entry.Name())
		manifest, err := readRequestCaptureSpoolManifest(sessionDir)
		if err != nil {
			if captureStatus == model.RequestCaptureStatusFinalizing {
				target, failedManifest, quarantineErr := quarantineUnreadableRequestCaptureSpoolSession(ctx, sessionDir, filepath.Join(filepath.Dir(root), requestCaptureSpoolStatusFailed), entry.Name(), err, now)
				if quarantineErr != nil {
					summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; quarantine: %s", entry.Name(), err.Error(), quarantineErr.Error()))
					continue
				}
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s; moved to failed spool", entry.Name(), err.Error()))
				if syncRequestCaptureRecordFromSpoolPaths(ctx, failedManifest, sessionDir, target, model.RequestCaptureStatusFailed, failedManifest.Error, now) {
					summary.FailedSynced++
				} else {
					summary.Skipped++
				}
				continue
			}
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s manifest: %s", entry.Name(), err.Error()))
			continue
		}
		message := ""
		if captureStatus == model.RequestCaptureStatusFailed {
			message = manifest.Error
		}
		if syncRequestCaptureRecordFromSpool(ctx, manifest, sessionDir, captureStatus, message, now) {
			if captureStatus == model.RequestCaptureStatusFinalizing {
				summary.FinalizeSynced++
			} else if captureStatus == model.RequestCaptureStatusFailed {
				summary.FailedSynced++
			}
		} else {
			summary.Skipped++
		}
	}
	return nil
}

func syncRequestCaptureRecordFromSpool(ctx context.Context, manifest RequestCaptureSpoolManifest, spoolDir string, captureStatus string, message string, now int64) bool {
	return syncRequestCaptureRecordFromSpoolPaths(ctx, manifest, "", spoolDir, captureStatus, message, now)
}

func syncRequestCaptureRecordFromSpoolPaths(ctx context.Context, manifest RequestCaptureSpoolManifest, previousSpoolDir string, spoolDir string, captureStatus string, message string, now int64) bool {
	requestId := strings.TrimSpace(manifest.RequestId)
	if model.DB == nil {
		return false
	}
	var record model.RequestCaptureRecord
	if !findRequestCaptureRecordForSpoolSync(ctx, requestId, previousSpoolDir, spoolDir, &record) {
		return false
	}
	switch record.CaptureStatus {
	case model.RequestCaptureStatusUploaded, model.RequestCaptureStatusDeleted, model.RequestCaptureStatusExpired:
		return false
	}
	updates := map[string]interface{}{
		"capture_status": captureStatus,
		"spool_dir":      spoolDir,
	}
	if captureStatus == model.RequestCaptureStatusFailed {
		updates["has_error"] = true
		updates["error_code"] = "request_capture_spool_recovered_failed"
		updates["last_error"] = truncateRequestCaptureError(message)
		updates["finished_at"] = now
	} else if captureStatus == model.RequestCaptureStatusFinalizing {
		if record.StartedAt == 0 && manifest.CreatedAt > 0 {
			updates["started_at"] = manifest.CreatedAt
		}
	}
	if err := model.DB.WithContext(ctx).Model(&model.RequestCaptureRecord{}).Where("id = ?", record.Id).Updates(updates).Error; err != nil {
		return false
	}
	return true
}

func findRequestCaptureRecordForSpoolSync(ctx context.Context, requestId string, previousSpoolDir string, spoolDir string, record *model.RequestCaptureRecord) bool {
	if record == nil || model.DB == nil {
		return false
	}
	requestId = strings.TrimSpace(requestId)
	previousSpoolDir = strings.TrimSpace(previousSpoolDir)
	spoolDir = strings.TrimSpace(spoolDir)
	if requestId != "" {
		err := model.DB.WithContext(ctx).Where("request_id = ?", requestId).Order("id desc").First(record).Error
		if err == nil {
			return true
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false
		}
	}
	for _, candidate := range []string{previousSpoolDir, spoolDir} {
		if candidate == "" {
			continue
		}
		err := model.DB.WithContext(ctx).Where("spool_dir = ?", candidate).Order("id desc").First(record).Error
		if err == nil {
			return true
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false
		}
	}
	return false
}

func quarantineUnreadableRequestCaptureSpoolSession(ctx context.Context, sessionDir string, targetRoot string, requestIdHint string, cause error, now int64) (string, RequestCaptureSpoolManifest, error) {
	requestId := strings.TrimSpace(requestIdHint)
	if requestId == "" {
		requestId = filepath.Base(sessionDir)
	}
	message := "request capture spool manifest unreadable"
	if cause != nil {
		message += ": " + cause.Error()
	}
	message = truncateRequestCaptureError(message)
	manifest := RequestCaptureSpoolManifest{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelMetadata,
		Status:       requestCaptureSpoolStatusFailed,
		Error:        message,
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   now,
		Artifacts:    []RequestCaptureSpoolArtifact{},
		Metadata:     map[string]any{"recovered_from": filepath.Base(sessionDir)},
	}
	if err := writeRequestCaptureSpoolManifest(sessionDir, manifest); err != nil {
		return "", manifest, err
	}
	target, err := requestCaptureMoveSessionDir(sessionDir, targetRoot)
	if err != nil {
		return "", manifest, err
	}
	return target, manifest, nil
}

func refreshRequestCaptureSpoolManifestArtifacts(sessionDir string, manifest *RequestCaptureSpoolManifest) error {
	for i := range manifest.Artifacts {
		artifact := &manifest.Artifacts[i]
		if strings.TrimSpace(artifact.Path) == "" {
			continue
		}
		size, sha256Value, err := requestCaptureFileSHA256(filepath.Join(sessionDir, artifact.Path))
		if err != nil {
			return err
		}
		artifact.Bytes = size
		artifact.SHA256 = sha256Value
	}
	return nil
}

func truncateRequestCaptureError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1024 {
		return value
	}
	return value[:1024]
}

func readRequestCaptureSpoolManifest(sessionDir string) (RequestCaptureSpoolManifest, error) {
	body, err := os.ReadFile(filepath.Join(sessionDir, requestCaptureSpoolManifestName))
	if err != nil {
		return RequestCaptureSpoolManifest{}, err
	}
	var manifest RequestCaptureSpoolManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return RequestCaptureSpoolManifest{}, err
	}
	return manifest, nil
}

func buildRequestCaptureTarBundle(sessionDir string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	err := filepath.WalkDir(sessionDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sessionDir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Mode = 0600
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func compressRequestCaptureBundleZstd(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	encoder, err := zstd.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	if _, err := encoder.Write(body); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func LoadDecodedRequestCaptureArtifactBundle(ctx context.Context, artifact model.RequestCaptureArtifact) ([]byte, error) {
	return LoadDecodedRequestCaptureArtifactBundleWithLimit(ctx, artifact, 0)
}

func LoadDecodedRequestCaptureArtifactBundleWithLimit(ctx context.Context, artifact model.RequestCaptureArtifact, maxDecodedBytes int64) ([]byte, error) {
	storageKey := strings.TrimSpace(artifact.StorageKey)
	if storageKey == "" {
		return nil, errors.New("request capture artifact storage key is empty")
	}
	body, err := LoadRequestCaptureObject(ctx, storageKey)
	if err != nil {
		return nil, err
	}
	return DecodeRequestCaptureBundleWithLimit(body, maxDecodedBytes)
}

func DecodeRequestCaptureBundle(body []byte) ([]byte, error) {
	return DecodeRequestCaptureBundleWithLimit(body, 0)
}

func DecodeRequestCaptureBundleWithLimit(body []byte, maxDecodedBytes int64) ([]byte, error) {
	key, _, err := requestCaptureBundleMasterKeyFromEnv()
	if err != nil {
		return nil, err
	}
	compressedBytes, err := decryptRequestCaptureBundle(body, key)
	if err != nil {
		return nil, err
	}
	return decompressRequestCaptureBundleZstdWithLimit(compressedBytes, maxDecodedBytes)
}

func decompressRequestCaptureBundleZstd(body []byte) ([]byte, error) {
	return decompressRequestCaptureBundleZstdWithLimit(body, 0)
}

func decompressRequestCaptureBundleZstdWithLimit(body []byte, maxDecodedBytes int64) ([]byte, error) {
	decoder, err := zstd.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	if maxDecodedBytes <= 0 {
		return io.ReadAll(decoder)
	}
	limited := io.LimitReader(decoder, maxDecodedBytes+1)
	decoded, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(decoded)) > maxDecodedBytes {
		return nil, fmt.Errorf("%w: max=%d", ErrRequestCaptureBundleDecodedTooLarge, maxDecodedBytes)
	}
	return decoded, nil
}

func encryptRequestCaptureBundle(body []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, body, nil)
	result := make([]byte, 0, len(requestCaptureBundleMagic)+len(nonce)+len(ciphertext))
	result = append(result, requestCaptureBundleMagic...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

func decryptRequestCaptureBundle(body []byte, key []byte) ([]byte, error) {
	if !bytes.HasPrefix(body, []byte(requestCaptureBundleMagic)) {
		return nil, errors.New("request capture bundle has invalid magic")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	offset := len(requestCaptureBundleMagic)
	if len(body) < offset+gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("request capture bundle is too short")
	}
	nonce := body[offset : offset+gcm.NonceSize()]
	ciphertext := body[offset+gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func requestCaptureBundleMasterKeyFromEnv() ([]byte, string, error) {
	value := strings.TrimSpace(os.Getenv(requestCaptureBundleMasterKeyEnv))
	if value == "" {
		return nil, "", errors.New("CAPTURE_BUNDLE_MASTER_KEY is required for request capture finalizer")
	}
	key, err := decodeRequestCaptureBundleMasterKey(value)
	if err != nil {
		return nil, "", err
	}
	if len(key) != 32 {
		return nil, "", fmt.Errorf("CAPTURE_BUNDLE_MASTER_KEY must decode to 32 bytes, got %d", len(key))
	}
	keyId := strings.TrimSpace(os.Getenv(requestCaptureBundleKeyIdEnv))
	if keyId == "" {
		keyId = "env:" + requestCaptureBundleMasterKeyEnv
	}
	return key, keyId, nil
}

func decodeRequestCaptureBundleMasterKey(value string) ([]byte, error) {
	switch {
	case strings.HasPrefix(value, "base64:"):
		return base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "base64:"))
	case strings.HasPrefix(value, "hex:"):
		return hex.DecodeString(strings.TrimPrefix(value, "hex:"))
	default:
		return []byte(value), nil
	}
}
