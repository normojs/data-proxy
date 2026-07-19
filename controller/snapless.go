package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	snaplessDeviceCodeTTLSeconds = 10 * 60
	snaplessDevicePollInterval   = 3
)

type snaplessDeviceRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	Client     string `json:"client"`
}

type snaplessDeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	DeviceName  string `json:"device_name"`
	Platform    string `json:"platform"`
	AppVersion  string `json:"app_version"`
	Client      string `json:"client"`
}

type snaplessModelAliases struct {
	ASR       string `json:"asr"`
	Chat      string `json:"chat"`
	Polish    string `json:"polish"`
	Translate string `json:"translate"`
	QA        string `json:"qa"`
}

type snaplessActionLink struct {
	Label  string `json:"label"`
	Href   string `json:"href"`
	Intent string `json:"intent,omitempty"`
}

type snaplessActionHints struct {
	Severity  string              `json:"severity,omitempty"`
	Reason    string              `json:"reason,omitempty"`
	Primary   *snaplessActionLink `json:"primary,omitempty"`
	Secondary *snaplessActionLink `json:"secondary,omitempty"`
}

type snaplessReadinessResponse struct {
	OK      bool                `json:"ok"`
	Status  string              `json:"status"`
	Checks  map[string]bool     `json:"checks"`
	Actions snaplessActionHints `json:"actions"`
}

type snaplessTokenResponse struct {
	App          snaplessAppResponse       `json:"app"`
	Grant        snaplessGrantResponse     `json:"grant"`
	Device       snaplessDeviceInfo        `json:"device"`
	Token        snaplessTokenSummary      `json:"token"`
	// User is filled on successful device poll (with api_key) so clients can
	// display the same account identity as the admin user list (DP-1).
	User         connectedAppClientUser    `json:"user,omitempty"`
	Models       snaplessModelAliases      `json:"models"`
	Endpoints    map[string]string         `json:"endpoints"`
	BaseURL      string                    `json:"base_url"`
	APIKey       string                    `json:"api_key,omitempty"`
	Created      bool                      `json:"created"`
	Rotated      bool                      `json:"rotated"`
	APIKeyOnce   bool                      `json:"api_key_once"`
	Instructions snaplessClientInstruction `json:"instructions"`
}

type snaplessClientInstruction struct {
	Authorization string `json:"authorization"`
}

