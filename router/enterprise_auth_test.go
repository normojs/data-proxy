package router

import (
	"bytes"
	"encoding/csv"
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

type enterprisePageResponseForTest[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Items []T `json:"items"`
		Total int `json:"total"`
	} `json:"data"`
}

type enterpriseMemberItemForTest struct {
	UserId    int `json:"user_id"`
	OrgUnitId int `json:"org_unit_id"`
}

type enterpriseQuotaPolicyItemForTest struct {
	Id         int    `json:"id"`
	TargetType string `json:"target_type"`
	TargetId   int    `json:"target_id"`
}

type enterpriseProjectItemForTest struct {
	Id          int `json:"id"`
	OwnerUserId int `json:"owner_user_id"`
}

type enterpriseQuotaRequestItemForTest struct {
	Id              int    `json:"id"`
	ApplicantUserId int    `json:"applicant_user_id"`
	Status          string `json:"status"`
}

type enterpriseUsageSummaryResponseForTest struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total struct {
			RequestCount int64 `json:"request_count"`
			Quota        int64 `json:"quota"`
		} `json:"total"`
	} `json:"data"`
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

func TestEnterpriseRBACDepartmentAdminScope(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	engineeringId := seedEnterpriseOrgUnitForTest(t, enterprise.Id, 0, "Engineering", "engineering")
	platformId := seedEnterpriseOrgUnitForTest(t, enterprise.Id, engineeringId, "Platform", "platform")
	salesId := seedEnterpriseOrgUnitForTest(t, enterprise.Id, 0, "Sales", "sales")
	departmentAdminId := 7301
	engineerId := 7302
	platformUserId := 7303
	salesUserId := 7304
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, departmentAdminId, service.EnterpriseRoleDepartmentAdmin, engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, engineerId, "", engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, platformUserId, "", platformId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, salesUserId, "", salesId)
	engineeringPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Engineering Policy", model.PolicyTargetOrgUnit, engineeringId)
	salesPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Sales Policy", model.PolicyTargetOrgUnit, salesId)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseQuotaRequest{
		{
			EnterpriseId:    enterprise.Id,
			ApplicantUserId: engineerId,
			PolicyId:        engineeringPolicy.Id,
			TargetType:      engineeringPolicy.TargetType,
			TargetId:        engineeringPolicy.TargetId,
			Metric:          engineeringPolicy.Metric,
			Period:          engineeringPolicy.Period,
			LimitDelta:      1,
			Status:          model.EnterpriseQuotaRequestStatusPending,
			EffectiveAt:     common.GetTimestamp(),
			ExpiresAt:       common.GetTimestamp() + 3600,
		},
		{
			EnterpriseId:    enterprise.Id,
			ApplicantUserId: salesUserId,
			PolicyId:        salesPolicy.Id,
			TargetType:      salesPolicy.TargetType,
			TargetId:        salesPolicy.TargetId,
			Metric:          salesPolicy.Metric,
			Period:          salesPolicy.Period,
			LimitDelta:      1,
			Status:          model.EnterpriseQuotaRequestStatusPending,
			EffectiveAt:     common.GetTimestamp(),
			ExpiresAt:       common.GetTimestamp() + 3600,
		},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseUsageAttribution{
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "dept-usage",
			UserId:       engineerId,
			OrgUnitId:    engineeringId,
			ModelName:    "gpt-4o",
			Quota:        100,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "sales-usage",
			UserId:       salesUserId,
			OrgUnitId:    salesId,
			ModelName:    "gpt-4o",
			Quota:        500,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
	}).Error)

	router := newEnterpriseRouterForTest(t)
	departmentCookies := loginEnterpriseRouterUserForTest(t, router, departmentAdminId, common.RoleCommonUser)

	members := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/members?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, members).Success)
	memberPage := decodeEnterprisePageResponseForTest[enterpriseMemberItemForTest](t, members)
	require.ElementsMatch(t, []int{departmentAdminId, engineerId, platformUserId}, enterpriseMemberUserIdsForTest(memberPage.Data.Items))

	policies := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/quota-policies?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, policies).Success)
	policyPage := decodeEnterprisePageResponseForTest[enterpriseQuotaPolicyItemForTest](t, policies)
	require.Len(t, policyPage.Data.Items, 1)
	require.Equal(t, engineeringPolicy.Id, policyPage.Data.Items[0].Id)

	createScopedPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Platform Scoped",
    "target_type": "org_unit",
    "target_id": `+strconv.Itoa(platformId)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, createScopedPolicy).Success)

	createCrossPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Sales Cross",
    "target_type": "org_unit",
    "target_id": `+strconv.Itoa(salesId)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	crossPolicyResponse := decodeEnterpriseAuthResponse(t, createCrossPolicy)
	require.False(t, crossPolicyResponse.Success)
	require.Contains(t, crossPolicyResponse.Message, "权限范围外")

	quotaRequests := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/quota-requests?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, quotaRequests).Success)
	requestPage := decodeEnterprisePageResponseForTest[enterpriseQuotaRequestItemForTest](t, quotaRequests)
	require.Len(t, requestPage.Data.Items, 1)
	require.Equal(t, engineerId, requestPage.Data.Items[0].ApplicantUserId)

	var engineeringRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.Where("applicant_user_id = ?", engineerId).First(&engineeringRequest).Error)
	var salesRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.Where("applicant_user_id = ?", salesUserId).First(&salesRequest).Error)
	approveScoped := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests/"+strconv.Itoa(engineeringRequest.Id)+"/approve", `{}`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, approveScoped).Success)
	approveCross := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-requests/"+strconv.Itoa(salesRequest.Id)+"/approve", `{}`, departmentCookies, departmentAdminId)
	approveCrossResponse := decodeEnterpriseAuthResponse(t, approveCross)
	require.False(t, approveCrossResponse.Success)
	require.Contains(t, approveCrossResponse.Message, "权限范围外")

	usageSummary := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100", "", departmentCookies, departmentAdminId)
	usageResponse := decodeEnterpriseUsageSummaryResponseForTest(t, usageSummary)
	require.True(t, usageResponse.Success, usageResponse.Message)
	require.EqualValues(t, 1, usageResponse.Data.Total.RequestCount)
	require.EqualValues(t, 100, usageResponse.Data.Total.Quota)

	usageExport := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/breakdown/export?start_time=900&end_time=1100&dimension=org_unit", "", departmentCookies, departmentAdminId)
	require.Equal(t, http.StatusOK, usageExport.Code)
	require.Contains(t, usageExport.Header().Get("Content-Type"), "text/csv")
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(usageExport.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF})))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, strconv.Itoa(engineeringId), records[1][1])
	require.Equal(t, "100", records[1][7])

	crossUsage := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100&org_unit_id="+strconv.Itoa(salesId), "", departmentCookies, departmentAdminId)
	crossUsageResponse := decodeEnterpriseUsageSummaryResponseForTest(t, crossUsage)
	require.False(t, crossUsageResponse.Success)
	require.Contains(t, crossUsageResponse.Message, "权限范围外")

	updateCrossMember := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/members/"+strconv.Itoa(salesUserId)+"/org-unit", `{
    "org_unit_id": `+strconv.Itoa(platformId)+`
  }`, departmentCookies, departmentAdminId)
	updateCrossMemberResponse := decodeEnterpriseAuthResponse(t, updateCrossMember)
	require.False(t, updateCrossMemberResponse.Success)
	require.Contains(t, updateCrossMemberResponse.Message, "权限范围外")
}

func TestEnterpriseRBACProjectAdminFinanceScope(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	projectAdminId := 7401
	otherOwnerId := 7402
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectAdminId, service.EnterpriseRoleProjectAdmin)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, otherOwnerId, "")
	adminProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Owned Project",
		Slug:         "owned-project",
		OwnerUserId:  projectAdminId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	otherProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Other Project",
		Slug:         "other-project",
		OwnerUserId:  otherOwnerId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&adminProject).Error)
	require.NoError(t, model.DB.Create(&otherProject).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseUsageAttribution{
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "owned-project-usage",
			UserId:       projectAdminId,
			ProjectId:    adminProject.Id,
			ModelName:    "gpt-4o",
			Quota:        120,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "other-project-usage",
			UserId:       otherOwnerId,
			ProjectId:    otherProject.Id,
			ModelName:    "gpt-4o",
			Quota:        800,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
	}).Error)

	router := newEnterpriseRouterForTest(t)
	projectAdminCookies := loginEnterpriseRouterUserForTest(t, router, projectAdminId, common.RoleCommonUser)

	projects := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects?page_size=20", "", projectAdminCookies, projectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, projects).Success)
	projectPage := decodeEnterprisePageResponseForTest[enterpriseProjectItemForTest](t, projects)
	require.Len(t, projectPage.Data.Items, 1)
	require.Equal(t, adminProject.Id, projectPage.Data.Items[0].Id)

	summary := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100", "", projectAdminCookies, projectAdminId)
	summaryResponse := decodeEnterpriseUsageSummaryResponseForTest(t, summary)
	require.True(t, summaryResponse.Success, summaryResponse.Message)
	require.EqualValues(t, 1, summaryResponse.Data.Total.RequestCount)
	require.EqualValues(t, 120, summaryResponse.Data.Total.Quota)

	crossProject := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100&project_id="+strconv.Itoa(otherProject.Id), "", projectAdminCookies, projectAdminId)
	crossProjectResponse := decodeEnterpriseUsageSummaryResponseForTest(t, crossProject)
	require.False(t, crossProjectResponse.Success)
	require.Contains(t, crossProjectResponse.Message, "权限范围外")

	export := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/breakdown/export?start_time=900&end_time=1100&dimension=project", "", projectAdminCookies, projectAdminId)
	require.Equal(t, http.StatusOK, export.Code)
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(export.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF})))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, strconv.Itoa(adminProject.Id), records[1][1])
	require.Equal(t, "120", records[1][7])
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

func seedEnterpriseOrgUnitForTest(t *testing.T, enterpriseId int, parentId int, name string, slug string) int {
	t.Helper()
	path := "/"
	depth := 0
	if parentId > 0 {
		var parent model.EnterpriseOrgUnit
		require.NoError(t, model.DB.Where("enterprise_id = ? AND id = ?", enterpriseId, parentId).First(&parent).Error)
		path = parent.Path
		depth = parent.Depth + 1
	}
	unit := model.EnterpriseOrgUnit{
		EnterpriseId: enterpriseId,
		ParentId:     parentId,
		Name:         name,
		Slug:         slug,
		Path:         "",
		Depth:        depth,
		Status:       model.OrgUnitStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&unit).Error)
	unit.Path = path + strconv.Itoa(unit.Id) + "/"
	require.NoError(t, model.DB.Save(&unit).Error)
	return unit.Id
}

func seedEnterpriseMembershipRoleForTest(t *testing.T, enterpriseId int, userId int, role string, orgUnitIds ...int) {
	t.Helper()
	require.NoError(t, model.DB.Where("id = ?", userId).Attrs(model.User{
		Username: "enterprise-rbac-test-" + strconv.Itoa(userId),
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		AffCode:  "enterprise-rbac-test-" + strconv.Itoa(userId),
	}).FirstOrCreate(&model.User{Id: userId}).Error)
	orgUnitId := 0
	if len(orgUnitIds) > 0 {
		orgUnitId = orgUnitIds[0]
	}
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterpriseId,
		UserId:       userId,
		OrgUnitId:    orgUnitId,
		Role:         role,
		IsPrimary:    true,
	}).Error)
}

func seedEnterpriseQuotaPolicyForTest(t *testing.T, enterpriseId int, name string, targetType string, targetId int) model.EnterpriseQuotaPolicy {
	t.Helper()
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterpriseId,
		Name:         name,
		TargetType:   targetType,
		TargetId:     targetId,
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
	return policy
}

func enterpriseMemberUserIdsForTest(items []enterpriseMemberItemForTest) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.UserId)
	}
	return ids
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

func decodeEnterprisePageResponseForTest[T any](t *testing.T, recorder *httptest.ResponseRecorder) enterprisePageResponseForTest[T] {
	t.Helper()
	var response enterprisePageResponseForTest[T]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeEnterpriseUsageSummaryResponseForTest(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseUsageSummaryResponseForTest {
	t.Helper()
	var response enterpriseUsageSummaryResponseForTest
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeEnterpriseNotificationResponseForTest(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseNotificationResponseForTest {
	t.Helper()
	var response enterpriseNotificationResponseForTest
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
