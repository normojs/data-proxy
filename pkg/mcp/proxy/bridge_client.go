package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
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

func (e BridgeEndpoint) IsAuto() bool {
	clientId := strings.TrimSpace(strings.ToLower(e.ClientId))
	return clientId == "auto" || clientId == "*"
}

type BridgeClient struct {
	Hub *bridge.Hub
}

type bridgeSessionCandidate struct {
	session bridge.SessionSnapshot
	policy  bridgepolicy.Policy
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
	endpoint, candidates, err := c.candidateSessions(server, 0)
	if err != nil {
		return TestResult{}, err
	}
	response, err := c.forwardCandidates(ctx, candidates, BridgeToolMCPProxyTest, bridgeProxyBaseArguments(server, endpoint), "", 0, 0)
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
	endpoint, candidates, err := c.candidateSessions(server, 0)
	if err != nil {
		return nil, err
	}
	response, err := c.forwardCandidates(ctx, candidates, BridgeToolMCPProxyListTools, bridgeProxyBaseArguments(server, endpoint), "", 0, 0)
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
	endpoint, candidates, err := c.candidateSessions(server, req.UserId)
	if err != nil {
		return CallResult{}, err
	}
	args := bridgeProxyBaseArguments(server, endpoint)
	args["name"] = strings.TrimSpace(req.ToolName)
	args["arguments"] = req.Arguments
	response, err := c.forwardCandidates(ctx, candidates, BridgeToolMCPProxyCallTool, args, req.RequestId, req.UserId, req.TokenId)
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
	sessions, err := c.sessionsByEndpoint(endpoint, 0)
	if err != nil {
		snapshot.LastError = err.Error()
		return snapshot
	}
	session := sessions[0]
	snapshot.HasSession = true
	snapshot.Initialized = bridgeSessionSupports(session, BridgeCapabilityMCPProxy)
	snapshot.MessageEndpoint = endpoint.ClientId
	snapshot.ActiveSessions = len(sessions)
	if !session.LastSeenAt.IsZero() {
		snapshot.LastActivityAt = session.LastSeenAt.Unix()
	}
	return snapshot
}

func (c *BridgeClient) candidateSessions(server model.MCPProxyServer, userId int) (BridgeEndpoint, []bridgeSessionCandidate, error) {
	endpoint, err := ParseBridgeEndpoint(server.Endpoint)
	if err != nil {
		return BridgeEndpoint{}, nil, err
	}
	sessions, err := c.sessionsByEndpoint(endpoint, userId)
	if err != nil {
		return BridgeEndpoint{}, nil, err
	}
	candidates := make([]bridgeSessionCandidate, 0, len(sessions))
	for _, session := range sessions {
		policy, err := model.GetBridgeClientPolicyByClientId(session.ClientId)
		if err != nil {
			return BridgeEndpoint{}, nil, err
		}
		candidates = append(candidates, bridgeSessionCandidate{
			session: session,
			policy:  policy,
		})
	}
	return endpoint, candidates, nil
}

func (c *BridgeClient) sessionsByEndpoint(endpoint BridgeEndpoint, userId int) ([]bridge.SessionSnapshot, error) {
	if c == nil || c.Hub == nil {
		return nil, bridge.ErrClientUnavailable
	}
	preferredClientId := strings.TrimSpace(endpoint.ClientId)
	if endpoint.IsAuto() {
		preferredClientId = ""
	}
	sessions := c.Hub.SelectSessions(userId, preferredClientId, BridgeCapabilityMCPProxy)
	if len(sessions) == 0 {
		return nil, bridge.ErrClientNotFound
	}
	return sessions, nil
}

