package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTunnelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	resetTunnelRateLimiterForTest()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.BridgeClient{},
		&model.BridgeAuditLog{},
		&model.BridgeAgentSetupToken{},
		&model.TunnelApp{},
		&model.TunnelConnection{},
		&model.TunnelSession{},
		&model.TunnelRoute{},
		&model.TunnelAuditLog{},
		&model.BillingEvent{},
	))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
	})
	return db
}

func seedTunnelBridgeClient(t *testing.T, userId int, clientId string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.BridgeClient{
		ClientId: clientId,
		UserId:   userId,
	}).Error)
}

func approveTunnelAppForTest(t *testing.T, app dto.TunnelAppItem) dto.TunnelAppItem {
	t.Helper()
	seedTunnelBridgeClient(t, app.UserId, app.BridgeClientId)
	status := model.TunnelAppStatusApproved
	approved, err := UpdateTunnelAppForAdmin(app.Id, 1, dto.TunnelAppAdminUpdateRequest{
		Status: &status,
	})
	require.NoError(t, err)
	return approved
}

func TestCreateAndApproveMCPCodeTunnelApp(t *testing.T) {
	db := setupTunnelTestDB(t)
	rawPolicy, err := bridgepolicy.Marshal(bridgepolicy.Policy{
		MaxResultBytes: 4096,
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.BridgeClient{
		ClientId: "bridge-local-code",
		UserId:   100,
		Policy:   rawPolicy,
	}).Error)

	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
		Policy: map[string]any{
			"workspace_root": "/workspace/project",
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), item.Id)
	require.Equal(t, model.TunnelAppTypeMCPCode, item.AppType)
	require.Equal(t, model.TunnelPermissionWrite, item.PermissionMode)
	require.Equal(t, model.TunnelAppStatusPending, item.Status)
	require.Equal(t, "/mcp", item.TargetPath)
	require.NotEmpty(t, item.PublicSlug)
	require.Equal(t, "/workspace/project", item.Policy["workspace_root"])

	note := "approved for test workspace"
	status := model.TunnelAppStatusApproved
	approved, err := UpdateTunnelAppForAdmin(item.Id, 1, dto.TunnelAppAdminUpdateRequest{
		Status:     &status,
		ReviewNote: &note,
	})
	require.NoError(t, err)
	require.Equal(t, model.TunnelAppStatusApproved, approved.Status)
	require.Equal(t, 1, approved.ApprovedBy)
	require.NotZero(t, approved.ApprovedAt)

	var client model.BridgeClient
	require.NoError(t, db.First(&client, "client_id = ?", "bridge-local-code").Error)
	policy, err := bridgepolicy.Parse(client.Policy)
	require.NoError(t, err)
	require.True(t, policy.AllowWrite)
	require.Contains(t, policy.AllowedTools, "mcp_proxy")
	require.Contains(t, policy.AllowedTools, "remote_write")
	require.Contains(t, policy.AllowedTools, "remote_edit")
	require.NotContains(t, policy.AllowedTools, "remote_exec")
	require.Equal(t, 4096, policy.MaxResultBytes)

	var auditCount int64
	require.NoError(t, db.Model(&model.TunnelAuditLog{}).Where("app_id = ?", item.Id).Count(&auditCount).Error)
	require.Equal(t, int64(2), auditCount)
}

