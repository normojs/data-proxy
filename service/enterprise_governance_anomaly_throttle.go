package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	enterpriseGovernanceAuditActionAnomalyThrottle = "enterprise_governance.anomaly_throttle"
	enterpriseAnomalyStatusThrottled               = "throttled"
	enterpriseAnomalyStatusWouldThrottle           = "would_throttle"
	enterpriseAnomalyStatusOrchestrated            = "orchestrated"
	enterpriseAnomalyReasonRequestSpike            = "request_spike"
	enterpriseAnomalyReasonCostSpike               = "cost_spike"
	enterpriseAnomalyReasonFailureRate             = "failure_rate"
	enterpriseAnomalyStatusHeader                  = "X-Data-Proxy-Enterprise-Anomaly-Status"
	enterpriseAnomalyReasonHeader                  = "X-Data-Proxy-Enterprise-Anomaly-Reason"
	enterpriseAnomalyProtectedUntilHeader          = "X-Data-Proxy-Enterprise-Anomaly-Protected-Until"
	enterpriseAnomalyCooldownSecondsHeader         = "X-Data-Proxy-Enterprise-Anomaly-Cooldown-Seconds"
)

var (
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
	Applied             bool                                     `json:"applied"`
	Status              string                                   `json:"status"`
	Reason              string                                   `json:"reason"`
	Triggers            []EnterpriseGovernanceAnomalyTrigger     `json:"triggers"`
	Current             EnterpriseGovernanceAnomalyUsageSnapshot `json:"current"`
	Baseline            EnterpriseGovernanceAnomalyUsageSnapshot `json:"baseline"`
	DetectedAt          int64                                    `json:"detected_at"`
	ProtectedUntil      int64                                    `json:"protected_until"`
	CooldownSeconds     int64                                    `json:"cooldown_seconds"`
	PolicyActions       []PolicyActionObservation                `json:"policy_actions"`
	OrchestrationAction string                                   `json:"orchestration_action,omitempty"`
	DryRun              bool                                     `json:"dry_run"`
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

type EnterpriseAnomalyThrottleConfig struct {
	Enabled               bool    `json:"enabled"`
	CurrentWindowSeconds  int64   `json:"current_window_seconds"`
	BaselineWindowSeconds int64   `json:"baseline_window_seconds"`
	CooldownSeconds       int64   `json:"cooldown_seconds"`
	MinCurrentRequests    int64   `json:"min_current_requests"`
	MinBaselineRequests   int64   `json:"min_baseline_requests"`
	RequestSpikeRatio     float64 `json:"request_spike_ratio"`
	MinCurrentQuota       int64   `json:"min_current_quota"`
	MinBaselineQuota      int64   `json:"min_baseline_quota"`
	CostSpikeRatio        float64 `json:"cost_spike_ratio"`
	MinFailureRequests    int64   `json:"min_failure_requests"`
	MinFailures           int64   `json:"min_failures"`
	FailureRate           float64 `json:"failure_rate"`
}

type EnterpriseAnomalyThrottleConfigInput struct {
	Enabled               *bool    `json:"enabled"`
	CurrentWindowSeconds  *int64   `json:"current_window_seconds"`
	BaselineWindowSeconds *int64   `json:"baseline_window_seconds"`
	CooldownSeconds       *int64   `json:"cooldown_seconds"`
	MinCurrentRequests    *int64   `json:"min_current_requests"`
	MinBaselineRequests   *int64   `json:"min_baseline_requests"`
	RequestSpikeRatio     *float64 `json:"request_spike_ratio"`
	MinCurrentQuota       *int64   `json:"min_current_quota"`
	MinBaselineQuota      *int64   `json:"min_baseline_quota"`
	CostSpikeRatio        *float64 `json:"cost_spike_ratio"`
	MinFailureRequests    *int64   `json:"min_failure_requests"`
	MinFailures           *int64   `json:"min_failures"`
	FailureRate           *float64 `json:"failure_rate"`
}

type enterpriseAnomalyProtection struct {
	ScopeType      string
	ScopeId        int
	Reason         string
	Triggers       []EnterpriseGovernanceAnomalyTrigger
	Current        EnterpriseGovernanceAnomalyUsageSnapshot
	Baseline       EnterpriseGovernanceAnomalyUsageSnapshot
	DetectedAt     int64
	ProtectedUntil int64
	PolicyActions  []PolicyActionObservation
}

type enterpriseAnomalyProtectionPayload struct {
	ScopeType      string                                   `json:"scope_type"`
	ScopeId        int                                      `json:"scope_id"`
	Reason         string                                   `json:"reason"`
	Triggers       []EnterpriseGovernanceAnomalyTrigger     `json:"triggers"`
	Current        EnterpriseGovernanceAnomalyUsageSnapshot `json:"current"`
	Baseline       EnterpriseGovernanceAnomalyUsageSnapshot `json:"baseline"`
	DetectedAt     int64                                    `json:"detected_at"`
	ProtectedUntil int64                                    `json:"protected_until"`
	PolicyActions  []PolicyActionObservation                `json:"policy_actions"`
}

func DefaultEnterpriseAnomalyThrottleConfig() EnterpriseAnomalyThrottleConfig {
	return EnterpriseAnomalyThrottleConfig{
		Enabled:               true,
		CurrentWindowSeconds:  int64(normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleCurrentWindow, 5*time.Minute) / time.Second),
		BaselineWindowSeconds: int64(normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleBaselineWindow, 30*time.Minute) / time.Second),
		CooldownSeconds:       int64(normalizedEnterpriseAnomalyWindow(enterpriseAnomalyThrottleCooldown, time.Minute) / time.Second),
		MinCurrentRequests:    enterpriseAnomalyThrottleMinCurrentRequests,
		MinBaselineRequests:   enterpriseAnomalyThrottleMinBaselineRequests,
		RequestSpikeRatio:     enterpriseAnomalyThrottleRequestSpikeRatio,
		MinCurrentQuota:       enterpriseAnomalyThrottleMinCurrentQuota,
		MinBaselineQuota:      enterpriseAnomalyThrottleMinBaselineQuota,
		CostSpikeRatio:        enterpriseAnomalyThrottleCostSpikeRatio,
		MinFailureRequests:    enterpriseAnomalyThrottleMinFailureRequests,
		MinFailures:           enterpriseAnomalyThrottleMinFailures,
		FailureRate:           enterpriseAnomalyThrottleFailureRate,
	}
}

