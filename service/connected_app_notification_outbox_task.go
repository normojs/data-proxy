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

const connectedAppNotificationOutboxTickInterval = 1 * time.Minute

var (
	connectedAppNotificationOutboxOnce    sync.Once
	connectedAppNotificationOutboxRunning atomic.Bool
)

func StartConnectedAppNotificationOutboxTask() {
	connectedAppNotificationOutboxOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("connected app notification outbox task started: tick=%s", connectedAppNotificationOutboxTickInterval))
			ticker := time.NewTicker(connectedAppNotificationOutboxTickInterval)
			defer ticker.Stop()

			runConnectedAppNotificationOutboxOnce()
			for range ticker.C {
				runConnectedAppNotificationOutboxOnce()
			}
		})
	})
}

func runConnectedAppNotificationOutboxOnce() {
	if !connectedAppNotificationOutboxRunning.CompareAndSwap(false, true) {
		return
	}
	defer connectedAppNotificationOutboxRunning.Store(false)

	ctx := context.Background()
	totalClaimed := 0
	totalSent := 0
	totalFailed := 0
	totalPermanentFailed := 0
	totalDurationMs := int64(0)
	for {
		stats, err := ProcessConnectedAppNotificationOutboxBatchWithStats(ConnectedAppNotificationOutboxWorkerBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("connected app notification outbox task failed: %v", err))
			return
		}
		totalClaimed += stats.Claimed
		totalSent += stats.Sent
		totalFailed += stats.Failed
		totalPermanentFailed += stats.PermanentFailed
		totalDurationMs += stats.DurationMs
		if stats.Claimed < ConnectedAppNotificationOutboxWorkerBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalClaimed > 0 {
		logger.LogDebug(ctx, "connected app notification outbox task: claimed_count=%d, sent_count=%d, failed_count=%d, permanent_failed_count=%d, duration_ms=%d", totalClaimed, totalSent, totalFailed, totalPermanentFailed, totalDurationMs)
	}
}
