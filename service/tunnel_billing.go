package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
)

const (
	tunnelBillingSource      = "tunnel"
	tunnelBillingUnitAudit   = "audit"
	tunnelBillingUnitPerCall = "per_call"
	tunnelBillingUnitTraffic = "request_traffic"
	tunnelBillingUnitPolicy  = "policy"

	tunnelBillingSettlementPhase      = "settlement"
	tunnelBillingMiB                  = 1024 * 1024
	tunnelBillingReasonInsufficient   = "billing_insufficient"
	tunnelBillingReasonOverdueDisable = "billing_overdue_disabled"
)

var ErrTunnelBillingInsufficient = errors.New("tunnel billing insufficient balance")

type tunnelBillingPolicy struct {
	Enabled                  bool
	PriceUnit                string
	QuotaPerCall             float64
	QuotaPerRequest          float64
	QuotaPerMiBIn            float64
	QuotaPerMiBOut           float64
	QuotaPerSecond           float64
	MinQuota                 int
	ChargeDenied             bool
	DeductBalance            bool
	RequirePositiveBalance   bool
	RequireSufficientBalance bool
	AutoDisableOnOverdue     bool
	EstimatedMinQuota        int
}

func recordTunnelAuditBillingEvent(source string, app model.TunnelApp, connection model.TunnelConnection, audit model.TunnelAuditLog) error {
	if audit.Id <= 0 || strings.TrimSpace(source) == "" {
		return nil
	}
	priceUnit := tunnelAuditBillingPriceUnit(audit)
	if configuredUnit := tunnelBillingConfiguredPriceUnit(app.BillingJson); configuredUnit != "" {
		priceUnit = configuredUnit
	}
	metadata := map[string]any{
		"usage_kind":            model.BillingEventUsageKindTunnel,
		"app_id":                app.Id,
		"app_type":              app.AppType,
		"public_slug":           app.PublicSlug,
		"connection_id":         connection.Id,
		"connection_key_prefix": connection.KeyPrefix,
		"permission_mode":       effectiveTunnelMCPPermissionMode(app, connection),
		"audit_log_id":          audit.Id,
		"action":                audit.Action,
		"decision":              audit.Decision,
		"reason":                audit.Reason,
		"method":                audit.Method,
		"path":                  audit.Path,
		"tool_name":             audit.ToolName,
		"bytes_in":              audit.BytesIn,
		"bytes_out":             audit.BytesOut,
		"duration_ms":           audit.DurationMS,
		"session_id":            audit.SessionId,
		"billing_configured":    strings.TrimSpace(app.BillingJson) != "",
	}
	if err := model.RecordFundingBillingEvent(nil, model.FundingBillingEventInput{
		Source:        source,
		SourceId:      fmt.Sprintf("audit:%d", audit.Id),
		Phase:         tunnelAuditBillingPhase(audit),
		UserId:        app.UserId,
		TokenId:       connection.AgentTokenId,
		RequestId:     audit.RequestId,
		BillingSource: tunnelBillingSource,
		PriceUnit:     priceUnit,
		EventType:     model.BillingEventTypeAudit,
		AllowZero:     true,
		Metadata:      metadata,
	}); err != nil {
		return err
	}

	policy := tunnelBillingPolicyFromJSON(app.BillingJson)
	amount, settlementMeta := tunnelAuditSettlementQuota(app, audit, policy)
	if amount <= 0 {
		return nil
	}
	settlementMetadata := copyTunnelBillingMetadata(metadata)
	for key, value := range settlementMeta {
		settlementMetadata[key] = value
	}
	settlementMetadata["billing_configured"] = true
	settlementMetadata["billing_settlement"] = true
	if policy.PriceUnit != "" {
		priceUnit = policy.PriceUnit
	} else if priceUnit == "" || priceUnit == tunnelBillingUnitAudit {
		priceUnit = tunnelBillingUnitPolicy
	}
	settlementInput := model.FundingBillingEventInput{
		Source:        source,
		SourceId:      fmt.Sprintf("audit:%d", audit.Id),
		Phase:         tunnelBillingSettlementPhase,
		UserId:        app.UserId,
		TokenId:       connection.AgentTokenId,
		RequestId:     audit.RequestId,
		BillingSource: tunnelBillingSource,
		PriceUnit:     priceUnit,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   amount,
		Metadata:      settlementMetadata,
	}
	if policy.DeductBalance {
		created, err := model.RecordFundingDebitEventAndDecreaseUserQuotaIfNotExists(settlementInput)
		if err != nil {
			return err
		}
		if created {
			return maybeDisableTunnelAppForBillingOverdue(source, app, connection, audit, policy, amount)
		}
		return err
	}
	_, err := model.RecordFundingBillingEventIfNotExists(nil, settlementInput)
	return err
}