func EnterpriseAnomalyThrottleConfigFromJSON(raw string) EnterpriseAnomalyThrottleConfig {
	config := DefaultEnterpriseAnomalyThrottleConfig()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config
	}
	var input EnterpriseAnomalyThrottleConfigInput
	if err := common.Unmarshal([]byte(raw), &input); err != nil {
		common.SysError("error unmarshaling enterprise anomaly throttle config: " + err.Error())
		return config
	}
	return NormalizeEnterpriseAnomalyThrottleConfigInput(config, &input)
}

func NormalizeEnterpriseAnomalyThrottleConfigInput(base EnterpriseAnomalyThrottleConfig, input *EnterpriseAnomalyThrottleConfigInput) EnterpriseAnomalyThrottleConfig {
	config := normalizeEnterpriseAnomalyThrottleConfig(base)
	if input == nil {
		return config
	}
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	if input.CurrentWindowSeconds != nil {
		config.CurrentWindowSeconds = *input.CurrentWindowSeconds
	}
	if input.BaselineWindowSeconds != nil {
		config.BaselineWindowSeconds = *input.BaselineWindowSeconds
	}
	if input.CooldownSeconds != nil {
		config.CooldownSeconds = *input.CooldownSeconds
	}
	if input.MinCurrentRequests != nil {
		config.MinCurrentRequests = *input.MinCurrentRequests
	}
	if input.MinBaselineRequests != nil {
		config.MinBaselineRequests = *input.MinBaselineRequests
	}
	if input.RequestSpikeRatio != nil {
		config.RequestSpikeRatio = *input.RequestSpikeRatio
	}
	if input.MinCurrentQuota != nil {
		config.MinCurrentQuota = *input.MinCurrentQuota
	}
	if input.MinBaselineQuota != nil {
		config.MinBaselineQuota = *input.MinBaselineQuota
	}
	if input.CostSpikeRatio != nil {
		config.CostSpikeRatio = *input.CostSpikeRatio
	}
	if input.MinFailureRequests != nil {
		config.MinFailureRequests = *input.MinFailureRequests
	}
	if input.MinFailures != nil {
		config.MinFailures = *input.MinFailures
	}
	if input.FailureRate != nil {
		config.FailureRate = *input.FailureRate
	}
	return normalizeEnterpriseAnomalyThrottleConfig(config)
}

