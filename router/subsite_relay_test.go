package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSubsiteRelayStatusGate(t *testing.T) {
	setupSubsiteRelayRouterTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "open-site",
		Name:   "Open Site",
		Status: model.SubsiteStatusEnabled,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "closed-site",
		Name:   "Closed Site",
		Status: model.SubsiteStatusDisabled,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "ended-site",
		Name:   "Ended Site",
		Status: model.SubsiteStatusEnabled,
		EndsAt: now - 1,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:     "soon-site",
		Name:     "Soon Site",
		Status:   model.SubsiteStatusEnabled,
		StartsAt: now + 3600,
	}))

	engine := gin.New()
	SetRelayRouter(engine)

	closed := performSubsiteRelayRequest(engine, "closed-site")
	require.Equal(t, http.StatusForbidden, closed.Code, closed.Body.String())
	require.Contains(t, closed.Body.String(), model.SubsiteAccessCodeDisabled)
	require.NotContains(t, closed.Body.String(), "invalid")

	ended := performSubsiteRelayRequest(engine, "ended-site")
	require.Equal(t, http.StatusForbidden, ended.Code, ended.Body.String())
	require.Contains(t, ended.Body.String(), model.SubsiteAccessCodeExpired)

	soon := performSubsiteRelayRequest(engine, "soon-site")
	require.Equal(t, http.StatusForbidden, soon.Code, soon.Body.String())
	require.Contains(t, soon.Body.String(), model.SubsiteAccessCodeNotStarted)

	missing := performSubsiteRelayRequest(engine, "missing-site")
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), model.SubsiteAccessCodeNotFound)

	open := performSubsiteRelayRequest(engine, "open-site")
	require.Equal(t, http.StatusUnauthorized, open.Code, open.Body.String())
	require.NotContains(t, open.Body.String(), model.SubsiteAccessCodeAPINotReady)
}

func TestSubsiteRelayTokenScopeIsolation(t *testing.T) {
	setupSubsiteRelayRouterTestDB(t)

	siteA := createRelayTestSubsite(t, "site-a")
	siteB := createRelayTestSubsite(t, "site-b")
	createRelayTestUser(t, 7001)
	createRelayTestToken(t, "mainrelaytoken", 7001, 0)
	createRelayTestToken(t, "sitearelaytoken", 7001, siteA.Id)
	createRelayTestToken(t, "sitebrelaytoken", 7001, siteB.Id)

	engine := gin.New()
	relay := engine.Group("/s/:slug/v1")
	relay.Use(middleware.SubsiteContext(true))
	relay.Use(middleware.TokenAuth())
	relay.Use(middleware.SubsiteTokenScopeAuth())
	relay.POST("/chat/completions", func(c *gin.Context) {
		require.Equal(t, siteA.Id, c.GetInt64(middleware.SubsiteIDContextKey))
		require.Equal(t, siteA.Id, c.GetInt64("token_subsite_id"))
		c.Status(http.StatusNoContent)
	})

	gemini := engine.Group("/s/:slug/v1beta")
	gemini.Use(middleware.SubsiteContext(true))
	gemini.Use(middleware.TokenAuth())
	gemini.Use(middleware.SubsiteTokenScopeAuth())
	gemini.GET("/models", func(c *gin.Context) {
		require.Equal(t, siteA.Id, c.GetInt64(middleware.SubsiteIDContextKey))
		require.Equal(t, siteA.Id, c.GetInt64("token_subsite_id"))
		c.Status(http.StatusNoContent)
	})

	mainKey := performSubsiteRelayRequestWithToken(engine, "site-a", "mainrelaytoken")
	require.Equal(t, http.StatusForbidden, mainKey.Code, mainKey.Body.String())
	require.Contains(t, mainKey.Body.String(), model.SubsiteAccessCodeTokenScope)

	wrongSubsiteKey := performSubsiteRelayRequestWithToken(engine, "site-a", "sitebrelaytoken")
	require.Equal(t, http.StatusForbidden, wrongSubsiteKey.Code, wrongSubsiteKey.Body.String())
	require.Contains(t, wrongSubsiteKey.Body.String(), model.SubsiteAccessCodeTokenScope)

	siteKey := performSubsiteRelayRequestWithToken(engine, "site-a", "sitearelaytoken")
	require.Equal(t, http.StatusNoContent, siteKey.Code, siteKey.Body.String())

	mainQueryKey := performSubsiteRelayRawRequest(engine, http.MethodGet, "/s/site-a/v1beta/models?key=sk-mainrelaytoken", "")
	require.Equal(t, http.StatusForbidden, mainQueryKey.Code, mainQueryKey.Body.String())
	require.Contains(t, mainQueryKey.Body.String(), model.SubsiteAccessCodeTokenScope)

	wrongSubsiteQueryKey := performSubsiteRelayRawRequest(engine, http.MethodGet, "/s/site-a/v1beta/models?key=sk-sitebrelaytoken", "")
	require.Equal(t, http.StatusForbidden, wrongSubsiteQueryKey.Code, wrongSubsiteQueryKey.Body.String())
	require.Contains(t, wrongSubsiteQueryKey.Body.String(), model.SubsiteAccessCodeTokenScope)

	siteQueryKey := performSubsiteRelayRawRequest(engine, http.MethodGet, "/s/site-a/v1beta/models?key=sk-sitearelaytoken", "")
	require.Equal(t, http.StatusNoContent, siteQueryKey.Code, siteQueryKey.Body.String())
}

func TestSubsiteGeminiNativeRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	require.True(t, registered[http.MethodGet+" /s/:slug/v1beta/models"])
	require.True(t, registered[http.MethodGet+" /s/:slug/v1beta/openai/models"])
	require.True(t, registered[http.MethodPost+" /s/:slug/v1beta/models/*path"])
}

func TestMainRelayRejectsSubsiteToken(t *testing.T) {
	setupSubsiteRelayRouterTestDB(t)

	site := createRelayTestSubsite(t, "main-scope-site")
	createRelayTestUser(t, 7002)
	createRelayTestToken(t, "subsiteonlytoken", 7002, site.Id)

	engine := gin.New()
	SetRelayRouter(engine)

	tests := []struct {
		name       string
		method     string
		path       string
		headerAuth bool
	}{
		{name: "openai relay", method: http.MethodPost, path: "/v1/chat/completions", headerAuth: true},
		{name: "openai models", method: http.MethodGet, path: "/v1/models", headerAuth: true},
		{name: "gemini models", method: http.MethodGet, path: "/v1beta/models", headerAuth: true},
		{name: "gemini models query key", method: http.MethodGet, path: "/v1beta/models?key=sk-subsiteonlytoken"},
		{name: "gemini openai-compatible models", method: http.MethodGet, path: "/v1beta/openai/models", headerAuth: true},
		{name: "gemini relay", method: http.MethodPost, path: "/v1beta/models/gemini-pro:generateContent", headerAuth: true},
		{name: "midjourney relay", method: http.MethodPost, path: "/mj/submit/imagine", headerAuth: true},
		{name: "mode midjourney relay", method: http.MethodPost, path: "/relax/mj/submit/imagine", headerAuth: true},
		{name: "suno relay", method: http.MethodPost, path: "/suno/submit/music", headerAuth: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.headerAuth {
				req.Header.Set("Authorization", "Bearer sk-subsiteonlytoken")
			}
			engine.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), model.SubsiteAccessCodeTokenScope)
		})
	}
}

func performSubsiteRelayRequest(engine *gin.Engine, slug string) *httptest.ResponseRecorder {
	return performSubsiteRelayRequestWithToken(engine, slug, "")
}

func performSubsiteRelayRequestWithToken(engine *gin.Engine, slug string, key string) *httptest.ResponseRecorder {
	return performSubsiteRelayRawRequest(engine, http.MethodPost, "/s/"+slug+"/v1/chat/completions", key)
}

func performSubsiteRelayRawRequest(engine *gin.Engine, method string, path string, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer sk-"+key)
	}
	engine.ServeHTTP(recorder, req)
	return recorder
}

func createRelayTestSubsite(t *testing.T, slug string) model.Subsite {
	t.Helper()
	subsite := model.Subsite{
		Slug:   slug,
		Name:   slug,
		Status: model.SubsiteStatusEnabled,
	}
	require.NoError(t, model.CreateSubsite(&subsite))
	return subsite
}

func createRelayTestUser(t *testing.T, userId int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: "relay-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "relay-user-aff",
	}).Error)
}

func createRelayTestToken(t *testing.T, key string, userId int, subsiteId int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Token{
		SubsiteId:      subsiteId,
		UserId:         userId,
		Key:            key,
		Name:           key,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
}

func setupSubsiteRelayRouterTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite

	common.RedisEnabled = false
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Subsite{}, &model.User{}, &model.Token{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
	})
}
