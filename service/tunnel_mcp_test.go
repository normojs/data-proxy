package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
	"github.com/stretchr/testify/require"
)

type fakeTunnelMCPProxyClient struct {
	tools       []mcpproxy.ToolDefinition
	result      mcpproxy.CallResult
	rawResult   mcpproxy.RawResult
	listErr     error
	callErr     error
	rawErr      error
	servers     []model.MCPProxyServer
	callReqs    []mcpproxy.CallRequest
	rawReqs     []mcpproxy.RawRequest
	listUserIds []int
	listCalled  int
	callCalled  int
	rawCalled   int
}

func (f *fakeTunnelMCPProxyClient) Test(ctx context.Context, server model.MCPProxyServer) (mcpproxy.TestResult, error) {
	return mcpproxy.TestResult{}, nil
}

func (f *fakeTunnelMCPProxyClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]mcpproxy.ToolDefinition, error) {
	f.listCalled++
	f.servers = append(f.servers, server)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

func (f *fakeTunnelMCPProxyClient) ListToolsForUser(ctx context.Context, server model.MCPProxyServer, userId int) ([]mcpproxy.ToolDefinition, error) {
	f.listUserIds = append(f.listUserIds, userId)
	return f.ListTools(ctx, server)
}

func (f *fakeTunnelMCPProxyClient) CallTool(ctx context.Context, server model.MCPProxyServer, req mcpproxy.CallRequest) (mcpproxy.CallResult, error) {
	f.callCalled++
	f.servers = append(f.servers, server)
	f.callReqs = append(f.callReqs, req)
	if f.callErr != nil {
		return mcpproxy.CallResult{}, f.callErr
	}
	return f.result, nil
}

func (f *fakeTunnelMCPProxyClient) CallRaw(ctx context.Context, server model.MCPProxyServer, req mcpproxy.RawRequest) (mcpproxy.RawResult, error) {
	f.rawCalled++
	f.servers = append(f.servers, server)
	f.rawReqs = append(f.rawReqs, req)
	if f.rawErr != nil {
		return mcpproxy.RawResult{}, f.rawErr
	}
	return f.rawResult, nil
}

func seedTunnelMCPApp(t *testing.T, app model.TunnelApp) model.TunnelApp {
	t.Helper()
	if app.UserId == 0 {
		app.UserId = 100
	}
	if app.Name == "" {
		app.Name = "Local user MCP"
	}
	if app.AppType == "" {
		app.AppType = model.TunnelAppTypeMCPCode
	}
	if app.PermissionMode == "" {
		app.PermissionMode = model.TunnelPermissionReadOnly
	}
	if app.Status == "" {
		app.Status = model.TunnelAppStatusApproved
	}
	if app.PublicSlug == "" {
		app.PublicSlug = "local-user-mcp"
	}
	if app.BridgeClientId == "" {
		app.BridgeClientId = "bridge-client-1"
	}
	require.NoError(t, model.DB.Create(&app).Error)
	return app
}

func seedTunnelMCPConnection(t *testing.T, app model.TunnelApp, key string) model.TunnelConnection {
	t.Helper()
	if key == "" {
		key = "tc_test_connection_key_100"
	}
	connection := model.TunnelConnection{
		AppId:          app.Id,
		UserId:         app.UserId,
		Name:           "Desktop Codex",
		KeyPrefix:      tunnelConnectionKeyPrefix(key),
		KeyHash:        tunnelConnectionKeyHash(key),
		PermissionMode: app.PermissionMode,
		Status:         model.TunnelConnectionStatusActive,
	}
	require.NoError(t, model.DB.Create(&connection).Error)
	return connection
}

func seedTunnelMCPSession(t *testing.T, app model.TunnelApp, connection model.TunnelConnection, sessionId string) model.TunnelSession {
	t.Helper()
	session := model.TunnelSession{
		AppId:          app.Id,
		UserId:         app.UserId,
		ConnectionId:   connection.Id,
		ConnectionName: connection.Name,
		KeyPrefix:      connection.KeyPrefix,
		SessionId:      sessionId,
		BridgeClientId: app.BridgeClientId,
		Status:         model.TunnelSessionStatusOnline,
	}
	require.NoError(t, model.DB.Create(&session).Error)
	return session
}

func TestListTunnelMCPToolsFiltersByGatewayPolicy(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	connection := seedTunnelMCPConnection(t, app, "")
	seedTunnelMCPSession(t, app, connection, "tmcp-call-session")
	client := &fakeTunnelMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "read_file", Description: "read file content", InputSchema: map[string]any{"type": "object"}},
			{Name: "write_file", Description: "write file content", InputSchema: map[string]any{"type": "object"}},
			{Name: "do_magic", Description: "opaque custom action", InputSchema: map[string]any{"type": "object"}},
		},
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	tools, err := ListTunnelMCPTools(TunnelMCPToolsListRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "tools-list-1",
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "read_file", tools[0].Name)
	require.Equal(t, 1, client.listCalled)
	require.Equal(t, []int{100}, client.listUserIds)
	require.Equal(t, "bridge://bridge-client-1", client.servers[0].Endpoint)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tools-list-1").Error)
	require.Equal(t, "tools_list", audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, connection.KeyPrefix, audit.ConnectionKeyPrefix)
	require.Contains(t, audit.MetadataJson, `"discovered_count":3`)
	require.Contains(t, audit.MetadataJson, `"exposed_count":1`)
}

