package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const snaplessRouterTestUserId = 8301

type snaplessAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type snaplessTokenData struct {
	APIKey     string `json:"api_key"`
	Created    bool   `json:"created"`
	Rotated    bool   `json:"rotated"`
	APIKeyOnce bool   `json:"api_key_once"`
	BaseURL    string `json:"base_url"`
	Grant      struct {
		Status string   `json:"status"`
		Scopes []string `json:"scopes"`
	} `json:"grant"`
	Token struct {
		ID                    int    `json:"id"`
		Status                int    `json:"status"`
		UnlimitedQuota        bool   `json:"unlimited_quota"`
		QuotaHardLimitEnabled bool   `json:"quota_hard_limit_enabled"`
		ModelLimitsEnabled    bool   `json:"model_limits_enabled"`
		ModelLimits           string `json:"model_limits"`
	} `json:"token"`
	Device struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"device"`
}

type snaplessRevokeData struct {
	Revoked      bool `json:"revoked"`
	TokenID      int  `json:"token_id"`
	GrantRevoked bool `json:"grant_revoked"`
}

type snaplessHealthData struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
	Token  struct {
		ID              int    `json:"id"`
		Enabled         bool   `json:"enabled"`
		QuotaOK         bool   `json:"quota_ok"`
		SnaplessBinding bool   `json:"snapless_binding"`
		BindingStatus   string `json:"binding_status"`
	} `json:"token"`
	User struct {
		ID      int  `json:"id"`
		Enabled bool `json:"enabled"`
		QuotaOK bool `json:"quota_ok"`
	} `json:"user"`
	Checks map[string]bool `json:"checks"`
}

type snaplessDeviceStartData struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int    `json:"interval"`
	App             struct {
		Slug string `json:"slug"`
	} `json:"app"`
	Device struct {
		Fingerprint string `json:"fingerprint"`
		DeviceName  string `json:"device_name"`
		Platform    string `json:"platform"`
		AppVersion  string `json:"app_version"`
	} `json:"device"`
}

type snaplessDeviceStatusData struct {
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
	Token     struct {
		ID int `json:"id"`
	} `json:"token"`
	Device struct {
		Fingerprint string `json:"fingerprint"`
	} `json:"device"`
}

type snaplessDevicePollStatusData struct {
	Status   string `json:"status"`
	Interval int    `json:"interval"`
}

func setupSnaplessRouterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalServerAddress := system_setting.ServerAddress
	originalOptionMap := common.OptionMap

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.OptionMap = map[string]string{
		"SnaplessModels": `{"asr":"snapless-asr","chat":"snapless-polish","polish":"snapless-polish","translate":"snapless-translate","qa":"snapless-qa"}`,
	}
	system_setting.ServerAddress = "https://data-proxy.test"

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.ConnectedApp{},
		&model.ConnectedAppGrant{},
		&model.ConnectedAppTokenBinding{},
		&model.ConnectedAppDeviceSession{},
	))
	require.NoError(t, model.EnsureBuiltinConnectedApps())
	seedSnaplessRouterUserAndAbilities(t)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		system_setting.ServerAddress = originalServerAddress
		common.OptionMap = originalOptionMap
	})
	return db
}

func seedSnaplessRouterUserAndAbilities(t *testing.T) {
	t.Helper()

	require.NoError(t, model.DB.Create(&model.User{
		Id:       snaplessRouterTestUserId,
		Username: "snapless-user",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    100000,
		AffCode:  "snapless-user",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "upstream-key",
		Status: common.ChannelStatusEnabled,
		Name:   "snapless-channel",
		Models: "snapless-asr,snapless-polish,snapless-translate,snapless-qa",
		Group:  "default",
	}).Error)
	for _, modelName := range []string{"snapless-asr", "snapless-polish", "snapless-translate", "snapless-qa"} {
		require.NoError(t, model.DB.Create(&model.Ability{
			Group:     "default",
			Model:     modelName,
			ChannelId: 1,
			Enabled:   true,
		}).Error)
	}
}

func newSnaplessRouterForTest(t *testing.T) *gin.Engine {
	t.Helper()
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("snapless-router-test"))))
	engine.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "snapless-user")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", snaplessRouterTestUserId)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)
	return engine
}

func loginSnaplessRouterUser(t *testing.T, router *gin.Engine) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func requestSnaplessRouter(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie, auth string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		request.Header.Set("Authorization", auth)
	}
	if len(cookies) > 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(snaplessRouterTestUserId))
	}
	for _, item := range cookies {
		request.AddCookie(item)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeSnaplessData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response snaplessAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, "unexpected response: %s", recorder.Body.String())
	var data T
	require.NoError(t, common.Unmarshal(response.Data, &data))
	return data
}

func TestSnaplessEnsureCreatesReusableConnectedAppToken(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newSnaplessRouterForTest(t)
	cookies := loginSnaplessRouterUser(t, router)
	body := `{"device_id":"macbook-1","device_name":"Alice Mac","platform":"macos","app_version":"1.0.0"}`

	first := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/tokens/ensure", body, cookies, ""))
	require.True(t, first.Created)
	require.False(t, first.Rotated)
	require.True(t, first.APIKeyOnce)
	require.True(t, strings.HasPrefix(first.APIKey, "sk-"))
	require.Equal(t, "https://data-proxy.test/v1", first.BaseURL)
	require.Equal(t, model.ConnectedAppGrantStatusAuthorized, first.Grant.Status)
	require.Contains(t, first.Grant.Scopes, "token.manage")
	require.Equal(t, common.TokenStatusEnabled, first.Token.Status)
	require.True(t, first.Token.UnlimitedQuota)
	require.False(t, first.Token.QuotaHardLimitEnabled)
	require.True(t, first.Token.ModelLimitsEnabled)
	require.ElementsMatch(t, []string{"snapless-asr", "snapless-polish", "snapless-translate", "snapless-qa"}, strings.Split(first.Token.ModelLimits, ","))

	rawKey := strings.TrimPrefix(first.APIKey, "sk-")
	var stored model.Token
	require.NoError(t, model.DB.Where("id = ?", first.Token.ID).First(&stored).Error)
	require.Equal(t, rawKey, stored.Key)

	var binding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("token_id = ?", first.Token.ID).First(&binding).Error)
	require.Equal(t, first.Device.Fingerprint, binding.DeviceFingerprint)
	require.Equal(t, model.ConnectedAppTokenBindingStatusActive, binding.Status)

	second := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/tokens/ensure", body, cookies, ""))
	require.False(t, second.Created)
	require.False(t, second.APIKeyOnce)
	require.Empty(t, second.APIKey)
	require.Equal(t, first.Token.ID, second.Token.ID)
}

func TestSnaplessBuiltinAppPreservesStatus(t *testing.T) {
	setupSnaplessRouterTestDB(t)

	require.NoError(t, model.DB.Model(&model.ConnectedApp{}).
		Where("slug = ?", model.ConnectedAppSlugSnapless).
		Update("status", model.ConnectedAppStatusDisabled).Error)
	require.NoError(t, model.EnsureBuiltinConnectedApps())

	app, err := model.GetConnectedAppBySlug(model.ConnectedAppSlugSnapless)
	require.NoError(t, err)
	require.Equal(t, model.ConnectedAppStatusDisabled, app.Status)
}

func TestSnaplessDeviceFlowAuthorizesAndReturnsKeyOnce(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newSnaplessRouterForTest(t)
	body := `{"device_id":"mac-device","device_name":"Alice Mac","platform":"macos","app_version":"1.1.0"}`

	started := decodeSnaplessData[snaplessDeviceStartData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/device/start", body, nil, ""))
	require.NotEmpty(t, started.DeviceCode)
	require.NotEmpty(t, started.UserCode)
	require.Contains(t, started.VerificationURI, "/snapless/device?user_code=")
	require.Equal(t, 3, started.Interval)
	require.Equal(t, model.ConnectedAppSlugSnapless, started.App.Slug)
	require.Equal(t, "Alice Mac", started.Device.DeviceName)
	require.Equal(t, "macos", started.Device.Platform)
	require.Equal(t, "1.1.0", started.Device.AppVersion)

	pending := decodeSnaplessData[snaplessDevicePollStatusData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/device/poll", fmt.Sprintf(`{"device_code":%q}`, started.DeviceCode), nil, ""))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, pending.Status)
	require.Equal(t, 3, pending.Interval)

	cookies := loginSnaplessRouterUser(t, router)
	status := decodeSnaplessData[snaplessDeviceStatusData](t, requestSnaplessRouter(t, router, http.MethodGet, "/api/snapless/device/status?user_code="+started.UserCode, "", cookies, ""))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, status.Status)
	require.Equal(t, started.Device.Fingerprint, status.Device.Fingerprint)

	authorized := decodeSnaplessData[snaplessDeviceStatusData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/device/authorize", fmt.Sprintf(`{"user_code":%q,"approve":true}`, started.UserCode), cookies, ""))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusAuthorized, authorized.Status)
	require.NotZero(t, authorized.Token.ID)

	firstPoll := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/device/poll", fmt.Sprintf(`{"device_code":%q}`, started.DeviceCode), nil, ""))
	require.True(t, firstPoll.Created)
	require.True(t, firstPoll.APIKeyOnce)
	require.True(t, strings.HasPrefix(firstPoll.APIKey, "sk-"))
	require.Equal(t, authorized.Token.ID, firstPoll.Token.ID)
	require.ElementsMatch(t, []string{"snapless-asr", "snapless-polish", "snapless-translate", "snapless-qa"}, strings.Split(firstPoll.Token.ModelLimits, ","))

	secondPoll := decodeSnaplessData[snaplessDevicePollStatusData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/device/poll", fmt.Sprintf(`{"device_code":%q}`, started.DeviceCode), nil, ""))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusConsumed, secondPoll.Status)
	require.Equal(t, 3, secondPoll.Interval)

	health := decodeSnaplessData[snaplessHealthData](t, requestSnaplessRouter(t, router, http.MethodGet, "/api/snapless/health", "", nil, "Bearer "+firstPoll.APIKey))
	require.True(t, health.OK)
	require.Equal(t, "ok", health.Status)
	require.True(t, health.Token.SnaplessBinding)
}

func TestSnaplessRotateAndHealth(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newSnaplessRouterForTest(t)
	cookies := loginSnaplessRouterUser(t, router)
	body := `{"device_id":"macbook-rotate","device_name":"Alice Mac","platform":"macos","app_version":"1.0.0"}`

	first := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/tokens/ensure", body, cookies, ""))
	rotated := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/tokens/rotate", body, cookies, ""))
	require.True(t, rotated.Created)
	require.True(t, rotated.Rotated)
	require.NotEqual(t, first.Token.ID, rotated.Token.ID)
	require.NotEqual(t, first.APIKey, rotated.APIKey)

	var oldToken model.Token
	require.NoError(t, model.DB.Where("id = ?", first.Token.ID).First(&oldToken).Error)
	require.Equal(t, common.TokenStatusDisabled, oldToken.Status)

	var binding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("device_fingerprint = ?", rotated.Device.Fingerprint).First(&binding).Error)
	require.Equal(t, rotated.Token.ID, binding.TokenId)

	health := decodeSnaplessData[snaplessHealthData](t, requestSnaplessRouter(t, router, http.MethodGet, "/api/snapless/health", "", nil, "Bearer "+rotated.APIKey))
	require.True(t, health.OK)
	require.Equal(t, "ok", health.Status)
	require.True(t, health.Token.SnaplessBinding)
	require.True(t, health.User.Enabled)
	require.True(t, health.User.QuotaOK)
	require.True(t, health.Checks["models_ready"])

	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", rotated.Token.ID).Update("status", common.TokenStatusDisabled).Error)
	disabledHealth := decodeSnaplessData[snaplessHealthData](t, requestSnaplessRouter(t, router, http.MethodGet, "/api/snapless/health", "", nil, "Bearer "+rotated.APIKey))
	require.False(t, disabledHealth.OK)
	require.Equal(t, "token_disabled", disabledHealth.Status)
}

func TestSnaplessRevokeCurrentTokenRevokesGrantWhenLastDevice(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newSnaplessRouterForTest(t)
	cookies := loginSnaplessRouterUser(t, router)
	body := `{"device_id":"macbook-revoke","device_name":"Alice Mac","platform":"macos","app_version":"1.0.0"}`

	created := decodeSnaplessData[snaplessTokenData](t, requestSnaplessRouter(t, router, http.MethodPost, "/api/snapless/tokens/ensure", body, cookies, ""))
	revoked := decodeSnaplessData[snaplessRevokeData](t, requestSnaplessRouter(t, router, http.MethodDelete, "/api/snapless/tokens/current", body, cookies, ""))
	require.True(t, revoked.Revoked)
	require.True(t, revoked.GrantRevoked)
	require.Equal(t, created.Token.ID, revoked.TokenID)

	var token model.Token
	require.NoError(t, model.DB.Where("id = ?", created.Token.ID).First(&token).Error)
	require.Equal(t, common.TokenStatusDisabled, token.Status)

	var binding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("token_id = ?", created.Token.ID).First(&binding).Error)
	require.Equal(t, model.ConnectedAppTokenBindingStatusRevoked, binding.Status)

	var grant model.ConnectedAppGrant
	require.NoError(t, model.DB.Where("app_id = ? AND user_id = ?", binding.AppId, snaplessRouterTestUserId).First(&grant).Error)
	require.Equal(t, model.ConnectedAppGrantStatusRevoked, grant.Status)

	health := decodeSnaplessData[snaplessHealthData](t, requestSnaplessRouter(t, router, http.MethodGet, "/api/snapless/health", "", nil, "Bearer "+created.APIKey))
	require.False(t, health.OK)
	require.Equal(t, "token_disabled", health.Status)
}
