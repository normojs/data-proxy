package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const (
	enterpriseGovernanceAuditActionAnomalyThrottle = "enterprise_governance.anomaly_throttle"
	enterpriseAnomalyStatusThrottled               = "throttled"
	enterpriseAnomalyStatusWouldThrottle           = "would_throttle"
	enterpriseAnomalyReasonRequestSpike            = "request_spike"
	enterpriseAnomalyReasonCostSpike               = "cost_spike"
	enterpriseAnomalyReasonFailureRate             = "failure_rate"
	enterpriseAnomalyStatusHeader                  = "X-Data-Proxy-Enterprise-Anomaly-Status"
	enterpriseAnomalyReasonHeader                  = "X-Data-Proxy-Enterprise-Anomaly-Reason"
	enterpriseAnomalyProtectedUntilHeader          = "X-Data-Proxy-Enterprise-Anomaly-Protected-Until"
	enterpriseAnomalyCooldownSecondsHeader         = "X-Data-Proxy-Enterprise-Anomaly-Cooldown-Seconds"
)

var (
	// Keep defaults conservative until these thresholds become administrator-configurable.
	enterpriseAnomalyThrottleEnabled                   = true
	enterpriseAnomalyThrottleCurrentWindow             = 5 * time.Minute
	enterpriseAnomalyThrottleBaselineWindow            = 30 * time.Minute
	enterpriseAnomalyThrottleCooldown                  = time.Minute
	enterpriseAnomalyThrottleMinCurrentRequests  int64 = 100
	enterpriseAnomalyThrottleMinBaselineRequests int64 = 100
	enterpriseAnomalyThrottleRequestSpikeRatio         = 8.0
	enterpriseAnomalyThrottleMinCurrentQuota     int64 = 1000000
	enterpriseAnomalyThrottleMinBaselineQuota    int64 = 1000000
	enterpriseAnomalyThrottleCostSpikeRatio            = 8.0
	enterpriseAnomalyThrottleMinFailureRequests  int64 = 50
	enterpriseAnomalyThrottleMinFailures         int64 = 25
	enterpriseAnomalyThrottleFailureRate               = 0.5
	enterpriseAnomalyProtections                       = sync.Map{}
)

var ErrEnterpriseGovernanceAnomalyThrottled = errors.New("enterprise governance anomaly throttle active")

type EnterpriseGovernanceAnomalyThrottleResult struct {
	Applied         bool                                     `json:"applied"`
	Status          string                                   `json:"status"`
	Reason          string                                   `json:"reason"`
	Triggers        []EnterpriseGovernanceAnomalyTrigger     `json:"triggers"`
	Current         EnterpriseGovernanceAnomalyUsageSnapshot `json:"current"`
	Baseline        EnterpriseGovernanceAnomalyUsageSnapshot `json:"baseline"`
	DetectedAt      int64                                    `json:"detected_at"`
	ProtectedUntil  int64                                    `json:"protected_until"`
	CooldownSeconds int64                                    `json:"cooldown_seconds"`
	DryRun          bool                                     `json:"dry_run"`
}

type EnterpriseGovernanceAnomalyUsageSnapshot struct {
	WindowStart     int64 `json:"window_start"`
	WindowEnd       int64 `json:"window_end"`
	RequestCount    int64 `json:"request_count"`
	SuccessCount    int64 `json:"success_count"`
	ErrorCount      int64 `json:"error_count"`
	Quota           int64 `json:"quota"`
	LogConsumeCount int64 `json:"log_consume_count"`
}

type EnterpriseGovernanceAnomalyTrigger struct {
	Reason        string  `json:"reason"`
	CurrentValue  int64   `json:"current_value"`
	BaselineValue int64   `json:"baseline_value"`
	CurrentRate   float64 `json:"current_rate"`
	BaselineRate  float64 `json:"baseline_rate"`
	Ratio         float64 `json:"ratio"`
	Threshold     float64 `json:"threshold"`
}

type enterpriseAnomalyProtection struct {
	Reason         string
	Triggers       []EnterpriseGovernanceAnomalyTrigger
	Current        EnterpriseGovernanceAnomalyUsageSnapshot
	Baseline       EnterpriseGovernanceAnomalyUsageSnapshot
	DetectedAt     int64
	ProtectedUntil int64
}

