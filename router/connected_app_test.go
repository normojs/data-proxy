package router

import (
	"bytes"
	"fmt"
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

type connectedAppDeveloperSDKConfigData struct {
	OpenAPIURL         string            `json:"openapi_url"`
	Scopes             []string          `json:"scopes"`
	APIEndpoints       map[string]string `json:"api_endpoints"`
	DeveloperEndpoints map[string]string `json:"developer_endpoints"`
	Environment        map[string]string `json:"environment"`
	Examples           []struct {
		ID       string `json:"id"`
		Label    string `json:"label"`
		Language string `json:"language"`
		Code     string `json:"code"`
	} `json:"examples"`
	Permissions struct {
		CanCreateKey bool `json:"can_create_key"`
		CanReadUsage bool `json:"can_read_usage"`
	} `json:"permissions"`
	SDK struct {
		OpenAICompatible bool   `json:"openai_compatible"`
		BaseURL          string `json:"base_url"`
		BaseURLEnv       string `json:"base_url_env"`
		APIKeyEnv        string `json:"api_key_env"`
		Authorization    string `json:"authorization"`
	} `json:"sdk"`
}

type connectedAppDeveloperUsageData struct {
	TokenCount int `json:"token_count"`
	Total      struct {
		RequestCount     int64 `json:"request_count"`
		Quota            int64 `json:"quota"`
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"total"`
	ByModel []struct {
		ModelName        string `json:"model_name"`
		RequestCount     int64  `json:"request_count"`
		Quota            int64  `json:"quota"`
		PromptTokens     int64  `json:"prompt_tokens"`
		CompletionTokens int64  `json:"completion_tokens"`
	} `json:"by_model"`
	ByToken []struct {
		TokenID int    `json:"token_id"`
		Status  string `json:"status"`
		Device  struct {
			DeviceName string `json:"device_name"`
		} `json:"device"`
		RequestCount     int64 `json:"request_count"`
		Quota            int64 `json:"quota"`
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"by_token"`
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

type connectedAppClientCapabilitiesData struct {
	Groups           bool `json:"groups"`
	DedicatedTokens  bool `json:"dedicated_tokens"`
	TokenRotate      bool `json:"token_rotate"`
	TokenRevoke      bool `json:"token_revoke"`
	TokenGroupUpdate bool `json:"token_group_update"`
	OpenAIModels     bool `json:"openai_models"`
	OpenAIResponses  bool `json:"openai_responses"`
	OpenAIChat       bool `json:"openai_chat"`
}

type connectedAppClientPollData struct {
	Status                   string `json:"status"`
	ManagementToken          string `json:"management_token"`
	ManagementTokenExpiresAt int64  `json:"management_token_expires_at"`
	APIKey                   string `json:"api_key"`
	ServerURL                string `json:"server_url"`
	BaseURL                  string `json:"base_url"`
	App                      struct {
		Slug string `json:"slug"`
	} `json:"app"`
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Group    string `json:"group"`
	} `json:"user"`
	Device struct {
		Fingerprint string `json:"fingerprint"`
		DeviceName  string `json:"device_name"`
	} `json:"device"`
	Capabilities    connectedAppClientCapabilitiesData `json:"capabilities"`
	APIEndpoints    map[string]string                  `json:"api_endpoints"`
	ClientEndpoints map[string]string                  `json:"client_endpoints"`
	Token           struct {
		ID int `json:"id"`
	} `json:"token"`
}

type connectedAppClientConfigData struct {
	ServerURL string `json:"server_url"`
	BaseURL   string `json:"base_url"`
	App       struct {
		Slug string `json:"slug"`
	} `json:"app"`
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Group    string `json:"group"`
	} `json:"user"`
	SelectedToken   connectedAppClientTokenData        `json:"selected_token"`
	Capabilities    connectedAppClientCapabilitiesData `json:"capabilities"`
	APIEndpoints    map[string]string                  `json:"api_endpoints"`
	ClientEndpoints map[string]string                  `json:"client_endpoints"`
}

type connectedAppClientGroupData struct {
	DefaultGroup string `json:"default_group"`
	Data         []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Available   bool   `json:"available"`
		IsDefault   bool   `json:"is_default"`
	} `json:"data"`
}

