package dto

type MCPProxyServerCreateRequest struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	Transport       string   `json:"transport"`
	Endpoint        string   `json:"endpoint"`
	Command         string   `json:"command"`
	AuthType        string   `json:"auth_type"`
	AuthRef         string   `json:"auth_ref"`
	TimeoutMS       int      `json:"timeout_ms"`
	MaxResultSize   int      `json:"max_result_size"`
	MaxMetadataSize int      `json:"max_metadata_size"`
	Visibility      string   `json:"visibility"`
	AllowedGroups   []string `json:"allowed_groups"`
	Status          string   `json:"status"`
}

type MCPProxyServerUpdateRequest struct {
	Name            *string   `json:"name,omitempty"`
	Namespace       *string   `json:"namespace,omitempty"`
	Transport       *string   `json:"transport,omitempty"`
	Endpoint        *string   `json:"endpoint,omitempty"`
	Command         *string   `json:"command,omitempty"`
	AuthType        *string   `json:"auth_type,omitempty"`
	AuthRef         *string   `json:"auth_ref,omitempty"`
	TimeoutMS       *int      `json:"timeout_ms,omitempty"`
	MaxResultSize   *int      `json:"max_result_size,omitempty"`
	MaxMetadataSize *int      `json:"max_metadata_size,omitempty"`
	Visibility      *string   `json:"visibility,omitempty"`
	AllowedGroups   *[]string `json:"allowed_groups,omitempty"`
	Status          *string   `json:"status,omitempty"`
}

type MCPProxyServerAdminItem struct {
	Id               int                       `json:"id"`
	Name             string                    `json:"name"`
	Namespace        string                    `json:"namespace"`
	Transport        string                    `json:"transport"`
	Endpoint         string                    `json:"endpoint"`
	Command          string                    `json:"command,omitempty"`
	AuthType         string                    `json:"auth_type"`
	AuthRef          string                    `json:"auth_ref,omitempty"`
	TimeoutMS        int                       `json:"timeout_ms"`
	MaxResultSize    int                       `json:"max_result_size"`
	MaxMetadataSize  int                       `json:"max_metadata_size"`
	Visibility       string                    `json:"visibility"`
	AllowedGroups    []string                  `json:"allowed_groups"`
	Status           string                    `json:"status"`
	LastError        string                    `json:"last_error"`
	LastDiscoveredAt int64                     `json:"last_discovered_at"`
	CreatedAt        int64                     `json:"created_at"`
	UpdatedAt        int64                     `json:"updated_at"`
	Archived         bool                      `json:"archived"`
	Health           *MCPProxyServerListHealth `json:"health,omitempty"`
}

type MCPProxyServerTestResult struct {
	ProxyServerId   int            `json:"proxy_server_id"`
	ProtocolVersion string         `json:"protocol_version"`
	ServerName      string         `json:"server_name"`
	Capabilities    map[string]any `json:"capabilities"`
}

type MCPProxyDiscoveryResult struct {
	ProxyServerId   int                     `json:"proxy_server_id"`
	DiscoveredCount int                     `json:"discovered_count"`
	CreatedCount    int                     `json:"created_count"`
	UpdatedCount    int                     `json:"updated_count"`
	DisabledCount   int                     `json:"disabled_count"`
	SchemaChanged   int                     `json:"schema_changed"`
	Tools           []MCPProxyToolAdminItem `json:"tools"`
}

type MCPProxyDiscoveryEventItem struct {
	Id              int64          `json:"id"`
	ProxyServerId   int            `json:"proxy_server_id"`
	EventType       string         `json:"event_type"`
	Status          string         `json:"status"`
	Message         string         `json:"message"`
	ProtocolVersion string         `json:"protocol_version"`
	ServerName      string         `json:"server_name"`
	Capabilities    map[string]any `json:"capabilities"`
	DiscoveredCount int            `json:"discovered_count"`
	CreatedCount    int            `json:"created_count"`
	UpdatedCount    int            `json:"updated_count"`
	DisabledCount   int            `json:"disabled_count"`
	SchemaChanged   int            `json:"schema_changed"`
	DurationMS      int            `json:"duration_ms"`
	StartedAt       int64          `json:"started_at"`
	FinishedAt      int64          `json:"finished_at"`
	CreatedAt       int64          `json:"created_at"`
}

