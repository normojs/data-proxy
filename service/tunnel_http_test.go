package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/stretchr/testify/require"
)

func seedTunnelHTTPApp(t *testing.T, app model.TunnelApp, policy bridgepolicy.Policy) model.TunnelApp {
	t.Helper()
	if app.UserId == 0 {
		app.UserId = 100
	}
	if app.Name == "" {
		app.Name = "Local HTTP service"
	}
	if app.AppType == "" {
		app.AppType = model.TunnelAppTypeHTTP
	}
	if app.PermissionMode == "" {
		app.PermissionMode = model.TunnelPermissionTraffic
	}
	if app.Status == "" {
		app.Status = model.TunnelAppStatusApproved
	}
	if app.PublicSlug == "" {
		app.PublicSlug = "local-http"
	}
	if app.BridgeClientId == "" {
		app.BridgeClientId = "bridge-http-1"
	}
	if app.TargetHost == "" {
		app.TargetHost = "127.0.0.1"
	}
	if app.TargetPort == 0 {
		app.TargetPort = 8080
	}
	if app.TargetPath == "" {
		app.TargetPath = "/api"
	}
	require.NoError(t, model.DB.Create(&app).Error)
	rawPolicy, err := bridgepolicy.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BridgeClient{
		ClientId: app.BridgeClientId,
		UserId:   app.UserId,
		Policy:   rawPolicy,
	}).Error)
	return app
}

func seedTunnelHTTPConnection(t *testing.T, app model.TunnelApp, key string) model.TunnelConnection {
	t.Helper()
	if key == "" {
		key = "tc_test_http_connection_key"
	}
	connection := model.TunnelConnection{
		AppId:          app.Id,
		UserId:         app.UserId,
		Name:           "HTTP client",
		KeyPrefix:      tunnelConnectionKeyPrefix(key),
		KeyHash:        tunnelConnectionKeyHash(key),
		PermissionMode: model.TunnelPermissionTraffic,
		Status:         model.TunnelConnectionStatusActive,
	}
	require.NoError(t, model.DB.Create(&connection).Error)
	return connection
}

func TestCreateHTTPConnectionEndpointPath(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local web",
		AppType:        model.TunnelAppTypeHTTP,
		PermissionMode: model.TunnelPermissionTraffic,
		BridgeClientId: "bridge-http-create",
		TargetHost:     "127.0.0.1",
		TargetPort:     8080,
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)

	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{Name: "Browser"})
	require.NoError(t, err)
	require.Contains(t, resp.EndpointPath, "/t/")
	require.Contains(t, resp.EndpointPath, "/tunnel/http/")
	require.Contains(t, resp.EndpointPath, item.PublicSlug)
}

func TestTunnelHTTPTargetURLJoinsPathAndQuery(t *testing.T) {
	target, err := tunnelHTTPTargetURL(model.TunnelApp{
		TargetHost: "127.0.0.1",
		TargetPort: 8080,
		TargetPath: "/api",
	}, "/v1/users", "page=1")
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:8080/api/v1/users?page=1", target)
}

