package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TunnelAppTypeMCPCode = "mcp_code"
	TunnelAppTypeHTTP    = "http_tunnel"
	TunnelAppTypeTCP     = "tcp_tunnel"
)

const (
	TunnelPermissionReadOnly    = "read_only"
	TunnelPermissionWrite       = "write"
	TunnelPermissionExecSafe    = "exec_safe"
	TunnelPermissionExecTrusted = "exec_trusted"
	TunnelPermissionTraffic     = "traffic"
)

const (
	TunnelAppStatusPending  = "pending"
	TunnelAppStatusApproved = "approved"
	TunnelAppStatusRejected = "rejected"
	TunnelAppStatusDisabled = "disabled"
	TunnelAppStatusArchived = "archived"
)

const (
	TunnelConnectionStatusActive  = "active"
	TunnelConnectionStatusRevoked = "revoked"
)

const (
	TunnelSessionStatusOnline  = "online"
	TunnelSessionStatusOffline = "offline"
)

const (
	TunnelRouteAuthPrivate = "private"
	TunnelRouteAuthToken   = "token"
	TunnelRouteAuthPublic  = "public"
)

const (
	TunnelAuditActionCreate       = "create"
	TunnelAuditActionUpdate       = "update"
	TunnelAuditActionAgentSetup   = "agent_setup"
	TunnelAuditActionReview       = "review"
	TunnelAuditActionConnect      = "connect"
	TunnelAuditActionDisconnect   = "disconnect"
	TunnelAuditActionRevoke       = "revoke"
	TunnelAuditActionProxyRequest = "proxy_request"
	TunnelAuditActionMCPToolCall  = "mcp_tool_call"
	TunnelAuditActionPolicyDeny   = "policy_deny"
	TunnelAuditActionRateLimit    = "rate_limit"
	TunnelAuditActionBillingDeny  = "billing_deny"
)

type TunnelApp struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	UserId         int    `json:"user_id" gorm:"not null;default:0;index"`
	Name           string `json:"name" gorm:"type:varchar(128);not null;default:''"`
	Description    string `json:"description" gorm:"type:varchar(512);not null;default:''"`
	AppType        string `json:"app_type" gorm:"type:varchar(32);not null;default:'mcp_code';index"`
	PermissionMode string `json:"permission_mode" gorm:"type:varchar(32);not null;default:'read_only';index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	PublicSlug     string `json:"public_slug" gorm:"type:varchar(128);not null;default:'';uniqueIndex"`

	BridgeClientId string `json:"bridge_client_id" gorm:"type:varchar(128);not null;default:'';index"`
	TargetHost     string `json:"target_host" gorm:"type:varchar(255);not null;default:''"`
	TargetPort     int    `json:"target_port" gorm:"not null;default:0"`
	TargetPath     string `json:"target_path" gorm:"type:varchar(255);not null;default:''"`

	PolicyJson  string `json:"policy_json" gorm:"type:text"`
	RouteJson   string `json:"route_json" gorm:"type:text"`
	BillingJson string `json:"billing_json" gorm:"type:text"`

	ApprovedBy int            `json:"approved_by" gorm:"not null;default:0;index"`
	ApprovedAt int64          `json:"approved_at" gorm:"not null;default:0;index"`
	ReviewNote string         `json:"review_note" gorm:"type:varchar(512);not null;default:''"`
	LastError  string         `json:"last_error" gorm:"type:varchar(1024);not null;default:''"`
	LastSeenAt int64          `json:"last_seen_at" gorm:"not null;default:0;index"`
	CreatedAt  int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt  int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TunnelApp) TableName() string {
	return "tunnel_apps"
}

func (app *TunnelApp) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if app.CreatedAt == 0 {
		app.CreatedAt = now
	}
	if app.UpdatedAt == 0 {
		app.UpdatedAt = now
	}
	normalizeTunnelApp(app)
	if app.PublicSlug == "" {
		app.PublicSlug = "tun-" + strings.ToLower(common.GetRandomString(12))
	}
	return nil
}

func (app *TunnelApp) BeforeUpdate(tx *gorm.DB) error {
	app.UpdatedAt = common.GetTimestamp()
	normalizeTunnelApp(app)
	return nil
}

type TunnelConnection struct {
	Id             int64          `json:"id" gorm:"primaryKey"`
	AppId          int64          `json:"app_id" gorm:"not null;default:0;index"`
	UserId         int            `json:"user_id" gorm:"not null;default:0;index"`
	AgentTokenId   int            `json:"agent_token_id" gorm:"not null;default:0;index"`
	Name           string         `json:"name" gorm:"type:varchar(128);not null;default:''"`
	KeyPrefix      string         `json:"key_prefix" gorm:"type:varchar(32);not null;default:'';index"`
	KeyHash        string         `json:"-" gorm:"type:varchar(64);not null;default:'';uniqueIndex"`
	PermissionMode string         `json:"permission_mode" gorm:"type:varchar(32);not null;default:'read_only';index"`
	Status         string         `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	ExpiresAt      int64          `json:"expires_at" gorm:"not null;default:0;index"`
	LastUsedAt     int64          `json:"last_used_at" gorm:"not null;default:0;index"`
	LastRequestId  string         `json:"last_request_id" gorm:"type:varchar(128);not null;default:''"`
	ConfigJson     string         `json:"config_json" gorm:"type:text"`
	RevokedAt      int64          `json:"revoked_at" gorm:"not null;default:0;index"`
	CreatedAt      int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt      int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TunnelConnection) TableName() string {
	return "tunnel_connections"
}

