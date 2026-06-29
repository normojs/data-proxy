package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	subsiteQuotaSettledContextKey     = "subsite_quota_settled"
	subsiteQuotaReservationContextKey = "subsite_quota_reservation"
	subsiteQuotaRefundedContextKey    = "subsite_quota_refunded"

	subsiteQuotaRedisRejectQuota   = int64(1)
	subsiteQuotaRedisRejectRequest = int64(2)

	subsiteQuotaCounterRedisReserveScript = `
local used_quota = tonumber(redis.call("HGET", KEYS[1], "used_quota")) or 0
local request_count = tonumber(redis.call("HGET", KEYS[1], "request_count")) or 0
local reserved_quota = tonumber(redis.call("HGET", KEYS[1], "reserved_quota")) or 0
local reserved_requests = tonumber(redis.call("HGET", KEYS[1], "reserved_requests")) or 0
local quota_delta = tonumber(ARGV[1]) or 0
local request_delta = tonumber(ARGV[2]) or 0
local quota_limit = tonumber(ARGV[3]) or 0
local request_limit = tonumber(ARGV[4]) or 0
local ttl = tonumber(ARGV[5]) or 0
local seed_used_quota = tonumber(ARGV[6]) or 0
local seed_request_count = tonumber(ARGV[7]) or 0
if redis.call("HGET", KEYS[1], "initialized") ~= "1" then
  redis.call("HSET", KEYS[1], "used_quota", seed_used_quota, "request_count", seed_request_count, "reserved_quota", 0, "reserved_requests", 0, "initialized", "1")
  used_quota = seed_used_quota
  request_count = seed_request_count
  reserved_quota = 0
  reserved_requests = 0
end
if quota_limit > 0 and used_quota + reserved_quota + quota_delta > quota_limit then
  if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
  return {0, 1, used_quota, request_count, reserved_quota, reserved_requests}
end
if request_limit > 0 and request_count + reserved_requests + request_delta > request_limit then
  if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
  return {0, 2, used_quota, request_count, reserved_quota, reserved_requests}
end
reserved_quota = reserved_quota + quota_delta
reserved_requests = reserved_requests + request_delta
redis.call("HSET", KEYS[1], "used_quota", used_quota, "request_count", request_count, "reserved_quota", reserved_quota, "reserved_requests", reserved_requests, "quota_limit", quota_limit, "request_limit", request_limit)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {1, 0, used_quota, request_count, reserved_quota, reserved_requests}
`
	subsiteQuotaCounterRedisSettleScript = `
local used_quota = tonumber(redis.call("HGET", KEYS[1], "used_quota")) or 0
local request_count = tonumber(redis.call("HGET", KEYS[1], "request_count")) or 0
local reserved_quota = tonumber(redis.call("HGET", KEYS[1], "reserved_quota")) or 0
local reserved_requests = tonumber(redis.call("HGET", KEYS[1], "reserved_requests")) or 0
local reserved_quota_delta = tonumber(ARGV[1]) or 0
local reserved_request_delta = tonumber(ARGV[2]) or 0
local actual_quota_delta = tonumber(ARGV[3]) or 0
local actual_request_delta = tonumber(ARGV[4]) or 0
local ttl = tonumber(ARGV[5]) or 0
reserved_quota = reserved_quota - reserved_quota_delta
reserved_requests = reserved_requests - reserved_request_delta
if reserved_quota < 0 then reserved_quota = 0 end
if reserved_requests < 0 then reserved_requests = 0 end
used_quota = used_quota + actual_quota_delta
request_count = request_count + actual_request_delta
redis.call("HSET", KEYS[1], "used_quota", used_quota, "request_count", request_count, "reserved_quota", reserved_quota, "reserved_requests", reserved_requests)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {used_quota, request_count, reserved_quota, reserved_requests}
`
	subsiteQuotaCounterRedisRefundScript = `
local used_quota = tonumber(redis.call("HGET", KEYS[1], "used_quota")) or 0
local request_count = tonumber(redis.call("HGET", KEYS[1], "request_count")) or 0
local reserved_quota = tonumber(redis.call("HGET", KEYS[1], "reserved_quota")) or 0
local reserved_requests = tonumber(redis.call("HGET", KEYS[1], "reserved_requests")) or 0
local quota_delta = tonumber(ARGV[1]) or 0
local request_delta = tonumber(ARGV[2]) or 0
local ttl = tonumber(ARGV[3]) or 0
reserved_quota = reserved_quota - quota_delta
reserved_requests = reserved_requests - request_delta
if reserved_quota < 0 then reserved_quota = 0 end
if reserved_requests < 0 then reserved_requests = 0 end
redis.call("HSET", KEYS[1], "used_quota", used_quota, "request_count", request_count, "reserved_quota", reserved_quota, "reserved_requests", reserved_requests)
if ttl > 0 then redis.call("EXPIRE", KEYS[1], ttl) end
return {used_quota, request_count, reserved_quota, reserved_requests}
`
)

