package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
)

const (
	MCPReviewSeverityWarning  = "warning"
	MCPReviewSeverityCritical = "critical"

	MCPReviewCategoryProxyServer  = "proxy_server"
	MCPReviewCategoryBridgeClient = "bridge_client"
	MCPReviewCategoryHealthCheck  = "health_check"
	MCPReviewCategoryHeartbeat    = "heartbeat"
	MCPReviewCategoryTool         = "tool"

	mcpReviewMaxItems                = 50
	mcpReviewProxyServerScanLimit    = 200
	mcpReviewBridgeClientScanLimit   = 200
	mcpReviewToolScanLimit           = 50
	mcpReviewToolMinCalls            = 5
	mcpReviewToolWarnSuccessRate     = 80.0
	mcpReviewToolCriticalSuccessRate = 50.0
)

// mcpProxyCriticalReviewReasons marks which existing proxy review reasons are
// escalated to critical severity inside the unified review queue.
var mcpProxyCriticalReviewReasons = map[string]bool{
	"server_error":        true,
	"latest_check_failed": true,
	"transport_error":     true,
}

type MCPReviewQueueParams struct {
	WindowSeconds int64
	StartTime     int64
}

// BuildMCPReviewQueue aggregates actionable MCP operations signals (proxy
// server health, stale bridge clients, failing background tasks, and
// high-error tools) into a single review queue for the admin dashboard.
func BuildMCPReviewQueue(params MCPReviewQueueParams) (*dto.MCPReviewQueue, error) {
	items := make([]dto.MCPReviewItem, 0)

	proxyItems, err := collectMCPProxyServerReviewItems()
	if err != nil {
		return nil, err
	}
	items = append(items, proxyItems...)

	bridgeItems, err := collectStaleBridgeClientReviewItems()
	if err != nil {
		return nil, err
	}
	items = append(items, bridgeItems...)

	items = append(items, collectMCPProxyTaskReviewItems()...)

	toolItems, err := collectHighErrorToolReviewItems(params.StartTime)
	if err != nil {
		return nil, err
	}
	items = append(items, toolItems...)

	return assembleMCPReviewQueue(items), nil
}

func assembleMCPReviewQueue(items []dto.MCPReviewItem) *dto.MCPReviewQueue {
	critical := 0
	warning := 0
	for _, item := range items {
		if item.Severity == MCPReviewSeverityCritical {
			critical++
		} else {
			warning++
		}
	}
	// Critical first; preserve collection order within the same severity.
	sort.SliceStable(items, func(i, j int) bool {
		return mcpReviewSeverityRank(items[i].Severity) < mcpReviewSeverityRank(items[j].Severity)
	})
	total := len(items)
	if len(items) > mcpReviewMaxItems {
		items = items[:mcpReviewMaxItems]
	}
	return &dto.MCPReviewQueue{
		Total:         total,
		CriticalCount: critical,
		WarningCount:  warning,
		Items:         items,
	}
}

func mcpReviewSeverityRank(severity string) int {
	if severity == MCPReviewSeverityCritical {
		return 0
	}
	return 1
}

func collectMCPProxyServerReviewItems() ([]dto.MCPReviewItem, error) {
	servers, _, err := ListMCPProxyServersForAdmin(MCPProxyServerListParams{
		Limit: mcpReviewProxyServerScanLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.MCPReviewItem, 0)
	for _, server := range servers {
		if server.Health == nil || !server.Health.NeedsReview {
			continue
		}
		reasons := server.Health.ReviewReasons
		severity := MCPReviewSeverityWarning
		for _, reason := range reasons {
			if mcpProxyCriticalReviewReasons[reason] {
				severity = MCPReviewSeverityCritical
				break
			}
		}
		name := server.Name
		if name == "" {
			name = server.Namespace
		}
		items = append(items, dto.MCPReviewItem{
			Category:   MCPReviewCategoryProxyServer,
			Severity:   severity,
			TargetType: "proxy_server",
			TargetId:   strconv.Itoa(server.Id),
			TargetName: name,
			Reasons:    reasons,
			Detail:     mcpProxyServerReviewDetail(server),
			CreatedAt:  server.Health.GeneratedAt,
		})
	}
	return items, nil
}

func mcpProxyServerReviewDetail(server dto.MCPProxyServerAdminItem) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(server.LastError) != "" {
		parts = append(parts, strings.TrimSpace(server.LastError))
	}
	if server.Health != nil && (server.Health.Calls.ErrorCalls > 0 || server.Health.Calls.TimeoutCalls > 0) {
		parts = append(parts, fmt.Sprintf("errors=%d timeouts=%d",
			server.Health.Calls.ErrorCalls, server.Health.Calls.TimeoutCalls))
	}
	return strings.Join(parts, " | ")
}