type connectedAppClientTokenData struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	Status                int    `json:"status"`
	Group                 string `json:"group"`
	EffectiveGroup        string `json:"effective_group"`
	GroupAvailable        bool   `json:"group_available"`
	OwnedByConnectedApp   bool   `json:"owned_by_connected_app"`
	ConnectedAppSlug      string `json:"connected_app_slug"`
	DeviceID              string `json:"device_id"`
	UnlimitedQuota        bool   `json:"unlimited_quota"`
	QuotaHardLimitEnabled bool   `json:"quota_hard_limit_enabled"`
	ModelLimitsEnabled    bool   `json:"model_limits_enabled"`
}

type connectedAppClientTokenResponseData struct {
	Selected         bool                        `json:"selected"`
	Created          bool                        `json:"created"`
	Rotated          bool                        `json:"rotated"`
	Revoked          bool                        `json:"revoked"`
	APIKeyOnce       bool                        `json:"api_key_once"`
	APIKey           string                      `json:"api_key"`
	BaseURL          string                      `json:"base_url"`
	Token            connectedAppClientTokenData `json:"token"`
	RequiresRotation bool                        `json:"requires_rotation"`
	Message          string                      `json:"message"`
}

type connectedAppClientUpdateGroupData struct {
	Updated bool                        `json:"updated"`
	Token   connectedAppClientTokenData `json:"token"`
}

