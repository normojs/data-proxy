package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"gorm.io/gorm"
)

type TunnelAppListParams struct {
	UserId  int
	AppType string
	Status  string
	Keyword string
	Offset  int
	Limit   int
}

type TunnelConnectionListParams struct {
	UserId  int
	AppId   int64
	Status  string
	Keyword string
	Offset  int
	Limit   int
}

type TunnelSessionListParams struct {
	UserId       int
	AppId        int64
	ConnectionId int64
	Status       string
	SessionId    string
	Keyword      string
	StartTime    int64
	EndTime      int64
	Offset       int
	Limit        int
}

type TunnelAuditLogListParams struct {
	UserId       int
	AppId        int64
	ConnectionId int64
	Action       string
	Decision     string
	RequestId    string
	ToolName     string
	SessionId    string
	Keyword      string
	StartTime    int64
	EndTime      int64
	Offset       int
	Limit        int
}

type TunnelAgentSetupParams struct {
	UserId  int
	AppId   int64
	BaseURL string
	Request dto.TunnelAgentSetupRequest
}

func CreateTunnelAppForUser(userId int, req dto.TunnelAppCreateRequest) (dto.TunnelAppItem, error) {
	if userId <= 0 {
		return dto.TunnelAppItem{}, errors.New("invalid tunnel user")
	}
	app, err := tunnelCreateRequestToModel(userId, req)
	if err != nil {
		return dto.TunnelAppItem{}, err
	}
	if err := model.CreateTunnelApp(app); err != nil {
		return dto.TunnelAppItem{}, err
	}
	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:       app.Id,
		UserId:      app.UserId,
		ActorUserId: app.UserId,
		Action:      model.TunnelAuditActionCreate,
		Decision:    "allow",
		Reason:      "user_requested_tunnel_app",
	})
	return tunnelAppToDTO(*app), nil
}

func ListTunnelApps(params TunnelAppListParams) ([]dto.TunnelAppItem, int64, error) {
	items, total, err := model.ListTunnelApps(model.TunnelAppFilter{
		UserId:  params.UserId,
		AppType: strings.TrimSpace(params.AppType),
		Status:  strings.TrimSpace(params.Status),
		Keyword: strings.TrimSpace(params.Keyword),
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.TunnelAppItem, 0, len(items))
	for _, item := range items {
		result = append(result, tunnelAppToDTO(item))
	}
	return result, total, nil
}

func GetTunnelAppForUser(id int64, userId int, isAdmin bool) (dto.TunnelAppItem, error) {
	app, err := model.GetTunnelAppById(id)
	if err != nil {
		return dto.TunnelAppItem{}, err
	}
	if !isAdmin && app.UserId != userId {
		return dto.TunnelAppItem{}, errors.New("tunnel app not found")
	}
	return tunnelAppToDTO(*app), nil
}

func UpdateTunnelAppForAdmin(id int64, actorUserId int, req dto.TunnelAppAdminUpdateRequest) (dto.TunnelAppItem, error) {
	if id <= 0 {
		return dto.TunnelAppItem{}, errors.New("invalid tunnel app id")
	}
	existing, err := model.GetTunnelAppById(id)
	if err != nil {
		return dto.TunnelAppItem{}, err
	}
	updates, err := tunnelAdminUpdateFields(*existing, actorUserId, req)
	if err != nil {
		return dto.TunnelAppItem{}, err
	}
	if len(updates) == 0 {
		return tunnelAppToDTO(*existing), nil
	}
	nextApp := applyTunnelAppUpdatesForValidation(*existing, updates)
	if err := validateTunnelAppApprovalReady(nextApp); err != nil {
		return dto.TunnelAppItem{}, err
	}
	app, err := model.UpdateTunnelAppFields(id, updates)
	if err != nil {
		return dto.TunnelAppItem{}, err
	}
	if err := syncTunnelAppBridgePolicy(*app); err != nil {
		return dto.TunnelAppItem{}, err
	}
	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:       app.Id,
		UserId:      app.UserId,
		ActorUserId: actorUserId,
		Action:      model.TunnelAuditActionReview,
		Decision:    tunnelReviewDecisionFromStatus(app.Status),
		Reason:      app.ReviewNote,
	})
	return tunnelAppToDTO(*app), nil
}