func TestTunnelConnectionCreateListAndRevoke(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)

	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name:           "Desktop Codex",
		PermissionMode: model.TunnelPermissionReadOnly,
		Config: map[string]any{
			"rate_limit": map[string]any{
				"max_requests_per_minute": 10,
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.ConnectionKey)
	require.True(t, strings.HasPrefix(resp.ConnectionKey, "tc_"))
	require.Contains(t, resp.EndpointPath, "/t/"+resp.ConnectionKey+"/tunnel/mcp/"+item.PublicSlug)
	require.Equal(t, resp.ConnectionKey[:12], resp.Connection.KeyPrefix)
	require.Equal(t, model.TunnelPermissionReadOnly, resp.Connection.PermissionMode)
	require.Equal(t, model.TunnelConnectionStatusActive, resp.Connection.Status)
	require.Equal(t, float64(10), resp.Connection.Config["rate_limit"].(map[string]any)["max_requests_per_minute"])

	connections, total, err := ListTunnelConnections(TunnelConnectionListParams{
		UserId: 100,
		AppId:  item.Id,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, connections, 1)
	require.Contains(t, connections[0].EndpointPath, "/t/<connection_key>/tunnel/mcp/")
	require.Equal(t, float64(10), connections[0].Config["rate_limit"].(map[string]any)["max_requests_per_minute"])

	revoked, err := RevokeTunnelConnectionForUser(item.Id, resp.Connection.Id, 100)
	require.NoError(t, err)
	require.Equal(t, model.TunnelConnectionStatusRevoked, revoked.Status)
	require.NotZero(t, revoked.RevokedAt)
}

func TestUpdateTunnelConnectionForUser(t *testing.T) {
	db := setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)
	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.NoError(t, err)

	expiresAt := int64(1999999999)
	name := "Desktop Codex Tight"
	updated, err := UpdateTunnelConnectionForUser(item.Id, resp.Connection.Id, 100, dto.TunnelConnectionUpdateRequest{
		Name:      &name,
		ExpiresAt: &expiresAt,
		Config: map[string]any{
			"rate_limit": map[string]any{
				"max_requests_per_minute":  5,
				"max_bytes_out_per_minute": 4096,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)
	require.Equal(t, expiresAt, updated.ExpiresAt)
	require.Equal(t, float64(5), updated.Config["rate_limit"].(map[string]any)["max_requests_per_minute"])
	require.Equal(t, float64(4096), updated.Config["rate_limit"].(map[string]any)["max_bytes_out_per_minute"])

	var audit model.TunnelAuditLog
	require.NoError(t, db.First(&audit, "connection_id = ? AND action = ?", resp.Connection.Id, model.TunnelAuditActionUpdate).Error)
	require.Equal(t, "allow", audit.Decision)
	require.Equal(t, "user_updated_tunnel_connection", audit.Reason)
	require.Contains(t, audit.MetadataJson, "config_json")
}

func TestListTunnelSessionsForConnection(t *testing.T) {
	db := setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)
	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.NoError(t, err)

	require.NoError(t, db.Create(&model.TunnelSession{
		AppId:          item.Id,
		UserId:         100,
		ConnectionId:   resp.Connection.Id,
		ConnectionName: resp.Connection.Name,
		KeyPrefix:      resp.Connection.KeyPrefix,
		SessionId:      "tmcp_visible",
		BridgeClientId: item.BridgeClientId,
		Status:         model.TunnelSessionStatusOnline,
		ClientVersion:  "codex@1.0.0",
		ClientIp:       "127.0.0.1",
		UserAgent:      "codex-test",
		BytesIn:        12,
		BytesOut:       34,
	}).Error)

	items, total, err := ListTunnelSessions(TunnelSessionListParams{
		UserId:       100,
		AppId:        item.Id,
		ConnectionId: resp.Connection.Id,
		Status:       model.TunnelSessionStatusOnline,
		Keyword:      "codex",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "tmcp_visible", items[0].SessionId)
	require.Equal(t, resp.Connection.Id, items[0].ConnectionId)
	require.Equal(t, resp.Connection.KeyPrefix, items[0].KeyPrefix)
	require.Equal(t, int64(12), items[0].BytesIn)
	require.Equal(t, int64(34), items[0].BytesOut)

	_, _, err = ListTunnelSessions(TunnelSessionListParams{
		UserId:       101,
		AppId:        item.Id,
		ConnectionId: resp.Connection.Id,
	})
	require.Error(t, err)
}

func TestEnsureTunnelAgentSetupCreatesReusesAndRotatesToken(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)
	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.NoError(t, err)

	setup, err := EnsureTunnelAgentSetup(TunnelAgentSetupParams{
		UserId:  100,
		AppId:   item.Id,
		BaseURL: "https://dp.example",
		Request: dto.TunnelAgentSetupRequest{
			ConnectionId: resp.Connection.Id,
			ClientName:   "Desktop Agent",
			Platform:     "darwin",
			Workspace:    "/workspace/project",
			Version:      "1.0.0",
		},
	})
	require.NoError(t, err)
	require.True(t, setup.Created)
	require.False(t, setup.Rotated)
	require.True(t, setup.APIKeyOnce)
	require.NotEmpty(t, setup.APIKey)
	require.True(t, strings.HasPrefix(setup.APIKey, "sk-"))
	require.Equal(t, "https://dp.example", setup.BaseURL)
	require.Equal(t, "wss://dp.example/bridge/ws", setup.BridgeWSURL)
	require.Equal(t, "https://dp.example/t/<connection_key>/tunnel/mcp/"+item.PublicSlug, setup.MCPURL)
	require.Equal(t, "bridge-local-code", setup.ClientId)
	require.Equal(t, setup.TokenId, setup.Connection.AgentTokenId)
	require.Equal(t, "Bearer "+setup.APIKey, setup.Headers["Authorization"])

	reused, err := EnsureTunnelAgentSetup(TunnelAgentSetupParams{
		UserId:  100,
		AppId:   item.Id,
		BaseURL: "https://dp.example",
		Request: dto.TunnelAgentSetupRequest{
			ConnectionId: resp.Connection.Id,
		},
	})
	require.NoError(t, err)
	require.False(t, reused.Created)
	require.False(t, reused.Rotated)
	require.False(t, reused.APIKeyOnce)
	require.Empty(t, reused.APIKey)
	require.Equal(t, setup.TokenId, reused.TokenId)
	require.Equal(t, "Bearer sk-<api_key>", reused.Headers["Authorization"])

	rotated, err := EnsureTunnelAgentSetup(TunnelAgentSetupParams{
		UserId:  100,
		AppId:   item.Id,
		BaseURL: "https://dp.example",
		Request: dto.TunnelAgentSetupRequest{
			ConnectionId: resp.Connection.Id,
			Rotate:       true,
		},
	})
	require.NoError(t, err)
	require.True(t, rotated.Created)
	require.True(t, rotated.Rotated)
	require.NotEqual(t, setup.TokenId, rotated.TokenId)
	require.NotEmpty(t, rotated.APIKey)

	oldToken, err := model.GetTokenByIds(setup.TokenId, 100)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, oldToken.Status)

	logs, total, err := ListTunnelAuditLogs(TunnelAuditLogListParams{
		UserId:       100,
		AppId:        item.Id,
		ConnectionId: resp.Connection.Id,
		Action:       model.TunnelAuditActionAgentSetup,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, logs, 3)
	require.Equal(t, model.TunnelAuditActionAgentSetup, logs[0].Action)
	require.Equal(t, float64(rotated.TokenId), logs[0].Metadata["token_id"])
}

func TestRevokeTunnelConnectionDisablesAgentToken(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionReadOnly,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)
	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.NoError(t, err)
	setup, err := EnsureTunnelAgentSetup(TunnelAgentSetupParams{
		UserId:  100,
		AppId:   item.Id,
		BaseURL: "https://dp.example",
		Request: dto.TunnelAgentSetupRequest{
			ConnectionId: resp.Connection.Id,
		},
	})
	require.NoError(t, err)

	_, err = RevokeTunnelConnectionForUser(item.Id, resp.Connection.Id, 100)
	require.NoError(t, err)
	token, err := model.GetTokenByIds(setup.TokenId, 100)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, token.Status)
}

