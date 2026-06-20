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

const enterpriseQuotaRequestMaintenanceTickInterval = 1 * time.Minute

var (
	enterpriseQuotaRequestMaintenanceOnce    sync.Once
	enterpriseQuotaRequestMaintenanceRunning atomic.Bool
)

func StartEnterpriseQuotaRequestMaintenanceTask() {
	enterpriseQuotaRequestMaintenanceOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise quota request maintenance task started: tick=%s", enterpriseQuotaRequestMaintenanceTickInterval))
			ticker := time.NewTicker(enterpriseQuotaRequestMaintenanceTickInterval)
			defer ticker.Stop()

			runEnterpriseQuotaRequestMaintenanceOnce()
			for range ticker.C {
				runEnterpriseQuotaRequestMaintenanceOnce()
			}
		})
	})
}

func runEnterpriseQuotaRequestMaintenanceOnce() {
	if !enterpriseQuotaRequestMaintenanceRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseQuotaRequestMaintenanceRunning.Store(false)

	ctx := context.Background()
	now := common.GetTimestamp()
	reminderCount, err := EnqueueExpiringSoonEnterpriseQuotaRequestOutbox(now, EnterpriseQuotaRequestExpiryBatchSize)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("enterprise quota request expiring soon outbox task failed: %v", err))
		return
	}
	totalExpired := 0
	for {
		n, err := ExpireDueEnterpriseQuotaRequests(now, EnterpriseQuotaRequestExpiryBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("enterprise quota request expiry task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalExpired += n
		if n < EnterpriseQuotaRequestExpiryBatchSize {
			break
		}
	}
	if common.DebugEnabled && reminderCount > 0 {
		logger.LogDebug(ctx, "enterprise quota request maintenance: expiring_soon_outbox_count=%d", reminderCount)
	}
	if common.DebugEnabled && totalExpired > 0 {
		logger.LogDebug(ctx, "enterprise quota request maintenance: expired_count=%d", totalExpired)
	}
}
