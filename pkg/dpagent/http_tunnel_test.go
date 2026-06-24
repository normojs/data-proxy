package dpagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
