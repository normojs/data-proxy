package bridge

import (
	"context"
	"testing"

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
