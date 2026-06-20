package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type hStationFlowTestProvider struct {
	enabled        bool
	user           *oauth.OAuthUser
	tokenErr       error
	userErr        error
	exchangeCalled bool
}

func (p *hStationFlowTestProvider) GetName() string {
	return "H 站"
}

func (p *hStationFlowTestProvider) IsEnabled() bool {
	return p.enabled
}

func (p *hStationFlowTestProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*oauth.OAuthToken, error) {
	p.exchangeCalled = true
	if p.tokenErr != nil {
		return nil, p.tokenErr
	}
	return &oauth.OAuthToken{AccessToken: "access-token", TokenType: "Bearer"}, nil
}

func (p *hStationFlowTestProvider) GetUserInfo(ctx context.Context, token *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	if p.userErr != nil {
		return nil, p.userErr
	}
	return p.user, nil
}

func (p *hStationFlowTestProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsHStationIdAlreadyTaken(providerUserID)
}

func (p *hStationFlowTestProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.HStationId = providerUserID
	return user.FillUserByHStationId()
}

func (p *hStationFlowTestProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.HStationId = providerUserID
}

func (p *hStationFlowTestProvider) GetProviderPrefix() string {
	return "hstation_"
}

type oauthControllerResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func setupOAuthControllerTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))
	require.NoError(t, i18n.Init())

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRegisterEnabled := common.RegisterEnabled
	originalQuotaForNewUser := common.QuotaForNewUser

	model.DB = db
	model.LOG_DB = db
	common.RegisterEnabled = true
	common.QuotaForNewUser = 0

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RegisterEnabled = originalRegisterEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func registerHStationFlowTestProvider(t *testing.T, provider *hStationFlowTestProvider) {
	t.Helper()
	original := oauth.GetProvider("hstation")
	oauth.Register("hstation", provider)
	t.Cleanup(func() {
		if original != nil {
			oauth.Register("hstation", original)
		} else {
			oauth.Unregister("hstation")
		}
	})
}

func newOAuthControllerTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("oauth-controller-test"))))
	router.GET("/test/oauth-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", c.Query("state"))
		if userID := c.Query("user_id"); userID != "" {
			id, err := strconv.Atoi(userID)
			require.NoError(t, err)
			session.Set("id", id)
			session.Set("username", c.Query("username"))
			session.Set("role", common.RoleCommonUser)
			session.Set("status", common.UserStatusEnabled)
			session.Set("group", "default")
		}
		require.NoError(t, session.Save())
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	router.GET("/oauth/:provider", HandleOAuth)
	router.DELETE("/user/bindings/:binding_type", func(c *gin.Context) {
		id, err := strconv.Atoi(c.GetHeader("X-Test-User-Id"))
		require.NoError(t, err)
		c.Set("id", id)
		ClearSelfUserBinding(c)
	})
	return router
}

func seedOAuthSession(t *testing.T, router *gin.Engine, state string, user *model.User) []*http.Cookie {
	t.Helper()
	target := "/test/oauth-session?state=" + state
	if user != nil {
		target += "&user_id=" + strconv.Itoa(user.Id) + "&username=" + user.Username
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Result().Cookies()
}

func performOAuthRequest(t *testing.T, router *gin.Engine, target string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeOAuthControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) oauthControllerResponse {
	t.Helper()
	var response oauthControllerResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestHandleHStationOAuthRegistersNewUserAndLogsIn(t *testing.T) {
	setupOAuthControllerTestDB(t)
	provider := &hStationFlowTestProvider{
		enabled: true,
		user: &oauth.OAuthUser{
			ProviderUserID: "hs-new-user",
			Username:       "stationuser",
			DisplayName:    "Station User",
			Email:          "station@example.com",
		},
	}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-new", nil)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-new&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.Equal(t, "stationuser", response.Data["username"])

	var user model.User
	require.NoError(t, model.DB.Where("h_station_id = ?", "hs-new-user").First(&user).Error)
	require.Equal(t, "station@example.com", user.Email)
	require.Equal(t, "Station User", user.DisplayName)
}

func TestHandleHStationOAuthLogsInExistingUser(t *testing.T) {
	setupOAuthControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:         10,
		Username:   "existing",
		Status:     common.UserStatusEnabled,
		Role:       common.RoleCommonUser,
		HStationId: "hs-existing",
		AffCode:    "aff-existing",
	}).Error)
	provider := &hStationFlowTestProvider{
		enabled: true,
		user:    &oauth.OAuthUser{ProviderUserID: "hs-existing", Username: "ignored"},
	}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-existing", nil)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-existing&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.Equal(t, float64(10), response.Data["id"])
	require.Equal(t, "existing", response.Data["username"])
}

