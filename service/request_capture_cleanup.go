package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	requestCaptureCleanupEnabledEnv         = "CAPTURE_CLEANUP_ENABLED"
	requestCaptureRetentionDaysEnv          = "CAPTURE_RETENTION_DAYS"
	requestCaptureSpoolRetentionDaysEnv     = "CAPTURE_SPOOL_RETENTION_DAYS"
	requestCaptureCleanupIntervalSecondsEnv = "CAPTURE_CLEANUP_INTERVAL_SECONDS"
	requestCaptureCleanupLimitEnv           = "CAPTURE_CLEANUP_LIMIT"
	requestCaptureSpoolWarnBytesEnv         = "CAPTURE_SPOOL_WARN_BYTES"

	requestCaptureDefaultRetentionDays          = 30
	requestCaptureDefaultSpoolRetentionDays     = 7
	requestCaptureDefaultCleanupIntervalSeconds = 3600
	requestCaptureDefaultCleanupLimit           = 500
	requestCaptureMaxCleanupLimit               = 5000
	requestCaptureArtifactCleanupDeleting       = "request capture cleanup is deleting object storage artifact"
)

type RequestCaptureCleanupOptions struct {
	SpoolDir           string `json:"spool_dir"`
	RetentionDays      int    `json:"retention_days"`
	SpoolRetentionDays int    `json:"spool_retention_days"`
	SpoolWarnBytes     int64  `json:"spool_warn_bytes,omitempty"`
	Limit              int    `json:"limit"`
	DryRun             bool   `json:"dry_run"`
	Now                func() int64
}

type RequestCaptureCleanupSummary struct {
	RetentionDays      int      `json:"retention_days"`
	SpoolRetentionDays int      `json:"spool_retention_days"`
	CutoffTime         int64    `json:"cutoff_time,omitempty"`
	SpoolCutoffTime    int64    `json:"spool_cutoff_time,omitempty"`
	DryRun             bool     `json:"dry_run"`
	ScannedRecords     int      `json:"scanned_records"`
	ExpiredRecords     int      `json:"expired_records"`
	ScannedArtifacts   int      `json:"scanned_artifacts"`
	DeletedArtifacts   int      `json:"deleted_artifacts"`
	DeletedObjects     int      `json:"deleted_objects"`
	DeletedObjectBytes int64    `json:"deleted_object_bytes"`
	ScannedSpoolDirs   int      `json:"scanned_spool_dirs"`
	DeletedSpoolDirs   int      `json:"deleted_spool_dirs"`
	SpoolBytes         int64    `json:"spool_bytes,omitempty"`
	SpoolWarnBytes     int64    `json:"spool_warn_bytes,omitempty"`
	SpoolWarning       bool     `json:"spool_warning,omitempty"`
	Skipped            int      `json:"skipped"`
	Failed             int      `json:"failed"`
	Errors             []string `json:"errors,omitempty"`
}

var (
	requestCaptureCleanupOnce    sync.Once
	requestCaptureCleanupRunning atomic.Bool
)

func StartRequestCaptureCleanupTask() {
	requestCaptureCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !RequestCaptureCleanupTaskEnabled() {
			return
		}
		intervalSeconds := RequestCaptureCleanupIntervalSeconds()
		if intervalSeconds <= 0 {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(intervalSeconds) * time.Second
			logger.LogInfo(context.Background(), fmt.Sprintf("request capture cleanup task started: tick=%s retention=%dd spool_retention=%dd", interval, RequestCaptureRetentionDays(), RequestCaptureSpoolRetentionDays()))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runRequestCaptureCleanupOnce()
			for range ticker.C {
				runRequestCaptureCleanupOnce()
			}
		})
	})
}

func RequestCaptureCleanupTaskEnabled() bool {
	return requestCaptureEnvBool(requestCaptureCleanupEnabledEnv, RequestCaptureObjectStorageEnabled())
}

func RequestCaptureRetentionDays() int {
	value := common.GetEnvOrDefault(requestCaptureRetentionDaysEnv, requestCaptureDefaultRetentionDays)
	if value < 0 {
		return requestCaptureDefaultRetentionDays
	}
	return value
}

func RequestCaptureSpoolRetentionDays() int {
	value := common.GetEnvOrDefault(requestCaptureSpoolRetentionDaysEnv, requestCaptureDefaultSpoolRetentionDays)
	if value < 0 {
		return requestCaptureDefaultSpoolRetentionDays
	}
	return value
}

func RequestCaptureCleanupIntervalSeconds() int {
	value := common.GetEnvOrDefault(requestCaptureCleanupIntervalSecondsEnv, requestCaptureDefaultCleanupIntervalSeconds)
	if value < 0 {
		return requestCaptureDefaultCleanupIntervalSeconds
	}
	return value
}

