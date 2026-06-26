package dpagent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gorilla/websocket"
)

const (
	httpTunnelStreamChunkBytes = 32 * 1024

	tunnelWebSocketFrameText   = "text"
	tunnelWebSocketFrameBinary = "binary"
	tunnelWebSocketFrameClose  = "close"
	tunnelWebSocketFramePing   = "ping"
	tunnelWebSocketFramePong   = "pong"
)

type bridgeStreamChunkEmitter func(dto.BridgeToolStreamChunk) error

func httpTunnelRequiresStream(args map[string]any) bool {
	return boolFromMap(args, "stream_response") || boolFromMap(args, "stream_request") || boolFromMap(args, "websocket")
}

func (c BridgeClient) handleHTTPTunnelStreamRequest(ctx context.Context, args map[string]any, emit bridgeStreamChunkEmitter, inputQueue *bridgeStreamInputQueue) (dto.BridgeToolCallResult, error) {
	if emit == nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_STREAM_EMITTER_MISSING", Message: "HTTP tunnel stream emitter is required"}
	}
	var input httpTunnelArgs
	if err := decodeBridgeData(args, &input); err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_INVALID_ARGUMENTS", Message: err.Error()}
	}
	if input.WebSocket {
		return c.handleHTTPTunnelWebSocket(ctx, input, emit, inputQueue)
	}
	return c.handleHTTPTunnelHTTPStream(ctx, input, emit, inputQueue)
}

func (c BridgeClient) handleHTTPTunnelHTTPStream(ctx context.Context, input httpTunnelArgs, emit bridgeStreamChunkEmitter, inputQueue *bridgeStreamInputQueue) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	target, err := allowedHTTPTarget(c.Config, input.Target)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if input.StreamRequest && httpTunnelMethodMayHaveBody(method) {
		if inputQueue == nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_STREAM_INPUT_MISSING", Message: "HTTP tunnel stream_request requires stream input"}
		}
		body = httpTunnelRequestBodyReader(ctx, inputQueue)
	} else if httpTunnelMethodMayHaveBody(method) && strings.TrimSpace(input.BodyBase64) != "" {
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

	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		if reqCtx.Err() != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_TIMEOUT", Message: fmt.Sprintf("HTTP tunnel request timed out after %s", timeout)}
		}
		var toolErr ToolError
		if errors.As(err, &toolErr) {
			return dto.BridgeToolCallResult{}, toolErr
		}
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_REQUEST_FAILED", Message: err.Error()}
	}
	defer response.Body.Close()

	if err := emit(dto.BridgeToolStreamChunk{
		StatusCode: response.StatusCode,
		Headers:    tunnelStreamHeaders(response.Header),
		Metadata: map[string]any{
			"target": target,
			"method": method,
		},
	}); err != nil {
		return dto.BridgeToolCallResult{}, err
	}

	maxResponseBytes := input.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = DefaultMaxResultBytes
	}
	buffer := make([]byte, httpTunnelStreamChunkBytes)
	var bytesOut int64
	truncated := false
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			if bytesOut+int64(len(chunk)) > maxResponseBytes {
				remaining := maxResponseBytes - bytesOut
				if remaining < 0 {
					remaining = 0
				}
				chunk = chunk[:remaining]
				truncated = true
			}
			if len(chunk) > 0 {
				bytesOut += int64(len(chunk))
				if err := emit(dto.BridgeToolStreamChunk{
					BodyBase64: base64.StdEncoding.EncodeToString(chunk),
					Bytes:      len(chunk),
				}); err != nil {
					return dto.BridgeToolCallResult{}, err
				}
			}
			if truncated {
				break
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_RESPONSE_READ_FAILED", Message: readErr.Error()}
		}
	}
	if err := emit(dto.BridgeToolStreamChunk{
		Done:      true,
		Truncated: truncated,
		Bytes:     int(bytesOut),
	}); err != nil {
		return dto.BridgeToolCallResult{}, err
	}

	payload := map[string]any{
		"status_code":    response.StatusCode,
		"headers":        tunnelResponseHeaders(response.Header),
		"body_base64":    "",
		"streamed":       true,
		"stream_request": input.StreamRequest,
		"truncated":      truncated,
		"bytes":          bytesOut,
		"target":         target,
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
		Summary:    fmt.Sprintf("%s %s -> %d streamed %d bytes", method, summaryPath, response.StatusCode, bytesOut),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: int(bytesOut),
		Metadata: map[string]any{
			"http_response":  payload,
			"target":         target,
			"method":         method,
			"streamed":       true,
			"stream_request": input.StreamRequest,
		},
	}, nil
}