func CreateTunnelConnectionForUser(appId int64, userId int, req dto.TunnelConnectionCreateRequest) (dto.TunnelConnectionCreateResponse, error) {
	app, err := getTunnelAppOwnedByUser(appId, userId)
	if err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	if app.AppType != model.TunnelAppTypeMCPCode && app.AppType != model.TunnelAppTypeHTTP && app.AppType != model.TunnelAppTypeTCP {
		return dto.TunnelConnectionCreateResponse{}, errors.New("tunnel connections are currently supported for MCP code, HTTP tunnel, and TCP tunnel apps")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return dto.TunnelConnectionCreateResponse{}, errors.New("tunnel app must be approved before creating a connection")
	}
	permissionMode, err := normalizeTunnelConnectionPermissionMode(*app, req.PermissionMode)
	if err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	configJson, err := marshalTunnelMap(req.Config)
	if err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	if err := validateTunnelConnectionConfigJSON(configJson); err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	key := generateTunnelConnectionKey()
	connection := &model.TunnelConnection{
		AppId:          app.Id,
		UserId:         app.UserId,
		Name:           strings.TrimSpace(req.Name),
		KeyPrefix:      tunnelConnectionKeyPrefix(key),
		KeyHash:        tunnelConnectionKeyHash(key),
		PermissionMode: permissionMode,
		Status:         model.TunnelConnectionStatusActive,
		ExpiresAt:      req.ExpiresAt,
		ConfigJson:     configJson,
	}
	if connection.Name == "" {
		connection.Name = "Tunnel connection"
	}
	if err := validateTunnelConnectionName(connection.Name); err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	if err := model.CreateTunnelConnection(connection); err != nil {
		return dto.TunnelConnectionCreateResponse{}, err
	}
	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         userId,
		Action:              model.TunnelAuditActionCreate,
		Decision:            "allow",
		Reason:              "user_created_tunnel_connection",
		MetadataJson:        tunnelAuditMetadataJSON(map[string]any{"permission_mode": connection.PermissionMode, "expires_at": connection.ExpiresAt}),
	})
	item := tunnelConnectionToDTO(*connection, *app, key)
	return dto.TunnelConnectionCreateResponse{
		Connection:    item,
		ConnectionKey: key,
		EndpointPath:  item.EndpointPath,
	}, nil
}

