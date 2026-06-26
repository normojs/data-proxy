package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
)

var defaultTunnelMCPProxyClient mcpproxy.Client = mcpproxy.NewDefaultClient(nil)

var ErrTunnelMCPPolicyDenied = errors.New("tunnel mcp policy denied")

type tunnelMCPRawPolicy struct {
	AllowedResourceURIPrefixes []string
	DeniedResourceURIPrefixes  []string
	AllowedPromptNames         []string
	DeniedPromptNames          []string
	AllowedPromptPrefixes      []string
	DeniedPromptPrefixes       []string
}

type TunnelMCPToolsListRequest struct {
	Context       context.Context
	UserId        int
	TokenId       int
	Slug          string
	ConnectionKey string
	RequestId     string
	SessionId     string
}

type TunnelMCPToolCallRequest struct {
	Context       context.Context
	UserId        int
	TokenId       int
	Slug          string
	ConnectionKey string
	RequestId     string
	SessionId     string
	Params        dto.MCPToolCallParams
}

type TunnelMCPRawRequest struct {
	Context       context.Context
	UserId        int
	TokenId       int
	Slug          string
	ConnectionKey string
	RequestId     string
	SessionId     string
	Method        string
	Params        json.RawMessage
}

func setTunnelMCPProxyClientForTest(client mcpproxy.Client) func() {
	previous := defaultTunnelMCPProxyClient
	if client == nil {
		client = mcpproxy.UnconfiguredClient{}
	}
	defaultTunnelMCPProxyClient = client
	return func() {
		defaultTunnelMCPProxyClient = previous
	}
}

func ListTunnelMCPTools(req TunnelMCPToolsListRequest) ([]dto.MCPTool, error) {
	app, connection, err := getAuthorizedTunnelMCPApp(req.Slug, req.ConnectionKey, req.UserId)
	if err != nil {
		return nil, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	if err := checkTunnelMCPBillingBalance(*app, *connection, req.SessionId, req.RequestId, mcpgateway.MethodToolsList, "", 0); err != nil {
		return nil, err
	}
	if err := checkTunnelMCPRequestRateLimit(*app, *connection, req.SessionId, req.RequestId, mcpgateway.MethodToolsList, "", 0); err != nil {
		return nil, err
	}
	startedAt := time.Now()
	tools, err := listTunnelMCPProxyTools(tunnelMCPContext(req.Context), *app)
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, mcpgateway.AuditActionToolsList, mcpgateway.DecisionDeny, req.RequestId, "", mcpgateway.MethodToolsList, 0, 0, durationMS, map[string]any{
			"error": err.Error(),
		})
		return nil, err
	}
	gatewayTools := tunnelMCPToolsFromProxyDefinitions(tools)
	policy := mcpgateway.PolicyForPermissionMode(effectiveTunnelMCPPermissionMode(*app, *connection))
	filtered := mcpgateway.FilterTools(policy, gatewayTools)
	snapshots := mcpgateway.BuildToolSnapshots(filtered)
	_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, mcpgateway.AuditActionToolsList, mcpgateway.DecisionAllow, req.RequestId, "", mcpgateway.MethodToolsList, 0, 0, durationMS, map[string]any{
		"connection_id":         connection.Id,
		"connection_key_prefix": connection.KeyPrefix,
		"discovered_count":      len(tools),
		"exposed_count":         len(filtered),
		"snapshots":             snapshots,
	})
	return tunnelMCPDTOFromGatewayTools(filtered), nil
}

