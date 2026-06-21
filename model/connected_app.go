package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ConnectedAppSlugSnapless = "snapless"

	ConnectedAppStatusEnabled  = 1
	ConnectedAppStatusDisabled = 2

	ConnectedAppAuthorizationFlowDeviceCode = "device_code"

	ConnectedAppGrantStatusAuthorized = "authorized"
	ConnectedAppGrantStatusRevoked    = "revoked"

	ConnectedAppTokenBindingStatusActive  = "active"
	ConnectedAppTokenBindingStatusRevoked = "revoked"

	ConnectedAppDeviceSessionStatusPending    = "pending"
	ConnectedAppDeviceSessionStatusAuthorized = "authorized"
	ConnectedAppDeviceSessionStatusConsumed   = "consumed"
	ConnectedAppDeviceSessionStatusExpired    = "expired"
	ConnectedAppDeviceSessionStatusDenied     = "denied"
)

type ConnectedApp struct {
	Id                int    `json:"id" gorm:"primaryKey"`
	Slug              string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex"`
	Name              string `json:"name" gorm:"type:varchar(128);not null"`
	Description       string `json:"description" gorm:"type:varchar(512);not null;default:''"`
	AllowedScopes     string `json:"allowed_scopes" gorm:"type:varchar(512);not null;default:''"`
	DefaultScopes     string `json:"default_scopes" gorm:"type:varchar(512);not null;default:''"`
	AuthorizationFlow string `json:"authorization_flow" gorm:"type:varchar(32);not null;default:'device_code'"`
	Trusted           bool   `json:"trusted" gorm:"not null;default:false"`
	Status            int    `json:"status" gorm:"not null;default:1;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedApp) TableName() string {
	return "connected_apps"
}

func (app *ConnectedApp) ScopeList() []string {
	return splitConnectedAppScopes(app.AllowedScopes)
}

func (app *ConnectedApp) DefaultScopeList() []string {
	return splitConnectedAppScopes(app.DefaultScopes)
}

type ConnectedAppGrant struct {
	Id           int64  `json:"id" gorm:"primaryKey"`
	AppId        int    `json:"app_id" gorm:"not null;uniqueIndex:idx_connected_app_grants_app_user,priority:1;index"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_connected_app_grants_app_user,priority:2;index"`
	Scopes       string `json:"scopes" gorm:"type:varchar(512);not null;default:''"`
	Status       string `json:"status" gorm:"type:varchar(32);not null;default:'authorized';index"`
	AuthorizedAt int64  `json:"authorized_at" gorm:"bigint;default:0"`
	LastUsedAt   int64  `json:"last_used_at" gorm:"bigint;default:0;index"`
	RevokedAt    int64  `json:"revoked_at" gorm:"bigint;default:0;index"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppGrant) TableName() string {
	return "connected_app_grants"
}

func (grant *ConnectedAppGrant) ScopeList() []string {
	return splitConnectedAppScopes(grant.Scopes)
}

type ConnectedAppTokenBinding struct {
	Id                int64  `json:"id" gorm:"primaryKey"`
	AppId             int    `json:"app_id" gorm:"not null;uniqueIndex:idx_connected_app_token_bindings_app_user_device,priority:1;index"`
	GrantId           int64  `json:"grant_id" gorm:"not null;index"`
	UserId            int    `json:"user_id" gorm:"not null;uniqueIndex:idx_connected_app_token_bindings_app_user_device,priority:2;index"`
	TokenId           int    `json:"token_id" gorm:"not null;uniqueIndex:idx_connected_app_token_bindings_token_id"`
	DeviceFingerprint string `json:"device_fingerprint" gorm:"type:varchar(128);not null;uniqueIndex:idx_connected_app_token_bindings_app_user_device,priority:3"`
	DeviceName        string `json:"device_name" gorm:"type:varchar(128);not null;default:''"`
	Platform          string `json:"platform" gorm:"type:varchar(32);not null;default:'';index"`
	AppVersion        string `json:"app_version" gorm:"type:varchar(64);not null;default:'';index"`
	Status            string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	LastUsedAt        int64  `json:"last_used_at" gorm:"bigint;default:0;index"`
	RevokedAt         int64  `json:"revoked_at" gorm:"bigint;default:0;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppTokenBinding) TableName() string {
	return "connected_app_token_bindings"
}