func ListTunnelConnections(params TunnelConnectionListParams) ([]dto.TunnelConnectionItem, int64, error) {
	app, err := getTunnelAppOwnedByUser(params.AppId, params.UserId)
	if err != nil {
		return nil, 0, err
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	items, total, err := model.ListTunnelConnections(model.TunnelConnectionFilter{
		AppId:   app.Id,
		UserId:  app.UserId,
		Status:  strings.TrimSpace(params.Status),
		Keyword: strings.TrimSpace(params.Keyword),
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.TunnelConnectionItem, 0, len(items))
	for _, item := range items {
		result = append(result, tunnelConnectionToDTO(item, *app, ""))
	}
	return result, total, nil
}

func ListTunnelSessions(params TunnelSessionListParams) ([]dto.TunnelSessionItem, int64, error) {
	app, err := getTunnelAppOwnedByUser(params.AppId, params.UserId)
	if err != nil {
		return nil, 0, err
	}
	if params.ConnectionId > 0 {
		connection, err := model.GetTunnelConnectionById(params.ConnectionId)
		if err != nil {
			return nil, 0, err
		}
		if connection.AppId != app.Id || connection.UserId != app.UserId {
			return nil, 0, errors.New("tunnel connection not found")
		}
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	items, total, err := model.ListTunnelSessions(model.TunnelSessionFilter{
		AppId:        app.Id,
		ConnectionId: params.ConnectionId,
		UserId:       app.UserId,
		Status:       strings.TrimSpace(params.Status),
		SessionId:    strings.TrimSpace(params.SessionId),
		Keyword:      strings.TrimSpace(params.Keyword),
		StartTime:    params.StartTime,
		EndTime:      params.EndTime,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.TunnelSessionItem, 0, len(items))
	for _, item := range items {
		result = append(result, tunnelSessionToDTO(item))
	}
	return result, total, nil
}

func UpdateTunnelConnectionForUser(appId int64, connectionId int64, userId int, req dto.TunnelConnectionUpdateRequest) (dto.TunnelConnectionItem, error) {
	app, err := getTunnelAppOwnedByUser(appId, userId)
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	connection, err := model.GetTunnelConnectionById(connectionId)
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	if connection.AppId != app.Id || connection.UserId != app.UserId {
		return dto.TunnelConnectionItem{}, errors.New("tunnel connection not found")
	}
	updates := map[string]any{}
	if req.Name != nil {
		if err := validateTunnelConnectionName(*req.Name); err != nil {
			return dto.TunnelConnectionItem{}, err
		}
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt < 0 {
			return dto.TunnelConnectionItem{}, errors.New("invalid tunnel connection expires_at")
		}
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.Config != nil {
		configJson, err := marshalTunnelMap(req.Config)
		if err != nil {
			return dto.TunnelConnectionItem{}, err
		}
		if err := validateTunnelConnectionConfigJSON(configJson); err != nil {
			return dto.TunnelConnectionItem{}, err
		}
		updates["config_json"] = configJson
	}
	if len(updates) == 0 {
		return tunnelConnectionToDTO(*connection, *app, ""), nil
	}
	connection, err = model.UpdateTunnelConnectionFields(connection.Id, updates)
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         userId,
		Action:              model.TunnelAuditActionUpdate,
		Decision:            "allow",
		Reason:              "user_updated_tunnel_connection",
		MetadataJson: tunnelAuditMetadataJSON(map[string]any{
			"updated_fields": tunnelUpdateFieldNames(updates),
		}),
	})
	return tunnelConnectionToDTO(*connection, *app, ""), nil
}

func RevokeTunnelConnectionForUser(appId int64, connectionId int64, userId int) (dto.TunnelConnectionItem, error) {
	app, err := getTunnelAppOwnedByUser(appId, userId)
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	connection, err := model.GetTunnelConnectionById(connectionId)
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	if connection.AppId != app.Id || connection.UserId != app.UserId {
		return dto.TunnelConnectionItem{}, errors.New("tunnel connection not found")
	}
	connection, err = model.UpdateTunnelConnectionFields(connection.Id, map[string]any{
		"status":     model.TunnelConnectionStatusRevoked,
		"revoked_at": common.GetTimestamp(),
	})
	if err != nil {
		return dto.TunnelConnectionItem{}, err
	}
	if connection.AgentTokenId > 0 {
		if err := model.DisableTokenWithTx(model.DB, connection.AgentTokenId, app.UserId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.TunnelConnectionItem{}, err
		}
	}
	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         userId,
		Action:              model.TunnelAuditActionRevoke,
		Decision:            "allow",
		Reason:              "user_revoked_tunnel_connection",
	})
	return tunnelConnectionToDTO(*connection, *app, ""), nil
}

func EnsureTunnelAgentSetup(params TunnelAgentSetupParams) (dto.TunnelAgentSetupResponse, error) {
	app, err := getTunnelAppOwnedByUser(params.AppId, params.UserId)
	if err != nil {
		return dto.TunnelAgentSetupResponse{}, err
	}
	if app.AppType != model.TunnelAppTypeMCPCode {
		return dto.TunnelAgentSetupResponse{}, errors.New("agent setup is currently supported for MCP code tunnel apps")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return dto.TunnelAgentSetupResponse{}, errors.New("tunnel app must be approved before creating agent setup")
	}
	connection, err := model.GetTunnelConnectionById(params.Request.ConnectionId)
	if err != nil {
		return dto.TunnelAgentSetupResponse{}, err
	}
	if connection.AppId != app.Id || connection.UserId != app.UserId {
		return dto.TunnelAgentSetupResponse{}, errors.New("tunnel connection not found")
	}
	if connection.Status != model.TunnelConnectionStatusActive {
		return dto.TunnelAgentSetupResponse{}, errors.New("tunnel connection is not active")
	}
	if connection.ExpiresAt > 0 && connection.ExpiresAt <= common.GetTimestamp() {
		return dto.TunnelAgentSetupResponse{}, errors.New("tunnel connection is expired")
	}

	baseURL := normalizeTunnelSetupBaseURL(params.BaseURL)
	token, key, created, rotated, err := ensureTunnelAgentToken(app, connection, params.Request.Rotate)
	if err != nil {
		return dto.TunnelAgentSetupResponse{}, err
	}
	if created || rotated || connection.AgentTokenId != token.Id {
		connection, err = model.UpdateTunnelConnectionFields(connection.Id, map[string]any{
			"agent_token_id": token.Id,
		})
		if err != nil {
			return dto.TunnelAgentSetupResponse{}, err
		}
	}

	clientId := strings.TrimSpace(app.BridgeClientId)
	if clientId == "" {
		clientId = tunnelAgentClientId(*app, *connection)
	}
	mcpPath := tunnelConnectionEndpointPath("<connection_key>", *app)
	mcpURL := baseURL + mcpPath
	bridgeWSURL := tunnelBridgeWebSocketURL(baseURL)
	apiKey := tunnelAPIKey(key)
	headers := map[string]string{
		"Authorization": "Bearer sk-<api_key>",
	}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	register := map[string]any{
		"type": "register",
		"data": map[string]any{
			"client_id":    clientId,
			"name":         tunnelAgentClientName(*app, params.Request.ClientName),
			"version":      strings.TrimSpace(params.Request.Version),
			"platform":     strings.TrimSpace(params.Request.Platform),
			"workspace":    strings.TrimSpace(params.Request.Workspace),
			"capabilities": []string{"mcp", "tunnel", app.PermissionMode},
		},
	}
	environment := map[string]string{
		"DATA_PROXY_BASE_URL":          baseURL,
		"DATA_PROXY_BRIDGE_WS_URL":     bridgeWSURL,
		"DATA_PROXY_TUNNEL_MCP_URL":    mcpURL,
		"DATA_PROXY_TUNNEL_CLIENT_ID":  clientId,
		"DATA_PROXY_TUNNEL_KEY_PREFIX": connection.KeyPrefix,
	}
	if apiKey != "" {
		environment["DATA_PROXY_API_KEY"] = apiKey
	}
	config := map[string]any{
		"base_url":      baseURL,
		"bridge_ws_url": bridgeWSURL,
		"mcp_url":       mcpURL,
		"client_id":     clientId,
		"api_key_env":   "DATA_PROXY_API_KEY",
		"headers":       headers,
		"register":      register,
		"connection": map[string]any{
			"id":         connection.Id,
			"key_prefix": connection.KeyPrefix,
			"endpoint":   mcpPath,
		},
		"app": map[string]any{
			"id":              app.Id,
			"slug":            app.PublicSlug,
			"permission_mode": app.PermissionMode,
			"target_path":     app.TargetPath,
		},
	}
	if apiKey != "" {
		config["api_key"] = apiKey
	}

	_ = model.CreateTunnelAuditLog(&model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         params.UserId,
		Action:              model.TunnelAuditActionAgentSetup,
		Decision:            "allow",
		Reason:              "user_ensured_tunnel_agent_setup",
		MetadataJson: tunnelAuditMetadataJSON(map[string]any{
			"token_id":     token.Id,
			"client_id":    clientId,
			"created":      created,
			"rotated":      rotated,
			"api_key_once": apiKey != "",
		}),
	})

	return dto.TunnelAgentSetupResponse{
		App:            tunnelAppToDTO(*app),
		Connection:     tunnelConnectionToDTO(*connection, *app, ""),
		BaseURL:        baseURL,
		BridgeWSURL:    bridgeWSURL,
		MCPURL:         mcpURL,
		ClientId:       clientId,
		APIKey:         apiKey,
		APIKeyOnce:     apiKey != "",
		TokenId:        token.Id,
		TokenName:      token.Name,
		TokenMaskedKey: "sk-" + token.GetMaskedKey(),
		Created:        created,
		Rotated:        rotated,
		Headers:        headers,
		Register:       register,
		Environment:    environment,
		Config:         config,
	}, nil
}