func ApplyEnterpriseGovernanceAnomalyThrottle(c *gin.Context, relayInfo *relaycommon.RelayInfo) (EnterpriseGovernanceAnomalyThrottleResult, error) {
	result := EnterpriseGovernanceAnomalyThrottleResult{}
	if !common.EnterpriseGovernanceEnabled || !enterpriseAnomalyThrottleEnabled || c == nil || relayInfo == nil {
		return result, nil
	}
	enterpriseCtx, ok := common.GetContextKeyType[*EnterpriseContext](c, constant.ContextKeyEnterpriseGovernanceContext)
	if !ok || enterpriseCtx == nil {
		var err error
		enterpriseCtx, err = resolveEnterpriseContextFromRelay(c, relayInfo)
		if err != nil {
			return result, err
		}
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled || enterpriseCtx.EnterpriseId <= 0 {
		return result, nil
	}

	now := time.Now()
	protectionKey := enterpriseAnomalyProtectionKey(enterpriseCtx)
	if protection, ok := loadEnterpriseAnomalyProtection(protectionKey, now); ok {
		result = enterpriseAnomalyResultFromProtection(protection, enterpriseCtx.DryRun)
		setEnterpriseAnomalyThrottleHeaders(c, result)
		recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
		if enterpriseCtx.DryRun {
			return result, nil
		}
		return result, ErrEnterpriseGovernanceAnomalyThrottled
	}

	result, err := detectEnterpriseGovernanceAnomalyThrottle(enterpriseCtx, now)
	if err != nil {
		return EnterpriseGovernanceAnomalyThrottleResult{}, err
	}
	if !result.Applied {
		return result, nil
	}
	if enterpriseCtx.DryRun {
		result.Status = enterpriseAnomalyStatusWouldThrottle
		result.DryRun = true
		setEnterpriseAnomalyThrottleHeaders(c, result)
		recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly throttle dry-run observed: reason=%s", result.Reason))
		return result, nil
	}

	enterpriseAnomalyProtections.Store(protectionKey, enterpriseAnomalyProtection{
		Reason:         result.Reason,
		Triggers:       append([]EnterpriseGovernanceAnomalyTrigger(nil), result.Triggers...),
		Current:        result.Current,
		Baseline:       result.Baseline,
		DetectedAt:     result.DetectedAt,
		ProtectedUntil: result.ProtectedUntil,
	})
	setEnterpriseAnomalyThrottleHeaders(c, result)
	recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
	logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly throttle activated: reason=%s protected_until=%d", result.Reason, result.ProtectedUntil))
	return result, ErrEnterpriseGovernanceAnomalyThrottled
}

func detectEnterpriseGovernanceAnomalyThrottle(enterpriseCtx *EnterpriseContext, now time.Time) (EnterpriseGovernanceAnomalyThrottleResult, error) {
	result := EnterpriseGovernanceAnomalyThrottleResult{}
	currentWindow := normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleCurrentWindow, 5*time.Minute)
	baselineWindow := normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleBaselineWindow, 30*time.Minute)
	cooldown := normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleCooldown, time.Minute)
	currentStart := now.Add(-currentWindow)
	baselineStart := currentStart.Add(-baselineWindow)

	current, err := loadEnterpriseAnomalyUsageSnapshot(enterpriseCtx, currentStart, now)
	if err != nil {
		return result, err
	}
	baseline, err := loadEnterpriseAnomalyUsageSnapshot(enterpriseCtx, baselineStart, currentStart)
	if err != nil {
		return result, err
	}

	triggers := enterpriseAnomalyTriggers(current, baseline, currentWindow, baselineWindow)
	if len(triggers) == 0 {
		return result, nil
	}
	return EnterpriseGovernanceAnomalyThrottleResult{
		Applied:         true,
		Status:          enterpriseAnomalyStatusThrottled,
		Reason:          triggers[0].Reason,
		Triggers:        triggers,
		Current:         current,
		Baseline:        baseline,
		DetectedAt:      now.Unix(),
		ProtectedUntil:  now.Add(cooldown).Unix(),
		CooldownSeconds: int64(cooldown / time.Second),
		DryRun:          enterpriseCtx.DryRun,
	}, nil
}

