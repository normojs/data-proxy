package dto

type MCPOpenAPIPreviewRequest struct {
	OpenAPIUrl string `json:"openapi_url"`
	Document   any    `json:"document,omitempty"`
	Namespace  string `json:"namespace"`
	Category   string `json:"category"`
}

type MCPOpenAPIImportRequest struct {
	OpenAPIUrl         string   `json:"openapi_url"`
	Document           any      `json:"document,omitempty"`
	Namespace          string   `json:"namespace"`
	Category           string   `json:"category"`
	SelectedOperations []string `json:"selected_operations"`
	UpdateExisting     bool     `json:"update_existing"`
	AuthType           string   `json:"auth_type"`
	AuthRef            string   `json:"auth_ref"`
	AuthHeaderName     string   `json:"auth_header_name"`
	PricePerCall       *float64 `json:"price_per_call,omitempty"`
	PriceUnit          string   `json:"price_unit"`
	FreeQuota          *int     `json:"free_quota,omitempty"`
	Status             *int     `json:"status,omitempty"`
	SortOrder          *int     `json:"sort_order,omitempty"`
}

type MCPOpenAPILifecycleRequest struct {
	OpenAPIUrl string `json:"openapi_url"`
	ToolIds    []int  `json:"tool_ids,omitempty"`
}

type MCPOpenAPIBinaryCleanupRequest struct {
	TTLSeconds int64 `json:"ttl_seconds,omitempty"`
	Limit      int   `json:"limit,omitempty"`
	DryRun     bool  `json:"dry_run,omitempty"`
}

type MCPOpenAPIBinaryObjectListItem struct {
	Id               int64  `json:"id"`
	ObjectId         string `json:"object_id"`
	Provider         string `json:"provider"`
	ContentType      string `json:"content_type"`
	ContentFamily    string `json:"content_family"`
	SHA256           string `json:"sha256"`
	Size             int    `json:"size"`
	Filename         string `json:"filename"`
	Disposition      string `json:"disposition"`
	MCPToolCallId    int64  `json:"mcp_tool_call_id"`
	MCPToolId        int    `json:"mcp_tool_id"`
	OpenAPIToolId    int64  `json:"openapi_tool_id"`
	UserId           int    `json:"user_id"`
	TokenId          int    `json:"token_id"`
	RequestId        string `json:"request_id"`
	OperationKey     string `json:"operation_key"`
	ExpiresAt        int64  `json:"expires_at"`
	ExpiryStatus     string `json:"expiry_status"`
	DownloadCount    int    `json:"download_count"`
	LastDownloadedAt int64  `json:"last_downloaded_at"`
	DownloadURL      string `json:"download_url"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type MCPOpenAPIBinaryObjectSummary struct {
	TotalCount          int64 `json:"total_count"`
	TotalBytes          int64 `json:"total_bytes"`
	ActiveCount         int64 `json:"active_count"`
	ExpiredCount        int64 `json:"expired_count"`
	NoExpiryCount       int64 `json:"no_expiry_count"`
	DownloadedCount     int64 `json:"downloaded_count"`
	DownloadCount       int64 `json:"download_count"`
	DefaultTTLSeconds   int64 `json:"default_ttl_seconds"`
	DefaultCleanupLimit int   `json:"default_cleanup_limit"`
	CheckedAt           int64 `json:"checked_at"`
}

type MCPOpenAPIPreviewOperation struct {
	Key            string         `json:"key"`
	OperationId    string         `json:"operation_id"`
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	Summary        string         `json:"summary"`
	Description    string         `json:"description"`
	ToolName       string         `json:"tool_name"`
	InputSchema    map[string]any `json:"input_schema"`
	HasRequestBody bool           `json:"has_request_body"`
}

type MCPOpenAPISchemaMetrics struct {
	OperationCount    int `json:"operation_count"`
	ImportedToolCount int `json:"imported_tool_count"`
	SchemaCount       int `json:"schema_count"`
	UniqueSchemaCount int `json:"unique_schema_count"`
	ReusedSchemaCount int `json:"reused_schema_count"`
}

type MCPOpenAPIPreviewResponse struct {
	OpenAPIUrl    string                       `json:"openapi_url"`
	Title         string                       `json:"title"`
	Version       string                       `json:"version"`
	ServerURL     string                       `json:"server_url"`
	Namespace     string                       `json:"namespace"`
	Category      string                       `json:"category"`
	SchemaMetrics MCPOpenAPISchemaMetrics      `json:"schema_metrics"`
	Operations    []MCPOpenAPIPreviewOperation `json:"operations"`
}

type MCPOpenAPIImportItem struct {
	OperationKey string                   `json:"operation_key"`
	Tool         MCPToolAdminItem         `json:"tool"`
	Changes      []MCPOpenAPIImportChange `json:"changes,omitempty"`
}

type MCPOpenAPIImportChange struct {
	Field    string `json:"field"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
}

type MCPOpenAPIImportResponse struct {
	OpenAPIUrl    string                 `json:"openapi_url"`
	ImportedCount int                    `json:"imported_count"`
	UpdatedCount  int                    `json:"updated_count"`
	SkippedCount  int                    `json:"skipped_count"`
	Imported      []MCPOpenAPIImportItem `json:"imported"`
	Updated       []MCPOpenAPIImportItem `json:"updated"`
	Skipped       []string               `json:"skipped"`
}

type MCPOpenAPILifecycleResponse struct {
	OpenAPIUrl    string             `json:"openapi_url"`
	AffectedCount int                `json:"affected_count"`
	Tools         []MCPToolAdminItem `json:"tools"`
}

type MCPOpenAPIBinaryCleanupResponse struct {
	Provider         string   `json:"provider"`
	TTLSeconds       int64    `json:"ttl_seconds"`
	CutoffTime       int64    `json:"cutoff_time"`
	DryRun           bool     `json:"dry_run"`
	Scanned          int      `json:"scanned"`
	Deleted          int      `json:"deleted"`
	DeletedBytes     int64    `json:"deleted_bytes"`
	DeletedObjectIds []string `json:"deleted_object_ids,omitempty"`
	RegistryDeleted  int64    `json:"registry_deleted"`
	Errors           []string `json:"errors,omitempty"`
}