func CallTunnelMCPTool(req TunnelMCPToolCallRequest) (*MCPToolCallResponse, error) {
	if strings.TrimSpace(req.Params.Name) == "" {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidParams,
			ErrorMessage: "Invalid params",
			ErrorData:    "tool name is required",
		}, nil
	}
	app, connection, err := getAuthorizedTunnelMCPApp(req.Slug, req.ConnectionKey, req.UserId)
	if err != nil {
		return nil, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	bytesIn := int64(len(mcpgateway.NormalizeRawJSON(req.Params.Arguments)))
	if err := checkTunnelMCPBillingBalance(*app, *connection, req.SessionId, req.RequestId, mcpgateway.MethodToolsCall, req.Params.Name, bytesIn); err != nil {
		if errors.Is(err, ErrTunnelBillingInsufficient) {
			return &MCPToolCallResponse{
				ErrorCode:    dto.MCPErrorCodeInvalidRequest,
				ErrorMessage: "MCP gateway billing insufficient balance",
				ErrorData: dto.MCPToolCallErrorData{
					ToolName: req.Params.Name,
					Reason:   tunnelBillingReasonInsufficient,
				},
			}, nil
		}
		return nil, err
	}
	if err := checkTunnelMCPRequestRateLimit(*app, *connection, req.SessionId, req.RequestId, mcpgateway.MethodToolsCall, req.Params.Name, bytesIn); err != nil {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidRequest,
			ErrorMessage: "MCP gateway rate limit exceeded",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: req.Params.Name,
				Reason:   err.Error(),
			},
		}, nil
	}
	ctx := tunnelMCPContext(req.Context)
	server := tunnelMCPProxyServer(*app)
	tools, err := listTunnelMCPProxyTools(ctx, *app)
	if err != nil {
		return nil, err
	}
	tool, found := tunnelMCPFindGatewayTool(tools, req.Params.Name)
	if !found {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeUnknownTool,
			ErrorMessage: "Unknown tool",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: req.Params.Name,
				Reason:   "tool is not exposed by user mcp",
			},
		}, nil
	}
	decision := mcpgateway.AuthorizeTool(mcpgateway.PolicyForPermissionMode(effectiveTunnelMCPPermissionMode(*app, *connection)), tool)
	auditEvent := mcpgateway.NewToolCallAuditEvent(mcpgateway.ToolCallRequest{
		Subject: mcpgateway.Subject{
			UserId:         req.UserId,
			TokenId:        req.TokenId,
			AppId:          app.Id,
			BridgeClientId: app.BridgeClientId,
			RequestId:      req.RequestId,
		},
		Tool:      tool,
		Arguments: req.Params.Arguments,
	}, decision)
	if !decision.Allowed() {
		_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, model.TunnelAuditActionPolicyDeny, decision.Decision, req.RequestId, tool.Name, mcpgateway.MethodToolsCall, auditEvent.BytesIn, 0, 0, map[string]any{
			"connection_id":         connection.Id,
			"connection_key_prefix": connection.KeyPrefix,
			"reason":                decision.Reason,
			"tool_category":         decision.Category,
			"tool_schema_hash":      auditEvent.ToolSchemaHash,
			"argument_hash":         auditEvent.ArgumentHash,
		})
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidRequest,
			ErrorMessage: "MCP gateway policy denied tool call",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: tool.Name,
				Reason:   decision.Reason,
			},
		}, nil
	}
	startedAt := time.Now()
	result, err := defaultTunnelMCPProxyClient.CallTool(ctx, server, mcpproxy.CallRequest{
		ToolName:  tool.Name,
		Arguments: req.Params.Arguments,
		RequestId: req.RequestId,
		UserId:    req.UserId,
		TokenId:   req.TokenId,
	})
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, model.TunnelAuditActionMCPToolCall, mcpgateway.DecisionDeny, req.RequestId, tool.Name, mcpgateway.MethodToolsCall, auditEvent.BytesIn, 0, durationMS, map[string]any{
			"connection_id":         connection.Id,
			"connection_key_prefix": connection.KeyPrefix,
			"error":                 err.Error(),
			"tool_category":         decision.Category,
			"tool_schema_hash":      auditEvent.ToolSchemaHash,
			"argument_hash":         auditEvent.ArgumentHash,
		})
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeExecutorFailed,
			ErrorMessage: "MCP gateway upstream tool call failed",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: tool.Name,
				Reason:   err.Error(),
			},
		}, nil
	}
	resultBody := NormalizeMCPToolResult(result)
	resultBytes := len(mcpgateway.NormalizeRawJSON(resultBody))
	if err := checkTunnelMCPResponseRateLimit(*app, *connection, req.SessionId, req.RequestId, mcpgateway.MethodToolsCall, tool.Name, int64(resultBytes), durationMS, map[string]any{
		"tool_category":     decision.Category,
		"tool_schema_hash":  auditEvent.ToolSchemaHash,
		"argument_hash":     auditEvent.ArgumentHash,
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
	}); err != nil {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidRequest,
			ErrorMessage: "MCP gateway rate limit exceeded",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: tool.Name,
				Reason:   err.Error(),
			},
		}, nil
	}
	_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, model.TunnelAuditActionMCPToolCall, mcpgateway.DecisionAllow, req.RequestId, tool.Name, mcpgateway.MethodToolsCall, auditEvent.BytesIn, int64(resultBytes), durationMS, map[string]any{
		"connection_id":         connection.Id,
		"connection_key_prefix": connection.KeyPrefix,
		"tool_category":         decision.Category,
		"tool_schema_hash":      auditEvent.ToolSchemaHash,
		"argument_hash":         auditEvent.ArgumentHash,
		"bridge_session_id":     result.BridgeSessionId,
		"target_client":         result.TargetClient,
		"downstream_summary":    result.Summary,
	})
	return &MCPToolCallResponse{Result: resultBody}, nil
}

