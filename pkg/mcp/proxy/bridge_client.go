package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
)

const (
	BridgeCapabilityMCPProxy = "mcp_proxy"

	BridgeToolMCPProxyTest      = "mcp_proxy.test"
	BridgeToolMCPProxyListTools = "mcp_proxy.tools_list"
	BridgeToolMCPProxyCallTool  = "mcp_proxy.tools_call"
)

type BridgeEndpoint struct {
	ClientId string
	Target   string
}

type BridgeClient struct {
	Hub *bridge.Hub
}

func NewBridgeClient(hub *bridge.Hub) *BridgeClient {
	if hub == nil {
		hub = bridge.DefaultHub
	}
	return &BridgeClient{Hub: hub}
}

func ParseBridgeEndpoint(endpoint string) (BridgeEndpoint, error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return BridgeEndpoint{}, errors.New("bridge endpoint is required")
	}
	if !strings.Contains(raw, "://") {
		return BridgeEndpoint{ClientId: raw}, nil
	}
	parsed, err := url.Parse(normalizeBridgeEndpointURL(raw))
	if err != nil {
		return BridgeEndpoint{}, err
	}
	switch strings.TrimSpace(parsed.Scheme) {
	case "bridge", "qidian", "qidian_browser":
	default:
		return BridgeEndpoint{}, fmt.Errorf("unsupported bridge endpoint scheme: %s", parsed.Scheme)
	}
	clientId := strings.TrimSpace(parsed.Host)
	if clientId == "" {
		return BridgeEndpoint{}, errors.New("bridge endpoint client_id is required")
	}
	target := strings.TrimSpace(parsed.Query().Get("target"))
	if target == "" {
		pathTarget := strings.TrimPrefix(parsed.EscapedPath(), "/")
		if pathTarget != "" {
			if unescaped, err := url.PathUnescape(pathTarget); err == nil {
				target = strings.TrimSpace(unescaped)
			}
		}
	}
	return BridgeEndpoint{ClientId: clientId, Target: target}, nil
}

func normalizeBridgeEndpointURL(raw string) string {
	const legacyScheme = "qidian_browser://"
	if strings.HasPrefix(strings.ToLower(raw), legacyScheme) {
		return "qidian://" + raw[len(legacyScheme):]
	}
	return raw
}

func (c *BridgeClient) Test(ctx context.Context, server model.MCPProxyServer) (TestResult, error) {
	endpoint, session, err := c.selectSession(server, 0)
	if err != nil {
		return TestResult{}, err
	}
	response, err := c.forward(ctx, session, BridgeToolMCPProxyTest, bridgeProxyBaseArguments(server, endpoint), "", 0, 0)
	if err != nil {
		return TestResult{}, err
	}
	object := bridgeResultObject(response.Result)
	protocolVersion := stringFromAny(firstNonNil(object["protocol_version"], object["protocolVersion"]))
	if protocolVersion == "" {
		protocolVersion = dto.MCPProtocolVersion
	}
	serverName := stringFromAny(firstNonNil(object["server_name"], object["serverName"]))
	if serverName == "" {
		serverName = server.Name
	}
	return TestResult{
		ProtocolVersion: protocolVersion,
		ServerName:      serverName,
		Capabilities:    mapFromAny(object["capabilities"]),
	}, nil
}

func (c *BridgeClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]ToolDefinition, error) {
	endpoint, session, err := c.selectSession(server, 0)
	if err != nil {
		return nil, err
	}
	response, err := c.forward(ctx, session, BridgeToolMCPProxyListTools, bridgeProxyBaseArguments(server, endpoint), "", 0, 0)
	if err != nil {
		return nil, err
	}
	object := bridgeResultObject(response.Result)
	tools, err := bridgeToolDefinitionsFromAny(firstNonNil(object["tools"], object["Tools"]))
	if err != nil {
		return nil, err
	}
	return tools, nil
}