func TestEnsureBridgeAgentSetupCreatesReservedClientReusesAndRotates(t *testing.T) {
	_ = setupTunnelTestDB(t)

	setup, err := EnsureBridgeAgentSetup(BridgeAgentSetupParams{
		UserId:  100,
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupRequest{
			ClientName: "Desktop Bridge Agent",
			Platform:   "darwin",
			Workspace:  "/workspace/project",
			Version:    "1.0.0",
		},
	})
	require.NoError(t, err)
	require.True(t, setup.Created)
	require.False(t, setup.Rotated)
	require.True(t, setup.APIKeyOnce)
	require.NotEmpty(t, setup.APIKey)
	require.True(t, strings.HasPrefix(setup.APIKey, "sk-"))
	require.True(t, strings.HasPrefix(setup.ClientId, "bridge-"))
	require.Equal(t, "https://dp.example", setup.BaseURL)
	require.Equal(t, "wss://dp.example/bridge/ws", setup.BridgeWSURL)
	require.Equal(t, setup.TokenId, setup.Client.TokenId)
	require.Equal(t, "Desktop Bridge Agent", setup.Client.Name)
	require.Equal(t, model.BridgeClientStatusOffline, setup.Client.Status)
	require.Contains(t, setup.Client.Capabilities, "local_agent")
	require.Equal(t, "Bearer "+setup.APIKey, setup.Headers["Authorization"])

	client, err := model.GetBridgeClientByClientId(setup.ClientId)
	require.NoError(t, err)
	require.Equal(t, setup.TokenId, client.TokenId)
	require.Equal(t, 100, client.UserId)

	reused, err := EnsureBridgeAgentSetup(BridgeAgentSetupParams{
		UserId:  100,
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupRequest{
			ClientId: setup.ClientId,
		},
	})
	require.NoError(t, err)
	require.False(t, reused.Created)
	require.False(t, reused.Rotated)
	require.False(t, reused.APIKeyOnce)
	require.Empty(t, reused.APIKey)
	require.Equal(t, setup.TokenId, reused.TokenId)
	require.Equal(t, "Bearer sk-<api_key>", reused.Headers["Authorization"])

	rotated, err := EnsureBridgeAgentSetup(BridgeAgentSetupParams{
		UserId:  100,
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupRequest{
			ClientId: setup.ClientId,
			Rotate:   true,
		},
	})
	require.NoError(t, err)
	require.True(t, rotated.Created)
	require.True(t, rotated.Rotated)
	require.NotEqual(t, setup.TokenId, rotated.TokenId)
	require.NotEmpty(t, rotated.APIKey)

	oldToken, err := model.GetTokenByIds(setup.TokenId, 100)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, oldToken.Status)
}

