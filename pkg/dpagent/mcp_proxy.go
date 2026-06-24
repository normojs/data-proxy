package dpagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	BridgeCapabilityMCPProxy      = "mcp_proxy"
	BridgeToolMCPProxyTest        = "mcp_proxy.test"
	BridgeToolMCPProxyListTools   = "mcp_proxy.tools_list"
	BridgeToolMCPProxyCallTool    = "mcp_proxy.tools_call"
	BridgeToolMCPProxyRPC         = "mcp_proxy.rpc"
	DefaultMCPProxyClientName     = "data-proxy-agent"
	DefaultMCPProxyServerFallback = "local-mcp"
)

var mcpProxyNextID int64

var defaultMCPProxySessions = newMCPProxySessionCache()

type mcpProxyArgs struct {
	Transport string         `json:"transport"`
	Endpoint  string         `json:"endpoint"`
	Target    string         `json:"target"`
	Server    map[string]any `json:"server"`
}

type mcpProxyEndpoint struct {
	Key         string
	Transport   string
	Target      string
	StdioServer MCPServer
}

func (e mcpProxyEndpoint) DisplayTarget() string {
	if strings.TrimSpace(e.Target) != "" {
		return e.Target
	}
	if e.Transport == "stdio" {
		return "stdio:" + e.StdioServer.Name
	}
	return e.Key
}

func (c BridgeClient) handleMCPProxy(ctx context.Context, toolName string, args map[string]any) (dto.BridgeToolCallResult, error) {
	switch toolName {
	case BridgeToolMCPProxyTest:
		return c.handleMCPProxyTest(ctx, args)
	case BridgeToolMCPProxyListTools:
		return c.handleMCPProxyListTools(ctx, args)
	case BridgeToolMCPProxyCallTool:
		return c.handleMCPProxyCallTool(ctx, args)
	case BridgeToolMCPProxyRPC:
		return c.handleMCPProxyRPC(ctx, args)
	default:
		return dto.BridgeToolCallResult{}, ToolError{
			Code:    "MCP_PROXY_TOOL_NOT_SUPPORTED",
			Message: fmt.Sprintf("unsupported MCP proxy bridge tool: %s", toolName),
		}
	}
}

