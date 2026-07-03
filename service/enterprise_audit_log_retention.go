package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	EnterpriseAuditLogRetentionDefaultDays            = 0
	EnterpriseAuditLogRetentionDefaultIntervalSeconds = 24 * 3600
	EnterpriseAuditLogRetentionDefaultLimit           = 1000
	EnterpriseAuditLogRetentionMaxLimit               = 10000

	enterpriseAuditLogRetentionDaysEnv            = "ENTERPRISE_AUDIT_LOG_RETENTION_DAYS"
	enterpriseAuditLogRetentionIntervalSecondsEnv = "ENTERPRISE_AUDIT_LOG_RETENTION_INTERVAL_SECONDS"
	enterpriseAuditLogRetentionLimitEnv           = "ENTERPRISE_AUDIT_LOG_RETENTION_LIMIT"
	enterpriseAuditLogRetentionDryRunEnv          = "ENTERPRISE_AUDIT_LOG_RETENTION_DRY_RUN"
)

type EnterpriseAuditLogRetentionCleanupParams struct {
	RetentionDays int
	Limit         int
	DryRun        bool
	ActorUserId   int
	RequestId     string
	Now           time.Time
}

type EnterpriseAuditLogRetentionCleanupResult struct {
	DryRun           bool        `json:"dry_run"`
	RetentionDays    int         `json:"retention_days"`
	Limit            int         `json:"limit"`
	Cutoff           int64       `json:"cutoff"`
	Scanned          int         `json:"scanned"`
	WouldDelete      int         `json:"would_delete"`
	Deleted          int         `json:"deleted"`
	HasMore          bool        `json:"has_more"`
	EnterpriseCounts map[int]int `json:"enterprise_counts"`
	DeletedIds       []int64     `json:"deleted_ids,omitempty"`
}

type enterpriseAuditLogRetentionRow struct {
	Id           int64
	EnterpriseId int
	CreatedAt    int64
}

var (
	enterpriseAuditLogRetentionOnce    sync.Once
	enterpriseAuditLogRetentionRunning atomic.Bool
)

func StartEnterpriseAuditLogRetentionTask() {
	enterpriseAuditLogRetentionOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		retentionDays := EnterpriseAuditLogRetentionDays()
		if retentionDays <= 0 {
			return
		}
		intervalSeconds := EnterpriseAuditLogRetentionIntervalSeconds()
		if intervalSeconds <= 0 {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(intervalSeconds) * time.Second
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise audit log retention task started: retention_days=%d tick=%s dry_run=%v", retentionDays, interval, EnterpriseAuditLogRetentionDryRun()))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runEnterpriseAuditLogRetentionOnce()
			for range ticker.C {
				runEnterpriseAuditLogRetentionOnce()
			}
		})
	})
}

func EnterpriseAuditLogRetentionDays() int {
	return common.GetEnvOrDefault(enterpriseAuditLogRetentionDaysEnv, EnterpriseAuditLogRetentionDefaultDays)
}

func EnterpriseAuditLogRetentionIntervalSeconds() int {
	return common.GetEnvOrDefault(enterpriseAuditLogRetentionIntervalSecondsEnv, EnterpriseAuditLogRetentionDefaultIntervalSeconds)
}

func EnterpriseAuditLogRetentionLimit() int {
	return normalizeEnterpriseAuditLogRetentionLimit(common.GetEnvOrDefault(enterpriseAuditLogRetentionLimitEnv, EnterpriseAuditLogRetentionDefaultLimit))
}

func EnterpriseAuditLogRetentionDryRun() bool {
	return common.GetEnvOrDefaultBool(enterpriseAuditLogRetentionDryRunEnv, false)
}