func TestForwardTunnelHTTPRequestForwardsThroughBridge(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_response_bytes":64}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "http-session-1",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.Equal(t, BridgeToolHTTPTunnelRequest, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		args := msg.Data.(dto.BridgeToolCallRequest).Arguments
		require.Equal(t, "POST", args["method"])
		require.Equal(t, "http://127.0.0.1:8080/api/hello?q=1", args["target"])
		require.Equal(t, int64(64), args["max_response_bytes"])
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code": 201,
					"headers": map[string]any{
						"Content-Type": []any{"application/json"},
					},
					"body_base64": base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`)),
				},
			},
			ResultSize: len(`{"ok":true}`),
		}))
	}()

	resp, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodPost,
		ProxyPath:     "/hello",
		RawQuery:      "q=1",
		Headers:       http.Header{"Content-Type": []string{"application/json"}},
		Body:          []byte(`{"name":"dp"}`),
		RequestId:     "http-forward-1",
		ClientIP:      "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, "http-forward-1", resp.RequestId)
	require.Equal(t, 201, resp.StatusCode)
	require.Equal(t, `{"ok":true}`, string(resp.Body))
	require.Equal(t, "application/json", resp.Headers.Get("Content-Type"))
	<-done

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-forward-1").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, connection.KeyPrefix, audit.ConnectionKeyPrefix)
	require.Equal(t, "http-session-1", audit.SessionId)

	var event model.BillingEvent
	require.NoError(t, db.First(&event, "source = ? AND request_id = ?", model.BillingEventSourceTunnelHTTP, "http-forward-1").Error)
	require.Equal(t, model.BillingEventTypeAudit, event.EventType)
	require.Equal(t, "request_traffic", event.PriceUnit)
	require.Equal(t, 0, event.AmountQuota)
	require.Equal(t, 0, event.QuotaDelta)
	require.Contains(t, event.Metadata, `"bytes_in":13`)
	require.Contains(t, event.Metadata, `"usage_kind":"tunnel"`)
}

func TestForwardTunnelHTTPStreamForwardsBridgeChunks(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_response_bytes":64}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
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
		SessionId:    "http-stream-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, BridgeToolHTTPTunnelRequest, call.ToolName)
		require.Equal(t, true, call.Arguments["stream_response"])
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			StatusCode: 202,
			Headers: map[string]any{
				"Content-Type": []any{"text/event-stream"},
			},
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("data: one\n\n")),
			Bytes:      len("data: one\n\n"),
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("data: two\n\n")),
			Bytes:      len("data: two\n\n"),
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{Done: true, Bytes: len("data: one\n\ndata: two\n\n")}))
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			ResultSize: len("data: one\n\ndata: two\n\n"),
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code": 202,
					"headers": map[string]any{
						"Content-Type": []any{"text/event-stream"},
					},
					"streamed": true,
				},
			},
		}))
	}()

	var body bytes.Buffer
	var headers http.Header
	var statusCode int
	resp, err := ForwardTunnelHTTPStream(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/events",
		RequestId:     "http-stream-1",
		ClientIP:      "127.0.0.1",
	}, func(event TunnelHTTPStreamEvent) error {
		if event.StatusCode > 0 {
			statusCode = event.StatusCode
			headers = event.Headers
		}
		_, _ = body.Write(event.Body)
		return nil
	})
	require.NoError(t, err)
	<-done
	require.Equal(t, 202, statusCode)
	require.Equal(t, 202, resp.StatusCode)
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "data: one\n\ndata: two\n\n", body.String())
	require.Equal(t, "http-stream-session", resp.BridgeSessionId)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-stream-1").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Contains(t, audit.MetadataJson, `"streamed":true`)
}

func TestForwardTunnelHTTPStreamForwardsRequestBodyChunks(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_request_bytes":64,"max_response_bytes":64}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 8)
	hub.Register(bridge.Session{
		SessionId:    "http-stream-request-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	requestBody := "hello streamed request"
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, BridgeToolHTTPTunnelRequest, call.ToolName)
		require.Equal(t, true, call.Arguments["stream_response"])
		require.Equal(t, true, call.Arguments["stream_request"])
		_, hasBufferedBody := call.Arguments["body_base64"]
		require.False(t, hasBufferedBody)

		var streamed bytes.Buffer
		for {
			inputMsg := <-outbound
			require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
			input := inputMsg.Data.(dto.BridgeToolStreamInput)
			require.Equal(t, "http_request_body", input.FrameType)
			if input.BodyBase64 != "" {
				chunk, err := base64.StdEncoding.DecodeString(input.BodyBase64)
				require.NoError(t, err)
				streamed.Write(chunk)
			}
			if input.Done {
				break
			}
		}
		require.Equal(t, requestBody, streamed.String())

		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			StatusCode: http.StatusCreated,
			Headers: map[string]any{
				"Content-Type": []any{"text/plain"},
			},
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("uploaded")),
			Bytes:      len("uploaded"),
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{Done: true, Bytes: len("uploaded")}))
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			ResultSize: len("uploaded"),
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code":    http.StatusCreated,
					"body_base64":    "",
					"streamed":       true,
					"stream_request": true,
				},
			},
		}))
	}()

	var body bytes.Buffer
	resp, err := ForwardTunnelHTTPStream(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodPost,
		ProxyPath:     "/upload",
		BodyReader:    strings.NewReader(requestBody),
		ContentLength: int64(len(requestBody)),
		RequestId:     "http-stream-upload",
		ClientIP:      "127.0.0.1",
	}, func(event TunnelHTTPStreamEvent) error {
		_, _ = body.Write(event.Body)
		return nil
	})
	require.NoError(t, err)
	<-done
	require.NotZero(t, resp.StatusCode)
	require.Equal(t, "uploaded", body.String())

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-stream-upload").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, int64(len(requestBody)), audit.BytesIn)
	require.Equal(t, int64(len("uploaded")), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"stream_request":true`)
}