func TestBridgeAgentSetupTokenCreateConsumeOnce(t *testing.T) {
	_ = setupTunnelTestDB(t)

	created, err := CreateBridgeAgentSetupToken(BridgeAgentSetupTokenCreateParams{
		UserId:  100,
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupTokenRequest{
			ClientName: "Desktop Bridge Agent",
			Workspace:  "/workspace/project",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.SetupToken)
	require.Contains(t, created.EnrollCommand, "--setup-token "+created.SetupToken)
	require.Equal(t, defaultBridgeAgentSetupTokenTTLSeconds, created.ExpiresInSeconds)

	consumed, err := ConsumeBridgeAgentSetupToken(BridgeAgentSetupTokenConsumeParams{
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupTokenConsumeRequest{
			SetupToken: created.SetupToken,
			Version:    "1.0.0",
			Platform:   "darwin",
		},
	})
	require.NoError(t, err)
	require.True(t, consumed.Created)
	require.True(t, consumed.APIKeyOnce)
	require.NotEmpty(t, consumed.APIKey)
	require.Equal(t, "Desktop Bridge Agent", consumed.Client.Name)
	require.Equal(t, "darwin", consumed.Client.Platform)
	require.Equal(t, "/workspace/project", consumed.Client.Workspace)

	_, err = ConsumeBridgeAgentSetupToken(BridgeAgentSetupTokenConsumeParams{
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupTokenConsumeRequest{
			SetupToken: created.SetupToken,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid, expired, or already used")
}

func TestBridgeAgentSetupTokenExpired(t *testing.T) {
	_ = setupTunnelTestDB(t)

	created, err := CreateBridgeAgentSetupToken(BridgeAgentSetupTokenCreateParams{
		UserId:  100,
		BaseURL: "https://dp.example",
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.BridgeAgentSetupToken{}).
		Where("token_hash = ?", bridgeAgentSetupTokenHash(created.SetupToken)).
		Update("expires_at", common.GetTimestamp()-1).Error)

	_, err = ConsumeBridgeAgentSetupToken(BridgeAgentSetupTokenConsumeParams{
		BaseURL: "https://dp.example",
		Request: dto.BridgeAgentSetupTokenConsumeRequest{
			SetupToken: created.SetupToken,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid, expired, or already used")
}

func TestTunnelConnectionRequiresApprovedApp(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Pending workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionReadOnly,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)

	_, err = CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "approved")
}

func TestListTunnelAuditLogsForConnection(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionWrite,
		BridgeClientId: "bridge-local-code",
	})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)
	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name: "Desktop Codex",
	})
	require.NoError(t, err)
	_, err = RevokeTunnelConnectionForUser(item.Id, resp.Connection.Id, 100)
	require.NoError(t, err)

	logs, total, err := ListTunnelAuditLogs(TunnelAuditLogListParams{
		UserId:       100,
		AppId:        item.Id,
		ConnectionId: resp.Connection.Id,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, logs, 2)
	require.Equal(t, model.TunnelAuditActionRevoke, logs[0].Action)
	require.Equal(t, model.TunnelAuditActionCreate, logs[1].Action)
	require.Equal(t, resp.Connection.KeyPrefix, logs[0].ConnectionKeyPrefix)
	require.Equal(t, model.TunnelPermissionWrite, logs[1].Metadata["permission_mode"])

	_, _, err = ListTunnelAuditLogs(TunnelAuditLogListParams{
		UserId:       101,
		AppId:        item.Id,
		ConnectionId: resp.Connection.Id,
	})
	require.Error(t, err)
}

func TestTunnelConnectionCannotExceedAppPermission(t *testing.T) {
	_ = setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionReadOnly,
	})
	require.NoError(t, err)
	item.BridgeClientId = "bridge-local-code"
	_, err = model.UpdateTunnelAppFields(item.Id, map[string]any{"bridge_client_id": "bridge-local-code"})
	require.NoError(t, err)
	item = approveTunnelAppForTest(t, item)

	_, err = CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{
		Name:           "Too powerful",
		PermissionMode: model.TunnelPermissionWrite,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot exceed")
}

func TestApproveMCPCodeTunnelAppRequiresOwnedBridgeClient(t *testing.T) {
	_ = setupTunnelTestDB(t)

	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local code workspace",
		AppType:        model.TunnelAppTypeMCPCode,
		PermissionMode: model.TunnelPermissionExecSafe,
		BridgeClientId: "missing-bridge",
	})
	require.NoError(t, err)

	status := model.TunnelAppStatusApproved
	_, err = UpdateTunnelAppForAdmin(item.Id, 1, dto.TunnelAppAdminUpdateRequest{
		Status: &status,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "record not found")
}

