package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	if err := checkTunnelMCPRequestRateLimit(*app, *connection, req.SessionId, req.RequestId, method, "", bytesIn); err != nil {
		return nil, err
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
	bytesOut := int64(result.ResultSize)
	if bytesOut <= 0 {
		bytesOut = int64(len(result.Result))
	}
	if err := checkTunnelMCPResponseRateLimit(*app, *connection, req.SessionId, req.RequestId, method, "", bytesOut, durationMS, map[string]any{
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
	}); err != nil {
		return nil, err
	}
	_ = createTunnelMCPAuditLog(*app, *connection, req.SessionId, action, mcpgateway.DecisionAllow, req.RequestId, "", method, bytesIn, bytesOut, durationMS, map[string]any{
		"connection_id":         connection.Id,
		"connection_key_prefix": connection.KeyPrefix,
		"bridge_session_id":     result.BridgeSessionId,
		"target_client":         result.TargetClient,
	})
	if len(bytes.TrimSpace(result.Result)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return result.Result, nil
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
