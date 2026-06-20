package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	EnterpriseQuotaCounterReconciliationDefaultLimit = 100
	EnterpriseQuotaCounterReconciliationMaxLimit     = 1000

	enterpriseQuotaCounterReconciliationTickInterval = 5 * time.Minute
)

const (
	EnterpriseQuotaCounterReconciliationStatusMatched      = "matched"
	EnterpriseQuotaCounterReconciliationStatusMissingRedis = "missing_redis"
	EnterpriseQuotaCounterReconciliationStatusMissingDB    = "missing_db"
	EnterpriseQuotaCounterReconciliationStatusMismatched   = "mismatched"
	EnterpriseQuotaCounterReconciliationStatusRepaired     = "repaired"
	EnterpriseQuotaCounterReconciliationStatusCreatedDB    = "created_db"
	EnterpriseQuotaCounterReconciliationStatusError        = "error"
)

type EnterpriseQuotaCounterReconciliationParams struct {
	EnterpriseId        int
	Limit               int
	Repair              bool
	IncludeRedisOrphans bool
	ActorUserId         int
	RequestId           string
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
	Enabled             bool                                       `json:"enabled"`
	Repair              bool                                       `json:"repair"`
	IncludeRedisOrphans bool                                       `json:"include_redis_orphans"`
	Limit               int                                        `json:"limit"`
	Scanned             int                                        `json:"scanned"`
	Matched             int                                        `json:"matched"`
	Mismatched          int                                        `json:"mismatched"`
	RedisOnly           int                                        `json:"redis_only"`
	Created             int                                        `json:"created"`
	Repaired            int                                        `json:"repaired"`
	Errors              int                                        `json:"errors"`
	HasMore             bool                                       `json:"has_more"`
	Items               []EnterpriseQuotaCounterReconciliationItem `json:"items"`
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
		Limit:               EnterpriseQuotaCounterReconciliationDefaultLimit,
		Repair:              true,
		IncludeRedisOrphans: true,
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
		Enabled:             enterpriseQuotaCounterBackendAvailable(),
		Repair:              params.Repair,
		IncludeRedisOrphans: params.IncludeRedisOrphans,
		Limit:               limit,
		Items:               []EnterpriseQuotaCounterReconciliationItem{},
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

	if params.IncludeRedisOrphans && result.Scanned < limit {
		hasMore, err := appendEnterpriseQuotaRedisOnlyCounterReconciliationItems(ctx, params, &result, limit-result.Scanned)
		if err != nil {
			return result, err
		}
		if hasMore {
			result.HasMore = true
		}
	}

	return result, nil
}

type enterpriseQuotaCounterRedisKeyParts struct {
	EnterpriseId int
	PolicyId     int
	TargetType   string
	TargetId     int
	Metric       string
	PeriodStart  int64
}

