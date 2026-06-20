package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

const enterpriseNotificationOutboxTickInterval = 1 * time.Minute

var (
	enterpriseNotificationOutboxOnce    sync.Once
	enterpriseNotificationOutboxRunning atomic.Bool
)

func StartEnterpriseNotificationOutboxTask() {
	enterpriseNotificationOutboxOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise notification outbox task started: tick=%s", enterpriseNotificationOutboxTickInterval))
			ticker := time.NewTicker(enterpriseNotificationOutboxTickInterval)
			defer ticker.Stop()

			runEnterpriseNotificationOutboxOnce()
			for range ticker.C {
				runEnterpriseNotificationOutboxOnce()
			}
		})
	})
}

func runEnterpriseNotificationOutboxOnce() {
	if !enterpriseNotificationOutboxRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseNotificationOutboxRunning.Store(false)

	ctx := context.Background()
	totalClaimed := 0
	totalSent := 0
	totalFailed := 0
	totalPermanentFailed := 0
	totalDurationMs := int64(0)
	for {
		stats, err := ProcessEnterpriseNotificationOutboxBatchWithStats(EnterpriseNotificationOutboxWorkerBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("enterprise notification outbox task failed: %v", err))
			return
		}
		totalClaimed += stats.Claimed
		totalSent += stats.Sent
		totalFailed += stats.Failed
		totalPermanentFailed += stats.PermanentFailed
		totalDurationMs += stats.DurationMs
		if stats.Claimed < EnterpriseNotificationOutboxWorkerBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalClaimed > 0 {
		logger.LogDebug(ctx, "enterprise notification outbox task: claimed_count=%d, sent_count=%d, failed_count=%d, permanent_failed_count=%d, duration_ms=%d", totalClaimed, totalSent, totalFailed, totalPermanentFailed, totalDurationMs)
	}
}