type MCPProxyServerHealth struct {
	ProxyServerId int                         `json:"proxy_server_id"`
	WindowSeconds int64                       `json:"window_seconds"`
	GeneratedAt   int64                       `json:"generated_at"`
	NeedsReview   bool                        `json:"needs_review"`
	ReviewReasons []string                    `json:"review_reasons"`
	Calls         MCPProxyServerCallHealth    `json:"calls"`
	Discovery     MCPProxyServerDiscoveryInfo `json:"discovery"`
	Transport     MCPProxyTransportHealth     `json:"transport"`
	TopTools      []MCPProxyServerToolHealth  `json:"top_tools"`
	RecentErrors  []MCPProxyServerRecentError `json:"recent_errors"`
	LatestCheck   *MCPProxyDiscoveryEventItem `json:"latest_check,omitempty"`
}

type MCPProxyServerListHealth struct {
	WindowSeconds int64                       `json:"window_seconds"`
	GeneratedAt   int64                       `json:"generated_at"`
	NeedsReview   bool                        `json:"needs_review"`
	ReviewReasons []string                    `json:"review_reasons"`
	Calls         MCPProxyServerCallHealth    `json:"calls"`
	Discovery     MCPProxyServerDiscoveryInfo `json:"discovery"`
	TopTool       *MCPProxyServerToolHealth   `json:"top_tool,omitempty"`
	LatestError   *MCPProxyServerRecentError  `json:"latest_error,omitempty"`
	LatestCheck   *MCPProxyDiscoveryEventItem `json:"latest_check,omitempty"`
}

type MCPProxyToolHealth struct {
	ProxyToolId   int64                       `json:"proxy_tool_id"`
	ProxyServerId int                         `json:"proxy_server_id"`
	MCPToolId     int                         `json:"mcp_tool_id"`
	WindowSeconds int64                       `json:"window_seconds"`
	GeneratedAt   int64                       `json:"generated_at"`
	Calls         MCPProxyServerCallHealth    `json:"calls"`
	Tool          MCPProxyServerToolHealth    `json:"tool"`
	RecentErrors  []MCPProxyServerRecentError `json:"recent_errors"`
}

