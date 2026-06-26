package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"

	"gorm.io/gorm"
)

type BridgeRegisterInput struct {
	UserId       int
	TokenId      int
	RequestIP    string
	UserAgent    string
	ClientId     string
	Name         string
	Version      string
	Platform     string
	Workspace    string
	Capabilities []string
}

type BridgeRegisterResult struct {
	SessionId string
	ClientId  string
}

func RegisterBridgeClient(input BridgeRegisterInput) (*BridgeRegisterResult, error) {
	clientId := strings.TrimSpace(input.ClientId)
	if clientId == "" {
		return nil, errors.New("client_id is required")
	}
	if len(clientId) > 128 {
		return nil, errors.New("client_id is too long")
	}
	existing, err := model.GetBridgeClientByClientIdUnscoped(clientId)
	if err == nil && existing.UserId != input.UserId {
		return nil, fmt.Errorf("bridge client_id %s is already registered by another user", clientId)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	capabilitiesJSON, err := common.Marshal(input.Capabilities)
	if err != nil {
		return nil, err
	}
	client := &model.BridgeClient{
		ClientId:     clientId,
		UserId:       input.UserId,
		TokenId:      input.TokenId,
		Name:         truncateBridgeString(input.Name, 128),
		Version:      truncateBridgeString(input.Version, 64),
		Platform:     truncateBridgeString(input.Platform, 64),
		Workspace:    truncateBridgeString(input.Workspace, 512),
		Capabilities: string(capabilitiesJSON),
		Status:       model.BridgeClientStatusOnline,
	}
	if err := model.UpsertBridgeClient(client); err != nil {
		return nil, err
	}
	if err := model.CloseActiveBridgeSessionsForClient(clientId, "replaced by new connection"); err != nil {
		return nil, err
	}

	sessionId := common.GetRandomString(32)
	session := &model.BridgeSession{
		SessionId: sessionId,
		ClientId:  clientId,
		UserId:    input.UserId,
		TokenId:   input.TokenId,
		RequestIP: truncateBridgeString(input.RequestIP, 64),
		UserAgent: truncateBridgeString(input.UserAgent, 512),
		Status:    model.BridgeSessionStatusOnline,
	}
	if err := model.CreateBridgeSession(session); err != nil {
		_ = model.MarkBridgeClientOffline(clientId)
		return nil, err
	}
	if err := model.MarkBridgeClientOnline(clientId); err != nil {
		return nil, err
	}

	return &BridgeRegisterResult{
		SessionId: sessionId,
		ClientId:  clientId,
	}, nil
}

func TouchBridgeClientSession(sessionId string) error {
	if !bridge.DefaultHub.Touch(sessionId) {
		return errors.New("bridge session is not online")
	}
	return model.TouchBridgeSession(sessionId)
}

func CloseBridgeClientSession(sessionId string, reason string) error {
	snapshot, ok := bridge.DefaultHub.CloseSession(sessionId, bridge.CloseSessionOptions{Reason: reason})
	if err := model.CloseBridgeSession(sessionId, model.BridgeSessionStatusClosed, reason); err != nil {
		return err
	}
	if ok {
		_, _ = model.MarkBridgeClientOfflineIfNoOnlineSession(snapshot.ClientId)
	}
	return nil
}

type BridgeClientListParams struct {
	UserId  int
	Status  *int
	Keyword string
	Offset  int
	Limit   int
}

type BridgeAuditLogListParams struct {
	UserId    int
	TokenId   int
	ClientId  string
	SessionId string
	ToolName  string
	Status    string
	RequestId string
	StartTime int64
	EndTime   int64
	Keyword   string
	Offset    int
	Limit     int
}

type BridgeClientDetailParams struct {
	UserId   int
	ClientId string
}

type BridgeClientHealthParams struct {
	UserId        int
	ClientId      string
	WindowSeconds int64
}

type BridgeClientUpdateParams struct {
	UserId   int
	ClientId string
	Request  dto.BridgeClientUpdateRequest
}

type BridgeSessionCloseParams struct {
	UserId    int
	SessionId string
	Reason    string
}

type BridgeAgentSetupParams struct {
	UserId  int
	BaseURL string
	Request dto.BridgeAgentSetupRequest
}

type BridgeAgentSetupTokenCreateParams struct {
	UserId  int
	BaseURL string
	Request dto.BridgeAgentSetupTokenRequest
}

type BridgeAgentSetupTokenConsumeParams struct {
	BaseURL string
	Request dto.BridgeAgentSetupTokenConsumeRequest
}

const (
	defaultBridgeAgentSetupTokenTTLSeconds = 10 * 60
	minBridgeAgentSetupTokenTTLSeconds     = 60
	maxBridgeAgentSetupTokenTTLSeconds     = 60 * 60
	maxBridgeAgentHealthChecks             = 100
	maxBridgeAgentMCPProcesses             = 50
	maxBridgeAgentHealthJSONBytes          = 60 * 1024
)

func EnsureBridgeAgentSetup(params BridgeAgentSetupParams) (dto.BridgeAgentSetupResponse, error) {
	if params.UserId <= 0 {
		return dto.BridgeAgentSetupResponse{}, errors.New("invalid bridge user")
	}
	clientId := strings.TrimSpace(params.Request.ClientId)
	var existing *model.BridgeClient
	var err error
	if clientId == "" {
		clientId, err = generateBridgeAgentClientId()
		if err != nil {
			return dto.BridgeAgentSetupResponse{}, err
		}
	} else {
		if len(clientId) > 128 {
			return dto.BridgeAgentSetupResponse{}, errors.New("client_id is too long")
		}
		existing, err = model.GetBridgeClientByClientIdUnscoped(clientId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.BridgeAgentSetupResponse{}, err
		}
		if err == nil && existing.UserId != params.UserId {
			return dto.BridgeAgentSetupResponse{}, errors.New("bridge client not found")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			existing = nil
		}
	}

	tokenName := bridgeAgentTokenName(params.Request, clientId, existing)
	token, key, created, rotated, err := ensureBridgeAgentToken(params.UserId, existing, tokenName, params.Request.Rotate)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	capabilities := bridgeAgentCapabilities()
	capabilitiesJSON, err := common.Marshal(capabilities)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	client := &model.BridgeClient{
		ClientId:     clientId,
		UserId:       params.UserId,
		TokenId:      token.Id,
		Name:         bridgeAgentClientName(params.Request, existing, clientId),
		Version:      truncateBridgeString(params.Request.Version, 64),
		Platform:     truncateBridgeString(params.Request.Platform, 64),
		Workspace:    truncateBridgeString(params.Request.Workspace, 512),
		Capabilities: string(capabilitiesJSON),
		Status:       model.BridgeClientStatusOffline,
	}
	if existing != nil {
		if client.Version == "" {
			client.Version = existing.Version
		}
		if client.Platform == "" {
			client.Platform = existing.Platform
		}
		if client.Workspace == "" {
			client.Workspace = existing.Workspace
		}
	}
	if err := model.UpsertBridgeClient(client); err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	stored, err := model.GetBridgeClientByClientId(clientId)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}

	baseURL := normalizeTunnelSetupBaseURL(params.BaseURL)
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
			"name":         stored.Name,
			"version":      stored.Version,
			"platform":     stored.Platform,
			"workspace":    stored.Workspace,
			"capabilities": capabilities,
		},
	}
	environment := map[string]string{
		"DATA_PROXY_BASE_URL":         baseURL,
		"DATA_PROXY_BRIDGE_WS_URL":    bridgeWSURL,
		"DATA_PROXY_BRIDGE_CLIENT_ID": clientId,
	}
	if apiKey != "" {
		environment["DATA_PROXY_API_KEY"] = apiKey
	}
	config := map[string]any{
		"base_url":      baseURL,
		"bridge_ws_url": bridgeWSURL,
		"client_id":     clientId,
		"api_key_env":   "DATA_PROXY_API_KEY",
		"headers":       headers,
		"register":      register,
	}
	if apiKey != "" {
		config["api_key"] = apiKey
	}

	return dto.BridgeAgentSetupResponse{
		Client:         bridgeClientToDTO(*stored),
		BaseURL:        baseURL,
		BridgeWSURL:    bridgeWSURL,
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

func CreateBridgeAgentSetupToken(params BridgeAgentSetupTokenCreateParams) (dto.BridgeAgentSetupTokenResponse, error) {
	if params.UserId <= 0 {
		return dto.BridgeAgentSetupTokenResponse{}, errors.New("invalid bridge user")
	}
	clientId := strings.TrimSpace(params.Request.ClientId)
	if len(clientId) > 128 {
		return dto.BridgeAgentSetupTokenResponse{}, errors.New("client_id is too long")
	}
	setupToken, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return dto.BridgeAgentSetupTokenResponse{}, err
	}
	setupToken = "dpat_" + setupToken
	ttlSeconds := params.Request.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = defaultBridgeAgentSetupTokenTTLSeconds
	}
	if ttlSeconds < minBridgeAgentSetupTokenTTLSeconds {
		ttlSeconds = minBridgeAgentSetupTokenTTLSeconds
	}
	if ttlSeconds > maxBridgeAgentSetupTokenTTLSeconds {
		ttlSeconds = maxBridgeAgentSetupTokenTTLSeconds
	}
	now := common.GetTimestamp()
	expiresAt := now + int64(ttlSeconds)
	token := &model.BridgeAgentSetupToken{
		TokenHash:  bridgeAgentSetupTokenHash(setupToken),
		UserId:     params.UserId,
		ClientId:   truncateBridgeString(clientId, 128),
		ClientName: truncateBridgeString(params.Request.ClientName, 128),
		Version:    truncateBridgeString(params.Request.Version, 64),
		Platform:   truncateBridgeString(params.Request.Platform, 64),
		Workspace:  truncateBridgeString(params.Request.Workspace, 512),
		Rotate:     params.Request.Rotate,
		ExpiresAt:  expiresAt,
	}
	if err := model.CreateBridgeAgentSetupToken(token); err != nil {
		return dto.BridgeAgentSetupTokenResponse{}, err
	}
	baseURL := normalizeTunnelSetupBaseURL(params.BaseURL)
	enrollCommand := fmt.Sprintf("data-proxy-agent enroll --server %s --setup-token %s", bridgeAgentShellQuote(baseURL), bridgeAgentShellQuote(setupToken))
	installCommand := "curl -fsSL https://raw.githubusercontent.com/normojs/data-proxy/main/scripts/install-data-proxy-agent.sh | sh"
	return dto.BridgeAgentSetupTokenResponse{
		SetupToken:       setupToken,
		ExpiresAt:        expiresAt,
		ExpiresInSeconds: ttlSeconds,
		ClientId:         clientId,
		EnrollCommand:    enrollCommand,
		InstallCommand:   installCommand,
		FullCommand:      installCommand + "\n" + enrollCommand,
	}, nil
}

