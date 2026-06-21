package controller

import (
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	connectedAppScopeOpenAIModels              = "openai.models"
	connectedAppScopeOpenAIChat                = "openai.chat"
	connectedAppScopeOpenAIAudioTranscriptions = "openai.audio.transcriptions"
	connectedAppScopeQuotaRead                 = "quota.read"
	connectedAppScopeTokenManage               = "token.manage"
)

type connectedAppDeveloperConfigResponse struct {
	App          snaplessAppResponse `json:"app"`
	Owner        bool                `json:"owner"`
	BaseURL      string              `json:"base_url"`
	APIEndpoints map[string]string   `json:"api_endpoints"`
	DeviceFlow   map[string]string   `json:"device_flow"`
	Scopes       []string            `json:"scopes"`
}

type connectedAppDeveloperSDKConfigResponse struct {
	App                snaplessAppResponse                 `json:"app"`
	Owner              bool                                `json:"owner"`
	BaseURL            string                              `json:"base_url"`
	APIEndpoints       map[string]string                   `json:"api_endpoints"`
	DeviceFlow         map[string]string                   `json:"device_flow"`
	DeveloperEndpoints map[string]string                   `json:"developer_endpoints"`
	Scopes             []string                            `json:"scopes"`
	Permissions        connectedAppDeveloperPermissions    `json:"permissions"`
	OpenAPIURL         string                              `json:"openapi_url"`
	SDK                connectedAppDeveloperSDKInstruction `json:"sdk"`
}

type connectedAppDeveloperPermissions struct {
	CanCreateKey bool `json:"can_create_key"`
	CanReadUsage bool `json:"can_read_usage"`
}

type connectedAppDeveloperSDKInstruction struct {
	OpenAICompatible bool   `json:"openai_compatible"`
	BaseURL          string `json:"base_url"`
	APIKeyEnv        string `json:"api_key_env"`
	APIKeyPrefix     string `json:"api_key_prefix"`
	Authorization    string `json:"authorization"`
}

type connectedAppDeveloperAuthorizationResponse struct {
	UserID   int                           `json:"user_id"`
	UserName string                        `json:"user_name"`
	Grant    snaplessGrantResponse         `json:"grant"`
	Devices  []connectedAppDeveloperDevice `json:"devices"`
}

type connectedAppDeveloperDevice struct {
	Device     snaplessDeviceInfo   `json:"device"`
	Token      snaplessTokenSummary `json:"token"`
	Status     string               `json:"status"`
	LastUsedAt int64                `json:"last_used_at,omitempty"`
	RevokedAt  int64                `json:"revoked_at,omitempty"`
	CreatedAt  int64                `json:"created_at,omitempty"`
	UpdatedAt  int64                `json:"updated_at,omitempty"`
}

type connectedAppDeveloperKeyRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
	Client     string `json:"client"`
	Rotate     bool   `json:"rotate"`
}

type connectedAppDeveloperUsageResponse struct {
	App        snaplessAppResponse               `json:"app"`
	StartTime  int64                             `json:"start_time"`
	EndTime    int64                             `json:"end_time"`
	TokenCount int                               `json:"token_count"`
	Total      connectedAppUsageTotals           `json:"total"`
	ByModel    []connectedAppUsageByModel        `json:"by_model"`
	ByToken    []connectedAppDeveloperTokenUsage `json:"by_token"`
}

type connectedAppUsageTotals struct {
	RequestCount     int64 `json:"request_count"`
	Quota            int64 `json:"quota"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

type connectedAppUsageByModel struct {
	ModelName        string `json:"model_name"`
	RequestCount     int64  `json:"request_count"`
	Quota            int64  `json:"quota"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

type connectedAppDeveloperTokenUsage struct {
	TokenID          int                `json:"token_id"`
	TokenName        string             `json:"token_name"`
	UserID           int                `json:"user_id"`
	Status           string             `json:"status"`
	Device           snaplessDeviceInfo `json:"device"`
	RequestCount     int64              `json:"request_count"`
	Quota            int64              `json:"quota"`
	PromptTokens     int64              `json:"prompt_tokens"`
	CompletionTokens int64              `json:"completion_tokens"`
}

type connectedAppDeveloperUsageTokenMeta struct {
	TokenID int
	UserID  int
	Status  string
	Device  snaplessDeviceInfo
}

type connectedAppDeveloperSessionResponse struct {
	ID           int64              `json:"id"`
	Status       string             `json:"status"`
	UserID       int                `json:"user_id"`
	UserName     string             `json:"user_name"`
	TokenID      int                `json:"token_id"`
	TokenCreated bool               `json:"token_created"`
	Device       snaplessDeviceInfo `json:"device"`
	ExpiresAt    int64              `json:"expires_at"`
	LastPolledAt int64              `json:"last_polled_at"`
	AuthorizedAt int64              `json:"authorized_at"`
	ConsumedAt   int64              `json:"consumed_at"`
	CreatedAt    int64              `json:"created_at"`
	UpdatedAt    int64              `json:"updated_at"`
}

type connectedAppTokenResponse struct {
	App          snaplessAppResponse       `json:"app"`
	Grant        snaplessGrantResponse     `json:"grant"`
	Device       snaplessDeviceInfo        `json:"device"`
	Token        snaplessTokenSummary      `json:"token"`
	Endpoints    map[string]string         `json:"endpoints"`
	BaseURL      string                    `json:"base_url"`
	APIKey       string                    `json:"api_key,omitempty"`
	Created      bool                      `json:"created"`
	Rotated      bool                      `json:"rotated"`
	APIKeyOnce   bool                      `json:"api_key_once"`
	Instructions snaplessClientInstruction `json:"instructions"`
}

func GetConnectedAppDeveloperConfig(c *gin.Context) {
	app, owner, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, connectedAppDeveloperConfigResponse{
		App:          buildSnaplessAppResponse(app),
		Owner:        owner,
		BaseURL:      snaplessAPIBaseURL(c),
		APIEndpoints: connectedAppAPIEndpoints(c, app),
		DeviceFlow:   connectedAppDeviceFlowEndpoints(c, app),
		Scopes:       app.ScopeList(),
	})
}