func ListTunnelAuditLogs(params TunnelAuditLogListParams) ([]dto.TunnelAuditLogItem, int64, error) {
	app, err := getTunnelAppOwnedByUser(params.AppId, params.UserId)
	if err != nil {
		return nil, 0, err
	}
	if params.ConnectionId > 0 {
		connection, err := model.GetTunnelConnectionById(params.ConnectionId)
		if err != nil {
			return nil, 0, err
		}
		if connection.AppId != app.Id || connection.UserId != app.UserId {
			return nil, 0, errors.New("tunnel connection not found")
		}
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	items, total, err := model.ListTunnelAuditLogs(model.TunnelAuditLogFilter{
		AppId:        app.Id,
		ConnectionId: params.ConnectionId,
		UserId:       app.UserId,
		Action:       strings.TrimSpace(params.Action),
		Decision:     strings.TrimSpace(params.Decision),
		RequestId:    strings.TrimSpace(params.RequestId),
		ToolName:     strings.TrimSpace(params.ToolName),
		SessionId:    strings.TrimSpace(params.SessionId),
		Keyword:      strings.TrimSpace(params.Keyword),
		StartTime:    params.StartTime,
		EndTime:      params.EndTime,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dto.TunnelAuditLogItem, 0, len(items))
	for _, item := range items {
		result = append(result, tunnelAuditLogToDTO(item))
	}
	return result, total, nil
}

func validateTunnelAppApprovalReady(app model.TunnelApp) error {
	if app.Status != model.TunnelAppStatusApproved {
		return nil
	}
	if app.BridgeClientId == "" {
		return errors.New("bridge_client_id is required before approving a tunnel app")
	}
	if err := validateTunnelHTTPRouteConfigJSON(app.AppType, app.RouteJson); err != nil {
		return err
	}
	_, err := getAuthorizedBridgeClient(app.BridgeClientId, app.UserId)
	return err
}

func syncTunnelAppBridgePolicy(app model.TunnelApp) error {
	if app.Status != model.TunnelAppStatusApproved {
		return nil
	}
	client, err := getAuthorizedBridgeClient(app.BridgeClientId, app.UserId)
	if err != nil {
		return err
	}
	if app.AppType != model.TunnelAppTypeMCPCode && app.AppType != model.TunnelAppTypeHTTP && app.AppType != model.TunnelAppTypeTCP {
		return nil
	}
	existingPolicy, err := bridgepolicy.Parse(client.Policy)
	if err != nil {
		existingPolicy = bridgepolicy.Policy{}
	}
	appPolicy, err := bridgepolicy.Parse(app.PolicyJson)
	if err != nil {
		return err
	}
	nextPolicy := tunnelBridgePolicyFromApp(app, existingPolicy, appPolicy)
	rawPolicy, err := bridgepolicy.Marshal(nextPolicy)
	if err != nil {
		return err
	}
	_, err = model.UpdateBridgeClientFields(client.ClientId, map[string]any{
		"policy": rawPolicy,
	})
	return err
}

func applyTunnelAppUpdatesForValidation(app model.TunnelApp, updates map[string]any) model.TunnelApp {
	if value, ok := updates["name"].(string); ok {
		app.Name = value
	}
	if value, ok := updates["description"].(string); ok {
		app.Description = value
	}
	if value, ok := updates["permission_mode"].(string); ok {
		app.PermissionMode = value
	}
	if value, ok := updates["status"].(string); ok {
		app.Status = value
	}
	if value, ok := updates["bridge_client_id"].(string); ok {
		app.BridgeClientId = value
	}
	if value, ok := updates["target_host"].(string); ok {
		app.TargetHost = value
	}
	if value, ok := updates["target_port"].(int); ok {
		app.TargetPort = value
	}
	if value, ok := updates["target_path"].(string); ok {
		app.TargetPath = value
	}
	if value, ok := updates["policy_json"].(string); ok {
		app.PolicyJson = value
	}
	if value, ok := updates["route_json"].(string); ok {
		app.RouteJson = value
	}
	if value, ok := updates["billing_json"].(string); ok {
		app.BillingJson = value
	}
	return app
}

func tunnelCreateRequestToModel(userId int, req dto.TunnelAppCreateRequest) (*model.TunnelApp, error) {
	appType := normalizeTunnelAppType(req.AppType)
	if appType == "" {
		return nil, errors.New("invalid tunnel app_type")
	}
	permissionMode := normalizeTunnelPermissionMode(appType, req.PermissionMode)
	if permissionMode == "" {
		return nil, errors.New("invalid tunnel permission_mode")
	}
	targetHost := strings.TrimSpace(req.TargetHost)
	targetPath := strings.TrimSpace(req.TargetPath)
	if targetPath == "" {
		if appType == model.TunnelAppTypeMCPCode {
			targetPath = "/mcp"
		} else {
			targetPath = "/"
		}
	}
	if appType != model.TunnelAppTypeMCPCode {
		if targetHost == "" {
			targetHost = "127.0.0.1"
		}
		if req.TargetPort <= 0 || req.TargetPort > 65535 {
			return nil, errors.New("target_port is required for traffic tunnel apps")
		}
	}
	policyJson, err := marshalTunnelMap(req.Policy)
	if err != nil {
		return nil, err
	}
	routeJson, err := marshalTunnelMap(req.Route)
	if err != nil {
		return nil, err
	}
	if err := validateTunnelHTTPRouteConfigJSON(appType, routeJson); err != nil {
		return nil, err
	}
	billingJson, err := marshalTunnelMap(req.Billing)
	if err != nil {
		return nil, err
	}
	return &model.TunnelApp{
		UserId:         userId,
		Name:           strings.TrimSpace(req.Name),
		Description:    strings.TrimSpace(req.Description),
		AppType:        appType,
		PermissionMode: permissionMode,
		Status:         model.TunnelAppStatusPending,
		BridgeClientId: strings.TrimSpace(req.BridgeClientId),
		TargetHost:     targetHost,
		TargetPort:     req.TargetPort,
		TargetPath:     targetPath,
		PolicyJson:     policyJson,
		RouteJson:      routeJson,
		BillingJson:    billingJson,
	}, validateTunnelAppName(req.Name)
}

func tunnelAdminUpdateFields(existing model.TunnelApp, actorUserId int, req dto.TunnelAppAdminUpdateRequest) (map[string]any, error) {
	updates := map[string]any{}
	nextType := existing.AppType
	if req.Name != nil {
		if err := validateTunnelAppName(*req.Name); err != nil {
			return nil, err
		}
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.PermissionMode != nil {
		mode := normalizeTunnelPermissionMode(nextType, *req.PermissionMode)
		if mode == "" {
			return nil, errors.New("invalid tunnel permission_mode")
		}
		updates["permission_mode"] = mode
	}
	if req.Status != nil {
		status := normalizeTunnelStatus(*req.Status)
		if status == "" {
			return nil, errors.New("invalid tunnel status")
		}
		updates["status"] = status
		if status == model.TunnelAppStatusApproved {
			updates["approved_by"] = actorUserId
			updates["approved_at"] = common.GetTimestamp()
		}
	}
	if req.BridgeClientId != nil {
		updates["bridge_client_id"] = strings.TrimSpace(*req.BridgeClientId)
	}
	if req.TargetHost != nil {
		updates["target_host"] = strings.TrimSpace(*req.TargetHost)
	}
	if req.TargetPort != nil {
		if existing.AppType != model.TunnelAppTypeMCPCode && (*req.TargetPort <= 0 || *req.TargetPort > 65535) {
			return nil, errors.New("invalid tunnel target_port")
		}
		updates["target_port"] = *req.TargetPort
	}
	if req.TargetPath != nil {
		updates["target_path"] = strings.TrimSpace(*req.TargetPath)
	}
	if req.Policy != nil {
		value, err := marshalTunnelMap(req.Policy)
		if err != nil {
			return nil, err
		}
		updates["policy_json"] = value
	}
	if req.Route != nil {
		value, err := marshalTunnelMap(req.Route)
		if err != nil {
			return nil, err
		}
		updates["route_json"] = value
	}
	if req.Billing != nil {
		value, err := marshalTunnelMap(req.Billing)
		if err != nil {
			return nil, err
		}
		updates["billing_json"] = value
	}
	if req.ReviewNote != nil {
		updates["review_note"] = strings.TrimSpace(*req.ReviewNote)
	}
	return updates, nil
}

func getTunnelAppOwnedByUser(id int64, userId int) (*model.TunnelApp, error) {
	if userId <= 0 {
		return nil, errors.New("invalid tunnel user")
	}
	app, err := model.GetTunnelAppById(id)
	if err != nil {
		return nil, err
	}
	if app.UserId != userId {
		return nil, errors.New("tunnel app not found")
	}
	return app, nil
}

func validateTunnelAppName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("tunnel app name is required")
	}
	if len([]rune(strings.TrimSpace(name))) > 128 {
		return errors.New("tunnel app name is too long")
	}
	return nil
}

func validateTunnelConnectionName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("tunnel connection name is required")
	}
	if len([]rune(strings.TrimSpace(name))) > 128 {
		return errors.New("tunnel connection name is too long")
	}
	return nil
}

