package service

import (
	"context"
	"encoding/base64"
	"net/http"
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
