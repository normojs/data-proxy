package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildMCPSummaryOperationsTrendsAggregatesOverviewSignals(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	now := common.GetTimestamp()
	start := now - 4*60*60
	bucketSeconds := int64(60 * 60)
	userId := 1

	require.NoError(t, model.DB.Create(&model.BridgeSession{
		SessionId:   "overview-live-session",
		ClientId:    "overview-live-client",
		UserId:      userId,
		Status:      model.BridgeSessionStatusOnline,
		ConnectedAt: start - 120,
		LastPingAt:  now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.BridgeSession{
		SessionId:   "overview-closed-session",
		ClientId:    "overview-closed-client",
		UserId:      userId,
		Status:      model.BridgeSessionStatusClosed,
		ConnectedAt: start + 300,
		ClosedAt:    start + bucketSeconds + 300,
		LastPingAt:  start + bucketSeconds + 300,
	}).Error)

	createOpenAPITrendObject(t, "overview-object-1", userId, 100, 2, now-60, start+300)
	createOpenAPITrendObject(t, "overview-object-2", userId, 250, 0, now+3600, start+bucketSeconds+300)

	tool := createOverviewProxyTool(t)
	for i := 0; i < 3; i++ {
		createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusError, 0, 0, start+600+int64(i))
	}
	createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusTimeout, 0, 0, start+900)
	createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusSuccess, 0, 0, start+1200)

	createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusSuccess, 100, 0, start+1300)
	createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusSuccess, 200, now, start+1400)
	createOverviewToolCall(t, tool.Id, tool.Name, userId, model.MCPToolCallStatusError, 50, now, start+1500)
	require.NoError(t, model.DB.Create(&model.BillingEvent{
		EventId:     "overview-mcp-refund",
		UserId:      userId,
		Source:      model.BillingEventSourceMCPToolCall,
		SourceId:    "overview-refund-source",
		EventType:   model.BillingEventTypeCredit,
		Status:      model.BillingEventStatusSettled,
		RequestId:   "overview-refund",
		AmountQuota: 50,
		QuotaDelta:  50,
		CreatedAt:   start + 1600,
	}).Error)

	trends, err := BuildMCPSummaryOperationsTrends(MCPSummaryOperationsTrendParams{
		UserId:        userId,
		StartTime:     start,
		EndTime:       now,
		BucketSeconds: bucketSeconds,
	})
	require.NoError(t, err)
	require.NotNil(t, trends)
	require.Equal(t, bucketSeconds, trends.BucketSeconds)
	require.NotEmpty(t, trends.BridgeOnline)
	require.NotEmpty(t, trends.OpenAPIStorage)
	require.NotEmpty(t, trends.ProxyErrorTopN)

	var sawStarted, sawClosed bool
	for _, bucket := range trends.BridgeOnline {
		if bucket.StartedSessions > 0 {
			sawStarted = true
		}
		if bucket.ClosedSessions > 0 {
			sawClosed = true
		}
	}
	require.True(t, sawStarted, "expected bridge started session bucket")
	require.True(t, sawClosed, "expected bridge closed session bucket")

	var objectCount, totalBytes, expiredCount, downloadCount int64
	for _, bucket := range trends.OpenAPIStorage {
		objectCount += bucket.ObjectCount
		totalBytes += bucket.TotalBytes
		expiredCount += bucket.ExpiredCount
		downloadCount += bucket.DownloadCount
	}
	require.EqualValues(t, 2, objectCount)
	require.EqualValues(t, 350, totalBytes)
	require.EqualValues(t, 1, expiredCount)
	require.EqualValues(t, 2, downloadCount)

	require.Equal(t, tool.Name, trends.ProxyErrorTopN[0].ToolName)
	require.EqualValues(t, 4, trends.ProxyErrorTopN[0].ErrorCalls)
	require.EqualValues(t, 1, trends.ProxyErrorTopN[0].TimeoutCalls)
	require.EqualValues(t, 8, trends.ProxyErrorTopN[0].TotalCalls)

	require.EqualValues(t, 1, trends.BillingAnomalies.UnsettledSuccessCalls)
	require.EqualValues(t, 1, trends.BillingAnomalies.FailedChargedCalls)
	require.EqualValues(t, 1, trends.BillingAnomalies.MissingDebitEvents)
	require.EqualValues(t, 1, trends.BillingAnomalies.RefundEvents)
	require.EqualValues(t, 50, trends.BillingAnomalies.RefundQuota)
	require.EqualValues(t, 50, trends.BillingAnomalies.NetMCPQuotaDelta)
}

