package service

import (
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
	if ok {
		_ = model.MarkBridgeClientOffline(snapshot.ClientId)
	}
	return model.CloseBridgeSession(sessionId, model.BridgeSessionStatusClosed, reason)
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
		_ = model.MarkBridgeClientOffline(snapshot.ClientId)
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