func CallTunnelMCPRaw(req TunnelMCPRawRequest) (json.RawMessage, error) {
	method := strings.TrimSpace(req.Method)
	if !isTunnelMCPRawForwardMethod(method) {
		return nil, errors.New("unsupported tunnel mcp method")
	}
	app, connection, err := getAuthorizedTunnelMCPApp(req.Slug, req.ConnectionKey, req.UserId)
	if err != nil {
		return nil, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	bytesIn := int64(len(bytes.TrimSpace(req.Params)))
	if err := checkTunnelMCPBillingBalance(*app, *connection, req.SessionId, req.RequestId, method, "", bytesIn); err != nil {
		return nil, err
	}
	if err := checkTunnelMCPRequestRateLimit(*app, *connection, req.SessionId, req.RequestId, method, "", bytesIn); err != nil {
		return nil, err
	}
	decision, policyMetadata := authorizeTunnelMCPRawForward(*app, method, req.Params)
	if !decision.Allowed() {
		metadata := map[string]any{
			"connection_id":         connection.Id,
			"connection_key_prefix": connection.KeyPrefix,
			"reason":                decision.Reason,
			"policy_category":       decision.Category,
		}
		for key, value := range policyMetadata {
			metadata[key] = value
		}
		_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, model.TunnelAuditActionPolicyDeny, decision.Decision, req.RequestId, "", method, bytesIn, 0, 0, metadata)
		return nil, fmt.Errorf("%w: %s", ErrTunnelMCPPolicyDenied, decision.Reason)
	}
	rawCaller, ok := defaultTunnelMCPProxyClient.(mcpproxy.RawCaller)
	if !ok {
		return nil, errors.New("tunnel mcp proxy client does not support raw MCP forwarding")
	}
	startedAt := time.Now()
	result, err := rawCaller.CallRaw(tunnelMCPContext(req.Context), tunnelMCPProxyServer(*app), mcpproxy.RawRequest{
		Method:    method,
		Params:    req.Params,
		RequestId: req.RequestId,
		UserId:    req.UserId,
		TokenId:   req.TokenId,
	})
	durationMS := int(time.Since(startedAt).Milliseconds())
	action := tunnelMCPRawAuditAction(method)
	if err != nil {
		_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, action, mcpgateway.DecisionDeny, req.RequestId, "", method, bytesIn, 0, durationMS, map[string]any{
			"connection_id":         connection.Id,
			"connection_key_prefix": connection.KeyPrefix,
			"error":                 err.Error(),
		})
		return nil, err
	}
	result.Result, policyMetadata = filterTunnelMCPRawResult(*app, method, result.Result)
	bytesOut := int64(result.ResultSize)
	if len(policyMetadata) > 0 {
		bytesOut = int64(len(result.Result))
	}
	if bytesOut <= 0 {
		bytesOut = int64(len(result.Result))
	}
	responseMetadata := map[string]any{
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
	}
	for key, value := range policyMetadata {
		responseMetadata[key] = value
	}
	if err := checkTunnelMCPResponseRateLimit(*app, *connection, req.SessionId, req.RequestId, method, "", bytesOut, durationMS, responseMetadata); err != nil {
		return nil, err
	}
	_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, action, mcpgateway.DecisionAllow, req.RequestId, "", method, bytesIn, bytesOut, durationMS, responseMetadata)
	if len(bytes.TrimSpace(result.Result)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return result.Result, nil
}