func ConsumeBridgeAgentSetupToken(params BridgeAgentSetupTokenConsumeParams) (dto.BridgeAgentSetupResponse, error) {
	setupToken := strings.TrimSpace(params.Request.SetupToken)
	if setupToken == "" {
		return dto.BridgeAgentSetupResponse{}, errors.New("setup_token is required")
	}
	token, err := model.ConsumeBridgeAgentSetupToken(bridgeAgentSetupTokenHash(setupToken), common.GetTimestamp())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.BridgeAgentSetupResponse{}, errors.New("setup token is invalid, expired, or already used")
		}
		return dto.BridgeAgentSetupResponse{}, err
	}
	request := dto.BridgeAgentSetupRequest{
		ClientId:   bridgeAgentSetupClientId(params.Request.ClientId, token.ClientId),
		Rotate:     params.Request.Rotate || token.Rotate,
		ClientName: firstNonEmptyBridgeString(params.Request.ClientName, token.ClientName),
		Version:    firstNonEmptyBridgeString(params.Request.Version, token.Version),
		Platform:   firstNonEmptyBridgeString(params.Request.Platform, token.Platform),
		Workspace:  firstNonEmptyBridgeString(params.Request.Workspace, token.Workspace),
	}
	return EnsureBridgeAgentSetup(BridgeAgentSetupParams{
		UserId:  token.UserId,
		BaseURL: params.BaseURL,
		Request: request,
	})
}