func (c BridgeClient) handleMCPProxyTest(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	endpoint, err := c.mcpEndpoint(args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	initialized, err := c.initializeMCP(ctx, endpoint, args, true)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	_, _ = c.mcpRPC(ctx, endpoint, initialized.SessionID, "ping", map[string]any{}, false)

	payload := map[string]any{
		"protocol_version": stringFromMap(initialized.Result, "protocolVersion", dto.MCPProtocolVersion),
		"server_name":      mcpServerName(initialized.Result, args),
		"capabilities":     mapFromAny(initialized.Result["capabilities"]),
	}
	return bridgeResult(payload, fmt.Sprintf("MCP %s ready", payload["server_name"]), time.Since(startedAt), map[string]any{
		"result":    payload,
		"target":    endpoint.DisplayTarget(),
		"transport": endpoint.Transport,
	})
}

func (c BridgeClient) handleMCPProxyListTools(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	endpoint, err := c.mcpEndpoint(args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	initialized, err := c.ensureMCPInitialized(ctx, endpoint, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	response, err := c.mcpRPC(ctx, endpoint, initialized.SessionID, "tools/list", map[string]any{}, false)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	tools, _ := response.Result["tools"].([]any)
	payload := map[string]any{"tools": tools}
	return bridgeResult(tools, fmt.Sprintf("%d tools discovered", len(tools)), time.Since(startedAt), map[string]any{
		"result":    payload,
		"target":    endpoint.DisplayTarget(),
		"transport": endpoint.Transport,
	})
}

func (c BridgeClient) handleMCPProxyCallTool(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	endpoint, err := c.mcpEndpoint(args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	toolName := stringFromMap(args, "name", "")
	if toolName == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "MCP_PROXY_INVALID_TOOL_NAME", Message: "MCP proxy tool name is required"}
	}
	arguments := mapFromAny(args["arguments"])
	initialized, err := c.ensureMCPInitialized(ctx, endpoint, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	response, err := c.mcpRPC(ctx, endpoint, initialized.SessionID, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": arguments,
	}, false)
	if err != nil {
		forgetMCPProxyEndpoint(endpoint)
		return dto.BridgeToolCallResult{}, err
	}
	content := mcpContentBlocksFromAny(response.Result["content"])
	if len(content) == 0 {
		content = []dto.MCPContentBlock{{Type: "text", Text: jsonText(response.RawResult)}}
	}
	summary := stringFromMap(response.Result, "summary", "")
	if summary == "" {
		summary = summarizeMCPContent(content, toolName)
	}
	metadata := mapFromAny(response.Result["metadata"])
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["target"] = endpoint.DisplayTarget()
	metadata["transport"] = endpoint.Transport
	metadata["tool_name"] = toolName
	resultSize := jsonSize(response.RawResult)
	if resultSize <= 0 {
		resultSize = mcpContentSize(content)
	}
	return dto.BridgeToolCallResult{
		Content:    content,
		Summary:    summary,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: resultSize,
		Metadata:   metadata,
	}, nil
}

func (c BridgeClient) handleMCPProxyRPC(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	endpoint, err := c.mcpEndpoint(args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	method := stringFromMap(args, "method", "")
	if method == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "MCP_PROXY_INVALID_METHOD", Message: "MCP proxy rpc method is required"}
	}
	params := mapFromAny(args["params"])
	initialized, err := c.ensureMCPInitialized(ctx, endpoint, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	response, err := c.mcpRPC(ctx, endpoint, initialized.SessionID, method, params, false)
	if err != nil {
		forgetMCPProxyEndpoint(endpoint)
		return dto.BridgeToolCallResult{}, err
	}
	raw := response.RawResult
	if raw == nil {
		raw = map[string]any{}
	}
	resultSize := jsonSize(raw)
	return dto.BridgeToolCallResult{
		Content: []dto.MCPContentBlock{{
			Type: "text",
			Text: jsonText(raw),
		}},
		Summary:    method + " forwarded",
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: resultSize,
		Metadata: map[string]any{
			"result":    raw,
			"target":    endpoint.DisplayTarget(),
			"transport": endpoint.Transport,
			"method":    method,
		},
	}, nil
}

type mcpRPCResponse struct {
	Result    map[string]any
	RawResult any
	SessionID string
}

type mcpProxySessionCache struct {
	mu       sync.Mutex
	sessions map[string]string
}

func newMCPProxySessionCache() *mcpProxySessionCache {
	return &mcpProxySessionCache{sessions: map[string]string{}}
}

func (c *mcpProxySessionCache) Get(target string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sessionID, ok := c.sessions[target]
	return sessionID, ok && strings.TrimSpace(sessionID) != ""
}

func (c *mcpProxySessionCache) Set(target string, sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[target] = sessionID
}

func (c *mcpProxySessionCache) Forget(target string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, target)
}

func (c BridgeClient) ensureMCPInitialized(ctx context.Context, endpoint mcpProxyEndpoint, args map[string]any) (mcpRPCResponse, error) {
	if endpoint.Transport == "stdio" {
		if defaultMCPStdioSessions.Initialized(endpoint.Key) {
			return mcpRPCResponse{Result: map[string]any{}, SessionID: endpoint.Key}, nil
		}
		return c.initializeMCP(ctx, endpoint, args, false)
	}
	if sessionID, ok := defaultMCPProxySessions.Get(endpoint.Key); ok {
		return mcpRPCResponse{Result: map[string]any{}, SessionID: sessionID}, nil
	}
	return c.initializeMCP(ctx, endpoint, args, false)
}

func (c BridgeClient) initializeMCP(ctx context.Context, endpoint mcpProxyEndpoint, args map[string]any, force bool) (mcpRPCResponse, error) {
	if endpoint.Transport == "stdio" {
		if force {
			defaultMCPStdioSessions.Forget(endpoint.Key)
		} else if defaultMCPStdioSessions.Initialized(endpoint.Key) {
			return mcpRPCResponse{Result: map[string]any{}, SessionID: endpoint.Key}, nil
		}
	}
	if !force {
		if sessionID, ok := defaultMCPProxySessions.Get(endpoint.Key); ok {
			return mcpRPCResponse{Result: map[string]any{}, SessionID: sessionID}, nil
		}
	}
	response, err := c.mcpRPC(ctx, endpoint, "", "initialize", map[string]any{
		"protocolVersion": dto.MCPProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    DefaultMCPProxyClientName,
			"version": agentVersion(c.Config),
		},
	}, false)
	if err != nil {
		return response, err
	}
	_, _ = c.mcpRPC(ctx, endpoint, response.SessionID, "notifications/initialized", map[string]any{}, true)
	if endpoint.Transport == "stdio" {
		defaultMCPStdioSessions.MarkInitialized(endpoint.Key)
	} else {
		defaultMCPProxySessions.Set(endpoint.Key, response.SessionID)
	}
	return response, nil
}