func authorizeTunnelMCPRawForward(app model.TunnelApp, method string, params json.RawMessage) (mcpgateway.Decision, map[string]any) {
	policy := tunnelMCPRawPolicyFromApp(app)
	switch strings.TrimSpace(method) {
	case dto.MCPMethodResourcesRead:
		uri := tunnelMCPRawParamString(params, "uri")
		allowed, matched := tunnelMCPResourceURIAllowed(policy, uri)
		metadata := map[string]any{
			"resource_uri_hash": tunnelMCPHashString(uri),
			"matched_policy":    matched,
		}
		if !allowed {
			return mcpgateway.Decision{
				Decision: mcpgateway.DecisionDeny,
				Reason:   "resource URI is denied by gateway policy",
				Category: "resource",
			}, metadata
		}
	case dto.MCPMethodPromptsGet:
		name := tunnelMCPRawParamString(params, "name")
		allowed, matched := tunnelMCPPromptAllowed(policy, name)
		metadata := map[string]any{
			"prompt_name":    limitTunnelString(name, 160),
			"matched_policy": matched,
		}
		if !allowed {
			return mcpgateway.Decision{
				Decision: mcpgateway.DecisionDeny,
				Reason:   "prompt is denied by gateway policy",
				Category: "prompt",
			}, metadata
		}
	}
	return mcpgateway.Decision{Decision: mcpgateway.DecisionAllow}, nil
}

func filterTunnelMCPRawResult(app model.TunnelApp, method string, raw json.RawMessage) (json.RawMessage, map[string]any) {
	policy := tunnelMCPRawPolicyFromApp(app)
	switch strings.TrimSpace(method) {
	case dto.MCPMethodResourcesList:
		return filterTunnelMCPResourceListResult(policy, raw)
	case dto.MCPMethodResourcesTemplatesList:
		return filterTunnelMCPResourceTemplatesListResult(policy, raw)
	case dto.MCPMethodPromptsList:
		return filterTunnelMCPPromptListResult(policy, raw)
	default:
		return raw, nil
	}
}

func filterTunnelMCPResourceListResult(policy tunnelMCPRawPolicy, raw json.RawMessage) (json.RawMessage, map[string]any) {
	var body map[string]any
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &body) != nil {
		return raw, nil
	}
	items, ok := body["resources"].([]any)
	if !ok {
		return raw, nil
	}
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		object := mapFromAny(item)
		uri := stringFromAny(object["uri"])
		allowed, _ := tunnelMCPResourceURIAllowed(policy, uri)
		if allowed {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == len(items) {
		return raw, nil
	}
	body["resources"] = filtered
	next, err := json.Marshal(body)
	if err != nil {
		return raw, nil
	}
	return next, map[string]any{
		"raw_filter_applied": true,
		"raw_filter_kind":    "resources",
		"discovered_count":   len(items),
		"exposed_count":      len(filtered),
		"filtered_count":     len(items) - len(filtered),
	}
}

func filterTunnelMCPResourceTemplatesListResult(policy tunnelMCPRawPolicy, raw json.RawMessage) (json.RawMessage, map[string]any) {
	var body map[string]any
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &body) != nil {
		return raw, nil
	}
	items, ok := body["resourceTemplates"].([]any)
	if !ok {
		return raw, nil
	}
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		object := mapFromAny(item)
		uriTemplate := tunnelMCPResourceTemplateURI(object)
		allowed, _ := tunnelMCPResourceURIAllowed(policy, uriTemplate)
		if allowed {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == len(items) {
		return raw, nil
	}
	body["resourceTemplates"] = filtered
	next, err := json.Marshal(body)
	if err != nil {
		return raw, nil
	}
	return next, map[string]any{
		"raw_filter_applied": true,
		"raw_filter_kind":    "resource_templates",
		"discovered_count":   len(items),
		"exposed_count":      len(filtered),
		"filtered_count":     len(items) - len(filtered),
	}
}

func filterTunnelMCPPromptListResult(policy tunnelMCPRawPolicy, raw json.RawMessage) (json.RawMessage, map[string]any) {
	var body map[string]any
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &body) != nil {
		return raw, nil
	}
	items, ok := body["prompts"].([]any)
	if !ok {
		return raw, nil
	}
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		object := mapFromAny(item)
		name := stringFromAny(object["name"])
		allowed, _ := tunnelMCPPromptAllowed(policy, name)
		if allowed {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == len(items) {
		return raw, nil
	}
	body["prompts"] = filtered
	next, err := json.Marshal(body)
	if err != nil {
		return raw, nil
	}
	return next, map[string]any{
		"raw_filter_applied": true,
		"raw_filter_kind":    "prompts",
		"discovered_count":   len(items),
		"exposed_count":      len(filtered),
		"filtered_count":     len(items) - len(filtered),
	}
}