func appendEnterpriseQuotaRedisOnlyCounterReconciliationItems(ctx context.Context, params EnterpriseQuotaCounterReconciliationParams, result *EnterpriseQuotaCounterReconciliationResult, limit int) (bool, error) {
	keys, hasMore, err := enterpriseQuotaCounterBackend.ScanKeys(ctx, enterpriseQuotaCounterRedisScanPrefix(params.EnterpriseId), limit)
	if err != nil {
		return false, err
	}
	now := common.GetTimestamp()
	for _, key := range keys {
		parts, err := parseEnterpriseQuotaCounterRedisKey(key)
		if err != nil {
			result.Items = append(result.Items, enterpriseQuotaCounterRedisOnlyErrorItem(key, err))
			result.Scanned++
			result.Errors++
			continue
		}
		if params.EnterpriseId > 0 && parts.EnterpriseId != params.EnterpriseId {
			continue
		}

		exists, err := enterpriseQuotaCounterDBMirrorExists(parts)
		if err != nil {
			return false, err
		}
		if exists {
			continue
		}

		_, periodEnd, err := enterpriseQuotaCounterPolicyForRedisKey(parts)
		if err != nil {
			result.Items = append(result.Items, enterpriseQuotaCounterRedisOnlyErrorItem(key, err))
			result.Scanned++
			result.Errors++
			continue
		}
		if periodEnd.Unix() < now {
			continue
		}

		redisSnapshot, found, err := enterpriseQuotaCounterBackend.Snapshot(ctx, key)
		if err != nil {
			result.Items = append(result.Items, enterpriseQuotaCounterRedisOnlyErrorItem(key, err))
			result.Scanned++
			result.Errors++
			continue
		}
		if !found {
			continue
		}

		item := buildEnterpriseQuotaRedisOnlyReconciliationItem(parts, periodEnd, key, redisSnapshot)
		result.Mismatched++
		result.RedisOnly++
		if params.Repair {
			counter, err := createEnterpriseQuotaCounterDBMirrorFromRedis(parts, periodEnd, redisSnapshot)
			if err != nil {
				item.Status = EnterpriseQuotaCounterReconciliationStatusError
				item.Error = err.Error()
				result.Errors++
			} else {
				item.CounterId = counter.Id
				after := enterpriseQuotaCounterReconciliationSnapshot(redisSnapshot)
				item.DBSnapshot = after
				item.AfterSnapshot = &after
				item.Status = EnterpriseQuotaCounterReconciliationStatusCreatedDB
				item.Repaired = true
				result.Created++
				result.Repaired++
				recordEnterpriseQuotaCounterRedisOnlyRecoveryAudit(params, counter, item)
			}
		}
		result.Items = append(result.Items, item)
		result.Scanned++
		if result.Scanned >= result.Limit {
			return true, nil
		}
	}
	return hasMore, nil
}

func parseEnterpriseQuotaCounterRedisKey(key string) (enterpriseQuotaCounterRedisKeyParts, error) {
	parts := strings.Split(key, ":")
	if len(parts) != 8 || parts[0]+":"+parts[1] != enterpriseQuotaCounterRedisKeyPrefix() {
		return enterpriseQuotaCounterRedisKeyParts{}, fmt.Errorf("invalid enterprise quota counter redis key: %s", key)
	}
	enterpriseId, err := strconv.Atoi(parts[2])
	if err != nil {
		return enterpriseQuotaCounterRedisKeyParts{}, fmt.Errorf("invalid enterprise id in redis key %s: %w", key, err)
	}
	policyId, err := strconv.Atoi(parts[3])
	if err != nil {
		return enterpriseQuotaCounterRedisKeyParts{}, fmt.Errorf("invalid policy id in redis key %s: %w", key, err)
	}
	targetId, err := strconv.Atoi(parts[5])
	if err != nil {
		return enterpriseQuotaCounterRedisKeyParts{}, fmt.Errorf("invalid target id in redis key %s: %w", key, err)
	}
	periodStart, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		return enterpriseQuotaCounterRedisKeyParts{}, fmt.Errorf("invalid period start in redis key %s: %w", key, err)
	}
	return enterpriseQuotaCounterRedisKeyParts{
		EnterpriseId: enterpriseId,
		PolicyId:     policyId,
		TargetType:   parts[4],
		TargetId:     targetId,
		Metric:       parts[6],
		PeriodStart:  periodStart,
	}, nil
}

func enterpriseQuotaCounterDBMirrorExists(parts enterpriseQuotaCounterRedisKeyParts) (bool, error) {
	var count int64
	err := model.DB.Model(&model.EnterpriseQuotaCounter{}).
		Where("policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period_start = ?",
			parts.PolicyId, parts.TargetType, parts.TargetId, parts.Metric, parts.PeriodStart).
		Count(&count).Error
	return count > 0, err
}