func TestTrafficTunnelRequiresTargetPortAndUsesTrafficPermission(t *testing.T) {
	_ = setupTunnelTestDB(t)

	_, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:    "Preview without port",
		AppType: model.TunnelAppTypeHTTP,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "target_port")

	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:       "Local preview",
		AppType:    model.TunnelAppTypeHTTP,
		TargetPort: 5173,
		Route: map[string]any{
			"auth_mode": model.TunnelRouteAuthPrivate,
		},
	})
	require.NoError(t, err)
	require.Equal(t, model.TunnelAppTypeHTTP, item.AppType)
	require.Equal(t, model.TunnelPermissionTraffic, item.PermissionMode)
	require.Equal(t, "127.0.0.1", item.TargetHost)
	require.Equal(t, 5173, item.TargetPort)
	require.Equal(t, "/", item.TargetPath)
	require.Equal(t, model.TunnelRouteAuthPrivate, item.Route["auth_mode"])
}

func TestTCPTunnelConnectionEndpointAndBridgePolicy(t *testing.T) {
	db := setupTunnelTestDB(t)
	item, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Local SSH",
		AppType:        model.TunnelAppTypeTCP,
		BridgeClientId: "bridge-tcp",
		TargetPort:     22,
	})
	require.NoError(t, err)
	require.Equal(t, model.TunnelAppTypeTCP, item.AppType)
	require.Equal(t, model.TunnelPermissionTraffic, item.PermissionMode)
	item = approveTunnelAppForTest(t, item)

	resp, err := CreateTunnelConnectionForUser(item.Id, 100, dto.TunnelConnectionCreateRequest{Name: "TCP client"})
	require.NoError(t, err)
	require.Contains(t, resp.EndpointPath, "/t/"+resp.ConnectionKey+"/tunnel/tcp/"+item.PublicSlug)
	connections, total, err := ListTunnelConnections(TunnelConnectionListParams{UserId: 100, AppId: item.Id})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Contains(t, connections[0].EndpointPath, "/t/<connection_key>/tunnel/tcp/"+item.PublicSlug)

	var client model.BridgeClient
	require.NoError(t, db.First(&client, "client_id = ?", "bridge-tcp").Error)
	policy, err := bridgepolicy.Parse(client.Policy)
	require.NoError(t, err)
	require.Contains(t, policy.AllowedTools, "tcp_tunnel")
}