func EnterpriseAnomalyThrottleConfigJSON(config EnterpriseAnomalyThrottleConfig) (string, error) {
	data, err := common.Marshal(normalizeEnterpriseAnomalyThrottleConfig(config))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeEnterpriseAnomalyThrottleConfig(config EnterpriseAnomalyThrottleConfig) EnterpriseAnomalyThrottleConfig {
	defaults := DefaultEnterpriseAnomalyThrottleConfig()
	if config.CurrentWindowSeconds <= 0 {
		config.CurrentWindowSeconds = defaults.CurrentWindowSeconds
	}
	if config.BaselineWindowSeconds <= 0 {
		config.BaselineWindowSeconds = defaults.BaselineWindowSeconds
	}
	if config.CooldownSeconds <= 0 {
		config.CooldownSeconds = defaults.CooldownSeconds
	}
	if config.MinCurrentRequests <= 0 {
		config.MinCurrentRequests = defaults.MinCurrentRequests
	}
	if config.MinBaselineRequests <= 0 {
		config.MinBaselineRequests = defaults.MinBaselineRequests
	}
	if config.RequestSpikeRatio <= 0 {
		config.RequestSpikeRatio = defaults.RequestSpikeRatio
	}
	if config.MinCurrentQuota <= 0 {
		config.MinCurrentQuota = defaults.MinCurrentQuota
	}
	if config.MinBaselineQuota <= 0 {
		config.MinBaselineQuota = defaults.MinBaselineQuota
	}
	if config.CostSpikeRatio <= 0 {
		config.CostSpikeRatio = defaults.CostSpikeRatio
	}
	if config.MinFailureRequests <= 0 {
		config.MinFailureRequests = defaults.MinFailureRequests
	}
	if config.MinFailures <= 0 {
		config.MinFailures = defaults.MinFailures
	}
	if config.FailureRate <= 0 {
		config.FailureRate = defaults.FailureRate
	}
	if config.FailureRate > 1 {
		config.FailureRate = 1
	}
	return config
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
	config, err := enterpriseAnomalyThrottleConfigForContext(enterpriseCtx)
	if err != nil {
		return result, err
	}
	if !config.Enabled {
		return result, nil
	}

	now := time.Now()
	protectionKey := enterpriseAnomalyProtectionKey(enterpriseCtx)
	if protection, ok := loadEnterpriseAnomalyProtection(enterpriseCtx, protectionKey, now); ok {
		result = enterpriseAnomalyResultFromProtection(protection, enterpriseCtx.DryRun)
		currentPolicyActions, hasCurrentPolicyDecision := enterpriseAnomalyPolicyActionsForContext(c)
		if hasCurrentPolicyDecision {
			result.PolicyActions = currentPolicyActions
		}
		orchestrationAction := enterpriseAnomalyOrchestrationAction(result.PolicyActions)
		orchestrated := !enterpriseCtx.DryRun && orchestrationAction != ""
		if orchestrated {
			result.Status = enterpriseAnomalyStatusOrchestrated
			result.OrchestrationAction = orchestrationAction
		}
		setEnterpriseAnomalyThrottleHeaders(c, result)
		recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
		if enterpriseCtx.DryRun {
			return result, nil
		}
		if orchestrated {
			logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly orchestrated by %s action: reason=%s protected_until=%d", orchestrationAction, result.Reason, result.ProtectedUntil))
			return result, nil
		}
		return result, ErrEnterpriseGovernanceAnomalyThrottled
	}

	result, err = detectEnterpriseGovernanceAnomalyThrottle(enterpriseCtx, config, now)
	if err != nil {
		return EnterpriseGovernanceAnomalyThrottleResult{}, err
	}
	if !result.Applied {
		return result, nil
	}
	result.PolicyActions, _ = enterpriseAnomalyPolicyActionsForContext(c)
	if enterpriseCtx.DryRun {
		result.Status = enterpriseAnomalyStatusWouldThrottle
		result.DryRun = true
		setEnterpriseAnomalyThrottleHeaders(c, result)
		recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly throttle dry-run observed: reason=%s", result.Reason))
		return result, nil
	}

	protectionScopeType, protectionScopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	protection := enterpriseAnomalyProtection{
		ScopeType:      protectionScopeType,
		ScopeId:        protectionScopeId,
		Reason:         result.Reason,
		Triggers:       append([]EnterpriseGovernanceAnomalyTrigger(nil), result.Triggers...),
		Current:        result.Current,
		Baseline:       result.Baseline,
		DetectedAt:     result.DetectedAt,
		ProtectedUntil: result.ProtectedUntil,
		PolicyActions:  cloneEnterprisePolicyActionObservations(result.PolicyActions),
	}
	enterpriseAnomalyProtections.Store(protectionKey, protection)
	persistEnterpriseAnomalyProtection(c, enterpriseCtx, protectionKey, protection)
	if orchestrationAction := enterpriseAnomalyOrchestrationAction(result.PolicyActions); orchestrationAction != "" {
		result.Status = enterpriseAnomalyStatusOrchestrated
		result.OrchestrationAction = orchestrationAction
		setEnterpriseAnomalyThrottleHeaders(c, result)
		recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly orchestrated by %s action: reason=%s protected_until=%d", orchestrationAction, result.Reason, result.ProtectedUntil))
		return result, nil
	}
	setEnterpriseAnomalyThrottleHeaders(c, result)
	recordEnterpriseGovernanceAnomalyThrottleAudit(c, enterpriseCtx, relayInfo, result)
	logger.LogWarn(c, fmt.Sprintf("enterprise governance anomaly throttle activated: reason=%s protected_until=%d", result.Reason, result.ProtectedUntil))
	return result, ErrEnterpriseGovernanceAnomalyThrottled
}

func detectEnterpriseGovernanceAnomalyThrottle(enterpriseCtx *EnterpriseContext, config EnterpriseAnomalyThrottleConfig, now time.Time) (EnterpriseGovernanceAnomalyThrottleResult, error) {
	result := EnterpriseGovernanceAnomalyThrottleResult{}
	config = normalizeEnterpriseAnomalyThrottleConfig(config)
	currentWindow := time.Duration(config.CurrentWindowSeconds) * time.Second
	baselineWindow := time.Duration(config.BaselineWindowSeconds) * time.Second
	cooldown := time.Duration(config.CooldownSeconds) * time.Second
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

	triggers := enterpriseAnomalyTriggers(current, baseline, currentWindow, baselineWindow, config)
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

func enterpriseAnomalyTriggers(current EnterpriseGovernanceAnomalyUsageSnapshot, baseline EnterpriseGovernanceAnomalyUsageSnapshot, currentWindow time.Duration, baselineWindow time.Duration, config EnterpriseAnomalyThrottleConfig) []EnterpriseGovernanceAnomalyTrigger {
	triggers := make([]EnterpriseGovernanceAnomalyTrigger, 0, 3)
	config = normalizeEnterpriseAnomalyThrottleConfig(config)
	if current.RequestCount >= config.MinFailureRequests &&
		current.ErrorCount >= config.MinFailures {
		failureRate := safeRatio(current.ErrorCount, current.RequestCount)
		if failureRate >= config.FailureRate {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonFailureRate,
				CurrentValue:  current.ErrorCount,
				BaselineValue: baseline.ErrorCount,
				CurrentRate:   failureRate,
				BaselineRate:  safeRatio(baseline.ErrorCount, baseline.RequestCount),
				Ratio:         failureRate,
				Threshold:     config.FailureRate,
			})
		}
	}
	if current.Quota >= config.MinCurrentQuota &&
		baseline.Quota >= config.MinBaselineQuota {
		currentRate := ratePerSecond(current.Quota, currentWindow)
		baselineRate := ratePerSecond(baseline.Quota, baselineWindow)
		ratio := currentRate / baselineRate
		if baselineRate > 0 && ratio >= config.CostSpikeRatio {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonCostSpike,
				CurrentValue:  current.Quota,
				BaselineValue: baseline.Quota,
				CurrentRate:   currentRate,
				BaselineRate:  baselineRate,
				Ratio:         ratio,
				Threshold:     config.CostSpikeRatio,
			})
		}
	}
	if current.RequestCount >= config.MinCurrentRequests &&
		baseline.RequestCount >= config.MinBaselineRequests {
		currentRate := ratePerSecond(current.RequestCount, currentWindow)
		baselineRate := ratePerSecond(baseline.RequestCount, baselineWindow)
		ratio := currentRate / baselineRate
		if baselineRate > 0 && ratio >= config.RequestSpikeRatio {
			triggers = append(triggers, EnterpriseGovernanceAnomalyTrigger{
				Reason:        enterpriseAnomalyReasonRequestSpike,
				CurrentValue:  current.RequestCount,
				BaselineValue: baseline.RequestCount,
				CurrentRate:   currentRate,
				BaselineRate:  baselineRate,
				Ratio:         ratio,
				Threshold:     config.RequestSpikeRatio,
			})
		}
	}
	return triggers
}