func (c BridgeClient) mcpRPC(ctx context.Context, endpoint mcpProxyEndpoint, sessionID string, method string, params map[string]any, notification bool) (mcpRPCResponse, error) {
	timeout := time.Duration(c.Config.Runtime.HTTPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultHTTPTimeoutMS * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := map[string]any{
		"jsonrpc": dto.MCPJSONRPCVersion,
		"method":  method,
		"params":  params,
	}
	if !notification {
		body["id"] = atomic.AddInt64(&mcpProxyNextID, 1)
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_ENCODE_FAILED", Message: err.Error()}
	}
	if endpoint.Transport == "stdio" {
		response, err := c.mcpStdioRPC(reqCtx, endpoint, bodyBytes, notification)
		if err != nil {
			defaultMCPStdioSessions.Forget(endpoint.Key)
		}
		return response, err
	}
	target := endpoint.Target
	request, err := http.NewRequestWithContext(reqCtx, http.MethodPost, target, bytes.NewReader(bodyBytes))
	if err != nil {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_HTTP_ERROR", Message: err.Error()}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", dto.MCPProtocolVersion)
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
	}

	client := http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		if reqCtx.Err() != nil {
			return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_TIMEOUT", Message: fmt.Sprintf("MCP proxy request timed out after %s", timeout)}
		}
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_HTTP_ERROR", Message: err.Error()}
	}
	defer response.Body.Close()
	nextSessionID := response.Header.Get("Mcp-Session-Id")
	textBytes, err := io.ReadAll(io.LimitReader(response.Body, DefaultMaxResultBytes+1))
	if err != nil {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_HTTP_ERROR", Message: err.Error()}
	}
	text := strings.TrimSpace(string(textBytes))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusNotFound && sessionID != "" {
			defaultMCPProxySessions.Forget(endpoint.Key)
		}
		return mcpRPCResponse{}, ToolError{
			Code:    "MCP_PROXY_HTTP_ERROR",
			Message: fmt.Sprintf("MCP upstream HTTP %d: %s", response.StatusCode, truncateString(text, 256)),
		}
	}
	if notification || text == "" {
		return mcpRPCResponse{Result: map[string]any{}, SessionID: nextSessionID}, nil
	}
	object, err := parseMCPResponseText(text)
	if err != nil {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_DECODE_FAILED", Message: err.Error()}
	}
	if errObject, ok := object["error"].(map[string]any); ok && len(errObject) > 0 {
		code := "MCP_PROXY_UPSTREAM_ERROR"
		if rawCode, ok := errObject["code"]; ok {
			code = "MCP_PROXY_UPSTREAM_" + sanitizeErrorCode(fmt.Sprint(rawCode))
		}
		return mcpRPCResponse{}, ToolError{Code: code, Message: stringFromMap(errObject, "message", "MCP upstream error")}
	}
	rawResult := object["result"]
	result := mapFromAny(rawResult)
	return mcpRPCResponse{Result: result, RawResult: rawResult, SessionID: nextSessionID}, nil
}

func (c BridgeClient) mcpEndpoint(args map[string]any) (mcpProxyEndpoint, error) {
	target := bridgeEndpointTarget(args)
	if endpoint, ok, err := c.httpMCPEndpoint(target); ok || err != nil {
		return endpoint, err
	}
	server, ok := c.localMCPServer(args, target)
	if ok {
		transport := normalizeMCPTransport(server.Transport, server.Endpoint, server.Command)
		if transport == "stdio" {
			if strings.TrimSpace(server.Command) == "" {
				return mcpProxyEndpoint{}, ToolError{Code: "MCP_PROXY_STDIO_NOT_CONFIGURED", Message: fmt.Sprintf("local MCP server %q has no command", server.Name)}
			}
			return mcpProxyEndpoint{
				Key:         "stdio:" + server.Name,
				Transport:   "stdio",
				StdioServer: server,
			}, nil
		}
		if endpoint, ok, err := c.httpMCPEndpoint(server.Endpoint); ok || err != nil {
			return endpoint, err
		}
	}
	transport := strings.TrimSpace(strings.ToLower(stringFromMap(args, "transport", "")))
	if transport == "stdio" || target != "" {
		name := target
		if name == "" {
			name = stringFromMap(mapFromAny(args["server"]), "name", "")
		}
		return mcpProxyEndpoint{}, ToolError{Code: "MCP_PROXY_STDIO_NOT_CONFIGURED", Message: fmt.Sprintf("stdio MCP server %q is not configured locally", name)}
	}
	return mcpProxyEndpoint{}, ToolError{Code: "MCP_PROXY_INVALID_TARGET", Message: "MCP proxy target is required"}
}