func RequestCaptureCleanupLimit() int {
	return normalizeRequestCaptureCleanupLimit(common.GetEnvOrDefault(requestCaptureCleanupLimitEnv, requestCaptureDefaultCleanupLimit))
}

func RequestCaptureCleanupOptionsFromEnv() RequestCaptureCleanupOptions {
	return RequestCaptureCleanupOptions{
		SpoolDir:           requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir),
		RetentionDays:      RequestCaptureRetentionDays(),
		SpoolRetentionDays: RequestCaptureSpoolRetentionDays(),
		SpoolWarnBytes:     requestCaptureEnvInt64(requestCaptureSpoolWarnBytesEnv, 0),
		Limit:              RequestCaptureCleanupLimit(),
	}
}

func CleanupExpiredRequestCaptureData(ctx context.Context, options RequestCaptureCleanupOptions) (RequestCaptureCleanupSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return RequestCaptureCleanupSummary{}, errors.New("database is not initialized")
	}
	options = normalizeRequestCaptureCleanupOptions(options)
	now := requestCaptureCleanupNow(options)
	summary := RequestCaptureCleanupSummary{
		RetentionDays:      options.RetentionDays,
		SpoolRetentionDays: options.SpoolRetentionDays,
		DryRun:             options.DryRun,
	}
	if options.RetentionDays > 0 {
		summary.CutoffTime = now - int64(options.RetentionDays)*86400
		if err := cleanupExpiredRequestCaptureRecords(ctx, options, now, &summary); err != nil {
			return summary, err
		}
		if err := cleanupExpiredRequestCaptureArtifacts(ctx, options, now, &summary); err != nil {
			return summary, err
		}
	}
	if options.SpoolRetentionDays > 0 {
		summary.SpoolCutoffTime = now - int64(options.SpoolRetentionDays)*86400
		if err := cleanupRequestCaptureSpoolDirs(ctx, options, now, &summary); err != nil {
			return summary, err
		}
	}
	if options.SpoolWarnBytes > 0 {
		spoolBytes, err := measureRequestCaptureDirectoryBytes(options.SpoolDir)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("measure spool size %s: %s", options.SpoolDir, err.Error()))
		} else {
			summary.SpoolBytes = spoolBytes
			summary.SpoolWarnBytes = options.SpoolWarnBytes
			summary.SpoolWarning = spoolBytes >= options.SpoolWarnBytes
		}
	}
	return summary, nil
}

func RequestCaptureExpiryFromNow(now int64) int64 {
	retentionDays := RequestCaptureRetentionDays()
	if retentionDays <= 0 {
		return 0
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	return now + int64(retentionDays)*86400
}

func runRequestCaptureCleanupOnce() {
	if !requestCaptureCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer requestCaptureCleanupRunning.Store(false)

	options := RequestCaptureCleanupOptionsFromEnv()
	summary, err := CleanupExpiredRequestCaptureData(context.Background(), options)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("request capture cleanup task failed: %v", err))
		return
	}
	if summary.SpoolWarning {
		logger.LogWarn(context.Background(), fmt.Sprintf("request capture spool usage warning: bytes=%d threshold=%d dir=%s", summary.SpoolBytes, summary.SpoolWarnBytes, options.SpoolDir))
	}
	if summary.ExpiredRecords > 0 || summary.DeletedArtifacts > 0 || summary.DeletedSpoolDirs > 0 || summary.Failed > 0 || len(summary.Errors) > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("request capture cleanup: records=%d artifacts=%d objects=%d object_bytes=%d spool_dirs=%d skipped=%d failed=%d errors=%d", summary.ExpiredRecords, summary.DeletedArtifacts, summary.DeletedObjects, summary.DeletedObjectBytes, summary.DeletedSpoolDirs, summary.Skipped, summary.Failed, len(summary.Errors)))
	}
}

func cleanupExpiredRequestCaptureRecords(ctx context.Context, options RequestCaptureCleanupOptions, now int64, summary *RequestCaptureCleanupSummary) error {
	var records []model.RequestCaptureRecord
	err := model.DB.WithContext(ctx).
		Select("id", "request_id", "capture_status", "expires_at", "created_at").
		Where("capture_status NOT IN ?", []string{
			model.RequestCaptureStatusSpooling,
			model.RequestCaptureStatusFinalizing,
			model.RequestCaptureStatusExpired,
			model.RequestCaptureStatusDeleted,
		}).
		Where("(expires_at > 0 AND expires_at <= ?) OR (expires_at = 0 AND created_at > 0 AND created_at <= ?)", now, summary.CutoffTime).
		Order("created_at asc, id asc").
		Limit(options.Limit).
		Find(&records).Error
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		summary.ScannedRecords++
		if options.DryRun {
			summary.ExpiredRecords++
			continue
		}
		updates := map[string]interface{}{
			"capture_status": model.RequestCaptureStatusExpired,
			"expires_at":     requestCaptureCleanupExpiryValue(record.ExpiresAt, now),
			"updated_at":     now,
		}
		result := model.DB.WithContext(ctx).Model(&model.RequestCaptureRecord{}).Where("id = ?", record.Id).Updates(updates)
		if result.Error != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("expire record %s: %s", record.RequestId, result.Error.Error()))
			continue
		}
		if result.RowsAffected == 0 {
			summary.Skipped++
			continue
		}
		summary.ExpiredRecords++
	}
	return nil
}