func collectStaleBridgeClientReviewItems() ([]dto.MCPReviewItem, error) {
	online := model.BridgeClientStatusOnline
	clients, _, err := model.ListBridgeClients(model.BridgeClientFilter{
		Status: &online,
	}, 0, mcpReviewBridgeClientScanLimit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.MCPReviewItem, 0)
	for _, client := range clients {
		if _, ok := bridge.DefaultHub.GetByClient(client.ClientId); ok {
			continue
		}
		name := client.Name
		if name == "" {
			name = client.ClientId
		}
		items = append(items, dto.MCPReviewItem{
			Category:   MCPReviewCategoryBridgeClient,
			Severity:   MCPReviewSeverityWarning,
			TargetType: "bridge_client",
			TargetId:   client.ClientId,
			TargetName: name,
			Reasons:    []string{"bridge_stale"},
			Detail:     "Marked online but no live bridge session",
			CreatedAt:  client.LastSeenAt,
		})
	}
	return items, nil
}

func collectMCPProxyTaskReviewItems() []dto.MCPReviewItem {
	items := make([]dto.MCPReviewItem, 0, 2)

	healthCheck := GetMCPProxyHealthCheckStatus()
	if healthCheck.Settings.Enabled && healthCheck.LastRun != nil && healthCheck.LastRun.ErrorCount > 0 {
		items = append(items, dto.MCPReviewItem{
			Category:   MCPReviewCategoryHealthCheck,
			Severity:   MCPReviewSeverityWarning,
			TargetType: "task",
			TargetId:   "mcp_proxy_health_check",
			TargetName: "MCP Proxy Health Check",
			Reasons:    []string{"health_check_failed"},
			Detail:     healthCheck.LastRunMessage,
			CreatedAt:  healthCheck.LastRunAt,
		})
	}

	heartbeat := GetMCPProxyHeartbeatStatus()
	if heartbeat.Settings.Enabled && heartbeat.LastRun != nil && heartbeat.LastRun.ErrorCount > 0 {
		items = append(items, dto.MCPReviewItem{
			Category:   MCPReviewCategoryHeartbeat,
			Severity:   MCPReviewSeverityWarning,
			TargetType: "task",
			TargetId:   "mcp_proxy_heartbeat",
			TargetName: "MCP Proxy Heartbeat",
			Reasons:    []string{"heartbeat_failed"},
			Detail:     heartbeat.LastRunMessage,
			CreatedAt:  heartbeat.LastRunAt,
		})
	}

	return items
}

func collectHighErrorToolReviewItems(startTime int64) ([]dto.MCPReviewItem, error) {
	stats, err := model.ListMCPToolCallToolStats(model.MCPToolCallFilter{
		StartTime: startTime,
	}, mcpReviewToolScanLimit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.MCPReviewItem, 0)
	for _, stat := range stats {
		severity, ok := mcpToolReviewSeverity(stat.Calls, stat.SuccessCalls)
		if !ok {
			continue
		}
		successRate := ratioPercent(stat.SuccessCalls, stat.Calls)
		name := stat.ToolName
		if name == "" {
			name = strconv.Itoa(stat.ToolId)
		}
		items = append(items, dto.MCPReviewItem{
			Category:   MCPReviewCategoryTool,
			Severity:   severity,
			TargetType: "tool",
			TargetId:   name,
			TargetName: name,
			Reasons:    []string{"high_error_rate_tool"},
			Detail: fmt.Sprintf("success_rate=%.1f%% calls=%d errors=%d timeouts=%d",
				successRate, stat.Calls, stat.ErrorCalls, stat.TimeoutCalls),
		})
	}
	return items, nil
}

// mcpToolReviewSeverity decides whether a tool's call stats warrant a review
// item and at which severity. Tools below the minimum call volume or above the
// warning success-rate threshold are ignored to avoid noisy low-signal items.
func mcpToolReviewSeverity(calls int64, successCalls int64) (string, bool) {
	if calls < mcpReviewToolMinCalls {
		return "", false
	}
	successRate := ratioPercent(successCalls, calls)
	if successRate >= mcpReviewToolWarnSuccessRate {
		return "", false
	}
	if successRate < mcpReviewToolCriticalSuccessRate {
		return MCPReviewSeverityCritical, true
	}
	return MCPReviewSeverityWarning, true
}
