package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/stretchr/testify/require"
)

func TestAssembleMCPReviewQueueSortsAndCounts(t *testing.T) {
	items := []dto.MCPReviewItem{
		{Category: "a", Severity: MCPReviewSeverityWarning},
		{Category: "b", Severity: MCPReviewSeverityCritical},
		{Category: "c", Severity: MCPReviewSeverityWarning},
		{Category: "d", Severity: MCPReviewSeverityCritical},
	}
	scanLimits := dto.MCPReviewScanLimits{
		ProxyServers: dto.MCPReviewScanScope{
			Scanned: 2,
			Total:   4,
			Limit:   10,
			Capped:  true,
		},
	}

	queue := assembleMCPReviewQueue(items, scanLimits)

	require.Equal(t, 4, queue.Total)
	require.Equal(t, 2, queue.CriticalCount)
	require.Equal(t, 2, queue.WarningCount)
	require.Equal(t, 4, queue.VisibleCount)
	require.Equal(t, mcpReviewMaxItems, queue.MaxItems)
	require.False(t, queue.Truncated)
	require.Equal(t, scanLimits, queue.ScanLimits)
	require.Len(t, queue.Items, 4)
	// Critical first, preserving collection order within each severity.
	require.Equal(t, MCPReviewSeverityCritical, queue.Items[0].Severity)
	require.Equal(t, "b", queue.Items[0].Category)
	require.Equal(t, "d", queue.Items[1].Category)
	require.Equal(t, MCPReviewSeverityWarning, queue.Items[2].Severity)
	require.Equal(t, "a", queue.Items[2].Category)
	require.Equal(t, "c", queue.Items[3].Category)
}

func TestAssembleMCPReviewQueueCapsItems(t *testing.T) {
	total := mcpReviewMaxItems + 10
	items := make([]dto.MCPReviewItem, 0, total)
	for i := 0; i < total; i++ {
		items = append(items, dto.MCPReviewItem{Severity: MCPReviewSeverityWarning})
	}

	queue := assembleMCPReviewQueue(items, dto.MCPReviewScanLimits{})

	require.Equal(t, total, queue.Total)
	require.Equal(t, total, queue.WarningCount)
	require.Equal(t, mcpReviewMaxItems, queue.VisibleCount)
	require.Equal(t, mcpReviewMaxItems, queue.MaxItems)
	require.True(t, queue.Truncated)
	require.Len(t, queue.Items, mcpReviewMaxItems)
}

func TestMCPToolReviewSeverity(t *testing.T) {
	if _, ok := mcpToolReviewSeverity(mcpReviewToolMinCalls-1, 0); ok {
		t.Fatal("tools below minimum call volume should not be reviewed")
	}
	if _, ok := mcpToolReviewSeverity(10, 10); ok {
		t.Fatal("healthy tools should not be reviewed")
	}

	sev, ok := mcpToolReviewSeverity(10, 7)
	require.True(t, ok)
	require.Equal(t, MCPReviewSeverityWarning, sev)

	sev, ok = mcpToolReviewSeverity(10, 4)
	require.True(t, ok)
	require.Equal(t, MCPReviewSeverityCritical, sev)
}

func TestBuildMCPReviewQueueAggregatesSignals(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	now := common.GetTimestamp()

	// Stale bridge client: marked online in the DB but with no live hub session.
	staleClient := &model.BridgeClient{
		ClientId:   "review-stale-client",
		UserId:     1,
		Name:       "Stale Client",
		Status:     model.BridgeClientStatusOnline,
		LastSeenAt: now - 600,
	}
	require.NoError(t, model.DB.Create(staleClient).Error)

	// Online bridge client that should NOT be flagged stale (a live session exists).
	healthyClient := &model.BridgeClient{
		ClientId:   "review-healthy-client",
		UserId:     1,
		Name:       "Healthy Client",
		Status:     model.BridgeClientStatusOnline,
		LastSeenAt: now,
	}
	require.NoError(t, model.DB.Create(healthyClient).Error)
	outbound := make(chan bridge.OutboundMessage, 1)
	bridge.DefaultHub.Register(bridge.Session{
		SessionId: "review-healthy-session",
		ClientId:  healthyClient.ClientId,
		UserId:    1,
		Send:      outbound,
	})
	t.Cleanup(func() {
		_, _ = bridge.DefaultHub.Unregister("review-healthy-session")
	})

	// High error-rate tool: 6 calls, 1 success (16.7%) -> critical.
	tool := &model.MCPTool{
		Name:        "review_high_error_tool",
		DisplayName: "Review High Error Tool",
		Description: "high error rate tool",
		Category:    "audit",
		Source:      model.MCPToolSourceCustom,
		InputSchema: `{"type":"object"}`,
		PriceUnit:   model.MCPToolPriceUnitPerCall,
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(tool))
	for i := 0; i < 6; i++ {
		status := model.MCPToolCallStatusError
		if i == 0 {
			status = model.MCPToolCallStatusSuccess
		}
		call := &model.MCPToolCall{
			UserId:    1,
			ToolId:    tool.Id,
			ToolName:  tool.Name,
			RequestId: fmt.Sprintf("review-call-%d", i),
			Status:    status,
		}
		require.NoError(t, model.CreateMCPToolCall(call))
	}

	queue, err := BuildMCPReviewQueue(MCPReviewQueueParams{
		WindowSeconds: 24 * 60 * 60,
		StartTime:     now - 24*60*60,
	})
	require.NoError(t, err)
	require.NotNil(t, queue)

	var sawStale, sawTool, sawHealthy bool
	for _, item := range queue.Items {
		switch item.Category {
		case MCPReviewCategoryBridgeClient:
			if item.TargetId == staleClient.ClientId {
				sawStale = true
				require.Equal(t, MCPReviewSeverityWarning, item.Severity)
				require.Contains(t, item.Reasons, "bridge_stale")
			}
			if item.TargetId == healthyClient.ClientId {
				sawHealthy = true
			}
		case MCPReviewCategoryTool:
			if item.TargetId == tool.Name {
				sawTool = true
				require.Equal(t, MCPReviewSeverityCritical, item.Severity)
				require.Contains(t, item.Reasons, "high_error_rate_tool")
			}
		}
	}

	require.True(t, sawStale, "expected stale bridge client review item")
	require.True(t, sawTool, "expected high error-rate tool review item")
	require.False(t, sawHealthy, "client with a live session must not be flagged stale")
	require.GreaterOrEqual(t, queue.Total, 2)
	require.GreaterOrEqual(t, queue.CriticalCount, 1)
	require.Equal(t, queue.VisibleCount, len(queue.Items))
	require.Equal(t, mcpReviewMaxItems, queue.MaxItems)
	require.False(t, queue.Truncated)
	require.Equal(t, mcpReviewBridgeClientScanLimit, queue.ScanLimits.BridgeClients.Limit)
	require.Equal(t, 2, queue.ScanLimits.BridgeClients.Total)
	require.Equal(t, 2, queue.ScanLimits.BridgeClients.Scanned)
	require.False(t, queue.ScanLimits.BridgeClients.Capped)
	require.Equal(t, mcpReviewToolScanLimit, queue.ScanLimits.Tools.Limit)
	require.Equal(t, 1, queue.ScanLimits.Tools.Total)
	require.Equal(t, 1, queue.ScanLimits.Tools.Scanned)
	require.False(t, queue.ScanLimits.Tools.Capped)
}
