package mcpgateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	AuditActionInitialize    = "initialize"
	AuditActionToolsList     = "tools_list"
	AuditActionToolCall      = "tool_call"
	AuditActionResourcesRead = "resources_read"
	AuditActionPromptsGet    = "prompts_get"
	AuditActionPolicyDeny    = "policy_deny"
)

type AuditEvent struct {
	Action         string          `json:"action"`
	Decision       string          `json:"decision,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	RequestId      string          `json:"request_id,omitempty"`
	UserId         int             `json:"user_id,omitempty"`
	AppId          int64           `json:"app_id,omitempty"`
	BridgeClientId string          `json:"bridge_client_id,omitempty"`
	SessionId      string          `json:"session_id,omitempty"`
	Method         string          `json:"method,omitempty"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolSchemaHash string          `json:"tool_schema_hash,omitempty"`
	ToolCategory   string          `json:"tool_category,omitempty"`
	ArgumentHash   string          `json:"argument_hash,omitempty"`
	ResultHash     string          `json:"result_hash,omitempty"`
	BytesIn        int64           `json:"bytes_in,omitempty"`
	BytesOut       int64           `json:"bytes_out,omitempty"`
	DurationMS     int             `json:"duration_ms,omitempty"`
	ErrorCode      string          `json:"error_code,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

func NewToolCallAuditEvent(req ToolCallRequest, decision Decision) AuditEvent {
	args := NormalizeRawJSON(req.Arguments)
	return AuditEvent{
		Action:         AuditActionToolCall,
		Decision:       decision.Decision,
		Reason:         decision.Reason,
		RequestId:      req.Subject.RequestId,
		UserId:         req.Subject.UserId,
		AppId:          req.Subject.AppId,
		BridgeClientId: req.Subject.BridgeClientId,
		SessionId:      req.Subject.SessionId,
		Method:         MethodToolsCall,
		ToolName:       strings.TrimSpace(req.Tool.Name),
		ToolSchemaHash: HashToolSchema(req.Tool),
		ToolCategory:   decision.Category,
		ArgumentHash:   HashRawJSON(args),
		BytesIn:        int64(len(args)),
	}
}

func HashRawJSON(body json.RawMessage) string {
	if len(body) == 0 {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