type ConnectedAppTokenAttribution struct {
	Id                int64  `json:"id" gorm:"primaryKey"`
	AppId             int    `json:"app_id" gorm:"not null;index"`
	GrantId           int64  `json:"grant_id" gorm:"not null;index"`
	BindingId         int64  `json:"binding_id" gorm:"not null;index"`
	UserId            int    `json:"user_id" gorm:"not null;index"`
	TokenId           int    `json:"token_id" gorm:"not null;uniqueIndex"`
	DeviceFingerprint string `json:"device_fingerprint" gorm:"type:varchar(128);not null;index"`
	DeviceName        string `json:"device_name" gorm:"type:varchar(128);not null;default:''"`
	Platform          string `json:"platform" gorm:"type:varchar(32);not null;default:'';index"`
	AppVersion        string `json:"app_version" gorm:"type:varchar(64);not null;default:'';index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppTokenAttribution) TableName() string {
	return "connected_app_token_attributions"
}

type ConnectedAppDeviceSession struct {
	Id                int64  `json:"id" gorm:"primaryKey"`
	AppId             int    `json:"app_id" gorm:"not null;index"`
	UserId            int    `json:"user_id" gorm:"not null;default:0;index"`
	TokenId           int    `json:"token_id" gorm:"not null;default:0;index"`
	TokenCreated      bool   `json:"token_created" gorm:"not null;default:false"`
	DeviceCode        string `json:"device_code" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserCode          string `json:"user_code" gorm:"type:varchar(32);not null;uniqueIndex"`
	DeviceFingerprint string `json:"device_fingerprint" gorm:"type:varchar(128);not null;index"`
	DeviceName        string `json:"device_name" gorm:"type:varchar(128);not null;default:''"`
	Platform          string `json:"platform" gorm:"type:varchar(32);not null;default:'';index"`
	AppVersion        string `json:"app_version" gorm:"type:varchar(64);not null;default:'';index"`
	Client            string `json:"client" gorm:"type:varchar(64);not null;default:'';index"`
	Status            string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	PollInterval      int    `json:"poll_interval" gorm:"not null;default:3"`
	ExpiresAt         int64  `json:"expires_at" gorm:"bigint;not null;index"`
	LastPolledAt      int64  `json:"last_polled_at" gorm:"bigint;default:0;index"`
	AuthorizedAt      int64  `json:"authorized_at" gorm:"bigint;default:0;index"`
	ConsumedAt        int64  `json:"consumed_at" gorm:"bigint;default:0;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppDeviceSession) TableName() string {
	return "connected_app_device_sessions"
}

func splitConnectedAppScopes(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	scopes := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		scope := strings.TrimSpace(field)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	return scopes
}

func EnsureBuiltinConnectedApps() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	app := ConnectedApp{
		Slug:              ConnectedAppSlugSnapless,
		Name:              "Snapless Desktop",
		Description:       "Desktop speech input, text processing, translation and selected-text Q&A through Data Proxy.",
		AllowedScopes:     "openai.models openai.chat openai.audio.transcriptions quota.read token.manage",
		DefaultScopes:     "openai.models openai.chat openai.audio.transcriptions quota.read token.manage",
		AuthorizationFlow: ConnectedAppAuthorizationFlowDeviceCode,
		Trusted:           true,
		Status:            ConnectedAppStatusEnabled,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description", "allowed_scopes", "default_scopes", "authorization_flow", "trusted", "updated_at"}),
	}).Create(&app).Error
}

func GetConnectedAppBySlug(slug string) (*ConnectedApp, error) {
	var app ConnectedApp
	if err := DB.Where("slug = ?", strings.TrimSpace(slug)).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func ListConnectedApps() ([]ConnectedApp, error) {
	var apps []ConnectedApp
	if err := DB.Order("slug ASC").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func GetConnectedAppByID(id int) (*ConnectedApp, error) {
	var app ConnectedApp
	if err := DB.Where("id = ?", id).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func CreateConnectedApp(app *ConnectedApp) error {
	return DB.Create(app).Error
}

func UpdateConnectedApp(app *ConnectedApp) error {
	return DB.Save(app).Error
}

func ListConnectedAppRequests(status string, applicantUserId int, offset int, limit int) ([]ConnectedAppRequest, int64, error) {
	var requests []ConnectedAppRequest
	query := DB.Model(&ConnectedAppRequest{})
	if normalizedStatus := strings.TrimSpace(status); normalizedStatus != "" {
		query = query.Where("status = ?", normalizedStatus)
	}
	if applicantUserId > 0 {
		query = query.Where("applicant_user_id = ?", applicantUserId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at desc, id desc").Offset(offset).Limit(limit).Find(&requests).Error; err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func GetConnectedAppRequestByID(id int) (*ConnectedAppRequest, error) {
	var request ConnectedAppRequest
	if err := DB.Where("id = ?", id).First(&request).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func CreateConnectedAppRequest(request *ConnectedAppRequest) error {
	return DB.Create(request).Error
}

func UpdateConnectedAppRequest(tx *gorm.DB, request *ConnectedAppRequest) error {
	if tx == nil {
		tx = DB
	}
	return tx.Save(request).Error
}

func ListConnectedAppAuditLogs(action string, targetType string, targetId int, actorUserId int, requestId string, offset int, limit int) ([]ConnectedAppAuditLog, int64, error) {
	var logs []ConnectedAppAuditLog
	query := DB.Model(&ConnectedAppAuditLog{})
	if normalizedAction := strings.TrimSpace(action); normalizedAction != "" {
		query = query.Where("action = ?", normalizedAction)
	}
	if normalizedTargetType := strings.TrimSpace(targetType); normalizedTargetType != "" {
		query = query.Where("target_type = ?", normalizedTargetType)
	}
	if targetId > 0 {
		query = query.Where("target_id = ?", targetId)
	}
	if actorUserId > 0 {
		query = query.Where("actor_user_id = ?", actorUserId)
	}
	if normalizedRequestId := strings.TrimSpace(requestId); normalizedRequestId != "" {
		query = query.Where("request_id = ?", normalizedRequestId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at desc, id desc").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func UpsertConnectedAppGrant(tx *gorm.DB, app ConnectedApp, userId int, scopes []string, now int64) (*ConnectedAppGrant, error) {
	if tx == nil {
		tx = DB
	}
	if app.Id <= 0 || userId <= 0 {
		return nil, errors.New("app and user are required")
	}
	scopeString := strings.Join(normalizeConnectedAppScopes(scopes), " ")
	grant := ConnectedAppGrant{
		AppId:        app.Id,
		UserId:       userId,
		Scopes:       scopeString,
		Status:       ConnectedAppGrantStatusAuthorized,
		AuthorizedAt: now,
		RevokedAt:    0,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "app_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"scopes":        scopeString,
			"status":        ConnectedAppGrantStatusAuthorized,
			"authorized_at": now,
			"revoked_at":    0,
			"updated_at":    now,
		}),
	}).Create(&grant).Error; err != nil {
		return nil, err
	}
	var stored ConnectedAppGrant
	if err := tx.Where("app_id = ? AND user_id = ?", app.Id, userId).First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}

func GetConnectedAppGrant(appId int, userId int) (*ConnectedAppGrant, error) {
	var grant ConnectedAppGrant
	if err := DB.Where("app_id = ? AND user_id = ?", appId, userId).First(&grant).Error; err != nil {
		return nil, err
	}
	return &grant, nil
}

func normalizeConnectedAppScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func NormalizeConnectedAppScopes(scopes []string) []string {
	return normalizeConnectedAppScopes(scopes)
}

func FindActiveConnectedAppTokenBinding(appId int, userId int, deviceFingerprint string) (*ConnectedAppTokenBinding, error) {
	var binding ConnectedAppTokenBinding
	err := DB.Where(
		"app_id = ? AND user_id = ? AND device_fingerprint = ? AND status = ?",
		appId,
		userId,
		deviceFingerprint,
		ConnectedAppTokenBindingStatusActive,
	).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func UpsertConnectedAppTokenBinding(tx *gorm.DB, binding ConnectedAppTokenBinding, now int64) (*ConnectedAppTokenBinding, error) {
	if tx == nil {
		tx = DB
	}
	if binding.AppId <= 0 || binding.GrantId <= 0 || binding.UserId <= 0 || binding.TokenId <= 0 || binding.DeviceFingerprint == "" {
		return nil, errors.New("app, grant, user, token and device fingerprint are required")
	}
	binding.Status = ConnectedAppTokenBindingStatusActive
	binding.RevokedAt = 0
	if binding.LastUsedAt == 0 {
		binding.LastUsedAt = now
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "app_id"}, {Name: "user_id"}, {Name: "device_fingerprint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"grant_id":     binding.GrantId,
			"token_id":     binding.TokenId,
			"device_name":  binding.DeviceName,
			"platform":     binding.Platform,
			"app_version":  binding.AppVersion,
			"status":       ConnectedAppTokenBindingStatusActive,
			"last_used_at": now,
			"revoked_at":   0,
			"updated_at":   now,
		}),
	}).Create(&binding).Error; err != nil {
		return nil, err
	}
	var stored ConnectedAppTokenBinding
	if err := tx.Where(
		"app_id = ? AND user_id = ? AND device_fingerprint = ?",
		binding.AppId,
		binding.UserId,
		binding.DeviceFingerprint,
	).First(&stored).Error; err != nil {
		return nil, err
	}
	return &stored, nil
}

func RecordConnectedAppTokenAttribution(tx *gorm.DB, binding ConnectedAppTokenBinding, now int64) error {
	if tx == nil {
		tx = DB
	}
	if binding.AppId <= 0 || binding.GrantId <= 0 || binding.Id <= 0 || binding.UserId <= 0 || binding.TokenId <= 0 || binding.DeviceFingerprint == "" {
		return errors.New("app, grant, binding, user, token and device fingerprint are required")
	}
	attribution := ConnectedAppTokenAttribution{
		AppId:             binding.AppId,
		GrantId:           binding.GrantId,
		BindingId:         binding.Id,
		UserId:            binding.UserId,
		TokenId:           binding.TokenId,
		DeviceFingerprint: binding.DeviceFingerprint,
		DeviceName:        binding.DeviceName,
		Platform:          binding.Platform,
		AppVersion:        binding.AppVersion,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoNothing: true,
	}).Create(&attribution).Error
}

func ListConnectedAppTokenAttributionsByApp(appId int) ([]ConnectedAppTokenAttribution, error) {
	var attributions []ConnectedAppTokenAttribution
	err := DB.Where("app_id = ?", appId).
		Order("created_at asc").
		Order("id asc").
		Find(&attributions).Error
	return attributions, err
}

func CountActiveConnectedAppTokenBindings(tx *gorm.DB, appId int, userId int) (int64, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&ConnectedAppTokenBinding{}).
		Where("app_id = ? AND user_id = ? AND status = ?", appId, userId, ConnectedAppTokenBindingStatusActive).
		Count(&count).Error
	return count, err
}

func GetConnectedAppTokenBindingByTokenId(tokenId int) (*ConnectedAppTokenBinding, error) {
	var binding ConnectedAppTokenBinding
	err := DB.Where("token_id = ?", tokenId).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func ListConnectedAppTokenBindings(appId int, userId int) ([]ConnectedAppTokenBinding, error) {
	var bindings []ConnectedAppTokenBinding
	err := DB.Where("app_id = ? AND user_id = ?", appId, userId).
		Order("status asc").
		Order("last_used_at desc").
		Order("updated_at desc").
		Order("id desc").
		Find(&bindings).Error
	return bindings, err
}

func TouchConnectedAppUsage(appId int, userId int, tokenId int, now int64) error {
	if appId <= 0 || userId <= 0 || now <= 0 {
		return nil
	}
	updates := map[string]any{"last_used_at": now, "updated_at": now}
	if err := DB.Model(&ConnectedAppGrant{}).
		Where("app_id = ? AND user_id = ? AND status = ?", appId, userId, ConnectedAppGrantStatusAuthorized).
		Updates(updates).Error; err != nil {
		return err
	}
	if tokenId > 0 {
		if err := DB.Model(&ConnectedAppTokenBinding{}).
			Where("app_id = ? AND user_id = ? AND token_id = ? AND status = ?", appId, userId, tokenId, ConnectedAppTokenBindingStatusActive).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func GetConnectedAppDeviceSessionByUserCode(userCode string) (*ConnectedAppDeviceSession, error) {
	var session ConnectedAppDeviceSession
	err := DB.Where("user_code = ?", strings.ToUpper(strings.TrimSpace(userCode))).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func GetConnectedAppDeviceSessionByDeviceCode(deviceCode string) (*ConnectedAppDeviceSession, error) {
	var session ConnectedAppDeviceSession
	err := DB.Where("device_code = ?", strings.TrimSpace(deviceCode)).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func CreateConnectedAppDeviceSession(session *ConnectedAppDeviceSession) error {
	if session == nil || session.AppId <= 0 || session.DeviceCode == "" || session.UserCode == "" || session.DeviceFingerprint == "" {
		return errors.New("app, device code, user code and device fingerprint are required")
	}
	session.UserCode = strings.ToUpper(strings.TrimSpace(session.UserCode))
	session.DeviceCode = strings.TrimSpace(session.DeviceCode)
	session.Status = ConnectedAppDeviceSessionStatusPending
	return DB.Create(session).Error
}

func ExpireConnectedAppDeviceSession(tx *gorm.DB, sessionId int64, now int64) error {
	if tx == nil {
		tx = DB
	}
	if sessionId <= 0 {
		return nil
	}
	return tx.Model(&ConnectedAppDeviceSession{}).
		Where("id = ? AND status = ?", sessionId, ConnectedAppDeviceSessionStatusPending).
		Updates(map[string]any{
			"status":     ConnectedAppDeviceSessionStatusExpired,
			"updated_at": now,
		}).Error
}

func RevokeConnectedAppTokenBinding(tx *gorm.DB, binding *ConnectedAppTokenBinding, now int64) error {
	if tx == nil {
		tx = DB
	}
	if binding == nil || binding.Id <= 0 {
		return nil
	}
	return tx.Model(&ConnectedAppTokenBinding{}).
		Where("id = ?", binding.Id).
		Updates(map[string]any{
			"status":     ConnectedAppTokenBindingStatusRevoked,
			"revoked_at": now,
			"updated_at": now,
		}).Error
}

func RevokeConnectedAppGrant(tx *gorm.DB, appId int, userId int, now int64) error {
	if tx == nil {
		tx = DB
	}
	if appId <= 0 || userId <= 0 {
		return nil
	}
	return tx.Model(&ConnectedAppGrant{}).
		Where("app_id = ? AND user_id = ?", appId, userId).
		Updates(map[string]any{
			"status":     ConnectedAppGrantStatusRevoked,
			"revoked_at": now,
			"updated_at": now,
		}).Error
}

func DisableTokenWithTx(tx *gorm.DB, tokenId int, userId int) error {
	if tx == nil {
		tx = DB
	}
	if tokenId <= 0 || userId <= 0 {
		return nil
	}
	var token Token
	if err := tx.Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
		return err
	}
	if err := tx.Model(&token).Select("status").Update("status", common.TokenStatusDisabled).Error; err != nil {
		return err
	}
	if !common.RedisEnabled {
		return nil
	}
	return cacheDeleteToken(token.Key)
}