func httpTunnelRequestBodyReader(ctx context.Context, queue *bridgeStreamInputQueue) io.Reader {
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		for {
			input, ok := queue.Next(ctx)
			if !ok {
				_ = writer.CloseWithError(ctx.Err())
				return
			}
			if input.ErrorCode != "" || input.ErrorMessage != "" {
				message := input.ErrorMessage
				if message == "" {
					message = input.ErrorCode
				}
				_ = writer.CloseWithError(ToolError{Code: normalizeErrorCode(input.ErrorCode, "HTTP_TUNNEL_REQUEST_BODY_FAILED"), Message: message})
				return
			}
			if input.Done {
				return
			}
			frameType := strings.TrimSpace(input.FrameType)
			if frameType != "" && frameType != "http_request_body" {
				continue
			}
			if strings.TrimSpace(input.BodyBase64) == "" {
				continue
			}
			body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
			if err != nil {
				_ = writer.CloseWithError(ToolError{Code: "HTTP_TUNNEL_INVALID_STREAM_BODY", Message: err.Error()})
				return
			}
			if len(body) == 0 {
				continue
			}
			if _, err := writer.Write(body); err != nil {
				_ = writer.CloseWithError(err)
				return
			}
		}
	}()
	return reader
}

func tunnelStreamHeaders(headers http.Header) map[string]any {
	result := map[string]any{}
	for key, values := range headers {
		if hopByHopHeader(key) {
			continue
		}
		result[key] = append([]string(nil), values...)
	}
	return result
}

func boolFromMap(value map[string]any, key string) bool {
	if value == nil {
		return false
	}
	switch typed := value[key].(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return fmt.Sprint(typed) == "true"
	}
}

func normalizeErrorCode(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return sanitizeErrorCode(value)
}

func (c BridgeClient) handleHTTPTunnelWebSocket(ctx context.Context, input httpTunnelArgs, emit bridgeStreamChunkEmitter, inputQueue *bridgeStreamInputQueue) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	if inputQueue == nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_STREAM_INPUT_MISSING", Message: "HTTP tunnel websocket requires stream input"}
	}
	target, err := allowedHTTPWebSocketTarget(c.Config, input.Target)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	timeout := time.Duration(c.Config.Runtime.HTTPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultHTTPTimeoutMS * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	headers := normalizeTunnelWebSocketHeaders(input.Headers)
	conn, response, err := websocket.DefaultDialer.DialContext(reqCtx, target, headers)
	if err != nil {
		if reqCtx.Err() != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_TIMEOUT", Message: fmt.Sprintf("WebSocket tunnel connect timed out after %s", timeout)}
		}
		status := ""
		if response != nil {
			status = fmt.Sprintf(" (HTTP %d)", response.StatusCode)
		}
		return dto.BridgeToolCallResult{}, ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_FAILED", Message: err.Error() + status}
	}
	defer conn.Close()
	defer inputQueue.Close()

	if err := emit(dto.BridgeToolStreamChunk{
		StatusCode: http.StatusSwitchingProtocols,
		Headers:    map[string]any{},
		Metadata: map[string]any{
			"target":    target,
			"websocket": true,
		},
	}); err != nil {
		return dto.BridgeToolCallResult{}, err
	}

	inputErr := make(chan error, 1)
	go func() {
		inputErr <- forwardWebSocketStreamInput(ctx, conn, inputQueue)
	}()

	var bytesOut int
	closeCode := websocket.CloseNormalClosure
	closeReason := ""
	for {
		messageType, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			var closeErr *websocket.CloseError
			if errors.As(readErr, &closeErr) {
				closeCode = closeErr.Code
				closeReason = closeErr.Text
			} else {
				closeCode = websocket.CloseInternalServerErr
				closeReason = readErr.Error()
			}
			break
		}
		frameType := tunnelWebSocketFrameBinary
		if messageType == websocket.TextMessage {
			frameType = tunnelWebSocketFrameText
		}
		bytesOut += len(payload)
		if err := emit(dto.BridgeToolStreamChunk{
			FrameType:  frameType,
			BodyBase64: base64.StdEncoding.EncodeToString(payload),
			Bytes:      len(payload),
		}); err != nil {
			return dto.BridgeToolCallResult{}, err
		}
		select {
		case err := <-inputErr:
			if err != nil {
				return dto.BridgeToolCallResult{}, err
			}
			inputErr = nil
		default:
		}
	}

	if err := emit(dto.BridgeToolStreamChunk{
		FrameType:   tunnelWebSocketFrameClose,
		Done:        true,
		CloseCode:   closeCode,
		CloseReason: closeReason,
		Bytes:       bytesOut,
	}); err != nil {
		return dto.BridgeToolCallResult{}, err
	}

	payload := map[string]any{
		"status_code": http.StatusSwitchingProtocols,
		"headers":     map[string][]string{},
		"body_base64": "",
		"streamed":    true,
		"websocket":   true,
		"bytes_out":   bytesOut,
		"target":      target,
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
		Summary:    fmt.Sprintf("WEBSOCKET %s streamed %d bytes", summaryPath, bytesOut),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: bytesOut,
		Metadata: map[string]any{
			"http_response": payload,
			"target":        target,
			"method":        "WEBSOCKET",
			"streamed":      true,
			"websocket":     true,
		},
	}, nil
}

