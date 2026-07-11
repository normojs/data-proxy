package dpagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gorilla/websocket"
)

func TestHandleHTTPTunnelRequestBasic(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-Agent-Test") != "yes" {
			t.Fatalf("missing forwarded header: %#v", r.Header)
		}
		if r.Header.Get("Connection") != "" {
			t.Fatalf("hop-by-hop header was forwarded: %#v", r.Header)
		}
		w.Header().Set("X-Local", "ok")
		_, _ = w.Write([]byte("hello agent"))
	}))
	defer local.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	result, err := client.handleHTTPTunnelRequest(context.Background(), map[string]any{
		"target":             local.URL + "/echo",
		"method":             "POST",
		"headers":            map[string]any{"X-Agent-Test": []any{"yes"}, "Connection": "close"},
		"body_base64":        base64.StdEncoding.EncodeToString([]byte("input")),
		"max_response_bytes": 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResultSize != len("hello agent") {
		t.Fatalf("unexpected result size: %d", result.ResultSize)
	}
	payload := httpPayloadFromResult(t, result)
	if payload["status_code"].(float64) != 200 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	body, err := base64.StdEncoding.DecodeString(payload["body_base64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello agent" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleHTTPTunnelRequestRejectsNonLoopbackByDefault(t *testing.T) {
	cfg := DefaultConfig()
	client := BridgeClient{Config: cfg}
	_, err := client.handleHTTPTunnelRequest(context.Background(), map[string]any{
		"target": "http://example.com",
		"method": "GET",
	})
	if err == nil {
		t.Fatal("expected non-loopback target to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok {
		t.Fatalf("expected ToolError, got %T", err)
	}
	if toolErr.Code != "HTTP_TUNNEL_FORBIDDEN_TARGET" {
		t.Fatalf("unexpected error: %#v", toolErr)
	}
}

func TestHandleHTTPTunnelRequestTruncatesResponse(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("abcdef"))
	}))
	defer local.Close()

	cfg := DefaultConfig()
	client := BridgeClient{Config: cfg}
	result, err := client.handleHTTPTunnelRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "GET",
		"max_response_bytes": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := httpPayloadFromResult(t, result)
	body, err := base64.StdEncoding.DecodeString(payload["body_base64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "abc" || payload["truncated"] != true {
		t.Fatalf("response was not truncated: %#v body=%q", payload, body)
	}
}

func TestHandleHTTPTunnelRequestStreamRequiresBridgeEmitter(t *testing.T) {
	cfg := DefaultConfig()
	client := BridgeClient{Config: cfg}
	_, err := client.handleHTTPTunnelRequest(context.Background(), map[string]any{
		"target":          "http://127.0.0.1:1",
		"method":          "GET",
		"stream_response": true,
	})
	if err == nil {
		t.Fatal("expected stream response to require bridge streaming context")
	}
	var toolErr ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected ToolError, got %T: %v", err, err)
	}
	if toolErr.Code != "HTTP_TUNNEL_STREAM_EMITTER_MISSING" {
		t.Fatalf("unexpected error: %#v", toolErr)
	}
}

func TestHandleHTTPTunnelStreamResponse(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Stream", "yes")
		_, _ = w.Write([]byte("one"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("two"))
	}))
	defer local.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	var chunks []dto.BridgeToolStreamChunk
	result, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "GET",
		"stream_response":    true,
		"max_response_bytes": 1024,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected header/body/done chunks, got %#v", chunks)
	}
	if chunks[0].StatusCode != http.StatusOK {
		t.Fatalf("unexpected header chunk: %#v", chunks[0])
	}
	body := joinedStreamBody(t, chunks)
	if body != "onetwo" {
		t.Fatalf("unexpected streamed body: %q", body)
	}
	if !chunks[len(chunks)-1].Done {
		t.Fatalf("missing done chunk: %#v", chunks)
	}
	payload := httpPayloadFromResult(t, result)
	if payload["streamed"] != true {
		t.Fatalf("unexpected result payload: %#v", payload)
	}
}

