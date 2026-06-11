package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

func TestHubRegistersSingleSessionPerClient(t *testing.T) {
	hub := NewHub()
	hub.Register(Session{SessionId: "session-1", ClientId: "client-1", UserId: 1})
	hub.Register(Session{SessionId: "session-2", ClientId: "client-1", UserId: 1})

	if hub.Count() != 1 {
		t.Fatalf("expected one active session, got %d", hub.Count())
	}
	snapshot, ok := hub.GetByClient("client-1")
	if !ok {
		t.Fatal("expected client session")
	}
	if snapshot.SessionId != "session-2" {
		t.Fatalf("session mismatch, got %s", snapshot.SessionId)
	}
	if _, ok := hub.Unregister("session-1"); ok {
		t.Fatal("old session should have been evicted")
	}
	if _, ok := hub.Unregister("session-2"); !ok {
		t.Fatal("current session should unregister")
	}
}

func TestHubTouchUpdatesSession(t *testing.T) {
	hub := NewHub()
	hub.Register(Session{SessionId: "session-1", ClientId: "client-1"})
	before, ok := hub.GetByClient("client-1")
	if !ok {
		t.Fatal("expected client session")
	}
	if !hub.Touch("session-1") {
		t.Fatal("expected touch to succeed")
	}
	after, ok := hub.GetByClient("client-1")
	if !ok {
		t.Fatal("expected client session after touch")
	}
	if after.LastSeenAt.Before(before.LastSeenAt) {
		t.Fatalf("last seen moved backwards, before=%v after=%v", before.LastSeenAt, after.LastSeenAt)
	}
}

func TestHubCloseSessionNotifiesAndRemovesSession(t *testing.T) {
	hub := NewHub()
	outbound := make(chan OutboundMessage, 1)
	hub.Register(Session{SessionId: "session-1", ClientId: "client-1", Send: outbound})

	snapshot, ok := hub.CloseSession("session-1", CloseSessionOptions{
		Reason: "admin close",
		Notify: true,
	})
	if !ok {
		t.Fatal("expected close to succeed")
	}
	if snapshot.ClientId != "client-1" {
		t.Fatalf("snapshot mismatch: %#v", snapshot)
	}
	if hub.Count() != 0 {
		t.Fatalf("expected no active sessions, got %d", hub.Count())
	}
	if hub.Touch("session-1") {
		t.Fatal("closed session should not be touchable")
	}
	msg := <-outbound
	if msg.Type != "close" {
		t.Fatalf("expected close message, got %#v", msg)
	}
}

func TestHubUpdateClientMetadata(t *testing.T) {
	hub := NewHub()
	hub.Register(Session{SessionId: "session-1", ClientId: "client-1", Name: "old"})

	if !hub.UpdateClientMetadata("client-1", SessionMetadata{
		Name:         "new",
		Version:      "1.2.3",
		Capabilities: []string{"remote_read"},
	}) {
		t.Fatal("expected metadata update to succeed")
	}
	snapshot, ok := hub.GetByClient("client-1")
	if !ok {
		t.Fatal("expected updated session")
	}
	if snapshot.Name != "new" || snapshot.Version != "1.2.3" || len(snapshot.Capabilities) != 1 {
		t.Fatalf("metadata mismatch: %#v", snapshot)
	}
}

func TestHubSelectSessionsSortsByLatestActivityAndCapability(t *testing.T) {
	hub := NewHub()
	base := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	hub.Register(Session{
		SessionId:    "session-old",
		ClientId:     "client-old",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base.Add(time.Minute),
		LastSeenAt:   base.Add(2 * time.Minute),
	})
	hub.Register(Session{
		SessionId:    "session-new",
		ClientId:     "client-new",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(3 * time.Minute),
	})
	hub.Register(Session{
		SessionId:    "session-other-tool",
		ClientId:     "client-other-tool",
		UserId:       1,
		Capabilities: []string{"remote_read"},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(4 * time.Minute),
	})
	hub.Register(Session{
		SessionId:    "session-other-user",
		ClientId:     "client-other-user",
		UserId:       2,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(5 * time.Minute),
	})

	sessions := hub.SelectSessions(1, "", "mcp_proxy")
	if len(sessions) != 2 {
		t.Fatalf("expected two candidate sessions, got %#v", sessions)
	}
	if sessions[0].ClientId != "client-new" || sessions[1].ClientId != "client-old" {
		t.Fatalf("unexpected candidate order: %#v", sessions)
	}
	selected, ok := hub.SelectSession(1, "", "mcp_proxy")
	if !ok {
		t.Fatal("expected selected session")
	}
	if selected.ClientId != "client-new" {
		t.Fatalf("expected latest active session, got %#v", selected)
	}
}