func ListBridgeClients(params BridgeClientListParams) ([]dto.BridgeClientItem, int64, error) {
	clients, total, err := model.ListBridgeClients(model.BridgeClientFilter{
		UserId:  params.UserId,
		Status:  params.Status,
		Keyword: params.Keyword,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.BridgeClientItem, 0, len(clients))
	for _, client := range clients {
		items = append(items, bridgeClientToDTO(client))
	}
	return items, total, nil
}

func GetBridgeClientDetail(params BridgeClientDetailParams) (dto.BridgeClientDetail, error) {
	client, err := getAuthorizedBridgeClient(params.ClientId, params.UserId)
	if err != nil {
		return dto.BridgeClientDetail{}, err
	}
	sessions, _, err := model.ListBridgeSessionsByClientId(client.ClientId, 0, 5)
	if err != nil {
		return dto.BridgeClientDetail{}, err
	}
	detail := dto.BridgeClientDetail{
		Client:         bridgeClientToDTO(*client),
		RecentSessions: bridgeSessionsToDTO(sessions),
	}
	if snapshot, ok := bridge.DefaultHub.GetByClient(client.ClientId); ok {
		converted := bridgeSessionSnapshotToDTO(snapshot)
		detail.OnlineSession = &converted
	}
	return detail, nil
}

func GetBridgeClientHealth(params BridgeClientHealthParams) (dto.BridgeClientHealth, error) {
	client, err := getAuthorizedBridgeClient(params.ClientId, params.UserId)
	if err != nil {
		return dto.BridgeClientHealth{}, err
	}
	windowSeconds := normalizeMCPSummaryWindow(params.WindowSeconds)
	now := common.GetTimestamp()
	filter := model.BridgeAuditLogFilter{
		UserId:    params.UserId,
		ClientId:  client.ClientId,
		StartTime: now - windowSeconds,
	}
	stats, err := model.GetBridgeAuditLogStats(filter)
	if err != nil {
		return dto.BridgeClientHealth{}, err
	}
	errors, err := model.ListRecentBridgeAuditErrors(filter, 5)
	if err != nil {
		return dto.BridgeClientHealth{}, err
	}
	sessions, _, err := model.ListBridgeSessionsByClientId(client.ClientId, 0, 5)
	if err != nil {
		return dto.BridgeClientHealth{}, err
	}

	health := dto.BridgeClientHealth{
		ClientId:       client.ClientId,
		WindowSeconds:  windowSeconds,
		GeneratedAt:    now,
		AgentHealth:    bridgeAgentHealthFromClient(*client),
		Calls:          bridgeAuditHealthFromStats(stats),
		RecentErrors:   bridgeRecentErrorsToDTO(errors),
		RecentSessions: bridgeSessionsToDTO(sessions),
	}
	if snapshot, ok := bridge.DefaultHub.GetByClient(client.ClientId); ok {
		converted := bridgeSessionSnapshotToDTO(snapshot)
		health.Online = true
		health.OnlineSession = &converted
	}
	return health, nil
}

func UpdateBridgeClientHealth(clientId string, report dto.BridgeAgentHealthReport) error {
	clientId = strings.TrimSpace(clientId)
	if clientId == "" {
		return errors.New("client_id is required")
	}
	report = normalizeBridgeAgentHealthReport(report)
	bytes, err := common.Marshal(report)
	if err != nil {
		return err
	}
	for len(bytes) > maxBridgeAgentHealthJSONBytes && len(report.Checks) > 0 {
		report.Checks = report.Checks[:len(report.Checks)-1]
		report.Summary = summarizeBridgeAgentHealthChecks(report.Checks)
		bytes, err = common.Marshal(report)
		if err != nil {
			return err
		}
	}
	if len(bytes) > maxBridgeAgentHealthJSONBytes {
		report.Checks = nil
		report.Summary = summarizeBridgeAgentHealthChecks(report.Checks)
		bytes, err = common.Marshal(report)
		if err != nil {
			return err
		}
	}
	return model.UpdateBridgeClientHealth(clientId, string(bytes), report.GeneratedAt)
}

func UpdateBridgeClient(params BridgeClientUpdateParams) (dto.BridgeClientDetail, error) {
	client, err := getAuthorizedBridgeClient(params.ClientId, params.UserId)
	if err != nil {
		return dto.BridgeClientDetail{}, err
	}
	updates, err := bridgeClientUpdateFields(params.Request)
	if err != nil {
		return dto.BridgeClientDetail{}, err
	}
	if len(updates) > 0 {
		client, err = model.UpdateBridgeClientFields(client.ClientId, updates)
		if err != nil {
			return dto.BridgeClientDetail{}, err
		}
		syncBridgeHubClientMetadata(*client)
	}
	return GetBridgeClientDetail(BridgeClientDetailParams{
		UserId:   params.UserId,
		ClientId: client.ClientId,
	})
}

func ArchiveBridgeClient(params BridgeClientDetailParams) (dto.BridgeClientItem, error) {
	client, err := getAuthorizedBridgeClient(params.ClientId, params.UserId)
	if err != nil {
		return dto.BridgeClientItem{}, err
	}
	snapshot, ok := bridge.DefaultHub.GetByClient(client.ClientId)
	if ok {
		_, _ = bridge.DefaultHub.CloseSession(snapshot.SessionId, bridge.CloseSessionOptions{
			Reason: "client archived by admin",
			Notify: true,
		})
	}
	if err := model.CloseActiveBridgeSessionsForClient(client.ClientId, "client archived by admin"); err != nil {
		return dto.BridgeClientItem{}, err
	}
	archived, err := model.ArchiveBridgeClient(client.ClientId)
	if err != nil {
		return dto.BridgeClientItem{}, err
	}
	return bridgeClientToDTO(*archived), nil
}

func CloseBridgeSessionForAdmin(params BridgeSessionCloseParams) (dto.BridgeSessionItem, error) {
	sessionId := strings.TrimSpace(params.SessionId)
	if sessionId == "" {
		return dto.BridgeSessionItem{}, errors.New("session_id is required")
	}
	session, err := model.GetBridgeSessionBySessionId(sessionId)
	if err != nil {
		return dto.BridgeSessionItem{}, err
	}
	if _, err := getAuthorizedBridgeClient(session.ClientId, params.UserId); err != nil {
		return dto.BridgeSessionItem{}, err
	}
	reason := truncateBridgeString(params.Reason, 256)
	if reason == "" {
		reason = "closed by admin"
	}
	snapshot, ok := bridge.DefaultHub.CloseSession(sessionId, bridge.CloseSessionOptions{
		Reason: reason,
		Notify: true,
	})
	if ok {
		defer func() {
			_, _ = model.MarkBridgeClientOfflineIfNoOnlineSession(snapshot.ClientId)
		}()
	}
	if err := model.CloseBridgeSession(sessionId, model.BridgeSessionStatusClosed, reason); err != nil {
		return dto.BridgeSessionItem{}, err
	}
	closed, err := model.GetBridgeSessionBySessionId(sessionId)
	if err != nil {
		return dto.BridgeSessionItem{}, err
	}
	return bridgeSessionToDTO(*closed), nil
}

func ListBridgeAuditLogs(params BridgeAuditLogListParams) ([]dto.BridgeAuditLogItem, int64, error) {
	logs, total, err := model.ListBridgeAuditLogs(model.BridgeAuditLogFilter{
		UserId:    params.UserId,
		TokenId:   params.TokenId,
		ClientId:  params.ClientId,
		SessionId: params.SessionId,
		ToolName:  params.ToolName,
		Status:    params.Status,
		RequestId: params.RequestId,
		StartTime: params.StartTime,
		EndTime:   params.EndTime,
		Keyword:   params.Keyword,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.BridgeAuditLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, bridgeAuditLogToDTO(log))
	}
	return items, total, nil
}

func getAuthorizedBridgeClient(clientId string, userId int) (*model.BridgeClient, error) {
	clientId = strings.TrimSpace(clientId)
	if clientId == "" {
		return nil, errors.New("client_id is required")
	}
	client, err := model.GetBridgeClientByClientId(clientId)
	if err != nil {
		return nil, err
	}
	if userId > 0 && client.UserId != userId {
		return nil, errors.New("bridge client not found")
	}
	return client, nil
}

func bridgeClientUpdateFields(req dto.BridgeClientUpdateRequest) (map[string]any, error) {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = truncateBridgeString(*req.Name, 128)
	}
	if req.Version != nil {
		updates["version"] = truncateBridgeString(*req.Version, 64)
	}
	if req.Platform != nil {
		updates["platform"] = truncateBridgeString(*req.Platform, 64)
	}
	if req.Workspace != nil {
		updates["workspace"] = truncateBridgeString(*req.Workspace, 512)
	}
	if req.Capabilities != nil {
		capabilities := normalizeBridgeCapabilities(*req.Capabilities)
		capabilitiesJSON, err := common.Marshal(capabilities)
		if err != nil {
			return nil, err
		}
		updates["capabilities"] = string(capabilitiesJSON)
	}
	if req.Status != nil {
		switch *req.Status {
		case model.BridgeClientStatusOffline, model.BridgeClientStatusOnline:
			updates["status"] = *req.Status
		default:
			return nil, errors.New("invalid bridge client status")
		}
	}
	if req.Policy != nil {
		rawPolicy, err := bridgepolicy.Marshal(*req.Policy)
		if err != nil {
			return nil, err
		}
		updates["policy"] = rawPolicy
	}
	return updates, nil
}

