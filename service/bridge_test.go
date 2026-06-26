package service

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/stretchr/testify/require"
)

func TestBridgeClientServiceSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the bridge client service smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = bridge.DefaultHub.Unregister("missing")
		_ = model.DB.Where("request_id = ?", "bridge-audit-service-smoke").Delete(&model.BridgeAuditLog{}).Error
		_ = model.DB.Where("client_id = ?", "bridge-service-smoke").Delete(&model.BridgeSession{}).Error
		_ = model.DB.Unscoped().Where("client_id = ?", "bridge-service-smoke").Delete(&model.BridgeClient{}).Error
		_ = model.CloseDB()
	})

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	result, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		RequestIP:    "127.0.0.1",
		UserAgent:    "bridge-test",
		ClientId:     "bridge-service-smoke",
		Name:         "Smoke Bridge",
		Version:      "0.1.0",
		Platform:     "darwin",
		Workspace:    "/tmp/project",
		Capabilities: []string{"remote_read"},
	})
	if err != nil {
		t.Fatalf("RegisterBridgeClient failed: %v", err)
	}
	if result.SessionId == "" {
		t.Fatal("session id is required")
	}
	bridge.DefaultHub.Register(bridge.Session{
		SessionId:    result.SessionId,
		ClientId:     "bridge-service-smoke",
		UserId:       user.Id,
		TokenId:      token.Id,
		Name:         "Smoke Bridge",
		Version:      "0.1.0",
		Platform:     "darwin",
		Workspace:    "/tmp/project",
		Capabilities: []string{"remote_read"},
	})
	if bridge.DefaultHub.Count() < 1 {
		t.Fatal("expected bridge hub to contain session")
	}

	items, total, err := ListBridgeClients(BridgeClientListParams{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListBridgeClients failed: %v", err)
	}
	if total < 1 || len(items) < 1 {
		t.Fatalf("expected bridge client, total=%d len=%d", total, len(items))
	}
	if !items[0].Online {
		t.Fatalf("expected online bridge client, got %#v", items[0])
	}

	detail, err := GetBridgeClientDetail(BridgeClientDetailParams{
		UserId:   user.Id,
		ClientId: "bridge-service-smoke",
	})
	if err != nil {
		t.Fatalf("GetBridgeClientDetail failed: %v", err)
	}
	if detail.Client.ClientId != "bridge-service-smoke" || detail.OnlineSession == nil {
		t.Fatalf("bridge client detail mismatch: %#v", detail)
	}
	if len(detail.RecentSessions) < 1 || detail.RecentSessions[0].SessionId != result.SessionId {
		t.Fatalf("expected recent bridge session in detail: %#v", detail.RecentSessions)
	}

	newName := "Updated Smoke Bridge"
	newCapabilities := []string{"remote_read", "remote_search"}
	updatedDetail, err := UpdateBridgeClient(BridgeClientUpdateParams{
		UserId:   user.Id,
		ClientId: "bridge-service-smoke",
		Request: dto.BridgeClientUpdateRequest{
			Name:         &newName,
			Capabilities: &newCapabilities,
		},
	})
	if err != nil {
		t.Fatalf("UpdateBridgeClient failed: %v", err)
	}
	if updatedDetail.Client.Name != newName || len(updatedDetail.Client.Capabilities) != 2 {
		t.Fatalf("bridge client update mismatch: %#v", updatedDetail.Client)
	}
	snapshot, ok := bridge.DefaultHub.GetByClient("bridge-service-smoke")
	if !ok || snapshot.Name != newName || len(snapshot.Capabilities) != 2 {
		t.Fatalf("expected hub metadata sync, ok=%v snapshot=%#v", ok, snapshot)
	}

	if err := TouchBridgeClientSession(result.SessionId); err != nil {
		t.Fatalf("TouchBridgeClientSession failed: %v", err)
	}
	closedSession, err := CloseBridgeSessionForAdmin(BridgeSessionCloseParams{
		UserId:    user.Id,
		SessionId: result.SessionId,
		Reason:    "smoke done",
	})
	if err != nil {
		t.Fatalf("CloseBridgeSessionForAdmin failed: %v", err)
	}
	if closedSession.Status != model.BridgeSessionStatusClosed || closedSession.CloseReason != "smoke done" {
		t.Fatalf("closed session mismatch: %#v", closedSession)
	}
	if _, ok := bridge.DefaultHub.GetByClient("bridge-service-smoke"); ok {
		t.Fatal("expected admin close to remove bridge hub session")
	}

	var client model.BridgeClient
	if err := model.DB.Where("client_id = ?", "bridge-service-smoke").First(&client).Error; err != nil {
		t.Fatalf("bridge client not found: %v", err)
	}
	if client.Status != model.BridgeClientStatusOffline {
		t.Fatalf("expected offline client, got %d", client.Status)
	}

	audit := &model.BridgeAuditLog{
		RequestId:   "bridge-audit-service-smoke",
		SessionId:   result.SessionId,
		ClientId:    "bridge-service-smoke",
		UserId:      user.Id,
		TokenId:     token.Id,
		ToolName:    "remote_read",
		RequestBody: `{"file_path":"README.md"}`,
		Status:      model.BridgeAuditStatusSuccess,
		DurationMS:  12,
		ResultSize:  34,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		t.Fatalf("CreateBridgeAuditLog failed: %v", err)
	}
	auditItems, auditTotal, err := ListBridgeAuditLogs(BridgeAuditLogListParams{
		UserId:    user.Id,
		ClientId:  "bridge-service-smoke",
		ToolName:  "remote_read",
		Status:    model.BridgeAuditStatusSuccess,
		RequestId: "bridge-audit-service-smoke",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListBridgeAuditLogs failed: %v", err)
	}
	if auditTotal != 1 || len(auditItems) != 1 {
		t.Fatalf("expected one bridge audit log, total=%d len=%d", auditTotal, len(auditItems))
	}
	if auditItems[0].RequestId != "bridge-audit-service-smoke" || auditItems[0].DurationMS != 12 || auditItems[0].ResultSize != 34 {
		t.Fatalf("bridge audit log item mismatch: %#v", auditItems[0])
	}

	health, err := GetBridgeClientHealth(BridgeClientHealthParams{
		UserId:        user.Id,
		ClientId:      "bridge-service-smoke",
		WindowSeconds: 24 * 60 * 60,
	})
	if err != nil {
		t.Fatalf("GetBridgeClientHealth failed: %v", err)
	}
	if health.ClientId != "bridge-service-smoke" || health.Calls.TotalRequests < 1 || health.Calls.Success < 1 {
		t.Fatalf("bridge client health mismatch: %#v", health)
	}
	if len(health.RecentSessions) < 1 || health.RecentSessions[0].SessionId != result.SessionId {
		t.Fatalf("expected recent bridge session in health: %#v", health.RecentSessions)
	}

	archived, err := ArchiveBridgeClient(BridgeClientDetailParams{
		UserId:   user.Id,
		ClientId: "bridge-service-smoke",
	})
	if err != nil {
		t.Fatalf("ArchiveBridgeClient failed: %v", err)
	}
	if archived.ClientId != "bridge-service-smoke" || archived.Status != model.BridgeClientStatusOffline {
		t.Fatalf("archived bridge client mismatch: %#v", archived)
	}
}

func TestRegisterBridgeClientRejectsCrossUserClientId(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	owner, ownerToken := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	intruder, intruderToken := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	registered, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       owner.Id,
		TokenId:      ownerToken.Id,
		ClientId:     "bridge-owner-lock",
		Name:         "Owner Bridge",
		Capabilities: []string{"remote_read"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, registered.SessionId)

	_, err = RegisterBridgeClient(BridgeRegisterInput{
		UserId:       intruder.Id,
		TokenId:      intruderToken.Id,
		ClientId:     "bridge-owner-lock",
		Name:         "Intruder Bridge",
		Capabilities: []string{"remote_read"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered by another user")

	var active model.BridgeClient
	require.NoError(t, model.DB.Where("client_id = ?", "bridge-owner-lock").First(&active).Error)
	require.Equal(t, owner.Id, active.UserId)

	require.NoError(t, model.DB.Delete(&active).Error)
	_, err = RegisterBridgeClient(BridgeRegisterInput{
		UserId:       intruder.Id,
		TokenId:      intruderToken.Id,
		ClientId:     "bridge-owner-lock",
		Name:         "Intruder Bridge",
		Capabilities: []string{"remote_read"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered by another user")

	var archived model.BridgeClient
	require.NoError(t, model.DB.Unscoped().Where("client_id = ?", "bridge-owner-lock").First(&archived).Error)
	require.Equal(t, owner.Id, archived.UserId)
}

func TestUpdateBridgeClientHealthReturnedByHealthAPI(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	_, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		ClientId:     "bridge-health-report",
		Name:         "Health Bridge",
		Version:      "0.1.0",
		Platform:     "darwin-arm64",
		Workspace:    "/tmp/project",
		Capabilities: []string{"remote_read"},
	})
	require.NoError(t, err)

	err = UpdateBridgeClientHealth("bridge-health-report", dto.BridgeAgentHealthReport{
		GeneratedAt: 12345,
		Version:     "0.2.0",
		Platform:    "darwin-arm64",
		Workspace:   "/tmp/project",
		Checks: []dto.BridgeAgentHealthCheck{
			{Name: "workspace", Status: "ok", Detail: "/tmp/project"},
			{Name: "mcp_server.coding", Status: "warning", Detail: "not reachable"},
		},
		MCPProcesses: []dto.BridgeAgentMCPProcess{
			{
				Name:        "coding",
				Transport:   "stdio",
				Status:      "running",
				PID:         2345,
				Initialized: true,
				StderrClass: "crash",
				ExitError:   strings.Repeat("x", 400),
				Detail:      "command_prefix=npx",
			},
			{
				Name:      "broken",
				Transport: "stdio",
				Status:    "invalid",
				Detail:    "stdio command is empty",
			},
		},
	})
	require.NoError(t, err)

	health, err := GetBridgeClientHealth(BridgeClientHealthParams{
		UserId:        user.Id,
		ClientId:      "bridge-health-report",
		WindowSeconds: 24 * 60 * 60,
	})
	require.NoError(t, err)
	require.NotNil(t, health.AgentHealth)
	require.Equal(t, int64(12345), health.AgentHealth.GeneratedAt)
	require.Equal(t, "0.2.0", health.AgentHealth.Version)
	require.Equal(t, "warn", health.AgentHealth.Summary.Status)
	require.Equal(t, 2, health.AgentHealth.Summary.Total)
	require.Equal(t, 1, health.AgentHealth.Summary.OK)
	require.Equal(t, 1, health.AgentHealth.Summary.Warn)
	require.Equal(t, "warn", health.AgentHealth.Checks[1].Status)
	require.Len(t, health.AgentHealth.MCPProcesses, 2)
	require.Equal(t, "running", health.AgentHealth.MCPProcesses[0].Status)
	require.Equal(t, 2345, health.AgentHealth.MCPProcesses[0].PID)
	require.True(t, health.AgentHealth.MCPProcesses[0].Initialized)
	require.Equal(t, "config_error", health.AgentHealth.MCPProcesses[1].Status)
	require.LessOrEqual(t, len(health.AgentHealth.MCPProcesses[0].ExitError), 256)
}

func TestCloseOldBridgeSessionKeepsReconnectedClientOnline(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	clientId := "bridge-reconnect-race"
	var firstSessionId, secondSessionId string
	t.Cleanup(func() {
		if firstSessionId != "" {
			_, _ = bridge.DefaultHub.Unregister(firstSessionId)
		}
		if secondSessionId != "" {
			_, _ = bridge.DefaultHub.Unregister(secondSessionId)
		}
	})

	first, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		ClientId:     clientId,
		Name:         "Reconnect Bridge",
		Capabilities: []string{"remote_read"},
	})
	require.NoError(t, err)
	firstSessionId = first.SessionId
	bridge.DefaultHub.Register(bridge.Session{
		SessionId:    first.SessionId,
		ClientId:     clientId,
		UserId:       user.Id,
		TokenId:      token.Id,
		Name:         "Reconnect Bridge",
		Capabilities: []string{"remote_read"},
	})

	second, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		ClientId:     clientId,
		Name:         "Reconnect Bridge New",
		Capabilities: []string{"remote_read"},
	})
	require.NoError(t, err)
	secondSessionId = second.SessionId
	require.NotEqual(t, first.SessionId, second.SessionId)

	require.NoError(t, CloseBridgeClientSession(first.SessionId, "old websocket closed after reconnect"))
	var client model.BridgeClient
	require.NoError(t, model.DB.Where("client_id = ?", clientId).First(&client).Error)
	require.Equal(t, model.BridgeClientStatusOnline, client.Status)

	var oldSession model.BridgeSession
	require.NoError(t, model.DB.Where("session_id = ?", first.SessionId).First(&oldSession).Error)
	require.Equal(t, model.BridgeSessionStatusClosed, oldSession.Status)

	bridge.DefaultHub.Register(bridge.Session{
		SessionId:    second.SessionId,
		ClientId:     clientId,
		UserId:       user.Id,
		TokenId:      token.Id,
		Name:         "Reconnect Bridge New",
		Capabilities: []string{"remote_read"},
	})
	require.NoError(t, CloseBridgeClientSession(second.SessionId, "new websocket closed"))
	require.NoError(t, model.DB.Where("client_id = ?", clientId).First(&client).Error)
	require.Equal(t, model.BridgeClientStatusOffline, client.Status)
}

func TestUpdateBridgeClientPolicyAndPreserveOnReconnect(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	registered, err := RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		ClientId:     "bridge-policy-client",
		Name:         "Policy Bridge",
		Capabilities: []string{"remote_read", "remote_write", "mcp_proxy"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, registered.SessionId)

	policy := bridgepolicy.Policy{
		AllowedTools:      []string{"remote_read", "mcp_proxy"},
		AllowWrite:        false,
		MaxResultBytes:    1024,
		MaxScanFileBytes:  2048,
		MaxResults:        20,
		TreeDepth:         3,
		WalkDepth:         4,
		MCPAllowedTargets: []string{"https://mcp.example.com/rpc"},
	}
	detail, err := UpdateBridgeClient(BridgeClientUpdateParams{
		UserId:   user.Id,
		ClientId: "bridge-policy-client",
		Request: dto.BridgeClientUpdateRequest{
			Policy: &policy,
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"remote_read", "mcp_proxy"}, detail.Client.Policy.AllowedTools)
	require.Equal(t, 1024, detail.Client.Policy.MaxResultBytes)
	require.Equal(t, []string{"https://mcp.example.com/rpc"}, detail.Client.Policy.MCPAllowedTargets)

	_, err = RegisterBridgeClient(BridgeRegisterInput{
		UserId:       user.Id,
		TokenId:      token.Id,
		ClientId:     "bridge-policy-client",
		Name:         "Policy Bridge Reconnected",
		Capabilities: []string{"remote_read", "remote_write", "mcp_proxy"},
	})
	require.NoError(t, err)

	preserved, err := GetBridgeClientDetail(BridgeClientDetailParams{
		UserId:   user.Id,
		ClientId: "bridge-policy-client",
	})
	require.NoError(t, err)
	require.Equal(t, policy.MaxScanFileBytes, preserved.Client.Policy.MaxScanFileBytes)
	require.False(t, preserved.Client.Policy.AllowWrite)
}