func (c BridgeClient) httpMCPEndpoint(target string) (mcpProxyEndpoint, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return mcpProxyEndpoint{}, false, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return mcpProxyEndpoint{}, false, ToolError{Code: "MCP_PROXY_INVALID_TARGET", Message: "only http/https MCP targets are supported by this agent"}
	}
	if c.Config.Policy.AllowNonLoopbackMCP || isLoopbackHost(parsed.Hostname()) {
		target := parsed.String()
		return mcpProxyEndpoint{Key: "http:" + target, Transport: "http", Target: target}, true, nil
	}
	return mcpProxyEndpoint{}, true, ToolError{Code: "MCP_PROXY_FORBIDDEN_TARGET", Message: "MCP proxy target must be loopback unless allow_non_loopback_mcp is enabled"}
}

func (c BridgeClient) localMCPServer(args map[string]any, target string) (MCPServer, bool) {
	serverArgs := mapFromAny(args["server"])
	candidates := []string{
		strings.TrimSpace(target),
		stringFromMap(serverArgs, "name", ""),
		stringFromMap(serverArgs, "namespace", ""),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if server, ok := findMCPServer(c.Config.MCPServers, candidate); ok {
			return server, true
		}
	}
	return MCPServer{}, false
}

func forgetMCPProxyEndpoint(endpoint mcpProxyEndpoint) {
	if endpoint.Transport == "stdio" {
		defaultMCPStdioSessions.Forget(endpoint.Key)
		return
	}
	defaultMCPProxySessions.Forget(endpoint.Key)
}

func bridgeEndpointTarget(args map[string]any) string {
	if target := stringFromMap(args, "target", ""); strings.TrimSpace(target) != "" {
		return strings.TrimSpace(target)
	}
	endpoint := stringFromMap(args, "endpoint", "")
	if endpoint == "" {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	if target := parsed.Query().Get("target"); strings.TrimSpace(target) != "" {
		return strings.TrimSpace(target)
	}
	pathTarget, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err == nil && strings.TrimSpace(pathTarget) != "" {
		return strings.TrimSpace(pathTarget)
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return parsed.String()
	}
	switch parsed.Scheme {
	case "bridge", "qidian", "qidian_browser":
		return ""
	}
	return endpoint
}

func parseMCPResponseText(text string) (map[string]any, error) {
	trimmed := strings.TrimSpace(text)
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			break
		}
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
		return nil, err
	}
	return object, nil
}

func bridgeResult(value any, summary string, duration time.Duration, metadata map[string]any) (dto.BridgeToolCallResult, error) {
	text := ""
	switch typed := value.(type) {
	case string:
		text = typed
	default:
		bytes, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "BRIDGE_RESULT_ENCODE_FAILED", Message: err.Error()}
		}
		text = string(bytes)
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    summary,
		DurationMS: int(duration.Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata:   metadata,
	}, nil
}

func mcpServerName(result map[string]any, args map[string]any) string {
	if serverInfo := mapFromAny(result["serverInfo"]); len(serverInfo) > 0 {
		if name := stringFromMap(serverInfo, "name", ""); name != "" {
			return name
		}
	}
	if server := mapFromAny(args["server"]); len(server) > 0 {
		if name := stringFromMap(server, "name", ""); name != "" {
			return name
		}
	}
	return DefaultMCPProxyServerFallback
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(bytes, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func mcpContentBlocksFromAny(value any) []dto.MCPContentBlock {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]dto.MCPContentBlock); ok {
		return typed
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var blocks []dto.MCPContentBlock
	if err := json.Unmarshal(bytes, &blocks); err != nil {
		return nil
	}
	return blocks
}

func summarizeMCPContent(content []dto.MCPContentBlock, fallback string) string {
	for _, block := range content {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		if len(text) > 160 {
			return text[:160]
		}
		return text
	}
	return fallback
}

func mcpContentSize(content []dto.MCPContentBlock) int {
	bytes, err := json.Marshal(content)
	if err != nil {
		return 0
	}
	return len(bytes)
}

func jsonText(value any) string {
	if value == nil {
		return "{}"
	}
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(bytes)
}

func jsonSize(value any) int {
	if value == nil {
		return 0
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(bytes)
}

func stringFromMap(value map[string]any, key string, fallback string) string {
	if value == nil {
		return fallback
	}
	raw, ok := value[key]
	if !ok || raw == nil {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" {
		return fallback
	}
	return text
}

func sanitizeErrorCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ERROR"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == ':':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

func truncateString(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
