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
	EnterpriseQuotaCounterReconciliationDefaultLimit = 100
	EnterpriseQuotaCounterReconciliationMaxLimit     = 1000

	enterpriseQuotaCounterReconciliationTickInterval = 5 * time.Minute
)

const (
	EnterpriseQuotaCounterReconciliationStatusMatched      = "matched"
	EnterpriseQuotaCounterReconciliationStatusMissingRedis = "missing_redis"
	EnterpriseQuotaCounterReconciliationStatusMismatched   = "mismatched"
	EnterpriseQuotaCounterReconciliationStatusRepaired     = "repaired"
	EnterpriseQuotaCounterReconciliationStatusError        = "error"
)

type EnterpriseQuotaCounterReconciliationParams struct {
	EnterpriseId int
	Limit        int
	Repair       bool
	ActorUserId  int
	RequestId    string
}

type EnterpriseQuotaCounterReconciliationSnapshot struct {
	UsedValue     int64 `json:"used_value"`
	ReservedValue int64 `json:"reserved_value"`
}

type EnterpriseQuotaCounterReconciliationItem struct {
	CounterId     int                                           `json:"counter_id"`
	EnterpriseId  int                                           `json:"enterprise_id"`
	PolicyId      int                                           `json:"policy_id"`
	TargetType    string                                        `json:"target_type"`
	TargetId      int                                           `json:"target_id"`
	Metric        string                                        `json:"metric"`
	PeriodStart   int64                                         `json:"period_start"`
	PeriodEnd     int64                                         `json:"period_end"`
	RedisKey      string                                        `json:"redis_key"`
	RedisFound    bool                                          `json:"redis_found"`
	DBSnapshot    EnterpriseQuotaCounterReconciliationSnapshot  `json:"db_snapshot"`
	RedisSnapshot *EnterpriseQuotaCounterReconciliationSnapshot `json:"redis_snapshot,omitempty"`
	AfterSnapshot *EnterpriseQuotaCounterReconciliationSnapshot `json:"after_snapshot,omitempty"`
	Status        string                                        `json:"status"`
	Repaired      bool                                          `json:"repaired"`
	Error         string                                        `json:"error,omitempty"`
}

type EnterpriseQuotaCounterReconciliationResult struct {
	Enabled    bool                                       `json:"enabled"`
	Repair     bool                                       `json:"repair"`
	Limit      int                                        `json:"limit"`
	Scanned    int                                        `json:"scanned"`
	Matched    int                                        `json:"matched"`
	Mismatched int                                        `json:"mismatched"`
	Repaired   int                                        `json:"repaired"`
	Errors     int                                        `json:"errors"`
	HasMore    bool                                       `json:"has_more"`
	Items      []EnterpriseQuotaCounterReconciliationItem `json:"items"`
}

var (
	enterpriseQuotaCounterReconciliationOnce    sync.Once
	enterpriseQuotaCounterReconciliationRunning atomic.Bool
)

func StartEnterpriseQuotaCounterReconciliationTask() {
	enterpriseQuotaCounterReconciliationOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise quota counter reconciliation task started: tick=%s", enterpriseQuotaCounterReconciliationTickInterval))
			ticker := time.NewTicker(enterpriseQuotaCounterReconciliationTickInterval)
			defer ticker.Stop()

			runEnterpriseQuotaCounterReconciliationOnce()
			for range ticker.C {
				runEnterpriseQuotaCounterReconciliationOnce()
			}
		})
	})
}

func runEnterpriseQuotaCounterReconciliationOnce() {
	if !common.EnterpriseQuotaRedisCounterEnabled || !enterpriseQuotaCounterBackendAvailable() {
		return
	}
	if !enterpriseQuotaCounterReconciliationRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseQuotaCounterReconciliationRunning.Store(false)

	result, err := ReconcileEnterpriseQuotaRedisCounters(EnterpriseQuotaCounterReconciliationParams{
		Limit:  EnterpriseQuotaCounterReconciliationDefaultLimit,
		Repair: true,
	})
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("enterprise quota counter reconciliation task failed: %v", err))
		return
	}
	if common.DebugEnabled && (result.Mismatched > 0 || result.Repaired > 0 || result.Errors > 0) {
		logger.LogDebug(context.Background(), "enterprise quota counter reconciliation task: scanned=%d, mismatched=%d, repaired=%d, errors=%d, has_more=%v", result.Scanned, result.Mismatched, result.Repaired, result.Errors, result.HasMore)
	}
}