func forwardWebSocketStreamInput(ctx context.Context, conn *websocket.Conn, queue *bridgeStreamInputQueue) error {
	for {
		input, ok := queue.Next(ctx)
		if !ok {
			return nil
		}
		if input.ErrorCode != "" || input.ErrorMessage != "" {
			message := input.ErrorMessage
			if message == "" {
				message = input.ErrorCode
			}
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, message), time.Now().Add(time.Second))
			return ToolError{Code: normalizeErrorCode(input.ErrorCode, "HTTP_TUNNEL_WEBSOCKET_INPUT_FAILED"), Message: message}
		}
		frameType := tunnelWebSocketFrameType(input.FrameType)
		if input.Done || frameType == tunnelWebSocketFrameClose {
			closeCode := input.CloseCode
			if closeCode <= 0 {
				closeCode = websocket.CloseNormalClosure
			}
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, truncateString(input.CloseReason, 120)), time.Now().Add(time.Second))
			return nil
		}
		body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
		if err != nil {
			return ToolError{Code: "HTTP_TUNNEL_INVALID_WEBSOCKET_BODY", Message: err.Error()}
		}
		switch frameType {
		case tunnelWebSocketFrameText:
			if err := conn.WriteMessage(websocket.TextMessage, body); err != nil {
				return ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_WRITE_FAILED", Message: err.Error()}
			}
		case tunnelWebSocketFramePing:
			if err := conn.WriteControl(websocket.PingMessage, body, time.Now().Add(time.Second)); err != nil {
				return ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_WRITE_FAILED", Message: err.Error()}
			}
		case tunnelWebSocketFramePong:
			if err := conn.WriteControl(websocket.PongMessage, body, time.Now().Add(time.Second)); err != nil {
				return ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_WRITE_FAILED", Message: err.Error()}
			}
		default:
			if err := conn.WriteMessage(websocket.BinaryMessage, body); err != nil {
				return ToolError{Code: "HTTP_TUNNEL_WEBSOCKET_WRITE_FAILED", Message: err.Error()}
			}
		}
	}
}

func allowedHTTPWebSocketTarget(cfg Config, target string) (string, error) {
	allowed, err := allowedHTTPTarget(cfg, target)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(allowed)
	if err != nil {
		return "", ToolError{Code: "HTTP_TUNNEL_INVALID_TARGET", Message: err.Error()}
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", ToolError{Code: "HTTP_TUNNEL_INVALID_TARGET", Message: "only http/https WebSocket tunnel targets are supported"}
	}
	return parsed.String(), nil
}

func normalizeTunnelWebSocketHeaders(value map[string]any) http.Header {
	headers := http.Header{}
	for key, values := range normalizeTunnelRequestHeaders(value) {
		if strings.HasPrefix(strings.ToLower(key), "sec-websocket-") {
			continue
		}
		for _, item := range values {
			headers.Add(key, item)
		}
	}
	return headers
}

func tunnelWebSocketFrameType(frameType string) string {
	switch strings.ToLower(strings.TrimSpace(frameType)) {
	case tunnelWebSocketFrameText:
		return tunnelWebSocketFrameText
	case tunnelWebSocketFramePing:
		return tunnelWebSocketFramePing
	case tunnelWebSocketFramePong:
		return tunnelWebSocketFramePong
	case tunnelWebSocketFrameClose:
		return tunnelWebSocketFrameClose
	case tunnelWebSocketFrameBinary:
		return tunnelWebSocketFrameBinary
	default:
		return tunnelWebSocketFrameBinary
	}
}