func enterpriseAnomalyTriggers(current EnterpriseGovernanceAnomalyUsageSnapshot, baseline EnterpriseGovernanceAnomalyUsageSnapshot, currentWindow time.Duration, baselineWindow time.Duration) []EnterpriseGovernanceAnomalyTrigger {
	triggers := make([]EnterpriseGovernanceAnomalyTrigger, 0, 3)
	if current.RequestCount >= enterpriseAnomalyThrottleMinFailureRequests &&
		current.ErrorCount >= enterpriseAnomalyThrottleMinFailures {
		failureRate := safeRatio(current.ErrorCount, current.RequestCount)
		if failureRate >= enterpriseAnomalyThrottleFailureRate {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonFailureRate,
				CurrentValue:  current.ErrorCount,
				BaselineValue: baseline.ErrorCount,
				CurrentRate:   failureRate,
				BaselineRate:  safeRatio(baseline.ErrorCount, baseline.RequestCount),
				Ratio:         failureRate,
				Threshold:     enterpriseAnomalyThrottleFailureRate,
			})
		}
	}
	if current.Quota >= enterpriseAnomalyThrottleMinCurrentQuota &&
		baseline.Quota >= enterpriseAnomalyThrottleMinBaselineQuota {
		currentRate := ratePerSecond(current.Quota, currentWindow)
		baselineRate := ratePerSecond(baseline.Quota, baselineWindow)
		ratio := currentRate / baselineRate
		if baselineRate > 0 && ratio >= enterpriseAnomalyThrottleCostSpikeRatio {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonCostSpike,
				CurrentValue:  current.Quota,
				BaselineValue: baseline.Quota,
				CurrentRate:   currentRate,
				BaselineRate:  baselineRate,
				Ratio:         ratio,
				Threshold:     enterpriseAnomalyThrottleCostSpikeRatio,
			})
		}
	}
	if current.RequestCount >= enterpriseAnomalyThrottleMinCurrentRequests &&
		baseline.RequestCount >= enterpriseAnomalyThrottleMinBaselineRequests {
		currentRate := ratePerSecond(current.RequestCount, currentWindow)
		baselineRate := ratePerSecond(baseline.RequestCount, baselineWindow)
		ratio := currentRate / baselineRate
		if baselineRate > 0 && ratio >= enterpriseAnomalyThrottleRequestSpikeRatio {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonRequestSpike,
				CurrentValue:  current.RequestCount,
				BaselineValue: baseline.RequestCount,
				CurrentRate:   currentRate,
				BaselineRate:  baselineRate,
				Ratio:         ratio,
				Threshold:     enterpriseAnomalyThrottleRequestSpikeRatio,
			})
		}
	}
	return triggers
}

func loadEnterpriseAnomalyUsageSnapshot(enterpriseCtx *EnterpriseContext, start time.Time, end time.Time) (EnterpriseGovernanceAnomalyUsageSnapshot, error) {
	snapshot := EnterpriseGovernanceAnomalyUsageSnapshot{
		WindowStart: start.Unix(),
		WindowEnd:   end.Unix(),
	}
	var usage struct {
		RequestCount int64
		Quota        int64
	}
	if err := model.DB.Model(&model.EnterpriseUsageAttribution{}).
		Select("COUNT(*) AS request_count, COALESCE(SUM(quota), 0) AS quota").
		Where("enterprise_id = ? AND created_at >= ? AND created_at < ?", enterpriseCtx.EnterpriseId, start.Unix(), end.Unix()).
		Scan(&usage).Error; err != nil {
		return snapshot, err
	}
	snapshot.SuccessCount = usage.RequestCount
	snapshot.Quota = usage.Quota
	snapshot.RequestCount = usage.RequestCount

	consumeCount, errorCount, err := loadEnterpriseAnomalyLogCounts(enterpriseCtx, start, end)
	if err != nil {
		return snapshot, err
	}
	snapshot.LogConsumeCount = consumeCount
	snapshot.ErrorCount = errorCount
	if consumeCount+errorCount > 0 {
		snapshot.RequestCount = consumeCount + errorCount
	}
	return snapshot, nil
}

func loadEnterpriseAnomalyLogCounts(enterpriseCtx *EnterpriseContext, start time.Time, end time.Time) (int64, int64, error) {
	if model.LOG_DB == nil {
		return 0, 0, nil
	}
	userIds, err := enterpriseAnomalyUserIds(enterpriseCtx)
	if err != nil {
		return 0, 0, err
	}
	type logCountRow struct {
		Type  int
		Count int64
	}
	var rows []logCountRow
	query := model.LOG_DB.Model(&model.Log{}).
		Select("type, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ? AND type IN ?", start.Unix(), end.Unix(), []int{model.LogTypeConsume, model.LogTypeError}).
		Group("type")
	if len(userIds) > 0 {
		query = query.Where("user_id IN ?", userIds)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return 0, 0, err
	}
	var consumeCount int64
	var errorCount int64
	for _, row := range rows {
		switch row.Type {
		case model.LogTypeConsume:
			consumeCount = row.Count
		case model.LogTypeError:
			errorCount = row.Count
		}
	}
	return consumeCount, errorCount, nil
}

func enterpriseAnomalyUserIds(enterpriseCtx *EnterpriseContext) ([]int, error) {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 {
		return nil, nil
	}
	var userIds []int
	if err := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ?", enterpriseCtx.EnterpriseId).
		Pluck("user_id", &userIds).Error; err != nil {
		return nil, err
	}
	return userIds, nil
}

