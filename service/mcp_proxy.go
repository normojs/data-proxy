package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
	"github.com/QuantumNous/new-api/pkg/mcp/secretref"

	"gorm.io/gorm"
)

var mcpProxyNamespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}[a-z0-9]$`)
var defaultMCPProxyClient mcpproxy.Client = mcpproxy.NewDefaultClient(nil)

type MCPProxyServerListParams struct {
	Transport string
	Status    string
	Keyword   string
	Offset    int
	Limit     int
}

type MCPProxyToolListParams struct {
	ProxyServerId int
	Status        string
	SchemaHash    string
	Keyword       string
	Offset        int
	Limit         int
}

type MCPProxyServerHealthParams struct {
	WindowSeconds int64
}

type MCPProxyTrendParams struct {
	ProxyServerId int
	ProxyToolId   int64
	Status        string
	StartTime     int64
	EndTime       int64
	BucketSeconds int64
}

const (
	defaultMCPProxyTrendWindowSeconds int64 = 14 * 24 * 60 * 60
	defaultMCPProxyTrendBucketSeconds int64 = 24 * 60 * 60
)

func setMCPProxyClientForTest(client mcpproxy.Client) func() {
	previous := defaultMCPProxyClient
	previousRegistry := defaultMCPExecutorRegistry
	if client == nil {
		client = mcpproxy.UnconfiguredClient{}
	}
	defaultMCPProxyClient = client
	defaultMCPExecutorRegistry = newDefaultMCPExecutorRegistry(client)
	return func() {
		defaultMCPProxyClient = previous
		defaultMCPExecutorRegistry = previousRegistry
	}
}

func ListMCPProxyServersForAdmin(params MCPProxyServerListParams) ([]dto.MCPProxyServerAdminItem, int64, error) {
	items, total, err := model.ListMCPProxyServers(model.MCPProxyServerFilter{
		Transport: strings.TrimSpace(params.Transport),
		Status:    strings.TrimSpace(params.Status),
		Keyword:   strings.TrimSpace(params.Keyword),
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.MCPProxyServerAdminItem, 0, len(items))
	for _, item := range items {
		result = append(result, mcpProxyServerToAdminDTO(item))
	}
	if err := attachMCPProxyServerListHealth(result, items, normalizeMCPSummaryWindow(0)); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func GetMCPProxyTrendsForAdmin(params MCPProxyTrendParams) (dto.MCPProxyTrendResponse, error) {
	startTime, endTime := normalizeMCPProxyTrendWindow(params.StartTime, params.EndTime)
	bucketSeconds := normalizeMCPProxyTrendBucket(params.BucketSeconds)
	if params.ProxyServerId > 0 {
		if _, err := model.GetMCPProxyServerById(params.ProxyServerId); err != nil {
			return dto.MCPProxyTrendResponse{}, err
		}
	}
	if params.ProxyToolId > 0 {
		if _, err := model.GetMCPProxyToolById(params.ProxyToolId); err != nil {
			return dto.MCPProxyTrendResponse{}, err
		}
	}

	summary, err := model.SummarizeMCPProxyTrends(model.MCPProxyTrendFilter{
		ProxyServerId: params.ProxyServerId,
		ProxyToolId:   params.ProxyToolId,
		Status:        strings.TrimSpace(params.Status),
		StartTime:     startTime,
		EndTime:       endTime,
	}, bucketSeconds, 5, 5)
	if err != nil {
		return dto.MCPProxyTrendResponse{}, err
	}
	return dto.MCPProxyTrendResponse{
		StartTime:     startTime,
		EndTime:       endTime,
		BucketSeconds: bucketSeconds,
		CheckedAt:     common.GetTimestamp(),
		Totals:        mcpProxyCallHealthFromTrendStats(summary.Totals),
		Buckets:       mcpProxyTrendBucketDTOs(summary.Buckets),
		Servers:       mcpProxyTrendServerDTOs(summary.Servers),
		Tools:         mcpProxyTrendToolDTOs(summary.Tools),
	}, nil
}

func GetMCPProxyServerForAdmin(id int) (dto.MCPProxyServerAdminItem, error) {
	server, err := model.GetMCPProxyServerById(id)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	return mcpProxyServerToAdminDTO(*server), nil
}

func CreateMCPProxyServerForAdmin(req dto.MCPProxyServerCreateRequest) (dto.MCPProxyServerAdminItem, error) {
	server, err := mcpProxyCreateRequestToModel(req)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	if err := model.CreateMCPProxyServer(server); err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	return mcpProxyServerToAdminDTO(*server), nil
}

func UpdateMCPProxyServerForAdmin(id int, req dto.MCPProxyServerUpdateRequest) (dto.MCPProxyServerAdminItem, error) {
	if id <= 0 {
		return dto.MCPProxyServerAdminItem{}, errors.New("invalid mcp proxy server id")
	}
	existing, err := model.GetMCPProxyServerById(id)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	updates, err := mcpProxyServerUpdateFields(*existing, req)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	if len(updates) == 0 {
		return GetMCPProxyServerForAdmin(id)
	}
	server, err := model.UpdateMCPProxyServerFields(id, updates)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	return mcpProxyServerToAdminDTO(*server), nil
}

func ArchiveMCPProxyServerForAdmin(id int) (dto.MCPProxyServerAdminItem, error) {
	if id <= 0 {
		return dto.MCPProxyServerAdminItem{}, errors.New("invalid mcp proxy server id")
	}
	if _, err := model.GetMCPProxyServerById(id); err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	server, err := model.ArchiveMCPProxyServer(id)
	if err != nil {
		return dto.MCPProxyServerAdminItem{}, err
	}
	return mcpProxyServerToAdminDTO(*server), nil
}

func TestMCPProxyServerForAdmin(ctx context.Context, id int) (dto.MCPProxyServerTestResult, error) {
	if id <= 0 {
		return dto.MCPProxyServerTestResult{}, errors.New("invalid mcp proxy server id")
	}
	server, err := model.GetMCPProxyServerById(id)
	if err != nil {
		return dto.MCPProxyServerTestResult{}, err
	}
	startedAt := common.GetTimestamp()
	startedClock := time.Now()
	result, err := defaultMCPProxyClient.Test(ctx, *server)
	if err != nil {
		_, _ = model.UpdateMCPProxyServerFields(id, map[string]any{
			"status":     model.MCPProxyServerStatusError,
			"last_error": truncateMCPString(err.Error(), 512),
			"updated_at": common.GetTimestamp(),
		})
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId: id,
			EventType:     model.MCPProxyDiscoveryEventTypeTest,
			Status:        model.MCPProxyDiscoveryEventStatusError,
			Message:       truncateMCPString(err.Error(), 1024),
			DurationMS:    durationMSSince(startedClock),
			StartedAt:     startedAt,
		})
		return dto.MCPProxyServerTestResult{}, err
	}
	_, _ = markMCPProxyServerHealthy(*server)
	recordMCPProxyDiscoveryEvent(mcpProxyTestEvent(id, startedAt, durationMSSince(startedClock), result))
	return dto.MCPProxyServerTestResult{
		ProxyServerId:   id,
		ProtocolVersion: result.ProtocolVersion,
		ServerName:      result.ServerName,
		Capabilities:    result.Capabilities,
	}, nil
}

func DiscoverMCPProxyServerToolsForAdmin(ctx context.Context, id int) (dto.MCPProxyDiscoveryResult, error) {
	if id <= 0 {
		return dto.MCPProxyDiscoveryResult{}, errors.New("invalid mcp proxy server id")
	}
	server, err := model.GetMCPProxyServerById(id)
	if err != nil {
		return dto.MCPProxyDiscoveryResult{}, err
	}
	startedAt := common.GetTimestamp()
	startedClock := time.Now()
	tools, err := defaultMCPProxyClient.ListTools(ctx, *server)
	if err != nil {
		_, _ = model.UpdateMCPProxyServerFields(id, map[string]any{
			"status":     model.MCPProxyServerStatusError,
			"last_error": truncateMCPString(err.Error(), 512),
			"updated_at": common.GetTimestamp(),
		})
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId: id,
			EventType:     model.MCPProxyDiscoveryEventTypeDiscover,
			Status:        model.MCPProxyDiscoveryEventStatusError,
			Message:       truncateMCPString(err.Error(), 1024),
			DurationMS:    durationMSSince(startedClock),
			StartedAt:     startedAt,
		})
		return dto.MCPProxyDiscoveryResult{}, err
	}
	result, err := syncMCPProxyDiscoveredTools(*server, tools)
	if err != nil {
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId: id,
			EventType:     model.MCPProxyDiscoveryEventTypeDiscover,
			Status:        model.MCPProxyDiscoveryEventStatusError,
			Message:       truncateMCPString(err.Error(), 1024),
			DurationMS:    durationMSSince(startedClock),
			StartedAt:     startedAt,
		})
		return result, err
	}
	recordMCPProxyDiscoveryEvent(mcpProxyDiscoverEvent(id, startedAt, durationMSSince(startedClock), result))
	return result, nil
}

func markMCPProxyServerHealthy(server model.MCPProxyServer) (*model.MCPProxyServer, error) {
	updates := map[string]any{
		"last_error": "",
		"updated_at": common.GetTimestamp(),
	}
	if server.Status == model.MCPProxyServerStatusError {
		updates["status"] = model.MCPProxyServerStatusEnabled
	}
	return model.UpdateMCPProxyServerFields(server.Id, updates)
}

func ListMCPProxyToolsForAdmin(params MCPProxyToolListParams) ([]dto.MCPProxyToolAdminItem, int64, error) {
	items, total, err := model.ListMCPProxyTools(model.MCPProxyToolFilter{
		ProxyServerId: params.ProxyServerId,
		Status:        strings.TrimSpace(params.Status),
		SchemaHash:    strings.TrimSpace(params.SchemaHash),
		Keyword:       strings.TrimSpace(params.Keyword),
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	mcpTools, err := mcpToolsByProxyTools(items)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.MCPProxyToolAdminItem, 0, len(items))
	for _, item := range items {
		result = append(result, mcpProxyToolToAdminDTO(item, mcpToolValueFromMap(mcpTools, item.MCPToolId)))
	}
	return result, total, nil
}

func GetMCPProxyToolForAdmin(id int64) (dto.MCPProxyToolAdminItem, error) {
	if id <= 0 {
		return dto.MCPProxyToolAdminItem{}, errors.New("invalid mcp proxy tool id")
	}
	tool, err := model.GetMCPProxyToolById(id)
	if err != nil {
		return dto.MCPProxyToolAdminItem{}, err
	}
	mcpTool, _ := model.GetMCPToolById(tool.MCPToolId)
	return mcpProxyToolToAdminDTO(*tool, mcpToolValue(mcpTool)), nil
}

func ListMCPProxyServerToolsForAdmin(proxyServerId int) ([]dto.MCPProxyToolAdminItem, error) {
	if proxyServerId <= 0 {
		return nil, errors.New("invalid mcp proxy server id")
	}
	if _, err := model.GetMCPProxyServerById(proxyServerId); err != nil {
		return nil, err
	}
	items, err := model.ListMCPProxyToolsByServerId(proxyServerId)
	if err != nil {
		return nil, err
	}
	mcpTools, err := mcpToolsByProxyTools(items)
	if err != nil {
		return nil, err
	}
	result := make([]dto.MCPProxyToolAdminItem, 0, len(items))
	for _, item := range items {
		result = append(result, mcpProxyToolToAdminDTO(item, mcpToolValueFromMap(mcpTools, item.MCPToolId)))
	}
	return result, nil
}

func ListMCPProxyDiscoveryEventsForAdmin(proxyServerId int, offset int, limit int) ([]dto.MCPProxyDiscoveryEventItem, int64, error) {
	if proxyServerId <= 0 {
		return nil, 0, errors.New("invalid mcp proxy server id")
	}
	if _, err := model.GetMCPProxyServerById(proxyServerId); err != nil {
		return nil, 0, err
	}
	events, total, err := model.ListMCPProxyDiscoveryEventsByServerId(proxyServerId, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.MCPProxyDiscoveryEventItem, 0, len(events))
	for _, event := range events {
		items = append(items, mcpProxyDiscoveryEventToDTO(event))
	}
	return items, total, nil
}

func GetMCPProxyServerHealthForAdmin(id int, params MCPProxyServerHealthParams) (dto.MCPProxyServerHealth, error) {
	if id <= 0 {
		return dto.MCPProxyServerHealth{}, errors.New("invalid mcp proxy server id")
	}
	server, err := model.GetMCPProxyServerById(id)
	if err != nil {
		return dto.MCPProxyServerHealth{}, err
	}
	tools, err := model.ListMCPProxyToolsByServerId(id)
	if err != nil {
		return dto.MCPProxyServerHealth{}, err
	}
	windowSeconds := normalizeMCPSummaryWindow(params.WindowSeconds)
	now := common.GetTimestamp()
	toolIds := mcpProxyToolIds(tools)
	callStats := model.MCPToolCallStats{}
	toolStats := []model.MCPToolCallToolStats{}
	callErrors := []model.MCPToolCall{}
	if len(toolIds) > 0 {
		callFilter := model.MCPToolCallFilter{
			ToolIds:   toolIds,
			StartTime: now - windowSeconds,
		}
		callStats, err = model.GetMCPToolCallStats(callFilter)
		if err != nil {
			return dto.MCPProxyServerHealth{}, err
		}
		toolStats, err = model.ListMCPToolCallToolStats(callFilter, 5)
		if err != nil {
			return dto.MCPProxyServerHealth{}, err
		}
		callErrors, err = model.ListRecentMCPToolCallErrors(callFilter, 5)
		if err != nil {
			return dto.MCPProxyServerHealth{}, err
		}
	}

	callHealth := mcpProxyServerCallHealthFromToolCallStats(callStats)
	discovery := mcpProxyServerDiscoveryInfo(*server, tools)
	transport := mcpProxyTransportHealth(*server)
	recentErrors := mcpProxyServerRecentErrors(callErrors)
	latestCheck, err := latestMCPProxyDiscoveryEventItem(id)
	if err != nil {
		return dto.MCPProxyServerHealth{}, err
	}
	needsReview, reviewReasons := mcpProxyReviewState(*server, callHealth, discovery, transport, latestCheck)

	return dto.MCPProxyServerHealth{
		ProxyServerId: id,
		WindowSeconds: windowSeconds,
		GeneratedAt:   now,
		NeedsReview:   needsReview,
		ReviewReasons: reviewReasons,
		Calls:         callHealth,
		Discovery:     discovery,
		Transport:     transport,
		TopTools:      mcpProxyServerToolHealth(tools, toolStats),
		RecentErrors:  recentErrors,
		LatestCheck:   latestCheck,
	}, nil
}

func GetMCPProxyToolHealthForAdmin(id int64, params MCPProxyServerHealthParams) (dto.MCPProxyToolHealth, error) {
	if id <= 0 {
		return dto.MCPProxyToolHealth{}, errors.New("invalid mcp proxy tool id")
	}
	tool, err := model.GetMCPProxyToolById(id)
	if err != nil {
		return dto.MCPProxyToolHealth{}, err
	}
	windowSeconds := normalizeMCPSummaryWindow(params.WindowSeconds)
	now := common.GetTimestamp()
	callStats := model.MCPToolCallStats{}
	callErrors := []model.MCPToolCall{}
	if tool.MCPToolId > 0 {
		callFilter := model.MCPToolCallFilter{
			ToolIds:   []int{tool.MCPToolId},
			StartTime: now - windowSeconds,
		}
		callStats, err = model.GetMCPToolCallStats(callFilter)
		if err != nil {
			return dto.MCPProxyToolHealth{}, err
		}
		callErrors, err = model.ListRecentMCPToolCallErrors(callFilter, 5)
		if err != nil {
			return dto.MCPProxyToolHealth{}, err
		}
	}
	return dto.MCPProxyToolHealth{
		ProxyToolId:   tool.Id,
		ProxyServerId: tool.ProxyServerId,
		MCPToolId:     tool.MCPToolId,
		WindowSeconds: windowSeconds,
		GeneratedAt:   now,
		Calls:         mcpProxyServerCallHealthFromToolCallStats(callStats),
		Tool: mcpProxyServerToolStatToHealth(*tool, model.MCPProxyServerToolCallStats{
			ProxyServerId: tool.ProxyServerId,
			ProxyToolId:   tool.Id,
			ToolId:        tool.MCPToolId,
			ToolName:      tool.ExposedToolName,
			Calls:         callStats.TotalCalls,
			SuccessCalls:  callStats.SuccessCalls,
			ErrorCalls:    callStats.ErrorCalls,
			TimeoutCalls:  callStats.TimeoutCalls,
			Quota:         callStats.Quota,
			Cost:          callStats.Cost,
			AvgDurationMS: callStats.AvgDurationMS,
		}),
		RecentErrors: mcpProxyServerRecentErrors(callErrors),
	}, nil
}

func UpdateMCPProxyToolForAdmin(id int64, req dto.MCPProxyToolUpdateRequest) (dto.MCPProxyToolAdminItem, error) {
	if id <= 0 {
		return dto.MCPProxyToolAdminItem{}, errors.New("invalid mcp proxy tool id")
	}
	existing, err := model.GetMCPProxyToolById(id)
	if err != nil {
		return dto.MCPProxyToolAdminItem{}, err
	}
	updates, mcpToolUpdates, err := mcpProxyToolUpdateFields(*existing, req)
	if err != nil {
		return dto.MCPProxyToolAdminItem{}, err
	}
	if len(updates) == 0 && len(mcpToolUpdates) == 0 {
		mcpTool, _ := model.GetMCPToolById(existing.MCPToolId)
		return mcpProxyToolToAdminDTO(*existing, mcpToolValue(mcpTool)), nil
	}
	updated := existing
	var mcpTool *model.MCPTool
	if len(updates) > 0 {
		updated, err = model.UpdateMCPProxyToolFields(id, updates)
		if err != nil {
			return dto.MCPProxyToolAdminItem{}, err
		}
	}
	if updated.MCPToolId > 0 && len(mcpToolUpdates) > 0 {
		mcpTool, err = model.UpdateMCPToolFields(updated.MCPToolId, mcpToolUpdates)
		if err != nil {
			return dto.MCPProxyToolAdminItem{}, err
		}
	} else if updated.MCPToolId > 0 {
		mcpTool, _ = model.GetMCPToolById(updated.MCPToolId)
	}
	return mcpProxyToolToAdminDTO(*updated, mcpToolValue(mcpTool)), nil
}

func mcpProxyToolIds(tools []model.MCPProxyTool) []int {
	ids := make([]int, 0, len(tools))
	for _, tool := range tools {
		if tool.MCPToolId > 0 {
			ids = append(ids, tool.MCPToolId)
		}
	}
	return ids
}

func attachMCPProxyServerListHealth(items []dto.MCPProxyServerAdminItem, servers []model.MCPProxyServer, windowSeconds int64) error {
	if len(items) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	serverIds := make([]int, 0, len(servers))
	serversById := make(map[int]model.MCPProxyServer, len(servers))
	for _, server := range servers {
		if server.Id <= 0 {
			continue
		}
		serverIds = append(serverIds, server.Id)
		serversById[server.Id] = server
	}
	tools, err := model.ListMCPProxyToolsByServerIds(serverIds)
	if err != nil {
		return err
	}
	toolsByServerId := make(map[int][]model.MCPProxyTool, len(serverIds))
	toolsByProxyToolId := make(map[int64]model.MCPProxyTool, len(tools))
	for _, tool := range tools {
		toolsByServerId[tool.ProxyServerId] = append(toolsByServerId[tool.ProxyServerId], tool)
		toolsByProxyToolId[tool.Id] = tool
	}
	callStats, err := model.ListMCPProxyServerCallStats(serverIds, now-windowSeconds)
	if err != nil {
		return err
	}
	callStatsByServerId := make(map[int]model.MCPProxyServerCallStats, len(callStats))
	for _, stat := range callStats {
		callStatsByServerId[stat.ProxyServerId] = stat
	}
	toolStats, err := model.ListMCPProxyServerToolCallStats(serverIds, now-windowSeconds)
	if err != nil {
		return err
	}
	topToolByServerId := make(map[int]dto.MCPProxyServerToolHealth, len(serverIds))
	for _, stat := range toolStats {
		if _, exists := topToolByServerId[stat.ProxyServerId]; exists {
			continue
		}
		proxyTool, ok := toolsByProxyToolId[stat.ProxyToolId]
		if !ok {
			continue
		}
		topToolByServerId[stat.ProxyServerId] = mcpProxyServerToolStatToHealth(proxyTool, stat)
	}
	toolIds := mcpProxyToolIds(tools)
	errors := []model.MCPToolCall{}
	if len(toolIds) > 0 {
		errors, err = model.ListRecentMCPToolCallErrors(model.MCPToolCallFilter{
			ToolIds:   toolIds,
			StartTime: now - windowSeconds,
		}, len(serverIds)*3)
		if err != nil {
			return err
		}
	}
	toolServerByMCPToolId := make(map[int]int, len(tools))
	for _, tool := range tools {
		if tool.MCPToolId > 0 {
			toolServerByMCPToolId[tool.MCPToolId] = tool.ProxyServerId
		}
	}
	latestErrorByServerId := make(map[int]dto.MCPProxyServerRecentError, len(serverIds))
	for _, call := range errors {
		serverId := toolServerByMCPToolId[call.ToolId]
		if serverId <= 0 {
			continue
		}
		if _, exists := latestErrorByServerId[serverId]; exists {
			continue
		}
		converted := mcpProxyServerRecentErrors([]model.MCPToolCall{call})
		if len(converted) > 0 {
			latestErrorByServerId[serverId] = converted[0]
		}
	}
	latestChecks, err := model.ListLatestMCPProxyDiscoveryEventsByServerIds(serverIds)
	if err != nil {
		return err
	}

	for index := range items {
		server := serversById[items[index].Id]
		stat := callStatsByServerId[items[index].Id]
		callHealth := mcpProxyServerCallHealthFromStats(stat)
		discovery := mcpProxyServerDiscoveryInfo(server, toolsByServerId[items[index].Id])
		var latestCheck *dto.MCPProxyDiscoveryEventItem
		if event, ok := latestChecks[items[index].Id]; ok {
			converted := mcpProxyDiscoveryEventToDTO(event)
			latestCheck = &converted
		}
		transport := mcpProxyTransportHealth(server)
		needsReview, reviewReasons := mcpProxyReviewState(server, callHealth, discovery, transport, latestCheck)
		health := dto.MCPProxyServerListHealth{
			WindowSeconds: windowSeconds,
			GeneratedAt:   now,
			NeedsReview:   needsReview,
			ReviewReasons: reviewReasons,
			Calls:         callHealth,
			Discovery:     discovery,
			LatestCheck:   latestCheck,
		}
		if topTool, ok := topToolByServerId[items[index].Id]; ok {
			health.TopTool = &topTool
		}
		if latestError, ok := latestErrorByServerId[items[index].Id]; ok {
			health.LatestError = &latestError
		}
		items[index].Health = &health
	}
	return nil
}

func normalizeMCPProxyTrendWindow(startTime int64, endTime int64) (int64, int64) {
	now := common.GetTimestamp()
	if endTime <= 0 {
		endTime = now
	}
	if startTime <= 0 {
		startTime = endTime - defaultMCPProxyTrendWindowSeconds
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	return startTime, endTime
}

func normalizeMCPProxyTrendBucket(bucketSeconds int64) int64 {
	if bucketSeconds <= 0 {
		return defaultMCPProxyTrendBucketSeconds
	}
	if bucketSeconds < 60 {
		return 60
	}
	if bucketSeconds > 7*24*60*60 {
		return 7 * 24 * 60 * 60
	}
	return bucketSeconds
}

func mcpProxyServerCallHealthFromStats(stats model.MCPProxyServerCallStats) dto.MCPProxyServerCallHealth {
	return dto.MCPProxyServerCallHealth{
		TotalCalls:    stats.TotalCalls,
		SuccessCalls:  stats.SuccessCalls,
		ErrorCalls:    stats.ErrorCalls,
		TimeoutCalls:  stats.TimeoutCalls,
		PendingCalls:  stats.PendingCalls,
		SettledCalls:  stats.SettledCalls,
		Unsettled:     stats.Unsettled,
		FreeCalls:     stats.FreeCalls,
		Quota:         stats.Quota,
		Cost:          stats.Cost,
		ResultSize:    stats.ResultSize,
		AvgDurationMS: stats.AvgDurationMS,
		SuccessRate:   ratioPercent(stats.SuccessCalls, stats.TotalCalls),
	}
}

func mcpProxyCallHealthFromTrendStats(stats model.MCPProxyTrendStats) dto.MCPProxyServerCallHealth {
	return dto.MCPProxyServerCallHealth{
		TotalCalls:    stats.TotalCalls,
		SuccessCalls:  stats.SuccessCalls,
		ErrorCalls:    stats.ErrorCalls,
		TimeoutCalls:  stats.TimeoutCalls,
		PendingCalls:  stats.PendingCalls,
		SettledCalls:  stats.SettledCalls,
		Unsettled:     stats.Unsettled,
		FreeCalls:     stats.FreeCalls,
		Quota:         stats.Quota,
		Cost:          stats.Cost,
		ResultSize:    stats.ResultSize,
		AvgDurationMS: stats.AvgDurationMS,
		SuccessRate:   ratioPercent(stats.SuccessCalls, stats.TotalCalls),
	}
}

func mcpProxyTrendBucketDTOs(items []model.MCPProxyTrendBucketStats) []dto.MCPProxyTrendBucket {
	result := make([]dto.MCPProxyTrendBucket, 0, len(items))
	for _, item := range items {
		result = append(result, dto.MCPProxyTrendBucket{
			BucketStart:              item.BucketStart,
			MCPProxyServerCallHealth: mcpProxyCallHealthFromTrendStats(item.MCPProxyTrendStats),
		})
	}
	return result
}

func mcpProxyTrendServerDTOs(items []model.MCPProxyTrendServerStats) []dto.MCPProxyTrendServerDimension {
	result := make([]dto.MCPProxyTrendServerDimension, 0, len(items))
	for _, item := range items {
		result = append(result, dto.MCPProxyTrendServerDimension{
			ProxyServerId:            item.ProxyServerId,
			Name:                     item.Name,
			Namespace:                item.Namespace,
			MCPProxyServerCallHealth: mcpProxyCallHealthFromTrendStats(item.MCPProxyTrendStats),
		})
	}
	return result
}

func mcpProxyTrendToolDTOs(items []model.MCPProxyTrendToolStats) []dto.MCPProxyTrendToolDimension {
	result := make([]dto.MCPProxyTrendToolDimension, 0, len(items))
	for _, item := range items {
		result = append(result, dto.MCPProxyTrendToolDimension{
			ProxyServerId:            item.ProxyServerId,
			ProxyToolId:              item.ProxyToolId,
			ToolId:                   item.ToolId,
			ExposedToolName:          item.ExposedToolName,
			DownstreamToolName:       item.DownstreamToolName,
			MCPProxyServerCallHealth: mcpProxyCallHealthFromTrendStats(item.MCPProxyTrendStats),
		})
	}
	return result
}

func mcpProxyServerCallHealthFromToolCallStats(stats model.MCPToolCallStats) dto.MCPProxyServerCallHealth {
	return dto.MCPProxyServerCallHealth{
		TotalCalls:    stats.TotalCalls,
		SuccessCalls:  stats.SuccessCalls,
		ErrorCalls:    stats.ErrorCalls,
		TimeoutCalls:  stats.TimeoutCalls,
		PendingCalls:  stats.PendingCalls,
		SettledCalls:  stats.SettledCalls,
		Unsettled:     stats.Unsettled,
		FreeCalls:     stats.FreeCalls,
		Quota:         stats.Quota,
		Cost:          stats.Cost,
		ResultSize:    stats.ResultSize,
		AvgDurationMS: stats.AvgDurationMS,
		SuccessRate:   ratioPercent(stats.SuccessCalls, stats.TotalCalls),
	}
}

func mcpProxyServerDiscoveryInfo(server model.MCPProxyServer, tools []model.MCPProxyTool) dto.MCPProxyServerDiscoveryInfo {
	info := dto.MCPProxyServerDiscoveryInfo{
		TotalTools:       len(tools),
		LastDiscoveredAt: server.LastDiscoveredAt,
	}
	for _, tool := range tools {
		switch tool.Status {
		case model.MCPProxyToolStatusEnabled:
			info.EnabledTools++
		case model.MCPProxyToolStatusPending:
			info.PendingTools++
		case model.MCPProxyToolStatusDisabled:
			info.DisabledTools++
		case model.MCPProxyToolStatusSchemaChanged:
			info.SchemaChangedTools++
		case model.MCPProxyToolStatusError:
			info.ErrorTools++
		}
		if tool.LastDiscoveredAt > info.LastDiscoveredAt {
			info.LastDiscoveredAt = tool.LastDiscoveredAt
		}
		if tool.UpdatedAt > info.LastToolUpdatedAt {
			info.LastToolUpdatedAt = tool.UpdatedAt
		}
	}
	return info
}

func mcpProxyTransportHealth(server model.MCPProxyServer) dto.MCPProxyTransportHealth {
	health := dto.MCPProxyTransportHealth{
		Transport: server.Transport,
	}
	type sessionSnapshotProvider interface {
		SessionSnapshot(server model.MCPProxyServer) mcpproxy.SessionSnapshot
	}
	provider, ok := defaultMCPProxyClient.(sessionSnapshotProvider)
	if !ok || provider == nil {
		return health
	}
	snapshot := provider.SessionSnapshot(server)
	health.Transport = snapshot.Transport
	if health.Transport == "" {
		health.Transport = server.Transport
	}
	health.HasSession = snapshot.HasSession
	health.Initialized = snapshot.Initialized
	health.MessageEndpoint = snapshot.MessageEndpoint
	health.LastError = snapshot.LastError
	health.StreamableSession = snapshot.StreamableSession
	health.SSEConnected = snapshot.SSEConnected
	health.ActiveSessions = snapshot.ActiveSessions
	health.PendingRequests = snapshot.PendingRequests
	health.LastActivityAt = snapshot.LastActivityAt
	health.Observable = true
	return health
}

func mcpProxyServerToolHealth(tools []model.MCPProxyTool, stats []model.MCPToolCallToolStats) []dto.MCPProxyServerToolHealth {
	toolsByMCPToolId := make(map[int]model.MCPProxyTool, len(tools))
	for _, tool := range tools {
		if tool.MCPToolId > 0 {
			toolsByMCPToolId[tool.MCPToolId] = tool
		}
	}
	result := make([]dto.MCPProxyServerToolHealth, 0, len(stats))
	for _, stat := range stats {
		proxyTool, ok := toolsByMCPToolId[stat.ToolId]
		if !ok {
			continue
		}
		result = append(result, mcpProxyServerToolStatToHealth(proxyTool, model.MCPProxyServerToolCallStats{
			ProxyServerId: proxyTool.ProxyServerId,
			ProxyToolId:   proxyTool.Id,
			ToolId:        stat.ToolId,
			ToolName:      stat.ToolName,
			Calls:         stat.Calls,
			SuccessCalls:  stat.SuccessCalls,
			ErrorCalls:    stat.ErrorCalls,
			TimeoutCalls:  stat.TimeoutCalls,
			Quota:         stat.Quota,
			Cost:          stat.Cost,
			AvgDurationMS: stat.AvgDurationMS,
		}))
	}
	return result
}

func mcpProxyServerToolStatToHealth(proxyTool model.MCPProxyTool, stat model.MCPProxyServerToolCallStats) dto.MCPProxyServerToolHealth {
	exposedToolName := proxyTool.ExposedToolName
	if exposedToolName == "" {
		exposedToolName = stat.ToolName
	}
	return dto.MCPProxyServerToolHealth{
		ToolId:             stat.ToolId,
		ProxyToolId:        proxyTool.Id,
		ExposedToolName:    exposedToolName,
		DownstreamToolName: proxyTool.DownstreamToolName,
		Status:             proxyTool.Status,
		Calls:              stat.Calls,
		SuccessCalls:       stat.SuccessCalls,
		ErrorCalls:         stat.ErrorCalls,
		TimeoutCalls:       stat.TimeoutCalls,
		Quota:              stat.Quota,
		Cost:               stat.Cost,
		AvgDurationMS:      stat.AvgDurationMS,
		SuccessRate:        ratioPercent(stat.SuccessCalls, stat.Calls),
	}
}

func mcpProxyServerRecentErrors(calls []model.MCPToolCall) []dto.MCPProxyServerRecentError {
	result := make([]dto.MCPProxyServerRecentError, 0, len(calls))
	for _, call := range calls {
		result = append(result, dto.MCPProxyServerRecentError{
			Id:           call.Id,
			RequestId:    call.RequestId,
			ToolName:     call.ToolName,
			Status:       call.Status,
			ErrorCode:    call.ErrorCode,
			ErrorMessage: call.ErrorMessage,
			DurationMS:   call.DurationMS,
			CreatedAt:    call.CreatedAt,
		})
	}
	return result
}

func latestMCPProxyDiscoveryEventItem(proxyServerId int) (*dto.MCPProxyDiscoveryEventItem, error) {
	events, err := model.ListLatestMCPProxyDiscoveryEventsByServerIds([]int{proxyServerId})
	if err != nil {
		return nil, err
	}
	event, ok := events[proxyServerId]
	if !ok {
		return nil, nil
	}
	item := mcpProxyDiscoveryEventToDTO(event)
	return &item, nil
}

func mcpProxyReviewState(server model.MCPProxyServer, calls dto.MCPProxyServerCallHealth, discovery dto.MCPProxyServerDiscoveryInfo, transport dto.MCPProxyTransportHealth, latestCheck *dto.MCPProxyDiscoveryEventItem) (bool, []string) {
	reasons := []string{}
	if server.Status == model.MCPProxyServerStatusError {
		reasons = appendMCPProxyReviewReason(reasons, "server_error")
	}
	if server.Status == model.MCPProxyServerStatusEnabled && latestCheck == nil {
		reasons = appendMCPProxyReviewReason(reasons, "no_recent_check")
	}
	if latestCheck != nil && latestCheck.Status == model.MCPProxyDiscoveryEventStatusError {
		reasons = appendMCPProxyReviewReason(reasons, "latest_check_failed")
	}
	if discovery.SchemaChangedTools > 0 {
		reasons = appendMCPProxyReviewReason(reasons, "schema_changed_tools")
	}
	if discovery.ErrorTools > 0 {
		reasons = appendMCPProxyReviewReason(reasons, "proxy_tool_errors")
	}
	if calls.ErrorCalls+calls.TimeoutCalls > 0 {
		reasons = appendMCPProxyReviewReason(reasons, "recent_call_errors")
	}
	if strings.TrimSpace(transport.LastError) != "" {
		reasons = appendMCPProxyReviewReason(reasons, "transport_error")
	}
	return len(reasons) > 0, reasons
}

func appendMCPProxyReviewReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func mcpProxyTestEvent(proxyServerId int, startedAt int64, durationMS int, result mcpproxy.TestResult) model.MCPProxyDiscoveryEvent {
	now := common.GetTimestamp()
	return model.MCPProxyDiscoveryEvent{
		ProxyServerId:   proxyServerId,
		EventType:       model.MCPProxyDiscoveryEventTypeTest,
		Status:          model.MCPProxyDiscoveryEventStatusSuccess,
		ProtocolVersion: result.ProtocolVersion,
		ServerName:      result.ServerName,
		Capabilities:    marshalMCPProxyCapabilities(result.Capabilities),
		DurationMS:      durationMS,
		StartedAt:       startedAt,
		FinishedAt:      now,
	}
}

func mcpProxyDiscoverEvent(proxyServerId int, startedAt int64, durationMS int, result dto.MCPProxyDiscoveryResult) model.MCPProxyDiscoveryEvent {
	now := common.GetTimestamp()
	return model.MCPProxyDiscoveryEvent{
		ProxyServerId:   proxyServerId,
		EventType:       model.MCPProxyDiscoveryEventTypeDiscover,
		Status:          model.MCPProxyDiscoveryEventStatusSuccess,
		DiscoveredCount: result.DiscoveredCount,
		CreatedCount:    result.CreatedCount,
		UpdatedCount:    result.UpdatedCount,
		DisabledCount:   result.DisabledCount,
		SchemaChanged:   result.SchemaChanged,
		DurationMS:      durationMS,
		StartedAt:       startedAt,
		FinishedAt:      now,
	}
}

func recordMCPProxyDiscoveryEvent(event model.MCPProxyDiscoveryEvent) {
	now := common.GetTimestamp()
	if event.FinishedAt == 0 {
		event.FinishedAt = now
	}
	if event.StartedAt == 0 {
		event.StartedAt = event.FinishedAt
	}
	if event.DurationMS == 0 && event.FinishedAt > event.StartedAt {
		event.DurationMS = int((event.FinishedAt - event.StartedAt) * 1000)
	}
	_ = model.CreateMCPProxyDiscoveryEvent(&event)
}

func durationMSSince(startedAt time.Time) int {
	if startedAt.IsZero() {
		return 0
	}
	duration := time.Since(startedAt).Milliseconds()
	if duration < 0 {
		return 0
	}
	return int(duration)
}

func mcpProxyDiscoveryEventToDTO(event model.MCPProxyDiscoveryEvent) dto.MCPProxyDiscoveryEventItem {
	return dto.MCPProxyDiscoveryEventItem{
		Id:              event.Id,
		ProxyServerId:   event.ProxyServerId,
		EventType:       event.EventType,
		Status:          event.Status,
		Message:         event.Message,
		ProtocolVersion: event.ProtocolVersion,
		ServerName:      event.ServerName,
		Capabilities:    parseMCPProxyCapabilities(event.Capabilities),
		DiscoveredCount: event.DiscoveredCount,
		CreatedCount:    event.CreatedCount,
		UpdatedCount:    event.UpdatedCount,
		DisabledCount:   event.DisabledCount,
		SchemaChanged:   event.SchemaChanged,
		DurationMS:      event.DurationMS,
		StartedAt:       event.StartedAt,
		FinishedAt:      event.FinishedAt,
		CreatedAt:       event.CreatedAt,
	}
}

func marshalMCPProxyCapabilities(capabilities map[string]any) string {
	if len(capabilities) == 0 {
		return "{}"
	}
	bytes, err := common.Marshal(capabilities)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func parseMCPProxyCapabilities(raw string) map[string]any {
	result := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return result
	}
	if err := common.UnmarshalJsonStr(raw, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func syncMCPProxyDiscoveredTools(server model.MCPProxyServer, discovered []mcpproxy.ToolDefinition) (dto.MCPProxyDiscoveryResult, error) {
	now := common.GetTimestamp()
	result := dto.MCPProxyDiscoveryResult{
		ProxyServerId: server.Id,
		Tools:         make([]dto.MCPProxyToolAdminItem, 0, len(discovered)),
	}
	seen := map[string]bool{}
	for _, discoveredTool := range discovered {
		normalized, schemaJSON, schemaHash, err := normalizeMCPProxyDiscoveredTool(discoveredTool)
		if err != nil {
			return result, err
		}
		if seen[normalized.Name] {
			return result, fmt.Errorf("duplicate downstream tool name: %s", normalized.Name)
		}
		seen[normalized.Name] = true
		result.DiscoveredCount++
		exposedName := server.Namespace + "." + normalized.Name

		mcpTool, err := ensureMCPProxyExposedTool(server, normalized, schemaJSON, exposedName)
		if err != nil {
			return result, err
		}

		proxyTool, created, schemaChanged, err := upsertMCPProxyTool(server, normalized, schemaJSON, schemaHash, exposedName, mcpTool.Id, now)
		if err != nil {
			return result, err
		}
		if schemaChanged || proxyTool.Status != model.MCPProxyToolStatusEnabled {
			mcpTool, err = model.UpdateMCPToolFields(mcpTool.Id, map[string]any{
				"status":     model.MCPToolStatusDisabled,
				"updated_at": now,
			})
			if err != nil {
				return result, err
			}
		}
		if created {
			result.CreatedCount++
		} else if schemaChanged {
			result.SchemaChanged++
		} else {
			result.UpdatedCount++
		}
		result.Tools = append(result.Tools, mcpProxyToolToAdminDTO(*proxyTool, *mcpTool))
	}

	disabled, err := disableMissingMCPProxyTools(server.Id, seen, now)
	if err != nil {
		return result, err
	}
	result.DisabledCount = disabled

	updates := map[string]any{
		"last_discovered_at": now,
		"last_error":         "",
		"updated_at":         now,
	}
	if server.Status == model.MCPProxyServerStatusError {
		updates["status"] = model.MCPProxyServerStatusEnabled
	}
	if _, err := model.UpdateMCPProxyServerFields(server.Id, updates); err != nil {
		return result, err
	}
	return result, nil
}

func normalizeMCPProxyDiscoveredTool(tool mcpproxy.ToolDefinition) (mcpproxy.ToolDefinition, string, string, error) {
	tool.Name = strings.TrimSpace(tool.Name)
	tool.Title = strings.TrimSpace(tool.Title)
	tool.Description = strings.TrimSpace(tool.Description)
	if tool.Name == "" {
		return tool, "", "", errors.New("downstream tool name is required")
	}
	if !isValidMCPProxyDownstreamToolName(tool.Name) {
		return tool, "", "", fmt.Errorf("invalid downstream tool name: %s", tool.Name)
	}
	if tool.InputSchema == nil {
		tool.InputSchema = map[string]any{"type": "object"}
	}
	schemaBytes, err := common.Marshal(tool.InputSchema)
	if err != nil {
		return tool, "", "", err
	}
	sum := sha256.Sum256(schemaBytes)
	return tool, string(schemaBytes), "sha256:" + hex.EncodeToString(sum[:]), nil
}

func ensureMCPProxyExposedTool(server model.MCPProxyServer, discovered mcpproxy.ToolDefinition, schemaJSON string, exposedName string) (*model.MCPTool, error) {
	existing, err := model.GetMCPToolByName(exposedName)
	if err == nil {
		updates := map[string]any{
			"display_name": mcpProxyToolDisplayName(server.Namespace, discovered),
			"description":  discovered.Description,
			"category":     "third_party",
			"source":       model.MCPToolSourceMCPProxy,
			"input_schema": schemaJSON,
			"is_remote":    false,
			"updated_at":   common.GetTimestamp(),
		}
		return model.UpdateMCPToolFields(existing.Id, updates)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tool := &model.MCPTool{
		Name:         exposedName,
		DisplayName:  mcpProxyToolDisplayName(server.Namespace, discovered),
		Description:  discovered.Description,
		Category:     "third_party",
		Source:       model.MCPToolSourceMCPProxy,
		InputSchema:  schemaJSON,
		PricePerCall: 0,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		FreeQuota:    0,
		IsRemote:     false,
		Status:       model.MCPToolStatusDisabled,
		SortOrder:    0,
	}
	if err := model.CreateMCPTool(tool); err != nil {
		return nil, err
	}
	return model.UpdateMCPToolFields(tool.Id, map[string]any{
		"status": model.MCPToolStatusDisabled,
	})
}

func upsertMCPProxyTool(server model.MCPProxyServer, discovered mcpproxy.ToolDefinition, schemaJSON string, schemaHash string, exposedName string, mcpToolId int, now int64) (*model.MCPProxyTool, bool, bool, error) {
	existing, err := model.GetMCPProxyToolByServerAndDownstreamName(server.Id, discovered.Name)
	if err == nil {
		schemaChanged := existing.SchemaHash != "" && existing.SchemaHash != schemaHash
		status := existing.Status
		if status == "" || status == model.MCPProxyToolStatusDisabled {
			status = model.MCPProxyToolStatusPending
		}
		if schemaChanged {
			status = model.MCPProxyToolStatusSchemaChanged
		}
		updated, err := model.UpdateMCPProxyToolFields(existing.Id, map[string]any{
			"mcp_tool_id":             mcpToolId,
			"downstream_title":        discovered.Title,
			"downstream_description":  discovered.Description,
			"downstream_input_schema": schemaJSON,
			"exposed_tool_name":       exposedName,
			"exposed_description":     discovered.Description,
			"schema_hash":             schemaHash,
			"status":                  status,
			"last_error":              "",
			"last_discovered_at":      now,
			"updated_at":              now,
		})
		return updated, false, schemaChanged, err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, false, err
	}
	proxyTool := &model.MCPProxyTool{
		ProxyServerId:         server.Id,
		MCPToolId:             mcpToolId,
		DownstreamToolName:    discovered.Name,
		DownstreamTitle:       discovered.Title,
		DownstreamDescription: discovered.Description,
		DownstreamInputSchema: schemaJSON,
		ExposedToolName:       exposedName,
		ExposedDescription:    discovered.Description,
		SchemaHash:            schemaHash,
		Status:                model.MCPProxyToolStatusPending,
		LastDiscoveredAt:      now,
	}
	if err := model.CreateMCPProxyTool(proxyTool); err != nil {
		return nil, false, false, err
	}
	return proxyTool, true, false, nil
}

func disableMissingMCPProxyTools(proxyServerId int, seen map[string]bool, now int64) (int, error) {
	existing, err := model.ListMCPProxyToolsByServerId(proxyServerId)
	if err != nil {
		return 0, err
	}
	disabled := 0
	for _, tool := range existing {
		if seen[tool.DownstreamToolName] || tool.Status == model.MCPProxyToolStatusDisabled {
			continue
		}
		if _, err := model.UpdateMCPProxyToolFields(tool.Id, map[string]any{
			"status":     model.MCPProxyToolStatusDisabled,
			"updated_at": now,
		}); err != nil {
			return disabled, err
		}
		if tool.MCPToolId > 0 {
			if _, err := model.UpdateMCPToolFields(tool.MCPToolId, map[string]any{
				"status":     model.MCPToolStatusDisabled,
				"updated_at": now,
			}); err != nil {
				return disabled, err
			}
		}
		disabled++
	}
	return disabled, nil
}

func mcpToolsByProxyTools(tools []model.MCPProxyTool) (map[int]model.MCPTool, error) {
	ids := make([]int, 0, len(tools))
	for _, tool := range tools {
		if tool.MCPToolId > 0 {
			ids = append(ids, tool.MCPToolId)
		}
	}
	return model.ListMCPToolsByIds(ids)
}

func mcpToolValue(tool *model.MCPTool) model.MCPTool {
	if tool == nil {
		return model.MCPTool{PriceUnit: model.MCPToolPriceUnitPerCall}
	}
	return *tool
}

func mcpToolValueFromMap(tools map[int]model.MCPTool, id int) model.MCPTool {
	if tool, ok := tools[id]; ok {
		return tool
	}
	return model.MCPTool{PriceUnit: model.MCPToolPriceUnitPerCall}
}

func mcpProxyToolToAdminDTO(tool model.MCPProxyTool, mcpTool model.MCPTool) dto.MCPProxyToolAdminItem {
	var inputSchema any = map[string]any{}
	if strings.TrimSpace(tool.DownstreamInputSchema) != "" {
		var schema map[string]any
		if err := common.UnmarshalJsonStr(tool.DownstreamInputSchema, &schema); err == nil {
			inputSchema = schema
		} else {
			inputSchema = tool.DownstreamInputSchema
		}
	}
	return dto.MCPProxyToolAdminItem{
		Id:                    tool.Id,
		ProxyServerId:         tool.ProxyServerId,
		MCPToolId:             tool.MCPToolId,
		DownstreamToolName:    tool.DownstreamToolName,
		DownstreamTitle:       tool.DownstreamTitle,
		DownstreamDescription: tool.DownstreamDescription,
		DownstreamInputSchema: inputSchema,
		ExposedToolName:       tool.ExposedToolName,
		ExposedDescription:    tool.ExposedDescription,
		SchemaHash:            tool.SchemaHash,
		Status:                tool.Status,
		PricePerCall:          mcpTool.PricePerCall,
		PriceUnit:             mcpTool.PriceUnit,
		FreeQuota:             mcpTool.FreeQuota,
		SortOrder:             mcpTool.SortOrder,
		LastError:             tool.LastError,
		LastDiscoveredAt:      tool.LastDiscoveredAt,
		CreatedAt:             tool.CreatedAt,
		UpdatedAt:             tool.UpdatedAt,
	}
}

func mcpProxyCreateRequestToModel(req dto.MCPProxyServerCreateRequest) (*model.MCPProxyServer, error) {
	transport := strings.TrimSpace(req.Transport)
	if transport == "" {
		transport = model.MCPProxyTransportHTTP
	}
	authType := strings.TrimSpace(req.AuthType)
	if authType == "" {
		authType = model.MCPProxyAuthTypeNone
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = model.MCPProxyVisibilityAdmin
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = model.MCPProxyServerStatusDisabled
	}
	allowedGroups, err := marshalMCPProxyAllowedGroups(req.AllowedGroups)
	if err != nil {
		return nil, err
	}
	server := &model.MCPProxyServer{
		Name:            strings.TrimSpace(req.Name),
		Namespace:       strings.TrimSpace(req.Namespace),
		Transport:       transport,
		Endpoint:        strings.TrimSpace(req.Endpoint),
		Command:         strings.TrimSpace(req.Command),
		AuthType:        authType,
		AuthRef:         strings.TrimSpace(req.AuthRef),
		TimeoutMS:       req.TimeoutMS,
		MaxResultSize:   req.MaxResultSize,
		MaxMetadataSize: req.MaxMetadataSize,
		Visibility:      visibility,
		AllowedGroups:   allowedGroups,
		Status:          status,
	}
	applyMCPProxyServerDefaults(server)
	normalizedAuthRef, err := normalizeMCPProxyAuthRef(server.AuthType, server.AuthRef)
	if err != nil {
		return nil, err
	}
	server.AuthRef = normalizedAuthRef
	if err := validateMCPProxyServerModel(server); err != nil {
		return nil, err
	}
	return server, nil
}

func mcpProxyServerUpdateFields(existing model.MCPProxyServer, req dto.MCPProxyServerUpdateRequest) (map[string]any, error) {
	updates := map[string]any{}
	candidate := existing
	if req.Name != nil {
		candidate.Name = strings.TrimSpace(*req.Name)
		updates["name"] = candidate.Name
	}
	if req.Namespace != nil {
		candidate.Namespace = strings.TrimSpace(*req.Namespace)
		updates["namespace"] = candidate.Namespace
	}
	if req.Transport != nil {
		candidate.Transport = strings.TrimSpace(*req.Transport)
		updates["transport"] = candidate.Transport
	}
	if req.Endpoint != nil {
		candidate.Endpoint = strings.TrimSpace(*req.Endpoint)
		updates["endpoint"] = candidate.Endpoint
	}
	if req.Command != nil {
		candidate.Command = strings.TrimSpace(*req.Command)
		updates["command"] = candidate.Command
	}
	if req.AuthType != nil {
		candidate.AuthType = strings.TrimSpace(*req.AuthType)
		updates["auth_type"] = candidate.AuthType
		if candidate.AuthType == model.MCPProxyAuthTypeNone {
			candidate.AuthRef = ""
			updates["auth_ref"] = ""
		}
	}
	if req.AuthRef != nil {
		candidate.AuthRef = strings.TrimSpace(*req.AuthRef)
		updates["auth_ref"] = candidate.AuthRef
	}
	if req.TimeoutMS != nil {
		candidate.TimeoutMS = *req.TimeoutMS
		updates["timeout_ms"] = candidate.TimeoutMS
	}
	if req.MaxResultSize != nil {
		candidate.MaxResultSize = *req.MaxResultSize
		updates["max_result_size"] = candidate.MaxResultSize
	}
	if req.MaxMetadataSize != nil {
		candidate.MaxMetadataSize = *req.MaxMetadataSize
		updates["max_metadata_size"] = candidate.MaxMetadataSize
	}
	if req.Visibility != nil {
		candidate.Visibility = strings.TrimSpace(*req.Visibility)
		updates["visibility"] = candidate.Visibility
	}
	if req.AllowedGroups != nil {
		allowedGroups, err := marshalMCPProxyAllowedGroups(*req.AllowedGroups)
		if err != nil {
			return nil, err
		}
		candidate.AllowedGroups = allowedGroups
		updates["allowed_groups"] = allowedGroups
	}
	if req.Status != nil {
		candidate.Status = strings.TrimSpace(*req.Status)
		updates["status"] = candidate.Status
	}
	if len(updates) == 0 {
		return updates, nil
	}
	normalizedAuthRef, err := normalizeMCPProxyAuthRef(candidate.AuthType, candidate.AuthRef)
	if err != nil {
		return nil, err
	}
	candidate.AuthRef = normalizedAuthRef
	if req.AuthType != nil || req.AuthRef != nil {
		updates["auth_ref"] = normalizedAuthRef
	}
	if err := validateMCPProxyServerModel(&candidate); err != nil {
		return nil, err
	}
	updates["updated_at"] = common.GetTimestamp()
	return updates, nil
}

func mcpProxyToolUpdateFields(existing model.MCPProxyTool, req dto.MCPProxyToolUpdateRequest) (map[string]any, map[string]any, error) {
	updates := map[string]any{}
	mcpToolUpdates := map[string]any{}
	if req.ExposedToolName != nil {
		exposedName := strings.TrimSpace(*req.ExposedToolName)
		if err := validateMCPProxyExposedToolName(exposedName); err != nil {
			return nil, nil, err
		}
		server, err := model.GetMCPProxyServerById(existing.ProxyServerId)
		if err != nil {
			return nil, nil, err
		}
		if !strings.HasPrefix(exposedName, server.Namespace+".") {
			return nil, nil, errors.New("exposed_tool_name namespace must match proxy server namespace")
		}
		updates["exposed_tool_name"] = exposedName
		mcpToolUpdates["name"] = exposedName
	}
	if req.ExposedDescription != nil {
		description := strings.TrimSpace(*req.ExposedDescription)
		updates["exposed_description"] = description
		mcpToolUpdates["description"] = description
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if err := validateMCPProxyToolStatus(status); err != nil {
			return nil, nil, err
		}
		updates["status"] = status
		switch status {
		case model.MCPProxyToolStatusEnabled:
			mcpToolUpdates["status"] = model.MCPToolStatusEnabled
		case model.MCPProxyToolStatusDisabled, model.MCPProxyToolStatusPending, model.MCPProxyToolStatusSchemaChanged, model.MCPProxyToolStatusError:
			mcpToolUpdates["status"] = model.MCPToolStatusDisabled
		}
	}
	if req.PricePerCall != nil {
		if *req.PricePerCall < 0 {
			return nil, nil, errors.New("price_per_call must be greater than or equal to 0")
		}
		mcpToolUpdates["price_per_call"] = *req.PricePerCall
	}
	if req.PriceUnit != nil {
		priceUnit := strings.TrimSpace(*req.PriceUnit)
		if err := validateMCPToolPriceUnit(priceUnit); err != nil {
			return nil, nil, err
		}
		mcpToolUpdates["price_unit"] = priceUnit
	}
	if req.FreeQuota != nil {
		if *req.FreeQuota < 0 {
			return nil, nil, errors.New("free_quota must be greater than or equal to 0")
		}
		mcpToolUpdates["free_quota"] = *req.FreeQuota
	}
	if req.SortOrder != nil {
		mcpToolUpdates["sort_order"] = *req.SortOrder
	}
	if len(updates) > 0 {
		updates["updated_at"] = common.GetTimestamp()
	}
	if len(mcpToolUpdates) > 0 {
		mcpToolUpdates["updated_at"] = common.GetTimestamp()
	}
	return updates, mcpToolUpdates, nil
}

func validateMCPProxyServerModel(server *model.MCPProxyServer) error {
	if server == nil {
		return errors.New("mcp proxy server is required")
	}
	if strings.TrimSpace(server.Name) == "" {
		return errors.New("name is required")
	}
	if err := validateMCPProxyNamespace(server.Namespace); err != nil {
		return err
	}
	if err := validateMCPProxyTransport(server.Transport); err != nil {
		return err
	}
	if err := validateMCPProxyAuthType(server.AuthType); err != nil {
		return err
	}
	if err := validateMCPProxyAuthRef(server.AuthType, server.AuthRef); err != nil {
		return err
	}
	if err := validateMCPProxyVisibility(server.Visibility); err != nil {
		return err
	}
	if err := validateMCPProxyServerStatus(server.Status); err != nil {
		return err
	}
	if isEndpointRequiredMCPProxyTransport(server.Transport) && strings.TrimSpace(server.Endpoint) == "" {
		return errors.New("endpoint is required for mcp proxy server")
	}
	if isBridgeMCPProxyTransport(server.Transport) {
		if _, err := mcpproxy.ParseBridgeEndpoint(server.Endpoint); err != nil {
			return err
		}
		if server.AuthType != model.MCPProxyAuthTypeNone {
			return errors.New("bridge mcp proxy transport does not accept server-side auth_ref")
		}
	}
	if strings.TrimSpace(server.Command) != "" {
		return errors.New("stdio/local command proxy is not enabled yet")
	}
	if server.TimeoutMS <= 0 {
		return errors.New("timeout_ms must be positive")
	}
	if server.MaxResultSize <= 0 {
		return errors.New("max_result_size must be positive")
	}
	if server.MaxMetadataSize <= 0 {
		return errors.New("max_metadata_size must be positive")
	}
	return nil
}

func applyMCPProxyServerDefaults(server *model.MCPProxyServer) {
	if server == nil {
		return
	}
	if strings.TrimSpace(server.Transport) == "" {
		server.Transport = model.MCPProxyTransportHTTP
	}
	if strings.TrimSpace(server.AuthType) == "" {
		server.AuthType = model.MCPProxyAuthTypeNone
	}
	if strings.TrimSpace(server.Visibility) == "" {
		server.Visibility = model.MCPProxyVisibilityAdmin
	}
	if strings.TrimSpace(server.Status) == "" {
		server.Status = model.MCPProxyServerStatusDisabled
	}
	if server.TimeoutMS <= 0 {
		server.TimeoutMS = 30000
	}
	if server.MaxResultSize <= 0 {
		server.MaxResultSize = 1048576
	}
	if server.MaxMetadataSize <= 0 {
		server.MaxMetadataSize = 65536
	}
}

func validateMCPProxyToolStatus(status string) error {
	switch strings.TrimSpace(status) {
	case model.MCPProxyToolStatusPending, model.MCPProxyToolStatusEnabled, model.MCPProxyToolStatusDisabled, model.MCPProxyToolStatusSchemaChanged, model.MCPProxyToolStatusError:
		return nil
	default:
		return fmt.Errorf("unsupported mcp proxy tool status: %s", status)
	}
}

func validateMCPProxyExposedToolName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("exposed_tool_name is required")
	}
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return errors.New("exposed_tool_name must use namespace.tool_name format")
	}
	if err := validateMCPProxyNamespace(parts[0]); err != nil {
		return err
	}
	if !isValidMCPProxyDownstreamToolName(parts[1]) {
		return errors.New("invalid exposed tool suffix")
	}
	return nil
}

func validateMCPProxyNamespace(namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return errors.New("namespace is required")
	}
	if !mcpProxyNamespacePattern.MatchString(namespace) {
		return errors.New("namespace must be 3-64 chars and contain only lowercase letters, numbers, underscores, or hyphens")
	}
	return nil
}

func validateMCPProxyTransport(transport string) error {
	switch strings.TrimSpace(transport) {
	case model.MCPProxyTransportHTTP, model.MCPProxyTransportSSE, model.MCPProxyTransportStreamableHTTP:
		return nil
	case model.MCPProxyTransportBridge, model.MCPProxyTransportQidianBrowser:
		return nil
	case model.MCPProxyTransportStdio, model.MCPProxyTransportLocalhost:
		return fmt.Errorf("transport %s is planned but not enabled yet", transport)
	default:
		return fmt.Errorf("unsupported mcp proxy transport: %s", transport)
	}
}

func validateMCPProxyAuthType(authType string) error {
	switch strings.TrimSpace(authType) {
	case model.MCPProxyAuthTypeNone, model.MCPProxyAuthTypeBearer, model.MCPProxyAuthTypeBasic, model.MCPProxyAuthTypeHeader, model.MCPProxyAuthTypeOAuth:
		return nil
	default:
		return fmt.Errorf("unsupported mcp proxy auth_type: %s", authType)
	}
}

func validateMCPProxyAuthRef(authType string, authRef string) error {
	authType = strings.TrimSpace(authType)
	authRef = strings.TrimSpace(authRef)
	if authType == "" || authType == model.MCPProxyAuthTypeNone {
		return nil
	}
	if authRef == "" {
		return errors.New("auth_ref is required when auth_type is not none")
	}
	return secretref.Validate(authRef)
}

func normalizeMCPProxyAuthRef(authType string, authRef string) (string, error) {
	authType = strings.TrimSpace(authType)
	authRef = strings.TrimSpace(authRef)
	if authType == "" || authType == model.MCPProxyAuthTypeNone {
		return "", nil
	}
	if authRef == "" {
		return "", errors.New("auth_ref is required when auth_type is not none")
	}
	return secretref.Normalize(authRef)
}

func validateMCPProxyVisibility(visibility string) error {
	switch strings.TrimSpace(visibility) {
	case model.MCPProxyVisibilityAdmin, model.MCPProxyVisibilityGroup, model.MCPProxyVisibilityPublic:
		return nil
	default:
		return fmt.Errorf("unsupported mcp proxy visibility: %s", visibility)
	}
}

func validateMCPProxyServerStatus(status string) error {
	switch strings.TrimSpace(status) {
	case model.MCPProxyServerStatusEnabled, model.MCPProxyServerStatusDisabled, model.MCPProxyServerStatusError, model.MCPProxyServerStatusArchived:
		return nil
	default:
		return fmt.Errorf("unsupported mcp proxy server status: %s", status)
	}
}

func isRemoteMCPProxyTransport(transport string) bool {
	switch strings.TrimSpace(transport) {
	case model.MCPProxyTransportHTTP, model.MCPProxyTransportSSE, model.MCPProxyTransportStreamableHTTP:
		return true
	default:
		return false
	}
}

func isBridgeMCPProxyTransport(transport string) bool {
	switch strings.TrimSpace(transport) {
	case model.MCPProxyTransportBridge, model.MCPProxyTransportQidianBrowser:
		return true
	default:
		return false
	}
}

func isEndpointRequiredMCPProxyTransport(transport string) bool {
	return isRemoteMCPProxyTransport(transport) || isBridgeMCPProxyTransport(transport)
}

func marshalMCPProxyAllowedGroups(groups []string) (string, error) {
	normalized := make([]string, 0, len(groups))
	seen := map[string]bool{}
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" || seen[group] {
			continue
		}
		seen[group] = true
		normalized = append(normalized, group)
	}
	if len(normalized) == 0 {
		return "", nil
	}
	bytes, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func unmarshalMCPProxyAllowedGroups(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var groups []string
	if err := common.UnmarshalJsonStr(raw, &groups); err != nil {
		return []string{}
	}
	return groups
}

func mcpProxyServerToAdminDTO(server model.MCPProxyServer) dto.MCPProxyServerAdminItem {
	return dto.MCPProxyServerAdminItem{
		Id:               server.Id,
		Name:             server.Name,
		Namespace:        server.Namespace,
		Transport:        server.Transport,
		Endpoint:         server.Endpoint,
		Command:          server.Command,
		AuthType:         server.AuthType,
		AuthRef:          maskMCPProxyAuthRef(server.AuthType, server.AuthRef),
		TimeoutMS:        server.TimeoutMS,
		MaxResultSize:    server.MaxResultSize,
		MaxMetadataSize:  server.MaxMetadataSize,
		Visibility:       server.Visibility,
		AllowedGroups:    unmarshalMCPProxyAllowedGroups(server.AllowedGroups),
		Status:           server.Status,
		LastError:        server.LastError,
		LastDiscoveredAt: server.LastDiscoveredAt,
		CreatedAt:        server.CreatedAt,
		UpdatedAt:        server.UpdatedAt,
		Archived:         server.DeletedAt.Valid || server.Status == model.MCPProxyServerStatusArchived,
	}
}

func maskMCPProxyAuthRef(authType string, authRef string) string {
	if strings.TrimSpace(authType) == "" || strings.TrimSpace(authType) == model.MCPProxyAuthTypeNone {
		return ""
	}
	authRef = strings.TrimSpace(authRef)
	if authRef == "" {
		return ""
	}
	return secretref.ConfiguredMask
}

func isValidMCPProxyDownstreamToolName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func mcpProxyToolDisplayName(namespace string, discovered mcpproxy.ToolDefinition) string {
	if strings.TrimSpace(discovered.Title) != "" {
		return strings.TrimSpace(discovered.Title)
	}
	return strings.TrimSpace(namespace) + "." + strings.TrimSpace(discovered.Name)
}