func TestForwardTunnelHTTPStreamFallsBackToToolResult(t *testing.T) {
	_ = setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "http-fallback-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	go func() {
		msg := <-outbound
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code": 200,
					"headers": map[string]any{
						"Content-Type": []any{"text/plain"},
					},
					"body_base64": base64.StdEncoding.EncodeToString([]byte("fallback")),
				},
			},
			ResultSize: len("fallback"),
		}))
	}()

	var body bytes.Buffer
	resp, err := ForwardTunnelHTTPStream(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/fallback",
		RequestId:     "http-stream-fallback",
	}, func(event TunnelHTTPStreamEvent) error {
		_, _ = body.Write(event.Body)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "fallback", body.String())
}

type fakeTunnelWebSocketPeer struct {
	reads     chan fakeTunnelWebSocketRead
	writes    chan TunnelHTTPWebSocketFrame
	closeOnce sync.Once
	closed    chan struct{}
}

type fakeTunnelWebSocketRead struct {
	frame TunnelHTTPWebSocketFrame
	err   error
}

func newFakeTunnelWebSocketPeer() *fakeTunnelWebSocketPeer {
	return &fakeTunnelWebSocketPeer{
		reads:  make(chan fakeTunnelWebSocketRead, 4),
		writes: make(chan TunnelHTTPWebSocketFrame, 4),
		closed: make(chan struct{}),
	}
}

func (p *fakeTunnelWebSocketPeer) ReadFrame() (TunnelHTTPWebSocketFrame, error) {
	select {
	case item := <-p.reads:
		return item.frame, item.err
	case <-p.closed:
		return TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: 1000}, errors.New("websocket closed")
	}
}

func (p *fakeTunnelWebSocketPeer) WriteFrame(frame TunnelHTTPWebSocketFrame) error {
	select {
	case p.writes <- frame:
		return nil
	case <-p.closed:
		return errors.New("websocket closed")
	}
}

func (p *fakeTunnelWebSocketPeer) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
	})
	return nil
}

func TestForwardTunnelHTTPWebSocketProxiesFrames(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
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
		SessionId:    "http-ws-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameText, Data: []byte("hello")},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		call := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, true, call.Arguments["websocket"])
		require.Equal(t, true, call.Arguments["stream_response"])
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{StatusCode: http.StatusSwitchingProtocols}))

		inputMsg := <-outbound
		require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
		input := inputMsg.Data.(dto.BridgeToolStreamInput)
		require.Equal(t, TunnelWebSocketFrameText, input.FrameType)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("hello")), input.BodyBase64)

		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  TunnelWebSocketFrameText,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("echo: hello")),
			Bytes:      len("echo: hello"),
		}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:   TunnelWebSocketFrameClose,
			Done:        true,
			CloseCode:   1000,
			CloseReason: "done",
		}))
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{ResultSize: len("echo: hello")}))
	}()

	resp, err := ForwardTunnelHTTPWebSocket(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/socket",
		RequestId:     "http-ws-1",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.NoError(t, err)
	<-done
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	firstWrite := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameText, firstWrite.FrameType)
	require.Equal(t, "echo: hello", string(firstWrite.Data))
	secondWrite := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, secondWrite.FrameType)
	require.Equal(t, 1000, secondWrite.CloseCode)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-ws-1").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Contains(t, audit.MetadataJson, `"websocket":true`)
	require.Equal(t, int64(len("hello")), audit.BytesIn)
	require.Equal(t, int64(len("echo: hello")), audit.BytesOut)
}