func tunnelMCPResourceTemplateURI(object map[string]any) string {
	return firstTunnelStringValue(object, "uriTemplate", "uri_template", "template", "uri")
}

func tunnelMCPRawPolicyFromApp(app model.TunnelApp) tunnelMCPRawPolicy {
	values := unmarshalTunnelMap(app.PolicyJson)
	if nested := mapFromAny(values["mcp_gateway"]); len(nested) > 0 {
		values = nested
	}
	return tunnelMCPRawPolicy{
		AllowedResourceURIPrefixes: firstTunnelStringSlice(values, "allowed_resource_uri_prefixes", "allowed_resource_prefixes"),
		DeniedResourceURIPrefixes:  firstTunnelStringSlice(values, "denied_resource_uri_prefixes", "denied_resource_prefixes"),
		AllowedPromptNames:         firstTunnelStringSlice(values, "allowed_prompt_names", "allowed_prompts"),
		DeniedPromptNames:          firstTunnelStringSlice(values, "denied_prompt_names", "denied_prompts"),
		AllowedPromptPrefixes:      firstTunnelStringSlice(values, "allowed_prompt_prefixes"),
		DeniedPromptPrefixes:       firstTunnelStringSlice(values, "denied_prompt_prefixes"),
	}
}

func tunnelMCPResourceURIAllowed(policy tunnelMCPRawPolicy, uri string) (bool, string) {
	uri = strings.TrimSpace(uri)
	if matched, prefix := tunnelStringHasPrefix(uri, policy.DeniedResourceURIPrefixes); matched {
		return false, "denied_resource_prefix:" + prefix
	}
	if len(policy.AllowedResourceURIPrefixes) > 0 {
		if matched, prefix := tunnelStringHasPrefix(uri, policy.AllowedResourceURIPrefixes); matched {
			return true, "allowed_resource_prefix:" + prefix
		}
		return false, "allowed_resource_prefix_required"
	}
	return true, ""
}

func tunnelMCPPromptAllowed(policy tunnelMCPRawPolicy, name string) (bool, string) {
	name = strings.TrimSpace(name)
	if matched, value := tunnelStringExactMatch(name, policy.DeniedPromptNames); matched {
		return false, "denied_prompt:" + value
	}
	if matched, prefix := tunnelStringHasPrefix(name, policy.DeniedPromptPrefixes); matched {
		return false, "denied_prompt_prefix:" + prefix
	}
	if len(policy.AllowedPromptNames) > 0 || len(policy.AllowedPromptPrefixes) > 0 {
		if matched, value := tunnelStringExactMatch(name, policy.AllowedPromptNames); matched {
			return true, "allowed_prompt:" + value
		}
		if matched, prefix := tunnelStringHasPrefix(name, policy.AllowedPromptPrefixes); matched {
			return true, "allowed_prompt_prefix:" + prefix
		}
		return false, "allowed_prompt_required"
	}
	return true, ""
}

func tunnelMCPRawParamString(params json.RawMessage, key string) string {
	var body map[string]any
	if len(bytes.TrimSpace(params)) == 0 || json.Unmarshal(params, &body) != nil {
		return ""
	}
	return stringFromAny(body[key])
}

func tunnelMCPHashString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return mcpgateway.HashRawJSON(body)
}

func firstTunnelStringSlice(values map[string]any, keys ...string) []string {
	for _, key := range keys {
		if result := sliceStringFromAny(values[key]); len(result) > 0 {
			return result
		}
	}
	return nil
}

func firstTunnelStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromAny(values[key]); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func tunnelStringHasPrefix(value string, prefixes []string) (bool, string) {
	value = strings.TrimSpace(value)
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(value, prefix) {
			return true, prefix
		}
	}
	return false, ""
}

func tunnelStringExactMatch(value string, candidates []string) (bool, string) {
	value = strings.TrimSpace(value)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && value == candidate {
			return true, candidate
		}
	}
	return false, ""
}

func listTunnelMCPProxyTools(ctx context.Context, app model.TunnelApp) ([]mcpproxy.ToolDefinition, error) {
	scoped, ok := defaultTunnelMCPProxyClient.(mcpproxy.UserScopedListToolsClient)
	if !ok {
		return nil, errors.New("tunnel mcp proxy client does not support user-scoped tools/list")
	}
	return scoped.ListToolsForUser(ctx, tunnelMCPProxyServer(app), app.UserId)
}

func isTunnelMCPRawForwardMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case dto.MCPMethodResourcesList,
		dto.MCPMethodResourcesRead,
		dto.MCPMethodResourcesTemplatesList,
		dto.MCPMethodPromptsList,
		dto.MCPMethodPromptsGet:
		return true
	default:
		return false
	}
}

func tunnelMCPRawAuditAction(method string) string {
	switch strings.TrimSpace(method) {
	case dto.MCPMethodResourcesRead:
		return mcpgateway.AuditActionResourcesRead
	case dto.MCPMethodPromptsGet:
		return mcpgateway.AuditActionPromptsGet
	default:
		return strings.ReplaceAll(strings.TrimSpace(method), "/", "_")
	}
}

func NormalizeMCPToolResult(result mcpproxy.CallResult) *dto.MCPToolCallResult {
	normalized := &dto.MCPToolCallResult{
		Content:  result.Content,
		Metadata: result.Metadata,
	}
	if normalized.Content == nil {
		normalized.Content = []dto.MCPContentBlock{}
	}
	return normalized
}

func getAuthorizedTunnelMCPApp(slug string, connectionKey string, userId int) (*model.TunnelApp, *model.TunnelConnection, error) {
	if userId <= 0 {
		return nil, nil, errors.New("invalid tunnel mcp user")
	}
	if strings.TrimSpace(connectionKey) == "" {
		return nil, nil, errors.New("tunnel mcp connection key is required")
	}
	connection, err := model.GetTunnelConnectionByKeyHash(tunnelConnectionKeyHash(connectionKey))
	if err != nil {
		return nil, nil, err
	}
	app, err := model.GetTunnelAppByPublicSlug(slug)
	if err != nil {
		return nil, nil, err
	}
	if app.UserId != userId || connection.UserId != userId || connection.AppId != app.Id {
		return nil, nil, errors.New("tunnel mcp app not found")
	}
	if app.AppType != model.TunnelAppTypeMCPCode {
		return nil, nil, errors.New("tunnel app is not an MCP code tunnel")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return nil, nil, errors.New("tunnel mcp app is not approved")
	}
	if strings.TrimSpace(app.BridgeClientId) == "" {
		return nil, nil, errors.New("tunnel mcp app has no bridge client")
	}
	if connection.Status != model.TunnelConnectionStatusActive {
		return nil, nil, errors.New("tunnel mcp connection is not active")
	}
	if connection.ExpiresAt > 0 && connection.ExpiresAt <= common.GetTimestamp() {
		return nil, nil, errors.New("tunnel mcp connection is expired")
	}
	return app, connection, nil
}

func tunnelMCPProxyServer(app model.TunnelApp) model.MCPProxyServer {
	return model.MCPProxyServer{
		Name:            app.Name,
		Namespace:       app.PublicSlug,
		Transport:       model.MCPProxyTransportBridge,
		Endpoint:        tunnelMCPBridgeEndpoint(app),
		Status:          model.MCPProxyServerStatusEnabled,
		TimeoutMS:       30000,
		MaxResultSize:   1048576,
		MaxMetadataSize: 65536,
	}
}

func tunnelMCPBridgeEndpoint(app model.TunnelApp) string {
	endpoint := "bridge://" + strings.TrimSpace(app.BridgeClientId)
	target := tunnelMCPTargetURL(app)
	if target == "" {
		return endpoint
	}
	return endpoint + "?target=" + url.QueryEscape(target)
}

func tunnelMCPTargetURL(app model.TunnelApp) string {
	targetPath := strings.TrimSpace(app.TargetPath)
	if strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://") {
		return targetPath
	}
	if strings.TrimSpace(app.TargetHost) == "" && app.TargetPort <= 0 {
		return ""
	}
	host := strings.TrimSpace(app.TargetHost)
	if host == "" {
		host = "127.0.0.1"
	}
	if targetPath == "" {
		targetPath = "/mcp"
	}
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	if app.TargetPort <= 0 {
		return "http://" + host + targetPath
	}
	return "http://" + host + ":" + strconv.Itoa(app.TargetPort) + targetPath
}

func tunnelMCPContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func tunnelMCPToolsFromProxyDefinitions(tools []mcpproxy.ToolDefinition) []mcpgateway.Tool {
	result := make([]mcpgateway.Tool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, mcpgateway.Tool{
			Name:        strings.TrimSpace(tool.Name),
			Description: strings.TrimSpace(tool.Description),
			InputSchema: tool.InputSchema,
		})
	}
	return result
}

func tunnelMCPDTOFromGatewayTools(tools []mcpgateway.Tool) []dto.MCPTool {
	result := make([]dto.MCPTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, dto.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return result
}

func tunnelMCPFindGatewayTool(tools []mcpproxy.ToolDefinition, name string) (mcpgateway.Tool, bool) {
	name = strings.TrimSpace(name)
	for _, tool := range tunnelMCPToolsFromProxyDefinitions(tools) {
		if tool.Name == name {
			return tool, true
		}
	}
	return mcpgateway.Tool{}, false
}

func createTunnelMCPAuditLog(app model.TunnelApp, connection model.TunnelConnection, sessionId string, action string, decision string, requestId string, toolName string, method string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	metadataJSON := ""
	if metadata != nil {
		if body, err := common.Marshal(metadata); err == nil {
			metadataJSON = string(body)
		}
	}
	log := &model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         app.UserId,
		Action:              action,
		Decision:            decision,
		Reason:              stringFromMCPGatewayMetadata(metadata, "reason"),
		RequestId:           requestId,
		ToolName:            toolName,
		Method:              method,
		BytesIn:             bytesIn,
		BytesOut:            bytesOut,
		DurationMS:          durationMS,
		MetadataJson:        metadataJSON,
		SessionId:           strings.TrimSpace(sessionId),
	}
	if err := model.CreateTunnelAuditLog(log); err != nil {
		return err
	}
	_ = model.AddTunnelSessionUsage(sessionId, bytesIn, bytesOut)
	return recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, *log)
}

func checkTunnelMCPRequestRateLimit(app model.TunnelApp, connection model.TunnelConnection, sessionId string, requestId string, method string, toolName string, bytesIn int64) error {
	if err := checkTunnelRequestRateLimit(app, connection, bytesIn); err != nil {
		metadata := tunnelMCPRateLimitMetadata(err)
		metadata["phase"] = "request"
		_ = createTunnelMCPAuditLog(app, connection, sessionId, model.TunnelAuditActionRateLimit, mcpgateway.DecisionDeny, requestId, toolName, method, bytesIn, 0, 0, metadata)
		return err
	}
	return nil
}

func checkTunnelMCPBillingBalance(app model.TunnelApp, connection model.TunnelConnection, sessionId string, requestId string, method string, toolName string, bytesIn int64) error {
	estimatedMinQuota := 0
	if method == mcpgateway.MethodToolsCall {
		estimatedMinQuota = tunnelBillingPreflightQuota(app, model.TunnelAuditActionMCPToolCall, bytesIn)
	}
	policy, quota, err := checkTunnelBillingBalance(app, estimatedMinQuota)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTunnelBillingInsufficient) {
		metadata := tunnelBillingDenyMetadata(policy, quota, err)
		_ = createTunnelMCPAuditLog(app, connection, sessionId, model.TunnelAuditActionBillingDeny, mcpgateway.DecisionDeny, requestId, toolName, method, bytesIn, 0, 0, metadata)
	}
	return err
}

func checkTunnelMCPResponseRateLimit(app model.TunnelApp, connection model.TunnelConnection, sessionId string, requestId string, method string, toolName string, bytesOut int64, durationMS int, metadata map[string]any) error {
	if err := checkTunnelResponseRateLimit(app, connection, bytesOut); err != nil {
		if metadata == nil {
			metadata = map[string]any{}
		}
		for key, value := range tunnelMCPRateLimitMetadata(err) {
			metadata[key] = value
		}
		metadata["phase"] = "response"
		_ = createTunnelMCPAuditLog(app, connection, sessionId, model.TunnelAuditActionRateLimit, mcpgateway.DecisionDeny, requestId, toolName, method, 0, bytesOut, durationMS, metadata)
		return err
	}
	return nil
}

func tunnelMCPRateLimitMetadata(err error) map[string]any {
	var limitErr TunnelRateLimitError
	if errors.As(err, &limitErr) {
		metadata := limitErr.Metadata()
		metadata["error"] = err.Error()
		return metadata
	}
	return map[string]any{
		"reason": "rate_limited",
		"error":  err.Error(),
	}
}

func stringFromMCPGatewayMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}