func TestHubSelectSessionsSortsTiesByConnectedAndClientId(t *testing.T) {
	hub := NewHub()
	base := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	hub.Register(Session{
		SessionId:    "session-b",
		ClientId:     "client-b",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base.Add(time.Minute),
		LastSeenAt:   base,
	})
	hub.Register(Session{
		SessionId:    "session-a",
		ClientId:     "client-a",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base.Add(time.Minute),
		LastSeenAt:   base,
	})
	hub.Register(Session{
		SessionId:    "session-newer-connection",
		ClientId:     "client-c",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base.Add(2 * time.Minute),
		LastSeenAt:   base,
	})

	sessions := hub.SelectSessions(1, "", "mcp_proxy")
	if len(sessions) != 3 {
		t.Fatalf("expected three candidate sessions, got %#v", sessions)
	}
	got := []string{sessions[0].ClientId, sessions[1].ClientId, sessions[2].ClientId}
	want := []string{"client-c", "client-a", "client-b"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("candidate order mismatch, got %v want %v", got, want)
		}
	}
}

func TestHubSelectSessionsPreferredClientIsExact(t *testing.T) {
	hub := NewHub()
	base := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	hub.Register(Session{
		SessionId:    "session-preferred",
		ClientId:     "client-preferred",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base,
		LastSeenAt:   base,
	})
	hub.Register(Session{
		SessionId:    "session-newer",
		ClientId:     "client-newer",
		UserId:       1,
		Capabilities: []string{"mcp_proxy"},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(time.Minute),
	})

	sessions := hub.SelectSessions(1, "client-preferred", "mcp_proxy")
	if len(sessions) != 1 || sessions[0].ClientId != "client-preferred" {
		t.Fatalf("expected exact preferred client match, got %#v", sessions)
	}
	if sessions := hub.SelectSessions(2, "client-preferred", "mcp_proxy"); len(sessions) != 0 {
		t.Fatalf("preferred client should not cross users, got %#v", sessions)
	}
}

func TestHubForwardToolCallCompletesResult(t *testing.T) {
	hub := NewHub()
	outbound := make(chan OutboundMessage, 1)
	hub.Register(Session{
		SessionId:    "session-1",
		ClientId:     "client-1",
		UserId:       1,
		Capabilities: []string{"remote_read"},
		Send:         outbound,
	})

	resultCh := make(chan ToolCallResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := hub.ForwardToolCall(context.Background(), "session-1", ToolCallRequest{
			Id:       "request-1",
			ToolName: "remote_read",
			Arguments: map[string]any{
				"file_path": "README.md",
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	msg := <-outbound
	if msg.Type != MessageTypeToolCall || msg.Id != "request-1" {
		t.Fatalf("unexpected outbound message: %#v", msg)
	}
	if !hub.CompleteToolCall("request-1", dto.BridgeToolCallResult{
		Content: []dto.MCPContentBlock{{Type: "text", Text: "ok"}},
	}) {
		t.Fatal("expected completion to find pending call")
	}

	select {
	case err := <-errCh:
		t.Fatalf("ForwardToolCall failed: %v", err)
	case result := <-resultCh:
		if len(result.Result.Content) != 1 || result.Result.Content[0].Text != "ok" {
			t.Fatalf("result mismatch: %#v", result)
		}
	}
}