func cleanupExpiredRequestCaptureArtifacts(ctx context.Context, options RequestCaptureCleanupOptions, now int64, summary *RequestCaptureCleanupSummary) error {
	var artifacts []model.RequestCaptureArtifact
	err := model.DB.WithContext(ctx).
		Select("id", "request_id", "status", "provider", "bucket", "storage_key", "size_bytes", "uploaded_at", "expires_at", "created_at").
		Where("status IN ?", []string{
			model.RequestCaptureArtifactStatusAvailable,
			model.RequestCaptureArtifactStatusFailed,
		}).
		Where("(expires_at > 0 AND expires_at <= ?) OR (expires_at = 0 AND ((uploaded_at > 0 AND uploaded_at <= ?) OR (uploaded_at = 0 AND created_at > 0 AND created_at <= ?)))", now, summary.CutoffTime, summary.CutoffTime).
		Order("uploaded_at asc, created_at asc, id asc").
		Limit(options.Limit).
		Find(&artifacts).Error
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := ctx.Err(); err != nil {
			return err
		}
		summary.ScannedArtifacts++
		if options.DryRun {
			summary.DeletedArtifacts++
			if strings.TrimSpace(artifact.StorageKey) != "" {
				summary.DeletedObjects++
				summary.DeletedObjectBytes += artifact.SizeBytes
			}
			continue
		}
		if strings.TrimSpace(artifact.StorageKey) != "" {
			if !markRequestCaptureArtifactCleanupDeleting(ctx, artifact, now, summary) {
				continue
			}
			if err := DeleteRequestCaptureObject(ctx, artifact.StorageKey); err != nil {
				summary.Failed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("delete capture object %s: %s", artifact.StorageKey, err.Error()))
				recordRequestCaptureArtifactCleanupError(ctx, artifact, err, now)
				continue
			}
			summary.DeletedObjects++
			summary.DeletedObjectBytes += artifact.SizeBytes
		}
		if !markRequestCaptureArtifactDeleted(ctx, artifact, now, summary) {
			continue
		}
		summary.DeletedArtifacts++
	}
	return nil
}

func markRequestCaptureArtifactCleanupDeleting(ctx context.Context, artifact model.RequestCaptureArtifact, now int64, summary *RequestCaptureCleanupSummary) bool {
	updates := map[string]interface{}{
		"status":     model.RequestCaptureArtifactStatusFailed,
		"last_error": requestCaptureArtifactCleanupDeleting,
		"updated_at": now,
	}
	result := model.DB.WithContext(ctx).Model(&model.RequestCaptureArtifact{}).Where("id = ? AND status IN ?", artifact.Id, []string{
		model.RequestCaptureArtifactStatusAvailable,
		model.RequestCaptureArtifactStatusFailed,
	}).Updates(updates)
	if result.Error != nil {
		summary.Failed++
		summary.Errors = append(summary.Errors, fmt.Sprintf("prepare artifact %d cleanup: %s", artifact.Id, result.Error.Error()))
		return false
	}
	if result.RowsAffected == 0 {
		summary.Skipped++
		return false
	}
	return true
}

func markRequestCaptureArtifactDeleted(ctx context.Context, artifact model.RequestCaptureArtifact, now int64, summary *RequestCaptureCleanupSummary) bool {
	updates := map[string]interface{}{
		"status":     model.RequestCaptureArtifactStatusDeleted,
		"deleted_at": now,
		"expires_at": requestCaptureCleanupExpiryValue(artifact.ExpiresAt, now),
		"last_error": "",
		"updated_at": now,
	}
	result := model.DB.WithContext(ctx).Model(&model.RequestCaptureArtifact{}).Where("id = ?", artifact.Id).Updates(updates)
	if result.Error != nil {
		summary.Failed++
		summary.Errors = append(summary.Errors, fmt.Sprintf("mark artifact %d deleted: %s", artifact.Id, result.Error.Error()))
		return false
	}
	if result.RowsAffected == 0 {
		summary.Skipped++
		return false
	}
	return true
}

