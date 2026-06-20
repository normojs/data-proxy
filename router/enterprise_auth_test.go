package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type enterpriseAuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestEnterpriseRoutesRequireAdminAuth(t *testing.T) {
	setupEnterpriseRouterTestDB(t)

	router := newEnterpriseRouterForTest(t)

	unauthenticated := requestEnterpriseCurrentForTest(t, router, nil, 0)
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, unauthenticated).Success)

	commonUserCookies := loginEnterpriseRouterUserForTest(t, router, 7101, common.RoleCommonUser)
	commonUser := requestEnterpriseCurrentForTest(t, router, commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, commonUser.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, commonUser).Success)

	adminCookies := loginEnterpriseRouterUserForTest(t, router, 7199, common.RoleAdminUser)
	admin := requestEnterpriseCurrentForTest(t, router, adminCookies, 7199)
	require.Equal(t, http.StatusOK, admin.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, admin).Success)
}

func TestEnterpriseRBACReadOnlyRoles(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	router := newEnterpriseRouterForTest(t)

	financeUserId := 7201
	auditUserId := 7202
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, financeUserId, service.EnterpriseRoleFinanceViewer)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, auditUserId, service.EnterpriseRoleAuditor)
	financeCookies := loginEnterpriseRouterUserForTest(t, router, financeUserId, common.RoleCommonUser)
	auditCookies := loginEnterpriseRouterUserForTest(t, router, auditUserId, common.RoleCommonUser)

	financeCurrent := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/current", "", financeCookies, financeUserId)
	require.Equal(t, http.StatusOK, financeCurrent.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, financeCurrent).Success)

	financeUsage := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary", "", financeCookies, financeUserId)
	require.Equal(t, http.StatusOK, financeUsage.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, financeUsage).Success)

	financeAudit := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs", "", financeCookies, financeUserId)
	require.Equal(t, http.StatusOK, financeAudit.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, financeAudit).Success)

	financeManage := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/current", `{
    "name": "finance cannot manage",
    "timezone": "Asia/Shanghai",
    "status": 1
  }`, financeCookies, financeUserId)
	require.Equal(t, http.StatusOK, financeManage.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, financeManage).Success)

	auditLogs := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs", "", auditCookies, auditUserId)
	require.Equal(t, http.StatusOK, auditLogs.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, auditLogs).Success)

	auditUsage := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary", "", auditCookies, auditUserId)
	require.Equal(t, http.StatusOK, auditUsage.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, auditUsage).Success)

	auditManage := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/current", `{
    "name": "auditor cannot manage",
    "timezone": "Asia/Shanghai",
    "status": 1
  }`, auditCookies, auditUserId)
	require.Equal(t, http.StatusOK, auditManage.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, auditManage).Success)
}

func TestEnterpriseRBACEnterpriseAdminCanManageWithoutSystemAdmin(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	router := newEnterpriseRouterForTest(t)

	enterpriseAdminUserId := 7203
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, enterpriseAdminUserId, service.EnterpriseRoleEnterpriseAdmin)
	enterpriseAdminCookies := loginEnterpriseRouterUserForTest(t, router, enterpriseAdminUserId, common.RoleCommonUser)

	update := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/current", `{
    "name": "RBAC Enterprise",
    "timezone": "Asia/Shanghai",
    "status": 1
  }`, enterpriseAdminCookies, enterpriseAdminUserId)
	require.Equal(t, http.StatusOK, update.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, update).Success)

	project := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/projects", `{
    "name": "RBAC Project",
    "slug": "rbac-project",
    "description": "created by enterprise admin",
    "status": 1
  }`, enterpriseAdminCookies, enterpriseAdminUserId)
	require.Equal(t, http.StatusOK, project.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, project).Success)
}

func setupEnterpriseRouterTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalEnabled := common.EnterpriseGovernanceEnabled
	originalDryRun := common.EnterpriseGovernanceDryRunEnabled
	common.EnterpriseGovernanceEnabled = true
	common.EnterpriseGovernanceDryRunEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Enterprise{},
		&model.EnterpriseOrgUnit{},
		&model.EnterpriseOrgMembership{},
		&model.EnterprisePolicyGroup{},
		&model.EnterprisePolicyGroupMember{},
		&model.EnterpriseProject{},
		&model.EnterpriseProjectOrgUnit{},
		&model.EnterpriseQuotaPolicy{},
		&model.EnterpriseQuotaCounter{},
		&model.EnterpriseQuotaRequest{},
		&model.EnterpriseWebhook{},
		&model.EnterpriseUsageAttribution{},
		&model.EnterpriseAuditLog{},
		&model.EnterpriseNotificationRead{},
		&model.EnterpriseNotificationPreference{},
		&model.EnterpriseNotificationOutbox{},
	))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		common.EnterpriseGovernanceEnabled = originalEnabled
		common.EnterpriseGovernanceDryRunEnabled = originalDryRun
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, model.EnsureDefaultEnterprise())
}

func TestEnterpriseQuotaRequestRoutesAllowUserSubmitButAdminDecision(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "router quota policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		ModelsJson:   "[]",
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&policy).Error)
	router := newEnterpriseRouterForTest(t)
	commonUserCookies := loginEnterpriseRouterUserForTest(t, router, 7101, common.RoleCommonUser)
	adminCookies := loginEnterpriseRouterUserForTest(t, router, 7199, common.RoleAdminUser)

	submit := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+strconv.Itoa(policy.Id)+`,
    "limit_delta": 2,
    "reason": "router smoke",
    "expires_at": `+strconv.FormatInt(common.GetTimestamp()+3600, 10)+`
  }`, commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, submit.Code)
	response := decodeEnterpriseAuthResponse(t, submit)
	require.True(t, response.Success, response.Message)

	list := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/quota-requests", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, list.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, list).Success)

	policies := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/quota-requests/policies", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, policies.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, policies).Success)

	var quotaRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.Where("applicant_user_id = ?", 7101).First(&quotaRequest).Error)
	approveAsUser := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests/"+strconv.Itoa(quotaRequest.Id)+"/approve", `{}`, commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, approveAsUser.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, approveAsUser).Success)

	approveAsAdmin := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests/"+strconv.Itoa(quotaRequest.Id)+"/approve", `{}`, adminCookies, 7199)
	require.Equal(t, http.StatusOK, approveAsAdmin.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, approveAsAdmin).Success)
}

type enterpriseNotificationResponseForTest struct {
	Success bool `json:"success"`
	Data    struct {
		Items       []enterpriseNotificationItemForTest `json:"items"`
		UnreadCount int                                 `json:"unread_count"`
		Page        int                                 `json:"page"`
		PageSize    int                                 `json:"page_size"`
		HasMore     bool                                `json:"has_more"`
	} `json:"data"`
	Message string `json:"message"`
}

type enterpriseNotificationItemForTest struct {
	Key            string            `json:"key"`
	Status         string            `json:"status"`
	Read           bool              `json:"read"`
	TitleKey       string            `json:"title_key"`
	ContentKey     string            `json:"content_key"`
	ContentParams  map[string]string `json:"content_params"`
	QuotaRequestId int               `json:"quota_request_id"`
	AuditLogId     int64             `json:"audit_log_id"`
}

func TestEnterpriseQuotaRequestNotificationsFollowApprovalStatus(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "notification quota policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		ModelsJson:   "[]",
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&policy).Error)
	router := newEnterpriseRouterForTest(t)
	commonUserCookies := loginEnterpriseRouterUserForTest(t, router, 7101, common.RoleCommonUser)
	adminCookies := loginEnterpriseRouterUserForTest(t, router, 7199, common.RoleAdminUser)

	submit := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+strconv.Itoa(policy.Id)+`,
    "limit_delta": 2,
    "reason": "notification smoke",
    "expires_at": `+strconv.FormatInt(common.GetTimestamp()+3600, 10)+`
  }`, commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, submit.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, submit).Success)

	var quotaRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.Where("applicant_user_id = ?", 7101).First(&quotaRequest).Error)

	adminPending := requestEnterpriseForTest(t, router, http.MethodGet, "/api/notifications/enterprise-quota-requests", "", adminCookies, 7199)
	require.Equal(t, http.StatusOK, adminPending.Code)
	adminPendingResponse := decodeEnterpriseNotificationResponseForTest(t, adminPending)
	require.True(t, adminPendingResponse.Success, adminPendingResponse.Message)
	require.GreaterOrEqual(t, adminPendingResponse.Data.UnreadCount, 1)
	require.NotEmpty(t, adminPendingResponse.Data.Items)
	require.Equal(t, model.EnterpriseQuotaRequestStatusPending, adminPendingResponse.Data.Items[0].Status)
	require.Equal(t, quotaRequest.Id, adminPendingResponse.Data.Items[0].QuotaRequestId)
	require.NotZero(t, adminPendingResponse.Data.Items[0].AuditLogId)

	userBeforeDecision := requestEnterpriseForTest(t, router, http.MethodGet, "/api/notifications/enterprise-quota-requests", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, userBeforeDecision.Code)
	userBeforeDecisionResponse := decodeEnterpriseNotificationResponseForTest(t, userBeforeDecision)
	require.True(t, userBeforeDecisionResponse.Success, userBeforeDecisionResponse.Message)
	require.Empty(t, userBeforeDecisionResponse.Data.Items)

	approve := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests/"+strconv.Itoa(quotaRequest.Id)+"/approve", `{}`, adminCookies, 7199)
	require.Equal(t, http.StatusOK, approve.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, approve).Success)

	userApproved := requestEnterpriseForTest(t, router, http.MethodGet, "/api/notifications/enterprise-quota-requests", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, userApproved.Code)
	userApprovedResponse := decodeEnterpriseNotificationResponseForTest(t, userApproved)
	require.True(t, userApprovedResponse.Success, userApprovedResponse.Message)
	require.Len(t, userApprovedResponse.Data.Items, 2)
	approvedNotification := findEnterpriseNotificationByStatusForTest(t, userApprovedResponse.Data.Items, model.EnterpriseQuotaRequestStatusApproved)
	expiringNotification := findEnterpriseNotificationByStatusForTest(t, userApprovedResponse.Data.Items, "expiring_soon")
	require.False(t, approvedNotification.Read)
	require.False(t, expiringNotification.Read)
	require.Equal(t, quotaRequest.Id, approvedNotification.QuotaRequestId)
	require.Equal(t, quotaRequest.Id, expiringNotification.QuotaRequestId)
	require.Equal(t, "notification.enterprise_quota_request.title.approved", approvedNotification.TitleKey)
	require.Equal(t, "notification.enterprise_quota_request.content.approved", approvedNotification.ContentKey)
	require.Equal(t, "notification quota policy", approvedNotification.ContentParams["policyName"])
	require.Equal(t, "notification.enterprise_quota_request.title.expiring_soon", expiringNotification.TitleKey)
	require.Equal(t, "notification.enterprise_quota_request.content.expiring_soon", expiringNotification.ContentKey)
	require.Equal(t, 2, userApprovedResponse.Data.UnreadCount)
	require.Equal(t, 1, userApprovedResponse.Data.Page)
	require.Equal(t, service.EnterpriseQuotaRequestNotificationLimit, userApprovedResponse.Data.PageSize)
	require.False(t, userApprovedResponse.Data.HasMore)

	userUnreadOnly := requestEnterpriseForTest(t, router, http.MethodGet, "/api/notifications/enterprise-quota-requests?page=1&page_size=1&unread_only=true", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, userUnreadOnly.Code)
	userUnreadOnlyResponse := decodeEnterpriseNotificationResponseForTest(t, userUnreadOnly)
	require.True(t, userUnreadOnlyResponse.Success, userUnreadOnlyResponse.Message)
	require.Len(t, userUnreadOnlyResponse.Data.Items, 1)
	require.Equal(t, 1, userUnreadOnlyResponse.Data.Page)
	require.Equal(t, 1, userUnreadOnlyResponse.Data.PageSize)
	require.True(t, userUnreadOnlyResponse.Data.HasMore)
	require.Equal(t, 2, userUnreadOnlyResponse.Data.UnreadCount)

	markRead := requestEnterpriseForTest(t, router, http.MethodPost, "/api/notifications/enterprise-quota-requests/read", `{
    "enterprise_notification_keys": ["`+approvedNotification.Key+`"]
  }`, commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, markRead.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, markRead).Success)

	userAfterRead := requestEnterpriseForTest(t, router, http.MethodGet, "/api/notifications/enterprise-quota-requests", "", commonUserCookies, 7101)
	require.Equal(t, http.StatusOK, userAfterRead.Code)
	userAfterReadResponse := decodeEnterpriseNotificationResponseForTest(t, userAfterRead)
	require.True(t, userAfterReadResponse.Success, userAfterReadResponse.Message)
	require.Len(t, userAfterReadResponse.Data.Items, 2)
	require.True(t, findEnterpriseNotificationByStatusForTest(t, userAfterReadResponse.Data.Items, model.EnterpriseQuotaRequestStatusApproved).Read)
	require.False(t, findEnterpriseNotificationByStatusForTest(t, userAfterReadResponse.Data.Items, "expiring_soon").Read)
	require.Equal(t, 1, userAfterReadResponse.Data.UnreadCount)
}

func findEnterpriseNotificationByStatusForTest(t *testing.T, items []enterpriseNotificationItemForTest, status string) enterpriseNotificationItemForTest {
	t.Helper()
	for _, item := range items {
		if item.Status == status {
			return item
		}
	}
	require.Failf(t, "notification status not found", "status=%s items=%v", status, items)
	return enterpriseNotificationItemForTest{}
}

func newEnterpriseRouterForTest(t *testing.T) *gin.Engine {
	t.Helper()
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("enterprise-router-test"))))
	engine.GET("/login/:role", func(c *gin.Context) {
		userId := 7101
		role := common.RoleCommonUser
		if c.Param("role") == "admin" {
			userId = 7199
			role = common.RoleAdminUser
		}
		if userIdQuery := c.Query("user_id"); userIdQuery != "" {
			parsedUserId, err := strconv.Atoi(userIdQuery)
			require.NoError(t, err)
			userId = parsedUserId
		}
		user := model.User{Id: userId}
		require.NoError(t, model.DB.Where("id = ?", userId).Attrs(model.User{
			Username: "enterprise-router-test-" + strconv.Itoa(userId),
			Status:   common.UserStatusEnabled,
			Role:     role,
			Group:    "default",
			AffCode:  "enterprise-router-test-" + strconv.Itoa(userId),
		}).FirstOrCreate(&user).Error)
		session := sessions.Default(c)
		session.Set("username", "enterprise-router-test")
		session.Set("role", role)
		session.Set("id", userId)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)
	return engine
}

func loginEnterpriseRouterUserForTest(t *testing.T, router *gin.Engine, userId int, role int) []*http.Cookie {
	t.Helper()
	target := "/login/common"
	if role >= common.RoleAdminUser {
		target = "/login/admin"
	}
	target += "?user_id=" + strconv.Itoa(userId)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func requestEnterpriseCurrentForTest(t *testing.T, router *gin.Engine, cookies []*http.Cookie, userId int) *httptest.ResponseRecorder {
	t.Helper()
	return requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/current", "", cookies, userId)
}

func seedEnterpriseMembershipRoleForTest(t *testing.T, enterpriseId int, userId int, role string) {
	t.Helper()
	require.NoError(t, model.DB.Where("id = ?", userId).Attrs(model.User{
		Username: "enterprise-rbac-test-" + strconv.Itoa(userId),
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		AffCode:  "enterprise-rbac-test-" + strconv.Itoa(userId),
	}).FirstOrCreate(&model.User{Id: userId}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterpriseId,
		UserId:       userId,
		Role:         role,
		IsPrimary:    true,
	}).Error)
}

func requestEnterpriseForTest(t *testing.T, router *gin.Engine, method string, target string, body string, cookies []*http.Cookie, userId int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if userId > 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(userId))
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeEnterpriseAuthResponse(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseAuthResponse {
	t.Helper()
	var response enterpriseAuthResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeEnterpriseNotificationResponseForTest(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseNotificationResponseForTest {
	t.Helper()
	var response enterpriseNotificationResponseForTest
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