type subsiteQuotaCounterSnapshot struct {
	UsedQuota        int
	RequestCount     int
	ReservedQuota    int
	ReservedRequests int
}

type subsiteQuotaCounterRef struct {
	SubsiteId    int64
	UserId       int
	Scope        string
	WindowType   string
	WindowStart  int64
	WindowEnd    int64
	QuotaLimit   int
	RequestLimit int
}

type subsiteQuotaReservedCounter struct {
	subsiteQuotaCounterRef
	Quota        int
	RequestCount int
	Key          string
	TTL          time.Duration
}

type subsiteQuotaReservation struct {
	EstimatedQuota int
	RedisUsed      bool
	Counters       []subsiteQuotaReservedCounter
}

type subsiteQuotaAtomicCounter interface {
	Enabled() bool
	Reserve(ctx context.Context, key string, quota int, requestCount int, quotaLimit int, requestLimit int, ttl time.Duration, seed subsiteQuotaCounterSnapshot) (subsiteQuotaCounterSnapshot, bool, int64, error)
	Settle(ctx context.Context, key string, reservedQuota int, reservedRequests int, actualQuota int, actualRequests int, ttl time.Duration) error
	Refund(ctx context.Context, key string, quota int, requestCount int, ttl time.Duration) error
}

type redisSubsiteQuotaAtomicCounter struct{}

var subsiteQuotaCounterBackend subsiteQuotaAtomicCounter = redisSubsiteQuotaAtomicCounter{}

func PreCheckSubsiteQuota(c *gin.Context, estimatedQuota int) *types.NewAPIError {
	if c == nil {
		return nil
	}
	subsiteId := c.GetInt64("subsite_id")
	userId := c.GetInt("id")
	if subsiteId <= 0 || userId <= 0 {
		return nil
	}

	policy, ok, err := getSubsiteQuotaPolicy(subsiteId)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if !ok {
		return nil
	}
	if estimatedQuota < 0 {
		estimatedQuota = 0
	}

	now := common.GetTimestamp()
	if subsiteQuotaAtomicCounterEnabled() {
		reservation, apiErr, err := reserveSubsiteQuotaWithRedis(c, policy, estimatedQuota, now)
		if apiErr != nil {
			return apiErr
		}
		if err == nil {
			if reservation != nil {
				c.Set(subsiteQuotaReservationContextKey, reservation)
			}
			return nil
		}
		common.SysLog("subsite quota redis counter failed, fallback to DB counter: " + err.Error())
	}
	return checkSubsiteQuotaWithDB(subsiteId, userId, policy, estimatedQuota, now)
}

