package dpagent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	BridgeCapabilityHTTPTunnel  = "http_tunnel"
	BridgeToolHTTPTunnelRequest = "http_tunnel.request"
	DefaultMaxResultBytes       = 512 * 1024
)

type ToolError struct {
	Code    string
	Message string
}

func (e ToolError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "tool call failed"
}

type httpTunnelArgs struct {
	Target           string         `json:"target"`
	Method           string         `json:"method"`
	Headers          map[string]any `json:"headers"`
	BodyBase64       string         `json:"body_base64"`
	MaxResponseBytes int64          `json:"max_response_bytes"`
	StreamResponse   bool           `json:"stream_response"`
	StreamRequest    bool           `json:"stream_request"`
	WebSocket        bool           `json:"websocket"`
}

func (c BridgeClient) handleToolCall(ctx context.Context, toolName string, args map[string]any) (dto.BridgeToolCallResult, error) {
	switch toolName {
	case BridgeToolHTTPTunnelRequest:
		return c.handleHTTPTunnelRequest(ctx, args)
	case BridgeToolMCPProxyTest, BridgeToolMCPProxyListTools, BridgeToolMCPProxyCallTool, BridgeToolMCPProxyRPC:
		return c.handleMCPProxy(ctx, toolName, args)
	case BridgeToolRemoteRead,
		BridgeToolRemoteTree,
		BridgeToolRemoteGlob,
		BridgeToolRemoteGrep,
		BridgeToolRemoteEnvInfo,
		BridgeToolRemoteGitStatus,
		BridgeToolRemoteGitDiff,
		BridgeToolRemoteGitLog,
		BridgeToolRemoteProjectInfo,
		BridgeToolRemoteGetRelatedFiles,
		BridgeToolRemoteWrite,
		BridgeToolRemoteEdit:
		return c.handleRemoteFileTool(ctx, toolName, args)
	case BridgeToolRemoteRunTests:
		return c.handleRemoteRunTests(ctx, args)
	case BridgeToolRemoteExec:
		return c.handleRemoteExec(ctx, args)
	case BridgeToolRemoteShellOpen:
		return c.handleRemoteShellOpen(ctx, args)
	case BridgeToolRemoteShellEval:
		return c.handleRemoteShellEval(ctx, args)
	case BridgeToolRemoteShellResize:
		return c.handleRemoteShellResize(ctx, args)
	case BridgeToolRemoteInstallPackage:
		return c.handleRemoteInstallPackage(ctx, args)
	default:
		return dto.BridgeToolCallResult{}, ToolError{
			Code:    "TOOL_NOT_SUPPORTED",
			Message: fmt.Sprintf("data-proxy-agent Go CLI has not implemented tool %q yet", toolName),
		}
	}
}

func (c BridgeClient) handleHTTPTunnelRequest(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	var input httpTunnelArgs
	if err := decodeBridgeData(args, &input); err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_INVALID_ARGUMENTS", Message: err.Error()}
	}
	if input.StreamResponse || input.WebSocket {
		return dto.BridgeToolCallResult{}, ToolError{
			Code:    "HTTP_TUNNEL_STREAM_NOT_IMPLEMENTED",
			Message: "data-proxy-agent Go CLI does not support streaming HTTP tunnel requests yet",
		}
	}
	target, err := allowedHTTPTarget(c.Config, input.Target)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if httpTunnelMethodMayHaveBody(method) && strings.TrimSpace(input.BodyBase64) != "" {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
		if err != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_INVALID_BODY", Message: err.Error()}
		}
		body = bytes.NewReader(decoded)
	}

	timeout := time.Duration(c.Config.Runtime.HTTPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultHTTPTimeoutMS * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(reqCtx, method, target, body)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_REQUEST_FAILED", Message: err.Error()}
	}
	for key, values := range normalizeTunnelRequestHeaders(input.Headers) {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	startedAt := time.Now()
	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		if reqCtx.Err() != nil {
			return dto.BridgeToolCallResult{}, ToolError{
				Code:    "HTTP_TUNNEL_TIMEOUT",
				Message: fmt.Sprintf("HTTP tunnel request timed out after %s", timeout),
			}
		}
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_REQUEST_FAILED", Message: err.Error()}
	}
	defer response.Body.Close()

	maxResponseBytes := input.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = DefaultMaxResultBytes
	}
	limited, err := readLimited(response.Body, maxResponseBytes)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_RESPONSE_READ_FAILED", Message: err.Error()}
	}
	payload := map[string]any{
		"status_code": response.StatusCode,
		"headers":     tunnelResponseHeaders(response.Header),
		"body_base64": base64.StdEncoding.EncodeToString(limited.Body),
		"truncated":   limited.Truncated,
		"target":      target,
	}
	if limited.Truncated {
		payload["headers"].(map[string][]string)["x-data-proxy-tunnel-truncated"] = []string{"true"}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_RESPONSE_ENCODE_FAILED", Message: err.Error()}
	}
	parsed, _ := url.Parse(target)
	summaryPath := "/"
	if parsed != nil && parsed.Path != "" {
		summaryPath = parsed.Path
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: string(payloadBytes)}},
		Summary:    fmt.Sprintf("%s %s -> %d", method, summaryPath, response.StatusCode),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(limited.Body),
		Metadata: map[string]any{
			"http_response": payload,
			"target":        target,
			"method":        method,
		},
	}, nil
}

type limitedReadResult struct {
	Body      []byte
	Truncated bool
}

func readLimited(reader io.Reader, maxBytes int64) (limitedReadResult, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResultBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return limitedReadResult{}, err
	}
	if int64(len(data)) > maxBytes {
		return limitedReadResult{Body: data[:maxBytes], Truncated: true}, nil
	}
	return limitedReadResult{Body: data}, nil
}

func allowedHTTPTarget(cfg Config, target string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("missing scheme or host")
		}
		return "", ToolError{Code: "HTTP_TUNNEL_INVALID_TARGET", Message: fmt.Sprintf("invalid HTTP tunnel target: %s", err)}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ToolError{Code: "HTTP_TUNNEL_INVALID_TARGET", Message: "only http/https HTTP tunnel targets are supported"}
	}
	if cfg.Policy.AllowNonLoopbackHTTP || isLoopbackHost(parsed.Hostname()) {
		return parsed.String(), nil
	}
	return "", ToolError{Code: "HTTP_TUNNEL_FORBIDDEN_TARGET", Message: "HTTP tunnel target must be loopback unless allow_non_loopback_http is enabled"}
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func httpTunnelMethodMayHaveBody(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead:
		return false
	default:
		return true
	}
}

func normalizeTunnelRequestHeaders(value map[string]any) map[string][]string {
	result := map[string][]string{}
	for key, raw := range value {
		key = strings.TrimSpace(key)
		if key == "" || hopByHopHeader(key) {
			continue
		}
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				result[key] = append(result[key], fmt.Sprint(item))
			}
		case []string:
			result[key] = append(result[key], typed...)
		case string:
			result[key] = append(result[key], typed)
		default:
			if raw != nil {
				result[key] = append(result[key], fmt.Sprint(raw))
			}
		}
	}
	return result
}

func tunnelResponseHeaders(headers http.Header) map[string][]string {
	result := map[string][]string{}
	for key, values := range headers {
		if hopByHopHeader(key) {
			continue
		}
		result[key] = append([]string(nil), values...)
	}
	return result
}

func hopByHopHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection",
		"proxy-connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade",
		"content-length",
		"host":
		return true
	default:
		return false
	}
}