func normalizeTunnelAppType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", model.TunnelAppTypeMCPCode:
		return model.TunnelAppTypeMCPCode
	case model.TunnelAppTypeHTTP:
		return model.TunnelAppTypeHTTP
	case model.TunnelAppTypeTCP:
		return model.TunnelAppTypeTCP
	default:
		return ""
	}
}

func normalizeTunnelPermissionMode(appType string, value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if appType == model.TunnelAppTypeHTTP || appType == model.TunnelAppTypeTCP {
		if value == "" || value == model.TunnelPermissionTraffic {
			return model.TunnelPermissionTraffic
		}
		return ""
	}
	switch value {
	case "", model.TunnelPermissionReadOnly:
		return model.TunnelPermissionReadOnly
	case model.TunnelPermissionWrite:
		return model.TunnelPermissionWrite
	case model.TunnelPermissionExecSafe:
		return model.TunnelPermissionExecSafe
	case model.TunnelPermissionExecTrusted:
		return model.TunnelPermissionExecTrusted
	default:
		return ""
	}
}

func normalizeTunnelConnectionPermissionMode(app model.TunnelApp, value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = app.PermissionMode
	}
	mode := normalizeTunnelPermissionMode(app.AppType, value)
	if mode == "" {
		return "", errors.New("invalid tunnel connection permission_mode")
	}
	if tunnelPermissionRank(mode) > tunnelPermissionRank(app.PermissionMode) {
		return "", errors.New("tunnel connection permission_mode cannot exceed app permission_mode")
	}
	return mode, nil
}

