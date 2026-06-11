package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BridgeClientStatusOffline = 0
	BridgeClientStatusOnline  = 1

	BridgeSessionStatusOnline  = "online"
	BridgeSessionStatusClosed  = "closed"
	BridgeSessionStatusTimeout = "timeout"

	BridgeAuditStatusPending = "pending"
	BridgeAuditStatusSuccess = "success"
	BridgeAuditStatusError   = "error"
	BridgeAuditStatusTimeout = "timeout"
)

type BridgeClient struct {
	Id           int            `json:"id"`
	ClientId     string         `json:"client_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserId       int            `json:"user_id" gorm:"not null;index"`
	TokenId      int            `json:"token_id" gorm:"index"`
	Name         string         `json:"name" gorm:"type:varchar(128);not null;default:''"`
	Version      string         `json:"version" gorm:"type:varchar(64);not null;default:''"`
	Platform     string         `json:"platform" gorm:"type:varchar(64);not null;default:''"`
	Workspace    string         `json:"workspace" gorm:"type:varchar(512);not null;default:''"`
	Capabilities string         `json:"capabilities" gorm:"type:text"`
	Policy       string         `json:"policy" gorm:"type:text"`
	Status       int            `json:"status" gorm:"not null;default:0;index"`
	LastSeenAt   int64          `json:"last_seen_at" gorm:"bigint;index"`
	CreatedAt    int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (BridgeClient) TableName() string {
	return "bridge_clients"
}

func (client *BridgeClient) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	client.CreatedAt = now
	client.UpdatedAt = now
	if client.LastSeenAt == 0 {
		client.LastSeenAt = now
	}
	return nil
}

func (client *BridgeClient) BeforeUpdate(tx *gorm.DB) error {
	client.UpdatedAt = common.GetTimestamp()
	return nil
}

type BridgeSession struct {
	Id          int64  `json:"id"`
	SessionId   string `json:"session_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	ClientId    string `json:"client_id" gorm:"type:varchar(128);not null;index"`
	UserId      int    `json:"user_id" gorm:"not null;index"`
	TokenId     int    `json:"token_id" gorm:"index"`
	RequestIP   string `json:"request_ip" gorm:"type:varchar(64);not null;default:''"`
	UserAgent   string `json:"user_agent" gorm:"type:varchar(512);not null;default:''"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'online';index"`
	ConnectedAt int64  `json:"connected_at" gorm:"bigint;index"`
	LastPingAt  int64  `json:"last_ping_at" gorm:"bigint;index"`
	ClosedAt    int64  `json:"closed_at" gorm:"bigint;index"`
	CloseReason string `json:"close_reason" gorm:"type:varchar(256);not null;default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (BridgeSession) TableName() string {
	return "bridge_sessions"
}

func (session *BridgeSession) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	session.CreatedAt = now
	session.UpdatedAt = now
	if session.ConnectedAt == 0 {
		session.ConnectedAt = now
	}
	if session.LastPingAt == 0 {
		session.LastPingAt = now
	}
	return nil
}

func (session *BridgeSession) BeforeUpdate(tx *gorm.DB) error {
	session.UpdatedAt = common.GetTimestamp()
	return nil
}

type BridgeAuditLog struct {
	Id           int64  `json:"id"`
	RequestId    string `json:"request_id" gorm:"type:varchar(64);not null;index"`
	SessionId    string `json:"session_id" gorm:"type:varchar(64);not null;index"`
	ClientId     string `json:"client_id" gorm:"type:varchar(128);not null;index"`
	UserId       int    `json:"user_id" gorm:"not null;index"`
	TokenId      int    `json:"token_id" gorm:"index"`
	ToolName     string `json:"tool_name" gorm:"type:varchar(128);not null;index"`
	RequestBody  string `json:"request_body" gorm:"type:text"`
	Status       string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ErrorCode    string `json:"error_code" gorm:"type:varchar(64);not null;default:''"`
	ErrorMessage string `json:"error_message" gorm:"type:varchar(512);not null;default:''"`
	DurationMS   int    `json:"duration_ms"`
	ResultSize   int    `json:"result_size"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func (BridgeAuditLog) TableName() string {
	return "bridge_audit_logs"
}

func (log *BridgeAuditLog) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	log.CreatedAt = now
	log.UpdatedAt = now
	return nil
}