func TestHandleHTTPTunnelStreamLargeResponseChunks(t *testing.T) {
	payload := strings.Repeat("x", httpTunnelStreamChunkBytes*3+123)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(payload))
	}))
	defer local.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	var chunks []dto.BridgeToolStreamChunk
	result, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "GET",
		"stream_response":    true,
		"max_response_bytes": len(payload),
	}, func(chunk dto.BridgeToolStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var bodyChunkCount int
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.BodyBase64) != "" {
			bodyChunkCount++
		}
	}
	if bodyChunkCount < 3 {
		t.Fatalf("expected multiple body chunks, got %d chunks=%#v", bodyChunkCount, chunks)
	}
	if body := joinedStreamBody(t, chunks); body != payload {
		t.Fatalf("unexpected streamed body length: got %d want %d", len(body), len(payload))
	}
	if result.ResultSize != len(payload) {
		t.Fatalf("unexpected result size: got %d want %d", result.ResultSize, len(payload))
	}
	response := httpPayloadFromResult(t, result)
	if response["streamed"] != true || response["truncated"] == true {
		t.Fatalf("unexpected large stream payload: %#v", response)
	}
}

func TestHandleHTTPTunnelStreamEmitterFailureReturns(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("chunk-one"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("chunk-two"))
	}))
	defer local.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	emitErr := errors.New("bridge stream emit failed")
	var emitCalls int
	_, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "GET",
		"stream_response":    true,
		"max_response_bytes": 1024,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		emitCalls++
		if strings.TrimSpace(chunk.BodyBase64) != "" {
			return emitErr
		}
		return nil
	}, nil)
	if !errors.Is(err, emitErr) {
		t.Fatalf("expected emitter error, got %v", err)
	}
	if emitCalls < 2 {
		t.Fatalf("expected header and body emit attempts, got %d", emitCalls)
	}
}

func TestHandleHTTPTunnelStreamRequestBody(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("echo:" + string(body)))
	}))
	defer local.Close()

	queue := newBridgeStreamInputQueue()
	if !queue.Push(dto.BridgeToolStreamInput{
		FrameType:  "http_request_body",
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("uploaded")),
	}) {
		t.Fatal("failed to queue request body chunk")
	}
	if !queue.Push(dto.BridgeToolStreamInput{FrameType: "http_request_body", Done: true}) {
		t.Fatal("failed to queue request body done")
	}

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	var chunks []dto.BridgeToolStreamChunk
	result, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "POST",
		"stream_response":    true,
		"stream_request":     true,
		"max_response_bytes": 1024,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}, queue)
	if err != nil {
		t.Fatal(err)
	}
	if body := joinedStreamBody(t, chunks); body != "echo:uploaded" {
		t.Fatalf("unexpected streamed body: %q", body)
	}
	payload := httpPayloadFromResult(t, result)
	if payload["stream_request"] != true {
		t.Fatalf("unexpected result payload: %#v", payload)
	}
}

func TestHandleHTTPTunnelStreamRequestBodyErrorFrame(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err == nil {
			t.Fatal("expected request body read to fail")
		}
	}))
	defer local.Close()

	queue := newBridgeStreamInputQueue()
	if !queue.Push(dto.BridgeToolStreamInput{
		FrameType:    "http_request_body",
		ErrorCode:    "CLIENT_ABORTED",
		ErrorMessage: "client upload aborted",
	}) {
		t.Fatal("failed to queue request body error")
	}

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	_, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":             local.URL,
		"method":             "POST",
		"stream_response":    true,
		"stream_request":     true,
		"max_response_bytes": 1024,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		t.Fatalf("unexpected stream chunk: %#v", chunk)
		return nil
	}, queue)
	if err == nil {
		t.Fatal("expected stream request body error")
	}
	var toolErr ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected ToolError, got %T: %v", err, err)
	}
	if toolErr.Code != "CLIENT_ABORTED" {
		t.Fatalf("unexpected tool error: %#v", toolErr)
	}
	if !strings.Contains(toolErr.Message, "client upload aborted") {
		t.Fatalf("unexpected tool error message: %#v", toolErr)
	}
}

func TestHandleHTTPTunnelWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteMessage(messageType, append([]byte("echo:"), payload...)); err != nil {
			t.Fatal(err)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), nowPlusSecond())
	}))
	defer local.Close()

	queue := newBridgeStreamInputQueue()
	if !queue.Push(dto.BridgeToolStreamInput{
		FrameType:  "text",
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
	}) {
		t.Fatal("failed to queue websocket input")
	}

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	var chunks []dto.BridgeToolStreamChunk
	result, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":          local.URL + "/ws",
		"method":          "GET",
		"stream_response": true,
		"websocket":       true,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}, queue)
	if err != nil {
		t.Fatal(err)
	}
	if chunks[0].StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("unexpected websocket open chunk: %#v", chunks[0])
	}
	if body := joinedStreamBody(t, chunks); body != "echo:hello" {
		t.Fatalf("unexpected websocket body: %q", body)
	}
	if chunks[len(chunks)-1].FrameType != "close" || !chunks[len(chunks)-1].Done {
		t.Fatalf("missing websocket close chunk: %#v", chunks)
	}
	payload := httpPayloadFromResult(t, result)
	if payload["websocket"] != true {
		t.Fatalf("unexpected websocket payload: %#v", payload)
	}
}

func TestHandleHTTPTunnelWebSocketBinaryAndPing(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	pingReceived := make(chan string, 1)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetPingHandler(func(appData string) error {
			pingReceived <- appData
			return conn.WriteControl(websocket.PongMessage, []byte(appData), nowPlusSecond())
		})
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if messageType != websocket.BinaryMessage {
			t.Fatalf("expected binary websocket message, got %d", messageType)
		}
		if string(payload) != "bin-input" {
			t.Fatalf("unexpected websocket payload: %q", payload)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte("bin-output")); err != nil {
			t.Fatal(err)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), nowPlusSecond())
	}))
	defer local.Close()

	queue := newBridgeStreamInputQueue()
	if !queue.Push(dto.BridgeToolStreamInput{
		FrameType:  tunnelWebSocketFramePing,
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("pulse")),
	}) {
		t.Fatal("failed to queue websocket ping")
	}
	if !queue.Push(dto.BridgeToolStreamInput{
		FrameType:  tunnelWebSocketFrameBinary,
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("bin-input")),
	}) {
		t.Fatal("failed to queue websocket binary input")
	}

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	var chunks []dto.BridgeToolStreamChunk
	result, err := client.handleHTTPTunnelStreamRequest(context.Background(), map[string]any{
		"target":          local.URL + "/ws",
		"method":          "GET",
		"stream_response": true,
		"websocket":       true,
	}, func(chunk dto.BridgeToolStreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}, queue)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-pingReceived:
		if value != "pulse" {
			t.Fatalf("unexpected ping payload: %q", value)
		}
	default:
		t.Fatal("expected local websocket server to receive ping")
	}
	if body := joinedStreamBody(t, chunks); body != "bin-output" {
		t.Fatalf("unexpected websocket body: %q chunks=%#v", body, chunks)
	}
	var sawBinary bool
	for _, chunk := range chunks {
		if chunk.FrameType == tunnelWebSocketFrameBinary {
			sawBinary = true
			break
		}
	}
	if !sawBinary {
		t.Fatalf("missing binary output frame: %#v", chunks)
	}
	payload := httpPayloadFromResult(t, result)
	if payload["websocket"] != true {
		t.Fatalf("unexpected websocket payload: %#v", payload)
	}
}