func checkSubsiteQuotaWithDB(subsiteId int64, userId int, policy model.SubsiteQuotaPolicy, estimatedQuota int, now int64) *types.NewAPIError {
	dailyStart, _ := dailyWindow(now)
	siteDaily, err := getSubsiteQuotaCounter(subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowDaily, dailyStart, now)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userDaily, err := getSubsiteQuotaCounter(subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowDaily, dailyStart, now)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}

	if exceedsLimit(siteDaily.UsedQuota, estimatedQuota, policy.SiteDailyQuota) {
		return subsiteQuotaError(model.SubsiteAccessCodeQuota, "Subsite daily quota has been exceeded")
	}
	if exceedsLimit(userDaily.UsedQuota, estimatedQuota, policy.UserDailyQuota) {
		return subsiteQuotaError(model.SubsiteAccessCodeUserQuota, "Subsite user daily quota has been exceeded")
	}
	if exceedsLimit(siteDaily.RequestCount, 1, policy.SiteDailyRequestLimit) {
		return subsiteRateLimitError("Subsite daily request limit has been exceeded")
	}
	if exceedsLimit(userDaily.RequestCount, 1, policy.UserDailyRequestLimit) {
		return subsiteRateLimitError("Subsite user daily request limit has been exceeded")
	}

	if policy.SiteWindowSeconds > 0 && (policy.SiteWindowQuota > 0 || policy.SiteWindowRequestLimit > 0) {
		siteRolling, err := getSubsiteQuotaCounter(subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowRolling, 0, now)
		if err != nil {
			return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if exceedsLimit(siteRolling.UsedQuota, estimatedQuota, policy.SiteWindowQuota) {
			return subsiteQuotaError(model.SubsiteAccessCodeQuota, "Subsite rolling quota has been exceeded")
		}
		if exceedsLimit(siteRolling.RequestCount, 1, policy.SiteWindowRequestLimit) {
			return subsiteRateLimitError("Subsite rolling request limit has been exceeded")
		}
	}

	if policy.UserWindowSeconds > 0 && (policy.UserWindowQuota > 0 || policy.UserWindowRequestLimit > 0) {
		userRolling, err := getSubsiteQuotaCounter(subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowRolling, 0, now)
		if err != nil {
			return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if exceedsLimit(userRolling.UsedQuota, estimatedQuota, policy.UserWindowQuota) {
			return subsiteQuotaError(model.SubsiteAccessCodeUserQuota, "Subsite user rolling quota has been exceeded")
		}
		if exceedsLimit(userRolling.RequestCount, 1, policy.UserWindowRequestLimit) {
			return subsiteRateLimitError("Subsite user rolling request limit has been exceeded")
		}
	}

	return nil
}

func SettleSubsiteQuotaUsage(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, quota int) error {
	if ctx == nil || relayInfo == nil || ctx.GetBool(subsiteQuotaSettledContextKey) {
		return nil
	}
	subsiteId := ctx.GetInt64("subsite_id")
	userId := relayInfo.UserId
	if subsiteId <= 0 || userId <= 0 {
		return nil
	}
	if quota < 0 {
		quota = 0
	}

	policy, _, err := getSubsiteQuotaPolicy(subsiteId)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		dailyStart, dailyEnd := dailyWindow(now)
		if err := upsertSubsiteQuotaCounter(tx, subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowDaily, dailyStart, dailyEnd, quota, 1); err != nil {
			return err
		}
		if err := upsertSubsiteQuotaCounter(tx, subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowDaily, dailyStart, dailyEnd, quota, 1); err != nil {
			return err
		}
		if policy.SiteWindowSeconds > 0 {
			start, end, err := currentSubsiteRollingWindow(tx, subsiteId, 0, model.SubsiteCounterScopeSite, policy.SiteWindowSeconds, now)
			if err != nil {
				return err
			}
			if err := upsertSubsiteQuotaCounter(tx, subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowRolling, start, end, quota, 1); err != nil {
				return err
			}
		}
		if policy.UserWindowSeconds > 0 {
			start, end, err := currentSubsiteRollingWindow(tx, subsiteId, userId, model.SubsiteCounterScopeUser, policy.UserWindowSeconds, now)
			if err != nil {
				return err
			}
			if err := upsertSubsiteQuotaCounter(tx, subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowRolling, start, end, quota, 1); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		RefundSubsiteQuotaReservation(ctx)
		return err
	}
	settleRedisSubsiteQuotaReservation(ctx, quota)
	ctx.Set(subsiteQuotaSettledContextKey, true)
	return nil
}

func RefundSubsiteQuotaReservation(ctx *gin.Context) {
	if ctx == nil || ctx.GetBool(subsiteQuotaSettledContextKey) || ctx.GetBool(subsiteQuotaRefundedContextKey) {
		return
	}
	reservation, ok := subsiteQuotaReservationFromContext(ctx)
	if !ok || reservation == nil || !reservation.RedisUsed || len(reservation.Counters) == 0 {
		return
	}
	ctx.Set(subsiteQuotaRefundedContextKey, true)
	if !subsiteQuotaCounterBackendAvailable() {
		return
	}
	for _, counter := range reservation.Counters {
		if counter.Key == "" {
			continue
		}
		if err := subsiteQuotaCounterBackend.Refund(context.Background(), counter.Key, counter.Quota, counter.RequestCount, counter.TTL); err != nil {
			common.SysLog("subsite quota redis reservation rollback failed: " + err.Error())
		}
	}
}

func reserveSubsiteQuotaWithRedis(c *gin.Context, policy model.SubsiteQuotaPolicy, estimatedQuota int, now int64) (*subsiteQuotaReservation, *types.NewAPIError, error) {
	subsiteId := c.GetInt64("subsite_id")
	userId := c.GetInt("id")
	refs, err := buildSubsiteQuotaLimitedCounterRefs(policy, subsiteId, userId, now)
	if err != nil {
		return nil, nil, err
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}

	reservation := &subsiteQuotaReservation{
		EstimatedQuota: estimatedQuota,
		RedisUsed:      true,
		Counters:       make([]subsiteQuotaReservedCounter, 0, len(refs)),
	}
	for _, ref := range refs {
		seedCounter, err := getSubsiteQuotaCounterByRef(ref)
		if err != nil {
			refundRedisSubsiteCounters(reservation.Counters)
			return nil, nil, err
		}
		key := subsiteQuotaCounterRedisKey(ref)
		ttl := subsiteQuotaCounterRedisTTL(ref.WindowEnd)
		_, allowed, reason, err := subsiteQuotaCounterBackend.Reserve(
			context.Background(),
			key,
			estimatedQuota,
			1,
			ref.QuotaLimit,
			ref.RequestLimit,
			ttl,
			subsiteQuotaCounterSnapshot{
				UsedQuota:    seedCounter.UsedQuota,
				RequestCount: seedCounter.RequestCount,
			},
		)
		if err != nil {
			refundRedisSubsiteCounters(reservation.Counters)
			return nil, nil, err
		}
		if !allowed {
			refundRedisSubsiteCounters(reservation.Counters)
			return nil, subsiteQuotaRedisLimitError(ref, reason), nil
		}
		reservation.Counters = append(reservation.Counters, subsiteQuotaReservedCounter{
			subsiteQuotaCounterRef: ref,
			Quota:                  estimatedQuota,
			RequestCount:           1,
			Key:                    key,
			TTL:                    ttl,
		})
	}
	return reservation, nil, nil
}

func buildSubsiteQuotaLimitedCounterRefs(policy model.SubsiteQuotaPolicy, subsiteId int64, userId int, now int64) ([]subsiteQuotaCounterRef, error) {
	dailyStart, dailyEnd := dailyWindow(now)
	refs := make([]subsiteQuotaCounterRef, 0, 4)
	if policy.SiteDailyQuota > 0 || policy.SiteDailyRequestLimit > 0 {
		refs = append(refs, subsiteQuotaCounterRef{
			SubsiteId:    subsiteId,
			UserId:       0,
			Scope:        model.SubsiteCounterScopeSite,
			WindowType:   model.SubsiteCounterWindowDaily,
			WindowStart:  dailyStart,
			WindowEnd:    dailyEnd,
			QuotaLimit:   policy.SiteDailyQuota,
			RequestLimit: policy.SiteDailyRequestLimit,
		})
	}
	if policy.UserDailyQuota > 0 || policy.UserDailyRequestLimit > 0 {
		refs = append(refs, subsiteQuotaCounterRef{
			SubsiteId:    subsiteId,
			UserId:       userId,
			Scope:        model.SubsiteCounterScopeUser,
			WindowType:   model.SubsiteCounterWindowDaily,
			WindowStart:  dailyStart,
			WindowEnd:    dailyEnd,
			QuotaLimit:   policy.UserDailyQuota,
			RequestLimit: policy.UserDailyRequestLimit,
		})
	}
	if policy.SiteWindowSeconds > 0 && (policy.SiteWindowQuota > 0 || policy.SiteWindowRequestLimit > 0) {
		start, end := deterministicSubsiteRollingWindow(now, policy.SiteWindowSeconds)
		ref := subsiteQuotaCounterRef{
			SubsiteId:    subsiteId,
			UserId:       0,
			Scope:        model.SubsiteCounterScopeSite,
			WindowType:   model.SubsiteCounterWindowRolling,
			WindowStart:  start,
			WindowEnd:    end,
			QuotaLimit:   policy.SiteWindowQuota,
			RequestLimit: policy.SiteWindowRequestLimit,
		}
		if err := ensureSubsiteQuotaCounterWindow(ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if policy.UserWindowSeconds > 0 && (policy.UserWindowQuota > 0 || policy.UserWindowRequestLimit > 0) {
		start, end := deterministicSubsiteRollingWindow(now, policy.UserWindowSeconds)
		ref := subsiteQuotaCounterRef{
			SubsiteId:    subsiteId,
			UserId:       userId,
			Scope:        model.SubsiteCounterScopeUser,
			WindowType:   model.SubsiteCounterWindowRolling,
			WindowStart:  start,
			WindowEnd:    end,
			QuotaLimit:   policy.UserWindowQuota,
			RequestLimit: policy.UserWindowRequestLimit,
		}
		if err := ensureSubsiteQuotaCounterWindow(ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func deterministicSubsiteRollingWindow(now int64, windowSeconds int64) (int64, int64) {
	if windowSeconds <= 0 {
		return now, now
	}
	start := now - (now % windowSeconds)
	return start, start + windowSeconds
}

func ensureSubsiteQuotaCounterWindow(ref subsiteQuotaCounterRef) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return upsertSubsiteQuotaCounter(tx, ref.SubsiteId, ref.UserId, ref.Scope, ref.WindowType, ref.WindowStart, ref.WindowEnd, 0, 0)
	})
}

func getSubsiteQuotaCounterByRef(ref subsiteQuotaCounterRef) (model.SubsiteQuotaCounter, error) {
	var counter model.SubsiteQuotaCounter
	err := model.DB.Where("subsite_id = ? AND user_id = ? AND scope = ? AND window_type = ? AND window_start = ?",
		ref.SubsiteId, ref.UserId, ref.Scope, ref.WindowType, ref.WindowStart).
		First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SubsiteQuotaCounter{}, nil
	}
	return counter, err
}

func subsiteQuotaRedisLimitError(ref subsiteQuotaCounterRef, reason int64) *types.NewAPIError {
	if reason == subsiteQuotaRedisRejectRequest {
		return subsiteRateLimitError(subsiteQuotaLimitMessage(ref, "request limit"))
	}
	if ref.Scope == model.SubsiteCounterScopeUser {
		return subsiteQuotaError(model.SubsiteAccessCodeUserQuota, subsiteQuotaLimitMessage(ref, "quota"))
	}
	return subsiteQuotaError(model.SubsiteAccessCodeQuota, subsiteQuotaLimitMessage(ref, "quota"))
}

func subsiteQuotaLimitMessage(ref subsiteQuotaCounterRef, limitName string) string {
	scope := "Subsite"
	if ref.Scope == model.SubsiteCounterScopeUser {
		scope = "Subsite user"
	}
	window := "daily"
	if ref.WindowType == model.SubsiteCounterWindowRolling {
		window = "rolling"
	}
	return fmt.Sprintf("%s %s %s has been exceeded", scope, window, limitName)
}

func refundRedisSubsiteCounters(counters []subsiteQuotaReservedCounter) {
	if len(counters) == 0 || !subsiteQuotaCounterBackendAvailable() {
		return
	}
	for _, counter := range counters {
		if counter.Key == "" {
			continue
		}
		if err := subsiteQuotaCounterBackend.Refund(context.Background(), counter.Key, counter.Quota, counter.RequestCount, counter.TTL); err != nil {
			common.SysLog("subsite quota redis reservation rollback failed: " + err.Error())
		}
	}
}

func settleRedisSubsiteQuotaReservation(ctx *gin.Context, actualQuota int) {
	reservation, ok := subsiteQuotaReservationFromContext(ctx)
	if !ok || reservation == nil || !reservation.RedisUsed || len(reservation.Counters) == 0 {
		return
	}
	if actualQuota < 0 {
		actualQuota = 0
	}
	if !subsiteQuotaCounterBackendAvailable() {
		return
	}
	for _, counter := range reservation.Counters {
		if counter.Key == "" {
			continue
		}
		if err := subsiteQuotaCounterBackend.Settle(context.Background(), counter.Key, counter.Quota, counter.RequestCount, actualQuota, 1, counter.TTL); err != nil {
			common.SysLog("subsite quota redis settle failed: " + err.Error())
		}
	}
}

func subsiteQuotaReservationFromContext(ctx *gin.Context) (*subsiteQuotaReservation, bool) {
	value, ok := ctx.Get(subsiteQuotaReservationContextKey)
	if !ok {
		return nil, false
	}
	reservation, ok := value.(*subsiteQuotaReservation)
	return reservation, ok
}

func getSubsiteQuotaPolicy(subsiteId int64) (model.SubsiteQuotaPolicy, bool, error) {
	var policy model.SubsiteQuotaPolicy
	err := model.DB.Where("subsite_id = ?", subsiteId).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SubsiteQuotaPolicy{}, false, nil
	}
	return policy, err == nil, err
}

func exceedsLimit(used int, incoming int, limit int) bool {
	return limit > 0 && used+incoming > limit
}

func subsiteQuotaError(code string, message string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", message),
		types.ErrorCode(code),
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func subsiteRateLimitError(message string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", message),
		types.ErrorCode(model.SubsiteAccessCodeRateLimited),
		http.StatusTooManyRequests,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func currentSubsiteRollingWindow(tx *gorm.DB, subsiteId int64, userId int, scope string, windowSeconds int64, now int64) (int64, int64, error) {
	var counter model.SubsiteQuotaCounter
	err := tx.Where("subsite_id = ? AND user_id = ? AND scope = ? AND window_type = ? AND (window_end = 0 OR window_end >= ?)",
		subsiteId, userId, scope, model.SubsiteCounterWindowRolling, now).
		Order("window_start DESC").
		First(&counter).Error
	if err == nil {
		return counter.WindowStart, counter.WindowEnd, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}
	return now, now + windowSeconds, nil
}

func upsertSubsiteQuotaCounter(tx *gorm.DB, subsiteId int64, userId int, scope string, windowType string, windowStart int64, windowEnd int64, quota int, requestCount int) error {
	counter := model.SubsiteQuotaCounter{
		SubsiteId:    subsiteId,
		UserId:       userId,
		Scope:        scope,
		WindowType:   windowType,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
		UsedQuota:    quota,
		RequestCount: requestCount,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "subsite_id"},
			{Name: "user_id"},
			{Name: "scope"},
			{Name: "window_type"},
			{Name: "window_start"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"window_end":    windowEnd,
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", requestCount),
			"updated_at":    common.GetTimestamp(),
		}),
	}).Create(&counter).Error
}