func newConnectedAppAdminRouterForTest(t *testing.T) *gin.Engine {
	t.Helper()

	require.NoError(t, model.DB.Create(&model.User{
		Id:       connectedAppRouterAdminUserId,
		Username: "connected-app-admin",
		Email:    "connected-app-admin@example.com",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleAdminUser,
		Group:    "default",
		Quota:    100000,
		AffCode:  "connected-app-admin",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       connectedAppRouterDeveloperUserId,
		Username: "connected-app-dev",
		Email:    "connected-app-dev@example.com",
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

func requestConnectedAppClient(t *testing.T, router *gin.Engine, method string, target string, body string, managementToken string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if managementToken != "" {
		request.Header.Set("Authorization", "Bearer "+managementToken)
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

func requireConnectedAppDeveloperExample(t *testing.T, examples []struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Code     string `json:"code"`
}, id string, codeContains string) {
	t.Helper()
	for _, example := range examples {
		if example.ID == id {
			require.NotEmpty(t, example.Label)
			require.NotEmpty(t, example.Language)
			require.Contains(t, example.Code, codeContains)
			return
		}
	}
	require.Failf(t, "developer sdk example missing", "id=%s examples=%v", id, examples)
}

func connectedAppDataBySlug(t *testing.T, apps []connectedAppData, slug string) connectedAppData {
	t.Helper()
	for _, app := range apps {
		if app.Slug == slug {
			return app
		}
	}
	require.Failf(t, "connected app missing", "slug=%s apps=%v", slug, apps)
	return connectedAppData{}
}

func connectedAppClientGroupByID(t *testing.T, groups connectedAppClientGroupData, id string) struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	IsDefault   bool   `json:"is_default"`
} {
	t.Helper()
	for _, group := range groups.Data {
		if group.ID == id {
			return group
		}
	}
	require.Failf(t, "connected app group missing", "id=%s groups=%v", id, groups.Data)
	return struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Available   bool   `json:"available"`
		IsDefault   bool   `json:"is_default"`
	}{}
}

func TestConnectedAppAdminCRUD(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	cookies := loginConnectedAppAdmin(t, router)

	list := decodeConnectedAppData[[]connectedAppData](t, requestConnectedAppAdmin(t, router, http.MethodGet, "/api/connected-apps", "", cookies))
	require.Len(t, list, 2)
	snapless := connectedAppDataBySlug(t, list, model.ConnectedAppSlugSnapless)
	require.Equal(t, model.ConnectedAppStatusEnabled, snapless.Status)
	require.True(t, snapless.Trusted)
	require.Equal(t, "device_code", snapless.AuthorizationFlow)
	require.Contains(t, snapless.AllowedScopes, "token.manage")
	codexDP := connectedAppDataBySlug(t, list, model.ConnectedAppSlugCodexDP)
	require.Equal(t, model.ConnectedAppStatusEnabled, codexDP.Status)
	require.True(t, codexDP.Trusted)
	require.Contains(t, codexDP.AllowedScopes, "token.create")
	require.NotContains(t, codexDP.AllowedScopes, "token.manage")

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
	require.Len(t, list, 3)
	connectedAppDataBySlug(t, list, model.ConnectedAppSlugSnapless)
	connectedAppDataBySlug(t, list, model.ConnectedAppSlugCodexDP)
	connectedAppDataBySlug(t, list, "snapless-beta")
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

	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              0,
		Channel:            model.ConnectedAppNotificationOutboxChannelEmail,
		EventType:          model.ConnectedAppAuditActionApprove,
		Enabled:            true,
		RecipientScopeJson: `{"applicant":true}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              0,
		Channel:            model.ConnectedAppNotificationOutboxChannelWebhook,
		EventType:          model.ConnectedAppAuditActionApprove,
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppWebhook{
		AppId:          0,
		Name:           "global approval webhook",
		Url:            "https://example.com/connected-app-approval",
		EventTypesJson: `["connected_app_request.approve"]`,
		Status:         model.ConnectedAppWebhookStatusEnabled,
	}).Error)

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

	var reviewOutboxRows []model.ConnectedAppNotificationOutbox
	require.NoError(t, model.DB.Where("target_type = ? AND target_id = ?", "connected_app_request", submitted.Request.ID).Order("channel asc").Find(&reviewOutboxRows).Error)
	require.Len(t, reviewOutboxRows, 2)
	reviewChannels := []string{reviewOutboxRows[0].Channel, reviewOutboxRows[1].Channel}
	require.ElementsMatch(t, []string{model.ConnectedAppNotificationOutboxChannelEmail, model.ConnectedAppNotificationOutboxChannelWebhook}, reviewChannels)
	for _, row := range reviewOutboxRows {
		require.Equal(t, approved.App.ID, row.AppId)
		require.Equal(t, model.ConnectedAppAuditActionApprove, row.EventType)
		require.Equal(t, model.ConnectedAppNotificationOutboxStatusPending, row.Status)
		if row.Channel == model.ConnectedAppNotificationOutboxChannelEmail {
			require.Equal(t, "connected-app-dev@example.com", row.RecipientEmail)
		}
		if row.Channel == model.ConnectedAppNotificationOutboxChannelWebhook {
			require.Equal(t, "webhook:1", row.RecipientEmail)
		}
	}

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

	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              approved.App.ID,
		Channel:            model.ConnectedAppNotificationOutboxChannelEmail,
		EventType:          model.ConnectedAppNotificationEventDeviceAuthorized,
		Enabled:            true,
		RecipientScopeJson: `{"app_developers":true}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              approved.App.ID,
		Channel:            model.ConnectedAppNotificationOutboxChannelWebhook,
		EventType:          model.ConnectedAppNotificationEventDeviceAuthorized,
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppWebhook{
		AppId:          approved.App.ID,
		Name:           "device authorization webhook",
		Url:            "https://example.com/device-authorization",
		EventTypesJson: `["connected_app_device.authorized"]`,
		Status:         model.ConnectedAppWebhookStatusEnabled,
	}).Error)

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
	require.Contains(t, started.VerificationURI, "/connect/device?")
	require.Contains(t, started.VerificationURI, "app_slug=snapless-addon")
	require.Contains(t, started.VerificationURI, "signup_app=snapless-addon")
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

	var deviceOutboxRows []model.ConnectedAppNotificationOutbox
	require.NoError(t, model.DB.Where("app_id = ? AND event_type = ?", approved.App.ID, model.ConnectedAppNotificationEventDeviceAuthorized).Order("channel asc").Find(&deviceOutboxRows).Error)
	require.Len(t, deviceOutboxRows, 2)
	deviceChannels := []string{deviceOutboxRows[0].Channel, deviceOutboxRows[1].Channel}
	require.ElementsMatch(t, []string{model.ConnectedAppNotificationOutboxChannelEmail, model.ConnectedAppNotificationOutboxChannelWebhook}, deviceChannels)
	for _, row := range deviceOutboxRows {
		require.Equal(t, model.ConnectedAppNotificationOutboxStatusPending, row.Status)
		if row.Channel == model.ConnectedAppNotificationOutboxChannelEmail {
			require.Equal(t, "connected-app-dev@example.com", row.RecipientEmail)
		}
		if row.Channel == model.ConnectedAppNotificationOutboxChannelWebhook {
			require.Equal(t, "webhook:1", row.RecipientEmail)
		}
	}

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
	require.Equal(t, connectedAppRouterDeveloperUserId, firstPoll.User.ID)
	require.Equal(t, "connected-app-dev", firstPoll.User.Username)

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

func TestConnectedAppCodexDPManagementTokenFlow(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	developerCookies := loginConnectedAppDeveloper(t, router)

	started := decodeConnectedAppData[snaplessDeviceStartData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/codex-dp/device/start",
		`{"device_id":"codex-mac","device_name":"Codex Mac","platform":"macos","app_version":"0.3.0","client":"codex-dp"}`,
		nil,
		0,
	))
	require.NotEmpty(t, started.DeviceCode)
	require.NotEmpty(t, started.UserCode)
	require.Equal(t, model.ConnectedAppSlugCodexDP, started.App.Slug)
	require.Equal(t, "Codex Mac", started.Device.DeviceName)

	pending := decodeConnectedAppData[snaplessDevicePollStatusData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/codex-dp/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, pending.Status)

	status := decodeConnectedAppData[snaplessDeviceStatusData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/codex-dp/device/status?user_code="+started.UserCode,
		"",
		developerCookies,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusPending, status.Status)
	require.True(t, status.Readiness.OK)

	authorized := decodeConnectedAppData[snaplessDeviceStatusData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/codex-dp/device/authorize",
		`{"user_code":`+strconv.Quote(started.UserCode)+`,"approve":true}`,
		developerCookies,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusAuthorized, authorized.Status)
	require.Zero(t, authorized.Token.ID)

	var tokenCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", connectedAppRouterDeveloperUserId).Count(&tokenCount).Error)
	require.EqualValues(t, 0, tokenCount)

	firstPoll := decodeConnectedAppData[connectedAppClientPollData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/codex-dp/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusAuthorized, firstPoll.Status)
	require.True(t, strings.HasPrefix(firstPoll.ManagementToken, "cdpat_"))
	require.NotZero(t, firstPoll.ManagementTokenExpiresAt)
	require.Empty(t, firstPoll.APIKey)
	require.Zero(t, firstPoll.Token.ID)
	require.Equal(t, model.ConnectedAppSlugCodexDP, firstPoll.App.Slug)
	require.Equal(t, connectedAppRouterDeveloperUserId, firstPoll.User.ID)
	require.Equal(t, started.Device.Fingerprint, firstPoll.Device.Fingerprint)
	require.True(t, firstPoll.Capabilities.Groups)
	require.True(t, firstPoll.Capabilities.DedicatedTokens)
	require.True(t, firstPoll.Capabilities.TokenRotate)
	require.True(t, firstPoll.Capabilities.TokenRevoke)
	require.True(t, firstPoll.Capabilities.TokenGroupUpdate)
	require.True(t, firstPoll.Capabilities.OpenAIModels)
	require.True(t, firstPoll.Capabilities.OpenAIResponses)
	require.True(t, firstPoll.Capabilities.OpenAIChat)
	require.Equal(t, "https://data-proxy.test/v1/models", firstPoll.APIEndpoints["models"])
	require.Equal(t, "https://data-proxy.test/v1/responses", firstPoll.APIEndpoints["responses"])
	require.Contains(t, firstPoll.ClientEndpoints["config"], "/api/connected-app-clients/codex-dp/config")
	require.Contains(t, firstPoll.ClientEndpoints["tokens_ensure"], "/api/connected-app-clients/codex-dp/tokens/ensure")

	secondPoll := decodeConnectedAppData[snaplessDevicePollStatusData](t, requestConnectedAppUser(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/codex-dp/device/poll",
		`{"device_code":`+strconv.Quote(started.DeviceCode)+`}`,
		nil,
		0,
	))
	require.Equal(t, model.ConnectedAppDeviceSessionStatusConsumed, secondPoll.Status)

	var accessTokenCount int64
	require.NoError(t, model.DB.Model(&model.ConnectedAppAccessToken{}).
		Where("user_id = ? AND status = ?", connectedAppRouterDeveloperUserId, model.ConnectedAppAccessTokenStatusActive).
		Count(&accessTokenCount).Error)
	require.EqualValues(t, 1, accessTokenCount)

	config := decodeConnectedAppData[connectedAppClientConfigData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodGet,
		"/api/connected-app-clients/codex-dp/config",
		"",
		firstPoll.ManagementToken,
	))
	require.Equal(t, "https://data-proxy.test", config.ServerURL)
	require.Equal(t, "https://data-proxy.test/v1", config.BaseURL)
	require.Equal(t, model.ConnectedAppSlugCodexDP, config.App.Slug)
	require.Equal(t, connectedAppRouterDeveloperUserId, config.User.ID)
	require.Zero(t, config.SelectedToken.ID)
	require.True(t, config.Capabilities.DedicatedTokens)
	require.Equal(t, firstPoll.APIEndpoints["responses"], config.APIEndpoints["responses"])
	require.Contains(t, config.ClientEndpoints["groups"], "/api/connected-app-clients/codex-dp/groups")

	groups := decodeConnectedAppData[connectedAppClientGroupData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodGet,
		"/api/connected-app-clients/codex-dp/groups",
		"",
		firstPoll.ManagementToken,
	))
	require.Equal(t, "default", groups.DefaultGroup)
	defaultGroup := connectedAppClientGroupByID(t, groups, "default")
	require.True(t, defaultGroup.Available)
	require.True(t, defaultGroup.IsDefault)
	vipGroup := connectedAppClientGroupByID(t, groups, "vip")
	require.True(t, vipGroup.Available)

	created := decodeConnectedAppData[connectedAppClientTokenResponseData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-clients/codex-dp/tokens/ensure",
		`{"device_id":"codex-mac","device_name":"Codex Mac","platform":"macos","app_version":"0.3.0","client":"codex-dp","group":"vip"}`,
		firstPoll.ManagementToken,
	))
	require.True(t, created.Selected)
	require.True(t, created.Created)
	require.True(t, created.APIKeyOnce)
	require.True(t, strings.HasPrefix(created.APIKey, "sk-"))
	require.Equal(t, "https://data-proxy.test/v1", created.BaseURL)
	require.NotZero(t, created.Token.ID)
	require.Equal(t, "vip", created.Token.Group)
	require.Equal(t, "vip", created.Token.EffectiveGroup)
	require.True(t, created.Token.GroupAvailable)
	require.True(t, created.Token.OwnedByConnectedApp)
	require.Equal(t, model.ConnectedAppSlugCodexDP, created.Token.ConnectedAppSlug)
	require.Equal(t, started.Device.Fingerprint, created.Token.DeviceID)
	require.True(t, created.Token.UnlimitedQuota)
	require.False(t, created.Token.ModelLimitsEnabled)

	reused := decodeConnectedAppData[connectedAppClientTokenResponseData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-clients/codex-dp/tokens/ensure",
		`{"device_id":"codex-mac","device_name":"Codex Mac","platform":"macos","app_version":"0.3.0","client":"codex-dp","group":"vip"}`,
		firstPoll.ManagementToken,
	))
	require.False(t, reused.Created)
	require.False(t, reused.APIKeyOnce)
	require.Empty(t, reused.APIKey)
	require.True(t, reused.RequiresRotation)
	require.Equal(t, created.Token.ID, reused.Token.ID)

	updated := decodeConnectedAppData[connectedAppClientUpdateGroupData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPut,
		fmt.Sprintf("/api/connected-app-clients/codex-dp/tokens/%d/group", created.Token.ID),
		`{"group":"default"}`,
		firstPoll.ManagementToken,
	))
	require.True(t, updated.Updated)
	require.Equal(t, "default", updated.Token.Group)
	require.Equal(t, "default", updated.Token.EffectiveGroup)

	ensureRotated := decodeConnectedAppData[connectedAppClientTokenResponseData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-clients/codex-dp/tokens/ensure",
		`{"device_id":"codex-mac","device_name":"Codex Mac","platform":"macos","app_version":"0.3.0","client":"codex-dp","rotate":true}`,
		firstPoll.ManagementToken,
	))
	require.False(t, ensureRotated.Created)
	require.True(t, ensureRotated.Rotated)
	require.True(t, ensureRotated.APIKeyOnce)
	require.True(t, strings.HasPrefix(ensureRotated.APIKey, "sk-"))
	require.NotEqual(t, created.Token.ID, ensureRotated.Token.ID)
	require.Equal(t, "default", ensureRotated.Token.Group)
	require.Equal(t, "default", ensureRotated.Token.EffectiveGroup)

	var createdTokenAfterEnsureRotate model.Token
	require.NoError(t, model.DB.First(&createdTokenAfterEnsureRotate, created.Token.ID).Error)
	require.Equal(t, common.TokenStatusDisabled, createdTokenAfterEnsureRotate.Status)

	rotated := decodeConnectedAppData[connectedAppClientTokenResponseData](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/connected-app-clients/codex-dp/tokens/%d/rotate", ensureRotated.Token.ID),
		"",
		firstPoll.ManagementToken,
	))
	require.True(t, rotated.Rotated)
	require.True(t, rotated.APIKeyOnce)
	require.True(t, strings.HasPrefix(rotated.APIKey, "sk-"))
	require.NotEqual(t, ensureRotated.Token.ID, rotated.Token.ID)

	var ensureRotatedOldToken model.Token
	require.NoError(t, model.DB.First(&ensureRotatedOldToken, ensureRotated.Token.ID).Error)
	require.Equal(t, common.TokenStatusDisabled, ensureRotatedOldToken.Status)

	revoked := decodeConnectedAppData[map[string]bool](t, requestConnectedAppClient(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/connected-app-clients/codex-dp/tokens/%d/revoke", rotated.Token.ID),
		"",
		firstPoll.ManagementToken,
	))
	require.True(t, revoked["revoked"])

	var binding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("token_id = ?", rotated.Token.ID).First(&binding).Error)
	require.Equal(t, model.ConnectedAppTokenBindingStatusRevoked, binding.Status)
}

