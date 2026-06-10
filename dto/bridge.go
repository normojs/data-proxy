package dto

type BridgeClientRegisterRequest struct {
	ClientId     string   `json:"client_id"`
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version,omitempty"`
	Platform     string   `json:"platform,omitempty"`
	Workspace    string   `json:"workspace,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type BridgeWSMessage struct {
	Type string `json:"type"`
	Id   string `json:"id,omitempty"`
	Data any    `json:"data,omitempty"`
}

type BridgeToolCallRequest struct {
	RequestId string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type BridgeToolCallResult struct {
	Content    []MCPContentBlock `json:"content,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	Summary    string            `json:"summary,omitempty"`
	DurationMS int               `json:"duration_ms,omitempty"`
	ResultSize int               `json:"result_size,omitempty"`
}

type BridgeToolCallError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type BridgeClientUpdateRequest struct {
	Name         *string   `json:"name,omitempty"`
	Version      *string   `json:"version,omitempty"`
	Platform     *string   `json:"platform,omitempty"`
	Workspace    *string   `json:"workspace,omitempty"`
	Capabilities *[]string `json:"capabilities,omitempty"`
	Status       *int      `json:"status,omitempty"`
}

type BridgeSessionCloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

type BridgeClientItem struct {
	Id           int      `json:"id"`
	ClientId     string   `json:"client_id"`
	UserId       int      `json:"user_id"`
	TokenId      int      `json:"token_id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Platform     string   `json:"platform"`
	Workspace    string   `json:"workspace"`
	Capabilities []string `json:"capabilities"`
	Status       int      `json:"status"`
	Online       bool     `json:"online"`
	SessionId    string   `json:"session_id,omitempty"`
	LastSeenAt   int64    `json:"last_seen_at"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

type BridgeClientDetail struct {
	Client         BridgeClientItem       `json:"client"`
	OnlineSession  *BridgeSessionSnapshot `json:"online_session,omitempty"`
	RecentSessions []BridgeSessionItem    `json:"recent_sessions"`
}

type BridgeSessionSnapshot struct {
	SessionId    string   `json:"session_id"`
	ClientId     string   `json:"client_id"`
	UserId       int      `json:"user_id"`
	TokenId      int      `json:"token_id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Platform     string   `json:"platform"`
	Workspace    string   `json:"workspace"`
	Capabilities []string `json:"capabilities"`
	ConnectedAt  int64    `json:"connected_at"`
	LastSeenAt   int64    `json:"last_seen_at"`
}

type BridgeSessionItem struct {
	Id          int64  `json:"id"`
	SessionId   string `json:"session_id"`
	ClientId    string `json:"client_id"`
	UserId      int    `json:"user_id"`
	TokenId     int    `json:"token_id"`
	RequestIP   string `json:"request_ip"`
	UserAgent   string `json:"user_agent"`
	Status      string `json:"status"`
	ConnectedAt int64  `json:"connected_at"`
	LastPingAt  int64  `json:"last_ping_at"`
	ClosedAt    int64  `json:"closed_at"`
	CloseReason string `json:"close_reason"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type BridgeClientHealth struct {
	ClientId       string                 `json:"client_id"`
	WindowSeconds  int64                  `json:"window_seconds"`
	GeneratedAt    int64                  `json:"generated_at"`
	Online         bool                   `json:"online"`
	OnlineSession  *BridgeSessionSnapshot `json:"online_session,omitempty"`
	Calls          BridgeAuditHealth      `json:"calls"`
	RecentErrors   []BridgeRecentError    `json:"recent_errors"`
	RecentSessions []BridgeSessionItem    `json:"recent_sessions"`
}

type BridgeAuditHealth struct {
	TotalRequests int64   `json:"total_requests"`
	Success       int64   `json:"success"`
	Error         int64   `json:"error"`
	Timeout       int64   `json:"timeout"`
	Pending       int64   `json:"pending"`
	ResultSize    int64   `json:"result_size"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
	SuccessRate   float64 `json:"success_rate"`
}

type BridgeRecentError struct {
	Id           int64  `json:"id"`
	RequestId    string `json:"request_id"`
	SessionId    string `json:"session_id"`
	ClientId     string `json:"client_id"`
	ToolName     string `json:"tool_name"`
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	DurationMS   int    `json:"duration_ms"`
	CreatedAt    int64  `json:"created_at"`
}

type BridgeAuditLogItem struct {
	Id           int64  `json:"id"`
	RequestId    string `json:"request_id"`
	SessionId    string `json:"session_id"`
	ClientId     string `json:"client_id"`
	UserId       int    `json:"user_id"`
	TokenId      int    `json:"token_id"`
	ToolName     string `json:"tool_name"`
	RequestBody  string `json:"request_body"`
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	DurationMS   int    `json:"duration_ms"`
	ResultSize   int    `json:"result_size"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}
