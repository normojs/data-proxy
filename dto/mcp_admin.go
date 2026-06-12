package dto

type MCPToolCreateRequest struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	InputSchema  any      `json:"input_schema"`
	PricePerCall *float64 `json:"price_per_call,omitempty"`
	PriceUnit    string   `json:"price_unit"`
	FreeQuota    *int     `json:"free_quota,omitempty"`
	Status       *int     `json:"status,omitempty"`
	SortOrder    *int     `json:"sort_order,omitempty"`
}

type MCPToolUpdateRequest struct {
	DisplayName  *string  `json:"display_name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Category     *string  `json:"category,omitempty"`
	PricePerCall *float64 `json:"price_per_call,omitempty"`
	PriceUnit    *string  `json:"price_unit,omitempty"`
	FreeQuota    *int     `json:"free_quota,omitempty"`
	Status       *int     `json:"status,omitempty"`
	SortOrder    *int     `json:"sort_order,omitempty"`
}

type MCPToolAdminItem struct {
	Id           int     `json:"id"`
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	Description  string  `json:"description"`
	Category     string  `json:"category"`
	Source       string  `json:"source"`
	PluginId     *int    `json:"plugin_id,omitempty"`
	OpenAPIUrl   string  `json:"openapi_url"`
	InputSchema  any     `json:"input_schema"`
	PricePerCall float64 `json:"price_per_call"`
	PriceUnit    string  `json:"price_unit"`
	FreeQuota    int     `json:"free_quota"`
	IsRemote     bool    `json:"is_remote"`
	Status       int     `json:"status"`
	SortOrder    int     `json:"sort_order"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}

type MCPToolCallAdminItem struct {
	Id              int64   `json:"id"`
	UserId          int     `json:"user_id"`
	TokenId         int     `json:"token_id"`
	ToolId          int     `json:"tool_id"`
	ToolName        string  `json:"tool_name"`
	RequestId       string  `json:"request_id"`
	RequestParams   string  `json:"request_params"`
	RequestIP       string  `json:"request_ip"`
	Status          string  `json:"status"`
	ResultSummary   string  `json:"result_summary"`
	ErrorCode       string  `json:"error_code"`
	ErrorMessage    string  `json:"error_message"`
	Metadata        string  `json:"metadata"`
	DurationMS      int     `json:"duration_ms"`
	ResultSize      int     `json:"result_size"`
	BridgeSessionId string  `json:"bridge_session_id"`
	TargetClient    string  `json:"target_client"`
	Cost            float64 `json:"cost"`
	Quota           int     `json:"quota"`
	FreeUsed        bool    `json:"free_used"`
	SettledAt       int64   `json:"settled_at"`
	CreatedAt       int64   `json:"created_at"`
}

type MCPSummaryToolStats struct {
	Total    int64 `json:"total"`
	Enabled  int64 `json:"enabled"`
	Disabled int64 `json:"disabled"`
	Remote   int64 `json:"remote"`
}

type MCPSummaryBridgeStats struct {
	TotalClients   int64 `json:"total_clients"`
	OnlineClients  int64 `json:"online_clients"`
	OfflineClients int64 `json:"offline_clients"`
	ActiveClients  int64 `json:"active_clients"`
	OnlineSessions int64 `json:"online_sessions"`
}

type MCPSummaryCallStats struct {
	TotalCalls    int64   `json:"total_calls"`
	SuccessCalls  int64   `json:"success_calls"`
	ErrorCalls    int64   `json:"error_calls"`
	TimeoutCalls  int64   `json:"timeout_calls"`
	PendingCalls  int64   `json:"pending_calls"`
	SettledCalls  int64   `json:"settled_calls"`
	Unsettled     int64   `json:"unsettled"`
	FreeCalls     int64   `json:"free_calls"`
	Quota         int64   `json:"quota"`
	Cost          float64 `json:"cost"`
	ResultSize    int64   `json:"result_size"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	SuccessRate   float64 `json:"success_rate"`
}

type MCPSummaryAuditStats struct {
	TotalRequests int64   `json:"total_requests"`
	Success       int64   `json:"success"`
	Error         int64   `json:"error"`
	Timeout       int64   `json:"timeout"`
	Pending       int64   `json:"pending"`
	ResultSize    int64   `json:"result_size"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	SuccessRate   float64 `json:"success_rate"`
}

type MCPSummaryTopTool struct {
	ToolName      string  `json:"tool_name"`
	Calls         int64   `json:"calls"`
	SuccessCalls  int64   `json:"success_calls"`
	Quota         int64   `json:"quota"`
	Cost          float64 `json:"cost"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	SuccessRate   float64 `json:"success_rate"`
}

type MCPSummaryRecentError struct {
	Source       string `json:"source"`
	RequestId    string `json:"request_id"`
	ToolName     string `json:"tool_name"`
	ClientId     string `json:"client_id,omitempty"`
	SessionId    string `json:"session_id,omitempty"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    int64  `json:"created_at"`
}

