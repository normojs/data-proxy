package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	EnterpriseGovernanceQueuePayloadCleanupDefaultTTLSeconds      = int64(7 * 24 * 3600)
	EnterpriseGovernanceQueuePayloadCleanupDefaultIntervalSeconds = int64(3600)
	EnterpriseGovernanceQueuePayloadCleanupDefaultBatchSize       = 500
	EnterpriseGovernanceQueuePayloadCleanupMaxBatchSize           = 5000

	enterpriseGovernanceQueuePayloadCleanupTTLEnv      = "ENTERPRISE_QUEUE_PAYLOAD_TTL_SECONDS"
	enterpriseGovernanceQueuePayloadCleanupIntervalEnv = "ENTERPRISE_QUEUE_PAYLOAD_CLEANUP_INTERVAL_SECONDS"
	enterpriseGovernanceQueuePayloadCleanupLimitEnv    = "ENTERPRISE_QUEUE_PAYLOAD_CLEANUP_LIMIT"
)

type EnterpriseGovernanceQueuePayloadCleanupStats struct {
	TTLSeconds      int64 `json:"ttl_seconds"`
	CutoffTime      int64 `json:"cutoff_time"`
	DryRun          bool  `json:"dry_run"`
	Scanned         int   `json:"scanned"`
	Deleted         int   `json:"deleted"`
	DeletedReleased int   `json:"deleted_released"`
	DeletedOrphaned int   `json:"deleted_orphaned"`
	DeletedBytes    int64 `json:"deleted_bytes"`
	Skipped         int   `json:"skipped"`
	Errors          int   `json:"errors"`
}

var (
	enterpriseGovernanceQueuePayloadCleanupOnce    sync.Once
	enterpriseGovernanceQueuePayloadCleanupRunning atomic.Bool
)

func StartEnterpriseGovernanceQueuePayloadCleanupTask() {
	enterpriseGovernanceQueuePayloadCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		ttlSeconds := EnterpriseGovernanceQueuePayloadCleanupTTLSeconds()
		if ttlSeconds <= 0 {
			return
		}
		intervalSeconds := EnterpriseGovernanceQueuePayloadCleanupIntervalSeconds()
		if intervalSeconds <= 0 {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(intervalSeconds) * time.Second
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise governance queue payload cleanup task started: ttl=%ds tick=%s", ttlSeconds, interval))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runEnterpriseGovernanceQueuePayloadCleanupOnce()
			for range ticker.C {
				runEnterpriseGovernanceQueuePayloadCleanupOnce()
			}
		})
	})
}

func EnterpriseGovernanceQueuePayloadCleanupTTLSeconds() int64 {
	return int64(common.GetEnvOrDefault(enterpriseGovernanceQueuePayloadCleanupTTLEnv, int(EnterpriseGovernanceQueuePayloadCleanupDefaultTTLSeconds)))
}

func EnterpriseGovernanceQueuePayloadCleanupIntervalSeconds() int64 {
	return int64(common.GetEnvOrDefault(enterpriseGovernanceQueuePayloadCleanupIntervalEnv, int(EnterpriseGovernanceQueuePayloadCleanupDefaultIntervalSeconds)))
}

func EnterpriseGovernanceQueuePayloadCleanupLimit() int {
	return normalizeEnterpriseGovernanceQueuePayloadCleanupBatchSize(common.GetEnvOrDefault(enterpriseGovernanceQueuePayloadCleanupLimitEnv, EnterpriseGovernanceQueuePayloadCleanupDefaultBatchSize))
}

func runEnterpriseGovernanceQueuePayloadCleanupOnce() {
	if !enterpriseGovernanceQueuePayloadCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseGovernanceQueuePayloadCleanupRunning.Store(false)

	stats, err := CleanupEnterpriseGovernanceQueuePayloads(common.GetTimestamp(), EnterpriseGovernanceQueuePayloadCleanupTTLSeconds(), EnterpriseGovernanceQueuePayloadCleanupLimit(), false)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("enterprise governance queue payload cleanup task failed: %v", err))
		return
	}
	if stats.Deleted > 0 || stats.Errors > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("enterprise governance queue payload cleanup: scanned=%d deleted=%d released=%d orphaned=%d bytes=%d skipped=%d errors=%d", stats.Scanned, stats.Deleted, stats.DeletedReleased, stats.DeletedOrphaned, stats.DeletedBytes, stats.Skipped, stats.Errors))
	}
}