func TestListTunnelMCPToolsRequiresOwnedConnection(t *testing.T) {
	_ = setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	key := "tc_other_user_connection_key"
	connection := model.TunnelConnection{
		AppId:          app.Id,
		UserId:         200,
		Name:           "Other user",
		KeyPrefix:      tunnelConnectionKeyPrefix(key),
		KeyHash:        tunnelConnectionKeyHash(key),
		PermissionMode: model.TunnelPermissionReadOnly,
		Status:         model.TunnelConnectionStatusActive,
	}
	require.NoError(t, model.DB.Create(&connection).Error)
	client := &fakeTunnelMCPProxyClient{}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	_, err := ListTunnelMCPTools(TunnelMCPToolsListRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: key,
		RequestId:     "tools-list-other-user",
	})
	require.Error(t, err)
	require.Equal(t, 0, client.listCalled)
}

func TestListTunnelMCPToolsRejectsConnectionRateLimit(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	connection := seedTunnelMCPConnection(t, app, "")
	require.NoError(t, db.Model(&model.TunnelConnection{}).Where("id = ?", connection.Id).Update("config_json", `{"rate_limit":{"max_requests_per_minute":1}}`).Error)
	client := &fakeTunnelMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "read_file", Description: "read file content", InputSchema: map[string]any{"type": "object"}},
		},
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	_, err := ListTunnelMCPTools(TunnelMCPToolsListRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "tools-rate-first",
	})
	require.NoError(t, err)

	_, err = ListTunnelMCPTools(TunnelMCPToolsListRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "tools-rate-second",
	})
	require.ErrorIs(t, err, ErrTunnelRateLimited)
	require.Equal(t, 1, client.listCalled)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "tools-rate-second").Error)
	require.Equal(t, model.TunnelAuditActionRateLimit, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "rate_limited", audit.Reason)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Contains(t, audit.MetadataJson, `"metric":"requests_per_minute"`)
}

func TestCallTunnelMCPToolDeniesWriteInReadOnlyMode(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	connection := seedTunnelMCPConnection(t, app, "")
	client := &fakeTunnelMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "write_file", Description: "write file content", InputSchema: map[string]any{"type": "object"}},
		},
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	resp, err := CallTunnelMCPTool(TunnelMCPToolCallRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "call-deny-1",
		Params: dto.MCPToolCallParams{
			Name:      "write_file",
			Arguments: map[string]any{"path": "main.go", "content": "x"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Nil(t, resp.Result)
	require.Equal(t, dto.MCPErrorCodeInvalidRequest, resp.ErrorCode)
	require.Equal(t, 1, client.listCalled)
	require.Equal(t, []int{100}, client.listUserIds)
	require.Equal(t, 0, client.callCalled)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "call-deny-1").Error)
	require.Equal(t, model.TunnelAuditActionPolicyDeny, audit.Action)
	require.Equal(t, "deny", audit.Decision)
	require.Equal(t, "write_file", audit.ToolName)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, connection.KeyPrefix, audit.ConnectionKeyPrefix)
	require.Contains(t, audit.MetadataJson, "argument_hash")
}