func effectiveTunnelMCPPermissionMode(app model.TunnelApp, connection model.TunnelConnection) string {
	if connection.PermissionMode == "" {
		return app.PermissionMode
	}
	if tunnelPermissionRank(connection.PermissionMode) <= tunnelPermissionRank(app.PermissionMode) {
		return connection.PermissionMode
	}
	return app.PermissionMode
}

func tunnelPermissionRank(mode string) int {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case model.TunnelPermissionReadOnly:
		return 1
	case model.TunnelPermissionWrite:
		return 2
	case model.TunnelPermissionExecSafe:
		return 3
	case model.TunnelPermissionExecTrusted:
		return 4
	case model.TunnelPermissionTraffic:
		return 1
	default:
		return 0
	}
}

func normalizeTunnelStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case model.TunnelAppStatusPending:
		return model.TunnelAppStatusPending
	case model.TunnelAppStatusApproved:
		return model.TunnelAppStatusApproved
	case model.TunnelAppStatusRejected:
		return model.TunnelAppStatusRejected
	case model.TunnelAppStatusDisabled:
		return model.TunnelAppStatusDisabled
	case model.TunnelAppStatusArchived:
		return model.TunnelAppStatusArchived
	default:
		return ""
	}
}

func tunnelReviewDecisionFromStatus(status string) string {
	switch status {
	case model.TunnelAppStatusApproved:
		return "allow"
	case model.TunnelAppStatusRejected, model.TunnelAppStatusDisabled, model.TunnelAppStatusArchived:
		return "deny"
	default:
		return "update"
	}
}

func tunnelBridgePolicyFromApp(app model.TunnelApp, existing bridgepolicy.Policy, appPolicy bridgepolicy.Policy) bridgepolicy.Policy {
	if app.AppType == model.TunnelAppTypeHTTP {
		return tunnelHTTPBridgePolicy(existing, appPolicy)
	}
	if app.AppType == model.TunnelAppTypeTCP {
		return tunnelTCPBridgePolicy(existing, appPolicy)
	}
	return tunnelBridgePolicyFromMode(app.PermissionMode, existing, appPolicy)
}

func tunnelBridgePolicyFromMode(permissionMode string, existing bridgepolicy.Policy, appPolicy bridgepolicy.Policy) bridgepolicy.Policy {
	next := existing
	next.AllowedTools = mergeTunnelAllowedTools(mcpgateway.AllowedToolsForPermissionMode(permissionMode), existingFamilyAllowedTools(existing.AllowedTools, "http_tunnel"))
	next.AllowWrite = permissionMode == model.TunnelPermissionWrite ||
		permissionMode == model.TunnelPermissionExecSafe ||
		permissionMode == model.TunnelPermissionExecTrusted
	if appPolicy.MaxResultBytes > 0 {
		next.MaxResultBytes = appPolicy.MaxResultBytes
	}
	if appPolicy.MaxScanFileBytes > 0 {
		next.MaxScanFileBytes = appPolicy.MaxScanFileBytes
	}
	if appPolicy.MaxResults > 0 {
		next.MaxResults = appPolicy.MaxResults
	}
	if appPolicy.TreeDepth > 0 {
		next.TreeDepth = appPolicy.TreeDepth
	}
	if appPolicy.WalkDepth > 0 {
		next.WalkDepth = appPolicy.WalkDepth
	}
	if len(appPolicy.MCPAllowedTargets) > 0 {
		next.MCPAllowedTargets = appPolicy.MCPAllowedTargets
	}
	if len(appPolicy.HTTPAllowedTargets) > 0 {
		next.HTTPAllowedTargets = appPolicy.HTTPAllowedTargets
	}
	if len(appPolicy.HTTPDeniedTargets) > 0 {
		next.HTTPDeniedTargets = appPolicy.HTTPDeniedTargets
	}
	if len(appPolicy.HTTPDeniedPorts) > 0 {
		next.HTTPDeniedPorts = appPolicy.HTTPDeniedPorts
	}
	return bridgepolicy.Normalize(next)
}

