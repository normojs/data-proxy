package mcpgateway

import "encoding/json"

const (
	MethodInitialize             = "initialize"
	MethodToolsList              = "tools/list"
	MethodToolsCall              = "tools/call"
	MethodResourcesList          = "resources/list"
	MethodResourcesRead          = "resources/read"
	MethodResourcesTemplatesList = "resources/templates/list"
	MethodPromptsList            = "prompts/list"
	MethodPromptsGet             = "prompts/get"
)

const (
	PermissionReadOnly    = "read_only"
	PermissionWrite       = "write"
	PermissionExecSafe    = "exec_safe"
	PermissionExecTrusted = "exec_trusted"
)

const (
	ToolCategoryRead     = "read"
	ToolCategoryWrite    = "write"
	ToolCategoryExec     = "exec"
	ToolCategoryNetwork  = "network"
	ToolCategoryBrowser  = "browser"
	ToolCategoryComputer = "computer"
	ToolCategoryUnknown  = "unknown"
)

const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Annotations map[string]any `json:"annotations,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ToolSnapshot struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	SchemaHash  string   `json:"schema_hash"`
	Categories  []string `json:"categories"`
	RiskLevel   string   `json:"risk_level"`
}

type Subject struct {
	UserId         int    `json:"user_id,omitempty"`
	TokenId        int    `json:"token_id,omitempty"`
	AppId          int64  `json:"app_id,omitempty"`
	BridgeClientId string `json:"bridge_client_id,omitempty"`
	SessionId      string `json:"session_id,omitempty"`
	RequestId      string `json:"request_id,omitempty"`
	ClientName     string `json:"client_name,omitempty"`
}

type ToolCallRequest struct {
	Subject   Subject        `json:"subject"`
	Tool      Tool           `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type Decision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Category string `json:"category,omitempty"`
}

func (d Decision) Allowed() bool {
	return d.Decision == DecisionAllow
}

func NormalizeRawJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return body
}