func CleanupEnterpriseGovernanceQueuePayloads(now int64, ttlSeconds int64, batchSize int, dryRun bool) (EnterpriseGovernanceQueuePayloadCleanupStats, error) {
	stats := EnterpriseGovernanceQueuePayloadCleanupStats{
		TTLSeconds: ttlSeconds,
		DryRun:     dryRun,
	}
	if ttlSeconds <= 0 {
		return stats, errors.New("ttl_seconds must be greater than 0")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	stats.CutoffTime = now - ttlSeconds
	batchSize = normalizeEnterpriseGovernanceQueuePayloadCleanupBatchSize(batchSize)

	var rows []model.EnterpriseGovernanceQueuePayload
	if err := model.DB.
		Select("id", "admission_id", "request_id", "enterprise_id", "user_id", "token_id", "content_length", "body_bytes", "storage_kind", "created_at", "updated_at").
		Where("created_at <= ?", stats.CutoffTime).
		Order("created_at asc, id asc").
		Limit(batchSize).
		Find(&rows).Error; err != nil {
		return stats, err
	}

	for _, row := range rows {
		stats.Scanned++
		reason, err := enterpriseGovernanceQueuePayloadCleanupReason(row, stats.CutoffTime)
		if err != nil {
			stats.Errors++
			return stats, err
		}
		if reason == "" {
			stats.Skipped++
			continue
		}
		if !dryRun {
			result := model.DB.Where("id = ?", row.Id).Delete(&model.EnterpriseGovernanceQueuePayload{})
			if result.Error != nil {
				stats.Errors++
				return stats, result.Error
			}
			if result.RowsAffected == 0 {
				stats.Skipped++
				continue
			}
		}
		stats.Deleted++
		if reason == "released" {
			stats.DeletedReleased++
		}
		if reason == "orphaned" {
			stats.DeletedOrphaned++
		}
		stats.DeletedBytes += enterpriseGovernanceQueuePayloadCleanupBytes(row)
	}
	return stats, nil
}

func enterpriseGovernanceQueuePayloadCleanupReason(row model.EnterpriseGovernanceQueuePayload, cutoff int64) (string, error) {
	admission, found, err := findEnterpriseGovernanceQueuePayloadCleanupAdmission(row)
	if err != nil {
		return "", err
	}
	if !found {
		return "orphaned", nil
	}
	if admission.Status != enterpriseQueueStatusReleased {
		return "", nil
	}
	releasedAt := admission.ReleasedAt
	if releasedAt <= 0 {
		releasedAt = admission.UpdatedAt
	}
	if releasedAt > 0 && releasedAt <= cutoff {
		return "released", nil
	}
	return "", nil
}

func findEnterpriseGovernanceQueuePayloadCleanupAdmission(row model.EnterpriseGovernanceQueuePayload) (model.EnterpriseGovernanceQueueAdmission, bool, error) {
	var admission model.EnterpriseGovernanceQueueAdmission
	baseQuery := func() *gorm.DB {
		return model.DB.Select("id", "request_id", "status", "released_at", "created_at", "updated_at")
	}
	if row.AdmissionId > 0 {
		err := baseQuery().Where("id = ?", row.AdmissionId).First(&admission).Error
		if err == nil {
			return admission, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return admission, false, err
		}
	}
	requestId := strings.TrimSpace(row.RequestId)
	if requestId == "" {
		return admission, false, nil
	}
	err := baseQuery().Where("request_id = ?", requestId).Order("id desc").First(&admission).Error
	if err == nil {
		return admission, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return admission, false, nil
	}
	return admission, false, err
}

func enterpriseGovernanceQueuePayloadCleanupBytes(row model.EnterpriseGovernanceQueuePayload) int64 {
	if row.BodyBytes > 0 {
		return row.BodyBytes
	}
	if row.ContentLength > 0 {
		return row.ContentLength
	}
	return 0
}

func normalizeEnterpriseGovernanceQueuePayloadCleanupBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return EnterpriseGovernanceQueuePayloadCleanupDefaultBatchSize
	}
	if batchSize > EnterpriseGovernanceQueuePayloadCleanupMaxBatchSize {
		return EnterpriseGovernanceQueuePayloadCleanupMaxBatchSize
	}
	return batchSize
}