func TestForwardTunnelHTTPWebSocketRejectsRequestTooLarge(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_request_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
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
		SessionId:    "http-ws-request-limit-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	peer.reads <- fakeTunnelWebSocketRead{
		frame: TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameText, Data: []byte("12345")},
	}
	done := make(chan struct{})
	var input dto.BridgeToolStreamInput
	go func() {
		defer close(done)
		msg := <-outbound
		require.Equal(t, BridgeToolHTTPTunnelRequest, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		inputMsg := <-outbound
		require.Equal(t, bridge.MessageTypeToolStreamInput, inputMsg.Type)
		input = inputMsg.Data.(dto.BridgeToolStreamInput)
	}()

	_, err := ForwardTunnelHTTPWebSocket(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/socket",
		RequestId:     "http-ws-too-large-in",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.ErrorIs(t, err, ErrTunnelHTTPRequestTooLarge)
	<-done
	require.Equal(t, TunnelWebSocketFrameClose, input.FrameType)
	require.Equal(t, "HTTP_TUNNEL_REQUEST_TOO_LARGE", input.ErrorCode)

	closeFrame := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, closeFrame.FrameType)
	require.Equal(t, tunnelWebSocketCloseMessageTooLarge, closeFrame.CloseCode)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-ws-too-large-in").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "request_too_large", audit.Reason)
	require.Equal(t, int64(5), audit.BytesIn)
	require.Contains(t, audit.MetadataJson, `"websocket":true`)
	require.Contains(t, audit.MetadataJson, `"max_request_bytes":4`)
}