func TestBridgeRunOnceHandlesHTTPTunnelToolCall(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Bridge", "ok")
		_, _ = w.Write([]byte("bridge-ok"))
	}))
	defer local.Close()

	server := newBridgeTestServer(t, func(t *testing.T, conn *websocket.Conn) {
		_ = readBridgeTestMessage(t, conn)
		if err := conn.WriteJSON(dto.BridgeWSMessage{Type: "registered", Data: map[string]any{
			"session_id": "sess-1",
			"client_id":  "test-agent",
		}}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{
			Type: "tool_call",
			Id:   "req-http",
			Data: dto.BridgeToolCallRequest{
				RequestId: "req-http",
				ToolName:  BridgeToolHTTPTunnelRequest,
				Arguments: map[string]any{
					"target":             local.URL,
					"method":             "GET",
					"max_response_bytes": 1024,
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
		msg := readBridgeTestMessage(t, conn)
		if msg.Type != "tool_result" {
			t.Fatalf("expected tool_result, got %s: %#v", msg.Type, msg.Data)
		}
		var result dto.BridgeToolCallResult
		if err := decodeBridgeData(msg.Data, &result); err != nil {
			t.Fatal(err)
		}
		payload := httpPayloadFromResult(t, result)
		body, err := base64.StdEncoding.DecodeString(payload["body_base64"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "bridge-ok" {
			t.Fatalf("unexpected bridge body: %q", body)
		}
		_ = conn.WriteJSON(dto.BridgeWSMessage{Type: "close"})
	})
	defer server.Close()

	cfg := bridgeTestConfig(server.URL)
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local", Target: local.URL}}
	client := BridgeClient{Config: cfg}
	if _, err := client.runOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestBridgeRunOnceHandlesHTTPTunnelStreamInput(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("stream:" + string(body)))
	}))
	defer local.Close()

	server := newBridgeTestServer(t, func(t *testing.T, conn *websocket.Conn) {
		_ = readBridgeTestMessage(t, conn)
		if err := conn.WriteJSON(dto.BridgeWSMessage{Type: "registered", Data: map[string]any{
			"session_id": "sess-1",
			"client_id":  "test-agent",
		}}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{
			Type: "tool_call",
			Id:   "req-stream",
			Data: dto.BridgeToolCallRequest{
				RequestId: "req-stream",
				ToolName:  BridgeToolHTTPTunnelRequest,
				Arguments: map[string]any{
					"target":             local.URL,
					"method":             "POST",
					"stream_response":    true,
					"stream_request":     true,
					"max_response_bytes": 1024,
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{
			Type: bridgeMessageTypeToolStreamInput,
			Id:   "req-stream",
			Data: dto.BridgeToolStreamInput{
				FrameType:  "http_request_body",
				BodyBase64: base64.StdEncoding.EncodeToString([]byte("uploaded")),
			},
		}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{
			Type: bridgeMessageTypeToolStreamInput,
			Id:   "req-stream",
			Data: dto.BridgeToolStreamInput{
				FrameType: "http_request_body",
				Done:      true,
			},
		}); err != nil {
			t.Fatal(err)
		}

		var chunks []dto.BridgeToolStreamChunk
		for {
			msg := readBridgeTestMessage(t, conn)
			switch msg.Type {
			case bridgeMessageTypeToolStreamChunk:
				var chunk dto.BridgeToolStreamChunk
				if err := decodeBridgeData(msg.Data, &chunk); err != nil {
					t.Fatal(err)
				}
				chunks = append(chunks, chunk)
			case "tool_result":
				if body := joinedStreamBody(t, chunks); body != "stream:uploaded" {
					t.Fatalf("unexpected stream body: %q chunks=%#v", body, chunks)
				}
				var result dto.BridgeToolCallResult
				if err := decodeBridgeData(msg.Data, &result); err != nil {
					t.Fatal(err)
				}
				payload := httpPayloadFromResult(t, result)
				if payload["stream_request"] != true {
					t.Fatalf("unexpected result payload: %#v", payload)
				}
				_ = conn.WriteJSON(dto.BridgeWSMessage{Type: "close"})
				return
			default:
				t.Fatalf("unexpected message: %#v", msg)
			}
		}
	})
	defer server.Close()

	cfg := bridgeTestConfig(server.URL)
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local", Target: local.URL}}
	client := BridgeClient{Config: cfg}
	if _, err := client.runOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func joinedStreamBody(t *testing.T, chunks []dto.BridgeToolStreamChunk) string {
	t.Helper()
	var builder strings.Builder
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.BodyBase64) == "" {
			continue
		}
		body, err := base64.StdEncoding.DecodeString(chunk.BodyBase64)
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(body)
	}
	return builder.String()
}

func nowPlusSecond() time.Time {
	return time.Now().Add(time.Second)
}

func TestEffectiveCapabilitiesAddsHTTPTunnelForRoutes(t *testing.T) {
	cfg := DefaultConfig()
	if strings.Contains(strings.Join(EffectiveCapabilities(cfg), ","), BridgeCapabilityHTTPTunnel) {
		t.Fatal("http_tunnel should not be advertised without routes or explicit capability")
	}
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local", Target: "http://127.0.0.1:3000"}}
	capabilities := EffectiveCapabilities(cfg)
	if !strings.Contains(strings.Join(capabilities, ","), BridgeCapabilityHTTPTunnel) {
		t.Fatalf("http_tunnel capability missing: %#v", capabilities)
	}
}

func httpPayloadFromResult(t *testing.T, result dto.BridgeToolCallResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("missing result content")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