func TestConnectedAppDeveloperSelfService(t *testing.T) {
	setupSnaplessRouterTestDB(t)
	router := newConnectedAppAdminRouterForTest(t)
	adminCookies := loginConnectedAppAdmin(t, router)
	developerCookies := loginConnectedAppDeveloper(t, router)

	submitted := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-app-requests",
		`{"slug":"sdk-addon","name":"SDK Addon","description":"SDK self-service client","requested_scopes":["openai.models","openai.chat","quota.read","token.manage"],"default_scopes":["openai.models","openai.chat","quota.read"],"authorization_flow":"device_code","homepage_url":"https://sdk.example","reason":"sdk integration"}`,
		developerCookies,
	))

	approved := decodeConnectedAppData[connectedAppRequestMutationData](t, requestConnectedAppAdmin(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/requests/"+strconv.Itoa(submitted.Request.ID)+"/review",
		`{"decision":"approved","review_note":"approved for self-service","allowed_scopes":["openai.models","openai.chat","quota.read","token.manage"],"default_scopes":["openai.models","openai.chat","quota.read"]}`,
		adminCookies,
	))
	require.Equal(t, model.ConnectedAppRequestStatusApproved, approved.Request.Status)

	sdkConfig := decodeConnectedAppData[connectedAppDeveloperSDKConfigData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/sdk-addon/developer/sdk-config",
		"",
		developerCookies,
	))
	require.True(t, sdkConfig.Permissions.CanCreateKey)
	require.True(t, sdkConfig.Permissions.CanReadUsage)
	require.True(t, sdkConfig.SDK.OpenAICompatible)
	require.Equal(t, "https://data-proxy.test/v1", sdkConfig.SDK.BaseURL)
	require.Equal(t, "OPENAI_BASE_URL", sdkConfig.SDK.BaseURLEnv)
	require.Equal(t, "OPENAI_API_KEY", sdkConfig.SDK.APIKeyEnv)
	require.Equal(t, "Bearer sk-<api_key>", sdkConfig.SDK.Authorization)
	require.Equal(t, "https://data-proxy.test/v1", sdkConfig.Environment["OPENAI_BASE_URL"])
	require.Equal(t, "sk-<api_key>", sdkConfig.Environment["OPENAI_API_KEY"])
	require.Equal(t, "https://data-proxy.test/v1/models", sdkConfig.APIEndpoints["models"])
	require.Contains(t, sdkConfig.DeveloperEndpoints["keys"], "/api/connected-apps/sdk-addon/developer/keys")
	require.Contains(t, sdkConfig.OpenAPIURL, "/api/connected-apps/sdk-addon/developer/openapi")
	requireConnectedAppDeveloperExample(t, sdkConfig.Examples, "openai-js-chat", "OPENAI_BASE_URL")
	requireConnectedAppDeveloperExample(t, sdkConfig.Examples, "curl-token-usage", "/api/usage/token")

	openAPI := decodeConnectedAppData[map[string]any](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/sdk-addon/developer/openapi",
		"",
		developerCookies,
	))
	require.Equal(t, "3.0.3", openAPI["openapi"])
	paths, ok := openAPI["paths"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, paths, "/v1/models")
	require.Contains(t, paths, "/v1/chat/completions")
	require.Contains(t, paths, "/api/usage/token")
	require.NotContains(t, paths, "/v1/audio/transcriptions")

	keyBody := `{"device_id":"developer-ci","device_name":"Developer CI","platform":"server","app_version":"1.0.0","client":"sdk-test"}`
	firstKey := decodeConnectedAppData[snaplessTokenData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/sdk-addon/developer/keys",
		keyBody,
		developerCookies,
	))
	require.True(t, firstKey.Created)
	require.True(t, firstKey.APIKeyOnce)
	require.True(t, strings.HasPrefix(firstKey.APIKey, "sk-"))
	require.ElementsMatch(t, []string{"openai.models", "openai.chat", "quota.read"}, firstKey.Grant.Scopes)
	require.True(t, firstKey.Token.UnlimitedQuota)
	require.False(t, firstKey.Token.ModelLimitsEnabled)

	reusedKey := decodeConnectedAppData[snaplessTokenData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/sdk-addon/developer/keys",
		keyBody,
		developerCookies,
	))
	require.False(t, reusedKey.Created)
	require.False(t, reusedKey.APIKeyOnce)
	require.Empty(t, reusedKey.APIKey)
	require.Equal(t, firstKey.Token.ID, reusedKey.Token.ID)

	rotatedKey := decodeConnectedAppData[snaplessTokenData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodPost,
		"/api/connected-apps/sdk-addon/developer/keys",
		`{"device_id":"developer-ci","device_name":"Developer CI","platform":"server","app_version":"1.0.1","client":"sdk-test","rotate":true}`,
		developerCookies,
	))
	require.True(t, rotatedKey.Created)
	require.True(t, rotatedKey.Rotated)
	require.True(t, rotatedKey.APIKeyOnce)
	require.NotEqual(t, firstKey.Token.ID, rotatedKey.Token.ID)

	var previousToken model.Token
	require.NoError(t, model.DB.First(&previousToken, firstKey.Token.ID).Error)
	require.Equal(t, common.TokenStatusDisabled, previousToken.Status)

	var currentBinding model.ConnectedAppTokenBinding
	require.NoError(t, model.DB.Where("app_id = ? AND user_id = ? AND device_fingerprint = ?", approved.App.ID, connectedAppRouterDeveloperUserId, rotatedKey.Device.Fingerprint).First(&currentBinding).Error)
	require.Equal(t, rotatedKey.Token.ID, currentBinding.TokenId)

	var attributionCount int64
	require.NoError(t, model.DB.Model(&model.ConnectedAppTokenAttribution{}).
		Where("app_id = ? AND token_id IN ?", approved.App.ID, []int{firstKey.Token.ID, rotatedKey.Token.ID}).
		Count(&attributionCount).Error)
	require.EqualValues(t, 2, attributionCount)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.ConnectedAppAuditLog{}).
		Where("target_type = ? AND target_id = ? AND action IN ?", "connected_app", approved.App.ID, []string{"connected_app_developer.key_create", "connected_app_developer.key_rotate"}).
		Count(&auditCount).Error)
	require.EqualValues(t, 2, auditCount)

	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{
			UserId:           connectedAppRouterDeveloperUserId,
			CreatedAt:        950,
			Type:             model.LogTypeConsume,
			TokenId:          firstKey.Token.ID,
			TokenName:        "Developer CI",
			ModelName:        "gpt-4o-mini",
			Quota:            5,
			PromptTokens:     2,
			CompletionTokens: 1,
		},
		{
			UserId:           connectedAppRouterDeveloperUserId,
			CreatedAt:        1000,
			Type:             model.LogTypeConsume,
			TokenId:          rotatedKey.Token.ID,
			TokenName:        "Developer CI",
			ModelName:        "gpt-4o-mini",
			Quota:            11,
			PromptTokens:     7,
			CompletionTokens: 5,
		},
		{
			UserId:           connectedAppRouterDeveloperUserId,
			CreatedAt:        1100,
			Type:             model.LogTypeConsume,
			TokenId:          rotatedKey.Token.ID,
			TokenName:        "Developer CI",
			ModelName:        "gpt-4o",
			Quota:            13,
			PromptTokens:     3,
			CompletionTokens: 2,
		},
		{
			UserId:           connectedAppRouterDeveloperUserId,
			CreatedAt:        1300,
			Type:             model.LogTypeConsume,
			TokenId:          rotatedKey.Token.ID,
			TokenName:        "Developer CI",
			ModelName:        "gpt-4o",
			Quota:            99,
			PromptTokens:     9,
			CompletionTokens: 9,
		},
		{
			UserId:           connectedAppRouterDeveloperUserId,
			CreatedAt:        1050,
			Type:             model.LogTypeConsume,
			TokenId:          123456,
			TokenName:        "Unbound",
			ModelName:        "gpt-4o",
			Quota:            77,
			PromptTokens:     7,
			CompletionTokens: 7,
		},
	}).Error)

	usage := decodeConnectedAppData[connectedAppDeveloperUsageData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/sdk-addon/developer/usage?start_time=900&end_time=1200",
		"",
		developerCookies,
	))
	require.Equal(t, 2, usage.TokenCount)
	require.EqualValues(t, 3, usage.Total.RequestCount)
	require.EqualValues(t, 29, usage.Total.Quota)
	require.EqualValues(t, 12, usage.Total.PromptTokens)
	require.EqualValues(t, 8, usage.Total.CompletionTokens)
	require.Len(t, usage.ByToken, 2)

	tokenUsage := map[int]struct {
		status     string
		deviceName string
		quota      int64
	}{}
	for _, item := range usage.ByToken {
		tokenUsage[item.TokenID] = struct {
			status     string
			deviceName string
			quota      int64
		}{
			status:     item.Status,
			deviceName: item.Device.DeviceName,
			quota:      item.Quota,
		}
	}
	require.Equal(t, "historical", tokenUsage[firstKey.Token.ID].status)
	require.Equal(t, "Developer CI", tokenUsage[firstKey.Token.ID].deviceName)
	require.EqualValues(t, 5, tokenUsage[firstKey.Token.ID].quota)
	require.Equal(t, model.ConnectedAppTokenBindingStatusActive, tokenUsage[rotatedKey.Token.ID].status)
	require.Equal(t, "Developer CI", tokenUsage[rotatedKey.Token.ID].deviceName)
	require.EqualValues(t, 24, tokenUsage[rotatedKey.Token.ID].quota)

	modelQuota := map[string]int64{}
	for _, item := range usage.ByModel {
		modelQuota[item.ModelName] = item.Quota
	}
	require.EqualValues(t, 16, modelQuota["gpt-4o-mini"])
	require.EqualValues(t, 13, modelQuota["gpt-4o"])

	previousTokenUsage := decodeConnectedAppData[connectedAppDeveloperUsageData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		fmt.Sprintf("/api/connected-apps/sdk-addon/developer/usage?token_id=%d", firstKey.Token.ID),
		"",
		developerCookies,
	))
	require.Equal(t, 1, previousTokenUsage.TokenCount)
	require.EqualValues(t, 1, previousTokenUsage.Total.RequestCount)
	require.EqualValues(t, 5, previousTokenUsage.Total.Quota)

	emptyUsage := decodeConnectedAppData[connectedAppDeveloperUsageData](t, requestConnectedAppDeveloper(
		t,
		router,
		http.MethodGet,
		"/api/connected-apps/sdk-addon/developer/usage?token_id=123456",
		"",
		developerCookies,
	))
	require.Equal(t, 0, emptyUsage.TokenCount)
	require.EqualValues(t, 0, emptyUsage.Total.RequestCount)
	require.Empty(t, emptyUsage.ByModel)
	require.Empty(t, emptyUsage.ByToken)
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