func (connection *TunnelConnection) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if connection.CreatedAt == 0 {
		connection.CreatedAt = now
	}
	if connection.UpdatedAt == 0 {
		connection.UpdatedAt = now
	}
	normalizeTunnelConnection(connection)
	return nil
}

func (connection *TunnelConnection) BeforeUpdate(tx *gorm.DB) error {
	connection.UpdatedAt = common.GetTimestamp()
	normalizeTunnelConnection(connection)
	return nil
}

type TunnelSession struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	AppId          int64  `json:"app_id" gorm:"not null;default:0;index"`
	UserId         int    `json:"user_id" gorm:"not null;default:0;index"`
	ConnectionId   int64  `json:"connection_id" gorm:"not null;default:0;index"`
	ConnectionName string `json:"connection_name" gorm:"type:varchar(128);not null;default:''"`
	KeyPrefix      string `json:"key_prefix" gorm:"type:varchar(32);not null;default:'';index"`
	SessionId      string `json:"session_id" gorm:"type:varchar(128);not null;default:'';uniqueIndex"`
	BridgeClientId string `json:"bridge_client_id" gorm:"type:varchar(128);not null;default:'';index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:'online';index"`
	ClientVersion  string `json:"client_version" gorm:"type:varchar(64);not null;default:''"`
	ClientIp       string `json:"client_ip" gorm:"type:varchar(64);not null;default:''"`
	UserAgent      string `json:"user_agent" gorm:"type:varchar(255);not null;default:''"`
	BytesIn        int64  `json:"bytes_in" gorm:"not null;default:0"`
	BytesOut       int64  `json:"bytes_out" gorm:"not null;default:0"`
	ConnectedAt    int64  `json:"connected_at" gorm:"not null;default:0;index"`
	LastSeenAt     int64  `json:"last_seen_at" gorm:"not null;default:0;index"`
	DisconnectedAt int64  `json:"disconnected_at" gorm:"not null;default:0;index"`
	CloseReason    string `json:"close_reason" gorm:"type:varchar(512);not null;default:''"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TunnelSession) TableName() string {
	return "tunnel_sessions"
}

func (session *TunnelSession) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if session.ConnectedAt == 0 {
		session.ConnectedAt = now
	}
	if session.LastSeenAt == 0 {
		session.LastSeenAt = session.ConnectedAt
	}
	if session.CreatedAt == 0 {
		session.CreatedAt = now
	}
	if session.UpdatedAt == 0 {
		session.UpdatedAt = now
	}
	normalizeTunnelSession(session)
	return nil
}

func (session *TunnelSession) BeforeUpdate(tx *gorm.DB) error {
	session.UpdatedAt = common.GetTimestamp()
	normalizeTunnelSession(session)
	return nil
}

type TunnelRoute struct {
	Id         int64          `json:"id" gorm:"primaryKey"`
	AppId      int64          `json:"app_id" gorm:"not null;default:0;index"`
	UserId     int            `json:"user_id" gorm:"not null;default:0;index"`
	Host       string         `json:"host" gorm:"type:varchar(255);not null;default:'';index"`
	PathPrefix string         `json:"path_prefix" gorm:"type:varchar(255);not null;default:'/'"`
	AuthMode   string         `json:"auth_mode" gorm:"type:varchar(32);not null;default:'private';index"`
	Status     string         `json:"status" gorm:"type:varchar(32);not null;default:'approved';index"`
	ConfigJson string         `json:"config_json" gorm:"type:text"`
	CreatedAt  int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt  int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TunnelRoute) TableName() string {
	return "tunnel_routes"
}

func (route *TunnelRoute) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if route.CreatedAt == 0 {
		route.CreatedAt = now
	}
	if route.UpdatedAt == 0 {
		route.UpdatedAt = now
	}
	normalizeTunnelRoute(route)
	return nil
}

func (route *TunnelRoute) BeforeUpdate(tx *gorm.DB) error {
	route.UpdatedAt = common.GetTimestamp()
	normalizeTunnelRoute(route)
	return nil
}

type TunnelAuditLog struct {
	Id                  int64  `json:"id" gorm:"primaryKey"`
	AppId               int64  `json:"app_id" gorm:"not null;default:0;index"`
	ConnectionId        int64  `json:"connection_id" gorm:"not null;default:0;index"`
	ConnectionKeyPrefix string `json:"connection_key_prefix" gorm:"type:varchar(32);not null;default:'';index"`
	SessionId           string `json:"session_id" gorm:"type:varchar(128);not null;default:'';index"`
	UserId              int    `json:"user_id" gorm:"not null;default:0;index"`
	ActorUserId         int    `json:"actor_user_id" gorm:"not null;default:0;index"`
	Action              string `json:"action" gorm:"type:varchar(64);not null;default:'';index"`
	Decision            string `json:"decision" gorm:"type:varchar(32);not null;default:'';index"`
	Reason              string `json:"reason" gorm:"type:varchar(512);not null;default:''"`
	RequestId           string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	ToolName            string `json:"tool_name" gorm:"type:varchar(160);not null;default:'';index"`
	Method              string `json:"method" gorm:"type:varchar(16);not null;default:''"`
	Path                string `json:"path" gorm:"type:varchar(512);not null;default:''"`
	BytesIn             int64  `json:"bytes_in" gorm:"not null;default:0"`
	BytesOut            int64  `json:"bytes_out" gorm:"not null;default:0"`
	DurationMS          int    `json:"duration_ms" gorm:"not null;default:0"`
	MetadataJson        string `json:"metadata_json" gorm:"type:text"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (TunnelAuditLog) TableName() string {
	return "tunnel_audit_logs"
}