func TestHandleHStationOAuthBindUpdatesCurrentUser(t *testing.T) {
	setupOAuthControllerTestDB(t)
	currentUser := &model.User{Id: 20, Username: "current", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "aff-current-bind"}
	require.NoError(t, model.DB.Create(currentUser).Error)
	provider := &hStationFlowTestProvider{
		enabled: true,
		user:    &oauth.OAuthUser{ProviderUserID: "hs-bound", Username: "stationuser"},
	}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-bind", currentUser)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-bind&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)
	require.Equal(t, "bind", response.Data["action"])

	var user model.User
	require.NoError(t, model.DB.First(&user, currentUser.Id).Error)
	require.Equal(t, "hs-bound", user.HStationId)
}

func TestHandleHStationOAuthBindRejectsDuplicateProviderUser(t *testing.T) {
	setupOAuthControllerTestDB(t)
	currentUser := &model.User{Id: 30, Username: "current", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "aff-current-duplicate"}
	require.NoError(t, model.DB.Create(currentUser).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 31, Username: "other", Status: common.UserStatusEnabled, HStationId: "hs-taken", AffCode: "aff-other-duplicate"}).Error)
	provider := &hStationFlowTestProvider{
		enabled: true,
		user:    &oauth.OAuthUser{ProviderUserID: "hs-taken", Username: "stationuser"},
	}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-duplicate", currentUser)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-duplicate&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, currentUser.Id).Error)
	require.Empty(t, user.HStationId)
}

func TestHandleHStationOAuthReturnsProviderCancelMessage(t *testing.T) {
	setupOAuthControllerTestDB(t)
	provider := &hStationFlowTestProvider{enabled: true}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-cancel", nil)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-cancel&error=access_denied&error_description=user_cancelled", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, response.Success)
	require.Equal(t, "user_cancelled", response.Message)
	require.False(t, provider.exchangeCalled)
}

func TestHandleHStationOAuthRejectsInvalidState(t *testing.T) {
	setupOAuthControllerTestDB(t)
	provider := &hStationFlowTestProvider{enabled: true}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "expected-state", nil)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=wrong-state&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, response.Success)
	require.False(t, provider.exchangeCalled)
}

func TestHandleHStationOAuthReturnsTokenError(t *testing.T) {
	setupOAuthControllerTestDB(t)
	provider := &hStationFlowTestProvider{
		enabled:  true,
		tokenErr: errors.New("token exchange failed"),
	}
	registerHStationFlowTestProvider(t, provider)
	router := newOAuthControllerTestRouter(t)
	cookies := seedOAuthSession(t, router, "state-token-error", nil)

	recorder := performOAuthRequest(t, router, "/oauth/hstation?state=state-token-error&code=oauth-code", cookies)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, response.Success)
	require.Equal(t, "token exchange failed", response.Message)
}

func TestClearSelfUserBindingClearsHStationBinding(t *testing.T) {
	setupOAuthControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:         40,
		Username:   "bound-user",
		Status:     common.UserStatusEnabled,
		Role:       common.RoleCommonUser,
		HStationId: "hs-to-clear",
		AffCode:    "aff-clear",
	}).Error)
	router := newOAuthControllerTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/user/bindings/hstation", nil)
	req.Header.Set("X-Test-User-Id", "40")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	response := decodeOAuthControllerResponse(t, recorder)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 40).Error)
	require.Empty(t, user.HStationId)
}