func tunnelAuditBillingPhase(audit model.TunnelAuditLog) string {
	action := normalizeTunnelBillingPart(audit.Action)
	if action == "" {
		action = "request"
	}
	decision := normalizeTunnelBillingPart(audit.Decision)
	if decision == "" {
		decision = "unknown"
	}
	return action + "_" + decision
}

func tunnelAuditBillingPriceUnit(audit model.TunnelAuditLog) string {
	if audit.Action == model.TunnelAuditActionMCPToolCall && audit.Decision == mcpgateway.DecisionAllow {
		return tunnelBillingUnitPerCall
	}
	if audit.Action == model.TunnelAuditActionProxyRequest {
		return tunnelBillingUnitTraffic
	}
	return tunnelBillingUnitAudit
}

func normalizeTunnelBillingPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.NewReplacer(" ", "_", "/", "_", ":", "_").Replace(value)
	return strings.Trim(value, "_")
}

func tunnelBillingConfiguredPriceUnit(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return ""
	}
	for _, key := range []string{"price_unit", "unit"} {
		if value, ok := body[key].(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return limitTunnelString(value, 32)
			}
		}
	}
	return ""
}

func tunnelBillingPolicyFromJSON(raw string) tunnelBillingPolicy {
	values := unmarshalTunnelMap(raw)
	if len(values) == 0 {
		return tunnelBillingPolicy{}
	}
	if nested := mapFromAny(values["settlement"]); len(nested) > 0 {
		values = nested
	}
	policy := tunnelBillingPolicy{
		Enabled:         boolFromAny(values["enabled"]),
		PriceUnit:       limitTunnelString(stringFromAny(values["price_unit"]), 32),
		QuotaPerCall:    sanitizeTunnelBillingRate(float64FromAny(values["quota_per_call"])),
		QuotaPerRequest: sanitizeTunnelBillingRate(float64FromAny(values["quota_per_request"])),
		QuotaPerMiBIn:   sanitizeTunnelBillingRate(firstPositiveFloat(values["quota_per_mib_in"], values["quota_per_mb_in"])),
		QuotaPerMiBOut:  sanitizeTunnelBillingRate(firstPositiveFloat(values["quota_per_mib_out"], values["quota_per_mb_out"])),
		QuotaPerSecond:  sanitizeTunnelBillingRate(float64FromAny(values["quota_per_second"])),
		MinQuota:        sanitizeTunnelBillingMinQuota(intFromAny(values["min_quota"])),
		ChargeDenied:    boolFromAny(values["charge_denied"]),
		DeductBalance:   true,
		RequirePositiveBalance: boolFromAny(firstNonNilAny(
			values["require_positive_balance"],
			values["require_balance"],
			values["deny_when_insufficient"],
		)),
		RequireSufficientBalance: boolFromAny(firstNonNilAny(
			values["require_sufficient_balance"],
			values["require_balance_for_minimum"],
			values["deny_when_estimated_insufficient"],
		)),
		AutoDisableOnOverdue: boolFromAny(firstNonNilAny(
			values["auto_disable_on_overdue"],
			values["disable_when_overdue"],
			values["disable_on_negative_balance"],
		)),
	}
	if value, ok := values["deduct_balance"]; ok {
		policy.DeductBalance = boolFromAny(value)
	}
	if boolFromAny(values["ledger_only"]) {
		policy.DeductBalance = false
	}
	if policy.PriceUnit == "" {
		policy.PriceUnit = limitTunnelString(stringFromAny(values["unit"]), 32)
	}
	return policy
}

func checkTunnelBillingPositiveBalance(app model.TunnelApp) (tunnelBillingPolicy, int, error) {
	return checkTunnelBillingBalance(app, 0)
}