func (c *BridgeClient) CallTool(ctx context.Context, server model.MCPProxyServer, req CallRequest) (CallResult, error) {
	endpoint, session, err := c.selectSession(server, req.UserId)
	if err != nil {
		return CallResult{}, err
	}
	args := bridgeProxyBaseArguments(server, endpoint)
	args["name"] = strings.TrimSpace(req.ToolName)
	args["arguments"] = req.Arguments
	response, err := c.forward(ctx, session, BridgeToolMCPProxyCallTool, args, req.RequestId, req.UserId, req.TokenId)
	result := bridgeCallResultFromResponse(response)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (c *BridgeClient) SessionSnapshot(server model.MCPProxyServer) SessionSnapshot {
	snapshot := SessionSnapshot{Transport: strings.TrimSpace(server.Transport)}
	endpoint, err := ParseBridgeEndpoint(server.Endpoint)
	if err != nil {
		snapshot.LastError = err.Error()
		return snapshot
	}
	session, err := c.sessionByEndpoint(endpoint, 0)
	if err != nil {
		snapshot.LastError = err.Error()
		return snapshot
	}
	snapshot.HasSession = true
	snapshot.Initialized = bridgeSessionSupports(session, BridgeCapabilityMCPProxy)
	snapshot.MessageEndpoint = endpoint.ClientId
	return snapshot
}

func (c *BridgeClient) selectSession(server model.MCPProxyServer, userId int) (BridgeEndpoint, bridge.SessionSnapshot, error) {
	endpoint, err := ParseBridgeEndpoint(server.Endpoint)
	if err != nil {
		return BridgeEndpoint{}, bridge.SessionSnapshot{}, err
	}
	session, err := c.sessionByEndpoint(endpoint, userId)
	if err != nil {
		return BridgeEndpoint{}, bridge.SessionSnapshot{}, err
	}
	return endpoint, session, nil
}

func (c *BridgeClient) sessionByEndpoint(endpoint BridgeEndpoint, userId int) (bridge.SessionSnapshot, error) {
	if c == nil || c.Hub == nil {
		return bridge.SessionSnapshot{}, bridge.ErrClientUnavailable
	}
	session, ok := c.Hub.GetByClient(endpoint.ClientId)
	if !ok {
		return bridge.SessionSnapshot{}, bridge.ErrClientNotFound
	}
	if userId > 0 && session.UserId != userId {
		return bridge.SessionSnapshot{}, fmt.Errorf("%w: bridge client belongs to a different user", bridge.ErrClientNotFound)
	}
	if !bridgeSessionSupports(session, BridgeCapabilityMCPProxy) {
		return bridge.SessionSnapshot{}, fmt.Errorf("%w: bridge client does not support %s", bridge.ErrClientNotFound, BridgeCapabilityMCPProxy)
	}
	return session, nil
}

func (c *BridgeClient) forward(ctx context.Context, session bridge.SessionSnapshot, toolName string, args map[string]any, requestId string, userId int, tokenId int) (bridge.ToolCallResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(requestId)
	if id == "" {
		id = common.GetRandomString(32)
	}
	startedAt := time.Now()
	audit := createBridgeProxyAudit(id, session, toolName, args, userId, tokenId)
	response, err := c.Hub.ForwardToolCall(ctx, session.SessionId, bridge.ToolCallRequest{
		Id:        id,
		ToolName:  toolName,
		Arguments: args,
	})
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		updateBridgeProxyAuditError(audit, bridgeProxyAuditStatus(err), bridgeProxyErrorCode(err), err.Error(), durationMS)
		return response, err
	}
	resultSize := response.Result.ResultSize
	if resultSize <= 0 {
		if bytes, marshalErr := common.Marshal(response.Result); marshalErr == nil {
			resultSize = len(bytes)
		}
	}
	updateBridgeProxyAuditSuccess(audit, durationMS, resultSize)
	return response, nil
}

func bridgeProxyBaseArguments(server model.MCPProxyServer, endpoint BridgeEndpoint) map[string]any {
	args := map[string]any{
		"transport": server.Transport,
		"endpoint":  server.Endpoint,
		"server": map[string]any{
			"id":        server.Id,
			"name":      server.Name,
			"namespace": server.Namespace,
		},
	}
	if endpoint.Target != "" {
		args["target"] = endpoint.Target
	}
	return args
}

func bridgeCallResultFromResponse(response bridge.ToolCallResponse) CallResult {
	result := response.Result
	resultSize := result.ResultSize
	if resultSize <= 0 {
		if bytes, err := common.Marshal(result); err == nil {
			resultSize = len(bytes)
		}
	}
	durationMS := result.DurationMS
	return CallResult{
		Content:         result.Content,
		Metadata:        result.Metadata,
		Summary:         result.Summary,
		DurationMS:      durationMS,
		ResultSize:      resultSize,
		BridgeSessionId: response.Session.SessionId,
		TargetClient:    response.Session.ClientId,
	}
}