func enterpriseAnomalyThrottleConfigForContext(enterpriseCtx *EnterpriseContext) (EnterpriseAnomalyThrottleConfig, error) {
	config := DefaultEnterpriseAnomalyThrottleConfig()
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 || model.DB == nil {
		return config, nil
	}
	var enterprise model.Enterprise
	if err := model.DB.Select("anomaly_throttle_config_json").Where("id = ?", enterpriseCtx.EnterpriseId).First(&enterprise).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return config, nil
		}
		return config, err
	}
	return EnterpriseAnomalyThrottleConfigFromJSON(enterprise.AnomalyThrottleConfigJson), nil
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
	query := model.DB.Model(&model.EnterpriseUsageAttribution{}).
		Select("COUNT(*) AS request_count, COALESCE(SUM(quota), 0) AS quota").
		Where("enterprise_id = ? AND created_at >= ? AND created_at < ?", enterpriseCtx.EnterpriseId, start.Unix(), end.Unix())
	scopeType, scopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	switch scopeType {
	case model.EnterpriseGovernanceAnomalyProtectionScopeProject:
		query = query.Where("project_id = ?", scopeId)
	case model.EnterpriseGovernanceAnomalyProtectionScopeOrgUnit:
		query = query.Where("org_unit_id = ?", scopeId)
	}
	if err := query.Scan(&usage).Error; err != nil {
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
	userIds, scoped, err := enterpriseAnomalyUserIds(enterpriseCtx)
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
	if scoped || len(userIds) > 0 {
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

func enterpriseAnomalyUserIds(enterpriseCtx *EnterpriseContext) ([]int, bool, error) {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 {
		return nil, false, nil
	}
	var userIds []int
	scopeType, scopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	query := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ?", enterpriseCtx.EnterpriseId).
		Where("is_primary = ?", true)
	scoped := false
	switch scopeType {
	case model.EnterpriseGovernanceAnomalyProtectionScopeProject:
		orgUnitIds := model.DB.Model(&model.EnterpriseProjectOrgUnit{}).
			Select("org_unit_id").
			Where("enterprise_id = ? AND project_id = ?", enterpriseCtx.EnterpriseId, scopeId)
		query = query.Where("org_unit_id IN (?)", orgUnitIds)
		scoped = true
	case model.EnterpriseGovernanceAnomalyProtectionScopeOrgUnit:
		query = query.Where("org_unit_id = ?", scopeId)
		scoped = true
	}
	if err := query.Pluck("user_id", &userIds).Error; err != nil {
		return nil, scoped, err
	}
	return userIds, scoped, nil
}

func loadEnterpriseAnomalyProtection(enterpriseCtx *EnterpriseContext, key string, now time.Time) (enterpriseAnomalyProtection, bool) {
	value, ok := enterpriseAnomalyProtections.Load(key)
	if !ok {
		return loadEnterpriseAnomalyProtectionFromDB(enterpriseCtx, key, now)
	}
	protection, ok := value.(enterpriseAnomalyProtection)
	if !ok || protection.ProtectedUntil <= now.Unix() {
		enterpriseAnomalyProtections.Delete(key)
		expireEnterpriseAnomalyProtection(enterpriseCtx, key, now)
		return enterpriseAnomalyProtection{}, false
	}
	return protection, true
}

func persistEnterpriseAnomalyProtection(c *gin.Context, enterpriseCtx *EnterpriseContext, key string, protection enterpriseAnomalyProtection) {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 || model.DB == nil {
		return
	}
	payload := enterpriseAnomalyProtectionPayload{
		ScopeType:      protection.ScopeType,
		ScopeId:        protection.ScopeId,
		Reason:         protection.Reason,
		Triggers:       append([]EnterpriseGovernanceAnomalyTrigger(nil), protection.Triggers...),
		Current:        protection.Current,
		Baseline:       protection.Baseline,
		DetectedAt:     protection.DetectedAt,
		ProtectedUntil: protection.ProtectedUntil,
		PolicyActions:  cloneEnterprisePolicyActionObservations(protection.PolicyActions),
	}
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		logger.LogError(c, "error marshaling enterprise anomaly protection: "+err.Error())
		return
	}
	now := common.GetTimestamp()
	if err := model.DB.Model(&model.EnterpriseGovernanceAnomalyProtection{}).
		Where("enterprise_id = ? AND protection_key = ? AND status = ?", enterpriseCtx.EnterpriseId, key, model.EnterpriseGovernanceAnomalyProtectionStatusActive).
		Updates(map[string]any{
			"status":     model.EnterpriseGovernanceAnomalyProtectionStatusExpired,
			"updated_at": now,
		}).Error; err != nil {
		logger.LogError(c, "error expiring previous enterprise anomaly protections: "+err.Error())
	}
	row := model.EnterpriseGovernanceAnomalyProtection{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ProtectionKey:  key,
		ScopeType:      protection.ScopeType,
		ScopeId:        protection.ScopeId,
		Reason:         protection.Reason,
		Status:         model.EnterpriseGovernanceAnomalyProtectionStatusActive,
		DetectedAt:     protection.DetectedAt,
		ProtectedUntil: protection.ProtectedUntil,
		PayloadJson:    string(payloadBytes),
	}
	if err := model.DB.Create(&row).Error; err != nil {
		logger.LogError(c, "error recording enterprise anomaly protection: "+err.Error())
	}
}

func loadEnterpriseAnomalyProtectionFromDB(enterpriseCtx *EnterpriseContext, key string, now time.Time) (enterpriseAnomalyProtection, bool) {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 || model.DB == nil {
		return enterpriseAnomalyProtection{}, false
	}
	var row model.EnterpriseGovernanceAnomalyProtection
	if err := model.DB.Where(
		"enterprise_id = ? AND protection_key = ? AND status = ? AND protected_until > ?",
		enterpriseCtx.EnterpriseId,
		key,
		model.EnterpriseGovernanceAnomalyProtectionStatusActive,
		now.Unix(),
	).Order("protected_until desc, id desc").First(&row).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			common.SysError("error loading enterprise anomaly protection: " + err.Error())
		}
		expireEnterpriseAnomalyProtection(enterpriseCtx, key, now)
		return enterpriseAnomalyProtection{}, false
	}
	var payload enterpriseAnomalyProtectionPayload
	if err := common.Unmarshal([]byte(row.PayloadJson), &payload); err != nil {
		common.SysError("error unmarshaling enterprise anomaly protection: " + err.Error())
		expireEnterpriseAnomalyProtection(enterpriseCtx, key, now)
		return enterpriseAnomalyProtection{}, false
	}
	protection := enterpriseAnomalyProtection{
		ScopeType:      row.ScopeType,
		ScopeId:        row.ScopeId,
		Reason:         payload.Reason,
		Triggers:       append([]EnterpriseGovernanceAnomalyTrigger(nil), payload.Triggers...),
		Current:        payload.Current,
		Baseline:       payload.Baseline,
		DetectedAt:     payload.DetectedAt,
		ProtectedUntil: payload.ProtectedUntil,
		PolicyActions:  cloneEnterprisePolicyActionObservations(payload.PolicyActions),
	}
	if protection.ProtectedUntil <= now.Unix() {
		expireEnterpriseAnomalyProtection(enterpriseCtx, key, now)
		return enterpriseAnomalyProtection{}, false
	}
	enterpriseAnomalyProtections.Store(key, protection)
	return protection, true
}