func GetConnectedAppDeveloperSDKConfig(c *gin.Context) {
	app, owner, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	developerEndpoints := connectedAppDeveloperEndpoints(c, app)
	common.ApiSuccess(c, connectedAppDeveloperSDKConfigResponse{
		App:                buildSnaplessAppResponse(app),
		Owner:              owner,
		BaseURL:            snaplessAPIBaseURL(c),
		APIEndpoints:       connectedAppAPIEndpoints(c, app),
		DeviceFlow:         connectedAppDeviceFlowEndpoints(c, app),
		DeveloperEndpoints: developerEndpoints,
		Scopes:             app.ScopeList(),
		Permissions:        connectedAppDeveloperScopePermissions(app),
		OpenAPIURL:         developerEndpoints["openapi"],
		SDK: connectedAppDeveloperSDKInstruction{
			OpenAICompatible: true,
			BaseURL:          snaplessAPIBaseURL(c),
			APIKeyEnv:        "OPENAI_API_KEY",
			APIKeyPrefix:     "sk-",
			Authorization:    "Bearer sk-<api_key>",
		},
	})
}

func GetConnectedAppDeveloperOpenAPI(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, connectedAppDeveloperOpenAPISpec(c, app))
}

func CreateConnectedAppDeveloperKey(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !connectedAppHasAllowedScope(app, connectedAppScopeTokenManage) {
		common.ApiError(c, errors.New("connected app requires token.manage scope to create developer keys"))
		return
	}
	req, err := bindConnectedAppDeveloperKeyRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userID := c.GetInt("id")
	device := connectedAppDeveloperKeyDevice(c, app, userID, req)
	var response connectedAppTokenResponse
	var tokenID int
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var txErr error
		response, tokenID, txErr = ensureConnectedAppTokenForDeviceTx(c, tx, app, userID, device, req.Rotate)
		return txErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if response.Created || response.Rotated {
		action := "connected_app_developer.key_create"
		if response.Rotated {
			action = "connected_app_developer.key_rotate"
		}
		recordConnectedAppAuditBestEffort(c, action, "connected_app", app.Id, nil, gin.H{
			"token_id":           tokenID,
			"device_fingerprint": device.Fingerprint,
			"device_name":        device.DeviceName,
			"rotated":            response.Rotated,
			"api_key_once":       response.APIKeyOnce,
		})
	}
	common.ApiSuccess(c, response)
}

func GetConnectedAppDeveloperUsage(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !connectedAppHasAllowedScope(app, connectedAppScopeQuotaRead) {
		common.ApiError(c, errors.New("connected app requires quota.read scope to read usage"))
		return
	}
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tokenID, err := parseOptionalIntQuery(c, "token_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userID, err := parseOptionalIntQuery(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := connectedAppDeveloperUsage(app, connectedAppDeveloperUsageFilters{
		StartTime: startTime,
		EndTime:   endTime,
		TokenID:   tokenID,
		UserID:    userID,
		ModelName: strings.TrimSpace(c.Query("model_name")),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func ListConnectedAppDeveloperAuthorizations(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo := common.GetPageQuery(c)
	var grants []model.ConnectedAppGrant
	query := model.DB.Model(&model.ConnectedAppGrant{}).Where("app_id = ?", app.Id)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.Order("last_used_at desc").Order("updated_at desc").Order("id desc").
		Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).
		Find(&grants).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	userIDs := make([]int, 0, len(grants))
	for _, grant := range grants {
		userIDs = append(userIDs, grant.UserId)
	}
	names := connectedAppUserNameMap(userIDs...)
	devicesByUser, err := connectedAppDeveloperDevicesByUser(app.Id, userIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]connectedAppDeveloperAuthorizationResponse, 0, len(grants))
	for i := range grants {
		grant := grants[i]
		items = append(items, connectedAppDeveloperAuthorizationResponse{
			UserID:   grant.UserId,
			UserName: names[grant.UserId],
			Grant:    buildSnaplessGrantResponse(&grant, app.DefaultScopeList()),
			Devices:  devicesByUser[grant.UserId],
		})
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
	})
}

