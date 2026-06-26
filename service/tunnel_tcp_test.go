package service

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/stretchr/testify/require"
)

func TestForwardTunnelTCPWebSocketProxiesFramesAndAudits(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp",
		BridgeClientId: "bridge-tcp-1",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameBinary, Data: []byte("ping")},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, BridgeToolTCPTunnelConnect, call.ToolName)
		require.Equal(t, "127.0.0.1:22", call.Arguments["target"])

		inputMsg := <-outbound
		require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
		input := inputMsg.Data.(dto.BridgeToolStreamInput)
		require.Equal(t, tunnelTCPFrameData, input.FrameType)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("ping")), input.BodyBase64)

		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  tunnelTCPFrameData,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("pong")),
			Bytes:      len("pong"),
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType: TunnelWebSocketFrameClose,
			Done:      true,
			Bytes:     len("pong"),
		}))
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{ResultSize: len("pong")}))
	}()

	resp, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-1",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.NoError(t, err)
	<-done
	require.Equal(t, "tcp-session", resp.BridgeSessionId)
	require.Equal(t, int64(len("ping")), resp.BytesIn)
	require.Equal(t, int64(len("pong")), resp.BytesOut)

	firstWrite := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameBinary, firstWrite.FrameType)
	require.Equal(t, "pong", string(firstWrite.Data))
	secondWrite := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, secondWrite.FrameType)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-1").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, "TCP", audit.Method)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, int64(len("ping")), audit.BytesIn)
	require.Equal(t, int64(len("pong")), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"tcp":true`)

	var event model.BillingEvent
	require.NoError(t, db.First(&event, "source = ? AND request_id = ?", model.BillingEventSourceTunnelTCP, "tcp-ws-1").Error)
	require.Equal(t, model.BillingEventTypeAudit, event.EventType)
	require.Contains(t, event.Metadata, `"usage_kind":"tunnel"`)
}

func TestForwardTunnelTCPWebSocketRespondsToPingWithoutForwardingData(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp-ping",
		BridgeClientId: "bridge-tcp-ping",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-ping-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFramePing, Data: []byte("pulse")},
	}
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: 1000, CloseReason: "done"},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, BridgeToolTCPTunnelConnect, call.ToolName)

		inputMsg := <-outbound
		require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
		input := inputMsg.Data.(dto.BridgeToolStreamInput)
		require.Equal(t, TunnelWebSocketFrameClose, input.FrameType)
		require.Empty(t, input.BodyBase64)
		require.True(t, input.Done)
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{}))
	}()

	resp, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-ping",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.NoError(t, err)
	<-done
	require.Equal(t, int64(0), resp.BytesIn)
	require.Equal(t, int64(0), resp.BytesOut)

	firstWrite := <-peer.writes
	require.Equal(t, TunnelWebSocketFramePong, firstWrite.FrameType)
	require.Equal(t, "pulse", string(firstWrite.Data))

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-ping").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, int64(0), audit.BytesIn)
	require.Equal(t, int64(0), audit.BytesOut)
}

func TestForwardTunnelTCPWebSocketCancelsBridgeWhenPeerWriteFails(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp-writer-fail",
		BridgeClientId: "bridge-tcp-writer-fail",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-writer-fail-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	writeErr := errors.New("tcp websocket client write failed")
	peer.writeErr = writeErr
	returned := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  tunnelTCPFrameData,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("server-push")),
			Bytes:      len("server-push"),
		}))
		<-returned
		require.False(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{FrameType: TunnelWebSocketFrameClose, Done: true}))
		require.False(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{}))
	}()

	_, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-writer-failed",
		ClientIP:      "127.0.0.1",
	}, peer)
	close(returned)
	require.ErrorIs(t, err, writeErr)
	<-done

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-writer-failed").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, tunnelHTTPReasonClientWriteFailed, audit.Reason)
	require.Equal(t, int64(len("server-push")), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"error":"tcp websocket client write failed"`)
	require.Contains(t, audit.MetadataJson, `"tcp":true`)

	var bridgeAudit model.BridgeAuditLog
	require.NoError(t, db.First(&bridgeAudit, "request_id = ?", "tcp-ws-writer-failed").Error)
	require.Equal(t, model.BridgeAuditStatusError, bridgeAudit.Status)
	require.Contains(t, bridgeAudit.ErrorMessage, "tcp websocket client write failed")
}