func (log *TunnelAuditLog) BeforeCreate(tx *gorm.DB) error {
	if log.CreatedAt == 0 {
		log.CreatedAt = common.GetTimestamp()
	}
	log.Action = strings.TrimSpace(log.Action)
	log.Decision = strings.TrimSpace(log.Decision)
	log.Reason = strings.TrimSpace(log.Reason)
	return nil
}

type TunnelAppFilter struct {
	UserId  int
	AppType string
	Status  string
	Keyword string
}

type TunnelConnectionFilter struct {
	AppId   int64
	UserId  int
	Status  string
	Keyword string
}

type TunnelSessionFilter struct {
	AppId        int64
	ConnectionId int64
	UserId       int
	Status       string
	SessionId    string
	Keyword      string
	StartTime    int64
	EndTime      int64
}

type TunnelAuditLogFilter struct {
	AppId        int64
	ConnectionId int64
	UserId       int
	ActorUserId  int
	Action       string
	Decision     string
	RequestId    string
	ToolName     string
	SessionId    string
	Keyword      string
	StartTime    int64
	EndTime      int64
}

func CreateTunnelApp(app *TunnelApp) error {
	return DB.Create(app).Error
}

func GetTunnelAppById(id int64) (*TunnelApp, error) {
	var app TunnelApp
	err := DB.First(&app, "id = ?", id).Error
	return &app, err
}

func GetTunnelAppByPublicSlug(slug string) (*TunnelApp, error) {
	var app TunnelApp
	err := DB.First(&app, "public_slug = ?", strings.TrimSpace(strings.ToLower(slug))).Error
	return &app, err
}

