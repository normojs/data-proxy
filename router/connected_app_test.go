package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const connectedAppRouterAdminUserId = 8401

type connectedAppData struct {
	ID                int      `json:"id"`
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	AllowedScopes     []string `json:"allowed_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	Trusted           bool     `json:"trusted"`
	Status            int      `json:"status"`
	AuthorizationFlow string   `json:"authorization_flow"`
	GrantCount        int64    `json:"grant_count"`
	DeviceCount       int64    `json:"device_count"`
	ActiveDeviceCount int64    `json:"active_device_count"`
}

func newConnectedAppAdminRouterForTest(t *testing.T) *gin.Engine {
	t.Helper()

	require.NoError(t, model.DB.Create(&model.User{
		Id:       connectedAppRouterAdminUserId,
		Username: "connected-app-admin",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleAdminUser,
		Group:    "default",
		Quota:    100000,
		AffCode:  "connected-app-admin",
	}).Error)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("connected-app-router-test"))))
	engine.GET("/admin-login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "connected-app-admin")
		session.Set("role", common.RoleAdminUser)
		session.Set("id", connectedAppRouterAdminUserId)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)
	return engine
}

func loginConnectedAppAdmin(t *testing.T, router *gin.Engine) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin-login", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func requestConnectedAppAdmin(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if len(cookies) > 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(connectedAppRouterAdminUserId))
	}
	for _, item := range cookies {
		request.AddCookie(item)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeConnectedAppData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response snaplessAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, "unexpected response: %s", recorder.Body.String())
	var data T
	require.NoError(t, common.Unmarshal(response.Data, &data))
	return data
}

func decodeConnectedAppFailure(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response snaplessAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success, "unexpected response: %s", recorder.Body.String())
	return response.Message
}

func TestConnectedAppAdminCRUD(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	cookies := loginConnectedAppAdmin(t, router)

	list := decodeConnectedAppData[[]connectedAppData](t, requestConnectedAppAdmin(t, router, http.MethodGet, "/api/connected-apps", "", cookies))
	require.Len(t, list, 1)
	require.Equal(t, model.ConnectedAppSlugSnapless, list[0].Slug)
	require.Equal(t, model.ConnectedAppStatusEnabled, list[0].Status)
	require.True(t, list[0].Trusted)
	require.Equal(t, "device_code", list[0].AuthorizationFlow)
	require.Contains(t, list[0].AllowedScopes, "token.manage")

	created := decodeConnectedAppData[connectedAppData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"snapless-beta","name":"Snapless Beta","description":"Beta desktop app","allowed_scopes":["openai.chat","quota.read","openai.chat"],"default_scopes":["openai.chat"],"trusted":false,"status":1}`,
		cookies,
	))
	require.NotZero(t, created.ID)
	require.Equal(t, "snapless-beta", created.Slug)
	require.Equal(t, []string{"openai.chat", "quota.read"}, created.AllowedScopes)
	require.Equal(t, []string{"openai.chat"}, created.DefaultScopes)
	require.False(t, created.Trusted)
	require.Equal(t, model.ConnectedAppStatusEnabled, created.Status)

	updated := decodeConnectedAppData[connectedAppData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPut,
		"/api/connected-apps/"+strconv.Itoa(created.ID),
		`{"name":"Snapless Beta Managed","description":"Managed desktop app","allowed_scopes":["openai.chat","quota.read","token.manage"],"default_scopes":["quota.read"],"trusted":true,"status":2}`,
		cookies,
	))
	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, "snapless-beta", updated.Slug)
	require.Equal(t, "Snapless Beta Managed", updated.Name)
	require.Equal(t, []string{"openai.chat", "quota.read", "token.manage"}, updated.AllowedScopes)
	require.Equal(t, []string{"quota.read"}, updated.DefaultScopes)
	require.True(t, updated.Trusted)
	require.Equal(t, model.ConnectedAppStatusDisabled, updated.Status)

	list = decodeConnectedAppData[[]connectedAppData](t, requestConnectedAppAdmin(t, router, http.MethodGet, "/api/connected-apps", "", cookies))
	require.Len(t, list, 2)
	require.Equal(t, model.ConnectedAppSlugSnapless, list[0].Slug)
	require.Equal(t, "snapless-beta", list[1].Slug)
}

func TestConnectedAppAdminRejectsDuplicateSlugAndInvalidScopes(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	cookies := loginConnectedAppAdmin(t, router)

	duplicate := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"snapless","name":"Duplicate Snapless","allowed_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		cookies,
	))
	require.Contains(t, duplicate, "already exists")

	invalidDefault := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"third-party","name":"Third Party","allowed_scopes":["openai.chat"],"default_scopes":["quota.read"]}`,
		cookies,
	))
	require.Contains(t, invalidDefault, `default scope "quota.read"`)

	invalidSlug := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"Bad Slug","name":"Bad Slug","allowed_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		cookies,
	))
	require.True(t, strings.Contains(invalidSlug, "slug") && strings.Contains(invalidSlug, "lowercase"))

	invalidFlow := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"oauth-app","name":"OAuth App","authorization_flow":"oauth","allowed_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		cookies,
	))
	require.Contains(t, invalidFlow, "authorization_flow")
}