func ensureBridgeAgentToken(userId int, existing *model.BridgeClient, tokenName string, rotate bool) (*model.Token, string, bool, bool, error) {
	now := common.GetTimestamp()
	if existing != nil && existing.TokenId > 0 {
		token, err := model.GetTokenByIds(existing.TokenId, userId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", false, false, err
		}
		if err == nil && !rotate && tunnelAgentTokenReusable(token, now) {
			return token, "", false, false, nil
		}
		if err == nil {
			if err := model.DisableTokenWithTx(model.DB, token.Id, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", false, false, err
			}
		}
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, "", false, false, err
	}
	token := &model.Token{
		UserId:                userId,
		Name:                  tokenName,
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
	return token, key, true, rotate && existing != nil && existing.TokenId > 0, nil
}

func generateBridgeAgentClientId() (string, error) {
	for range 8 {
		clientId := "bridge-" + strings.ToLower(common.GetRandomString(24))
		_, err := model.GetBridgeClientByClientIdUnscoped(clientId)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return clientId, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("failed to allocate bridge client_id")
}

func bridgeAgentSetupTokenHash(token string) string {
	sum := sha256.Sum256([]byte("bridge_agent_setup_token:v1:" + strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func bridgeAgentSetupClientId(requestClientId string, tokenClientId string) string {
	if strings.TrimSpace(tokenClientId) != "" {
		return strings.TrimSpace(tokenClientId)
	}
	return strings.TrimSpace(requestClientId)
}

func firstNonEmptyBridgeString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func bridgeAgentShellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == '/' || r == ':' {
			continue
		}
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}
	return value
}

func bridgeAgentClientName(req dto.BridgeAgentSetupRequest, existing *model.BridgeClient, clientId string) string {
	name := truncateBridgeString(req.ClientName, 128)
	if name != "" {
		return name
	}
	if existing != nil && strings.TrimSpace(existing.Name) != "" {
		return existing.Name
	}
	if strings.TrimSpace(req.Workspace) != "" {
		return limitTunnelString("Bridge Agent - "+req.Workspace, 128)
	}
	return limitTunnelString("Bridge Agent - "+clientId, 128)
}

func bridgeAgentTokenName(req dto.BridgeAgentSetupRequest, clientId string, existing *model.BridgeClient) string {
	return limitTunnelString("Bridge Agent - "+bridgeAgentClientName(req, existing, clientId), 50)
}

func bridgeAgentCapabilities() []string {
	return []string{"mcp", "tunnel", "local_agent"}
}

func normalizeBridgeCapabilities(capabilities []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		capability = truncateBridgeString(capability, 128)
		if capability == "" || seen[capability] {
			continue
		}
		seen[capability] = true
		result = append(result, capability)
	}
	return result
}

func syncBridgeHubClientMetadata(client model.BridgeClient) {
	item := bridgeClientToDTO(client)
	bridge.DefaultHub.UpdateClientMetadata(client.ClientId, bridge.SessionMetadata{
		Name:         client.Name,
		Version:      client.Version,
		Platform:     client.Platform,
		Workspace:    client.Workspace,
		Capabilities: item.Capabilities,
	})
}

func bridgeClientToDTO(client model.BridgeClient) dto.BridgeClientItem {
	var capabilities []string
	if client.Capabilities != "" {
		_ = common.UnmarshalJsonStr(client.Capabilities, &capabilities)
	}
	policy, err := bridgepolicy.Parse(client.Policy)
	if err != nil {
		policy = bridgepolicy.Policy{}
	}
	item := dto.BridgeClientItem{
		Id:           client.Id,
		ClientId:     client.ClientId,
		UserId:       client.UserId,
		TokenId:      client.TokenId,
		Name:         client.Name,
		Version:      client.Version,
		Platform:     client.Platform,
		Workspace:    client.Workspace,
		Capabilities: capabilities,
		Policy:       policy,
		Status:       client.Status,
		LastSeenAt:   client.LastSeenAt,
		CreatedAt:    client.CreatedAt,
		UpdatedAt:    client.UpdatedAt,
	}
	if snapshot, ok := bridge.DefaultHub.GetByClient(client.ClientId); ok {
		item.Online = true
		item.SessionId = snapshot.SessionId
	}
	return item
}

func bridgeSessionsToDTO(sessions []model.BridgeSession) []dto.BridgeSessionItem {
	items := make([]dto.BridgeSessionItem, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, bridgeSessionToDTO(session))
	}
	return items
}

func bridgeSessionToDTO(session model.BridgeSession) dto.BridgeSessionItem {
	return dto.BridgeSessionItem{
		Id:          session.Id,
		SessionId:   session.SessionId,
		ClientId:    session.ClientId,
		UserId:      session.UserId,
		TokenId:     session.TokenId,
		RequestIP:   session.RequestIP,
		UserAgent:   session.UserAgent,
		Status:      session.Status,
		ConnectedAt: session.ConnectedAt,
		LastPingAt:  session.LastPingAt,
		ClosedAt:    session.ClosedAt,
		CloseReason: session.CloseReason,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}

func bridgeSessionSnapshotToDTO(snapshot bridge.SessionSnapshot) dto.BridgeSessionSnapshot {
	return dto.BridgeSessionSnapshot{
		SessionId:    snapshot.SessionId,
		ClientId:     snapshot.ClientId,
		UserId:       snapshot.UserId,
		TokenId:      snapshot.TokenId,
		Name:         snapshot.Name,
		Version:      snapshot.Version,
		Platform:     snapshot.Platform,
		Workspace:    snapshot.Workspace,
		Capabilities: snapshot.Capabilities,
		ConnectedAt:  bridgeSnapshotTimeUnix(snapshot.ConnectedAt),
		LastSeenAt:   bridgeSnapshotTimeUnix(snapshot.LastSeenAt),
	}
}

func bridgeAuditHealthFromStats(stats model.BridgeAuditLogStats) dto.BridgeAuditHealth {
	return dto.BridgeAuditHealth{
		TotalRequests: stats.TotalRequests,
		Success:       stats.Success,
		Error:         stats.Error,
		Timeout:       stats.Timeout,
		Pending:       stats.Pending,
		ResultSize:    stats.ResultSize,
		AvgDurationMS: stats.AvgDurationMS,
		SuccessRate:   ratioPercent(stats.Success, stats.TotalRequests),
	}
}

func bridgeRecentErrorsToDTO(logs []model.BridgeAuditLog) []dto.BridgeRecentError {
	items := make([]dto.BridgeRecentError, 0, len(logs))
	for _, log := range logs {
		items = append(items, dto.BridgeRecentError{
			Id:           log.Id,
			RequestId:    log.RequestId,
			SessionId:    log.SessionId,
			ClientId:     log.ClientId,
			ToolName:     log.ToolName,
			Status:       log.Status,
			ErrorCode:    log.ErrorCode,
			ErrorMessage: log.ErrorMessage,
			DurationMS:   log.DurationMS,
			CreatedAt:    log.CreatedAt,
		})
	}
	return items
}

func bridgeAgentHealthFromClient(client model.BridgeClient) *dto.BridgeAgentHealthReport {
	if strings.TrimSpace(client.HealthJson) == "" {
		return nil
	}
	var report dto.BridgeAgentHealthReport
	if err := common.UnmarshalJsonStr(client.HealthJson, &report); err != nil {
		return nil
	}
	if report.GeneratedAt == 0 {
		report.GeneratedAt = client.HealthReportedAt
	}
	return &report
}

func normalizeBridgeAgentHealthReport(report dto.BridgeAgentHealthReport) dto.BridgeAgentHealthReport {
	if report.GeneratedAt <= 0 {
		report.GeneratedAt = common.GetTimestamp()
	}
	report.Version = truncateBridgeString(report.Version, 64)
	report.Platform = truncateBridgeString(report.Platform, 64)
	report.Workspace = truncateBridgeString(report.Workspace, 512)
	if len(report.Checks) > maxBridgeAgentHealthChecks {
		report.Checks = report.Checks[:maxBridgeAgentHealthChecks]
	}
	if len(report.MCPProcesses) > maxBridgeAgentMCPProcesses {
		report.MCPProcesses = report.MCPProcesses[:maxBridgeAgentMCPProcesses]
	}
	for i := range report.Checks {
		report.Checks[i].Name = truncateBridgeString(report.Checks[i].Name, 128)
		report.Checks[i].Status = normalizeBridgeAgentHealthStatus(report.Checks[i].Status)
		report.Checks[i].Detail = truncateBridgeString(report.Checks[i].Detail, 512)
	}
	for i := range report.MCPProcesses {
		report.MCPProcesses[i].Name = truncateBridgeString(report.MCPProcesses[i].Name, 128)
		report.MCPProcesses[i].Transport = truncateBridgeString(report.MCPProcesses[i].Transport, 32)
		report.MCPProcesses[i].Status = normalizeBridgeAgentMCPProcessStatus(report.MCPProcesses[i].Status)
		report.MCPProcesses[i].StderrClass = truncateBridgeString(report.MCPProcesses[i].StderrClass, 64)
		report.MCPProcesses[i].ExitError = truncateBridgeString(report.MCPProcesses[i].ExitError, 256)
		report.MCPProcesses[i].Detail = truncateBridgeString(report.MCPProcesses[i].Detail, 256)
	}
	report.Summary = summarizeBridgeAgentHealthChecks(report.Checks)
	return report
}

func normalizeBridgeAgentMCPProcessStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "running":
		return "running"
	case "exited", "exit":
		return "exited"
	case "config_error", "config-error", "invalid":
		return "config_error"
	default:
		return "not_started"
	}
}