func ListConnectedAppDeveloperDeviceSessions(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo := common.GetPageQuery(c)
	var sessions []model.ConnectedAppDeviceSession
	query := model.DB.Model(&model.ConnectedAppDeviceSession{}).Where("app_id = ?", app.Id)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.Order("created_at desc").Order("id desc").
		Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).
		Find(&sessions).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	userIDs := make([]int, 0, len(sessions))
	for _, session := range sessions {
		userIDs = append(userIDs, session.UserId)
	}
	names := connectedAppUserNameMap(userIDs...)
	items := make([]connectedAppDeveloperSessionResponse, 0, len(sessions))
	for i := range sessions {
		items = append(items, buildConnectedAppDeveloperSessionResponse(&sessions[i], names))
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
	})
}

func StartConnectedAppDeviceFlow(c *gin.Context) {
	req, err := bindSnaplessDeviceRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	app, err := connectedAppForPublicDeviceFlow(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	device := connectedAppDeviceFromRequest(c, app, req)
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
		VerificationURI: connectedAppVerificationURI(c, app, session.UserCode),
		ExpiresIn:       expiresAt - now,
		Interval:        session.PollInterval,
		App:             buildSnaplessAppResponse(app),
		Device:          device,
	})
}

func GetConnectedAppDeviceStatus(c *gin.Context) {
	app, err := connectedAppForPublicDeviceFlow(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
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
	if session.AppId != app.Id {
		common.ApiError(c, errors.New("设备授权码不属于该应用"))
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
	readiness, err := connectedAppReadinessForUser(app, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !readiness.OK {
		if err := service.EnqueueConnectedAppHealthWarningOutboxWithDB(model.DB, service.ConnectedAppHealthWarningInput{
			App:       *app,
			UserId:    c.GetInt("id"),
			Session:   session,
			Status:    readiness.Status,
			Checks:    readiness.Checks,
			CreatedAt: now,
		}); err != nil {
			common.SysLog("failed to enqueue connected app health warning notification outbox: " + err.Error())
		}
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

func AuthorizeConnectedAppDevice(c *gin.Context) {
	app, err := connectedAppForPublicDeviceFlow(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
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

	userId := c.GetInt("id")
	now := common.GetTimestamp()
	readiness, err := connectedAppReadinessForUser(app, userId)
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
			return errors.New("设备授权码不属于该应用")
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

		tokenResponse, tokenId, err := ensureConnectedAppTokenForDeviceTx(c, tx, app, userId, snaplessDeviceInfoFromSession(&session), false)
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
			common.SysLog("failed to enqueue connected app device authorization notification outbox: " + err.Error())
		}
	}
	common.ApiSuccess(c, response)
}

func PollConnectedAppDeviceFlow(c *gin.Context) {
	app, err := connectedAppForPublicDeviceFlow(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
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
	response := connectedAppTokenResponse{
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
		response.Endpoints = connectedAppAPIEndpoints(c, app)

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
		response = buildConnectedAppTokenResponse(c, app, grant, binding, token, snaplessDeviceInfoFromSession(&session), token.Key, session.TokenCreated, false)
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

type connectedAppDeveloperUsageFilters struct {
	StartTime int64
	EndTime   int64
	TokenID   int
	UserID    int
	ModelName string
}

func bindConnectedAppDeveloperKeyRequest(c *gin.Context) (connectedAppDeveloperKeyRequest, error) {
	var req connectedAppDeveloperKeyRequest
	if c.Request.ContentLength != 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			return req, err
		}
	}
	return req, nil
}

func connectedAppDeveloperKeyDevice(c *gin.Context, app *model.ConnectedApp, userID int, req connectedAppDeveloperKeyRequest) snaplessDeviceInfo {
	if strings.TrimSpace(req.DeviceID) == "" {
		req.DeviceID = "developer:user"
		if userID > 0 {
			req.DeviceID = "developer:user:" + strconv.Itoa(userID)
		}
	}
	if strings.TrimSpace(req.DeviceName) == "" {
		req.DeviceName = "Developer self-service"
	}
	if strings.TrimSpace(req.Platform) == "" {
		req.Platform = "server"
	}
	if strings.TrimSpace(req.Client) == "" && app != nil {
		req.Client = app.Slug + "-developer"
	}
	return connectedAppDeviceFromRequest(c, app, snaplessDeviceRequest{
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
		Client:     req.Client,
	})
}

func connectedAppDeveloperScopePermissions(app *model.ConnectedApp) connectedAppDeveloperPermissions {
	return connectedAppDeveloperPermissions{
		CanCreateKey: connectedAppHasAllowedScope(app, connectedAppScopeTokenManage),
		CanReadUsage: connectedAppHasAllowedScope(app, connectedAppScopeQuotaRead),
	}
}

func connectedAppDeveloperEndpoints(c *gin.Context, app *model.ConnectedApp) map[string]string {
	if app == nil {
		return map[string]string{}
	}
	base := snaplessServerBaseURL(c) + "/api/connected-apps/" + url.PathEscape(app.Slug) + "/developer"
	return map[string]string{
		"keys":       base + "/keys",
		"openapi":    base + "/openapi",
		"sdk_config": base + "/sdk-config",
		"usage":      base + "/usage",
	}
}

func connectedAppHasAllowedScope(app *model.ConnectedApp, scope string) bool {
	if app == nil || strings.TrimSpace(scope) == "" {
		return false
	}
	for _, item := range app.ScopeList() {
		if item == scope {
			return true
		}
	}
	return false
}

func connectedAppDeveloperOpenAPISpec(c *gin.Context, app *model.ConnectedApp) gin.H {
	paths := gin.H{}
	if connectedAppHasAllowedScope(app, connectedAppScopeOpenAIModels) {
		paths["/v1/models"] = gin.H{
			"get": connectedAppOpenAPIOperation("List models", []string{connectedAppScopeOpenAIModels}, nil),
		}
		paths["/v1/models/{model}"] = gin.H{
			"get": connectedAppOpenAPIOperation("Retrieve model", []string{connectedAppScopeOpenAIModels}, nil),
		}
	}
	if connectedAppHasAllowedScope(app, connectedAppScopeOpenAIChat) {
		paths["/v1/chat/completions"] = gin.H{
			"post": connectedAppOpenAPIOperation("Create chat completion", []string{connectedAppScopeOpenAIChat}, gin.H{
				"required": true,
				"content": gin.H{
					"application/json": gin.H{
						"schema": gin.H{
							"type":     "object",
							"required": []string{"model", "messages"},
							"properties": gin.H{
								"model": gin.H{"type": "string"},
								"messages": gin.H{
									"type":  "array",
									"items": gin.H{"type": "object"},
								},
								"stream": gin.H{"type": "boolean"},
							},
						},
					},
				},
			}),
		}
	}
	if connectedAppHasAllowedScope(app, connectedAppScopeOpenAIAudioTranscriptions) {
		paths["/v1/audio/transcriptions"] = gin.H{
			"post": connectedAppOpenAPIOperation("Create audio transcription", []string{connectedAppScopeOpenAIAudioTranscriptions}, gin.H{
				"required": true,
				"content": gin.H{
					"multipart/form-data": gin.H{
						"schema": gin.H{
							"type":     "object",
							"required": []string{"file", "model"},
							"properties": gin.H{
								"file":  gin.H{"type": "string", "format": "binary"},
								"model": gin.H{"type": "string"},
							},
						},
					},
				},
			}),
		}
	}
	if connectedAppHasAllowedScope(app, connectedAppScopeQuotaRead) {
		paths["/api/usage/token"] = gin.H{
			"get": connectedAppOpenAPIOperation("Read current token usage", []string{connectedAppScopeQuotaRead}, nil),
		}
	}

	title := "Connected App Data Proxy API"
	if app != nil && strings.TrimSpace(app.Name) != "" {
		title = app.Name + " Data Proxy API"
	}
	return gin.H{
		"openapi": "3.0.3",
		"info": gin.H{
			"title":   title,
			"version": "1.0.0",
		},
		"servers": []gin.H{
			{"url": snaplessServerBaseURL(c)},
		},
		"security": []gin.H{
			{"bearerAuth": []string{}},
		},
		"paths": paths,
		"components": gin.H{
			"securitySchemes": gin.H{
				"bearerAuth": gin.H{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "API key",
				},
			},
		},
	}
}

func connectedAppOpenAPIOperation(summary string, scopes []string, requestBody gin.H) gin.H {
	operation := gin.H{
		"summary":                summary,
		"x-connected-app-scopes": scopes,
		"security": []gin.H{
			{"bearerAuth": []string{}},
		},
		"responses": gin.H{
			"200": gin.H{"description": "OK"},
		},
	}
	if requestBody != nil {
		operation["requestBody"] = requestBody
	}
	return operation
}

func connectedAppDeveloperUsage(app *model.ConnectedApp, filters connectedAppDeveloperUsageFilters) (connectedAppDeveloperUsageResponse, error) {
	if model.LOG_DB == nil {
		return connectedAppDeveloperUsageResponse{}, errors.New("log database is not initialized")
	}
	tokenIDs, tokensByID, err := connectedAppDeveloperUsageTokens(app.Id)
	if err != nil {
		return connectedAppDeveloperUsageResponse{}, err
	}
	if filters.TokenID > 0 {
		if _, ok := tokensByID[filters.TokenID]; !ok {
			return emptyConnectedAppDeveloperUsage(app, filters, 0), nil
		}
		tokenIDs = []int{filters.TokenID}
	}
	if filters.UserID > 0 {
		filtered := make([]int, 0, len(tokenIDs))
		for _, tokenID := range tokenIDs {
			if token, ok := tokensByID[tokenID]; ok && token.UserID == filters.UserID {
				filtered = append(filtered, tokenID)
			}
		}
		tokenIDs = filtered
	}
	if len(tokenIDs) == 0 {
		return emptyConnectedAppDeveloperUsage(app, filters, 0), nil
	}

	response := emptyConnectedAppDeveloperUsage(app, filters, len(tokenIDs))
	if err := connectedAppDeveloperUsageLogQuery(filters, tokenIDs).
		Select("COUNT(*) AS request_count, COALESCE(SUM(quota), 0) AS quota, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Scan(&response.Total).Error; err != nil {
		return connectedAppDeveloperUsageResponse{}, err
	}
	if err := connectedAppDeveloperUsageLogQuery(filters, tokenIDs).
		Select("model_name, COUNT(*) AS request_count, COALESCE(SUM(quota), 0) AS quota, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Group("model_name").
		Order("quota DESC").
		Scan(&response.ByModel).Error; err != nil {
		return connectedAppDeveloperUsageResponse{}, err
	}

	var tokenRows []struct {
		TokenID          int    `gorm:"column:token_id"`
		TokenName        string `gorm:"column:token_name"`
		UserID           int    `gorm:"column:user_id"`
		RequestCount     int64  `gorm:"column:request_count"`
		Quota            int64  `gorm:"column:quota"`
		PromptTokens     int64  `gorm:"column:prompt_tokens"`
		CompletionTokens int64  `gorm:"column:completion_tokens"`
	}
	if err := connectedAppDeveloperUsageLogQuery(filters, tokenIDs).
		Select("token_id, MAX(token_name) AS token_name, MAX(user_id) AS user_id, COUNT(*) AS request_count, COALESCE(SUM(quota), 0) AS quota, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Group("token_id").
		Order("quota DESC").
		Scan(&tokenRows).Error; err != nil {
		return connectedAppDeveloperUsageResponse{}, err
	}
	response.ByToken = make([]connectedAppDeveloperTokenUsage, 0, len(tokenRows))
	for _, row := range tokenRows {
		token, ok := tokensByID[row.TokenID]
		if !ok {
			continue
		}
		response.ByToken = append(response.ByToken, connectedAppDeveloperTokenUsage{
			TokenID:          row.TokenID,
			TokenName:        row.TokenName,
			UserID:           token.UserID,
			Status:           token.Status,
			Device:           token.Device,
			RequestCount:     row.RequestCount,
			Quota:            row.Quota,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
		})
	}
	return response, nil
}

func connectedAppDeveloperUsageTokens(appID int) ([]int, map[int]connectedAppDeveloperUsageTokenMeta, error) {
	var bindings []model.ConnectedAppTokenBinding
	if err := model.DB.Where("app_id = ?", appID).Find(&bindings).Error; err != nil {
		return nil, nil, err
	}
	attributions, err := model.ListConnectedAppTokenAttributionsByApp(appID)
	if err != nil {
		return nil, nil, err
	}

	tokenIDs := make([]int, 0, len(attributions)+len(bindings))
	tokensByID := make(map[int]connectedAppDeveloperUsageTokenMeta, len(attributions)+len(bindings))
	bindingsByID := make(map[int64]model.ConnectedAppTokenBinding, len(bindings))
	for _, binding := range bindings {
		bindingsByID[binding.Id] = binding
	}

	addToken := func(token connectedAppDeveloperUsageTokenMeta) {
		if token.TokenID <= 0 {
			return
		}
		if _, ok := tokensByID[token.TokenID]; ok {
			return
		}
		tokenIDs = append(tokenIDs, token.TokenID)
		tokensByID[token.TokenID] = token
	}

	for _, attribution := range attributions {
		status := "historical"
		if binding, ok := bindingsByID[attribution.BindingId]; ok && binding.TokenId == attribution.TokenId {
			status = binding.Status
		}
		addToken(connectedAppDeveloperUsageTokenMeta{
			TokenID: attribution.TokenId,
			UserID:  attribution.UserId,
			Status:  status,
			Device: snaplessDeviceInfo{
				Fingerprint: attribution.DeviceFingerprint,
				DeviceName:  attribution.DeviceName,
				Platform:    attribution.Platform,
				AppVersion:  attribution.AppVersion,
			},
		})
	}

	for _, binding := range bindings {
		addToken(connectedAppDeveloperUsageTokenMeta{
			TokenID: binding.TokenId,
			UserID:  binding.UserId,
			Status:  binding.Status,
			Device:  snaplessDeviceInfoFromBinding(&binding),
		})
	}
	return tokenIDs, tokensByID, nil
}

func emptyConnectedAppDeveloperUsage(app *model.ConnectedApp, filters connectedAppDeveloperUsageFilters, tokenCount int) connectedAppDeveloperUsageResponse {
	return connectedAppDeveloperUsageResponse{
		App:        buildSnaplessAppResponse(app),
		StartTime:  filters.StartTime,
		EndTime:    filters.EndTime,
		TokenCount: tokenCount,
		ByModel:    []connectedAppUsageByModel{},
		ByToken:    []connectedAppDeveloperTokenUsage{},
	}
}

func connectedAppDeveloperUsageLogQuery(filters connectedAppDeveloperUsageFilters, tokenIDs []int) *gorm.DB {
	query := model.LOG_DB.Model(&model.Log{}).
		Where("type = ? AND token_id IN ?", model.LogTypeConsume, tokenIDs)
	if filters.StartTime > 0 {
		query = query.Where("created_at >= ?", filters.StartTime)
	}
	if filters.EndTime > 0 {
		query = query.Where("created_at <= ?", filters.EndTime)
	}
	if filters.UserID > 0 {
		query = query.Where("user_id = ?", filters.UserID)
	}
	if filters.ModelName != "" {
		query = query.Where("model_name = ?", filters.ModelName)
	}
	return query
}

func connectedAppForDeveloper(c *gin.Context) (*model.ConnectedApp, bool, error) {
	app, err := connectedAppBySlugParam(c)
	if err != nil {
		return nil, false, err
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return nil, false, errors.New("connected app is disabled")
	}
	if app.AuthorizationFlow != model.ConnectedAppAuthorizationFlowDeviceCode {
		return nil, false, errors.New("connected app authorization_flow is not supported")
	}
	owner, err := connectedAppDeveloperAccess(c, app)
	if err != nil {
		return nil, false, err
	}
	return app, owner, nil
}

func connectedAppForPublicDeviceFlow(c *gin.Context) (*model.ConnectedApp, error) {
	app, err := connectedAppBySlugParam(c)
	if err != nil {
		return nil, err
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return nil, errors.New("connected app is disabled")
	}
	if !app.Trusted {
		return nil, errors.New("connected app is not trusted")
	}
	if app.AuthorizationFlow != model.ConnectedAppAuthorizationFlowDeviceCode {
		return nil, errors.New("connected app authorization_flow is not supported")
	}
	return app, nil
}

func connectedAppBySlugParam(c *gin.Context) (*model.ConnectedApp, error) {
	slug, err := normalizeConnectedAppSlug(c.Param("slug"))
	if err != nil {
		return nil, err
	}
	app, err := model.GetConnectedAppBySlug(slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("connected app not found")
		}
		return nil, err
	}
	return app, nil
}

func connectedAppDeveloperAccess(c *gin.Context, app *model.ConnectedApp) (bool, error) {
	if app == nil {
		return false, errors.New("connected app not found")
	}
	if c.GetInt("role") >= common.RoleAdminUser {
		return true, nil
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		return false, errors.New("login is required")
	}
	var count int64
	if err := model.DB.Model(&model.ConnectedAppRequest{}).
		Where("app_id = ? AND applicant_user_id = ? AND status = ?", app.Id, userID, model.ConnectedAppRequestStatusApproved).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, errors.New("connected app developer access is restricted to the approved applicant")
	}
	return true, nil
}

func connectedAppDeveloperDevicesByUser(appID int, userIDs []int) (map[int][]connectedAppDeveloperDevice, error) {
	result := make(map[int][]connectedAppDeveloperDevice, len(userIDs))
	if appID <= 0 || len(userIDs) == 0 {
		return result, nil
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		ids = append(ids, userID)
	}
	if len(ids) == 0 {
		return result, nil
	}

	var bindings []model.ConnectedAppTokenBinding
	if err := model.DB.Where("app_id = ? AND user_id IN ?", appID, ids).
		Order("status asc").
		Order("last_used_at desc").
		Order("updated_at desc").
		Order("id desc").
		Find(&bindings).Error; err != nil {
		return nil, err
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
		if err := model.DB.Where("id IN ?", tokenIDs).Find(&tokens).Error; err != nil {
			return nil, err
		}
		for i := range tokens {
			tokensByID[tokens[i].Id] = &tokens[i]
		}
	}
	for i := range bindings {
		binding := bindings[i]
		result[binding.UserId] = append(result[binding.UserId], connectedAppDeveloperDevice{
			Device:     snaplessDeviceInfoFromBinding(&binding),
			Token:      buildSnaplessTokenSummary(tokensByID[binding.TokenId], &binding),
			Status:     binding.Status,
			LastUsedAt: binding.LastUsedAt,
			RevokedAt:  binding.RevokedAt,
			CreatedAt:  binding.CreatedAt,
			UpdatedAt:  binding.UpdatedAt,
		})
	}
	return result, nil
}

func buildConnectedAppDeveloperSessionResponse(session *model.ConnectedAppDeviceSession, names map[int]string) connectedAppDeveloperSessionResponse {
	if session == nil {
		return connectedAppDeveloperSessionResponse{}
	}
	return connectedAppDeveloperSessionResponse{
		ID:           session.Id,
		Status:       session.Status,
		UserID:       session.UserId,
		UserName:     names[session.UserId],
		TokenID:      session.TokenId,
		TokenCreated: session.TokenCreated,
		Device:       snaplessDeviceInfoFromSession(session),
		ExpiresAt:    session.ExpiresAt,
		LastPolledAt: session.LastPolledAt,
		AuthorizedAt: session.AuthorizedAt,
		ConsumedAt:   session.ConsumedAt,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}
}

func connectedAppReadinessForUser(app *model.ConnectedApp, userID int) (snaplessReadinessResponse, error) {
	userHealth, err := getSnaplessUserHealth(userID)
	if err != nil {
		return snaplessReadinessResponse{}, err
	}
	status := evaluateSnaplessReadiness(app, userHealth, true)
	return snaplessReadinessResponse{
		OK:      status.OK,
		Status:  status.Status,
		Checks:  status.Checks,
		Actions: buildConnectedAppActionHints(status.Status, app),
	}, nil
}

func buildConnectedAppActionHints(status string, app *model.ConnectedApp) snaplessActionHints {
	appName := "connected app"
	if app != nil && strings.TrimSpace(app.Name) != "" {
		appName = strings.TrimSpace(app.Name)
	}
	switch status {
	case "ok":
		return snaplessActionHints{Severity: "success"}
	case "quota_insufficient":
		return snaplessActionHints{
			Severity: "warning",
			Reason:   "Your account balance is too low for " + appName + " requests.",
			Primary: &snaplessActionLink{
				Label:  "Recharge",
				Href:   "/wallet?source=connected-app",
				Intent: "recharge",
			},
		}
	case "user_disabled":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   "Your account is disabled. Review the account before approving access.",
			Primary: &snaplessActionLink{
				Label:  "Account settings",
				Href:   "/profile",
				Intent: "account",
			},
		}
	case "app_disabled":
		return snaplessActionHints{
			Severity: "danger",
			Reason:   appName + " is disabled.",
			Primary: &snaplessActionLink{
				Label:  "Open profile",
				Href:   "/profile",
				Intent: "connected_app",
			},
		}
	default:
		return snaplessActionHints{Severity: "neutral"}
	}
}

func ensureConnectedAppTokenForDeviceTx(c *gin.Context, tx *gorm.DB, app *model.ConnectedApp, userID int, device snaplessDeviceInfo, rotate bool) (connectedAppTokenResponse, int, error) {
	if app == nil {
		return connectedAppTokenResponse{}, 0, errors.New("connected app not found")
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return connectedAppTokenResponse{}, 0, errors.New("connected app is disabled")
	}
	if tx == nil {
		tx = model.DB
	}
	now := common.GetTimestamp()
	grant, err := model.UpsertConnectedAppGrant(tx, *app, userID, app.DefaultScopeList(), now)
	if err != nil {
		return connectedAppTokenResponse{}, 0, err
	}

	var existingBinding *model.ConnectedAppTokenBinding
	if binding, err := findSnaplessBindingTx(tx, app.Id, userID, device.Fingerprint); err == nil {
		existingBinding = binding
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return connectedAppTokenResponse{}, 0, err
	}

	if existingBinding != nil && !rotate {
		if token, err := getTokenByIdTx(tx, existingBinding.TokenId, userID); err == nil && snaplessTokenReusable(token, now) {
			if app.Slug == model.ConnectedAppSlugSnapless {
				if err := syncSnaplessTokenModelLimits(tx, token, strings.Join(getSnaplessModelAliases().All(), ",")); err != nil {
					return connectedAppTokenResponse{}, 0, err
				}
			}
			return buildConnectedAppTokenResponse(c, app, grant, existingBinding, token, device, "", false, false), token.Id, nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return connectedAppTokenResponse{}, 0, err
		}
	}

	if existingBinding != nil {
		if err := model.RecordConnectedAppTokenAttribution(tx, *existingBinding, now); err != nil {
			return connectedAppTokenResponse{}, 0, err
		}
		if err := model.DisableTokenWithTx(tx, existingBinding.TokenId, userID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return connectedAppTokenResponse{}, 0, err
		}
	}
	token, key, err := createConnectedAppToken(tx, app, userID, device, now)
	if err != nil {
		return connectedAppTokenResponse{}, 0, err
	}
	binding, err := model.UpsertConnectedAppTokenBinding(tx, model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            userID,
		TokenId:           token.Id,
		DeviceFingerprint: device.Fingerprint,
		DeviceName:        device.DeviceName,
		Platform:          device.Platform,
		AppVersion:        device.AppVersion,
	}, now)
	if err != nil {
		return connectedAppTokenResponse{}, 0, err
	}
	if err := model.RecordConnectedAppTokenAttribution(tx, *binding, now); err != nil {
		return connectedAppTokenResponse{}, 0, err
	}
	return buildConnectedAppTokenResponse(c, app, grant, binding, token, device, key, true, rotate && existingBinding != nil), token.Id, nil
}

func createConnectedAppToken(tx *gorm.DB, app *model.ConnectedApp, userID int, device snaplessDeviceInfo, now int64) (*model.Token, string, error) {
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

func buildConnectedAppTokenResponse(c *gin.Context, app *model.ConnectedApp, grant *model.ConnectedAppGrant, binding *model.ConnectedAppTokenBinding, token *model.Token, device snaplessDeviceInfo, key string, created bool, rotated bool) connectedAppTokenResponse {
	return connectedAppTokenResponse{
		App:        buildSnaplessAppResponse(app),
		Grant:      buildSnaplessGrantResponse(grant, app.DefaultScopeList()),
		Device:     device,
		Token:      buildSnaplessTokenSummary(token, binding),
		Endpoints:  connectedAppAPIEndpoints(c, app),
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

func connectedAppDeviceFromRequest(c *gin.Context, app *model.ConnectedApp, req snaplessDeviceRequest) snaplessDeviceInfo {
	appName := "Connected App"
	appSlug := "connected-app"
	if app != nil {
		appSlug = app.Slug
		if strings.TrimSpace(app.Name) != "" {
			appName = app.Name
		}
	}
	req.DeviceID = firstNonEmpty(req.DeviceID, c.Query("device_id"), c.GetHeader("X-Connected-App-Device-Id"), c.GetHeader("X-Snapless-Device-Id"))
	req.DeviceName = firstNonEmpty(req.DeviceName, c.Query("device_name"), c.GetHeader("X-Connected-App-Device-Name"), c.GetHeader("X-Snapless-Device-Name"), appName)
	req.Platform = firstNonEmpty(req.Platform, c.Query("platform"), c.GetHeader("X-Connected-App-Platform"), c.GetHeader("X-Snapless-Platform"), "desktop")
	req.AppVersion = firstNonEmpty(req.AppVersion, c.Query("app_version"), c.GetHeader("X-Connected-App-Version"), c.GetHeader("X-Snapless-App-Version"))
	req.Client = firstNonEmpty(req.Client, c.Query("client"), c.GetHeader("X-Connected-App-Client"), c.GetHeader("X-Snapless-Client"), c.GetHeader("User-Agent"), appSlug)

	deviceName := limitString(req.DeviceName, 128)
	platform := limitString(strings.ToLower(req.Platform), 32)
	appVersion := limitString(req.AppVersion, 64)
	client := limitString(req.Client, 64)

	seed := strings.TrimSpace(req.DeviceID)
	if seed != "" {
		seed = appSlug + ":device_id:" + strings.ToLower(seed)
	} else {
		seed = appSlug + ":derived:" + strings.ToLower(strings.Join([]string{deviceName, platform, client, c.GetHeader("User-Agent")}, "|"))
	}
	return snaplessDeviceInfo{
		Fingerprint: stableSnaplessFingerprint(seed),
		DeviceName:  deviceName,
		Platform:    platform,
		AppVersion:  appVersion,
		Client:      client,
	}
}

func connectedAppTokenName(app *model.ConnectedApp, device snaplessDeviceInfo) string {
	name := "Connected App"
	if app != nil && strings.TrimSpace(app.Name) != "" {
		name = strings.TrimSpace(app.Name)
	}
	if device.DeviceName != "" && device.DeviceName != name {
		name += " - " + device.DeviceName
	}
	return limitString(name, 50)
}

func connectedAppVerificationURI(c *gin.Context, app *model.ConnectedApp, userCode string) string {
	base := snaplessServerBaseURL(c)
	values := url.Values{}
	values.Set("user_code", normalizeSnaplessUserCode(userCode))
	if app != nil && app.Slug != model.ConnectedAppSlugSnapless {
		values.Set("app_slug", app.Slug)
	}
	return base + "/snapless/device?" + values.Encode()
}

func connectedAppDeviceFlowEndpoints(c *gin.Context, app *model.ConnectedApp) map[string]string {
	if app == nil {
		return map[string]string{}
	}
	base := snaplessServerBaseURL(c) + "/api/connected-apps/" + url.PathEscape(app.Slug)
	return map[string]string{
		"start":     base + "/device/start",
		"status":    base + "/device/status",
		"authorize": base + "/device/authorize",
		"poll":      base + "/device/poll",
	}
}

func connectedAppAPIEndpoints(c *gin.Context, app *model.ConnectedApp) map[string]string {
	if app == nil {
		return map[string]string{}
	}
	baseURL := snaplessAPIBaseURL(c)
	endpoints := map[string]string{}
	for _, scope := range app.ScopeList() {
		switch scope {
		case "openai.models":
			endpoints["models"] = baseURL + "/models"
		case "openai.chat":
			endpoints["chat_completions"] = baseURL + "/chat/completions"
		case "openai.audio.transcriptions":
			endpoints["audio_transcriptions"] = baseURL + "/audio/transcriptions"
		case "quota.read":
			endpoints["token_usage"] = snaplessServerBaseURL(c) + "/api/usage/token"
		}
	}
	keys := make([]string, 0, len(endpoints))
	for key := range endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(endpoints))
	for _, key := range keys {
		ordered[key] = endpoints[key]
	}
	return ordered
}
