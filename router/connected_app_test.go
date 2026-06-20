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

type connectedAppDeveloperConfigData struct {
	App struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"app"`
	Owner        bool              `json:"owner"`
	BaseURL      string            `json:"base_url"`
	APIEndpoints map[string]string `json:"api_endpoints"`
	DeviceFlow   map[string]string `json:"device_flow"`
	Scopes       []string          `json:"scopes"`
}

type connectedAppDeveloperAuthorizationData struct {
	UserID   int    `json:"user_id"`
	UserName string `json:"user_name"`
	Grant    struct {
		Status string   `json:"status"`
		Scopes []string `json:"scopes"`
	} `json:"grant"`
	Devices []struct {
		Status string `json:"status"`
		Device struct {
			Fingerprint string `json:"fingerprint"`
			DeviceName  string `json:"device_name"`
		} `json:"device"`
		Token struct {
			ID                 int  `json:"id"`
			UnlimitedQuota     bool `json:"unlimited_quota"`
			ModelLimitsEnabled bool `json:"model_limits_enabled"`
		} `json:"token"`
	} `json:"devices"`
}

type connectedAppDeveloperSessionData struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	UserID       int    `json:"user_id"`
	UserName     string `json:"user_name"`
	TokenID      int    `json:"token_id"`
	TokenCreated bool   `json:"token_created"`
	Device       struct {
		Fingerprint string `json:"fingerprint"`
		DeviceName  string `json:"device_name"`
	} `json:"device"`
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

func TestConnectedAppDeveloperAPIAndDeviceFlow(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	adminCookies := loginConnectedAppAdmin(t, router)
	developerCookies := loginConnectedAppDeveloper(t, router)

	submitted := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"snapless-addon","name":"Snapless Addon","description":"Addon desktop client","requested_scopes":["openai.models","openai.chat","quota.read"],"default_scopes":["openai.chat"],"authorization_flow":"device_code","homepage_url":"https://snapless.example","reason":"desktop integration"}`,
		developerCookies,
	))

	approved := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"approved","review_note":"approved for device flow","allowed_scopes":["openai.models","openai.chat","quota.read"],"default_scopes":["openai.chat"]}`,
		adminCookies,
	))
	require.Equal(t, model.ConnectedAppRequestStatusApproved, approved.Request.Status)
	require.Equal(t, "snapless-addon", approved.App.Slug)

	config := decodeConnectedAppData[connectedAppDeveloperConfigData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/snapless-addon/developer/config",
		"",
		developerCookies,
	))
	require.True(t, config.Owner)
	require.Equal(t, "snapless-addon", config.App.Slug)
	require.Equal(t, "https://data-proxy.test/v1", config.BaseURL)
	require.ElementsMatch(t, []string{"openai.models", "openai.chat", "quota.read"}, config.Scopes)
	require.Equal(t, "https://data-proxy.test/v1/models", config.APIEndpoints["models"])
	require.Equal(t, "https://data-proxy.test/v1/chat/completions", config.APIEndpoints["chat_completions"])
	require.Equal(t, "https://data-proxy.test/api/usage/token", config.APIEndpoints["token_usage"])
	require.Contains(t, config.DeviceFlow["start"], "/api/connected-apps/snapless-addon/device/start")

	forbidden := decodeConnectedAppFailure(t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/snapless/developer/config",
		"",
		developerCookies,
	))
	require.Contains(t, forbidden, "developer access")

	started := decodeConnectedAppData[snaplessDeviceStartData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/snapless-addon/device/start",
		`{"device_id":"addon-device","device_name":"Addon Mac","platform":"macos","app_version":"2.0.0","client":"snapless-addon"}`,
		nil,
		0,
	))
	require.NotEmpty(t, started.DeviceCode)
	require.NotEmpty(t, started.UserCode)
	require.Equal(t, "snapless-addon", started.App.Slug)
	require.Equal(t, "Addon Mac", started.Device.DeviceName)
	require.Contains(t, started.VerificationURI, "/snapless/device?")
	require.Contains(t, started.VerificationURI, "app_slug=snapless-addon")
	require.Contains(t, started.VerificationURI, "user_code=")

	pending := decodeConnectedAppData[snaplessDevicePollStatusData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/snapless-addon/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, pending.Status)
	require.Equal(t, 3, pending.Interval)

	status := decodeConnectedAppData[snaplessDeviceStatusData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/snapless-addon/device/status?user_code="+started.UserCode,
		"",
		developerCookies,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, status.Status)
	require.True(t, status.Readiness.OK)

	authorized := decodeConnectedAppData[snaplessDeviceStatusData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/snapless-addon/device/authorize",
		`{"user_code":`+strconv.Quote(started.UserCode)+`,"approve":true}`,
		developerCookies,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusAuthorized, authorized.Status)
	require.NotZero(t, authorized.Token.ID)

	firstPoll := decodeConnectedAppData[snaplessTokenData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/snapless-addon/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.True(t, firstPoll.Created)
	require.True(t, firstPoll.APIKeyOnce)
	require.True(t, strings.HasPrefix(firstPoll.APIKey, "sk-"))
	require.Equal(t, authorized.Token.ID, firstPoll.Token.ID)
	require.True(t, firstPoll.Token.UnlimitedQuota)
	require.False(t, firstPoll.Token.ModelLimitsEnabled)

	secondPoll := decodeConnectedAppData[snaplessDevicePollStatusData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/snapless-addon/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusConsumed, secondPoll.Status)

	authorizations := decodeConnectedAppData[connectedAppPageData[connectedAppDeveloperAuthorizationData]](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/snapless-addon/developer/authorizations",
		"",
		developerCookies,
	))
	require.EqualValues(t, 1, authorizations.Total)
	require.Len(t, authorizations.Items, 1)
	require.Equal(t, connectedAppRouterDeveloperUserId, authorizations.Items[0].UserID)
	require.Equal(t, model.ConnectedAppGrantStatusAuthorized, authorizations.Items[0].Grant.Status)
	require.ElementsMatch(t, []string{"openai.chat"}, authorizations.Items[0].Grant.Scopes)
	require.Len(t, authorizations.Items[0].Devices, 1)
	require.Equal(t, model.ConnectedAppTokenBindingStatusActive, authorizations.Items[0].Devices[0].Status)
	require.Equal(t, firstPoll.Token.ID, authorizations.Items[0].Devices[0].Token.ID)
	require.False(t, authorizations.Items[0].Devices[0].Token.ModelLimitsEnabled)

	sessions := decodeConnectedAppData[connectedAppPageData[connectedAppDeveloperSessionData]](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/snapless-addon/developer/device-sessions?status=consumed",
		"",
		developerCookies,
	))
	require.EqualValues(t, 1, sessions.Total)
	require.Len(t, sessions.Items, 1)
	require.Equal(t, model.ConnectedAppDeviceSessionStatusConsumed, sessions.Items[0].Status)
	require.Equal(t, connectedAppRouterDeveloperUserId, sessions.Items[0].UserID)
	require.Equal(t, firstPoll.Token.ID, sessions.Items[0].TokenID)
	require.Equal(t, "Addon Mac", sessions.Items[0].Device.DeviceName)

	untrusted := decodeConnectedAppData[connectedAppData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps",
		`{"slug":"untrusted-client","name":"Untrusted Client","allowed_scopes":["openai.chat"],"default_scopes":["openai.chat"],"trusted":false,"status":1}`,
		adminCookies,
	))
	require.False(t, untrusted.Trusted)
	untrustedStart := decodeConnectedAppFailure(t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/untrusted-client/device/start",
		`{"device_id":"untrusted-device"}`,
		nil,
		0,
	))
	require.Contains(t, untrustedStart, "not trusted")
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
