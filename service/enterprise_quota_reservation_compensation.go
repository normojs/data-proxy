package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	EnterpriseQuotaReservationCompensationDefaultLimit       = 100
	EnterpriseQuotaReservationCompensationMaxLimit           = 1000
	EnterpriseQuotaReservationCompensationDefaultStaleSecond = 1800
	enterpriseQuotaReservationCompensationTickInterval       = time.Minute
	enterpriseQuotaReservationCompensationStaleSecondsEnv    = "ENTERPRISE_QUOTA_RESERVATION_STALE_SECONDS"
)

type EnterpriseQuotaReservationCompensationParams struct {
	EnterpriseId      int
	Limit             int
	StaleAfterSeconds int64
	DryRun            bool
	ActorUserId       int
	RequestId         string
	Now               time.Time
}

type EnterpriseQuotaReservationCompensationItem struct {
	EventId        int    `json:"event_id"`
	EnterpriseId   int    `json:"enterprise_id"`
	RequestId      string `json:"request_id"`
	PolicyId       int    `json:"policy_id"`
	CounterId      int    `json:"counter_id"`
	Metric         string `json:"metric"`
	ReservedValue  int64  `json:"reserved_value"`
	BeforeReserved int64  `json:"before_reserved"`
	AfterReserved  int64  `json:"after_reserved"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
}

type EnterpriseQuotaReservationCompensationResult struct {
	DryRun            bool                                         `json:"dry_run"`
	Limit             int                                          `json:"limit"`
	StaleAfterSeconds int64                                        `json:"stale_after_seconds"`
	Cutoff            int64                                        `json:"cutoff"`
	Scanned           int                                          `json:"scanned"`
	Compensated       int                                          `json:"compensated"`
	Failed            int                                          `json:"failed"`
	HasMore           bool                                         `json:"has_more"`
	Items             []EnterpriseQuotaReservationCompensationItem `json:"items"`
}

var (
	enterpriseQuotaReservationCompensationOnce    sync.Once
	enterpriseQuotaReservationCompensationRunning atomic.Bool
)

func StartEnterpriseQuotaReservationCompensationTask() {
	enterpriseQuotaReservationCompensationOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("enterprise quota reservation compensation task started: tick=%s", enterpriseQuotaReservationCompensationTickInterval))
			ticker := time.NewTicker(enterpriseQuotaReservationCompensationTickInterval)
			defer ticker.Stop()

			runEnterpriseQuotaReservationCompensationOnce()
			for range ticker.C {
				runEnterpriseQuotaReservationCompensationOnce()
			}
		})
	})
}

func runEnterpriseQuotaReservationCompensationOnce() {
	if !enterpriseQuotaReservationCompensationRunning.CompareAndSwap(false, true) {
		return
	}
	defer enterpriseQuotaReservationCompensationRunning.Store(false)

	result, err := CompensateStaleEnterpriseQuotaReservations(EnterpriseQuotaReservationCompensationParams{
		Limit:             EnterpriseQuotaReservationCompensationDefaultLimit,
		StaleAfterSeconds: enterpriseQuotaReservationCompensationStaleSeconds(),
	})
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("enterprise quota reservation compensation task failed: %v", err))
		return
	}
	if common.DebugEnabled && (result.Compensated > 0 || result.Failed > 0) {
		logger.LogDebug(context.Background(), "enterprise quota reservation compensation task: scanned=%d, compensated=%d, failed=%d, has_more=%v", result.Scanned, result.Compensated, result.Failed, result.HasMore)
	}
}

func CompensateStaleEnterpriseQuotaReservations(params EnterpriseQuotaReservationCompensationParams) (EnterpriseQuotaReservationCompensationResult, error) {
	limit := normalizeEnterpriseQuotaReservationCompensationLimit(params.Limit)
	staleAfter := params.StaleAfterSeconds
	if staleAfter <= 0 {
		staleAfter = enterpriseQuotaReservationCompensationStaleSeconds()
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-time.Duration(staleAfter) * time.Second).Unix()
	result := EnterpriseQuotaReservationCompensationResult{
		DryRun:            params.DryRun,
		Limit:             limit,
		StaleAfterSeconds: staleAfter,
		Cutoff:            cutoff,
		Items:             []EnterpriseQuotaReservationCompensationItem{},
	}

	var events []model.EnterpriseQuotaReservationEvent
	query := model.DB.Where("status = ? AND created_at <= ?", model.EnterpriseQuotaReservationStatusReserved, cutoff).
		Order("id asc").
		Limit(limit + 1)
	if params.EnterpriseId > 0 {
		query = query.Where("enterprise_id = ?", params.EnterpriseId)
	}
	if err := query.Find(&events).Error; err != nil {
		return result, err
	}
	if len(events) > limit {
		result.HasMore = true
		events = events[:limit]
	}
	result.Scanned = len(events)
	for _, event := range events {
		item := EnterpriseQuotaReservationCompensationItem{
			EventId:       event.Id,
			EnterpriseId:  event.EnterpriseId,
			RequestId:     event.RequestId,
			PolicyId:      event.PolicyId,
			CounterId:     event.CounterId,
			Metric:        event.Metric,
			ReservedValue: event.ReservedValue,
			Status:        model.EnterpriseQuotaReservationStatusReserved,
		}
		if params.DryRun {
			result.Items = append(result.Items, item)
			continue
		}
		compensated, err := compensateEnterpriseQuotaReservationEvent(event.Id, now.Unix(), params.ActorUserId, params.RequestId, &item)
		if err != nil {
			item.Status = model.EnterpriseQuotaReservationStatusFailed
			item.Error = err.Error()
			result.Failed++
		} else if compensated {
			result.Compensated++
		} else if item.Status == model.EnterpriseQuotaReservationStatusFailed {
			result.Failed++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func createEnterpriseQuotaReservationEventWithDB(tx *gorm.DB, reservation *Reservation, policy model.EnterpriseQuotaPolicy, counterId int, reservedValue int64, redisKey string, redisTTL time.Duration) error {
	if tx == nil || reservation == nil || reservedValue <= 0 {
		return nil
	}
	ttlSeconds := int64(0)
	if redisTTL > 0 {
		ttlSeconds = int64(redisTTL.Seconds())
	}
	event := model.EnterpriseQuotaReservationEvent{
		EnterpriseId:           reservation.EnterpriseId,
		RequestId:              reservation.RequestId,
		UserId:                 reservation.UserId,
		PolicyId:               policy.Id,
		CounterId:              counterId,
		Metric:                 policy.Metric,
		ReservedValue:          reservedValue,
		Status:                 model.EnterpriseQuotaReservationStatusReserved,
		RedisCounterKey:        redisKey,
		RedisCounterTTLSeconds: ttlSeconds,
		ReservedAt:             common.GetTimestamp(),
	}
	if err := tx.Create(&event).Error; err != nil {
		return err
	}
	if reservation.EventIds == nil {
		reservation.EventIds = map[int]int{}
	}
	reservation.EventIds[policy.Id] = event.Id
	return nil
}

func markEnterpriseQuotaReservationSettledWithDB(tx *gorm.DB, reservation *Reservation, policyId int, actualValue int64) error {
	if tx == nil || reservation == nil || reservation.EventIds == nil {
		return nil
	}
	eventId := reservation.EventIds[policyId]
	if eventId <= 0 {
		return nil
	}
	return tx.Model(&model.EnterpriseQuotaReservationEvent{}).
		Where("id = ? AND status = ?", eventId, model.EnterpriseQuotaReservationStatusReserved).
		Updates(map[string]any{
			"status":       model.EnterpriseQuotaReservationStatusSettled,
			"actual_value": actualValue,
			"settled_at":   common.GetTimestamp(),
		}).Error
}

func markEnterpriseQuotaReservationRefundedWithDB(tx *gorm.DB, reservation *Reservation, policyId int) error {
	if tx == nil || reservation == nil || reservation.EventIds == nil {
		return nil
	}
	eventId := reservation.EventIds[policyId]
	if eventId <= 0 {
		return nil
	}
	return tx.Model(&model.EnterpriseQuotaReservationEvent{}).
		Where("id = ? AND status = ?", eventId, model.EnterpriseQuotaReservationStatusReserved).
		Updates(map[string]any{
			"status":      model.EnterpriseQuotaReservationStatusRefunded,
			"refunded_at": common.GetTimestamp(),
		}).Error
}

func compensateEnterpriseQuotaReservationEvent(eventId int, now int64, actorUserId int, requestId string, item *EnterpriseQuotaReservationCompensationItem) (bool, error) {
	var event model.EnterpriseQuotaReservationEvent
	var counter model.EnterpriseQuotaCounter
	compensated := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, eventId).Error; err != nil {
			return err
		}
		if event.Status != model.EnterpriseQuotaReservationStatusReserved {
			if item != nil {
				item.Status = event.Status
			}
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&counter, event.CounterId).Error; err != nil {
			message := err.Error()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				message = "quota counter not found"
			}
			if updateErr := markEnterpriseQuotaReservationFailedWithDB(tx, event.Id, message); updateErr != nil {
				return updateErr
			}
			if item != nil {
				item.Status = model.EnterpriseQuotaReservationStatusFailed
				item.Error = message
			}
			return nil
		}
		before := counter.ReservedValue
		counter.ReservedValue -= event.ReservedValue
		if counter.ReservedValue < 0 {
			counter.ReservedValue = 0
		}
		if err := tx.Save(&counter).Error; err != nil {
			return err
		}
		event.Status = model.EnterpriseQuotaReservationStatusCompensated
		event.RefundedAt = now
		event.ErrorMessage = "stale reservation compensated"
		if err := tx.Save(&event).Error; err != nil {
			return err
		}
		if _, err := model.RecordEnterpriseAuditLogWithDB(tx, model.EnterpriseAuditInput{
			EnterpriseId: event.EnterpriseId,
			ActorUserId:  actorUserId,
			Action:       "enterprise_quota_reservation.compensate",
			TargetType:   "quota_counter",
			TargetId:     event.CounterId,
			Before: map[string]any{
				"event_id":       event.Id,
				"request_id":     event.RequestId,
				"reserved_value": before,
			},
			After: map[string]any{
				"event_id":       event.Id,
				"request_id":     event.RequestId,
				"reserved_value": counter.ReservedValue,
				"released_value": event.ReservedValue,
			},
			RequestId: requestId,
		}); err != nil {
			return err
		}
		if item != nil {
			item.BeforeReserved = before
			item.AfterReserved = counter.ReservedValue
			item.Status = model.EnterpriseQuotaReservationStatusCompensated
		}
		compensated = true
		return nil
	})
	if err != nil || !compensated {
		return compensated, err
	}
	refundRedisEnterpriseQuotaReservationEvent(event)
	return true, nil
}

func markEnterpriseQuotaReservationFailedWithDB(tx *gorm.DB, eventId int, message string) error {
	return tx.Model(&model.EnterpriseQuotaReservationEvent{}).
		Where("id = ? AND status = ?", eventId, model.EnterpriseQuotaReservationStatusReserved).
		Updates(map[string]any{
			"status":        model.EnterpriseQuotaReservationStatusFailed,
			"error_message": message,
		}).Error
}

func refundRedisEnterpriseQuotaReservationEvent(event model.EnterpriseQuotaReservationEvent) {
	if event.RedisCounterKey == "" || !enterpriseQuotaCounterBackendAvailable() {
		return
	}
	ttl := time.Duration(event.RedisCounterTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	if err := enterpriseQuotaCounterBackend.Refund(context.Background(), event.RedisCounterKey, event.ReservedValue, ttl); err != nil {
		common.SysLog("enterprise quota reservation compensation redis refund failed: " + err.Error())
	}
}

func normalizeEnterpriseQuotaReservationCompensationLimit(limit int) int {
	if limit <= 0 {
		return EnterpriseQuotaReservationCompensationDefaultLimit
	}
	if limit > EnterpriseQuotaReservationCompensationMaxLimit {
		return EnterpriseQuotaReservationCompensationMaxLimit
	}
	return limit
}

func enterpriseQuotaReservationCompensationStaleSeconds() int64 {
	return int64(common.GetEnvOrDefault(enterpriseQuotaReservationCompensationStaleSecondsEnv, EnterpriseQuotaReservationCompensationDefaultStaleSecond))
}