func recordRequestCaptureArtifactCleanupError(ctx context.Context, artifact model.RequestCaptureArtifact, cause error, now int64) {
	if cause == nil || model.DB == nil {
		return
	}
	updates := map[string]interface{}{
		"status":     model.RequestCaptureArtifactStatusFailed,
		"last_error": truncateRequestCaptureError(cause.Error()),
		"updated_at": now,
	}
	_ = model.DB.WithContext(ctx).Model(&model.RequestCaptureArtifact{}).Where("id = ?", artifact.Id).Updates(updates).Error
}

func cleanupRequestCaptureSpoolDirs(ctx context.Context, options RequestCaptureCleanupOptions, now int64, summary *RequestCaptureCleanupSummary) error {
	spoolDir := strings.TrimSpace(options.SpoolDir)
	if spoolDir == "" {
		spoolDir = requestCaptureDefaultSpoolDir
	}
	if err := cleanupRequestCaptureFailedSpoolDirs(ctx, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed), options, summary); err != nil {
		return err
	}
	if err := cleanupRequestCaptureFinalizedSpoolDirs(ctx, filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize), options, now, summary); err != nil {
		return err
	}
	return nil
}

func cleanupRequestCaptureFailedSpoolDirs(ctx context.Context, root string, options RequestCaptureCleanupOptions, summary *RequestCaptureCleanupSummary) error {
	return cleanupRequestCaptureSpoolStatusDirs(ctx, root, options, summary, func(manifest RequestCaptureSpoolManifest) bool {
		return true
	})
}

func cleanupRequestCaptureFinalizedSpoolDirs(ctx context.Context, root string, options RequestCaptureCleanupOptions, now int64, summary *RequestCaptureCleanupSummary) error {
	return cleanupRequestCaptureSpoolStatusDirs(ctx, root, options, summary, func(manifest RequestCaptureSpoolManifest) bool {
		if strings.TrimSpace(manifest.RequestId) == "" || model.DB == nil {
			return true
		}
		decision, err := requestCaptureFinalizeDecision(ctx, manifest.RequestId, now)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("spool finalize decision %s: %s", manifest.RequestId, err.Error()))
			return false
		}
		return decision.skip && decision.cleanup
	})
}

func cleanupRequestCaptureSpoolStatusDirs(ctx context.Context, root string, options RequestCaptureCleanupOptions, summary *RequestCaptureCleanupSummary, shouldDelete func(RequestCaptureSpoolManifest) bool) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if summary.ScannedSpoolDirs >= options.Limit {
			break
		}
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
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("read spool manifest %s: %s", entry.Name(), err.Error()))
			continue
		}
		summary.ScannedSpoolDirs++
		if requestCaptureSpoolManifestAgeTime(manifest) > summary.SpoolCutoffTime {
			summary.Skipped++
			continue
		}
		if shouldDelete != nil && !shouldDelete(manifest) {
			summary.Skipped++
			continue
		}
		if options.DryRun {
			summary.DeletedSpoolDirs++
			continue
		}
		if err := os.RemoveAll(sessionDir); err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("remove spool dir %s: %s", sessionDir, err.Error()))
			continue
		}
		summary.DeletedSpoolDirs++
	}
	return nil
}

func normalizeRequestCaptureCleanupOptions(options RequestCaptureCleanupOptions) RequestCaptureCleanupOptions {
	if strings.TrimSpace(options.SpoolDir) == "" {
		options.SpoolDir = requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir)
	}
	if options.RetentionDays < 0 {
		options.RetentionDays = requestCaptureDefaultRetentionDays
	}
	if options.SpoolRetentionDays < 0 {
		options.SpoolRetentionDays = requestCaptureDefaultSpoolRetentionDays
	}
	if options.SpoolWarnBytes < 0 {
		options.SpoolWarnBytes = 0
	}
	options.Limit = normalizeRequestCaptureCleanupLimit(options.Limit)
	return options
}

func normalizeRequestCaptureCleanupLimit(limit int) int {
	if limit <= 0 {
		return requestCaptureDefaultCleanupLimit
	}
	if limit > requestCaptureMaxCleanupLimit {
		return requestCaptureMaxCleanupLimit
	}
	return limit
}

func requestCaptureCleanupNow(options RequestCaptureCleanupOptions) int64 {
	if options.Now != nil {
		return options.Now()
	}
	return common.GetTimestamp()
}

func requestCaptureCleanupExpiryValue(current int64, now int64) int64 {
	if current > 0 {
		return current
	}
	return now
}

func requestCaptureSpoolManifestAgeTime(manifest RequestCaptureSpoolManifest) int64 {
	if manifest.FinishedAt > 0 {
		return manifest.FinishedAt
	}
	if manifest.UpdatedAt > 0 {
		return manifest.UpdatedAt
	}
	return manifest.CreatedAt
}

func measureRequestCaptureDirectoryBytes(root string) (int64, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, nil
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var total int64
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry == nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	}); err != nil {
		return 0, err
	}
	return total, nil
}