func expireEnterpriseAnomalyProtection(enterpriseCtx *EnterpriseContext, key string, now time.Time) {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 || key == "" || model.DB == nil {
		return
	}
	_ = model.DB.Model(&model.EnterpriseGovernanceAnomalyProtection{}).
		Where("enterprise_id = ? AND protection_key = ? AND status = ? AND protected_until <= ?", enterpriseCtx.EnterpriseId, key, model.EnterpriseGovernanceAnomalyProtectionStatusActive, now.Unix()).
		Updates(map[string]any{
			"status":     model.EnterpriseGovernanceAnomalyProtectionStatusExpired,
			"updated_at": now.Unix(),
		}).Error
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
		PolicyActions:   cloneEnterprisePolicyActionObservations(protection.PolicyActions),
		DryRun:          dryRun,
	}
}

func enterpriseAnomalyPolicyActionsForContext(c *gin.Context) ([]PolicyActionObservation, bool) {
	if c == nil {
		return []PolicyActionObservation{}, false
	}
	decision, ok := common.GetContextKeyType[PolicyDecision](c, constant.ContextKeyEnterpriseGovernanceDecision)
	if !ok {
		return []PolicyActionObservation{}, false
	}
	return cloneEnterprisePolicyActionObservations(decision.ActionObservations), true
}

