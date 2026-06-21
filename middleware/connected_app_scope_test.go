package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	connectedAppScopeTestUserId  = 9301
	connectedAppScopeTestTokenId = 9401
)

func TestConnectedAppScopeAuthAllowsGrantedScopesAndTouchesUsage(t *testing.T) {
	setupConnectedAppScopeMiddlewareTestDB(t)
	seedConnectedAppScopeBinding(t, "openai.models openai.chat quota.read", "openai.models openai.chat quota.read")

	chat := performConnectedAppScopeRequest(t, http.MethodPost, "/v1/chat/completions", "/v1/chat/completions", connectedAppScopeTestTokenId, connectedAppScopeTestUserId)
	require.Equal(t, http.StatusNoContent, chat.Code, chat.Body.String())

	usage := performConnectedAppScopeRequest(t, http.MethodGet, "/api/usage/token/", "/api/usage/token/", connectedAppScopeTestTokenId, connectedAppScopeTestUserId)
	require.Equal(t, http.StatusNoContent, usage.Code, usage.Body.String())

	var binding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("token_id = ?", connectedAppScopeTestTokenId).First(&binding).Error)
	require.Greater(t, binding.LastUsedAt, int64(0))

	var grant model.ConnectedAppGrant
	require.NoError(t, model.DB.Where("id = ?", binding.GrantId).First(&grant).Error)
	require.Greater(t, grant.LastUsedAt, int64(0))
}

func TestConnectedAppScopeAuthRejectsMissingScope(t *testing.T) {
	tests := []struct {
		name        string
		appScopes   string
		grantScopes string
	}{
		{
			name:        "missing app scope",
			appScopes:   "openai.chat",
			grantScopes: "openai.chat openai.audio.transcriptions",
		},
		{
			name:        "missing grant scope",
			appScopes:   "openai.chat openai.audio.transcriptions",
			grantScopes: "openai.chat",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			setupConnectedAppScopeMiddlewareTestDB(t)
			seedConnectedAppScopeBinding(t, tt.appScopes, tt.grantScopes)

			recorder := performConnectedAppScopeRequest(t, http.MethodPost, "/v1/audio/transcriptions", "/v1/audio/transcriptions", connectedAppScopeTestTokenId, connectedAppScopeTestUserId)

			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), "openai.audio.transcriptions")
		})
	}
}

func TestConnectedAppScopeAuthRejectsInactiveConnectedAppState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, app model.ConnectedApp, grant model.ConnectedAppGrant, binding model.ConnectedAppTokenBinding)
		want   string
	}{
		{
			name: "disabled app",
			mutate: func(t *testing.T, app model.ConnectedApp, _ model.ConnectedAppGrant, _ model.ConnectedAppTokenBinding) {
				t.Helper()
				require.NoError(t, model.DB.Model(&model.ConnectedApp{}).
					Where("id = ?", app.Id).
					Update("status", model.ConnectedAppStatusDisabled).Error)
			},
			want: "connected app is disabled",
		},
		{
			name: "revoked grant",
			mutate: func(t *testing.T, _ model.ConnectedApp, grant model.ConnectedAppGrant, _ model.ConnectedAppTokenBinding) {
				t.Helper()
				require.NoError(t, model.DB.Model(&model.ConnectedAppGrant{}).
					Where("id = ?", grant.Id).
					Update("status", model.ConnectedAppGrantStatusRevoked).Error)
			},
			want: "connected app grant is not authorized",
		},
		{
			name: "revoked binding",
			mutate: func(t *testing.T, _ model.ConnectedApp, _ model.ConnectedAppGrant, binding model.ConnectedAppTokenBinding) {
				t.Helper()
				require.NoError(t, model.DB.Model(&model.ConnectedAppTokenBinding{}).
					Where("id = ?", binding.Id).
					Update("status", model.ConnectedAppTokenBindingStatusRevoked).Error)
			},
			want: "connected app token binding is not active",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			setupConnectedAppScopeMiddlewareTestDB(t)
			app, grant, binding := seedConnectedAppScopeBinding(t, "openai.chat", "openai.chat")
			tt.mutate(t, app, grant, binding)

			recorder := performConnectedAppScopeRequest(t, http.MethodPost, "/v1/chat/completions", "/v1/chat/completions", connectedAppScopeTestTokenId, connectedAppScopeTestUserId)

			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), tt.want)
		})
	}
}

func TestConnectedAppScopeAuthRejectsUnmappedEndpointForConnectedAppToken(t *testing.T) {
	setupConnectedAppScopeMiddlewareTestDB(t)
	seedConnectedAppScopeBinding(t, "openai.chat", "openai.chat")

	recorder := performConnectedAppScopeRequest(t, http.MethodPost, "/v1/completions", "/v1/completions", connectedAppScopeTestTokenId, connectedAppScopeTestUserId)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "not allowed to access this endpoint")
}

func TestConnectedAppScopeAuthAllowsRegularTokenWithoutBinding(t *testing.T) {
	setupConnectedAppScopeMiddlewareTestDB(t)

	recorder := performConnectedAppScopeRequest(t, http.MethodPost, "/v1/completions", "/v1/completions", 9901, connectedAppScopeTestUserId)

	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}

func setupConnectedAppScopeMiddlewareTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite

	common.RedisEnabled = false
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(
		&model.ConnectedApp{},
		&model.ConnectedAppGrant{},
		&model.ConnectedAppTokenBinding{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
	})
}

func seedConnectedAppScopeBinding(t *testing.T, appScopes string, grantScopes string) (model.ConnectedApp, model.ConnectedAppGrant, model.ConnectedAppTokenBinding) {
	t.Helper()

	now := common.GetTimestamp()
	app := model.ConnectedApp{
		Slug:              "scope-test-app",
		Name:              "Scope Test App",
		AllowedScopes:     appScopes,
		DefaultScopes:     grantScopes,
		AuthorizationFlow: model.ConnectedAppAuthorizationFlowDeviceCode,
		Status:            model.ConnectedAppStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&app).Error)
	grant := model.ConnectedAppGrant{
		AppId:        app.Id,
		UserId:       connectedAppScopeTestUserId,
		Scopes:       grantScopes,
		Status:       model.ConnectedAppGrantStatusAuthorized,
		AuthorizedAt: now,
	}
	require.NoError(t, model.DB.Create(&grant).Error)
	binding := model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            connectedAppScopeTestUserId,
		TokenId:           connectedAppScopeTestTokenId,
		DeviceFingerprint: "scope-test-device",
		DeviceName:        "Scope Test Device",
		Status:            model.ConnectedAppTokenBindingStatusActive,
	}
	require.NoError(t, model.DB.Create(&binding).Error)
	return app, grant, binding
}

func performConnectedAppScopeRequest(t *testing.T, method string, target string, route string, tokenId int, userId int) *httptest.ResponseRecorder {
	t.Helper()

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("token_id", tokenId)
		c.Set("id", userId)
		c.Next()
	})
	engine.Use(ConnectedAppScopeAuth())
	engine.Handle(method, route, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(""))
	engine.ServeHTTP(recorder, request)
	return recorder
}