func checkTunnelBillingBalance(app model.TunnelApp, estimatedMinQuota int) (tunnelBillingPolicy, int, error) {
	policy := tunnelBillingPolicyFromJSON(app.BillingJson)
	policy.EstimatedMinQuota = sanitizeTunnelBillingMinQuota(estimatedMinQuota)
	if !tunnelBillingRequiresPositiveBalance(policy) {
		return policy, 0, nil
	}
	quota, err := model.GetUserQuota(app.UserId, true)
	if err != nil {
		return policy, 0, err
	}
	if quota <= 0 {
		return policy, quota, fmt.Errorf("%w: user %d quota is %d", ErrTunnelBillingInsufficient, app.UserId, quota)
	}
	if tunnelBillingRequiresSufficientBalance(policy) && policy.EstimatedMinQuota > 0 && quota < policy.EstimatedMinQuota {
		return policy, quota, fmt.Errorf("%w: user %d quota %d is below estimated minimum %d", ErrTunnelBillingInsufficient, app.UserId, quota, policy.EstimatedMinQuota)
	}
	return policy, quota, nil
}

func tunnelBillingRequiresPositiveBalance(policy tunnelBillingPolicy) bool {
	return policy.Enabled && policy.DeductBalance && policy.RequirePositiveBalance
}

func tunnelBillingRequiresSufficientBalance(policy tunnelBillingPolicy) bool {
	return tunnelBillingRequiresPositiveBalance(policy) && policy.RequireSufficientBalance
}