func runEnterpriseAuditLogRetentionOnce() {
	if !enterpriseAuditLogRetentionRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseAuditLogRetentionRunning.Store(false)

	result, err := CleanupEnterpriseAuditLogsByRetention(EnterpriseAuditLogRetentionCleanupParams{
		RetentionDays: EnterpriseAuditLogRetentionDays(),
		Limit:         EnterpriseAuditLogRetentionLimit(),
		DryRun:        EnterpriseAuditLogRetentionDryRun(),
	})
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("enterprise audit log retention task failed: %v", err))
		return
	}
	if result.WouldDelete > 0 || result.Deleted > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("enterprise audit log retention task: scanned=%d would_delete=%d deleted=%d has_more=%v dry_run=%v", result.Scanned, result.WouldDelete, result.Deleted, result.HasMore, result.DryRun))
	}
}

func CleanupEnterpriseAuditLogsByRetention(params EnterpriseAuditLogRetentionCleanupParams) (EnterpriseAuditLogRetentionCleanupResult, error) {
	retentionDays := params.RetentionDays
	if retentionDays <= 0 {
		return EnterpriseAuditLogRetentionCleanupResult{DryRun: params.DryRun, RetentionDays: retentionDays}, errors.New("retention_days must be greater than 0")
	}
	limit := normalizeEnterpriseAuditLogRetentionLimit(params.Limit)
	now := params.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	result := EnterpriseAuditLogRetentionCleanupResult{
		DryRun:           params.DryRun,
		RetentionDays:    retentionDays,
		Limit:            limit,
		Cutoff:           cutoff,
		EnterpriseCounts: map[int]int{},
		DeletedIds:       []int64{},
	}

	var rows []enterpriseAuditLogRetentionRow
	if err := model.DB.Model(&model.EnterpriseAuditLog{}).
		Select("id", "enterprise_id", "created_at").
		Where("created_at <= ?", cutoff).
		Order("created_at asc, id asc").
		Limit(limit + 1).
		Find(&rows).Error; err != nil {
		return result, err
	}
	if len(rows) > limit {
		result.HasMore = true
		rows = rows[:limit]
	}
	result.Scanned = len(rows)
	result.WouldDelete = len(rows)
	if len(rows) == 0 {
		return result, nil
	}

	ids := make([]int64, 0, len(rows))
	oldestByEnterprise := map[int]int64{}
	newestByEnterprise := map[int]int64{}
	for _, row := range rows {
		ids = append(ids, row.Id)
		result.EnterpriseCounts[row.EnterpriseId]++
		if oldestByEnterprise[row.EnterpriseId] == 0 || row.CreatedAt < oldestByEnterprise[row.EnterpriseId] {
			oldestByEnterprise[row.EnterpriseId] = row.CreatedAt
		}
		if row.CreatedAt > newestByEnterprise[row.EnterpriseId] {
			newestByEnterprise[row.EnterpriseId] = row.CreatedAt
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if params.DryRun {
		result.DeletedIds = ids
		return result, nil
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		deleteResult := tx.Where("id IN ?", ids).Delete(&model.EnterpriseAuditLog{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		result.Deleted = int(deleteResult.RowsAffected)
		for enterpriseId, count := range result.EnterpriseCounts {
			if count <= 0 {
				continue
			}
			_, err := model.RecordEnterpriseAuditLogWithDB(tx, model.EnterpriseAuditInput{
				EnterpriseId: enterpriseId,
				ActorUserId:  params.ActorUserId,
				Action:       "audit_log.retention_cleanup",
				TargetType:   "enterprise",
				TargetId:     enterpriseId,
				After: map[string]any{
					"retention_days":    retentionDays,
					"cutoff":            cutoff,
					"deleted_count":     count,
					"oldest_created_at": oldestByEnterprise[enterpriseId],
					"newest_created_at": newestByEnterprise[enterpriseId],
					"limit":             limit,
					"has_more":          result.HasMore,
					"dry_run":           false,
				},
				RequestId: params.RequestId,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	result.DeletedIds = ids
	return result, nil
}

func normalizeEnterpriseAuditLogRetentionLimit(limit int) int {
	if limit <= 0 {
		return EnterpriseAuditLogRetentionDefaultLimit
	}
	if limit > EnterpriseAuditLogRetentionMaxLimit {
		return EnterpriseAuditLogRetentionMaxLimit
	}
	return limit
}
