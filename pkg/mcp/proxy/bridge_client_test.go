package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestParseBridgeEndpoint(t *testing.T) {
	endpoint, err := ParseBridgeEndpoint("bridge://qidian-client?target=http%3A%2F%2F127.0.0.1%3A8765%2Fmcp")
	require.NoError(t, err)
	require.Equal(t, "qidian-client", endpoint.ClientId)
	require.Equal(t, "http://127.0.0.1:8765/mcp", endpoint.Target)

	legacy, err := ParseBridgeEndpoint("qidian_browser://legacy-client?target=http%3A%2F%2F127.0.0.1%3A8766%2Fmcp")
	require.NoError(t, err)
	require.Equal(t, "legacy-client", legacy.ClientId)
	require.Equal(t, "http://127.0.0.1:8766/mcp", legacy.Target)

	plain, err := ParseBridgeEndpoint("qidian-client")
	require.NoError(t, err)
	require.Equal(t, "qidian-client", plain.ClientId)
	require.Empty(t, plain.Target)

	auto, err := ParseBridgeEndpoint("bridge://auto?target=http%3A%2F%2F127.0.0.1%3A8767%2Fmcp")
	require.NoError(t, err)
	require.True(t, auto.IsAuto())
	require.Equal(t, "http://127.0.0.1:8767/mcp", auto.Target)

	_, err = ParseBridgeEndpoint("http://127.0.0.1:8765/mcp")
	require.Error(t, err)
}

func TestBridgeClientListToolsParsesMetadata(t *testing.T) {
	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-1",
		ClientId:     "client-1",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		Send:         outbound,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		require.Equal(t, bridge.MessageTypeToolCall, msg.Type)
		require.Equal(t, BridgeToolMCPProxyListTools, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"tools": []map[string]any{
					{
						"name":        "search_repos",
						"title":       "Search Repos",
						"description": "Search repositories",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		}))
	}()

	client := NewBridgeClient(hub)
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://client-1",
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "search_repos", tools[0].Name)
	require.Equal(t, "Search Repos", tools[0].Title)
	require.Equal(t, "object", tools[0].InputSchema["type"])
	<-done
}

func TestBridgeClientListToolsAutoSelectsLatestActiveSession(t *testing.T) {
	originalDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = originalDB
	})

	hub := bridge.NewHub()
	base := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	oldOutbound := make(chan bridge.OutboundMessage, 1)
	newOutbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-old",
		ClientId:     "client-old",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(time.Minute),
		Send:         oldOutbound,
	})
	hub.Register(bridge.Session{
		SessionId:    "session-new",
		ClientId:     "client-new",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(2 * time.Minute),
		Send:         newOutbound,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-newOutbound
		require.Equal(t, BridgeToolMCPProxyListTools, msg.Data.(dto.BridgeToolCallRequest).ToolName)
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"tools": []map[string]any{
					{
						"name":        "status",
						"description": "Status",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			},
		}))
	}()

	client := NewBridgeClient(hub)
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://auto?target=http%3A%2F%2F127.0.0.1%3A8765%2Fmcp",
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "status", tools[0].Name)
	<-done
	select {
	case msg := <-oldOutbound:
		t.Fatalf("older session should not receive first auto call: %#v", msg)
	default:
	}
}

func TestBridgeClientCallToolAutoFailoversToNextEligibleSession(t *testing.T) {
	originalDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = originalDB
	})

	hub := bridge.NewHub()
	base := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	backupOutbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-unavailable",
		ClientId:     "client-unavailable",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(2 * time.Minute),
	})
	hub.Register(bridge.Session{
		SessionId:    "session-backup",
		ClientId:     "client-backup",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		ConnectedAt:  base,
		LastSeenAt:   base.Add(time.Minute),
		Send:         backupOutbound,
	})

	done := make(chan error, 1)
	var result CallResult
	go func() {
		var err error
		result, err = NewBridgeClient(hub).CallTool(context.Background(), model.MCPProxyServer{
			Transport: model.MCPProxyTransportQidianBrowser,
			Endpoint:  "bridge://*?target=http%3A%2F%2F127.0.0.1%3A8765%2Fmcp",
		}, CallRequest{
			ToolName:  "status",
			Arguments: map[string]any{"verbose": true},
			RequestId: "auto-failover",
			UserId:    1,
			TokenId:   2,
		})
		done <- err
	}()

	msg := <-backupOutbound
	req := msg.Data.(dto.BridgeToolCallRequest)
	require.Equal(t, BridgeToolMCPProxyCallTool, req.ToolName)
	require.Equal(t, "status", req.Arguments["name"])
	require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: "ok"}},
		Summary:    "ok",
		ResultSize: 2,
	}))
	require.NoError(t, <-done)
	require.Equal(t, "ok", result.Summary)
	require.Equal(t, "session-backup", result.BridgeSessionId)
	require.Equal(t, "client-backup", result.TargetClient)
}

func TestBridgeClientRequiresMCPProxyCapability(t *testing.T) {
	hub := bridge.NewHub()
	hub.Register(bridge.Session{
		SessionId:    "session-1",
		ClientId:     "client-1",
		UserId:       1,
		Capabilities: []string{"remote_read"},
		Send:         make(chan bridge.OutboundMessage, 1),
	})

	client := NewBridgeClient(hub)
	_, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://client-1",
	})
	require.ErrorIs(t, err, bridge.ErrClientNotFound)
}

func TestBridgeClientEnforcesMCPTargetPolicy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BridgeClient{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	rawPolicy, err := bridgepolicy.Marshal(bridgepolicy.Policy{
		AllowedTools:      []string{"mcp_proxy"},
		MCPAllowedTargets: []string{"https://allowed.example/mcp"},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BridgeClient{
		ClientId: "client-1",
		UserId:   1,
		Policy:   rawPolicy,
		Status:   model.BridgeClientStatusOnline,
	}).Error)

	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-1",
		ClientId:     "client-1",
		UserId:       1,
		Capabilities: []string{BridgeCapabilityMCPProxy},
		Send:         outbound,
	})

	client := NewBridgeClient(hub)
	_, err = client.ListTools(context.Background(), model.MCPProxyServer{
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://client-1?target=https%3A%2F%2Fblocked.example%2Fmcp",
	})
	require.Equal(t, bridgepolicy.ErrorCodeMCPTargetForbidden, bridgepolicy.ErrorCode(err))
	select {
	case msg := <-outbound:
		t.Fatalf("target policy denial should not forward to bridge daemon: %#v", msg)
	default:
	}
}