func summarizeBridgeAgentHealthChecks(checks []dto.BridgeAgentHealthCheck) dto.BridgeAgentHealthSummary {
	summary := dto.BridgeAgentHealthSummary{
		Status: "ok",
		Total:  len(checks),
	}
	for _, check := range checks {
		switch normalizeBridgeAgentHealthStatus(check.Status) {
		case "fail":
			summary.Fail++
		case "warn":
			summary.Warn++
		default:
			summary.OK++
		}
	}
	if summary.Fail > 0 {
		summary.Status = "fail"
	} else if summary.Warn > 0 {
		summary.Status = "warn"
	}
	return summary
}

func normalizeBridgeAgentHealthStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "fail", "failed", "error":
		return "fail"
	case "warn", "warning":
		return "warn"
	default:
		return "ok"
	}
}

func bridgeAuditLogToDTO(log model.BridgeAuditLog) dto.BridgeAuditLogItem {
	return dto.BridgeAuditLogItem{
		Id:           log.Id,
		RequestId:    log.RequestId,
		SessionId:    log.SessionId,
		ClientId:     log.ClientId,
		UserId:       log.UserId,
		TokenId:      log.TokenId,
		ToolName:     log.ToolName,
		RequestBody:  log.RequestBody,
		Status:       log.Status,
		ErrorCode:    log.ErrorCode,
		ErrorMessage: log.ErrorMessage,
		DurationMS:   log.DurationMS,
		ResultSize:   log.ResultSize,
		CreatedAt:    log.CreatedAt,
		UpdatedAt:    log.UpdatedAt,
	}
}

func bridgeSnapshotTimeUnix(value interface {
	IsZero() bool
	Unix() int64
}) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

func truncateBridgeString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