func createBridgeProxyAudit(requestId string, session bridge.SessionSnapshot, toolName string, args map[string]any, userId int, tokenId int) *model.BridgeAuditLog {
	if model.DB == nil || requestId == "" || userId <= 0 {
		return nil
	}
	requestBody := ""
	if bytes, err := common.Marshal(args); err == nil {
		requestBody = string(bytes)
	}
	audit := &model.BridgeAuditLog{
		RequestId:   requestId,
		SessionId:   session.SessionId,
		ClientId:    session.ClientId,
		UserId:      userId,
		TokenId:     tokenId,
		ToolName:    toolName,
		RequestBody: requestBody,
		Status:      model.BridgeAuditStatusPending,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		return nil
	}
	return audit
}

func updateBridgeProxyAuditSuccess(audit *model.BridgeAuditLog, durationMS int, resultSize int) {
	if audit == nil {
		return
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusSuccess, map[string]any{
		"duration_ms": durationMS,
		"result_size": resultSize,
	})
}

func updateBridgeProxyAuditError(audit *model.BridgeAuditLog, status string, code string, message string, durationMS int) {
	if audit == nil {
		return
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, status, map[string]any{
		"error_code":    code,
		"error_message": truncateBridgeProxyError(message, 512),
		"duration_ms":   durationMS,
	})
}

func bridgeProxyAuditStatus(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return model.BridgeAuditStatusTimeout
	}
	return model.BridgeAuditStatusError
}

func bridgeProxyErrorCode(err error) string {
	var clientErr *bridge.ClientError
	if errors.As(err, &clientErr) && clientErr.Code != "" {
		return clientErr.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "EXECUTOR_TIMEOUT"
	}
	if errors.Is(err, bridge.ErrClientDisconnected) {
		return "BRIDGE_CLIENT_DISCONNECTED"
	}
	if errors.Is(err, bridge.ErrClientNotFound) {
		return "BRIDGE_CLIENT_NOT_FOUND"
	}
	return "EXECUTOR_FAILED"
}

func bridgeResultObject(result dto.BridgeToolCallResult) map[string]any {
	if object := mapFromAny(firstNonNil(result.Metadata["result"], result.Metadata["structuredContent"])); len(object) > 0 {
		return object
	}
	if len(result.Metadata) > 0 {
		return result.Metadata
	}
	for _, block := range result.Content {
		text := strings.TrimSpace(block.Text)
		if text == "" || !json.Valid([]byte(text)) {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(text), &object); err == nil {
			return object
		}
	}
	return map[string]any{}
}

func bridgeToolDefinitionsFromAny(value any) ([]ToolDefinition, error) {
	items := sliceFromAny(value)
	if len(items) == 0 {
		return nil, errors.New("bridge mcp proxy tools_list result did not include tools")
	}
	tools := make([]ToolDefinition, 0, len(items))
	for _, item := range items {
		object := mapFromAny(item)
		name := stringFromAny(object["name"])
		if name == "" {
			return nil, errors.New("bridge mcp proxy tool name is required")
		}
		schema := mapFromAny(firstNonNil(object["inputSchema"], object["input_schema"]))
		if len(schema) == 0 {
			schema = map[string]any{"type": "object"}
		}
		tools = append(tools, ToolDefinition{
			Name:        name,
			Title:       stringFromAny(object["title"]),
			Description: stringFromAny(object["description"]),
			InputSchema: schema,
		})
	}
	return tools, nil
}

func bridgeSessionSupports(session bridge.SessionSnapshot, capability string) bool {
	if capability == "" {
		return true
	}
	for _, item := range session.Capabilities {
		if strings.TrimSpace(item) == capability {
			return true
		}
	}
	return false
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	raw, err := json.Marshal(value)
	if err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return map[string]any{}
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return map[string]any{}
	}
	return object
}

func sliceFromAny(value any) []any {
	if value == nil {
		return nil
	}
	if items, ok := value.([]any); ok {
		return items
	}
	raw, err := json.Marshal(value)
	if err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return nil
	}
	var items []any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return items
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func truncateBridgeProxyError(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