func subsiteQuotaAtomicCounterEnabled() bool {
	return subsiteQuotaCounterBackendAvailable()
}

func subsiteQuotaCounterBackendAvailable() bool {
	return subsiteQuotaCounterBackend != nil && subsiteQuotaCounterBackend.Enabled()
}

func (redisSubsiteQuotaAtomicCounter) Enabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

func (redisSubsiteQuotaAtomicCounter) Reserve(ctx context.Context, key string, quota int, requestCount int, quotaLimit int, requestLimit int, ttl time.Duration, seed subsiteQuotaCounterSnapshot) (subsiteQuotaCounterSnapshot, bool, int64, error) {
	values, err := evalSubsiteQuotaCounterScript(ctx, subsiteQuotaCounterRedisReserveScript, key, quota, requestCount, quotaLimit, requestLimit, int64(ttl.Seconds()), seed.UsedQuota, seed.RequestCount)
	if err != nil {
		return subsiteQuotaCounterSnapshot{}, false, 0, err
	}
	if len(values) < 6 {
		return subsiteQuotaCounterSnapshot{}, false, 0, errors.New("invalid redis subsite quota reserve response")
	}
	return subsiteQuotaCounterSnapshot{
		UsedQuota:        int(values[2]),
		RequestCount:     int(values[3]),
		ReservedQuota:    int(values[4]),
		ReservedRequests: int(values[5]),
	}, values[0] == 1, values[1], nil
}