type snaplessAppResponse struct {
	ID            int      `json:"id"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Trusted       bool     `json:"trusted"`
	Status        int      `json:"status"`
	AllowedScopes []string `json:"allowed_scopes"`
	DefaultScopes []string `json:"default_scopes"`
}

type snaplessGrantResponse struct {
	ID           int64    `json:"id,omitempty"`
	Status       string   `json:"status"`
	Scopes       []string `json:"scopes"`
	AuthorizedAt int64    `json:"authorized_at,omitempty"`
	LastUsedAt   int64    `json:"last_used_at,omitempty"`
	RevokedAt    int64    `json:"revoked_at,omitempty"`
}

type snaplessTokenSummary struct {
	ID                    int    `json:"id,omitempty"`
	Name                  string `json:"name,omitempty"`
	Status                int    `json:"status,omitempty"`
	MaskedKey             string `json:"masked_key,omitempty"`
	ExpiredTime           int64  `json:"expired_time,omitempty"`
	UnlimitedQuota        bool   `json:"unlimited_quota"`
	QuotaHardLimitEnabled bool   `json:"quota_hard_limit_enabled"`
	ModelLimitsEnabled    bool   `json:"model_limits_enabled"`
	ModelLimits           string `json:"model_limits,omitempty"`
	BindingStatus         string `json:"binding_status,omitempty"`
	LastUsedAt            int64  `json:"last_used_at,omitempty"`
}

type snaplessHealthResponse struct {
	OK      bool                           `json:"ok"`
	Status  string                         `json:"status"`
	Actions snaplessActionHints            `json:"actions"`
	App     snaplessAppResponse            `json:"app"`
	Grant   snaplessGrantResponse          `json:"grant"`
	Token   snaplessTokenHealth            `json:"token"`
	User    snaplessUserHealth             `json:"user"`
	Device  snaplessDeviceInfo             `json:"device"`
	Models  map[string]snaplessModelHealth `json:"models"`
	Aliases snaplessModelAliases           `json:"aliases"`
	BaseURL string                         `json:"base_url"`
	Checks  map[string]bool                `json:"checks"`
}

type snaplessDevicesResponse struct {
	OK      bool                           `json:"ok"`
	Status  string                         `json:"status"`
	Actions snaplessActionHints            `json:"actions"`
	App     snaplessAppResponse            `json:"app"`
	Grant   snaplessGrantResponse          `json:"grant"`
	Devices []snaplessManagedDevice        `json:"devices"`
	Models  map[string]snaplessModelHealth `json:"models"`
	Aliases snaplessModelAliases           `json:"aliases"`
	BaseURL string                         `json:"base_url"`
	Checks  map[string]bool                `json:"checks"`
}

type snaplessManagedDevice struct {
	OK         bool                 `json:"ok"`
	Status     string               `json:"status"`
	Actions    snaplessActionHints  `json:"actions"`
	Device     snaplessDeviceInfo   `json:"device"`
	Token      snaplessTokenSummary `json:"token"`
	Checks     map[string]bool      `json:"checks"`
	LastUsedAt int64                `json:"last_used_at,omitempty"`
	RevokedAt  int64                `json:"revoked_at,omitempty"`
	CreatedAt  int64                `json:"created_at,omitempty"`
	UpdatedAt  int64                `json:"updated_at,omitempty"`
}

type snaplessStatusEvaluation struct {
	OK     bool
	Status string
	Checks map[string]bool
}

type snaplessTokenHealth struct {
	ID              int    `json:"id,omitempty"`
	Status          int    `json:"status,omitempty"`
	Enabled         bool   `json:"enabled"`
	Expired         bool   `json:"expired"`
	QuotaOK         bool   `json:"quota_ok"`
	SnaplessBinding bool   `json:"snapless_binding"`
	BindingStatus   string `json:"binding_status,omitempty"`
}

type snaplessUserHealth struct {
	ID      int  `json:"id,omitempty"`
	Enabled bool `json:"enabled"`
	Quota   int  `json:"quota"`
	QuotaOK bool `json:"quota_ok"`
}

type snaplessModelHealth struct {
	Model     string `json:"model"`
	Available bool   `json:"available"`
}

type snaplessDeviceStartResponse struct {
	DeviceCode      string              `json:"device_code"`
	UserCode        string              `json:"user_code"`
	VerificationURI string              `json:"verification_uri"`
	ExpiresIn       int64               `json:"expires_in"`
	Interval        int                 `json:"interval"`
	App             snaplessAppResponse `json:"app"`
	Device          snaplessDeviceInfo  `json:"device"`
}

type snaplessDeviceStatusResponse struct {
	Status    string                    `json:"status"`
	ExpiresAt int64                     `json:"expires_at"`
	App       snaplessAppResponse       `json:"app"`
	Device    snaplessDeviceInfo        `json:"device"`
	Token     snaplessTokenSummary      `json:"token"`
	Readiness snaplessReadinessResponse `json:"readiness"`
}

type snaplessDeviceAuthorizeRequest struct {
	UserCode string `json:"user_code"`
	Approve  *bool  `json:"approve"`
}

type snaplessDevicePollRequest struct {
	DeviceCode string `json:"device_code"`
}

func GetSnaplessConfig(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aliases := getSnaplessModelAliases()
	availability, err := snaplessModelAvailability(aliases)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelsOK := snaplessModelsReady(availability)
	device := snaplessDeviceFromRequest(c, snaplessDeviceRequest{})
	userId := c.GetInt("id")

	var grant *model.ConnectedAppGrant
	if existingGrant, err := model.GetConnectedAppGrant(app.Id, userId); err == nil {
		grant = existingGrant
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	var tokenSummary snaplessTokenSummary
	var tokenForStatus *model.Token
	var bindingForStatus *model.ConnectedAppTokenBinding
	if binding, err := model.FindActiveConnectedAppTokenBinding(app.Id, userId, device.Fingerprint); err == nil {
		bindingForStatus = binding
		if token, tokenErr := model.GetTokenByIds(binding.TokenId, userId); tokenErr == nil {
			tokenForStatus = token
			tokenSummary = buildSnaplessTokenSummary(token, binding)
		} else if !errors.Is(tokenErr, gorm.ErrRecordNotFound) {
			common.ApiError(c, tokenErr)
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	userHealth, err := getSnaplessUserHealth(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	status := evaluateSnaplessReadiness(app, userHealth, modelsOK)
	if bindingForStatus != nil {
		status = evaluateSnaplessStatus(app, grant, bindingForStatus, tokenForStatus, userHealth, modelsOK, common.GetTimestamp())
	}

	common.ApiSuccess(c, gin.H{
		"app":          buildSnaplessAppResponse(app),
		"grant":        buildSnaplessGrantResponse(grant, app.DefaultScopeList()),
		"device":       device,
		"token":        tokenSummary,
		"models":       aliases,
		"model_health": buildSnaplessModelHealth(aliases, availability),
		"model_limits": aliases.All(),
		"base_url":     snaplessAPIBaseURL(c),
		"endpoints":    snaplessEndpoints(c),
		"ok":           status.OK,
		"status":       status.Status,
		"checks":       status.Checks,
		"actions":      buildSnaplessActionHints(status.Status),
	})
}

func StartSnaplessDeviceFlow(c *gin.Context) {
	req, err := bindSnaplessDeviceRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		common.ApiError(c, errors.New("Snapless 应用已停用"))
		return
	}
	device := snaplessDeviceFromRequest(c, req)
	now := common.GetTimestamp()
	expiresAt := now + snaplessDeviceCodeTTLSeconds

	var session *model.ConnectedAppDeviceSession
	for attempt := 0; attempt < 5; attempt++ {
		deviceCode, err := common.GenerateRandomCharsKey(64)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		userCode, err := snaplessUserCode()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		candidate := &model.ConnectedAppDeviceSession{
			AppId:             app.Id,
			DeviceCode:        deviceCode,
			UserCode:          userCode,
			DeviceFingerprint: device.Fingerprint,
			DeviceName:        device.DeviceName,
			Platform:          device.Platform,
			AppVersion:        device.AppVersion,
			Client:            device.Client,
			Status:            model.ConnectedAppDeviceSessionStatusPending,
			PollInterval:      snaplessDevicePollInterval,
			ExpiresAt:         expiresAt,
		}
		if err := model.CreateConnectedAppDeviceSession(candidate); err == nil {
			session = candidate
			break
		} else if attempt == 4 {
			common.ApiError(c, err)
			return
		}
	}
	if session == nil {
		common.ApiError(c, errors.New("创建设备授权会话失败"))
		return
	}

	common.ApiSuccess(c, snaplessDeviceStartResponse{
		DeviceCode:      session.DeviceCode,
		UserCode:        session.UserCode,
		VerificationURI: snaplessVerificationURI(c, session.UserCode),
		ExpiresIn:       expiresAt - now,
		Interval:        session.PollInterval,
		App:             buildSnaplessAppResponse(app),
		Device:          device,
	})
}

func GetSnaplessDeviceStatus(c *gin.Context) {
	userCode := normalizeSnaplessUserCode(firstNonEmpty(c.Query("user_code"), c.Param("user_code")))
	session, err := model.GetConnectedAppDeviceSessionByUserCode(userCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, errors.New("设备授权码不存在"))
			return
		}
		common.ApiError(c, err)
		return
	}
	app, err := model.GetConnectedAppBySlug(model.ConnectedAppSlugSnapless)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	if session.Status == model.ConnectedAppDeviceSessionStatusPending && session.ExpiresAt <= now {
		_ = model.ExpireConnectedAppDeviceSession(nil, session.Id, now)
		session.Status = model.ConnectedAppDeviceSessionStatusExpired
	}
	if session.UserId > 0 && session.UserId != c.GetInt("id") {
		common.ApiError(c, errors.New("设备授权码已被其他用户授权"))
		return
	}
	var tokenSummary snaplessTokenSummary
	if session.TokenId > 0 && session.UserId == c.GetInt("id") {
		if token, err := model.GetTokenByIds(session.TokenId, session.UserId); err == nil {
			tokenSummary = buildSnaplessTokenSummary(token, nil)
		}
	}
	readiness, err := snaplessReadinessForUser(app, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, snaplessDeviceStatusResponse{
		Status:    session.Status,
		ExpiresAt: session.ExpiresAt,
		App:       buildSnaplessAppResponse(app),
		Device:    snaplessDeviceInfoFromSession(session),
		Token:     tokenSummary,
		Readiness: readiness,
	})
}

func AuthorizeSnaplessDevice(c *gin.Context) {
	var req snaplessDeviceAuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.UserCode = normalizeSnaplessUserCode(req.UserCode)
	if req.UserCode == "" {
		common.ApiError(c, errors.New("设备授权码不能为空"))
		return
	}
	approve := true
	if req.Approve != nil {
		approve = *req.Approve
	}

	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		common.ApiError(c, errors.New("Snapless 应用已停用"))
		return
	}
	userId := c.GetInt("id")
	now := common.GetTimestamp()
	readiness, err := snaplessReadinessForUser(app, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var response snaplessDeviceStatusResponse
	var notifiedSession model.ConnectedAppDeviceSession
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var session model.ConnectedAppDeviceSession
		if err := tx.Clauses(clauseLockingUpdate()).Where("user_code = ?", req.UserCode).First(&session).Error; err != nil {
			return err
		}
		if session.AppId != app.Id {
			return errors.New("设备授权码不属于 Snapless")
		}
		if session.Status == model.ConnectedAppDeviceSessionStatusPending && session.ExpiresAt <= now {
			if err := model.ExpireConnectedAppDeviceSession(tx, session.Id, now); err != nil {
				return err
			}
			session.Status = model.ConnectedAppDeviceSessionStatusExpired
		}
		if session.Status != model.ConnectedAppDeviceSessionStatusPending {
			return snaplessDeviceFlowStatusError(session.Status)
		}
		if !approve {
			result := tx.Model(&model.ConnectedAppDeviceSession{}).
				Where("id = ? AND status = ?", session.Id, model.ConnectedAppDeviceSessionStatusPending).
				Updates(map[string]any{
					"user_id":    userId,
					"status":     model.ConnectedAppDeviceSessionStatusDenied,
					"updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.New("设备授权状态已更新，请刷新后重试")
			}
			session.UserId = userId
			session.Status = model.ConnectedAppDeviceSessionStatusDenied
			session.UpdatedAt = now
			notifiedSession = session
			response = snaplessDeviceStatusResponse{
				Status:    model.ConnectedAppDeviceSessionStatusDenied,
				ExpiresAt: session.ExpiresAt,
				App:       buildSnaplessAppResponse(app),
				Device:    snaplessDeviceInfoFromSession(&session),
				Readiness: readiness,
			}
			return nil
		}

		tokenResponse, tokenId, err := ensureSnaplessTokenForDeviceTx(c, tx, app, userId, snaplessDeviceInfoFromSession(&session), false)
		if err != nil {
			return err
		}
		result := tx.Model(&model.ConnectedAppDeviceSession{}).
			Where("id = ? AND status = ?", session.Id, model.ConnectedAppDeviceSessionStatusPending).
			Updates(map[string]any{
				"user_id":       userId,
				"token_id":      tokenId,
				"token_created": tokenResponse.Created,
				"status":        model.ConnectedAppDeviceSessionStatusAuthorized,
				"authorized_at": now,
				"updated_at":    now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("设备授权状态已更新，请刷新后重试")
		}
		session.UserId = userId
		session.TokenId = tokenId
		session.TokenCreated = tokenResponse.Created
		session.Status = model.ConnectedAppDeviceSessionStatusAuthorized
		session.AuthorizedAt = now
		session.UpdatedAt = now
		notifiedSession = session
		response = snaplessDeviceStatusResponse{
			Status:    model.ConnectedAppDeviceSessionStatusAuthorized,
			ExpiresAt: session.ExpiresAt,
			App:       tokenResponse.App,
			Device:    tokenResponse.Device,
			Token:     tokenResponse.Token,
			Readiness: readiness,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, errors.New("设备授权码不存在"))
			return
		}
		common.ApiError(c, err)
		return
	}
	if notifiedSession.Id > 0 {
		if err := service.EnqueueConnectedAppDeviceAuthorizationOutboxWithDB(model.DB, *app, notifiedSession); err != nil {
			common.SysLog("failed to enqueue snapless device authorization notification outbox: " + err.Error())
		}
	}
	common.ApiSuccess(c, response)
}

func PollSnaplessDeviceFlow(c *gin.Context) {
	var req snaplessDevicePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.DeviceCode = strings.TrimSpace(req.DeviceCode)
	if req.DeviceCode == "" {
		common.ApiError(c, errors.New("device_code 不能为空"))
		return
	}
	now := common.GetTimestamp()
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		common.ApiError(c, errors.New("Snapless 应用已停用"))
		return
	}

	response := snaplessTokenResponse{
		App:        buildSnaplessAppResponse(app),
		Created:    false,
		APIKeyOnce: false,
	}
	var status string
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var session model.ConnectedAppDeviceSession
		if err := tx.Clauses(clauseLockingUpdate()).Where("device_code = ?", req.DeviceCode).First(&session).Error; err != nil {
			return err
		}
		if session.AppId != app.Id {
			status = "invalid_app"
			return nil
		}
		response.Device = snaplessDeviceInfoFromSession(&session)
		response.BaseURL = snaplessAPIBaseURL(c)
		response.Endpoints = snaplessEndpoints(c)
		response.Models = getSnaplessModelAliases()

		if session.Status == model.ConnectedAppDeviceSessionStatusPending && session.ExpiresAt <= now {
			if err := model.ExpireConnectedAppDeviceSession(tx, session.Id, now); err != nil {
				return err
			}
			session.Status = model.ConnectedAppDeviceSessionStatusExpired
		}
		status = session.Status
		if session.Status != model.ConnectedAppDeviceSessionStatusAuthorized {
			if session.Status == model.ConnectedAppDeviceSessionStatusPending {
				if err := tx.Model(&model.ConnectedAppDeviceSession{}).
					Where("id = ?", session.Id).
					Updates(map[string]any{
						"last_polled_at": now,
						"updated_at":     now,
					}).Error; err != nil {
					return err
				}
			}
			return nil
		}
		if session.TokenId <= 0 || session.UserId <= 0 {
			status = "missing_token"
			return nil
		}
		token, err := getTokenByIdTx(tx, session.TokenId, session.UserId)
		if err != nil {
			return err
		}
		grant, err := getGrantTx(tx, app.Id, session.UserId)
		if err != nil {
			return err
		}
		binding, err := getBindingByTokenIdTx(tx, session.TokenId)
		if err != nil {
			return err
		}
		user, err := getUserByIdTx(tx, session.UserId)
		if err != nil {
			return err
		}
		result := tx.Model(&model.ConnectedAppDeviceSession{}).
			Where("id = ? AND status = ?", session.Id, model.ConnectedAppDeviceSessionStatusAuthorized).
			Updates(map[string]any{
				"status":         model.ConnectedAppDeviceSessionStatusConsumed,
				"consumed_at":    now,
				"last_polled_at": now,
				"updated_at":     now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			status = model.ConnectedAppDeviceSessionStatusConsumed
			return nil
		}
		response = buildSnaplessTokenResponse(c, app, grant, binding, token, getSnaplessModelAliases(), snaplessDeviceInfoFromSession(&session), token.Key, session.TokenCreated, false)
		response.User = connectedAppClientUserFromModel(user)
		status = model.ConnectedAppDeviceSessionStatusAuthorized
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiSuccess(c, gin.H{"status": "not_found"})
			return
		}
		common.ApiError(c, err)
		return
	}
	if status != model.ConnectedAppDeviceSessionStatusAuthorized {
		common.ApiSuccess(c, gin.H{
			"status":   status,
			"interval": snaplessDevicePollInterval,
		})
		return
	}
	common.ApiSuccess(c, response)
}

func EnsureSnaplessToken(c *gin.Context) {
	req, err := bindSnaplessDeviceRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response, tokenId, err := ensureSnaplessTokenForDevice(c, req, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if tokenId > 0 {
		_ = model.TouchConnectedAppUsage(response.App.ID, c.GetInt("id"), tokenId, common.GetTimestamp())
	}
	common.ApiSuccess(c, response)
}

func RotateSnaplessToken(c *gin.Context) {
	req, err := bindSnaplessDeviceRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	device := snaplessDeviceFromRequest(c, req)
	var previousBinding *model.ConnectedAppTokenBinding
	if binding, err := model.FindActiveConnectedAppTokenBinding(app.Id, userId, device.Fingerprint); err == nil {
		bindingCopy := *binding
		previousBinding = &bindingCopy
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	response, tokenId, err := ensureSnaplessTokenForDevice(c, req, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if tokenId > 0 {
		_ = model.TouchConnectedAppUsage(response.App.ID, userId, tokenId, common.GetTimestamp())
	}
	if response.Rotated && previousBinding != nil {
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventTokenRotated, userId, *previousBinding, previousBinding.TokenId, tokenId, common.GetTimestamp())
	}
	common.ApiSuccess(c, response)
}

func RevokeCurrentSnaplessToken(c *gin.Context) {
	req, err := bindSnaplessDeviceRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	device := snaplessDeviceFromRequest(c, req)
	now := common.GetTimestamp()

	var revokedTokenId int
	var grantRevoked bool
	var revokedBinding *model.ConnectedAppTokenBinding
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		binding, err := findSnaplessBindingTx(tx, app.Id, userId, device.Fingerprint)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		bindingCopy := *binding
		revokedBinding = &bindingCopy
		revokedTokenId = binding.TokenId
		if err := model.DisableTokenWithTx(tx, binding.TokenId, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := model.RevokeConnectedAppTokenBinding(tx, binding, now); err != nil {
			return err
		}
		count, err := model.CountActiveConnectedAppTokenBindings(tx, app.Id, userId)
		if err != nil {
			return err
		}
		if count == 0 {
			grantRevoked = true
			return model.RevokeConnectedAppGrant(tx, app.Id, userId, now)
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if revokedBinding != nil && revokedTokenId > 0 {
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventDeviceRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventTokenRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		if grantRevoked {
			enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventGrantRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		}
	}

	common.ApiSuccess(c, gin.H{
		"revoked":       revokedTokenId > 0,
		"token_id":      revokedTokenId,
		"grant_revoked": grantRevoked,
		"device":        device,
	})
}

func ListSnaplessDevices(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aliases := getSnaplessModelAliases()
	availability, err := snaplessModelAvailability(aliases)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelsOK := snaplessModelsReady(availability)
	userId := c.GetInt("id")

	var grant *model.ConnectedAppGrant
	if existingGrant, err := model.GetConnectedAppGrant(app.Id, userId); err == nil {
		grant = existingGrant
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	userHealth, err := getSnaplessUserHealth(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	bindings, err := model.ListConnectedAppTokenBindings(app.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	tokenIDs := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		if binding.TokenId > 0 {
			tokenIDs = append(tokenIDs, binding.TokenId)
		}
	}
	tokensByID := map[int]*model.Token{}
	if len(tokenIDs) > 0 {
		var tokens []model.Token
		if err := model.DB.Where("user_id = ? AND id IN ?", userId, tokenIDs).Find(&tokens).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		for i := range tokens {
			tokensByID[tokens[i].Id] = &tokens[i]
		}
	}

	now := common.GetTimestamp()
	devices := make([]snaplessManagedDevice, 0, len(bindings))
	for i := range bindings {
		binding := &bindings[i]
		devices = append(devices, buildSnaplessManagedDevice(app, grant, binding, tokensByID[binding.TokenId], userHealth, modelsOK, now))
	}

	globalStatus := evaluateSnaplessReadiness(app, userHealth, modelsOK)
	common.ApiSuccess(c, snaplessDevicesResponse{
		OK:      globalStatus.OK,
		Status:  globalStatus.Status,
		Actions: buildSnaplessActionHints(globalStatus.Status),
		App:     buildSnaplessAppResponse(app),
		Grant:   buildSnaplessGrantResponse(grant, app.DefaultScopeList()),
		Devices: devices,
		Models:  buildSnaplessModelHealth(aliases, availability),
		Aliases: aliases,
		BaseURL: snaplessAPIBaseURL(c),
		Checks:  globalStatus.Checks,
	})
}

func RotateSnaplessDevice(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	fingerprint := snaplessFingerprintParam(c)
	if fingerprint == "" {
		common.ApiError(c, errors.New("设备指纹不能为空"))
		return
	}

	var response snaplessTokenResponse
	var tokenId int
	var previousBinding *model.ConnectedAppTokenBinding
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		binding, err := findSnaplessBindingTx(tx, app.Id, userId, fingerprint)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("设备不存在或已撤销")
			}
			return err
		}
		bindingCopy := *binding
		previousBinding = &bindingCopy
		device := snaplessDeviceInfoFromBinding(binding)
		response, tokenId, err = ensureSnaplessTokenForDeviceTx(c, tx, app, userId, device, true)
		return err
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if tokenId > 0 {
		_ = model.TouchConnectedAppUsage(response.App.ID, userId, tokenId, common.GetTimestamp())
	}
	if response.Rotated && previousBinding != nil {
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventTokenRotated, userId, *previousBinding, previousBinding.TokenId, tokenId, common.GetTimestamp())
	}
	common.ApiSuccess(c, response)
}

func RevokeSnaplessDevice(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	fingerprint := snaplessFingerprintParam(c)
	if fingerprint == "" {
		common.ApiError(c, errors.New("设备指纹不能为空"))
		return
	}

	now := common.GetTimestamp()
	var revokedTokenId int
	var grantRevoked bool
	var device snaplessDeviceInfo
	var revokedBinding *model.ConnectedAppTokenBinding
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		binding, err := findSnaplessBindingTx(tx, app.Id, userId, fingerprint)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("设备不存在或已撤销")
			}
			return err
		}
		device = snaplessDeviceInfoFromBinding(binding)
		bindingCopy := *binding
		revokedBinding = &bindingCopy
		revokedTokenId = binding.TokenId
		if err := model.DisableTokenWithTx(tx, binding.TokenId, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := model.RevokeConnectedAppTokenBinding(tx, binding, now); err != nil {
			return err
		}
		count, err := model.CountActiveConnectedAppTokenBindings(tx, app.Id, userId)
		if err != nil {
			return err
		}
		if count == 0 {
			grantRevoked = true
			return model.RevokeConnectedAppGrant(tx, app.Id, userId, now)
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if revokedBinding != nil && revokedTokenId > 0 {
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventDeviceRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventTokenRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		if grantRevoked {
			enqueueSnaplessTokenLifecycleNotification(app, model.ConnectedAppNotificationEventGrantRevoked, userId, *revokedBinding, revokedTokenId, 0, now)
		}
	}

	common.ApiSuccess(c, gin.H{
		"revoked":       revokedTokenId > 0,
		"token_id":      revokedTokenId,
		"grant_revoked": grantRevoked,
		"device":        device,
	})
}

func GetSnaplessHealth(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aliases := getSnaplessModelAliases()
	availability, err := snaplessModelAvailability(aliases)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response := snaplessHealthResponse{
		OK:      false,
		Status:  "missing_token",
		App:     buildSnaplessAppResponse(app),
		Aliases: aliases,
		BaseURL: snaplessAPIBaseURL(c),
		Models:  buildSnaplessModelHealth(aliases, availability),
		Checks:  map[string]bool{},
	}

	key := snaplessBearerKey(c)
	if key == "" {
		writeSnaplessHealth(c, response)
		return
	}
	token, err := model.GetTokenByKey(key, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Status = "token_not_found"
			writeSnaplessHealth(c, response)
			return
		}
		common.ApiError(c, err)
		return
	}

	now := common.GetTimestamp()
	response.Token = buildSnaplessTokenHealth(token, now)

	binding, err := model.GetConnectedAppTokenBindingByTokenId(token.Id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Status = "not_snapless_token"
			writeSnaplessHealth(c, response)
			return
		}
		common.ApiError(c, err)
		return
	}
	response.Token.SnaplessBinding = binding.AppId == app.Id
	response.Token.BindingStatus = binding.Status
	response.Device = snaplessDeviceInfo{
		Fingerprint: binding.DeviceFingerprint,
		DeviceName:  binding.DeviceName,
		Platform:    binding.Platform,
		AppVersion:  binding.AppVersion,
	}
	if binding.AppId != app.Id {
		response.Status = "not_snapless_token"
		writeSnaplessHealth(c, response)
		return
	}

	grant, err := model.GetConnectedAppGrant(app.Id, token.UserId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
			return
		}
	} else {
		response.Grant = buildSnaplessGrantResponse(grant, app.DefaultScopeList())
	}

	response.User, err = getSnaplessUserHealth(token.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	status := evaluateSnaplessStatus(app, grant, binding, token, response.User, snaplessModelsReady(availability), now)
	response.OK = status.OK
	response.Status = status.Status
	response.Checks = status.Checks
	if status.OK {
		response.OK = true
		_ = model.TouchConnectedAppUsage(app.Id, token.UserId, token.Id, now)
	}
	writeSnaplessHealth(c, response)
}

func ensureSnaplessTokenForDevice(c *gin.Context, req snaplessDeviceRequest, rotate bool) (snaplessTokenResponse, int, error) {
	app, err := ensureSnaplessApp()
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return snaplessTokenResponse{}, 0, errors.New("Snapless 应用已停用")
	}
	userId := c.GetInt("id")
	device := snaplessDeviceFromRequest(c, req)

	var response snaplessTokenResponse
	var responseTokenId int
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		response, responseTokenId, err = ensureSnaplessTokenForDeviceTx(c, tx, app, userId, device, rotate)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	return response, responseTokenId, nil
}

func ensureSnaplessTokenForDeviceTx(c *gin.Context, tx *gorm.DB, app *model.ConnectedApp, userId int, device snaplessDeviceInfo, rotate bool) (snaplessTokenResponse, int, error) {
	if app == nil {
		return snaplessTokenResponse{}, 0, errors.New("Snapless 应用不存在")
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return snaplessTokenResponse{}, 0, errors.New("Snapless 应用已停用")
	}
	if tx == nil {
		tx = model.DB
	}
	aliases := getSnaplessModelAliases()
	modelLimits := strings.Join(aliases.All(), ",")
	now := common.GetTimestamp()

	grant, err := model.UpsertConnectedAppGrant(tx, *app, userId, app.DefaultScopeList(), now)
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	var existingBinding *model.ConnectedAppTokenBinding
	if binding, err := findSnaplessBindingTx(tx, app.Id, userId, device.Fingerprint); err == nil {
		existingBinding = binding
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return snaplessTokenResponse{}, 0, err
	}

	if existingBinding != nil && !rotate {
		if token, err := getTokenByIdTx(tx, existingBinding.TokenId, userId); err == nil && snaplessTokenReusable(token, now) {
			if err := syncSnaplessTokenModelLimits(tx, token, modelLimits); err != nil {
				return snaplessTokenResponse{}, 0, err
			}
			return buildSnaplessTokenResponse(c, app, grant, existingBinding, token, aliases, device, "", false, false), token.Id, nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return snaplessTokenResponse{}, 0, err
		}
	}

	if existingBinding != nil {
		if err := model.RecordConnectedAppTokenAttribution(tx, *existingBinding, now); err != nil {
			return snaplessTokenResponse{}, 0, err
		}
		if err := model.DisableTokenWithTx(tx, existingBinding.TokenId, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return snaplessTokenResponse{}, 0, err
		}
	}
	token, key, err := createSnaplessToken(tx, userId, device, modelLimits, now)
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	binding, err := model.UpsertConnectedAppTokenBinding(tx, model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            userId,
		TokenId:           token.Id,
		DeviceFingerprint: device.Fingerprint,
		DeviceName:        device.DeviceName,
		Platform:          device.Platform,
		AppVersion:        device.AppVersion,
	}, now)
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	if err := model.RecordConnectedAppTokenAttribution(tx, *binding, now); err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	response := buildSnaplessTokenResponse(c, app, grant, binding, token, aliases, device, key, true, rotate && existingBinding != nil)
	return response, token.Id, nil
}

func ensureSnaplessApp() (*model.ConnectedApp, error) {
	if err := model.EnsureBuiltinConnectedApps(); err != nil {
		return nil, err
	}
	return model.GetConnectedAppBySlug(model.ConnectedAppSlugSnapless)
}

func bindSnaplessDeviceRequest(c *gin.Context) (snaplessDeviceRequest, error) {
	var req snaplessDeviceRequest
	if c.Request.ContentLength != 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			return req, err
		}
	}
	return req, nil
}

func snaplessDeviceFromRequest(c *gin.Context, req snaplessDeviceRequest) snaplessDeviceInfo {
	req.DeviceID = firstNonEmpty(req.DeviceID, c.Query("device_id"), c.GetHeader("X-Snapless-Device-Id"))
	req.DeviceName = firstNonEmpty(req.DeviceName, c.Query("device_name"), c.GetHeader("X-Snapless-Device-Name"), "Snapless Desktop")
	req.Platform = firstNonEmpty(req.Platform, c.Query("platform"), c.GetHeader("X-Snapless-Platform"), "desktop")
	req.AppVersion = firstNonEmpty(req.AppVersion, c.Query("app_version"), c.GetHeader("X-Snapless-App-Version"))
	req.Client = firstNonEmpty(req.Client, c.Query("client"), c.GetHeader("X-Snapless-Client"), c.GetHeader("User-Agent"), "snapless")

	deviceName := limitString(req.DeviceName, 128)
	platform := limitString(strings.ToLower(req.Platform), 32)
	appVersion := limitString(req.AppVersion, 64)
	client := limitString(req.Client, 64)

	seed := strings.TrimSpace(req.DeviceID)
	if seed != "" {
		seed = "device_id:" + strings.ToLower(seed)
	} else {
		seed = "derived:" + strings.ToLower(strings.Join([]string{deviceName, platform, client, c.GetHeader("User-Agent")}, "|"))
	}
	return snaplessDeviceInfo{
		Fingerprint: stableSnaplessFingerprint(seed),
		DeviceName:  deviceName,
		Platform:    platform,
		AppVersion:  appVersion,
		Client:      client,
	}
}

func getSnaplessModelAliases() snaplessModelAliases {
	aliases := defaultSnaplessModelAliases()
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap["SnaplessModels"])
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return aliases
	}
	var configured snaplessModelAliases
	if err := common.Unmarshal([]byte(raw), &configured); err == nil {
		configured.fillDefaults()
		return configured
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		aliases.ASR = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		aliases.Chat = strings.TrimSpace(parts[1])
		aliases.Polish = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		aliases.Translate = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
		aliases.QA = strings.TrimSpace(parts[3])
	}
	aliases.fillDefaults()
	return aliases
}

func defaultSnaplessModelAliases() snaplessModelAliases {
	return snaplessModelAliases{
		ASR:       "snapless-asr",
		Chat:      "snapless-polish",
		Polish:    "snapless-polish",
		Translate: "snapless-translate",
		QA:        "snapless-qa",
	}
}

func (aliases *snaplessModelAliases) fillDefaults() {
	defaults := defaultSnaplessModelAliases()
	if aliases.ASR == "" {
		aliases.ASR = defaults.ASR
	}
	if aliases.Chat == "" {
		aliases.Chat = defaults.Chat
	}
	if aliases.Polish == "" {
		aliases.Polish = aliases.Chat
	}
	if aliases.Translate == "" {
		aliases.Translate = defaults.Translate
	}
	if aliases.QA == "" {
		aliases.QA = defaults.QA
	}
}

func (aliases snaplessModelAliases) All() []string {
	values := []string{aliases.ASR, aliases.Chat, aliases.Polish, aliases.Translate, aliases.QA}
	seen := map[string]struct{}{}
	models := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		models = append(models, value)
	}
	return models
}

func createSnaplessToken(tx *gorm.DB, userId int, device snaplessDeviceInfo, modelLimits string, now int64) (*model.Token, string, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, "", err
	}
	token := model.Token{
		UserId:                userId,
		Name:                  snaplessTokenName(device),
		Key:                   key,
		Status:                common.TokenStatusEnabled,
		CreatedTime:           now,
		AccessedTime:          now,
		ExpiredTime:           -1,
		RemainQuota:           0,
		UnlimitedQuota:        true,
		QuotaHardLimitEnabled: false,
		ModelLimitsEnabled:    true,
		ModelLimits:           modelLimits,
	}
	if err := tx.Create(&token).Error; err != nil {
		return nil, "", err
	}
	return &token, key, nil
}

func findSnaplessBindingTx(tx *gorm.DB, appId int, userId int, deviceFingerprint string) (*model.ConnectedAppTokenBinding, error) {
	var binding model.ConnectedAppTokenBinding
	err := tx.Where(
		"app_id = ? AND user_id = ? AND device_fingerprint = ? AND status = ?",
		appId,
		userId,
		deviceFingerprint,
		model.ConnectedAppTokenBindingStatusActive,
	).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func enqueueSnaplessTokenLifecycleNotification(app *model.ConnectedApp, eventType string, userId int, binding model.ConnectedAppTokenBinding, previousTokenId int, newTokenId int, occurredAt int64) {
	if app == nil || binding.Id <= 0 || userId <= 0 {
		return
	}
	tokenId := newTokenId
	if tokenId == 0 {
		tokenId = previousTokenId
	}
	input := service.ConnectedAppTokenLifecycleNotificationInput{
		EventType:         eventType,
		App:               *app,
		UserId:            userId,
		GrantId:           binding.GrantId,
		BindingId:         binding.Id,
		TokenId:           tokenId,
		PreviousTokenId:   previousTokenId,
		NewTokenId:        newTokenId,
		DeviceFingerprint: binding.DeviceFingerprint,
		DeviceName:        binding.DeviceName,
		Platform:          binding.Platform,
		AppVersion:        binding.AppVersion,
		OccurredAt:        occurredAt,
	}
	if err := service.EnqueueConnectedAppTokenLifecycleOutboxWithDB(model.DB, input); err != nil {
		common.SysLog("failed to enqueue snapless token lifecycle notification outbox: " + err.Error())
	}
}

func snaplessFingerprintParam(c *gin.Context) string {
	return strings.ToLower(strings.TrimSpace(c.Param("fingerprint")))
}

func getTokenByIdTx(tx *gorm.DB, tokenId int, userId int) (*model.Token, error) {
	var token model.Token
	if err := tx.Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func getGrantTx(tx *gorm.DB, appId int, userId int) (*model.ConnectedAppGrant, error) {
	var grant model.ConnectedAppGrant
	if err := tx.Where("app_id = ? AND user_id = ?", appId, userId).First(&grant).Error; err != nil {
		return nil, err
	}
	return &grant, nil
}

func getBindingByTokenIdTx(tx *gorm.DB, tokenId int) (*model.ConnectedAppTokenBinding, error) {
	var binding model.ConnectedAppTokenBinding
	if err := tx.Where("token_id = ?", tokenId).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func syncSnaplessTokenModelLimits(tx *gorm.DB, token *model.Token, modelLimits string) error {
	if token == nil || (token.ModelLimits == modelLimits && token.ModelLimitsEnabled) {
		return nil
	}
	if err := tx.Model(token).Select("model_limits_enabled", "model_limits").Updates(map[string]any{
		"model_limits_enabled": true,
		"model_limits":         modelLimits,
	}).Error; err != nil {
		return err
	}
	_ = model.InvalidateTokenCacheByKey(token.Key)
	token.ModelLimitsEnabled = true
	token.ModelLimits = modelLimits
	return nil
}

func snaplessTokenReusable(token *model.Token, now int64) bool {
	if token == nil {
		return false
	}
	if token.Status != common.TokenStatusEnabled {
		return false
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < now {
		return false
	}
	if token.IsQuotaLimited() && token.RemainQuota <= 0 {
		return false
	}
	return true
}

func snaplessModelAvailability(aliases snaplessModelAliases) (map[string]bool, error) {
	modelNames := aliases.All()
	availability := make(map[string]bool, len(modelNames))
	for _, name := range modelNames {
		availability[name] = false
	}
	if len(modelNames) == 0 {
		return availability, nil
	}
	var available []string
	err := model.DB.Table("abilities").
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Where("abilities.enabled = ? AND channels.status = ? AND abilities.model IN ?", true, common.ChannelStatusEnabled, modelNames).
		Distinct("abilities.model").
		Pluck("abilities.model", &available).Error
	if err != nil {
		return nil, err
	}
	for _, name := range available {
		availability[name] = true
	}
	return availability, nil
}

func buildSnaplessTokenResponse(c *gin.Context, app *model.ConnectedApp, grant *model.ConnectedAppGrant, binding *model.ConnectedAppTokenBinding, token *model.Token, aliases snaplessModelAliases, device snaplessDeviceInfo, key string, created bool, rotated bool) snaplessTokenResponse {
	return snaplessTokenResponse{
		App:        buildSnaplessAppResponse(app),
		Grant:      buildSnaplessGrantResponse(grant, app.DefaultScopeList()),
		Device:     device,
		Token:      buildSnaplessTokenSummary(token, binding),
		Models:     aliases,
		Endpoints:  snaplessEndpoints(c),
		BaseURL:    snaplessAPIBaseURL(c),
		APIKey:     snaplessAPIKey(key),
		Created:    created,
		Rotated:    rotated,
		APIKeyOnce: key != "",
		Instructions: snaplessClientInstruction{
			Authorization: "Bearer sk-<api_key>",
		},
	}
}

func buildSnaplessAppResponse(app *model.ConnectedApp) snaplessAppResponse {
	if app == nil {
		return snaplessAppResponse{}
	}
	return snaplessAppResponse{
		ID:            app.Id,
		Slug:          app.Slug,
		Name:          app.Name,
		Trusted:       app.Trusted,
		Status:        app.Status,
		AllowedScopes: app.ScopeList(),
		DefaultScopes: app.DefaultScopeList(),
	}
}

func buildSnaplessGrantResponse(grant *model.ConnectedAppGrant, defaultScopes []string) snaplessGrantResponse {
	if grant == nil {
		return snaplessGrantResponse{
			Status: model.ConnectedAppGrantStatusRevoked,
			Scopes: defaultScopes,
		}
	}
	return snaplessGrantResponse{
		ID:           grant.Id,
		Status:       grant.Status,
		Scopes:       grant.ScopeList(),
		AuthorizedAt: grant.AuthorizedAt,
		LastUsedAt:   grant.LastUsedAt,
		RevokedAt:    grant.RevokedAt,
	}
}

func buildSnaplessTokenSummary(token *model.Token, binding *model.ConnectedAppTokenBinding) snaplessTokenSummary {
	if token == nil {
		return snaplessTokenSummary{}
	}
	summary := snaplessTokenSummary{
		ID:                    token.Id,
		Name:                  token.Name,
		Status:                token.Status,
		MaskedKey:             "sk-" + token.GetMaskedKey(),
		ExpiredTime:           token.ExpiredTime,
		UnlimitedQuota:        token.UnlimitedQuota,
		QuotaHardLimitEnabled: token.QuotaHardLimitEnabled,
		ModelLimitsEnabled:    token.ModelLimitsEnabled,
		ModelLimits:           token.ModelLimits,
	}
	if binding != nil {
		summary.BindingStatus = binding.Status
		summary.LastUsedAt = binding.LastUsedAt
	}
	return summary
}

func buildSnaplessTokenHealth(token *model.Token, now int64) snaplessTokenHealth {
	health := snaplessTokenHealth{}
	if token == nil {
		return health
	}
	health.ID = token.Id
	health.Status = token.Status
	health.Enabled = token.Status == common.TokenStatusEnabled
	health.Expired = token.ExpiredTime != -1 && token.ExpiredTime < now
	health.QuotaOK = !token.IsQuotaLimited() || token.RemainQuota > 0
	return health
}

func getSnaplessUserHealth(userId int) (snaplessUserHealth, error) {
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return snaplessUserHealth{}, err
	}
	return snaplessUserHealth{
		ID:      userId,
		Enabled: userCache.Status == common.UserStatusEnabled,
		Quota:   userCache.Quota,
		QuotaOK: userCache.Quota > 0,
	}, nil
}

func snaplessReadinessForUser(app *model.ConnectedApp, userId int) (snaplessReadinessResponse, error) {
	aliases := getSnaplessModelAliases()
	availability, err := snaplessModelAvailability(aliases)
	if err != nil {
		return snaplessReadinessResponse{}, err
	}
	userHealth, err := getSnaplessUserHealth(userId)
	if err != nil {
		return snaplessReadinessResponse{}, err
	}
	return buildSnaplessReadinessResponse(evaluateSnaplessReadiness(app, userHealth, snaplessModelsReady(availability))), nil
}

func buildSnaplessManagedDevice(app *model.ConnectedApp, grant *model.ConnectedAppGrant, binding *model.ConnectedAppTokenBinding, token *model.Token, user snaplessUserHealth, modelsOK bool, now int64) snaplessManagedDevice {
	status := evaluateSnaplessStatus(app, grant, binding, token, user, modelsOK, now)
	return snaplessManagedDevice{
		OK:         status.OK,
		Status:     status.Status,
		Actions:    buildSnaplessActionHints(status.Status),
		Device:     snaplessDeviceInfoFromBinding(binding),
		Token:      buildSnaplessTokenSummary(token, binding),
		Checks:     status.Checks,
		LastUsedAt: binding.LastUsedAt,
		RevokedAt:  binding.RevokedAt,
		CreatedAt:  binding.CreatedAt,
		UpdatedAt:  binding.UpdatedAt,
	}
}

func evaluateSnaplessReadiness(app *model.ConnectedApp, user snaplessUserHealth, modelsOK bool) snaplessStatusEvaluation {
	appOK := app != nil && app.Status == model.ConnectedAppStatusEnabled
	checks := map[string]bool{
		"app_enabled":   appOK,
		"user_enabled":  user.Enabled,
		"user_quota_ok": user.QuotaOK,
		"models_ready":  modelsOK,
	}

	switch {
	case !appOK:
		return snaplessStatusEvaluation{Status: "app_disabled", Checks: checks}
	case !user.Enabled:
		return snaplessStatusEvaluation{Status: "user_disabled", Checks: checks}
	case !user.QuotaOK:
		return snaplessStatusEvaluation{Status: "quota_insufficient", Checks: checks}
	case !modelsOK:
		return snaplessStatusEvaluation{Status: "models_unavailable", Checks: checks}
	default:
		return snaplessStatusEvaluation{OK: true, Status: "ok", Checks: checks}
	}
}

func buildSnaplessReadinessResponse(status snaplessStatusEvaluation) snaplessReadinessResponse {
	return snaplessReadinessResponse{
		OK:      status.OK,
		Status:  status.Status,
		Checks:  status.Checks,
		Actions: buildSnaplessActionHints(status.Status),
	}
}

func evaluateSnaplessStatus(app *model.ConnectedApp, grant *model.ConnectedAppGrant, binding *model.ConnectedAppTokenBinding, token *model.Token, user snaplessUserHealth, modelsOK bool, now int64) snaplessStatusEvaluation {
	tokenHealth := buildSnaplessTokenHealth(token, now)
	appOK := app != nil && app.Status == model.ConnectedAppStatusEnabled
	grantOK := grant != nil && grant.Status == model.ConnectedAppGrantStatusAuthorized
	bindingOK := binding != nil && binding.Status == model.ConnectedAppTokenBindingStatusActive
	checks := map[string]bool{
		"app_enabled":    appOK,
		"grant_active":   grantOK,
		"binding_active": bindingOK,
		"token_enabled":  tokenHealth.Enabled,
		"token_quota_ok": tokenHealth.QuotaOK,
		"user_enabled":   user.Enabled,
		"user_quota_ok":  user.QuotaOK,
		"models_ready":   modelsOK,
	}

	switch {
	case !appOK:
		return snaplessStatusEvaluation{Status: "app_disabled", Checks: checks}
	case token == nil:
		return snaplessStatusEvaluation{Status: "token_not_found", Checks: checks}
	case !tokenHealth.Enabled:
		return snaplessStatusEvaluation{Status: "token_disabled", Checks: checks}
	case tokenHealth.Expired:
		return snaplessStatusEvaluation{Status: "token_expired", Checks: checks}
	case !tokenHealth.QuotaOK:
		return snaplessStatusEvaluation{Status: "token_quota_insufficient", Checks: checks}
	case !user.Enabled:
		return snaplessStatusEvaluation{Status: "user_disabled", Checks: checks}
	case !grantOK:
		return snaplessStatusEvaluation{Status: "grant_revoked", Checks: checks}
	case !bindingOK:
		return snaplessStatusEvaluation{Status: "binding_revoked", Checks: checks}
	case !user.QuotaOK:
		return snaplessStatusEvaluation{Status: "quota_insufficient", Checks: checks}
	case !modelsOK:
		return snaplessStatusEvaluation{Status: "models_unavailable", Checks: checks}
	default:
		return snaplessStatusEvaluation{OK: true, Status: "ok", Checks: checks}
	}
}

func buildSnaplessActionHints(status string) snaplessActionHints {
	switch status {
	case "ok":
		return snaplessActionHints{Severity: "success"}
	case "quota_insufficient":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "Your account balance is too low for Snapless requests.",
			Primary: &snaplessActionLink{
				Label:  "Recharge",
				Href:   "/wallet?source=snapless",
				Intent: "recharge",
			},
		}
	case "user_disabled":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "Your account is disabled. Review the account before approving Snapless access.",
			Primary: &snaplessActionLink{
				Label:  "Account settings",
				Href:   "/profile",
				Intent: "account",
			},
		}
	case "models_unavailable":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "Required Snapless models are unavailable for this user group.",
			Primary: &snaplessActionLink{
				Label:  "Model settings",
				Href:   "/system-settings/models",
				Intent: "model_settings",
			},
			Secondary: &snaplessActionLink{
				Label:  "Model catalog",
				Href:   "/models",
				Intent: "models",
			},
		}
	case "app_disabled":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "The built-in Snapless connected app is disabled.",
			Primary: &snaplessActionLink{
				Label:  "Open profile",
				Href:   "/profile",
				Intent: "connected_app",
			},
		}
	case "token_disabled":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "This device token is disabled. Rotate the device key or authorize the device again.",
		}
	case "token_expired":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "This device token is expired. Rotate the device key or authorize the device again.",
		}
	case "token_quota_insufficient":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "This device token has no remaining token quota. Rotate the key or remove the token quota limit.",
		}
	case "grant_revoked":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "The Snapless grant is revoked. Authorize the device again to create a fresh grant.",
		}
	case "binding_revoked":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "This device was revoked. Start a new Snapless authorization flow from the desktop app.",
		}
	case "token_not_found", "missing_token":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "No active Snapless token was found for this device. Refresh authorization from Snapless Desktop.",
		}
	case "not_snapless_token":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "The supplied token is not managed by the Snapless connected app.",
		}
	default:
		return snaplessActionHints{Severity: "neutral"}
	}
}

func writeSnaplessHealth(c *gin.Context, response snaplessHealthResponse) {
	response.Actions = buildSnaplessActionHints(response.Status)
	common.ApiSuccess(c, response)
}

func buildSnaplessModelHealth(aliases snaplessModelAliases, availability map[string]bool) map[string]snaplessModelHealth {
	return map[string]snaplessModelHealth{
		"asr": {
			Model:     aliases.ASR,
			Available: availability[aliases.ASR],
		},
		"chat": {
			Model:     aliases.Chat,
			Available: availability[aliases.Chat],
		},
		"polish": {
			Model:     aliases.Polish,
			Available: availability[aliases.Polish],
		},
		"translate": {
			Model:     aliases.Translate,
			Available: availability[aliases.Translate],
		},
		"qa": {
			Model:     aliases.QA,
			Available: availability[aliases.QA],
		},
	}
}

func snaplessModelsReady(availability map[string]bool) bool {
	for _, available := range availability {
		if !available {
			return false
		}
	}
	return true
}

func snaplessUserCode() (string, error) {
	raw, err := common.GenerateRandomCharsKey(8)
	if err != nil {
		return "", err
	}
	raw = strings.ToUpper(raw)
	return raw[:4] + "-" + raw[4:], nil
}

func normalizeSnaplessUserCode(userCode string) string {
	code := strings.ToUpper(strings.TrimSpace(userCode))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 8 && !strings.Contains(code, "-") {
		return code[:4] + "-" + code[4:]
	}
	return code
}

func snaplessVerificationURI(c *gin.Context, userCode string) string {
	base := snaplessServerBaseURL(c)
	values := url.Values{}
	values.Set("user_code", normalizeSnaplessUserCode(userCode))
	return base + "/connect/device?" + values.Encode()
}

func snaplessServerBaseURL(c *gin.Context) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" && c != nil && c.Request != nil {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		if c.Request.Host != "" {
			base = scheme + "://" + c.Request.Host
		}
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	return strings.TrimRight(base, "/")
}

func snaplessEndpoints(c *gin.Context) map[string]string {
	baseURL := snaplessAPIBaseURL(c)
	return map[string]string{
		"models":               baseURL + "/models",
		"chat_completions":     baseURL + "/chat/completions",
		"audio_transcriptions": baseURL + "/audio/transcriptions",
	}
}

func snaplessAPIBaseURL(c *gin.Context) string {
	return snaplessServerBaseURL(c) + "/v1"
}

func snaplessBearerKey(c *gin.Context) string {
	key := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	key = strings.TrimPrefix(key, "sk-")
	parts := strings.Split(key, "-")
	return strings.TrimSpace(parts[0])
}

func snaplessAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func snaplessTokenName(device snaplessDeviceInfo) string {
	name := "Snapless Desktop"
	if device.DeviceName != "" && device.DeviceName != name {
		name += " - " + device.DeviceName
	}
	return limitString(name, 50)
}

func stableSnaplessFingerprint(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func snaplessDeviceInfoFromSession(session *model.ConnectedAppDeviceSession) snaplessDeviceInfo {
	if session == nil {
		return snaplessDeviceInfo{}
	}
	return snaplessDeviceInfo{
		Fingerprint: session.DeviceFingerprint,
		DeviceName:  session.DeviceName,
		Platform:    session.Platform,
		AppVersion:  session.AppVersion,
		Client:      session.Client,
	}
}

func snaplessDeviceInfoFromBinding(binding *model.ConnectedAppTokenBinding) snaplessDeviceInfo {
	if binding == nil {
		return snaplessDeviceInfo{}
	}
	return snaplessDeviceInfo{
		Fingerprint: binding.DeviceFingerprint,
		DeviceName:  binding.DeviceName,
		Platform:    binding.Platform,
		AppVersion:  binding.AppVersion,
	}
}

func snaplessDeviceFlowStatusError(status string) error {
	switch status {
	case model.ConnectedAppDeviceSessionStatusAuthorized:
		return errors.New("设备授权码已完成授权")
	case model.ConnectedAppDeviceSessionStatusConsumed:
		return errors.New("设备授权码已被使用")
	case model.ConnectedAppDeviceSessionStatusExpired:
		return errors.New("设备授权码已过期")
	case model.ConnectedAppDeviceSessionStatusDenied:
		return errors.New("设备授权已拒绝")
	default:
		return errors.New("设备授权码状态不可用")
	}
}

func clauseLockingUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func limitString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