type MCPSummaryBridgeTrendBucket struct {
	BucketStart     int64 `json:"bucket_start"`
	OnlineClients   int64 `json:"online_clients"`
	StartedSessions int64 `json:"started_sessions"`
	ClosedSessions  int64 `json:"closed_sessions"`
}

type MCPSummaryOpenAPIStorageBucket struct {
	BucketStart   int64 `json:"bucket_start"`
	ObjectCount   int64 `json:"object_count"`
	TotalBytes    int64 `json:"total_bytes"`
	ExpiredCount  int64 `json:"expired_count"`
	DownloadCount int64 `json:"download_count"`
}

type MCPSummaryProxyErrorTool struct {
	ProxyServerId      int     `json:"proxy_server_id"`
	ProxyToolId        int64   `json:"proxy_tool_id"`
	ToolId             int     `json:"tool_id"`
	ToolName           string  `json:"tool_name"`
	DownstreamToolName string  `json:"downstream_tool_name"`
	TotalCalls         int64   `json:"total_calls"`
	SuccessCalls       int64   `json:"success_calls"`
	ErrorCalls         int64   `json:"error_calls"`
	TimeoutCalls       int64   `json:"timeout_calls"`
	SuccessRate        float64 `json:"success_rate"`
	AvgDurationMS      float64 `json:"avg_duration_ms"`
}

type MCPSummaryBillingAnomalies struct {
	UnsettledSuccessCalls int64 `json:"unsettled_success_calls"`
	FailedChargedCalls    int64 `json:"failed_charged_calls"`
	MissingDebitEvents    int64 `json:"missing_debit_events"`
	RefundEvents          int64 `json:"refund_events"`
	RefundQuota           int64 `json:"refund_quota"`
	NetMCPQuotaDelta      int64 `json:"net_mcp_quota_delta"`
}

type MCPSummaryOperationsTrends struct {
	StartTime        int64                            `json:"start_time"`
	EndTime          int64                            `json:"end_time"`
	BucketSeconds    int64                            `json:"bucket_seconds"`
	CheckedAt        int64                            `json:"checked_at"`
	BridgeOnline     []MCPSummaryBridgeTrendBucket    `json:"bridge_online"`
	OpenAPIStorage   []MCPSummaryOpenAPIStorageBucket `json:"openapi_storage"`
	ProxyErrorTopN   []MCPSummaryProxyErrorTool       `json:"proxy_error_top_n"`
	BillingAnomalies MCPSummaryBillingAnomalies       `json:"billing_anomalies"`
}

type MCPSummary struct {
	WindowSeconds    int64                       `json:"window_seconds"`
	GeneratedAt      int64                       `json:"generated_at"`
	Tools            MCPSummaryToolStats         `json:"tools"`
	Bridge           MCPSummaryBridgeStats       `json:"bridge"`
	Calls            MCPSummaryCallStats         `json:"calls"`
	Audit            MCPSummaryAuditStats        `json:"audit"`
	TopTools         []MCPSummaryTopTool         `json:"top_tools"`
	RecentErrors     []MCPSummaryRecentError     `json:"recent_errors"`
	OperationsTrends *MCPSummaryOperationsTrends `json:"operations_trends,omitempty"`
	ReviewQueue      *MCPReviewQueue             `json:"review_queue,omitempty"`
}

// MCPReviewItem is a single actionable operations item surfaced in the MCP
// dashboard review queue. It aggregates review signals from MCP proxy servers,
// bridge clients, background tasks, and high-error tools into a uniform shape.
type MCPReviewItem struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	TargetType string   `json:"target_type"`
	TargetId   string   `json:"target_id"`
	TargetName string   `json:"target_name"`
	Reasons    []string `json:"reasons"`
	Detail     string   `json:"detail"`
	CreatedAt  int64    `json:"created_at,omitempty"`
}

type MCPReviewQueue struct {
	Total         int                 `json:"total"`
	CriticalCount int                 `json:"critical_count"`
	WarningCount  int                 `json:"warning_count"`
	VisibleCount  int                 `json:"visible_count"`
	MaxItems      int                 `json:"max_items"`
	Truncated     bool                `json:"truncated"`
	ScanLimits    MCPReviewScanLimits `json:"scan_limits"`
	Items         []MCPReviewItem     `json:"items"`
}

type MCPReviewScanLimits struct {
	ProxyServers  MCPReviewScanScope `json:"proxy_servers"`
	BridgeClients MCPReviewScanScope `json:"bridge_clients"`
	Tools         MCPReviewScanScope `json:"tools"`
}

type MCPReviewScanScope struct {
	Scanned int  `json:"scanned"`
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Capped  bool `json:"capped"`
}