func TestForwardTunnelHTTPWebSocketRejectsResponseTooLarge(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_response_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
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
		SessionId:    "http-ws-response-limit-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{StatusCode: http.StatusSwitchingProtocols}))
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  TunnelWebSocketFrameText,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("12345")),
			Bytes:      len("12345"),
		}))
	}()

	_, err := ForwardTunnelHTTPWebSocket(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/socket",
		RequestId:     "http-ws-too-large-out",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.ErrorIs(t, err, ErrTunnelHTTPResponseTooLarge)
	<-done

	closeFrame := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, closeFrame.FrameType)
	require.Equal(t, tunnelWebSocketCloseMessageTooLarge, closeFrame.CloseCode)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-ws-too-large-out").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "response_too_large", audit.Reason)
	require.Equal(t, int64(0), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"websocket":true`)
	require.Contains(t, audit.MetadataJson, `"max_response_bytes":4`)
}

func TestForwardTunnelHTTPWebSocketRejectsResponseRateLimit(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	require.NoError(t, db.Model(&model.TunnelConnection{}).Where("id = ?", connection.Id).Update("config_json", `{"rate_limit":{"max_bytes_out_per_minute":4}}`).Error)
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 4)
	hub.Register(bridge.Session{
		SessionId:    "http-ws-response-rate-session",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	peer := newFakeTunnelWebSocketPeer()
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.True(t, hub.PushToolStreamChunk(msg.Id, dto.BridgeToolStreamChunk{
			FrameType:  TunnelWebSocketFrameText,
			BodyBase64: base64.StdEncoding.EncodeToString([]byte("12345")),
			Bytes:      len("12345"),
		}))
	}()

	_, err := ForwardTunnelHTTPWebSocket(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/socket",
		RequestId:     "http-ws-rate-out",
		ClientIP:      "127.0.0.1",
	}, peer)
	require.ErrorIs(t, err, ErrTunnelRateLimited)
	<-done

	closeFrame := <-peer.writes
	require.Equal(t, TunnelWebSocketFrameClose, closeFrame.FrameType)
	require.Equal(t, tunnelWebSocketClosePolicyViolation, closeFrame.CloseCode)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-ws-rate-out").Error)
	require.Equal(t, model.TunnelAuditActionRateLimit, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "rate_limited", audit.Reason)
	require.Equal(t, int64(0), audit.BytesOut)
	require.Contains(t, audit.MetadataJson, `"metric":"bytes_out_per_minute"`)
	require.Contains(t, audit.MetadataJson, `"websocket":true`)
}

func TestForwardTunnelHTTPRequestRejectsRouteMaxRequestBytes(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_request_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodPost,
		ProxyPath:     "/hello",
		Body:          []byte(`12345`),
		RequestId:     "http-too-large-1",
		ClientIP:      "127.0.0.1",
	})
	require.ErrorIs(t, err, ErrTunnelHTTPRequestTooLarge)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-too-large-1").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "request_too_large", audit.Reason)
	require.Contains(t, audit.MetadataJson, `"max_request_bytes":4`)

	var event model.BillingEvent
	require.NoError(t, db.First(&event, "source = ? AND request_id = ?", model.BillingEventSourceTunnelHTTP, "http-too-large-1").Error)
	require.Equal(t, model.BillingEventTypeAudit, event.EventType)
	require.Equal(t, "request_traffic", event.PriceUnit)
}

func TestForwardTunnelHTTPStreamRejectsKnownLargeRequestBody(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"max_request_bytes":4}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPStream(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodPost,
		ProxyPath:     "/upload",
		BodyReader:    strings.NewReader("12345"),
		ContentLength: 5,
		RequestId:     "http-stream-too-large-known",
	}, func(event TunnelHTTPStreamEvent) error {
		return nil
	})
	require.ErrorIs(t, err, ErrTunnelHTTPRequestTooLarge)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-stream-too-large-known").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "request_too_large", audit.Reason)
	require.Equal(t, int64(5), audit.BytesIn)
	require.Contains(t, audit.MetadataJson, `"max_request_bytes":4`)
}

func TestForwardTunnelHTTPRequestRejectsConnectionRateLimit(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	require.NoError(t, db.Model(&model.TunnelConnection{}).Where("id = ?", connection.Id).Update("config_json", `{"rate_limit":{"max_requests_per_minute":1}}`).Error)

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/health",
		RequestId:     "http-rate-first",
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrTunnelRateLimited)

	_, err = ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/health",
		RequestId:     "http-rate-second",
	})
	require.ErrorIs(t, err, ErrTunnelRateLimited)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-rate-second").Error)
	require.Equal(t, model.TunnelAuditActionRateLimit, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "rate_limited", audit.Reason)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Contains(t, audit.MetadataJson, `"metric":"requests_per_minute"`)
}

func TestForwardTunnelHTTPRequestGeneratesConsistentRequestId(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "http-session-generated-id",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	var bridgeMessageId string
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		bridgeMessageId = msg.Id
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code": http.StatusNoContent,
					"body_base64": "",
				},
			},
		}))
	}()

	resp, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/health",
	})
	require.NoError(t, err)
	<-done
	require.NotEmpty(t, bridgeMessageId)
	require.Contains(t, bridgeMessageId, "tunnel-http-")
	require.Equal(t, bridgeMessageId, resp.RequestId)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", bridgeMessageId).Error)
	require.Equal(t, bridgeMessageId, audit.RequestId)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)

	var bridgeAudit model.BridgeAuditLog
	require.NoError(t, db.First(&bridgeAudit, "request_id = ?", bridgeMessageId).Error)
	require.Equal(t, bridgeMessageId, bridgeAudit.RequestId)
	require.Equal(t, model.BridgeAuditStatusSuccess, bridgeAudit.Status)
}

func TestForwardTunnelHTTPRequestRejectsNonLoopbackByDefault(t *testing.T) {
	_ = setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		TargetHost: "192.168.0.10",
		TargetPort: 8080,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/",
		RequestId:     "http-deny-1",
	})
	require.Error(t, err)
	require.Equal(t, bridgepolicy.ErrorCodeHTTPTargetForbidden, bridgepolicy.ErrorCode(err))
}

func TestForwardTunnelHTTPRequestRequiresRouteToken(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"auth_mode":"token","auth_token":"secret-token"}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/status",
		RequestId:     "http-token-missing",
	})
	require.ErrorIs(t, err, ErrTunnelHTTPAuthRequired)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-token-missing").Error)
	require.Equal(t, model.TunnelAuditActionProxyRequest, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "auth_token_required", audit.Reason)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Contains(t, audit.MetadataJson, `"auth_mode":"token"`)
}

func TestForwardTunnelHTTPRequestAllowsRouteToken(t *testing.T) {
	_ = setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"auth_mode":"token","auth_token":"secret-token","path_prefix":"/reports"}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")
	hub := bridge.NewHub()
	previousHub := bridge.DefaultHub
	bridge.DefaultHub = hub
	t.Cleanup(func() {
		bridge.DefaultHub = previousHub
	})
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "http-session-token",
		ClientId:     app.BridgeClientId,
		UserId:       app.UserId,
		Capabilities: []string{BridgeCapabilityHTTPTunnel},
		Send:         outbound,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		args := msg.Data.(dto.BridgeToolCallRequest).Arguments
		require.Equal(t, "http://127.0.0.1:8080/api/reports/daily", args["target"])
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"http_response": map[string]any{
					"status_code": http.StatusOK,
					"body_base64": base64.StdEncoding.EncodeToString([]byte("ok")),
				},
			},
		}))
	}()

	resp, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/reports/daily",
		Headers:       http.Header{"Authorization": []string{"Bearer secret-token"}},
		RequestId:     "http-token-allow",
	})
	require.NoError(t, err)
	require.Equal(t, "ok", string(resp.Body))
	<-done
}

func TestForwardTunnelHTTPRequestRejectsRouteHostMismatch(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"host":"tenant.example.com"}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		Host:          "other.example.com",
		ProxyPath:     "/",
		RequestId:     "http-host-deny",
	})
	require.ErrorIs(t, err, ErrTunnelHTTPRouteForbidden)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-host-deny").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "route_forbidden", audit.Reason)
	require.Contains(t, audit.MetadataJson, `"route_host":"tenant.example.com"`)
	require.Contains(t, audit.MetadataJson, `"request_host":"other.example.com"`)
}

func TestForwardTunnelHTTPRequestRejectsRoutePathPrefixMismatch(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		RouteJson: `{"path_prefix":"/public"}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	seedTunnelHTTPConnection(t, app, "")

	_, err := ForwardTunnelHTTPRequest(TunnelHTTPForwardRequest{
		Context:       context.Background(),
		ConnectionKey: "tc_test_http_connection_key",
		Slug:          app.PublicSlug,
		Method:        http.MethodGet,
		ProxyPath:     "/admin",
		RequestId:     "http-path-deny",
	})
	require.ErrorIs(t, err, ErrTunnelHTTPRouteForbidden)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "http-path-deny").Error)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "route_forbidden", audit.Reason)
	require.Contains(t, audit.MetadataJson, `"route_path_prefix":"/public"`)
}

func TestCreateHTTPRouteTokenRequiresSecret(t *testing.T) {
	_ = setupTunnelTestDB(t)
	_, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local web",
		AppType:        model.TunnelAppTypeHTTP,
		PermissionMode: model.TunnelPermissionTraffic,
		BridgeClientId: "bridge-http-create",
		TargetHost:     "127.0.0.1",
		TargetPort:     8080,
		Route: map[string]any{
			"auth_mode": model.TunnelRouteAuthToken,
		},
	})
	require.ErrorIs(t, err, ErrTunnelHTTPRouteConfigInvalid)
}