func TestTrafficTunnelRejectsCodePermissionMode(t *testing.T) {
	_ = setupTunnelTestDB(t)

	_, err := CreateTunnelAppForUser(100, dto.TunnelAppCreateRequest{
		Name:           "Preview with invalid mode",
		AppType:        model.TunnelAppTypeHTTP,
		PermissionMode: model.TunnelPermissionWrite,
		TargetPort:     5173,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission_mode")
}

func TestTunnelBridgePolicyPermissionModes(t *testing.T) {
	existing := bridgepolicy.Policy{
		MaxResultBytes: 2048,
	}
	appPolicy := bridgepolicy.Policy{
		MaxResults: 10,
	}

	readOnly := tunnelBridgePolicyFromMode(model.TunnelPermissionReadOnly, existing, appPolicy)
	require.False(t, readOnly.AllowWrite)
	require.Contains(t, readOnly.AllowedTools, "remote_read")
	require.NotContains(t, readOnly.AllowedTools, "remote_write")
	require.NotContains(t, readOnly.AllowedTools, "remote_run_tests")
	require.Equal(t, 2048, readOnly.MaxResultBytes)
	require.Equal(t, 10, readOnly.MaxResults)

	httpPolicy := tunnelHTTPBridgePolicy(existing, bridgepolicy.Policy{
		HTTPAllowedTargets: []string{"*"},
		HTTPDeniedTargets:  []string{"169.254.169.254"},
		HTTPDeniedPorts:    []int{3306, 6379},
	})
	require.Contains(t, httpPolicy.AllowedTools, "http_tunnel")
	require.Equal(t, []string{"169.254.169.254"}, httpPolicy.HTTPDeniedTargets)
	require.Equal(t, []int{3306, 6379}, httpPolicy.HTTPDeniedPorts)

	tcpPolicy := tunnelTCPBridgePolicy(existing, bridgepolicy.Policy{})
	require.Contains(t, tcpPolicy.AllowedTools, "tcp_tunnel")

	execSafe := tunnelBridgePolicyFromMode(model.TunnelPermissionExecSafe, existing, bridgepolicy.Policy{})
	require.True(t, execSafe.AllowWrite)
	require.Contains(t, execSafe.AllowedTools, "remote_write")
	require.Contains(t, execSafe.AllowedTools, "remote_run_tests")
	require.NotContains(t, execSafe.AllowedTools, "remote_exec")
	require.NotContains(t, execSafe.AllowedTools, "remote_shell_resize")

	execTrusted := tunnelBridgePolicyFromMode(model.TunnelPermissionExecTrusted, existing, bridgepolicy.Policy{})
	require.True(t, execTrusted.AllowWrite)
	require.Contains(t, execTrusted.AllowedTools, "remote_exec")
	require.Contains(t, execTrusted.AllowedTools, "remote_shell_eval")
	require.Contains(t, execTrusted.AllowedTools, "remote_shell_resize")
	require.Contains(t, execTrusted.AllowedTools, "remote_install_package")
}
