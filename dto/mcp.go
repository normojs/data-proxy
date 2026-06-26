package dto

import "encoding/json"

const MCPJSONRPCVersion = "2.0"
const MCPProtocolVersion = "2025-06-18"

const (
	MCPMethodInitialize             = "initialize"
	MCPMethodInitialized            = "notifications/initialized"
	MCPMethodPing                   = "ping"
	MCPMethodToolsList              = "tools/list"
	MCPMethodToolsCall              = "tools/call"
	MCPMethodResourcesList          = "resources/list"
	MCPMethodResourcesRead          = "resources/read"
	MCPMethodResourcesTemplatesList = "resources/templates/list"
	MCPMethodPromptsList            = "prompts/list"
	MCPMethodPromptsGet             = "prompts/get"
)

const (
	MCPErrorCodeParseError        = -32700
	MCPErrorCodeInvalidRequest    = -32600
	MCPErrorCodeMethodNotFound    = -32601
	MCPErrorCodeInvalidParams     = -32602
	MCPErrorCodeInternalError     = -32603
	MCPErrorCodeUnknownTool       = -32000
	MCPErrorCodeNotImplemented    = -32001
	MCPErrorCodeBillingFailed     = -32002
	MCPErrorCodeExecutorFailed    = -32003
	MCPErrorCodeExecutorTimeout   = -32004
	MCPErrorCodeBridgeUnavailable = -32005
	MCPErrorCodeDuplicateRequest  = -32006
)

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type MCPImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]any         `json:"capabilities,omitempty"`
	ClientInfo      *MCPImplementationInfo `json:"clientInfo,omitempty"`
}

type MCPInitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]any         `json:"capabilities"`
	ServerInfo      *MCPImplementationInfo `json:"serverInfo"`
}

type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

type MCPToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type MCPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type MCPToolCallResult struct {
	Content  []MCPContentBlock `json:"content"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

type MCPToolCallErrorData struct {
	ToolName string `json:"tool_name,omitempty"`
	CallId   int64  `json:"call_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Quota    int    `json:"quota,omitempty"`
	Cost     any    `json:"cost,omitempty"`
}