func enterpriseQuotaCounterPolicyForRedisKey(parts enterpriseQuotaCounterRedisKeyParts) (model.EnterpriseQuotaPolicy, time.Time, error) {
	var policy model.EnterpriseQuotaPolicy
	if err := model.DB.Where("id = ? AND enterprise_id = ?", parts.PolicyId, parts.EnterpriseId).First(&policy).Error; err != nil {
		return policy, time.Time{}, err
	}
	if policy.TargetType != parts.TargetType || policy.TargetId != parts.TargetId || policy.Metric != parts.Metric {
		return policy, time.Time{}, fmt.Errorf("redis counter key does not match current policy dimensions: policy_id=%d", policy.Id)
	}
	periodStart := time.Unix(parts.PeriodStart, 0)
	resolvedStart, periodEnd, err := ResolveEnterprisePolicyPeriod(policy, periodStart)
	if err != nil {
		return policy, time.Time{}, err
	}
	if resolvedStart.Unix() != parts.PeriodStart {
		return policy, time.Time{}, fmt.Errorf("redis counter period start does not align with policy period: policy_id=%d period_start=%d resolved_start=%d", policy.Id, parts.PeriodStart, resolvedStart.Unix())
	}
	return policy, periodEnd, nil
}

func buildEnterpriseQuotaRedisOnlyReconciliationItem(parts enterpriseQuotaCounterRedisKeyParts, periodEnd time.Time, redisKey string, redisSnapshot enterpriseQuotaCounterSnapshot) EnterpriseQuotaCounterReconciliationItem {
	snapshot := enterpriseQuotaCounterReconciliationSnapshot(redisSnapshot)
	return EnterpriseQuotaCounterReconciliationItem{
		EnterpriseId:  parts.EnterpriseId,
		PolicyId:      parts.PolicyId,
		TargetType:    parts.TargetType,
		TargetId:      parts.TargetId,
		Metric:        parts.Metric,
		PeriodStart:   parts.PeriodStart,
		PeriodEnd:     periodEnd.Unix(),
		RedisKey:      redisKey,
		RedisFound:    true,
		DBSnapshot:    EnterpriseQuotaCounterReconciliationSnapshot{},
		RedisSnapshot: &snapshot,
		Status:        EnterpriseQuotaCounterReconciliationStatusMissingDB,
		Error:         "",
	}
}

func enterpriseQuotaCounterRedisOnlyErrorItem(redisKey string, err error) EnterpriseQuotaCounterReconciliationItem {
	return EnterpriseQuotaCounterReconciliationItem{
		RedisKey:   redisKey,
		RedisFound: true,
		Status:     EnterpriseQuotaCounterReconciliationStatusError,
		Error:      err.Error(),
	}
}

func createEnterpriseQuotaCounterDBMirrorFromRedis(parts enterpriseQuotaCounterRedisKeyParts, periodEnd time.Time, snapshot enterpriseQuotaCounterSnapshot) (model.EnterpriseQuotaCounter, error) {
	counter := model.EnterpriseQuotaCounter{
		EnterpriseId:  parts.EnterpriseId,
		PolicyId:      parts.PolicyId,
		TargetType:    parts.TargetType,
		TargetId:      parts.TargetId,
		Metric:        parts.Metric,
		PeriodStart:   parts.PeriodStart,
		PeriodEnd:     periodEnd.Unix(),
		UsedValue:     snapshot.UsedValue,
		ReservedValue: snapshot.ReservedValue,
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.EnterpriseQuotaCounter
		err := tx.Where("policy_id = ? AND target_type = ? AND target_id = ? AND metric = ? AND period_start = ?",
			parts.PolicyId, parts.TargetType, parts.TargetId, parts.Metric, parts.PeriodStart).
			First(&existing).Error
		if err == nil {
			counter = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&counter).Error
	})
	return counter, err
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

func recordEnterpriseQuotaCounterRedisOnlyRecoveryAudit(params EnterpriseQuotaCounterReconciliationParams, counter model.EnterpriseQuotaCounter, item EnterpriseQuotaCounterReconciliationItem) {
	before := map[string]any{
		"redis_key":      item.RedisKey,
		"redis_found":    true,
		"db_found":       false,
		"redis_snapshot": item.RedisSnapshot,
		"status":         EnterpriseQuotaCounterReconciliationStatusMissingDB,
	}
	after := map[string]any{
		"redis_key":      item.RedisKey,
		"redis_found":    true,
		"db_found":       true,
		"db_snapshot":    item.AfterSnapshot,
		"redis_snapshot": item.RedisSnapshot,
		"status":         EnterpriseQuotaCounterReconciliationStatusCreatedDB,
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