func UpdateTunnelAppFields(id int64, updates map[string]any) (*TunnelApp, error) {
	if err := DB.Model(&TunnelApp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetTunnelAppById(id)
}

func ListTunnelApps(filter TunnelAppFilter, offset int, limit int) ([]TunnelApp, int64, error) {
	query := DB.Model(&TunnelApp{})
	query = applyTunnelAppFilter(query, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]TunnelApp, 0, limit)
	err := query.Order("id desc").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func CreateTunnelConnection(connection *TunnelConnection) error {
	return DB.Create(connection).Error
}

func GetTunnelConnectionById(id int64) (*TunnelConnection, error) {
	var connection TunnelConnection
	err := DB.First(&connection, "id = ?", id).Error
	return &connection, err
}

func GetTunnelConnectionByKeyHash(keyHash string) (*TunnelConnection, error) {
	var connection TunnelConnection
	err := DB.First(&connection, "key_hash = ?", strings.TrimSpace(strings.ToLower(keyHash))).Error
	return &connection, err
}

func UpdateTunnelConnectionFields(id int64, updates map[string]any) (*TunnelConnection, error) {
	if err := DB.Model(&TunnelConnection{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetTunnelConnectionById(id)
}

func ListTunnelConnections(filter TunnelConnectionFilter, offset int, limit int) ([]TunnelConnection, int64, error) {
	query := DB.Model(&TunnelConnection{})
	query = applyTunnelConnectionFilter(query, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]TunnelConnection, 0, limit)
	err := query.Order("id desc").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func CreateTunnelSession(session *TunnelSession) error {
	if session == nil {
		return nil
	}
	return DB.Create(session).Error
}

func GetTunnelSessionBySessionId(sessionId string) (*TunnelSession, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var session TunnelSession
	err := DB.First(&session, "session_id = ?", sessionId).Error
	return &session, err
}

func TouchTunnelSession(sessionId string) error {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&TunnelSession{}).Where("session_id = ?", sessionId).Updates(map[string]any{
		"last_seen_at": now,
		"updated_at":   now,
	}).Error
}

func AddTunnelSessionUsage(sessionId string, bytesIn int64, bytesOut int64) error {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil
	}
	now := common.GetTimestamp()
	updates := map[string]any{
		"last_seen_at": now,
		"updated_at":   now,
	}
	exprs := map[string]any{}
	if bytesIn > 0 {
		exprs["bytes_in"] = gorm.Expr("bytes_in + ?", bytesIn)
	}
	if bytesOut > 0 {
		exprs["bytes_out"] = gorm.Expr("bytes_out + ?", bytesOut)
	}
	for key, value := range exprs {
		updates[key] = value
	}
	return DB.Model(&TunnelSession{}).Where("session_id = ?", sessionId).Updates(updates).Error
}

func CloseTunnelSession(sessionId string, reason string) error {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&TunnelSession{}).Where("session_id = ?", sessionId).Updates(map[string]any{
		"status":          TunnelSessionStatusOffline,
		"last_seen_at":    now,
		"disconnected_at": now,
		"close_reason":    strings.TrimSpace(reason),
		"updated_at":      now,
	}).Error
}

func ListTunnelSessions(filter TunnelSessionFilter, offset int, limit int) ([]TunnelSession, int64, error) {
	query := DB.Model(&TunnelSession{})
	query = applyTunnelSessionFilter(query, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]TunnelSession, 0, limit)
	err := query.Order("last_seen_at desc, id desc").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func TouchTunnelConnectionUsage(id int64, requestId string) error {
	if id <= 0 {
		return nil
	}
	return DB.Model(&TunnelConnection{}).Where("id = ?", id).Updates(map[string]any{
		"last_used_at":    common.GetTimestamp(),
		"last_request_id": strings.TrimSpace(requestId),
	}).Error
}

func CreateTunnelAuditLog(log *TunnelAuditLog) error {
	return DB.Create(log).Error
}

func ListTunnelAuditLogs(filter TunnelAuditLogFilter, offset int, limit int) ([]TunnelAuditLog, int64, error) {
	query := DB.Model(&TunnelAuditLog{})
	query = applyTunnelAuditLogFilter(query, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]TunnelAuditLog, 0, limit)
	err := query.Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func normalizeTunnelApp(app *TunnelApp) {
	app.Name = strings.TrimSpace(app.Name)
	app.Description = strings.TrimSpace(app.Description)
	app.AppType = strings.TrimSpace(strings.ToLower(app.AppType))
	app.PermissionMode = strings.TrimSpace(strings.ToLower(app.PermissionMode))
	app.Status = strings.TrimSpace(strings.ToLower(app.Status))
	app.PublicSlug = strings.TrimSpace(strings.ToLower(app.PublicSlug))
	app.BridgeClientId = strings.TrimSpace(app.BridgeClientId)
	app.TargetHost = strings.TrimSpace(strings.ToLower(app.TargetHost))
	app.TargetPath = strings.TrimSpace(app.TargetPath)
	app.ReviewNote = strings.TrimSpace(app.ReviewNote)
	app.LastError = strings.TrimSpace(app.LastError)
}

func normalizeTunnelSession(session *TunnelSession) {
	session.ConnectionName = strings.TrimSpace(session.ConnectionName)
	session.KeyPrefix = strings.TrimSpace(session.KeyPrefix)
	session.SessionId = strings.TrimSpace(session.SessionId)
	session.BridgeClientId = strings.TrimSpace(session.BridgeClientId)
	session.Status = strings.TrimSpace(strings.ToLower(session.Status))
	session.ClientVersion = strings.TrimSpace(session.ClientVersion)
	session.ClientIp = strings.TrimSpace(session.ClientIp)
	session.UserAgent = strings.TrimSpace(session.UserAgent)
	session.CloseReason = strings.TrimSpace(session.CloseReason)
}