func ReconcileEnterpriseQuotaRedisCounters(params EnterpriseQuotaCounterReconciliationParams) (EnterpriseQuotaCounterReconciliationResult, error) {
	limit := normalizeEnterpriseQuotaCounterReconciliationLimit(params.Limit)
	result := EnterpriseQuotaCounterReconciliationResult{
		Enabled: enterpriseQuotaCounterBackendAvailable(),
		Repair:  params.Repair,
		Limit:   limit,
		Items:   []EnterpriseQuotaCounterReconciliationItem{},
	}
	if !result.Enabled {
		return result, fmt.Errorf("enterprise quota redis counter backend is not available")
	}

	query := model.DB.Where("period_end >= ?", common.GetTimestamp())
	if params.EnterpriseId > 0 {
		query = query.Where("enterprise_id = ?", params.EnterpriseId)
	}
	var counters []model.EnterpriseQuotaCounter
	if err := query.Order("id asc").Limit(limit + 1).Find(&counters).Error; err != nil {
		return result, err
	}
	if len(counters) > limit {
		result.HasMore = true
		counters = counters[:limit]
	}

	ctx := context.Background()
	for _, counter := range counters {
		item := buildEnterpriseQuotaCounterReconciliationItem(counter)
		redisSnapshot, found, err := enterpriseQuotaCounterBackend.Snapshot(ctx, item.RedisKey)
		if err != nil {
			item.Status = EnterpriseQuotaCounterReconciliationStatusError
			item.Error = err.Error()
			result.Errors++
			result.Items = append(result.Items, item)
			result.Scanned++
			continue
		}
		item.RedisFound = found
		if found {
			snapshot := enterpriseQuotaCounterReconciliationSnapshot(redisSnapshot)
			item.RedisSnapshot = &snapshot
		}

		dbSnapshot := enterpriseQuotaCounterSnapshot{UsedValue: counter.UsedValue, ReservedValue: counter.ReservedValue}
		if enterpriseQuotaCounterSnapshotsEqual(dbSnapshot, redisSnapshot) || (!found && enterpriseQuotaCounterSnapshotEmpty(dbSnapshot)) {
			item.Status = EnterpriseQuotaCounterReconciliationStatusMatched
			result.Matched++
			result.Items = append(result.Items, item)
			result.Scanned++
			continue
		}

		if found {
			item.Status = EnterpriseQuotaCounterReconciliationStatusMismatched
		} else {
			item.Status = EnterpriseQuotaCounterReconciliationStatusMissingRedis
		}
		result.Mismatched++

		if params.Repair {
			beforeStatus := item.Status
			ttl := enterpriseQuotaCounterRedisTTL(time.Unix(counter.PeriodEnd, 0))
			if err := enterpriseQuotaCounterBackend.SetSnapshot(ctx, item.RedisKey, dbSnapshot, ttl); err != nil {
				item.Status = EnterpriseQuotaCounterReconciliationStatusError
				item.Error = err.Error()
				result.Errors++
			} else {
				after := enterpriseQuotaCounterReconciliationSnapshot(dbSnapshot)
				item.AfterSnapshot = &after
				item.Status = EnterpriseQuotaCounterReconciliationStatusRepaired
				item.Repaired = true
				result.Repaired++
				recordEnterpriseQuotaCounterReconciliationAudit(params, counter, item, beforeStatus)
			}
		}

		result.Items = append(result.Items, item)
		result.Scanned++
	}

	return result, nil
}

func normalizeEnterpriseQuotaCounterReconciliationLimit(limit int) int {
	if limit <= 0 {
		return EnterpriseQuotaCounterReconciliationDefaultLimit
	}
	if limit > EnterpriseQuotaCounterReconciliationMaxLimit {
		return EnterpriseQuotaCounterReconciliationMaxLimit
	}
	return limit
}

func buildEnterpriseQuotaCounterReconciliationItem(counter model.EnterpriseQuotaCounter) EnterpriseQuotaCounterReconciliationItem {
	return EnterpriseQuotaCounterReconciliationItem{
		CounterId:    counter.Id,
		EnterpriseId: counter.EnterpriseId,
		PolicyId:     counter.PolicyId,
		TargetType:   counter.TargetType,
		TargetId:     counter.TargetId,
		Metric:       counter.Metric,
		PeriodStart:  counter.PeriodStart,
		PeriodEnd:    counter.PeriodEnd,
		RedisKey:     enterpriseQuotaCounterRedisKeyForCounter(counter),
		DBSnapshot: EnterpriseQuotaCounterReconciliationSnapshot{
			UsedValue:     counter.UsedValue,
			ReservedValue: counter.ReservedValue,
		},
	}
}

func enterpriseQuotaCounterReconciliationSnapshot(snapshot enterpriseQuotaCounterSnapshot) EnterpriseQuotaCounterReconciliationSnapshot {
	return EnterpriseQuotaCounterReconciliationSnapshot{
		UsedValue:     snapshot.UsedValue,
		ReservedValue: snapshot.ReservedValue,
	}
}

func enterpriseQuotaCounterSnapshotsEqual(left enterpriseQuotaCounterSnapshot, right enterpriseQuotaCounterSnapshot) bool {
	return left.UsedValue == right.UsedValue && left.ReservedValue == right.ReservedValue
}

func enterpriseQuotaCounterSnapshotEmpty(snapshot enterpriseQuotaCounterSnapshot) bool {
	return snapshot.UsedValue == 0 && snapshot.ReservedValue == 0
}

func recordEnterpriseQuotaCounterReconciliationAudit(params EnterpriseQuotaCounterReconciliationParams, counter model.EnterpriseQuotaCounter, item EnterpriseQuotaCounterReconciliationItem, beforeStatus string) {
	before := map[string]any{
		"redis_key":      item.RedisKey,
		"redis_found":    item.RedisFound,
		"db_snapshot":    item.DBSnapshot,
		"redis_snapshot": item.RedisSnapshot,
		"status":         beforeStatus,
	}
	after := map[string]any{
		"redis_key":      item.RedisKey,
		"redis_found":    true,
		"db_snapshot":    item.DBSnapshot,
		"redis_snapshot": item.AfterSnapshot,
		"status":         EnterpriseQuotaCounterReconciliationStatusRepaired,
	}
	_ = model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: counter.EnterpriseId,
		ActorUserId:  params.ActorUserId,
		Action:       "quota_counter.reconcile",
		TargetType:   "quota_counter",
		TargetId:     counter.Id,
		Before:       before,
		After:        after,
		RequestId:    params.RequestId,
	})
}
