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

type MCPOpenAPIPreviewResponse struct {
	OpenAPIUrl string                       `json:"openapi_url"`
	Title      string                       `json:"title"`
	Version    string                       `json:"version"`
	ServerURL  string                       `json:"server_url"`
	Namespace  string                       `json:"namespace"`
	Category   string                       `json:"category"`
	Operations []MCPOpenAPIPreviewOperation `json:"operations"`
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
	Provider     string   `json:"provider"`
	TTLSeconds   int64    `json:"ttl_seconds"`
	CutoffTime   int64    `json:"cutoff_time"`
	DryRun       bool     `json:"dry_run"`
	Scanned      int      `json:"scanned"`
	Deleted      int      `json:"deleted"`
	DeletedBytes int64    `json:"deleted_bytes"`
	Errors       []string `json:"errors,omitempty"`
}
