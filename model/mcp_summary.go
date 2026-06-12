package model

import (
	"strings"

	"gorm.io/gorm"
)

type MCPToolStats struct {
	TotalTools    int64 `gorm:"column:total_tools"`
	EnabledTools  int64 `gorm:"column:enabled_tools"`
	DisabledTools int64 `gorm:"column:disabled_tools"`
	RemoteTools   int64 `gorm:"column:remote_tools"`
}

type MCPToolCallStats struct {
	TotalCalls    int64   `gorm:"column:total_calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	ErrorCalls    int64   `gorm:"column:error_calls"`
	TimeoutCalls  int64   `gorm:"column:timeout_calls"`
	PendingCalls  int64   `gorm:"column:pending_calls"`
	SettledCalls  int64   `gorm:"column:settled_calls"`
	Unsettled     int64   `gorm:"column:unsettled"`
	FreeCalls     int64   `gorm:"column:free_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	ResultSize    int64   `gorm:"column:result_size"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPTopToolStats struct {
	ToolName      string  `gorm:"column:tool_name"`
	Calls         int64   `gorm:"column:calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPToolCallToolStats struct {
	ToolId        int     `gorm:"column:tool_id"`
	ToolName      string  `gorm:"column:tool_name"`
	Calls         int64   `gorm:"column:calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	ErrorCalls    int64   `gorm:"column:error_calls"`
	TimeoutCalls  int64   `gorm:"column:timeout_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPBillingAnomalyStats struct {
	UnsettledSuccessCalls int64 `gorm:"column:unsettled_success_calls"`
	FailedChargedCalls    int64 `gorm:"column:failed_charged_calls"`
	SettledChargedCalls   int64 `gorm:"column:settled_charged_calls"`
}

type MCPProxyServerCallStats struct {
	ProxyServerId int     `gorm:"column:proxy_server_id"`
	TotalCalls    int64   `gorm:"column:total_calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	ErrorCalls    int64   `gorm:"column:error_calls"`
	TimeoutCalls  int64   `gorm:"column:timeout_calls"`
	PendingCalls  int64   `gorm:"column:pending_calls"`
	SettledCalls  int64   `gorm:"column:settled_calls"`
	Unsettled     int64   `gorm:"column:unsettled"`
	FreeCalls     int64   `gorm:"column:free_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	ResultSize    int64   `gorm:"column:result_size"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPProxyServerToolCallStats struct {
	ProxyServerId int     `gorm:"column:proxy_server_id"`
	ProxyToolId   int64   `gorm:"column:proxy_tool_id"`
	ToolId        int     `gorm:"column:tool_id"`
	ToolName      string  `gorm:"column:tool_name"`
	Calls         int64   `gorm:"column:calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	ErrorCalls    int64   `gorm:"column:error_calls"`
	TimeoutCalls  int64   `gorm:"column:timeout_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPProxyTrendFilter struct {
	UserId        int
	ProxyServerId int
	ProxyToolId   int64
	Status        string
	StartTime     int64
	EndTime       int64
}

type MCPProxyTrendStats struct {
	TotalCalls    int64   `gorm:"column:total_calls"`
	SuccessCalls  int64   `gorm:"column:success_calls"`
	ErrorCalls    int64   `gorm:"column:error_calls"`
	TimeoutCalls  int64   `gorm:"column:timeout_calls"`
	PendingCalls  int64   `gorm:"column:pending_calls"`
	SettledCalls  int64   `gorm:"column:settled_calls"`
	Unsettled     int64   `gorm:"column:unsettled"`
	FreeCalls     int64   `gorm:"column:free_calls"`
	Quota         int64   `gorm:"column:quota"`
	Cost          float64 `gorm:"column:cost"`
	ResultSize    int64   `gorm:"column:result_size"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

type MCPProxyTrendBucketStats struct {
	BucketStart int64 `gorm:"column:bucket_start"`
	MCPProxyTrendStats
}

type MCPProxyTrendServerStats struct {
	ProxyServerId int    `gorm:"column:proxy_server_id"`
	Name          string `gorm:"column:name"`
	Namespace     string `gorm:"column:namespace"`
	MCPProxyTrendStats
}

type MCPProxyTrendToolStats struct {
	ProxyServerId      int    `gorm:"column:proxy_server_id"`
	ProxyToolId        int64  `gorm:"column:proxy_tool_id"`
	ToolId             int    `gorm:"column:tool_id"`
	ExposedToolName    string `gorm:"column:exposed_tool_name"`
	DownstreamToolName string `gorm:"column:downstream_tool_name"`
	MCPProxyTrendStats
}

type MCPProxyTrendSummary struct {
	Totals  MCPProxyTrendStats
	Buckets []MCPProxyTrendBucketStats
	Servers []MCPProxyTrendServerStats
	Tools   []MCPProxyTrendToolStats
}

func GetMCPToolStats() (MCPToolStats, error) {
	var stats MCPToolStats
	err := DB.Model(&MCPTool{}).
		Select(
			`COUNT(*) AS total_tools,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS enabled_tools,
			COALESCE(SUM(CASE WHEN status <> ? THEN 1 ELSE 0 END), 0) AS disabled_tools,
			COALESCE(SUM(CASE WHEN is_remote = ? THEN 1 ELSE 0 END), 0) AS remote_tools`,
			MCPToolStatusEnabled,
			MCPToolStatusEnabled,
			true,
		).
		Scan(&stats).Error
	return stats, err
}

func SummarizeMCPProxyTrends(filter MCPProxyTrendFilter, bucketSeconds int64, serverLimit int, toolLimit int) (MCPProxyTrendSummary, error) {
	if bucketSeconds <= 0 {
		bucketSeconds = 86400
	}
	if serverLimit <= 0 {
		serverLimit = 5
	}
	if serverLimit > 20 {
		serverLimit = 20
	}
	if toolLimit <= 0 {
		toolLimit = 5
	}
	if toolLimit > 20 {
		toolLimit = 20
	}

	summary := MCPProxyTrendSummary{}
	if err := mcpProxyTrendQuery(filter).
		Select(mcpProxyTrendAggregateSelect(), mcpProxyTrendAggregateArgs()...).
		Scan(&summary.Totals).Error; err != nil {
		return summary, err
	}

	bucketExpression := "calls.created_at - (calls.created_at % ?)"
	bucketArgs := append([]any{bucketSeconds}, mcpProxyTrendAggregateArgs()...)
	if err := mcpProxyTrendQuery(filter).
		Select(bucketExpression+" AS bucket_start, "+mcpProxyTrendAggregateSelect(), bucketArgs...).
		Group("bucket_start").
		Order("bucket_start ASC").
		Scan(&summary.Buckets).Error; err != nil {
		return summary, err
	}

	if err := mcpProxyTrendQuery(filter).
		Select(
			"proxy_servers.id AS proxy_server_id, proxy_servers.name AS name, proxy_servers.namespace AS namespace, "+mcpProxyTrendAggregateSelect(),
			mcpProxyTrendAggregateArgs()...,
		).
		Group("proxy_servers.id, proxy_servers.name, proxy_servers.namespace").
		Order("total_calls DESC, quota DESC, proxy_servers.id ASC").
		Limit(serverLimit).
		Scan(&summary.Servers).Error; err != nil {
		return summary, err
	}

	if err := mcpProxyTrendQuery(filter).
		Select(
			`proxy_tools.proxy_server_id AS proxy_server_id,
			proxy_tools.id AS proxy_tool_id,
			calls.tool_id AS tool_id,
			proxy_tools.exposed_tool_name AS exposed_tool_name,
			proxy_tools.downstream_tool_name AS downstream_tool_name, `+mcpProxyTrendAggregateSelect(),
			mcpProxyTrendAggregateArgs()...,
		).
		Group("proxy_tools.proxy_server_id, proxy_tools.id, calls.tool_id, proxy_tools.exposed_tool_name, proxy_tools.downstream_tool_name").
		Order("total_calls DESC, error_calls DESC, quota DESC, proxy_tools.id ASC").
		Limit(toolLimit).
		Scan(&summary.Tools).Error; err != nil {
		return summary, err
	}
	return summary, nil
}

func mcpProxyTrendQuery(filter MCPProxyTrendFilter) *gorm.DB {
	query := DB.Table("mcp_tool_calls AS calls").
		Joins("JOIN mcp_proxy_tools AS proxy_tools ON proxy_tools.mcp_tool_id = calls.tool_id").
		Joins("JOIN mcp_proxy_servers AS proxy_servers ON proxy_servers.id = proxy_tools.proxy_server_id").
		Where("proxy_tools.deleted_at IS NULL").
		Where("proxy_servers.deleted_at IS NULL")
	if filter.UserId > 0 {
		query = query.Where("calls.user_id = ?", filter.UserId)
	}
	if filter.ProxyServerId > 0 {
		query = query.Where("proxy_tools.proxy_server_id = ?", filter.ProxyServerId)
	}
	if filter.ProxyToolId > 0 {
		query = query.Where("proxy_tools.id = ?", filter.ProxyToolId)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("calls.status = ?", strings.TrimSpace(filter.Status))
	}
	if filter.StartTime > 0 {
		query = query.Where("calls.created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("calls.created_at <= ?", filter.EndTime)
	}
	return query
}

func mcpProxyTrendAggregateSelect() string {
	return `COUNT(*) AS total_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS pending_calls,
		COALESCE(SUM(CASE WHEN calls.settled_at > 0 THEN 1 ELSE 0 END), 0) AS settled_calls,
		COALESCE(SUM(CASE WHEN calls.settled_at = 0 AND calls.quota > 0 THEN 1 ELSE 0 END), 0) AS unsettled,
		COALESCE(SUM(CASE WHEN calls.free_used = ? THEN 1 ELSE 0 END), 0) AS free_calls,
		COALESCE(SUM(calls.quota), 0) AS quota,
		COALESCE(SUM(calls.cost), 0) AS cost,
		COALESCE(SUM(calls.result_size), 0) AS result_size,
		COALESCE(AVG(calls.duration_ms), 0) AS avg_duration_ms`
}

func mcpProxyTrendAggregateArgs() []any {
	return []any{
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
		MCPToolCallStatusPending,
		true,
	}
}

func GetMCPToolCallStats(filter MCPToolCallFilter) (MCPToolCallStats, error) {
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var stats MCPToolCallStats
	err := query.Select(
		`COUNT(*) AS total_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending_calls,
		COALESCE(SUM(CASE WHEN settled_at > 0 THEN 1 ELSE 0 END), 0) AS settled_calls,
		COALESCE(SUM(CASE WHEN settled_at = 0 AND quota > 0 THEN 1 ELSE 0 END), 0) AS unsettled,
		COALESCE(SUM(CASE WHEN free_used = ? THEN 1 ELSE 0 END), 0) AS free_calls,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(cost), 0) AS cost,
		COALESCE(SUM(result_size), 0) AS result_size,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
		MCPToolCallStatusPending,
		true,
	).Scan(&stats).Error
	return stats, err
}

