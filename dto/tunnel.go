package dto

type TunnelAppCreateRequest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	AppType        string         `json:"app_type"`
	PermissionMode string         `json:"permission_mode"`
	BridgeClientId string         `json:"bridge_client_id"`
	TargetHost     string         `json:"target_host"`
	TargetPort     int            `json:"target_port"`
	TargetPath     string         `json:"target_path"`
	Policy         map[string]any `json:"policy"`
	Route          map[string]any `json:"route"`
	Billing        map[string]any `json:"billing"`
}

type TunnelAppAdminUpdateRequest struct {
	Name           *string        `json:"name,omitempty"`
	Description    *string        `json:"description,omitempty"`
	PermissionMode *string        `json:"permission_mode,omitempty"`
	Status         *string        `json:"status,omitempty"`
	BridgeClientId *string        `json:"bridge_client_id,omitempty"`
	TargetHost     *string        `json:"target_host,omitempty"`
	TargetPort     *int           `json:"target_port,omitempty"`
	TargetPath     *string        `json:"target_path,omitempty"`
	Policy         map[string]any `json:"policy,omitempty"`
	Route          map[string]any `json:"route,omitempty"`
	Billing        map[string]any `json:"billing,omitempty"`
	ReviewNote     *string        `json:"review_note,omitempty"`
}

type TunnelAppItem struct {
	Id             int64          `json:"id"`
	UserId         int            `json:"user_id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	AppType        string         `json:"app_type"`
	PermissionMode string         `json:"permission_mode"`
	Status         string         `json:"status"`
	PublicSlug     string         `json:"public_slug"`
	BridgeClientId string         `json:"bridge_client_id"`
	TargetHost     string         `json:"target_host"`
	TargetPort     int            `json:"target_port"`
	TargetPath     string         `json:"target_path"`
	Policy         map[string]any `json:"policy,omitempty"`
	Route          map[string]any `json:"route,omitempty"`
	Billing        map[string]any `json:"billing,omitempty"`
	ApprovedBy     int            `json:"approved_by"`
	ApprovedAt     int64          `json:"approved_at"`
	ReviewNote     string         `json:"review_note"`
	LastError      string         `json:"last_error"`
	LastSeenAt     int64          `json:"last_seen_at"`
	CreatedAt      int64          `json:"created_at"`
	UpdatedAt      int64          `json:"updated_at"`
}

type TunnelConnectionCreateRequest struct {
	Name           string         `json:"name"`
	PermissionMode string         `json:"permission_mode"`
	ExpiresAt      int64          `json:"expires_at"`
	Config         map[string]any `json:"config"`
}

type TunnelConnectionUpdateRequest struct {
	Name      *string        `json:"name,omitempty"`
	ExpiresAt *int64         `json:"expires_at,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type TunnelConnectionItem struct {
	Id             int64          `json:"id"`
	AppId          int64          `json:"app_id"`
	UserId         int            `json:"user_id"`
	AgentTokenId   int            `json:"agent_token_id"`
	Name           string         `json:"name"`
	KeyPrefix      string         `json:"key_prefix"`
	PermissionMode string         `json:"permission_mode"`
	Status         string         `json:"status"`
	EndpointPath   string         `json:"endpoint_path"`
	Config         map[string]any `json:"config,omitempty"`
	ExpiresAt      int64          `json:"expires_at"`
	LastUsedAt     int64          `json:"last_used_at"`
	LastRequestId  string         `json:"last_request_id"`
	RevokedAt      int64          `json:"revoked_at"`
	CreatedAt      int64          `json:"created_at"`
	UpdatedAt      int64          `json:"updated_at"`
}

type TunnelSessionItem struct {
	Id             int64  `json:"id"`
	AppId          int64  `json:"app_id"`
	UserId         int    `json:"user_id"`
	ConnectionId   int64  `json:"connection_id"`
	ConnectionName string `json:"connection_name"`
	KeyPrefix      string `json:"key_prefix"`
	SessionId      string `json:"session_id"`
	BridgeClientId string `json:"bridge_client_id"`
	Status         string `json:"status"`
	ClientVersion  string `json:"client_version"`
	ClientIp       string `json:"client_ip"`
	UserAgent      string `json:"user_agent"`
	BytesIn        int64  `json:"bytes_in"`
	BytesOut       int64  `json:"bytes_out"`
	ConnectedAt    int64  `json:"connected_at"`
	LastSeenAt     int64  `json:"last_seen_at"`
	DisconnectedAt int64  `json:"disconnected_at"`
	CloseReason    string `json:"close_reason"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type TunnelConnectionCreateResponse struct {
	Connection    TunnelConnectionItem `json:"connection"`
	ConnectionKey string               `json:"connection_key"`
	EndpointPath  string               `json:"endpoint_path"`
}

type TunnelAgentSetupRequest struct {
	ConnectionId int64  `json:"connection_id"`
	Rotate       bool   `json:"rotate"`
	ClientName   string `json:"client_name"`
	Platform     string `json:"platform"`
	Workspace    string `json:"workspace"`
	Version      string `json:"version"`
}

type TunnelAgentSetupResponse struct {
	App            TunnelAppItem        `json:"app"`
	Connection     TunnelConnectionItem `json:"connection"`
	BaseURL        string               `json:"base_url"`
	BridgeWSURL    string               `json:"bridge_ws_url"`
	MCPURL         string               `json:"mcp_url"`
	ClientId       string               `json:"client_id"`
	APIKey         string               `json:"api_key,omitempty"`
	APIKeyOnce     bool                 `json:"api_key_once"`
	TokenId        int                  `json:"token_id"`
	TokenName      string               `json:"token_name"`
	TokenMaskedKey string               `json:"token_masked_key"`
	Created        bool                 `json:"created"`
	Rotated        bool                 `json:"rotated"`
	Headers        map[string]string    `json:"headers"`
	Register       map[string]any       `json:"register"`
	Environment    map[string]string    `json:"environment"`
	Config         map[string]any       `json:"config"`
}

type TunnelAuditLogItem struct {
	Id                  int64          `json:"id"`
	AppId               int64          `json:"app_id"`
	ConnectionId        int64          `json:"connection_id"`
	ConnectionKeyPrefix string         `json:"connection_key_prefix"`
	SessionId           string         `json:"session_id"`
	UserId              int            `json:"user_id"`
	ActorUserId         int            `json:"actor_user_id"`
	Action              string         `json:"action"`
	Decision            string         `json:"decision"`
	Reason              string         `json:"reason"`
	RequestId           string         `json:"request_id"`
	ToolName            string         `json:"tool_name"`
	Method              string         `json:"method"`
	Path                string         `json:"path"`
	BytesIn             int64          `json:"bytes_in"`
	BytesOut            int64          `json:"bytes_out"`
	DurationMS          int            `json:"duration_ms"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	CreatedAt           int64          `json:"created_at"`
}