func (c *BridgeClient) forward(ctx context.Context, session bridge.SessionSnapshot, toolName string, args map[string]any, requestId string, userId int, tokenId int, policy bridgepolicy.Policy) (bridge.ToolCallResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(requestId)
	if id == "" {
		id = common.GetRandomString(32)
	}
	startedAt := time.Now()
	audit := createBridgeProxyAudit(id, session, toolName, args, userId, tokenId)
	if err := bridgepolicy.ValidateTool(policy, toolName); err != nil {
		updateBridgeProxyAuditError(audit, model.BridgeAuditStatusError, bridgeProxyErrorCode(err), err.Error(), int(time.Since(startedAt).Milliseconds()))
		return bridge.ToolCallResponse{Session: session}, err
	}
	if err := bridgepolicy.ValidateMCPTarget(policy, bridgeProxyTargetFromArgs(args)); err != nil {
		updateBridgeProxyAuditError(audit, model.BridgeAuditStatusError, bridgeProxyErrorCode(err), err.Error(), int(time.Since(startedAt).Milliseconds()))
		return bridge.ToolCallResponse{Session: session}, err
	}
	arguments := bridgepolicy.ApplyArgumentLimits(policy, toolName, args)
	response, err := c.Hub.ForwardToolCall(ctx, session.SessionId, bridge.ToolCallRequest{
		Id:        id,
		ToolName:  toolName,
		Arguments: arguments,
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
	if err := bridgepolicy.ValidateResultSize(policy, resultSize); err != nil {
		updateBridgeProxyAuditError(audit, model.BridgeAuditStatusError, bridgeProxyErrorCode(err), err.Error(), durationMS)
		return response, err
	}
	updateBridgeProxyAuditSuccess(audit, durationMS, resultSize)
	return response, nil
}

func (c *BridgeClient) forwardCandidates(ctx context.Context, candidates []bridgeSessionCandidate, toolName string, args map[string]any, requestId string, userId int, tokenId int) (bridge.ToolCallResponse, error) {
	var lastResponse bridge.ToolCallResponse
	var lastErr error
	for index, candidate := range candidates {
		response, err := c.forward(ctx, candidate.session, toolName, args, requestId, userId, tokenId, candidate.policy)
		if err == nil {
			return response, nil
		}
		lastResponse = response
		lastErr = err
		if index == len(candidates)-1 || !bridgeProxyShouldFailover(err) {
			return response, err
		}
	}
	return lastResponse, lastErr
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

func bridgeProxyTargetFromArgs(args map[string]any) string {
	if target := stringFromAny(args["target"]); target != "" {
		return target
	}
	endpoint := stringFromAny(args["endpoint"])
	if endpoint == "" {
		return ""
	}
	parsed, err := ParseBridgeEndpoint(endpoint)
	if err != nil {
		return ""
	}
	return parsed.Target
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

func bridgeProxyShouldFailover(err error) bool {
	switch bridgepolicy.ErrorCode(err) {
	case bridgepolicy.ErrorCodeToolNotAllowed, bridgepolicy.ErrorCodeWriteDisabled, bridgepolicy.ErrorCodeMCPTargetForbidden:
		return true
	}
	return errors.Is(err, bridge.ErrClientNotFound) ||
		errors.Is(err, bridge.ErrClientUnavailable) ||
		errors.Is(err, bridge.ErrClientDisconnected)
}

func bridgeProxyErrorCode(err error) string {
	if code := bridgepolicy.ErrorCode(err); code != "" {
		return code
	}
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
	if errors.Is(err, bridge.ErrClientUnavailable) {
		return "BRIDGE_CLIENT_UNAVAILABLE"
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
		if text == "" {
			continue
		}
		var object map[string]any
		if err := common.Unmarshal(common.StringToByteSlice(text), &object); err == nil {
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
	raw, err := common.Marshal(value)
	if err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return map[string]any{}
	}
	var object map[string]any
	if err := common.Unmarshal(raw, &object); err != nil {
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
	raw, err := common.Marshal(value)
	if err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return nil
	}
	var items []any
	if err := common.Unmarshal(raw, &items); err != nil {
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
