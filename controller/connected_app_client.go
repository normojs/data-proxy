package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	connectedAppScopeProfileRead      = "profile.read"
	connectedAppScopeGroupRead        = "group.read"
	connectedAppScopeTokenCreate      = "token.create"
	connectedAppScopeTokenRotateOwn   = "token.rotate.own"
	connectedAppScopeTokenRevokeOwn   = "token.revoke.own"
	connectedAppScopeTokenGroupUpdate = "token.group.update"
	connectedAppScopeOpenAIResponses  = "openai.responses"

	connectedAppManagementTokenTTLSeconds = 90 * 24 * 60 * 60
)

type connectedAppClientConfigResponse struct {
	ServerURL       string                         `json:"server_url"`
	BaseURL         string                         `json:"base_url"`
	App             snaplessAppResponse            `json:"app"`
	User            connectedAppClientUser         `json:"user"`
	SelectedToken   connectedAppClientTokenSummary `json:"selected_token,omitempty"`
	Capabilities    connectedAppClientCapabilities `json:"capabilities"`
	APIEndpoints    map[string]string              `json:"api_endpoints"`
	ClientEndpoints map[string]string              `json:"client_endpoints"`
}

type connectedAppClientDevicePollResponse struct {
	Status                   string                         `json:"status"`
	ManagementToken          string                         `json:"management_token"`
	ManagementTokenExpiresAt int64                          `json:"management_token_expires_at"`
	ServerURL                string                         `json:"server_url"`
	BaseURL                  string                         `json:"base_url"`
	App                      snaplessAppResponse            `json:"app"`
	User                     connectedAppClientUser         `json:"user"`
	Device                   snaplessDeviceInfo             `json:"device"`
	Capabilities             connectedAppClientCapabilities `json:"capabilities"`
	APIEndpoints             map[string]string              `json:"api_endpoints"`
	ClientEndpoints          map[string]string              `json:"client_endpoints"`
}

type connectedAppClientUser struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Group       string `json:"group,omitempty"`
}

type connectedAppClientCapabilities struct {
	Groups           bool `json:"groups"`
	DedicatedTokens  bool `json:"dedicated_tokens"`
	TokenRotate      bool `json:"token_rotate"`
	TokenRevoke      bool `json:"token_revoke"`
	TokenGroupUpdate bool `json:"token_group_update"`
	OpenAIModels     bool `json:"openai_models"`
	OpenAIResponses  bool `json:"openai_responses"`
	OpenAIChat       bool `json:"openai_chat"`
}