func ListMCPTopToolStats(filter MCPToolCallFilter, limit int) ([]MCPTopToolStats, error) {
	if limit <= 0 {
		limit = 5
	}
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var stats []MCPTopToolStats
	err := query.Select(
		`tool_name,
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(cost), 0) AS cost,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
	).
		Group("tool_name").
		Order("calls DESC, quota DESC").
		Limit(limit).
		Scan(&stats).Error
	return stats, err
}

func ListMCPToolCallToolStats(filter MCPToolCallFilter, limit int) ([]MCPToolCallToolStats, error) {
	if limit <= 0 {
		limit = 5
	}
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var stats []MCPToolCallToolStats
	err := query.Select(
		`tool_id,
		tool_name,
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(cost), 0) AS cost,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
	).
		Group("tool_id, tool_name").
		Order("calls DESC, error_calls DESC, quota DESC").
		Limit(limit).
		Scan(&stats).Error
	return stats, err
}

func CountMCPToolCallToolStats(filter MCPToolCallFilter) (int64, error) {
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)
	subQuery := query.Select("tool_id, tool_name").Group("tool_id, tool_name")

	var total int64
	err := DB.Table("(?) AS tool_stats", subQuery).Count(&total).Error
	return total, err
}

func ListMCPToolErrorStats(filter MCPToolCallFilter, limit int) ([]MCPToolCallToolStats, error) {
	if limit <= 0 {
		limit = 5
	}
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var stats []MCPToolCallToolStats
	err := query.Select(
		`tool_id,
		tool_name,
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(cost), 0) AS cost,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
	).
		Group("tool_id, tool_name").
		Having("COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) + COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) > 0", MCPToolCallStatusError, MCPToolCallStatusTimeout).
		Order("error_calls DESC, timeout_calls DESC, calls DESC, tool_name ASC").
		Limit(limit).
		Scan(&stats).Error
	return stats, err
}

func GetMCPBillingAnomalyStats(filter MCPToolCallFilter) (MCPBillingAnomalyStats, error) {
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var stats MCPBillingAnomalyStats
	err := query.Select(
		`COALESCE(SUM(CASE WHEN status = ? AND quota > 0 AND settled_at = 0 THEN 1 ELSE 0 END), 0) AS unsettled_success_calls,
		COALESCE(SUM(CASE WHEN status IN ? AND quota > 0 THEN 1 ELSE 0 END), 0) AS failed_charged_calls,
		COALESCE(SUM(CASE WHEN status = ? AND quota > 0 AND settled_at > 0 THEN 1 ELSE 0 END), 0) AS settled_charged_calls`,
		MCPToolCallStatusSuccess,
		[]string{MCPToolCallStatusError, MCPToolCallStatusTimeout},
		MCPToolCallStatusSuccess,
	).Scan(&stats).Error
	return stats, err
}

func ListRecentMCPToolCallErrors(filter MCPToolCallFilter, limit int) ([]MCPToolCall, error) {
	if limit <= 0 {
		limit = 5
	}
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var calls []MCPToolCall
	err := query.Where(
		"status IN ? OR error_message <> ''",
		[]string{MCPToolCallStatusError, MCPToolCallStatusTimeout},
	).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&calls).Error
	return calls, err
}

func ListMCPProxyServerCallStats(proxyServerIds []int, startTime int64) ([]MCPProxyServerCallStats, error) {
	ids := uniquePositiveInts(proxyServerIds)
	if len(ids) == 0 {
		return []MCPProxyServerCallStats{}, nil
	}
	query := DB.Table("mcp_tool_calls AS calls").
		Joins("JOIN mcp_proxy_tools AS proxy_tools ON proxy_tools.mcp_tool_id = calls.tool_id").
		Where("proxy_tools.proxy_server_id IN ?", ids).
		Where("proxy_tools.deleted_at IS NULL")
	if startTime > 0 {
		query = query.Where("calls.created_at >= ?", startTime)
	}

	var stats []MCPProxyServerCallStats
	err := query.Select(
		`proxy_tools.proxy_server_id AS proxy_server_id,
		COUNT(*) AS total_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS pending_calls,
		COALESCE(SUM(CASE WHEN calls.settled_at > 0 THEN 1 ELSE 0 END), 0) AS settled_calls,
		COALESCE(SUM(CASE WHEN calls.settled_at = 0 AND calls.quota > 0 THEN 1 ELSE 0 END), 0) AS unsettled,
		COALESCE(SUM(CASE WHEN calls.free_used = ? THEN 1 ELSE 0 END), 0) AS free_calls,
		COALESCE(SUM(calls.quota), 0) AS quota,
		COALESCE(SUM(calls.cost), 0) AS cost,
		COALESCE(SUM(calls.result_size), 0) AS result_size,
		COALESCE(AVG(calls.duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
		MCPToolCallStatusPending,
		true,
	).
		Group("proxy_tools.proxy_server_id").
		Scan(&stats).Error
	return stats, err
}

func ListMCPProxyServerToolCallStats(proxyServerIds []int, startTime int64) ([]MCPProxyServerToolCallStats, error) {
	ids := uniquePositiveInts(proxyServerIds)
	if len(ids) == 0 {
		return []MCPProxyServerToolCallStats{}, nil
	}
	query := DB.Table("mcp_tool_calls AS calls").
		Joins("JOIN mcp_proxy_tools AS proxy_tools ON proxy_tools.mcp_tool_id = calls.tool_id").
		Where("proxy_tools.proxy_server_id IN ?", ids).
		Where("proxy_tools.deleted_at IS NULL")
	if startTime > 0 {
		query = query.Where("calls.created_at >= ?", startTime)
	}

	var stats []MCPProxyServerToolCallStats
	err := query.Select(
		`proxy_tools.proxy_server_id AS proxy_server_id,
		proxy_tools.id AS proxy_tool_id,
		calls.tool_id AS tool_id,
		calls.tool_name AS tool_name,
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS error_calls,
		COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) AS timeout_calls,
		COALESCE(SUM(calls.quota), 0) AS quota,
		COALESCE(SUM(calls.cost), 0) AS cost,
		COALESCE(AVG(calls.duration_ms), 0) AS avg_duration_ms`,
		MCPToolCallStatusSuccess,
		MCPToolCallStatusError,
		MCPToolCallStatusTimeout,
	).
		Group("proxy_tools.proxy_server_id, proxy_tools.id, calls.tool_id, calls.tool_name").
		Order("calls DESC, error_calls DESC, quota DESC").
		Scan(&stats).Error
	return stats, err
}

func ListMCPProxyErrorToolStats(filter MCPProxyTrendFilter, limit int) ([]MCPProxyTrendToolStats, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	var stats []MCPProxyTrendToolStats
	err := mcpProxyTrendQuery(filter).
		Select(
			`proxy_tools.proxy_server_id AS proxy_server_id,
			proxy_tools.id AS proxy_tool_id,
			calls.tool_id AS tool_id,
			proxy_tools.exposed_tool_name AS exposed_tool_name,
			proxy_tools.downstream_tool_name AS downstream_tool_name, `+mcpProxyTrendAggregateSelect(),
			mcpProxyTrendAggregateArgs()...,
		).
		Group("proxy_tools.proxy_server_id, proxy_tools.id, calls.tool_id, proxy_tools.exposed_tool_name, proxy_tools.downstream_tool_name").
		Having("COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) + COALESCE(SUM(CASE WHEN calls.status = ? THEN 1 ELSE 0 END), 0) > 0", MCPToolCallStatusError, MCPToolCallStatusTimeout).
		Order("error_calls DESC, timeout_calls DESC, total_calls DESC, proxy_tools.id ASC").
		Limit(limit).
		Scan(&stats).Error
	return stats, err
}