func normalizeTunnelRoute(route *TunnelRoute) {
	route.Host = strings.TrimSpace(strings.ToLower(route.Host))
	route.PathPrefix = strings.TrimSpace(route.PathPrefix)
	if route.PathPrefix == "" {
		route.PathPrefix = "/"
	}
	route.AuthMode = strings.TrimSpace(strings.ToLower(route.AuthMode))
	route.Status = strings.TrimSpace(strings.ToLower(route.Status))
}

func normalizeTunnelConnection(connection *TunnelConnection) {
	connection.Name = strings.TrimSpace(connection.Name)
	connection.KeyPrefix = strings.TrimSpace(connection.KeyPrefix)
	connection.KeyHash = strings.TrimSpace(strings.ToLower(connection.KeyHash))
	connection.PermissionMode = strings.TrimSpace(strings.ToLower(connection.PermissionMode))
	if connection.PermissionMode == "" {
		connection.PermissionMode = TunnelPermissionReadOnly
	}
	connection.Status = strings.TrimSpace(strings.ToLower(connection.Status))
	if connection.Status == "" {
		connection.Status = TunnelConnectionStatusActive
	}
	connection.LastRequestId = strings.TrimSpace(connection.LastRequestId)
}

func applyTunnelAppFilter(query *gorm.DB, filter TunnelAppFilter) *gorm.DB {
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if strings.TrimSpace(filter.AppType) != "" {
		query = query.Where("app_type = ?", strings.TrimSpace(filter.AppType))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("name LIKE ? OR public_slug LIKE ? OR bridge_client_id LIKE ?", keyword, keyword, keyword)
	}
	return query
}

func applyTunnelConnectionFilter(query *gorm.DB, filter TunnelConnectionFilter) *gorm.DB {
	if filter.AppId > 0 {
		query = query.Where("app_id = ?", filter.AppId)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(strings.ToLower(filter.Status)))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("name LIKE ? OR key_prefix LIKE ? OR last_request_id LIKE ?", keyword, keyword, keyword)
	}
	return query
}

func applyTunnelSessionFilter(query *gorm.DB, filter TunnelSessionFilter) *gorm.DB {
	if filter.AppId > 0 {
		query = query.Where("app_id = ?", filter.AppId)
	}
	if filter.ConnectionId > 0 {
		query = query.Where("connection_id = ?", filter.ConnectionId)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(strings.ToLower(filter.Status)))
	}
	if strings.TrimSpace(filter.SessionId) != "" {
		query = query.Where("session_id = ?", strings.TrimSpace(filter.SessionId))
	}
	if filter.StartTime > 0 {
		query = query.Where("last_seen_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("last_seen_at <= ?", filter.EndTime)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("session_id LIKE ? OR bridge_client_id LIKE ? OR connection_name LIKE ? OR key_prefix LIKE ? OR close_reason LIKE ?",
			keyword, keyword, keyword, keyword, keyword)
	}
	return query
}

func applyTunnelAuditLogFilter(query *gorm.DB, filter TunnelAuditLogFilter) *gorm.DB {
	if filter.AppId > 0 {
		query = query.Where("app_id = ?", filter.AppId)
	}
	if filter.ConnectionId > 0 {
		query = query.Where("connection_id = ?", filter.ConnectionId)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.ActorUserId > 0 {
		query = query.Where("actor_user_id = ?", filter.ActorUserId)
	}
	if strings.TrimSpace(filter.Action) != "" {
		query = query.Where("action = ?", strings.TrimSpace(filter.Action))
	}
	if strings.TrimSpace(filter.Decision) != "" {
		query = query.Where("decision = ?", strings.TrimSpace(filter.Decision))
	}
	if strings.TrimSpace(filter.RequestId) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(filter.RequestId))
	}
	if strings.TrimSpace(filter.ToolName) != "" {
		query = query.Where("tool_name = ?", strings.TrimSpace(filter.ToolName))
	}
	if strings.TrimSpace(filter.SessionId) != "" {
		query = query.Where("session_id = ?", strings.TrimSpace(filter.SessionId))
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("request_id LIKE ? OR session_id LIKE ? OR connection_key_prefix LIKE ? OR action LIKE ? OR decision LIKE ? OR tool_name LIKE ? OR reason LIKE ?",
			keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	return query
}