func (log *BridgeAuditLog) BeforeUpdate(tx *gorm.DB) error {
	log.UpdatedAt = common.GetTimestamp()
	return nil
}

type BridgeClientFilter struct {
	UserId  int
	Status  *int
	Keyword string
}

type BridgeAuditLogFilter struct {
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
}

func UpsertBridgeClient(client *BridgeClient) error {
	if client == nil || client.ClientId == "" {
		return nil
	}
	now := common.GetTimestamp()
	client.LastSeenAt = now
	client.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "client_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"user_id":      client.UserId,
			"token_id":     client.TokenId,
			"name":         client.Name,
			"version":      client.Version,
			"platform":     client.Platform,
			"workspace":    client.Workspace,
			"capabilities": client.Capabilities,
			"status":       client.Status,
			"last_seen_at": client.LastSeenAt,
			"updated_at":   client.UpdatedAt,
			"deleted_at":   nil,
		}),
	}).Create(client).Error
}

func MarkBridgeClientOnline(clientId string) error {
	if clientId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&BridgeClient{}).Where("client_id = ?", clientId).Updates(map[string]any{
		"status":       BridgeClientStatusOnline,
		"last_seen_at": now,
		"updated_at":   now,
	}).Error
}

func MarkBridgeClientOffline(clientId string) error {
	if clientId == "" {
		return nil
	}
	return DB.Model(&BridgeClient{}).Where("client_id = ?", clientId).Updates(map[string]any{
		"status":     BridgeClientStatusOffline,
		"updated_at": common.GetTimestamp(),
	}).Error
}

func UpdateBridgeClientFields(clientId string, updates map[string]any) (*BridgeClient, error) {
	clientId = strings.TrimSpace(clientId)
	if clientId == "" || len(updates) == 0 {
		return GetBridgeClientByClientId(clientId)
	}
	updates["updated_at"] = common.GetTimestamp()
	if err := DB.Model(&BridgeClient{}).Where("client_id = ?", clientId).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetBridgeClientByClientId(clientId)
}

func ArchiveBridgeClient(clientId string) (*BridgeClient, error) {
	clientId = strings.TrimSpace(clientId)
	now := common.GetTimestamp()
	if err := DB.Model(&BridgeClient{}).Where("client_id = ?", clientId).Updates(map[string]any{
		"status":     BridgeClientStatusOffline,
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("client_id = ?", clientId).Delete(&BridgeClient{}).Error; err != nil {
		return nil, err
	}
	var client BridgeClient
	err := DB.Unscoped().Where("client_id = ?", clientId).First(&client).Error
	return &client, err
}

func CreateBridgeSession(session *BridgeSession) error {
	return DB.Create(session).Error
}

func GetBridgeSessionBySessionId(sessionId string) (*BridgeSession, error) {
	sessionId = strings.TrimSpace(sessionId)
	var session BridgeSession
	err := DB.Where("session_id = ?", sessionId).First(&session).Error
	return &session, err
}

func CloseActiveBridgeSessionsForClient(clientId string, reason string) error {
	if clientId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&BridgeSession{}).
		Where("client_id = ? AND status = ?", clientId, BridgeSessionStatusOnline).
		Updates(map[string]any{
			"status":       BridgeSessionStatusClosed,
			"closed_at":    now,
			"close_reason": truncateBridgeModelString(reason, 256),
			"updated_at":   now,
		}).Error
}

func TouchBridgeSession(sessionId string) error {
	if sessionId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&BridgeSession{}).Where("session_id = ?", sessionId).Updates(map[string]any{
		"last_ping_at": now,
		"updated_at":   now,
	}).Error
}