type connectedAppClientGroupItem struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description,omitempty"`
	Available         bool   `json:"available"`
	IsDefault         bool   `json:"is_default"`
	IsAuto            bool   `json:"is_auto"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

type connectedAppClientGroupsResponse struct {
	DefaultGroup string                        `json:"default_group"`
	Data         []connectedAppClientGroupItem `json:"data"`
}

type connectedAppClientTokenSummary struct {
	ID                     int    `json:"id,omitempty"`
	Name                   string `json:"name,omitempty"`
	MaskedKey              string `json:"masked_key,omitempty"`
	Status                 int    `json:"status,omitempty"`
	Group                  string `json:"group"`
	EffectiveGroup         string `json:"effective_group"`
	GroupAvailable         bool   `json:"group_available"`
	GroupUnavailableReason string `json:"group_unavailable_reason,omitempty"`
	OwnedByConnectedApp    bool   `json:"owned_by_connected_app"`
	ConnectedAppSlug       string `json:"connected_app_slug,omitempty"`
	DeviceID               string `json:"device_id,omitempty"`
	ExpiredAt              *int64 `json:"expired_at"`
	LastUsedAt             *int64 `json:"last_used_at"`
	UnlimitedQuota         bool   `json:"unlimited_quota"`
	QuotaHardLimitEnabled  bool   `json:"quota_hard_limit_enabled"`
	ModelLimitsEnabled     bool   `json:"model_limits_enabled"`
}

type connectedAppClientTokenResponse struct {
	Selected         bool                           `json:"selected,omitempty"`
	Created          bool                           `json:"created"`
	Rotated          bool                           `json:"rotated"`
	Revoked          bool                           `json:"revoked,omitempty"`
	APIKeyOnce       bool                           `json:"api_key_once"`
	APIKey           string                         `json:"api_key,omitempty"`
	BaseURL          string                         `json:"base_url"`
	Token            connectedAppClientTokenSummary `json:"token"`
	RequiresRotation bool                           `json:"requires_rotation,omitempty"`
	Message          string                         `json:"message,omitempty"`
}

type connectedAppClientEnsureTokenRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	Client     string `json:"client"`
	Group      string `json:"group"`
	Rotate     bool   `json:"rotate"`
}

type connectedAppClientUpdateGroupRequest struct {
	Group string `json:"group"`
}

type connectedAppClientRevokeDeviceRequest struct {
	DeviceID             string `json:"device_id"`
	RevokeDedicatedToken bool   `json:"revoke_dedicated_token"`
}

func GetConnectedAppClientConfig(c *gin.Context) {
	app, _, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	user, err := model.GetUserById(accessToken.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tokenSummary := connectedAppClientTokenSummary{}
	if binding, token, err := connectedAppClientActiveBindingToken(app.Id, accessToken.UserId, accessToken.DeviceFingerprint); err == nil {
		tokenSummary = buildConnectedAppClientTokenSummary(app, binding, token, user)
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, connectedAppClientConfigResponse{
		ServerURL:       snaplessServerBaseURL(c),
		BaseURL:         snaplessAPIBaseURL(c),
		App:             buildSnaplessAppResponse(app),
		User:            connectedAppClientUserFromModel(user),
		SelectedToken:   tokenSummary,
		Capabilities:    connectedAppClientCapabilitiesForScopes(accessToken.ScopeList()),
		APIEndpoints:    connectedAppAPIEndpointsForScopes(c, accessToken.ScopeList()),
		ClientEndpoints: connectedAppClientEndpoints(c, app),
	})
}

func ListConnectedAppClientGroups(c *gin.Context) {
	_, _, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	user, err := model.GetUserById(accessToken.UserId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	available := service.GetUserUsableGroupsWithBindings(user.Group, user.GetTokenGroups())
	items := make([]connectedAppClientGroupItem, 0)
	seen := map[string]struct{}{}
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		item := connectedAppClientGroupItem{
			ID:          groupName,
			Name:        groupName,
			DisplayName: groupName,
			Description: setting.GetUsableGroupDescription(groupName),
			Available:   false,
			IsDefault:   groupName == tokenEffectiveGroup("", user.Group),
			IsAuto:      groupName == "auto",
		}
		if availableDesc, ok := available[groupName]; ok {
			item.Available = true
			if strings.TrimSpace(availableDesc) != "" {
				item.Description = availableDesc
			}
		} else {
			item.UnavailableReason = "当前账号暂不可用"
		}
		items = append(items, item)
		seen[groupName] = struct{}{}
	}
	for groupName, desc := range available {
		if _, ok := seen[groupName]; ok {
			continue
		}
		items = append(items, connectedAppClientGroupItem{
			ID:          groupName,
			Name:        groupName,
			DisplayName: groupName,
			Description: firstNonEmpty(desc, setting.GetUsableGroupDescription(groupName)),
			Available:   true,
			IsDefault:   groupName == tokenEffectiveGroup("", user.Group),
			IsAuto:      groupName == "auto",
		})
	}
	common.ApiSuccess(c, connectedAppClientGroupsResponse{
		DefaultGroup: tokenEffectiveGroup("", user.Group),
		Data:         items,
	})
}

func EnsureConnectedAppClientDedicatedToken(c *gin.Context) {
	app, grant, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	req, err := bindConnectedAppClientEnsureTokenRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	device := connectedAppClientDeviceFromRequest(c, app, accessToken, req)
	if device.Fingerprint != accessToken.DeviceFingerprint {
		connectedAppClientError(c, http.StatusForbidden, "app_not_authorized", "device_id does not match management token")
		return
	}

	var response connectedAppClientTokenResponse
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		user, err := getUserByIdTx(tx, accessToken.UserId)
		if err != nil {
			return err
		}
		group := strings.TrimSpace(req.Group)
		group, err = connectedAppClientEnsureTokenGroupTx(tx, app, grant, accessToken, group, req.Rotate)
		if err != nil {
			return err
		}
		if group != "" {
			if err := validateConnectedAppClientGroupForUser(user, group); err != nil {
				return err
			}
		}
		var txErr error
		response, txErr = ensureConnectedAppClientDedicatedTokenTx(c, tx, app, grant, user, device, group, req.Rotate)
		return txErr
	})
	if err != nil {
		connectedAppClientHandleError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func UpdateConnectedAppClientDedicatedTokenGroup(c *gin.Context) {
	app, _, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	var req connectedAppClientUpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	tokenID, err := parseConnectedAppClientTokenID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var summary connectedAppClientTokenSummary
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		user, err := getUserByIdTx(tx, accessToken.UserId)
		if err != nil {
			return err
		}
		if err := validateConnectedAppClientGroupForUser(user, req.Group); err != nil {
			return err
		}
		binding, token, err := connectedAppClientOwnedBindingTokenTx(tx, app.Id, accessToken.UserId, accessToken.DeviceFingerprint, tokenID)
		if err != nil {
			return err
		}
		beforeKey := token.Key
		token.Group = strings.TrimSpace(req.Group)
		if err := tx.Model(token).Select("group").Updates(token).Error; err != nil {
			return err
		}
		if err := model.InvalidateTokenCacheByKey(beforeKey); err != nil {
			return err
		}
		summary = buildConnectedAppClientTokenSummary(app, binding, token, user)
		return nil
	})
	if err != nil {
		connectedAppClientHandleError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"updated": true, "token": summary})
}

func RotateConnectedAppClientDedicatedToken(c *gin.Context) {
	app, grant, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	tokenID, err := parseConnectedAppClientTokenID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var response connectedAppClientTokenResponse
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		user, err := getUserByIdTx(tx, accessToken.UserId)
		if err != nil {
			return err
		}
		binding, token, err := connectedAppClientOwnedBindingTokenTx(tx, app.Id, accessToken.UserId, accessToken.DeviceFingerprint, tokenID)
		if err != nil {
			return err
		}
		device := snaplessDeviceInfoFromBinding(binding)
		response, err = rotateConnectedAppClientDedicatedTokenTx(c, tx, app, grant, user, binding, token, device)
		return err
	})
	if err != nil {
		connectedAppClientHandleError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func RevokeConnectedAppClientDedicatedToken(c *gin.Context) {
	app, _, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	tokenID, err := parseConnectedAppClientTokenID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		binding, token, err := connectedAppClientOwnedBindingTokenTx(tx, app.Id, accessToken.UserId, accessToken.DeviceFingerprint, tokenID)
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := model.RevokeConnectedAppTokenBinding(tx, binding, now); err != nil {
			return err
		}
		if err := model.DisableTokenWithTx(tx, token.Id, accessToken.UserId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		connectedAppClientHandleError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"revoked": true})
}

func RevokeConnectedAppClientDevice(c *gin.Context) {
	app, _, accessToken, ok := middleware.GetConnectedAppManagementContext(c)
	if !ok {
		connectedAppClientError(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
		return
	}
	var req connectedAppClientRevokeDeviceRequest
	if c.Request.ContentLength != 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	deviceFingerprint := accessToken.DeviceFingerprint
	if strings.TrimSpace(req.DeviceID) != "" {
		device := connectedAppDeviceFromRequest(c, app, snaplessDeviceRequest{DeviceID: req.DeviceID})
		if device.Fingerprint != deviceFingerprint {
			connectedAppClientError(c, http.StatusForbidden, "app_not_authorized", "device_id does not match management token")
			return
		}
	}
	dedicatedTokenRevoked := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		if err := model.RevokeConnectedAppAccessTokens(tx, app.Id, accessToken.UserId, deviceFingerprint, now); err != nil {
			return err
		}
		if req.RevokeDedicatedToken {
			if binding, err := findSnaplessBindingTx(tx, app.Id, accessToken.UserId, deviceFingerprint); err == nil {
				if err := model.RevokeConnectedAppTokenBinding(tx, binding, now); err != nil {
					return err
				}
				if err := model.DisableTokenWithTx(tx, binding.TokenId, accessToken.UserId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				dedicatedTokenRevoked = true
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"revoked": true, "dedicated_token_revoked": dedicatedTokenRevoked})
}

func issueConnectedAppManagementToken(tx *gorm.DB, app *model.ConnectedApp, grant *model.ConnectedAppGrant, userID int, device snaplessDeviceInfo, now int64) (string, int64, error) {
	if app == nil || grant == nil || userID <= 0 || strings.TrimSpace(device.Fingerprint) == "" {
		return "", 0, errors.New("app, grant, user and device are required")
	}
	raw, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return "", 0, err
	}
	token := "cdpat_" + raw
	expiresAt := now + connectedAppManagementTokenTTLSeconds
	accessToken := &model.ConnectedAppAccessToken{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            userID,
		DeviceFingerprint: device.Fingerprint,
		TokenHash:         middleware.ConnectedAppManagementTokenHash(token),
		Scopes:            strings.Join(grant.ScopeList(), " "),
		Status:            model.ConnectedAppAccessTokenStatusActive,
		ExpiresAt:         expiresAt,
		LastUsedAt:        now,
	}
	if err := model.CreateConnectedAppAccessToken(tx, accessToken); err != nil {
		return "", 0, err
	}
	return token, expiresAt, nil
}

func bindConnectedAppClientEnsureTokenRequest(c *gin.Context) (connectedAppClientEnsureTokenRequest, error) {
	var req connectedAppClientEnsureTokenRequest
	if c.Request.ContentLength != 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			return req, err
		}
	}
	return req, nil
}

func connectedAppClientEnsureTokenGroupTx(tx *gorm.DB, app *model.ConnectedApp, grant *model.ConnectedAppGrant, accessToken *model.ConnectedAppAccessToken, requestedGroup string, rotate bool) (string, error) {
	requestedGroup = strings.TrimSpace(requestedGroup)
	if app == nil || grant == nil || accessToken == nil {
		return requestedGroup, nil
	}
	binding, err := findSnaplessBindingTx(tx, app.Id, accessToken.UserId, accessToken.DeviceFingerprint)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return requestedGroup, nil
		}
		return "", err
	}
	token, err := getTokenByIdTx(tx, binding.TokenId, accessToken.UserId)
	if err != nil {
		return "", err
	}
	if rotate {
		if !model.ConnectedAppHasScope(accessToken.ScopeList(), connectedAppScopeTokenRotateOwn) ||
			!model.ConnectedAppHasScope(grant.ScopeList(), connectedAppScopeTokenRotateOwn) {
			return "", connectedAppClientCodedError{Status: http.StatusForbidden, Code: "insufficient_scope", Message: "management token scope is insufficient"}
		}
		if requestedGroup == "" {
			requestedGroup = strings.TrimSpace(token.Group)
		}
	}
	if requestedGroup == "" || strings.TrimSpace(token.Group) == requestedGroup {
		return requestedGroup, nil
	}
	if model.ConnectedAppHasScope(accessToken.ScopeList(), connectedAppScopeTokenGroupUpdate) &&
		model.ConnectedAppHasScope(grant.ScopeList(), connectedAppScopeTokenGroupUpdate) {
		return requestedGroup, nil
	}
	return "", connectedAppClientCodedError{Status: http.StatusForbidden, Code: "insufficient_scope", Message: "management token scope is insufficient"}
}

func connectedAppClientDeviceFromRequest(c *gin.Context, app *model.ConnectedApp, accessToken *model.ConnectedAppAccessToken, req connectedAppClientEnsureTokenRequest) snaplessDeviceInfo {
	if strings.TrimSpace(req.DeviceID) != "" {
		return connectedAppDeviceFromRequest(c, app, snaplessDeviceRequest{
			DeviceID:   req.DeviceID,
			DeviceName: req.DeviceName,
			Platform:   req.Platform,
			AppVersion: req.AppVersion,
			Client:     req.Client,
		})
	}
	appName := "Connected App"
	appSlug := "connected-app"
	if app != nil {
		appSlug = app.Slug
		if strings.TrimSpace(app.Name) != "" {
			appName = app.Name
		}
	}
	return snaplessDeviceInfo{
		Fingerprint: accessToken.DeviceFingerprint,
		DeviceName:  limitString(firstNonEmpty(req.DeviceName, c.Query("device_name"), c.GetHeader("X-Connected-App-Device-Name"), c.GetHeader("X-Snapless-Device-Name"), appName), 128),
		Platform:    limitString(strings.ToLower(firstNonEmpty(req.Platform, c.Query("platform"), c.GetHeader("X-Connected-App-Platform"), c.GetHeader("X-Snapless-Platform"), "desktop")), 32),
		AppVersion:  limitString(firstNonEmpty(req.AppVersion, c.Query("app_version"), c.GetHeader("X-Connected-App-Version"), c.GetHeader("X-Snapless-App-Version")), 64),
		Client:      limitString(firstNonEmpty(req.Client, c.Query("client"), c.GetHeader("X-Connected-App-Client"), c.GetHeader("X-Snapless-Client"), c.GetHeader("User-Agent"), appSlug), 64),
	}
}

func ensureConnectedAppClientDedicatedTokenTx(c *gin.Context, tx *gorm.DB, app *model.ConnectedApp, grant *model.ConnectedAppGrant, user *model.User, device snaplessDeviceInfo, group string, rotate bool) (connectedAppClientTokenResponse, error) {
	now := common.GetTimestamp()
	var existingBinding *model.ConnectedAppTokenBinding
	if binding, err := findSnaplessBindingTx(tx, app.Id, user.Id, device.Fingerprint); err == nil {
		existingBinding = binding
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return connectedAppClientTokenResponse{}, err
	}
	if existingBinding != nil && !rotate {
		token, err := getTokenByIdTx(tx, existingBinding.TokenId, user.Id)
		if err != nil {
			return connectedAppClientTokenResponse{}, err
		}
		if group != "" && strings.TrimSpace(token.Group) != group {
			token.Group = group
			if err := tx.Model(token).Select("group").Updates(token).Error; err != nil {
				return connectedAppClientTokenResponse{}, err
			}
			if err := model.InvalidateTokenCacheByKey(token.Key); err != nil {
				return connectedAppClientTokenResponse{}, err
			}
		}
		if snaplessTokenReusable(token, now) {
			return connectedAppClientTokenResponse{
				Selected:         true,
				Created:          false,
				Rotated:          false,
				APIKeyOnce:       false,
				BaseURL:          snaplessAPIBaseURL(c),
				Token:            buildConnectedAppClientTokenSummary(app, existingBinding, token, user),
				RequiresRotation: true,
				Message:          "Existing dedicated token key is not recoverable; rotate to receive a new key.",
			}, nil
		}
	}
	if existingBinding != nil {
		if err := model.RecordConnectedAppTokenAttribution(tx, *existingBinding, now); err != nil {
			return connectedAppClientTokenResponse{}, err
		}
		if err := model.DisableTokenWithTx(tx, existingBinding.TokenId, user.Id); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return connectedAppClientTokenResponse{}, err
		}
	}
	token, key, err := createConnectedAppClientToken(tx, app, user.Id, device, group, now)
	if err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	binding, err := model.UpsertConnectedAppTokenBinding(tx, model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            user.Id,
		TokenId:           token.Id,
		DeviceFingerprint: device.Fingerprint,
		DeviceName:        device.DeviceName,
		Platform:          device.Platform,
		AppVersion:        device.AppVersion,
	}, now)
	if err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	if err := model.RecordConnectedAppTokenAttribution(tx, *binding, now); err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	return connectedAppClientTokenResponse{
		Selected:   true,
		Created:    existingBinding == nil,
		Rotated:    existingBinding != nil,
		APIKeyOnce: true,
		APIKey:     snaplessAPIKey(key),
		BaseURL:    snaplessAPIBaseURL(c),
		Token:      buildConnectedAppClientTokenSummary(app, binding, token, user),
	}, nil
}

func rotateConnectedAppClientDedicatedTokenTx(c *gin.Context, tx *gorm.DB, app *model.ConnectedApp, grant *model.ConnectedAppGrant, user *model.User, binding *model.ConnectedAppTokenBinding, token *model.Token, device snaplessDeviceInfo) (connectedAppClientTokenResponse, error) {
	now := common.GetTimestamp()
	group := strings.TrimSpace(token.Group)
	if err := model.RecordConnectedAppTokenAttribution(tx, *binding, now); err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	if err := model.DisableTokenWithTx(tx, token.Id, user.Id); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return connectedAppClientTokenResponse{}, err
	}
	newToken, key, err := createConnectedAppClientToken(tx, app, user.Id, device, group, now)
	if err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	newBinding, err := model.UpsertConnectedAppTokenBinding(tx, model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            user.Id,
		TokenId:           newToken.Id,
		DeviceFingerprint: device.Fingerprint,
		DeviceName:        device.DeviceName,
		Platform:          device.Platform,
		AppVersion:        device.AppVersion,
	}, now)
	if err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	if err := model.RecordConnectedAppTokenAttribution(tx, *newBinding, now); err != nil {
		return connectedAppClientTokenResponse{}, err
	}
	return connectedAppClientTokenResponse{
		Selected:   true,
		Created:    false,
		Rotated:    true,
		APIKeyOnce: true,
		APIKey:     snaplessAPIKey(key),
		BaseURL:    snaplessAPIBaseURL(c),
		Token:      buildConnectedAppClientTokenSummary(app, newBinding, newToken, user),
	}, nil
}

func createConnectedAppClientToken(tx *gorm.DB, app *model.ConnectedApp, userID int, device snaplessDeviceInfo, group string, now int64) (*model.Token, string, error) {
	key, err := common.GenerateKey()
	if err != nil {
		return nil, "", err
	}
	token := model.Token{
		UserId:                userID,
		Name:                  connectedAppTokenName(app, device),
		Key:                   key,
		Status:                common.TokenStatusEnabled,
		CreatedTime:           now,
		AccessedTime:          now,
		ExpiredTime:           -1,
		RemainQuota:           0,
		UnlimitedQuota:        true,
		QuotaHardLimitEnabled: false,
		ModelLimitsEnabled:    false,
		Group:                 strings.TrimSpace(group),
	}
	if app != nil && app.Slug == model.ConnectedAppSlugSnapless {
		token.ModelLimitsEnabled = true
		token.ModelLimits = strings.Join(getSnaplessModelAliases().All(), ",")
	}
	if err := tx.Create(&token).Error; err != nil {
		return nil, "", err
	}
	return &token, key, nil
}

func connectedAppClientActiveBindingToken(appID int, userID int, deviceFingerprint string) (*model.ConnectedAppTokenBinding, *model.Token, error) {
	binding, err := model.FindActiveConnectedAppTokenBinding(appID, userID, deviceFingerprint)
	if err != nil {
		return nil, nil, err
	}
	token, err := model.GetTokenByIds(binding.TokenId, userID)
	if err != nil {
		return nil, nil, err
	}
	return binding, token, nil
}

func connectedAppClientOwnedBindingTokenTx(tx *gorm.DB, appID int, userID int, deviceFingerprint string, tokenID int) (*model.ConnectedAppTokenBinding, *model.Token, error) {
	binding, err := findSnaplessBindingTx(tx, appID, userID, deviceFingerprint)
	if err != nil {
		return nil, nil, err
	}
	if binding.TokenId != tokenID {
		return nil, nil, connectedAppClientCodedError{Status: http.StatusConflict, Code: "token_not_owned_by_connected_app", Message: "token is not owned by this connected app device"}
	}
	token, err := getTokenByIdTx(tx, tokenID, userID)
	if err != nil {
		return nil, nil, err
	}
	return binding, token, nil
}

func buildConnectedAppClientTokenSummary(app *model.ConnectedApp, binding *model.ConnectedAppTokenBinding, token *model.Token, user *model.User) connectedAppClientTokenSummary {
	if token == nil {
		return connectedAppClientTokenSummary{}
	}
	userGroup := ""
	var boundGroups []string
	if user != nil {
		userGroup = user.Group
		boundGroups = user.GetTokenGroups()
	}
	effectiveGroup := tokenEffectiveGroup(token.Group, userGroup)
	groupAvailable := false
	groupUnavailableReason := ""
	if user != nil {
		groups := service.GetUserUsableGroupsWithBindings(user.Group, boundGroups)
		_, groupAvailable = groups[effectiveGroup]
		if !groupAvailable {
			groupUnavailableReason = "当前账号暂不可用"
		}
	}
	expiredAt := token.ExpiredTime
	var expiredAtPtr *int64
	if expiredAt != -1 {
		expiredAtPtr = &expiredAt
	}
	var lastUsedAtPtr *int64
	if binding != nil && binding.LastUsedAt > 0 {
		last := binding.LastUsedAt
		lastUsedAtPtr = &last
	}
	slug := ""
	deviceID := ""
	owned := false
	if app != nil && binding != nil && binding.AppId == app.Id {
		slug = app.Slug
		deviceID = binding.DeviceFingerprint
		owned = true
	}
	return connectedAppClientTokenSummary{
		ID:                     token.Id,
		Name:                   token.Name,
		MaskedKey:              "sk-" + token.GetMaskedKey(),
		Status:                 token.Status,
		Group:                  strings.TrimSpace(token.Group),
		EffectiveGroup:         effectiveGroup,
		GroupAvailable:         groupAvailable,
		GroupUnavailableReason: groupUnavailableReason,
		OwnedByConnectedApp:    owned,
		ConnectedAppSlug:       slug,
		DeviceID:               deviceID,
		ExpiredAt:              expiredAtPtr,
		LastUsedAt:             lastUsedAtPtr,
		UnlimitedQuota:         token.UnlimitedQuota,
		QuotaHardLimitEnabled:  token.QuotaHardLimitEnabled,
		ModelLimitsEnabled:     token.ModelLimitsEnabled,
	}
}

func connectedAppClientCapabilitiesForApp(app *model.ConnectedApp) connectedAppClientCapabilities {
	if app == nil {
		return connectedAppClientCapabilities{}
	}
	return connectedAppClientCapabilitiesForScopes(app.ScopeList())
}

func connectedAppClientCapabilitiesForScopes(scopes []string) connectedAppClientCapabilities {
	return connectedAppClientCapabilities{
		Groups:           model.ConnectedAppHasScope(scopes, connectedAppScopeGroupRead),
		DedicatedTokens:  model.ConnectedAppHasScope(scopes, connectedAppScopeTokenCreate),
		TokenRotate:      model.ConnectedAppHasScope(scopes, connectedAppScopeTokenRotateOwn),
		TokenRevoke:      model.ConnectedAppHasScope(scopes, connectedAppScopeTokenRevokeOwn),
		TokenGroupUpdate: model.ConnectedAppHasScope(scopes, connectedAppScopeTokenGroupUpdate),
		OpenAIModels:     model.ConnectedAppHasScope(scopes, connectedAppScopeOpenAIModels),
		OpenAIResponses:  model.ConnectedAppHasScope(scopes, connectedAppScopeOpenAIResponses),
		OpenAIChat:       model.ConnectedAppHasScope(scopes, connectedAppScopeOpenAIChat),
	}
}

func connectedAppUsesManagementTokenFlow(app *model.ConnectedApp) bool {
	if app == nil {
		return false
	}
	scopes := app.ScopeList()
	return model.ConnectedAppHasScope(scopes, connectedAppScopeTokenCreate) &&
		!model.ConnectedAppHasScope(scopes, connectedAppScopeTokenManage)
}

func connectedAppClientEndpoints(c *gin.Context, app *model.ConnectedApp) map[string]string {
	if app == nil {
		return map[string]string{}
	}
	base := snaplessServerBaseURL(c) + "/api/connected-app-clients/" + url.PathEscape(app.Slug)
	return map[string]string{
		"config":        base + "/config",
		"groups":        base + "/groups",
		"tokens_ensure": base + "/tokens/ensure",
		"token_group":   base + "/tokens/{id}/group",
		"token_rotate":  base + "/tokens/{id}/rotate",
		"token_revoke":  base + "/tokens/{id}/revoke",
		"device_revoke": base + "/device/revoke",
	}
}

func connectedAppClientUserFromModel(user *model.User) connectedAppClientUser {
	if user == nil {
		return connectedAppClientUser{}
	}
	return connectedAppClientUser{
		ID:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Group:       user.Group,
	}
}

func validateConnectedAppClientGroupForUser(user *model.User, group string) error {
	group = strings.TrimSpace(group)
	if user == nil || user.Id <= 0 {
		return errors.New("user is required")
	}
	if group == "" {
		return nil
	}
	groups := service.GetUserUsableGroupsWithBindings(user.Group, user.GetTokenGroups())
	if _, ok := groups[group]; ok {
		return nil
	}
	return connectedAppClientCodedError{Status: http.StatusConflict, Code: "token_group_unavailable", Message: "当前账号不能使用该分组"}
}

func getUserByIdTx(tx *gorm.DB, userID int) (*model.User, error) {
	var user model.User
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func parseConnectedAppClientTokenID(c *gin.Context) (int, error) {
	return parseIntPathParam(c, "id", "token id")
}

type connectedAppClientCodedError struct {
	Status  int
	Code    string
	Message string
}

func (err connectedAppClientCodedError) Error() string {
	return err.Message
}

func connectedAppClientHandleError(c *gin.Context, err error) {
	var coded connectedAppClientCodedError
	if errors.As(err, &coded) {
		connectedAppClientError(c, coded.Status, coded.Code, coded.Message)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		connectedAppClientError(c, http.StatusNotFound, "token_not_found", "token not found")
		return
	}
	common.ApiError(c, err)
}

func connectedAppClientError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
		"error": gin.H{
			"code":       code,
			"message":    message,
			"request_id": c.GetHeader(common.RequestIdKey),
		},
	})
}
