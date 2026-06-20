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
const connectedAppRouterDeveloperUserId = 8402

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

type connectedAppRequestData struct {
	ID                int      `json:"id"`
	ApplicantUserID   int      `json:"applicant_user_id"`
	ApplicantName     string   `json:"applicant_name"`
	AppID             int      `json:"app_id"`
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	RequestedScopes   []string `json:"requested_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	AuthorizationFlow string   `json:"authorization_flow"`
	Status            string   `json:"status"`
	ReviewerUserID    int      `json:"reviewer_user_id"`
	ReviewerName      string   `json:"reviewer_name"`
	ReviewNote        string   `json:"review_note"`
	CreatedAt         int64    `json:"created_at"`
	ReviewedAt        int64    `json:"reviewed_at"`
}

type connectedAppAuditData struct {
	ID          int64  `json:"id"`
	ActorUserID int    `json:"actor_user_id"`
	Action      string `json:"action"`
	TargetType  string `json:"target_type"`
	TargetID    int    `json:"target_id"`
}

type connectedAppRequestMutationData struct {
	Request connectedAppRequestData `json:"request"`
	App     connectedAppData        `json:"app"`
	Audit   connectedAppAuditData   `json:"audit"`
}

type connectedAppPageData[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type connectedAppNotificationData struct {
	Key       string   `json:"key"`
	Kind      string   `json:"kind"`
	Status    string   `json:"status"`
	Read      bool     `json:"read"`
	RequestID int      `json:"request_id"`
	AppID     int      `json:"app_id"`
	AuditID   int64    `json:"audit_log_id"`
	AppName   string   `json:"app_name"`
	Scopes    []string `json:"requested_scopes"`
}

type connectedAppNotificationPageData struct {
	Items       []connectedAppNotificationData `json:"items"`
	UnreadCount int                            `json:"unread_count"`
	Page        int                            `json:"page"`
	PageSize    int                            `json:"page_size"`
	HasMore     bool                           `json:"has_more"`
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
	require.NoError(t, model.DB.Create(&model.User{
		Id:       connectedAppRouterDeveloperUserId,
		Username: "connected-app-dev",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    100000,
		AffCode:  "connected-app-dev",
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
	engine.GET("/developer-login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "connected-app-dev")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", connectedAppRouterDeveloperUserId)
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

func loginConnectedAppDeveloper(t *testing.T, router *gin.Engine) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/developer-login", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func requestConnectedAppAdmin(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return requestConnectedAppUser(t, router, method, target, body, cookies, connectedAppRouterAdminUserId)
}

func requestConnectedAppDeveloper(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return requestConnectedAppUser(t, router, method, target, body, cookies, connectedAppRouterDeveloperUserId)
}

func requestConnectedAppUser(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie, userId int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if len(cookies) > 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(userId))
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

func TestConnectedAppRequestApprovalCreatesAppAuditAndNotifications(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	adminCookies := loginConnectedAppAdmin(t, router)
	developerCookies := loginConnectedAppDeveloper(t, router)

	submitted := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"snapless-addon","name":"Snapless Addon","description":"Addon desktop client","requested_scopes":["openai.chat","quota.read","openai.chat"],"default_scopes":["openai.chat"],"authorization_flow":"device_code","homepage_url":"https://snapless.example","callback_url":"https://snapless.example/callback","reason":"desktop integration"}`,
		developerCookies,
	))
	require.NotZero(t, submitted.Request.ID)
	require.Equal(t, model.ConnectedAppRequestStatusPending, submitted.Request.Status)
	require.Equal(t, connectedAppRouterDeveloperUserId, submitted.Request.ApplicantUserID)
	require.Equal(t, []string{"openai.chat", "quota.read"}, submitted.Request.RequestedScopes)
	require.Equal(t, model.ConnectedAppAuditActionSubmit, submitted.Audit.Action)
	require.Equal(t, submitted.Request.ID, submitted.Audit.TargetID)

	selfRequests := decodeConnectedAppData[connectedAppPageData[connectedAppRequestData]](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-app-requests/self",
		"",
		developerCookies,
	))
	require.EqualValues(t, 1, selfRequests.Total)
	require.Equal(t, submitted.Request.ID, selfRequests.Items[0].ID)

	adminRequests := decodeConnectedAppData[connectedAppPageData[connectedAppRequestData]](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/requests?status=pending",
		"",
		adminCookies,
	))
	require.EqualValues(t, 1, adminRequests.Total)
	require.Equal(t, "connected-app-dev", adminRequests.Items[0].ApplicantName)

	adminNotifications := decodeConnectedAppData[connectedAppNotificationPageData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodGet,
		"/api/notifications/connected-app-requests",
		"",
		adminCookies,
	))
	require.Equal(t, 1, adminNotifications.UnreadCount)
	require.Len(t, adminNotifications.Items, 1)
	require.Equal(t, model.ConnectedAppRequestStatusPending, adminNotifications.Items[0].Status)
	require.Equal(t, submitted.Request.ID, adminNotifications.Items[0].RequestID)
	require.False(t, adminNotifications.Items[0].Read)

	approved := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"approved","review_note":"approved for device flow","allowed_scopes":["openai.chat","quota.read","token.manage"],"default_scopes":["quota.read"]}`,
		adminCookies,
	))
	require.Equal(t, model.ConnectedAppRequestStatusApproved, approved.Request.Status)
	require.NotZero(t, approved.Request.AppID)
	require.Equal(t, approved.Request.AppID, approved.App.ID)
	require.Equal(t, "snapless-addon", approved.App.Slug)
	require.True(t, approved.App.Trusted)
	require.Equal(t, []string{"openai.chat", "quota.read", "token.manage"}, approved.App.AllowedScopes)
	require.Equal(t, []string{"quota.read"}, approved.App.DefaultScopes)
	require.Equal(t, model.ConnectedAppAuditActionApprove, approved.Audit.Action)

	audits := decodeConnectedAppData[connectedAppPageData[connectedAppAuditData]](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/audit-logs?target_type=connected_app_request&target_id="+strconv.Itoa(submitted.Request.ID),
		"",
		adminCookies,
	))
	require.EqualValues(t, 2, audits.Total)
	require.Equal(t, model.ConnectedAppAuditActionApprove, audits.Items[0].Action)
	require.Equal(t, model.ConnectedAppAuditActionSubmit, audits.Items[1].Action)

	developerNotifications := decodeConnectedAppData[connectedAppNotificationPageData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/notifications/connected-app-requests",
		"",
		developerCookies,
	))
	require.Equal(t, 1, developerNotifications.UnreadCount)
	require.Len(t, developerNotifications.Items, 1)
	require.Equal(t, model.ConnectedAppRequestStatusApproved, developerNotifications.Items[0].Status)
	require.Equal(t, approved.Request.AppID, developerNotifications.Items[0].AppID)
	require.NotZero(t, developerNotifications.Items[0].AuditID)

	read := decodeConnectedAppData[map[string][]string](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/notifications/connected-app-requests/read",
		`{"connected_app_request_keys":["`+developerNotifications.Items[0].Key+`"]}`,
		developerCookies,
	))
	require.Contains(t, read["connected_app_request_keys"], developerNotifications.Items[0].Key)

	developerUnreadOnly := decodeConnectedAppData[connectedAppNotificationPageData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/notifications/connected-app-requests?unread_only=true",
		"",
		developerCookies,
	))
	require.Equal(t, 0, developerUnreadOnly.UnreadCount)
	require.Empty(t, developerUnreadOnly.Items)
}

func TestConnectedAppRequestRejectsDuplicateAndInvalidReview(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	adminCookies := loginConnectedAppAdmin(t, router)
	developerCookies := loginConnectedAppDeveloper(t, router)

	duplicateBuiltin := decodeConnectedAppFailure(t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"snapless","name":"Duplicate Builtin","requested_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		developerCookies,
	))
	require.Contains(t, duplicateBuiltin, "already exists")

	submitted := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"review-me","name":"Review Me","requested_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		developerCookies,
	))

	duplicatePending := decodeConnectedAppFailure(t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"review-me","name":"Review Me Again","requested_scopes":["openai.chat"],"default_scopes":["openai.chat"]}`,
		developerCookies,
	))
	require.Contains(t, duplicatePending, "already pending")

	invalidReview := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"approved","allowed_scopes":["openai.chat"],"default_scopes":["quota.read"]}`,
		adminCookies,
	))
	require.Contains(t, invalidReview, `default scope "quota.read"`)

	rejected := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"rejected","review_note":"scope too broad"}`,
		adminCookies,
	))
	require.Equal(t, model.ConnectedAppRequestStatusRejected, rejected.Request.Status)
	require.Zero(t, rejected.Request.AppID)
	require.Equal(t, model.ConnectedAppAuditActionReject, rejected.Audit.Action)

	reviewAgain := decodeConnectedAppFailure(t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"approved"}`,
		adminCookies,
	))
	require.Contains(t, reviewAgain, "already been reviewed")
}
