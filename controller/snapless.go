package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

type snaplessTokenResponse struct {
	App          snaplessAppResponse       `json:"app"`
	Grant        snaplessGrantResponse     `json:"grant"`
	Device       snaplessDeviceInfo        `json:"device"`
	Token        snaplessTokenSummary      `json:"token"`
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

func GetSnaplessConfig(c *gin.Context) {
	app, err := ensureSnaplessApp()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aliases := getSnaplessModelAliases()
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
	if binding, err := model.FindActiveConnectedAppTokenBinding(app.Id, userId, device.Fingerprint); err == nil {
		if token, tokenErr := model.GetTokenByIds(binding.TokenId, userId); tokenErr == nil {
			tokenSummary = buildSnaplessTokenSummary(token, binding)
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"app":          buildSnaplessAppResponse(app),
		"grant":        buildSnaplessGrantResponse(grant, app.DefaultScopeList()),
		"device":       device,
		"token":        tokenSummary,
		"models":       aliases,
		"model_limits": aliases.All(),
		"base_url":     snaplessAPIBaseURL(c),
		"endpoints":    snaplessEndpoints(c),
	})
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
	response, tokenId, err := ensureSnaplessTokenForDevice(c, req, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if tokenId > 0 {
		_ = model.TouchConnectedAppUsage(response.App.ID, c.GetInt("id"), tokenId, common.GetTimestamp())
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
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		binding, err := findSnaplessBindingTx(tx, app.Id, userId, device.Fingerprint)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
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
		common.ApiSuccess(c, response)
		return
	}
	token, err := model.GetTokenByKey(key, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Status = "token_not_found"
			common.ApiSuccess(c, response)
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
			common.ApiSuccess(c, response)
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
		common.ApiSuccess(c, response)
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

	userCache, err := model.GetUserCache(token.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response.User = snaplessUserHealth{
		ID:      token.UserId,
		Enabled: userCache.Status == common.UserStatusEnabled,
		Quota:   userCache.Quota,
		QuotaOK: userCache.Quota > 0,
	}

	modelsOK := true
	for _, item := range response.Models {
		if !item.Available {
			modelsOK = false
			break
		}
	}
	grantOK := grant != nil && grant.Status == model.ConnectedAppGrantStatusAuthorized
	appOK := app.Status == model.ConnectedAppStatusEnabled
	bindingOK := binding.Status == model.ConnectedAppTokenBindingStatusActive
	response.Checks = map[string]bool{
		"app_enabled":    appOK,
		"grant_active":   grantOK,
		"binding_active": bindingOK,
		"token_enabled":  response.Token.Enabled,
		"token_quota_ok": response.Token.QuotaOK,
		"user_enabled":   response.User.Enabled,
		"user_quota_ok":  response.User.QuotaOK,
		"models_ready":   modelsOK,
	}

	switch {
	case !appOK:
		response.Status = "app_disabled"
	case !response.Token.Enabled:
		response.Status = "token_disabled"
	case response.Token.Expired:
		response.Status = "token_expired"
	case !response.Token.QuotaOK:
		response.Status = "token_quota_insufficient"
	case !response.User.Enabled:
		response.Status = "user_disabled"
	case !grantOK:
		response.Status = "grant_revoked"
	case !bindingOK:
		response.Status = "binding_revoked"
	case !response.User.QuotaOK:
		response.Status = "quota_insufficient"
	case !modelsOK:
		response.Status = "models_unavailable"
	default:
		response.OK = true
		response.Status = "ok"
		_ = model.TouchConnectedAppUsage(app.Id, token.UserId, token.Id, now)
	}
	common.ApiSuccess(c, response)
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
	aliases := getSnaplessModelAliases()
	modelLimits := strings.Join(aliases.All(), ",")
	now := common.GetTimestamp()

	var response snaplessTokenResponse
	var responseTokenId int
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		grant, err := model.UpsertConnectedAppGrant(tx, *app, userId, app.DefaultScopeList(), now)
		if err != nil {
			return err
		}

		var existingBinding *model.ConnectedAppTokenBinding
		if binding, err := findSnaplessBindingTx(tx, app.Id, userId, device.Fingerprint); err == nil {
			existingBinding = binding
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if existingBinding != nil && !rotate {
			if token, err := getTokenByIdTx(tx, existingBinding.TokenId, userId); err == nil && snaplessTokenReusable(token, now) {
				if err := syncSnaplessTokenModelLimits(tx, token, modelLimits); err != nil {
					return err
				}
				responseTokenId = token.Id
				response = buildSnaplessTokenResponse(c, app, grant, existingBinding, token, aliases, device, "", false, false)
				return nil
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		if existingBinding != nil {
			if err := model.DisableTokenWithTx(tx, existingBinding.TokenId, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		token, key, err := createSnaplessToken(tx, userId, device, modelLimits, now)
		if err != nil {
			return err
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
			return err
		}
		responseTokenId = token.Id
		response = buildSnaplessTokenResponse(c, app, grant, binding, token, aliases, device, key, true, rotate && existingBinding != nil)
		return nil
	})
	if err != nil {
		return snaplessTokenResponse{}, 0, err
	}
	return response, responseTokenId, nil
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

func getTokenByIdTx(tx *gorm.DB, tokenId int, userId int) (*model.Token, error) {
	var token model.Token
	if err := tx.Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
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

func snaplessEndpoints(c *gin.Context) map[string]string {
	baseURL := snaplessAPIBaseURL(c)
	return map[string]string{
		"models":               baseURL + "/models",
		"chat_completions":     baseURL + "/chat/completions",
		"audio_transcriptions": baseURL + "/audio/transcriptions",
	}
}

func snaplessAPIBaseURL(c *gin.Context) string {
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
	return strings.TrimRight(base, "/") + "/v1"
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