func CloseBridgeSession(sessionId string, status string, reason string) error {
	if sessionId == "" {
		return nil
	}
	if status == "" {
		status = BridgeSessionStatusClosed
	}
	now := common.GetTimestamp()
	return DB.Model(&BridgeSession{}).Where("session_id = ?", sessionId).Updates(map[string]any{
		"status":       status,
		"closed_at":    now,
		"close_reason": truncateBridgeModelString(reason, 256),
		"updated_at":   now,
	}).Error
}

func CreateBridgeAuditLog(log *BridgeAuditLog) error {
	return DB.Create(log).Error
}

func UpdateBridgeAuditLogStatus(id int64, status string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["status"] = status
	updates["updated_at"] = common.GetTimestamp()
	return DB.Model(&BridgeAuditLog{}).Where("id = ?", id).Updates(updates).Error
}

func GetBridgeClientByClientId(clientId string) (*BridgeClient, error) {
	clientId = strings.TrimSpace(clientId)
	var client BridgeClient
	err := DB.Where("client_id = ?", clientId).First(&client).Error
	return &client, err
}

func GetBridgeClientByClientIdUnscoped(clientId string) (*BridgeClient, error) {
	clientId = strings.TrimSpace(clientId)
	var client BridgeClient
	err := DB.Unscoped().Where("client_id = ?", clientId).First(&client).Error
	return &client, err
}

func GetBridgeClientPolicyByClientId(clientId string) (bridgepolicy.Policy, error) {
	clientId = strings.TrimSpace(clientId)
	if clientId == "" || DB == nil {
		return bridgepolicy.Policy{}, nil
	}
	if !DB.Migrator().HasTable(&BridgeClient{}) {
		return bridgepolicy.Policy{}, nil
	}
	client, err := GetBridgeClientByClientId(clientId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bridgepolicy.Policy{}, nil
		}
		return bridgepolicy.Policy{}, err
	}
	return bridgepolicy.Parse(client.Policy)
}

func ListBridgeClients(filter BridgeClientFilter, offset int, limit int) ([]BridgeClient, int64, error) {
	query := DB.Model(&BridgeClient{})
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("client_id LIKE ? OR name LIKE ? OR platform LIKE ? OR workspace LIKE ?", keyword, keyword, keyword, keyword)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var clients []BridgeClient
	err := query.Order("last_seen_at desc, id desc").Limit(limit).Offset(offset).Find(&clients).Error
	return clients, total, err
}

func ListBridgeSessionsByClientId(clientId string, offset int, limit int) ([]BridgeSession, int64, error) {
	query := DB.Model(&BridgeSession{}).Where("client_id = ?", strings.TrimSpace(clientId))
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var sessions []BridgeSession
	err := query.Order("connected_at desc, id desc").Limit(limit).Offset(offset).Find(&sessions).Error
	return sessions, total, err
}

func ListBridgeAuditLogs(filter BridgeAuditLogFilter, offset int, limit int) ([]BridgeAuditLog, int64, error) {
	query := DB.Model(&BridgeAuditLog{})
	query = applyBridgeAuditLogFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []BridgeAuditLog
	err := query.Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func applyBridgeAuditLogFilter(query *gorm.DB, filter BridgeAuditLogFilter) *gorm.DB {
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if strings.TrimSpace(filter.ClientId) != "" {
		query = query.Where("client_id = ?", strings.TrimSpace(filter.ClientId))
	}
	if strings.TrimSpace(filter.SessionId) != "" {
		query = query.Where("session_id = ?", strings.TrimSpace(filter.SessionId))
	}
	if strings.TrimSpace(filter.ToolName) != "" {
		query = query.Where("tool_name = ?", strings.TrimSpace(filter.ToolName))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.RequestId) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(filter.RequestId))
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("request_id LIKE ? OR client_id LIKE ? OR session_id LIKE ? OR tool_name LIKE ? OR error_message LIKE ?",
			keyword, keyword, keyword, keyword, keyword)
	}
	return query
}

func truncateBridgeModelString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