func enterpriseAnomalyHasQueuePolicyAction(actions []PolicyActionObservation) bool {
	return enterpriseAnomalyOrchestrationAction(actions) == model.PolicyActionQueue
}

// enterpriseAnomalyOrchestrationAction picks the first non-reject policy action that
// can continue the relay path instead of hard-throttling. Priority:
// queue > fallback_model > shared_pool > alert.
func enterpriseAnomalyOrchestrationAction(actions []PolicyActionObservation) string {
	priority := []string{
		model.PolicyActionQueue,
		model.PolicyActionFallbackModel,
		model.PolicyActionSharedPool,
		model.PolicyActionAlert,
	}
	seen := map[string]struct{}{}
	for _, action := range actions {
		name := strings.TrimSpace(action.Action)
		if name == "" || name == model.PolicyActionReject {
			continue
		}
		seen[name] = struct{}{}
	}
	for _, name := range priority {
		if _, ok := seen[name]; ok {
			return name
		}
	}
	return ""
}

func enterpriseAnomalyProtectionScope(enterpriseCtx *EnterpriseContext) (string, int) {
	if enterpriseCtx == nil {
		return model.EnterpriseGovernanceAnomalyProtectionScopeEnterprise, 0
	}
	if enterpriseCtx.ProjectId > 0 {
		return model.EnterpriseGovernanceAnomalyProtectionScopeProject, enterpriseCtx.ProjectId
	}
	if enterpriseCtx.PrimaryOrgUnitId > 0 {
		return model.EnterpriseGovernanceAnomalyProtectionScopeOrgUnit, enterpriseCtx.PrimaryOrgUnitId
	}
	return model.EnterpriseGovernanceAnomalyProtectionScopeEnterprise, enterpriseCtx.EnterpriseId
}