func TestForwardTunnelTCPWebSocketAuditsBridgeStreamError(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp-error",
		BridgeClientId: "bridge-tcp-error",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-error-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameBinary, Data: []byte("ping")},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, BridgeToolTCPTunnelConnect, call.ToolName)
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			ErrorCode:    "TCP_TUNNEL_CONNECT_FAILED",
			ErrorMessage: "dial tcp 127.0.0.1:22: connection refused",
		}))
	}()

	_, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-bridge-error",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection refused")
	<-done

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-bridge-error").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, "tcp-error-session", audit.SessionId)
	require.Equal(t, int64(0), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"error":"dial tcp 127.0.0.1:22: connection refused"`)
	require.Contains(t, audit.MetadataJson, `"tcp":true`)

	var bridgeAudit model.BridgeAuditLog
	require.NoError(t, db.First(&bridgeAudit, "request_id = ?", "tcp-ws-bridge-error").Error)
	require.Equal(t, model.BridgeAuditStatusError, bridgeAudit.Status)
	require.Equal(t, "TCP_TUNNEL_CONNECT_FAILED", bridgeAudit.ErrorCode)
	require.Contains(t, bridgeAudit.ErrorMessage, "connection refused")
}

func TestForwardTunnelTCPWebSocketRejectsRequestTooLarge(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp-request-limit",
		BridgeClientId: "bridge-tcp-request-limit",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
		RouteJson:      `{"max_request_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-request-limit-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameBinary, Data: []byte("12345")},
	}
	done := make(chan struct{})
	var input dto.BridgeToolStreamInput
	go func() {
		defer close(done)
		msg := <-outbound
		require.Equal(t, BridgeToolTCPTunnelConnect, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		inputMsg := <-outbound
		require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
		input = inputMsg.Data.(dto.BridgeToolStreamInput)
	}()

	_, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-too-large-in",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.ErrorIs(t, err, ErrTunnelHTTPRequestTooLarge)
	<-done
	require.Equal(t, TunnelWebSocketFrameClose, input.FrameType)
	require.True(t, input.Done)
	require.Equal(t, tunnelWebSocketCloseMessageTooLarge, input.CloseCode)
	require.Equal(t, "request too large", input.CloseReason)
	require.Equal(t, "TCP_TUNNEL_REQUEST_TOO_LARGE", input.ErrorCode)

	closeFrame := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, closeFrame.FrameType)
	require.Equal(t, tunnelWebSocketCloseMessageTooLarge, closeFrame.CloseCode)
	require.Equal(t, "request too large", closeFrame.CloseReason)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-too-large-in").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "request_too_large", audit.Reason)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, int64(5), audit.BytesIn)
	require.Equal(t, int64(0), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"tcp":true`)
	require.Contains(t, audit.MetadataJson, `"max_request_bytes":4`)

	var bridgeAudit model.BridgeAuditLog
	require.NoError(t, db.First(&bridgeAudit, "request_id = ?", "tcp-ws-too-large-in").Error)
	require.Equal(t, model.BridgeAuditStatusError, bridgeAudit.Status)
	require.Contains(t, bridgeAudit.ErrorMessage, "tcp request exceeded 4")
}

func TestForwardTunnelTCPWebSocketRejectsResponseTooLarge(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		AppType:        model.TunnelAppTypeTCP,
		PublicSlug:     "local-tcp-response-limit",
		BridgeClientId: "bridge-tcp-response-limit",
		TargetHost:     "127.0.0.1",
		TargetPort:     22,
		RouteJson:      `{"max_response_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"tcp_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "tcp-response-limit-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityTCPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	returned := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.Equal(t, BridgeToolTCPTunnelConnect, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  tunnelTCPFrameData,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("12345")),
			Bytes:      len("12345"),
		}))
		<-returned
		require.False(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{FrameType: TunnelWebSocketFrameClose, Done: true}))
		require.False(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{}))
	}()

	_, err := ForwardTunnelTCPWebSocket(TunnelTCPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		RequestId:     "tcp-ws-too-large-out",
		ClientIP:      "127.0.0.1",
	}, peer)
	close(returned)
	require.ErrorIs(t, err, ErrTunnelHTTPResponseTooLarge)
	<-done

	closeFrame := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, closeFrame.FrameType)
	require.Equal(t, tunnelWebSocketCloseMessageTooLarge, closeFrame.CloseCode)
	require.Equal(t, "response too large", closeFrame.CloseReason)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tcp-ws-too-large-out").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "response_too_large", audit.Reason)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, int64(0), audit.BytesIn)
	require.Equal(t, int64(0), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"tcp":true`)
	require.Contains(t, audit.MetadataJson, `"max_response_bytes":4`)

	var bridgeAudit model.BridgeAuditLog
	require.NoError(t, db.First(&bridgeAudit, "request_id = ?", "tcp-ws-too-large-out").Error)
	require.Equal(t, model.BridgeAuditStatusError, bridgeAudit.Status)
	require.Contains(t, bridgeAudit.ErrorMessage, "tcp response exceeded 4")
}