func tunnelHTTPBridgePolicy(existing bridgepolicy.Policy, appPolicy bridgepolicy.Policy) bridgepolicy.Policy {
	next := existing
	base := existing.AllowedTools
	next.AllowedTools = mergeTunnelAllowedTools(base, []string{"http_tunnel"})
	if appPolicy.MaxResultBytes > 0 {
		next.MaxResultBytes = appPolicy.MaxResultBytes
	}
	if len(appPolicy.HTTPAllowedTargets) > 0 {
		next.HTTPAllowedTargets = appPolicy.HTTPAllowedTargets
	}
	if len(appPolicy.HTTPDeniedTargets) > 0 {
		next.HTTPDeniedTargets = appPolicy.HTTPDeniedTargets
	}
	if len(appPolicy.HTTPDeniedPorts) > 0 {
		next.HTTPDeniedPorts = appPolicy.HTTPDeniedPorts
	}
	if len(appPolicy.MCPAllowedTargets) > 0 {
		next.MCPAllowedTargets = appPolicy.MCPAllowedTargets
	}
	return bridgepolicy.Normalize(next)
}

func tunnelTCPBridgePolicy(existing bridgepolicy.Policy, appPolicy bridgepolicy.Policy) bridgepolicy.Policy {
	next := existing
	base := existing.AllowedTools
	next.AllowedTools = mergeTunnelAllowedTools(base, []string{"tcp_tunnel"})
	if appPolicy.MaxResultBytes > 0 {
		next.MaxResultBytes = appPolicy.MaxResultBytes
	}
	if len(appPolicy.MCPAllowedTargets) > 0 {
		next.MCPAllowedTargets = appPolicy.MCPAllowedTargets
	}
	return bridgepolicy.Normalize(next)
}

func mergeTunnelAllowedTools(base []string, extra []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(base)+len(extra))
	for _, item := range append(append([]string{}, base...), extra...) {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func existingFamilyAllowedTools(values []string, family string) []string {
	family = strings.TrimSpace(family)
	if family == "" {
		return nil
	}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == family || strings.HasPrefix(value, family+".") {
			result = append(result, value)
		}
	}
	return result
}

func marshalTunnelMap(value map[string]any) (string, error) {
	if value == nil {
		return "", nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal tunnel json failed: %w", err)
	}
	return string(body), nil
}

func unmarshalTunnelMap(value string) map[string]any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil
	}
	return result
}

func tunnelAuditMetadataJSON(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	body, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(body)
}

func tunnelUpdateFieldNames(updates map[string]any) []string {
	fields := make([]string, 0, len(updates))
	for key := range updates {
		fields = append(fields, key)
	}
	return fields
}

func generateTunnelConnectionKey() string {
	return "tc_" + common.GetRandomString(40)
}