func enterpriseAnomalyProtectionKey(enterpriseCtx *EnterpriseContext) string {
	if enterpriseCtx == nil || enterpriseCtx.EnterpriseId <= 0 {
		return "enterprise:unknown"
	}
	scopeType, scopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	switch scopeType {
	case model.EnterpriseGovernanceAnomalyProtectionScopeProject:
		return fmt.Sprintf("project:%d:%d", enterpriseCtx.EnterpriseId, scopeId)
	case model.EnterpriseGovernanceAnomalyProtectionScopeOrgUnit:
		return fmt.Sprintf("org_unit:%d:%d", enterpriseCtx.EnterpriseId, scopeId)
	default:
		return fmt.Sprintf("enterprise:%d", enterpriseCtx.EnterpriseId)
	}
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
	scopeType, scopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	targetType := scopeType
	targetId := scopeId
	if targetType == model.EnterpriseGovernanceAnomalyProtectionScopeEnterprise {
		targetId = enterpriseCtx.EnterpriseId
	}
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ActorUserId:    enterpriseCtx.UserId,
		Action:         enterpriseGovernanceAuditActionAnomalyThrottle,
		TargetType:     targetType,
		TargetId:       targetId,
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
	scopeType, scopeId := enterpriseAnomalyProtectionScope(enterpriseCtx)
	payload := map[string]any{
		"request_id":       requestId,
		"model":            modelName,
		"channel_id":       channelId,
		"token_id":         enterpriseCtx.TokenId,
		"org_unit_id":      enterpriseCtx.PrimaryOrgUnitId,
		"project_id":       enterpriseCtx.ProjectId,
		"scope_type":       scopeType,
		"scope_id":         scopeId,
		"protection_key":   enterpriseAnomalyProtectionKey(enterpriseCtx),
		"policy_group_ids": cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"policy_actions":   cloneEnterprisePolicyActionObservations(result.PolicyActions),
		"anomaly_status":   result.Status,
		"anomaly_reason":   result.Reason,
		"anomaly_triggers": result.Triggers,
		"current_window":   result.Current,
		"baseline_window":  result.Baseline,
		"detected_at":      result.DetectedAt,
		"protected_until":  result.ProtectedUntil,
		"cooldown_seconds": result.CooldownSeconds,
		"user_message_key": enterpriseAnomalyAuditUserMessageKey(result),
		"error_code":       enterpriseAnomalyAuditErrorCode(result),
		"dry_run":          result.DryRun,
	}
	if result.Status == enterpriseAnomalyStatusOrchestrated {
		action := strings.TrimSpace(result.OrchestrationAction)
		if action == "" {
			action = enterpriseAnomalyOrchestrationAction(result.PolicyActions)
		}
		if action == "" {
			action = model.PolicyActionQueue
		}
		payload["orchestration_action"] = action
	}
	return payload
}

func enterpriseAnomalyAuditUserMessageKey(result EnterpriseGovernanceAnomalyThrottleResult) string {
	if result.Status == enterpriseAnomalyStatusOrchestrated {
		return "enterprise_governance.anomaly_orchestrated"
	}
	return "enterprise_governance.anomaly_throttled"
}

func enterpriseAnomalyAuditErrorCode(result EnterpriseGovernanceAnomalyThrottleResult) string {
	if result.Status == enterpriseAnomalyStatusOrchestrated {
		return "enterprise_governance_anomaly_orchestrated"
	}
	return "enterprise_governance_anomaly_throttled"
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