func (redisSubsiteQuotaAtomicCounter) Settle(ctx context.Context, key string, reservedQuota int, reservedRequests int, actualQuota int, actualRequests int, ttl time.Duration) error {
	_, err := evalSubsiteQuotaCounterScript(ctx, subsiteQuotaCounterRedisSettleScript, key, reservedQuota, reservedRequests, actualQuota, actualRequests, int64(ttl.Seconds()))
	return err
}

func (redisSubsiteQuotaAtomicCounter) Refund(ctx context.Context, key string, quota int, requestCount int, ttl time.Duration) error {
	_, err := evalSubsiteQuotaCounterScript(ctx, subsiteQuotaCounterRedisRefundScript, key, quota, requestCount, int64(ttl.Seconds()))
	return err
}

func evalSubsiteQuotaCounterScript(ctx context.Context, script string, key string, args ...any) ([]int64, error) {
	result, err := common.RDB.Eval(ctx, script, []string{key}, args...).Result()
	if err != nil {
		return nil, err
	}
	rawValues, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected redis subsite quota script result: %T", result)
	}
	values := make([]int64, 0, len(rawValues))
	for _, raw := range rawValues {
		switch value := raw.(type) {
		case int64:
			values = append(values, value)
		case int:
			values = append(values, int64(value))
		case string:
			var parsed int64
			if _, err := fmt.Sscan(value, &parsed); err != nil {
				return nil, err
			}
			values = append(values, parsed)
		default:
			return nil, fmt.Errorf("unexpected redis subsite quota script value: %T", raw)
		}
	}
	return values, nil
}

func subsiteQuotaCounterRedisKey(ref subsiteQuotaCounterRef) string {
	return fmt.Sprintf("subsite_quota_counter:v1:%d:%d:%s:%s:%d", ref.SubsiteId, ref.UserId, ref.Scope, ref.WindowType, ref.WindowStart)
}

func subsiteQuotaCounterRedisTTL(windowEnd int64) time.Duration {
	ttl := time.Until(time.Unix(windowEnd, 0)) + 48*time.Hour
	if ttl < time.Hour {
		return time.Hour
	}
	return ttl
}
