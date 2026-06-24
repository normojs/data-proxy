package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
)

const (
	tunnelBillingSource      = "tunnel"
	tunnelBillingUnitAudit   = "audit"
	tunnelBillingUnitPerCall = "per_call"
	tunnelBillingUnitTraffic = "request_traffic"
)

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
	return model.RecordFundingBillingEvent(nil, model.FundingBillingEventInput{
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
	})
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