func TestCallTunnelMCPToolForwardsAllowedToolAndAudits(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		PermissionMode: model.TunnelPermissionWrite,
		TargetHost:     "127.0.0.1",
		TargetPort:     30837,
		TargetPath:     "/mcp",
	})
	connection := seedTunnelMCPConnection(t, app, "")
	seedTunnelMCPSession(t, app, connection, "tmcp-call-session")
	client := &fakeTunnelMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "write_file", Description: "write file content", InputSchema: map[string]any{"type": "object"}},
		},
		result: mcpproxy.CallResult{
			Content:         []dto.MCPContentBlock{{Type: "text", Text: "ok"}},
			Metadata:        map[string]any{"effect": "file_write"},
			Summary:         "ok",
			ResultSize:      2,
			BridgeSessionId: "bridge-session-1",
			TargetClient:    "bridge-client-1",
		},
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	resp, err := CallTunnelMCPTool(TunnelMCPToolCallRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "call-allow-1",
		SessionId:     "tmcp-call-session",
		Params: dto.MCPToolCallParams{
			Name:      "write_file",
			Arguments: map[string]any{"path": "main.go", "content": "x"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Result)
	require.Equal(t, "ok", resp.Result.Content[0].Text)
	require.Equal(t, []int{100}, client.listUserIds)
	require.Equal(t, 1, client.callCalled)
	require.Equal(t, "write_file", client.callReqs[0].ToolName)
	require.Contains(t, client.servers[0].Endpoint, "bridge://bridge-client-1?target=")
	encodedTarget := client.servers[0].Endpoint[strings.Index(client.servers[0].Endpoint, "target=")+len("target="):]
	target, err := url.QueryUnescape(encodedTarget)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:30837/mcp", target)

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "call-allow-1").Error)
	require.Equal(t, model.TunnelAuditActionMCPToolCall, audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, "write_file", audit.ToolName)
	require.Equal(t, "tmcp-call-session", audit.SessionId)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, connection.KeyPrefix, audit.ConnectionKeyPrefix)
	require.Contains(t, audit.MetadataJson, `"bridge_session_id":"bridge-session-1"`)
	require.Contains(t, audit.MetadataJson, `"target_client":"bridge-client-1"`)

	var session model.TunnelSession
	require.NoError(t, db.First(&session, "session_id = ?", "tmcp-call-session").Error)
	require.Greater(t, session.BytesIn, int64(0))
	require.Greater(t, session.BytesOut, int64(0))

	var event model.BillingEvent
	require.NoError(t, db.First(&event, "source = ? AND request_id = ?", model.BillingEventSourceTunnelMCP, "call-allow-1").Error)
	require.Equal(t, model.BillingEventTypeAudit, event.EventType)
	require.Equal(t, "per_call", event.PriceUnit)
	require.Equal(t, 0, event.AmountQuota)
	require.Equal(t, 0, event.QuotaDelta)
	require.Contains(t, event.Metadata, `"tool_name":"write_file"`)
	require.Contains(t, event.Metadata, `"usage_kind":"tunnel"`)
}

func TestCallTunnelMCPToolReturnsUpstreamError(t *testing.T) {
	_ = setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		PermissionMode: model.TunnelPermissionWrite,
	})
	seedTunnelMCPConnection(t, app, "")
	client := &fakeTunnelMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "write_file", Description: "write file content", InputSchema: map[string]any{"type": "object"}},
		},
		callErr: errors.New("bridge offline"),
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	resp, err := CallTunnelMCPTool(TunnelMCPToolCallRequest{
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "call-error-1",
		Params: dto.MCPToolCallParams{
			Name: "write_file",
		},
	})
	require.NoError(t, err)
	require.Nil(t, resp.Result)
	require.Equal(t, dto.MCPErrorCodeExecutorFailed, resp.ErrorCode)
	require.Contains(t, resp.ErrorMessage, "upstream")
}

func TestCallTunnelMCPRawForwardsResourcesAndAudits(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		TargetHost: "127.0.0.1",
		TargetPort: 30837,
		TargetPath: "/mcp",
	})
	connection := seedTunnelMCPConnection(t, app, "")
	seedTunnelMCPSession(t, app, connection, "tmcp-raw-session")
	client := &fakeTunnelMCPProxyClient{
		rawResult: mcpproxy.RawResult{
			Result:          json.RawMessage(`{"contents":[{"uri":"file:///README.md","mimeType":"text/markdown","text":"hello"}]}`),
			ResultSize:      84,
			BridgeSessionId: "bridge-session-raw",
			TargetClient:    "bridge-client-1",
		},
	}
	restore := setTunnelMCPProxyClientForTest(client)
	defer restore()

	result, err := CallTunnelMCPRaw(TunnelMCPRawRequest{
		Context:       context.Background(),
		UserId:        100,
		TokenId:       10,
		Slug:          "local-user-mcp",
		ConnectionKey: "tc_test_connection_key_100",
		RequestId:     "raw-resource-1",
		SessionId:     "tmcp-raw-session",
		Method:        dto.MCPMethodResourcesRead,
		Params:        json.RawMessage(`{"uri":"file:///README.md"}`),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"contents":[{"uri":"file:///README.md","mimeType":"text/markdown","text":"hello"}]}`, string(result))
	require.Equal(t, 1, client.rawCalled)
	require.Equal(t, dto.MCPMethodResourcesRead, client.rawReqs[0].Method)
	require.JSONEq(t, `{"uri":"file:///README.md"}`, string(client.rawReqs[0].Params))
	require.Contains(t, client.servers[0].Endpoint, "bridge://bridge-client-1?target=")

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "request_id = ?", "raw-resource-1").Error)
	require.Equal(t, "resources_read", audit.Action)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, dto.MCPMethodResourcesRead, audit.Method)
	require.Equal(t, connection.Id, audit.ConnectionId)
	require.Equal(t, connection.KeyPrefix, audit.ConnectionKeyPrefix)
	require.Equal(t, int64(len(`{"uri":"file:///README.md"}`)), audit.BytesIn)
	require.Equal(t, int64(84), audit.BytesOut)
	require.Equal(t, "tmcp-raw-session", audit.SessionId)
	require.Contains(t, audit.MetadataJson, `"bridge_session_id":"bridge-session-raw"`)
	require.Contains(t, audit.MetadataJson, `"target_client":"bridge-client-1"`)
}