func tunnelBillingDenyMetadata(policy tunnelBillingPolicy, quota int, err error) map[string]any {
	metadata := map[string]any{
		"reason":                     tunnelBillingReasonInsufficient,
		"current_quota":              quota,
		"settlement_policy_enabled":  policy.Enabled,
		"settlement_balance_debit":   policy.DeductBalance,
		"require_positive_balance":   policy.RequirePositiveBalance,
		"require_sufficient_balance": policy.RequireSufficientBalance,
	}
	if policy.PriceUnit != "" {
		metadata["price_unit"] = policy.PriceUnit
	}
	if policy.EstimatedMinQuota > 0 {
		metadata["estimated_min_quota"] = policy.EstimatedMinQuota
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	return metadata
}

func tunnelBillingPreflightQuota(app model.TunnelApp, action string, bytesIn int64) int {
	policy := tunnelBillingPolicyFromJSON(app.BillingJson)
	if !policy.Enabled {
		return 0
	}
	amount, _ := tunnelAuditSettlementQuota(app, model.TunnelAuditLog{
		Action:   action,
		Decision: mcpgateway.DecisionAllow,
		BytesIn:  bytesIn,
	}, policy)
	return amount
}

func tunnelAuditSettlementQuota(app model.TunnelApp, audit model.TunnelAuditLog, policy tunnelBillingPolicy) (int, map[string]any) {
	if !policy.Enabled {
		return 0, nil
	}
	if audit.Decision != mcpgateway.DecisionAllow && !policy.ChargeDenied {
		return 0, nil
	}
	var quota float64
	components := map[string]any{}
	switch audit.Action {
	case model.TunnelAuditActionMCPToolCall:
		quota += policy.QuotaPerCall
		if policy.QuotaPerCall > 0 {
			components["quota_per_call"] = policy.QuotaPerCall
		}
	case model.TunnelAuditActionProxyRequest:
		quota += policy.QuotaPerRequest
		if policy.QuotaPerRequest > 0 {
			components["quota_per_request"] = policy.QuotaPerRequest
		}
		if app.AppType == model.TunnelAppTypeHTTP || app.AppType == model.TunnelAppTypeTCP {
			bytesInMiB := float64(maxTunnelBillingInt64(audit.BytesIn, 0)) / tunnelBillingMiB
			bytesOutMiB := float64(maxTunnelBillingInt64(audit.BytesOut, 0)) / tunnelBillingMiB
			durationSeconds := float64(maxTunnelBillingInt(audit.DurationMS, 0)) / 1000
			if policy.QuotaPerMiBIn > 0 && bytesInMiB > 0 {
				quota += policy.QuotaPerMiBIn * bytesInMiB
				components["quota_per_mib_in"] = policy.QuotaPerMiBIn
				components["billed_mib_in"] = bytesInMiB
			}
			if policy.QuotaPerMiBOut > 0 && bytesOutMiB > 0 {
				quota += policy.QuotaPerMiBOut * bytesOutMiB
				components["quota_per_mib_out"] = policy.QuotaPerMiBOut
				components["billed_mib_out"] = bytesOutMiB
			}
			if policy.QuotaPerSecond > 0 && durationSeconds > 0 {
				quota += policy.QuotaPerSecond * durationSeconds
				components["quota_per_second"] = policy.QuotaPerSecond
				components["billed_seconds"] = durationSeconds
			}
		}
	default:
		return 0, nil
	}
	amount := ceilTunnelBillingQuota(quota)
	if amount > 0 && policy.MinQuota > 0 && amount < policy.MinQuota {
		components["min_quota"] = policy.MinQuota
		amount = policy.MinQuota
	}
	if amount <= 0 {
		return 0, nil
	}
	components["settlement_policy_enabled"] = true
	components["settlement_balance_debit"] = policy.DeductBalance
	components["settlement_amount_quota"] = amount
	components["settlement_phase"] = tunnelBillingSettlementPhase
	return amount, components
}

func maybeDisableTunnelAppForBillingOverdue(source string, app model.TunnelApp, connection model.TunnelConnection, audit model.TunnelAuditLog, policy tunnelBillingPolicy, settlementAmount int) error {
	if !policy.AutoDisableOnOverdue || app.Id <= 0 || app.UserId <= 0 || model.DB == nil {
		return nil
	}
	quota, err := model.GetUserQuota(app.UserId, true)
	if err != nil {
		return err
	}
	if quota > 0 {
		return nil
	}
	now := common.GetTimestamp()
	message := fmt.Sprintf("Tunnel App auto-disabled because user quota is %d after tunnel settlement", quota)
	result := model.DB.Model(&model.TunnelApp{}).
		Where("id = ? AND status = ?", app.Id, model.TunnelAppStatusApproved).
		Updates(map[string]any{
			"status":     model.TunnelAppStatusDisabled,
			"last_error": limitTunnelString(message, 1024),
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	metadata := map[string]any{
		"reason":                  tunnelBillingReasonOverdueDisable,
		"current_quota":           quota,
		"settlement_amount_quota": settlementAmount,
		"trigger_request_id":      audit.RequestId,
		"trigger_audit_log_id":    audit.Id,
		"connection_id":           connection.Id,
		"connection_key_prefix":   connection.KeyPrefix,
		"auto_disable_on_overdue": true,
	}
	disableAudit := model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		SessionId:           audit.SessionId,
		UserId:              app.UserId,
		ActorUserId:         app.UserId,
		Action:              model.TunnelAuditActionUpdate,
		Decision:            mcpgateway.DecisionAllow,
		Reason:              tunnelBillingReasonOverdueDisable,
		RequestId:           audit.RequestId,
		Path:                audit.Path,
		ToolName:            audit.ToolName,
		MetadataJson:        tunnelAuditMetadataJSON(metadata),
		CreatedAt:           now,
	}
	if err := model.CreateTunnelAuditLog(&disableAudit); err != nil {
		return err
	}
	return recordTunnelAuditBillingEvent(source, app, connection, disableAudit)
}

func copyTunnelBillingMetadata(metadata map[string]any) map[string]any {
	copied := make(map[string]any, len(metadata)+8)
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

func sanitizeTunnelBillingRate(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func sanitizeTunnelBillingMinQuota(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func ceilTunnelBillingQuota(value float64) int {
	if value <= 0 || math.IsNaN(value) {
		return 0
	}
	value = math.Ceil(value)
	maxInt := int(^uint(0) >> 1)
	if value > float64(maxInt) {
		return maxInt
	}
	return int(value)
}

func firstPositiveFloat(values ...any) float64 {
	for _, value := range values {
		parsed := float64FromAny(value)
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}

func firstNonNilAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func float64FromAny(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on", "enabled":
			return true
		default:
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed != 0
	default:
		return false
	}
}

func maxTunnelBillingInt64(value int64, min int64) int64 {
	if value < min {
		return min
	}
	return value
}

func maxTunnelBillingInt(value int, min int) int {
	if value < min {
		return min
	}
	return value
}
