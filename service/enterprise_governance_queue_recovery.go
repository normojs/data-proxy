package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	EnterpriseGovernanceQueueRecoveryDefaultBatchSize = 100
	EnterpriseGovernanceQueueRecoveryMaxBatchSize     = 1000

	enterpriseGovernanceQueueRecoveryTickInterval = 1 * time.Minute
)

type EnterpriseGovernanceQueueRecoveryStats struct {
	Scanned  int `json:"scanned"`
	TimedOut int `json:"timed_out"`
	Canceled int `json:"canceled"`
	Errors   int `json:"errors"`
}

var (
	enterpriseGovernanceQueueRecoveryOnce    sync.Once
	enterpriseGovernanceQueueRecoveryRunning atomic.Bool
)

func StartEnterpriseGovernanceQueueRecoveryTask() {
	enterpriseGovernanceQueueRecoveryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise governance queue recovery task started: tick=%s", enterpriseGovernanceQueueRecoveryTickInterval))
			ticker := time.NewTicker(enterpriseGovernanceQueueRecoveryTickInterval)
			defer ticker.Stop()

			runEnterpriseGovernanceQueueRecoveryOnce()
			for range ticker.C {
				runEnterpriseGovernanceQueueRecoveryOnce()
			}
		})
	})
}

func runEnterpriseGovernanceQueueRecoveryOnce() {
	if !enterpriseGovernanceQueueRecoveryRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseGovernanceQueueRecoveryRunning.Store(false)

	stats, err := RecoverStaleEnterpriseGovernanceQueueAdmissions(common.GetTimestamp(), EnterpriseGovernanceQueueRecoveryDefaultBatchSize)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("enterprise governance queue recovery task failed: %v", err))
		return
	}
	if common.DebugEnabled && (stats.TimedOut > 0 || stats.Canceled > 0 || stats.Errors > 0) {
		logger.LogDebug(context.Background(), "enterprise governance queue recovery task: scanned=%d, timed_out=%d, canceled=%d, errors=%d", stats.Scanned, stats.TimedOut, stats.Canceled, stats.Errors)
	}
}

func RecoverStaleEnterpriseGovernanceQueueAdmissions(now int64, batchSize int) (EnterpriseGovernanceQueueRecoveryStats, error) {
	stats := EnterpriseGovernanceQueueRecoveryStats{}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	batchSize = normalizeEnterpriseGovernanceQueueRecoveryBatchSize(batchSize)
	queuedBefore := now - durationSecondsCeil(enterprisePolicyQueueTimeout)
	admittedBefore := now - durationSecondsCeil(enterprisePolicyQueueAdmittedStale)

	var rows []model.EnterpriseGovernanceQueueAdmission
	if err := model.DB.
		Where(
			"(status = ? AND created_at <= ?) OR (status = ? AND admitted_at > 0 AND admitted_at <= ? AND released_at = 0 AND canceled_at = 0)",
			enterpriseQueueStatusQueued,
			queuedBefore,
			enterpriseQueueStatusAdmitted,
			admittedBefore,
		).
		Order("created_at asc, id asc").
		Limit(batchSize).
		Find(&rows).Error; err != nil {
		return stats, err
	}

	for _, row := range rows {
		stats.Scanned++
		before := row
		status, updates := enterpriseGovernanceQueueRecoveryUpdates(row, now)
		if status == "" {
			continue
		}
		result := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).
			Where("id = ? AND status = ?", row.Id, row.Status).
			Updates(updates)
		if result.Error != nil {
			stats.Errors++
			return stats, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		var after model.EnterpriseGovernanceQueueAdmission
		if err := model.DB.Where("id = ?", row.Id).First(&after).Error; err != nil {
			stats.Errors++
			return stats, err
		}
		if err := recordEnterpriseGovernanceQueueRecoveryAudit(before, after); err != nil {
			stats.Errors++
			return stats, err
		}
		if status == enterpriseQueueStatusTimeout {
			stats.TimedOut++
		}
		if status == enterpriseQueueStatusCanceled {
			stats.Canceled++
		}
	}
	return stats, nil
}

func enterpriseGovernanceQueueRecoveryUpdates(row model.EnterpriseGovernanceQueueAdmission, now int64) (string, map[string]any) {
	switch row.Status {
	case enterpriseQueueStatusQueued:
		waitMs := row.WaitMs
		if row.CreatedAt > 0 && now > row.CreatedAt {
			waitMs = maxEnterpriseQueueInt64(waitMs, (now-row.CreatedAt)*1000)
		}
		timeoutMs := row.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = durationMillis(enterprisePolicyQueueTimeout)
		}
		return enterpriseQueueStatusTimeout, map[string]any{
			"status":           enterpriseQueueStatusTimeout,
			"wait_ms":          waitMs,
			"timeout_ms":       timeoutMs,
			"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusTimeout),
			"updated_at":       now,
		}
	case enterpriseQueueStatusAdmitted:
		runMs := row.RunMs
		if row.AdmittedAt > 0 && now > row.AdmittedAt {
			runMs = maxEnterpriseQueueInt64(runMs, (now-row.AdmittedAt)*1000)
		}
		return enterpriseQueueStatusCanceled, map[string]any{
			"status":           enterpriseQueueStatusCanceled,
			"canceled_at":      now,
			"run_ms":           runMs,
			"user_message_key": enterpriseQueueUserMessageKey(enterpriseQueueStatusCanceled),
			"updated_at":       now,
		}
	default:
		return "", nil
	}
}

func recordEnterpriseGovernanceQueueRecoveryAudit(before, after model.EnterpriseGovernanceQueueAdmission) error {
	return model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   after.EnterpriseId,
		Action:         enterpriseGovernanceAuditActionQueueRecovery,
		TargetType:     "enterprise_governance_queue_admission",
		TargetId:       int(after.Id),
		ScopeUserId:    after.UserId,
		ScopeOrgUnitId: after.OrgUnitId,
		ScopeProjectId: after.ProjectId,
		Before:         before,
		After:          after,
		RequestId:      after.RequestId,
	})
}

func normalizeEnterpriseGovernanceQueueRecoveryBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return EnterpriseGovernanceQueueRecoveryDefaultBatchSize
	}
	if batchSize > EnterpriseGovernanceQueueRecoveryMaxBatchSize {
		return EnterpriseGovernanceQueueRecoveryMaxBatchSize
	}
	return batchSize
}

func durationSecondsCeil(duration time.Duration) int64 {
	if duration <= 0 {
		return 1
	}
	seconds := int64(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func maxEnterpriseQueueInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