func TestBuildMCPSummaryOperationsTrendsCountsBridgeSessionsBeyondLegacyCap(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	now := common.GetTimestamp()
	start := now - 60*60
	userId := 7
	sessionCount := 10005
	sessions := make([]model.BridgeSession, 0, sessionCount)
	for i := 0; i < sessionCount; i++ {
		sessions = append(sessions, model.BridgeSession{
			SessionId:   fmt.Sprintf("overview-overflow-session-%d", i),
			ClientId:    fmt.Sprintf("overview-overflow-client-%d", i),
			UserId:      userId,
			Status:      model.BridgeSessionStatusOnline,
			ConnectedAt: start + 60,
			LastPingAt:  now,
		})
	}
	require.NoError(t, model.DB.CreateInBatches(sessions, 500).Error)

	trends, err := BuildMCPSummaryOperationsTrends(MCPSummaryOperationsTrendParams{
		UserId:        userId,
		StartTime:     start,
		EndTime:       now,
		BucketSeconds: 60 * 60,
	})
	require.NoError(t, err)
	require.NotNil(t, trends)

	var maxOnline int64
	for _, bucket := range trends.BridgeOnline {
		if bucket.OnlineClients > maxOnline {
			maxOnline = bucket.OnlineClients
		}
	}
	require.EqualValues(t, sessionCount, maxOnline)
}

func createOpenAPITrendObject(t *testing.T, objectId string, userId int, size int, downloadCount int, expiresAt int64, createdAt int64) {
	t.Helper()
	object := &model.MCPOpenAPIBinaryObject{
		ObjectId:      objectId,
		Provider:      "local",
		StorageKey:    objectId,
		ContentType:   "application/octet-stream",
		ContentFamily: "binary",
		Size:          size,
		UserId:        userId,
		ExpiresAt:     expiresAt,
		DownloadCount: downloadCount,
	}
	require.NoError(t, model.DB.Create(object).Error)
	require.NoError(t, model.DB.Model(&model.MCPOpenAPIBinaryObject{}).
		Where("object_id = ?", objectId).
		Update("created_at", createdAt).Error)
}

func createOverviewProxyTool(t *testing.T) *model.MCPTool {
	t.Helper()
	server := &model.MCPProxyServer{
		Name:      "Overview Proxy",
		Namespace: "overview_proxy",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://overview.example/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	}
	require.NoError(t, model.CreateMCPProxyServer(server))
	tool := &model.MCPTool{
		Name:        "overview_proxy.status",
		DisplayName: "Overview Proxy Status",
		Description: "overview proxy status",
		Category:    "third_party",
		Source:      model.MCPToolSourceMCPProxy,
		InputSchema: `{"type":"object"}`,
		PriceUnit:   model.MCPToolPriceUnitPerCall,
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(tool))
	require.NoError(t, model.CreateMCPProxyTool(&model.MCPProxyTool{
		ProxyServerId:         server.Id,
		MCPToolId:             tool.Id,
		DownstreamToolName:    "status",
		DownstreamTitle:       "Status",
		DownstreamDescription: "Status",
		DownstreamInputSchema: `{"type":"object"}`,
		ExposedToolName:       tool.Name,
		ExposedDescription:    "Status",
		SchemaHash:            "sha256:overview",
		Status:                model.MCPProxyToolStatusEnabled,
	}))
	return tool
}

func createOverviewToolCall(t *testing.T, toolId int, toolName string, userId int, status string, quota int, settledAt int64, createdAt int64) {
	t.Helper()
	call := &model.MCPToolCall{
		UserId:    userId,
		ToolId:    toolId,
		ToolName:  toolName,
		RequestId: fmt.Sprintf("overview-call-%s-%d", status, createdAt),
		Status:    status,
		Quota:     quota,
		SettledAt: settledAt,
	}
	require.NoError(t, model.CreateMCPToolCall(call))
	require.NoError(t, model.DB.Model(&model.MCPToolCall{}).
		Where("id = ?", call.Id).
		Update("created_at", createdAt).Error)
}
