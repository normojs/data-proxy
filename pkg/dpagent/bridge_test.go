package dpagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gorilla/websocket"
)

func TestBridgeRunOnceRegistersAndHandlesServerClose(t *testing.T) {
	server := newBridgeTestServer(t, func(t *testing.T, conn *websocket.Conn) {
		msg := readBridgeTestMessage(t, conn)
		if msg.Type != "register" {
			t.Fatalf("expected register, got %s", msg.Type)
		}
		var register dto.BridgeClientRegisterRequest
		if err := decodeBridgeData(msg.Data, &register); err != nil {
			t.Fatal(err)
		}
		if register.ClientId != "test-agent" {
			t.Fatalf("unexpected client id: %s", register.ClientId)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{Type: "registered", Data: map[string]any{
			"session_id": "sess-1",
			"client_id":  "test-agent",
		}}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(dto.BridgeWSMessage{Type: "close"}); err != nil {
			t.Fatal(err)
		}
	})
	defer server.Close()

	cfg := bridgeTestConfig(server.URL)
	client := BridgeClient{Config: cfg}
	result, err := client.runOnce(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Registered || result.SessionID != "sess-1" || result.ClientID != "test-agent" {
		t.Fatalf("unexpected run result: %#v", result)
	}
}

func TestBridgeRunOnceSendsHealthAfterRegistered(t *testing.T) {
	server := newBridgeTestServer(t, func(t *testing.T, conn *websocket.Conn) {
		_ = readBridgeTestMessage(t, conn)
		if err := conn.WriteJSON(dto.BridgeWSMessage{Type: "registered", Data: map[string]any{
			"session_id": "sess-health",
			"client_id":  "test-agent",
		}}); err != nil {
			t.Fatal(err)
		}
		msg := readBridgeTestMessage(t, conn)
		if msg.Type != "health" {
			t.Fatalf("expected health, got %s", msg.Type)
		}
		var report dto.BridgeAgentHealthReport
		if err := decodeBridgeData(msg.Data, &report); err != nil {
			t.Fatal(err)
		}
		if report.GeneratedAt == 0 || report.Summary.Total == 0 || len(report.Checks) == 0 {
			t.Fatalf("unexpected health report: %#v", report)
		}
		_ = conn.WriteJSON(dto.BridgeWSMessage{Type: "close"})
	})
	defer server.Close()

	cfg := bridgeTestConfig(server.URL)
	cfg.Agent.Workspace = t.TempDir()
	cfg.Runtime.HealthIntervalMS = 60 * 1000
	client := BridgeClient{Config: cfg}
	if _, err := client.runOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestBridgeRunOnceRejectsUnknownToolCall(t *testing.T) {
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
			Id:   "req-1",
			Data: dto.BridgeToolCallRequest{
				RequestId: "req-1",
				ToolName:  "unknown.tool",
			},
		}); err != nil {
			t.Fatal(err)
		}
		msg := readBridgeTestMessage(t, conn)
		if msg.Type != "tool_error" {
			t.Fatalf("expected tool_error, got %s", msg.Type)
		}
		if msg.Id != "req-1" {
			t.Fatalf("unexpected request id: %s", msg.Id)
		}
		var toolErr dto.BridgeToolCallError
		if err := decodeBridgeData(msg.Data, &toolErr); err != nil {
			t.Fatal(err)
		}
		if toolErr.Code != "TOOL_NOT_SUPPORTED" {
			t.Fatalf("unexpected tool error: %#v", toolErr)
		}
		_ = conn.WriteJSON(dto.BridgeWSMessage{Type: "close"})
	})
	defer server.Close()

	cfg := bridgeTestConfig(server.URL)
	client := BridgeClient{Config: cfg}
	if _, err := client.runOnce(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func bridgeTestConfig(serverURL string) Config {
	cfg := DefaultConfig()
	cfg.Server.BridgeWSURL = "ws" + strings.TrimPrefix(serverURL, "http")
	cfg.Agent.ClientID = "test-agent"
	cfg.Agent.Name = "Test Agent"
	cfg.Agent.Token = "test-token"
	cfg.Agent.Workspace = "/tmp"
	cfg.Runtime.PingIntervalMS = 0
	cfg.Runtime.HealthIntervalMS = 0
	cfg.Runtime.Reconnect = false
	return cfg
}

func newBridgeTestServer(t *testing.T, handler func(*testing.T, *websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		handler(t, conn)
	}))
}

func readBridgeTestMessage(t *testing.T, conn *websocket.Conn) dto.BridgeWSMessage {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var msg dto.BridgeWSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	return msg
}