func tunnelConnectionKeyHash(key string) string {
	sum := sha256.Sum256([]byte("tunnel_connection:v1:" + strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

func tunnelConnectionKeyPrefix(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

func tunnelConnectionEndpointPath(key string, app model.TunnelApp) string {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "<connection_key>"
	}
	if app.AppType == model.TunnelAppTypeHTTP {
		return "/t/" + key + "/tunnel/http/" + app.PublicSlug
	}
	if app.AppType == model.TunnelAppTypeTCP {
		return "/t/" + key + "/tunnel/tcp/" + app.PublicSlug
	}
	return "/t/" + key + "/tunnel/mcp/" + app.PublicSlug
}

func ensureTunnelAgentToken(app *model.TunnelApp, connection *model.TunnelConnection, rotate bool) (*model.Token, string, bool, bool, error) {
	now := common.GetTimestamp()
	if connection.AgentTokenId > 0 {
		token, err := model.GetTokenByIds(connection.AgentTokenId, app.UserId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", false, false, err
		}
		if err == nil && !rotate && tunnelAgentTokenReusable(token, now) {
			return token, "", false, false, nil
		}
		if err == nil {
			if err := model.DisableTokenWithTx(model.DB, token.Id, app.UserId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", false, false, err
			}
		}
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, "", false, false, err
	}
	token := &model.Token{
		UserId:                app.UserId,
		Name:                  tunnelAgentTokenName(*app, *connection),
		Key:                   key,
		Status:                common.TokenStatusEnabled,
		CreatedTime:           now,
		AccessedTime:          now,
		ExpiredTime:           -1,
		RemainQuota:           0,
		UnlimitedQuota:        true,
		QuotaHardLimitEnabled: false,
		ModelLimitsEnabled:    false,
	}
	if err := token.Insert(); err != nil {
		return nil, "", false, false, err
	}
	return token, key, true, rotate && connection.AgentTokenId > 0, nil
}

func tunnelAgentTokenReusable(token *model.Token, now int64) bool {
	if token == nil || token.Status != common.TokenStatusEnabled {
		return false
	}
	return token.ExpiredTime == -1 || token.ExpiredTime > now
}

func tunnelAgentTokenName(app model.TunnelApp, connection model.TunnelConnection) string {
	name := "Tunnel Agent - " + app.Name
	if connection.Name != "" && connection.Name != app.Name {
		name += " - " + connection.Name
	}
	return limitTunnelString(name, 50)
}

func tunnelAgentClientName(app model.TunnelApp, value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return limitTunnelString(value, 128)
	}
	return limitTunnelString("Tunnel Agent - "+app.Name, 128)
}

func tunnelAgentClientId(app model.TunnelApp, connection model.TunnelConnection) string {
	seed := fmt.Sprintf("%d:%s:%d:%s", app.UserId, app.PublicSlug, connection.Id, connection.KeyPrefix)
	sum := sha256.Sum256([]byte("tunnel_agent_client:v1:" + seed))
	return "tun-" + app.PublicSlug + "-" + hex.EncodeToString(sum[:])[:12]
}

func normalizeTunnelSetupBaseURL(value string) string {
	base := strings.TrimRight(strings.TrimSpace(value), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	return strings.TrimRight(base, "/")
}

func tunnelBridgeWebSocketURL(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" {
		return strings.TrimRight(baseURL, "/") + "/bridge/ws"
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/bridge/ws"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func tunnelAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func limitTunnelString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func tunnelAppToDTO(app model.TunnelApp) dto.TunnelAppItem {
	return dto.TunnelAppItem{
		Id:             app.Id,
		UserId:         app.UserId,
		Name:           app.Name,
		Description:    app.Description,
		AppType:        app.AppType,
		PermissionMode: app.PermissionMode,
		Status:         app.Status,
		PublicSlug:     app.PublicSlug,
		BridgeClientId: app.BridgeClientId,
		TargetHost:     app.TargetHost,
		TargetPort:     app.TargetPort,
		TargetPath:     app.TargetPath,
		Policy:         unmarshalTunnelMap(app.PolicyJson),
		Route:          unmarshalTunnelMap(app.RouteJson),
		Billing:        unmarshalTunnelMap(app.BillingJson),
		ApprovedBy:     app.ApprovedBy,
		ApprovedAt:     app.ApprovedAt,
		ReviewNote:     app.ReviewNote,
		LastError:      app.LastError,
		LastSeenAt:     app.LastSeenAt,
		CreatedAt:      app.CreatedAt,
		UpdatedAt:      app.UpdatedAt,
	}
}

func tunnelConnectionToDTO(connection model.TunnelConnection, app model.TunnelApp, key string) dto.TunnelConnectionItem {
	return dto.TunnelConnectionItem{
		Id:             connection.Id,
		AppId:          connection.AppId,
		UserId:         connection.UserId,
		AgentTokenId:   connection.AgentTokenId,
		Name:           connection.Name,
		KeyPrefix:      connection.KeyPrefix,
		PermissionMode: connection.PermissionMode,
		Status:         connection.Status,
		EndpointPath:   tunnelConnectionEndpointPath(key, app),
		Config:         unmarshalTunnelMap(connection.ConfigJson),
		ExpiresAt:      connection.ExpiresAt,
		LastUsedAt:     connection.LastUsedAt,
		LastRequestId:  connection.LastRequestId,
		RevokedAt:      connection.RevokedAt,
		CreatedAt:      connection.CreatedAt,
		UpdatedAt:      connection.UpdatedAt,
	}
}

func tunnelSessionToDTO(session model.TunnelSession) dto.TunnelSessionItem {
	return dto.TunnelSessionItem{
		Id:             session.Id,
		AppId:          session.AppId,
		UserId:         session.UserId,
		ConnectionId:   session.ConnectionId,
		ConnectionName: session.ConnectionName,
		KeyPrefix:      session.KeyPrefix,
		SessionId:      session.SessionId,
		BridgeClientId: session.BridgeClientId,
		Status:         session.Status,
		ClientVersion:  session.ClientVersion,
		ClientIp:       session.ClientIp,
		UserAgent:      session.UserAgent,
		BytesIn:        session.BytesIn,
		BytesOut:       session.BytesOut,
		ConnectedAt:    session.ConnectedAt,
		LastSeenAt:     session.LastSeenAt,
		DisconnectedAt: session.DisconnectedAt,
		CloseReason:    session.CloseReason,
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
	}
}

func tunnelAuditLogToDTO(log model.TunnelAuditLog) dto.TunnelAuditLogItem {
	return dto.TunnelAuditLogItem{
		Id:                  log.Id,
		AppId:               log.AppId,
		ConnectionId:        log.ConnectionId,
		ConnectionKeyPrefix: log.ConnectionKeyPrefix,
		SessionId:           log.SessionId,
		UserId:              log.UserId,
		ActorUserId:         log.ActorUserId,
		Action:              log.Action,
		Decision:            log.Decision,
		Reason:              log.Reason,
		RequestId:           log.RequestId,
		ToolName:            log.ToolName,
		Method:              log.Method,
		Path:                log.Path,
		BytesIn:             log.BytesIn,
		BytesOut:            log.BytesOut,
		DurationMS:          log.DurationMS,
		Metadata:            unmarshalTunnelMap(log.MetadataJson),
		CreatedAt:           log.CreatedAt,
	}
}