func loadEnterpriseAnomalyProtection(key string, now time.Time) (enterpriseAnomalyProtection, bool) {
	value, ok := enterpriseAnomalyProtections.Load(key)
	if !ok {
		return enterpriseAnomalyProtection{}, false
	}
	protection, ok := value.(enterpriseAnomalyProtection)
	if !ok || protection.ProtectedUntil <= now.Unix() {
		enterpriseAnomalyProtections.Delete(key)
		return enterpriseAnomalyProtection{}, false
	}
	return protection, true
}

func enterpriseAnomalyResultFromProtection(protection enterpriseAnomalyProtection, dryRun bool) EnterpriseGovernanceAnomalyThrottleResult {
	status := enterpriseAnomalyStatusThrottled
	if dryRun {
		status = enterpriseAnomalyStatusWouldThrottle
	}
	cooldownSeconds := protection.ProtectedUntil - time.Now().Unix()
	if cooldownSeconds < 0 {
		cooldownSeconds = 0
	}
	return EnterpriseGovernanceAnomalyThrottleResult{
		Applied:         true,
		Status:          status,
		Reason:          protection.Reason,
		Triggers:        append([]EnterpriseGovernanceAnomalyTrigger(nil), protection.Triggers...),
		Current:         protection.Current,
		Baseline:        protection.Baseline,
		DetectedAt:      protection.DetectedAt,
		ProtectedUntil:  protection.ProtectedUntil,
		CooldownSeconds: cooldownSeconds,
		DryRun:          dryRun,
	}
}

func enterpriseAnomalyProtectionKey(enterpriseCtx *EnterpriseContext) string {
	if enterpriseCtx != nil && enterpriseCtx.EnterpriseId > 0 {
		return fmt.Sprintf("enterprise:%d", enterpriseCtx.EnterpriseId)
	}
	return "enterprise:unknown"
}

func setEnterpriseAnomalyThrottleHeaders(c *gin.Context, result EnterpriseGovernanceAnomalyThrottleResult) {
	if c == nil || !result.Applied {
		return
	}
	c.Header(enterpriseAnomalyStatusHeader, result.Status)
	c.Header(enterpriseAnomalyReasonHeader, result.Reason)
	c.Header(enterpriseAnomalyProtectedUntilHeader, strconv.FormatInt(result.ProtectedUntil, 10))
	c.Header(enterpriseAnomalyCooldownSecondsHeader, strconv.FormatInt(result.CooldownSeconds, 10))
}

func recordEnterpriseGovernanceAnomalyThrottleAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, result EnterpriseGovernanceAnomalyThrottleResult) {
	if enterpriseCtx == nil || !result.Applied {
		return
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ActorUserId:    enterpriseCtx.UserId,
		Action:         enterpriseGovernanceAuditActionAnomalyThrottle,
		TargetType:     "enterprise",
		TargetId:       enterpriseCtx.EnterpriseId,
		ScopeUserId:    enterpriseCtx.UserId,
		ScopeOrgUnitId: enterpriseCtx.PrimaryOrgUnitId,
		ScopeProjectId: enterpriseCtx.ProjectId,
		After:          enterpriseGovernanceAnomalyThrottleAuditPayload(c, enterpriseCtx, relayInfo, result, requestId),
		RequestId:      requestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance anomaly throttle audit: "+err.Error())
	}
}

func enterpriseGovernanceAnomalyThrottleAuditPayload(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, result EnterpriseGovernanceAnomalyThrottleResult, requestId string) map[string]any {
	modelName := ""
	channelId := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		channelId = enterpriseChannelIdFromRelay(c, relayInfo)
	}
	return map[string]any{
		"request_id":       requestId,
		"model":            modelName,
		"channel_id":       channelId,
		"token_id":         enterpriseCtx.TokenId,
		"org_unit_id":      enterpriseCtx.PrimaryOrgUnitId,
		"project_id":       enterpriseCtx.ProjectId,
		"policy_group_ids": cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"anomaly_status":   result.Status,
		"anomaly_reason":   result.Reason,
		"anomaly_triggers": result.Triggers,
		"current_window":   result.Current,
		"baseline_window":  result.Baseline,
		"detected_at":      result.DetectedAt,
		"protected_until":  result.ProtectedUntil,
		"cooldown_seconds": result.CooldownSeconds,
		"user_message_key": "enterprise_governance.anomaly_throttled",
		"error_code":       "enterprise_governance_anomaly_throttled",
		"dry_run":          result.DryRun,
	}
}

func normalizedEnterpriseAnomalyWindow(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func ratePerSecond(value int64, window time.Duration) float64 {
	if value <= 0 || window <= 0 {
		return 0
	}
	return float64(value) / window.Seconds()
}

func safeRatio(numerator int64, denominator int64) float64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return math.Round((float64(numerator)/float64(denominator))*10000) / 10000
}