type MCPProxyServerCallHealth struct {
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

type MCPProxyTrendResponse struct {
	StartTime     int64                          `json:"start_time"`
	EndTime       int64                          `json:"end_time"`
	BucketSeconds int64                          `json:"bucket_seconds"`
	CheckedAt     int64                          `json:"checked_at"`
	Totals        MCPProxyServerCallHealth       `json:"totals"`
	Buckets       []MCPProxyTrendBucket          `json:"buckets"`
	Servers       []MCPProxyTrendServerDimension `json:"servers"`
	Tools         []MCPProxyTrendToolDimension   `json:"tools"`
}

type MCPProxyTrendBucket struct {
	BucketStart int64 `json:"bucket_start"`
	MCPProxyServerCallHealth
}

type MCPProxyTrendServerDimension struct {
	ProxyServerId int    `json:"proxy_server_id"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	MCPProxyServerCallHealth
}

type MCPProxyTrendToolDimension struct {
	ProxyServerId      int    `json:"proxy_server_id"`
	ProxyToolId        int64  `json:"proxy_tool_id"`
	ToolId             int    `json:"tool_id"`
	ExposedToolName    string `json:"exposed_tool_name"`
	DownstreamToolName string `json:"downstream_tool_name"`
	MCPProxyServerCallHealth
}

type MCPProxyServerDiscoveryInfo struct {
	TotalTools         int   `json:"total_tools"`
	EnabledTools       int   `json:"enabled_tools"`
	PendingTools       int   `json:"pending_tools"`
	DisabledTools      int   `json:"disabled_tools"`
	SchemaChangedTools int   `json:"schema_changed_tools"`
	ErrorTools         int   `json:"error_tools"`
	LastDiscoveredAt   int64 `json:"last_discovered_at"`
	LastToolUpdatedAt  int64 `json:"last_tool_updated_at"`
}

type MCPProxyTransportHealth struct {
	Transport         string `json:"transport"`
	HasSession        bool   `json:"has_session"`
	Initialized       bool   `json:"initialized"`
	MessageEndpoint   string `json:"message_endpoint,omitempty"`
	LastError         string `json:"last_error,omitempty"`
	StreamableSession bool   `json:"streamable_session"`
	SSEConnected      bool   `json:"sse_connected"`
	ActiveSessions    int    `json:"active_sessions"`
	PendingRequests   int    `json:"pending_requests"`
	LastActivityAt    int64  `json:"last_activity_at,omitempty"`
	Observable        bool   `json:"observable"`
}

type MCPProxyServerToolHealth struct {
	ToolId             int     `json:"tool_id"`
	ProxyToolId        int64   `json:"proxy_tool_id"`
	ExposedToolName    string  `json:"exposed_tool_name"`
	DownstreamToolName string  `json:"downstream_tool_name"`
	Status             string  `json:"status"`
	Calls              int64   `json:"calls"`
	SuccessCalls       int64   `json:"success_calls"`
	ErrorCalls         int64   `json:"error_calls"`
	TimeoutCalls       int64   `json:"timeout_calls"`
	Quota              int64   `json:"quota"`
	Cost               float64 `json:"cost"`
	AvgDurationMS      float64 `json:"avg_duration_ms"`
	SuccessRate        float64 `json:"success_rate"`
}

type MCPProxyServerRecentError struct {
	Id           int64  `json:"id"`
	RequestId    string `json:"request_id"`
	ToolName     string `json:"tool_name"`
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	DurationMS   int    `json:"duration_ms"`
	CreatedAt    int64  `json:"created_at"`
}

type MCPProxyToolAdminItem struct {
	Id                    int64   `json:"id"`
	ProxyServerId         int     `json:"proxy_server_id"`
	MCPToolId             int     `json:"mcp_tool_id"`
	DownstreamToolName    string  `json:"downstream_tool_name"`
	DownstreamTitle       string  `json:"downstream_title"`
	DownstreamDescription string  `json:"downstream_description"`
	DownstreamInputSchema any     `json:"downstream_input_schema"`
	ExposedToolName       string  `json:"exposed_tool_name"`
	ExposedDescription    string  `json:"exposed_description"`
	SchemaHash            string  `json:"schema_hash"`
	Status                string  `json:"status"`
	PricePerCall          float64 `json:"price_per_call"`
	PriceUnit             string  `json:"price_unit"`
	FreeQuota             int     `json:"free_quota"`
	SortOrder             int     `json:"sort_order"`
	LastError             string  `json:"last_error"`
	LastDiscoveredAt      int64   `json:"last_discovered_at"`
	CreatedAt             int64   `json:"created_at"`
	UpdatedAt             int64   `json:"updated_at"`
}

type MCPProxyToolUpdateRequest struct {
	ExposedToolName    *string  `json:"exposed_tool_name,omitempty"`
	ExposedDescription *string  `json:"exposed_description,omitempty"`
	Status             *string  `json:"status,omitempty"`
	PricePerCall       *float64 `json:"price_per_call,omitempty"`
	PriceUnit          *string  `json:"price_unit,omitempty"`
	FreeQuota          *int     `json:"free_quota,omitempty"`
	SortOrder          *int     `json:"sort_order,omitempty"`
}

type MCPProxyHealthCheckSettings struct {
	Enabled          bool  `json:"enabled"`
	IntervalMinutes  int   `json:"interval_minutes"`
	Limit            int   `json:"limit"`
	StaleSeconds     int64 `json:"stale_seconds"`
	Discover         bool  `json:"discover"`
	MaxDiscoverTools int   `json:"max_discover_tools"`
}

type MCPProxyHealthCheckSettingsRequest struct {
	Enabled          bool  `json:"enabled"`
	IntervalMinutes  int   `json:"interval_minutes"`
	Limit            int   `json:"limit"`
	StaleSeconds     int64 `json:"stale_seconds"`
	Discover         bool  `json:"discover"`
	MaxDiscoverTools int   `json:"max_discover_tools"`
}

type MCPProxyHealthCheckRunRequest struct {
	DryRun           bool  `json:"dry_run"`
	Force            bool  `json:"force"`
	Discover         *bool `json:"discover,omitempty"`
	Limit            int   `json:"limit,omitempty"`
	StaleSeconds     int64 `json:"stale_seconds,omitempty"`
	MaxDiscoverTools int   `json:"max_discover_tools,omitempty"`
}

type MCPProxyHealthCheckStatusResponse struct {
	Settings       MCPProxyHealthCheckSettings     `json:"settings"`
	Running        bool                            `json:"running"`
	LastRunAt      int64                           `json:"last_run_at"`
	LastRunStatus  string                          `json:"last_run_status"`
	LastRunMessage string                          `json:"last_run_message"`
	LastRun        *MCPProxyHealthCheckRunResponse `json:"last_run,omitempty"`
}

type MCPProxyHealthCheckRunResponse struct {
	Manual          bool                               `json:"manual"`
	DryRun          bool                               `json:"dry_run"`
	Status          string                             `json:"status"`
	Message         string                             `json:"message"`
	Settings        MCPProxyHealthCheckSettings        `json:"settings"`
	CheckedAt       int64                              `json:"checked_at"`
	ScannedCount    int                                `json:"scanned_count"`
	CheckedCount    int                                `json:"checked_count"`
	SkippedCount    int                                `json:"skipped_count"`
	SuccessCount    int                                `json:"success_count"`
	ErrorCount      int                                `json:"error_count"`
	DiscoverCount   int                                `json:"discover_count"`
	BlockedCount    int                                `json:"blocked_count"`
	Items           []MCPProxyHealthCheckRunItem       `json:"items"`
	LatestEventById map[int]MCPProxyDiscoveryEventItem `json:"latest_event_by_id,omitempty"`
}

type MCPProxyHealthCheckRunItem struct {
	ProxyServerId int                         `json:"proxy_server_id"`
	Name          string                      `json:"name"`
	Namespace     string                      `json:"namespace"`
	Status        string                      `json:"status"`
	Action        string                      `json:"action"`
	SkippedReason string                      `json:"skipped_reason,omitempty"`
	PreviousCheck *MCPProxyDiscoveryEventItem `json:"previous_check,omitempty"`
	LatestCheck   *MCPProxyDiscoveryEventItem `json:"latest_check,omitempty"`
	Error         string                      `json:"error,omitempty"`
	Discovered    int                         `json:"discovered"`
	SchemaChanged int                         `json:"schema_changed"`
}

type MCPProxyHeartbeatSettings struct {
	Enabled             bool  `json:"enabled"`
	IntervalSeconds     int64 `json:"interval_seconds"`
	Limit               int   `json:"limit"`
	ActiveWindowSeconds int64 `json:"active_window_seconds"`
	TimeoutSeconds      int64 `json:"timeout_seconds"`
}

type MCPProxyHeartbeatSettingsRequest struct {
	Enabled             bool  `json:"enabled"`
	IntervalSeconds     int64 `json:"interval_seconds"`
	Limit               int   `json:"limit"`
	ActiveWindowSeconds int64 `json:"active_window_seconds"`
	TimeoutSeconds      int64 `json:"timeout_seconds"`
}

type MCPProxyHeartbeatStatusResponse struct {
	Settings       MCPProxyHeartbeatSettings     `json:"settings"`
	Running        bool                          `json:"running"`
	LastRunAt      int64                         `json:"last_run_at"`
	LastRunStatus  string                        `json:"last_run_status"`
	LastRunMessage string                        `json:"last_run_message"`
	LastRun        *MCPProxyHeartbeatRunResponse `json:"last_run,omitempty"`
}

type MCPProxyHeartbeatRunResponse struct {
	Manual       bool                       `json:"manual"`
	Status       string                     `json:"status"`
	Message      string                     `json:"message"`
	Settings     MCPProxyHeartbeatSettings  `json:"settings"`
	CheckedAt    int64                      `json:"checked_at"`
	ScannedCount int                        `json:"scanned_count"`
	PingedCount  int                        `json:"pinged_count"`
	SkippedCount int                        `json:"skipped_count"`
	SuccessCount int                        `json:"success_count"`
	ErrorCount   int                        `json:"error_count"`
	Items        []MCPProxyHeartbeatRunItem `json:"items"`
}

type MCPProxyHeartbeatRunItem struct {
	ProxyServerId  int    `json:"proxy_server_id"`
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Transport      string `json:"transport"`
	Action         string `json:"action"`
	SkippedReason  string `json:"skipped_reason,omitempty"`
	Error          string `json:"error,omitempty"`
	LastActivityAt int64  `json:"last_activity_at,omitempty"`
}
