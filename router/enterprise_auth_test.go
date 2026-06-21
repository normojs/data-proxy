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
	UserId    int    `json:"user_id"`
	OrgUnitId int    `json:"org_unit_id"`
	Role      string `json:"role"`
}

type enterpriseQuotaPolicyItemForTest struct {
	Id         int    `json:"id"`
	TargetType string `json:"target_type"`
	TargetId   int    `json:"target_id"`
}

type enterprisePolicyGroupItemForTest struct {
	Id               int   `json:"id"`
	OrgUnitId        int   `json:"org_unit_id"`
	SharedOrgUnitIds []int `json:"shared_org_unit_ids"`
	SharedExpiresAt  int64 `json:"shared_expires_at"`
	CanManage        bool  `json:"can_manage"`
}

type enterprisePolicyGroupShareRequestItemForTest struct {
	Id                 int    `json:"id"`
	PolicyGroupId      int    `json:"policy_group_id"`
	RequesterOrgUnitId int    `json:"requester_org_unit_id"`
	TargetOrgUnitId    int    `json:"target_org_unit_id"`
	Status             string `json:"status"`
	CanDecide          bool   `json:"can_decide"`
}

type enterpriseProjectItemForTest struct {
	Id          int    `json:"id"`
	OwnerUserId int    `json:"owner_user_id"`
	MemberRole  string `json:"member_role"`
	CanManage   bool   `json:"can_manage"`
	MemberCount int64  `json:"member_count"`
}

type enterpriseQuotaRequestItemForTest struct {
	Id              int    `json:"id"`
	ApplicantUserId int    `json:"applicant_user_id"`
	Status          string `json:"status"`
}

type enterpriseProjectMemberItemForTest struct {
	UserId int    `json:"user_id"`
	Role   string `json:"role"`
}

type enterpriseAuditLogItemForTest struct {
	Id             int64  `json:"id"`
	Action         string `json:"action"`
	TargetType     string `json:"target_type"`
	TargetId       int    `json:"target_id"`
	ScopeUserId    int    `json:"scope_user_id"`
	ScopeOrgUnitId int    `json:"scope_org_unit_id"`
	ScopeProjectId int    `json:"scope_project_id"`
	RequestId      string `json:"request_id"`
}

type enterpriseQueueAdmissionItemForTest struct {
	Id        int64  `json:"id"`
	RequestId string `json:"request_id"`
	Status    string `json:"status"`
	OrgUnitId int    `json:"org_unit_id"`
	ProjectId int    `json:"project_id"`
	PolicyId  int    `json:"policy_id"`
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

	financeQueueAdmissions := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/queue-admissions", "", financeCookies, financeUserId)
	require.Equal(t, http.StatusOK, financeQueueAdmissions.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, financeQueueAdmissions).Success)

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

	auditQueueAdmissions := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/queue-admissions", "", auditCookies, auditUserId)
	require.Equal(t, http.StatusOK, auditQueueAdmissions.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, auditQueueAdmissions).Success)

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
	salesDepartmentAdminId := 7305
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, departmentAdminId, service.EnterpriseRoleDepartmentAdmin, engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, engineerId, "", engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, platformUserId, "", platformId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, salesUserId, "", salesId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, salesDepartmentAdminId, service.EnterpriseRoleDepartmentAdmin, salesId)
	engineeringPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Engineering Policy", model.PolicyTargetOrgUnit, engineeringId)
	salesPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Sales Policy", model.PolicyTargetOrgUnit, salesId)
	globalGroup := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		Name:         "Global Pilot",
		Slug:         "global-pilot",
		Status:       model.PolicyGroupStatusEnabled,
	}
	engineeringGroup := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		OrgUnitId:    engineeringId,
		Name:         "Engineering Pilot",
		Slug:         "engineering-pilot",
		Status:       model.PolicyGroupStatusEnabled,
	}
	salesGroup := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		OrgUnitId:    salesId,
		Name:         "Sales Pilot",
		Slug:         "sales-pilot",
		Status:       model.PolicyGroupStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&globalGroup).Error)
	require.NoError(t, model.DB.Create(&engineeringGroup).Error)
	require.NoError(t, model.DB.Create(&salesGroup).Error)
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
	enterpriseAdminCookies := loginEnterpriseRouterUserForTest(t, router, 1, common.RoleAdminUser)
	departmentCookies := loginEnterpriseRouterUserForTest(t, router, departmentAdminId, common.RoleCommonUser)
	salesDepartmentCookies := loginEnterpriseRouterUserForTest(t, router, salesDepartmentAdminId, common.RoleCommonUser)

	members := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/members?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, members).Success)
	memberPage := decodeEnterprisePageResponseForTest[enterpriseMemberItemForTest](t, members)
	require.ElementsMatch(t, []int{departmentAdminId, engineerId, platformUserId}, enterpriseMemberUserIdsForTest(memberPage.Data.Items))

	policies := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/quota-policies?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, policies).Success)
	policyPage := decodeEnterprisePageResponseForTest[enterpriseQuotaPolicyItemForTest](t, policies)
	require.Len(t, policyPage.Data.Items, 1)
	require.Equal(t, engineeringPolicy.Id, policyPage.Data.Items[0].Id)

	policyGroups := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, policyGroups).Success)
	policyGroupPage := decodeEnterprisePageResponseForTest[enterprisePolicyGroupItemForTest](t, policyGroups)
	require.ElementsMatch(t, []int{engineeringGroup.Id}, enterprisePolicyGroupIdsForTest(policyGroupPage.Data.Items))
	require.Equal(t, engineeringId, policyGroupPage.Data.Items[0].OrgUnitId)
	require.True(t, policyGroupPage.Data.Items[0].CanManage)

	shareRequestExpiresAt := common.GetTimestamp() + 7200
	createShareRequest := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(engineeringGroup.Id)+"/share-requests", `{
    "org_unit_id": `+strconv.Itoa(salesId)+`,
    "shared_expires_at": `+strconv.FormatInt(shareRequestExpiresAt, 10)+`,
    "reason": "sales collaboration"
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, createShareRequest).Success)
	var shareRequest model.EnterprisePolicyGroupShareRequest
	require.NoError(t, model.DB.Where("enterprise_id = ? AND policy_group_id = ? AND target_org_unit_id = ?", enterprise.Id, engineeringGroup.Id, salesId).First(&shareRequest).Error)
	require.Equal(t, model.PolicyGroupShareRequestStatusPending, shareRequest.Status)
	require.Equal(t, engineeringId, shareRequest.RequesterOrgUnitId)
	require.Equal(t, salesId, shareRequest.TargetOrgUnitId)

	salesPolicyGroupsBeforeApproval := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups?page_size=20", "", salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, salesPolicyGroupsBeforeApproval).Success)
	salesPolicyGroupPageBeforeApproval := decodeEnterprisePageResponseForTest[enterprisePolicyGroupItemForTest](t, salesPolicyGroupsBeforeApproval)
	require.NotContains(t, enterprisePolicyGroupIdsForTest(salesPolicyGroupPageBeforeApproval.Data.Items), engineeringGroup.Id)

	outgoingShareRequests := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-group-share-requests?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, outgoingShareRequests).Success)
	outgoingShareRequestPage := decodeEnterprisePageResponseForTest[enterprisePolicyGroupShareRequestItemForTest](t, outgoingShareRequests)
	require.Len(t, outgoingShareRequestPage.Data.Items, 1)
	require.Equal(t, shareRequest.Id, outgoingShareRequestPage.Data.Items[0].Id)
	require.False(t, outgoingShareRequestPage.Data.Items[0].CanDecide)

	incomingShareRequests := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-group-share-requests?page_size=20", "", salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, incomingShareRequests).Success)
	incomingShareRequestPage := decodeEnterprisePageResponseForTest[enterprisePolicyGroupShareRequestItemForTest](t, incomingShareRequests)
	require.Len(t, incomingShareRequestPage.Data.Items, 1)
	require.Equal(t, shareRequest.Id, incomingShareRequestPage.Data.Items[0].Id)
	require.True(t, incomingShareRequestPage.Data.Items[0].CanDecide)

	approveAsRequester := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-group-share-requests/"+strconv.Itoa(shareRequest.Id)+"/approve", `{}`, departmentCookies, departmentAdminId)
	approveAsRequesterResponse := decodeEnterpriseAuthResponse(t, approveAsRequester)
	require.False(t, approveAsRequesterResponse.Success)
	require.Contains(t, approveAsRequesterResponse.Message, "权限范围外")

	approveShareRequest := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-group-share-requests/"+strconv.Itoa(shareRequest.Id)+"/approve", `{
    "decision_reason": "approved for campaign"
  }`, salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, approveShareRequest).Success)
	require.NoError(t, model.DB.First(&shareRequest, shareRequest.Id).Error)
	require.Equal(t, model.PolicyGroupShareRequestStatusApproved, shareRequest.Status)
	require.Equal(t, salesDepartmentAdminId, shareRequest.ApproverUserId)
	var approvedShare model.EnterprisePolicyGroupShare
	require.NoError(t, model.DB.Where("enterprise_id = ? AND policy_group_id = ? AND org_unit_id = ?", enterprise.Id, engineeringGroup.Id, salesId).First(&approvedShare).Error)
	require.Equal(t, shareRequestExpiresAt, approvedShare.ExpiresAt)

	salesPolicyGroupsAfterApproval := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups?page_size=20", "", salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, salesPolicyGroupsAfterApproval).Success)
	salesPolicyGroupPageAfterApproval := decodeEnterprisePageResponseForTest[enterprisePolicyGroupItemForTest](t, salesPolicyGroupsAfterApproval)
	require.Contains(t, enterprisePolicyGroupIdsForTest(salesPolicyGroupPageAfterApproval.Data.Items), engineeringGroup.Id)
	salesGroupsById := enterprisePolicyGroupsByIdForTest(salesPolicyGroupPageAfterApproval.Data.Items)
	require.False(t, salesGroupsById[engineeringGroup.Id].CanManage)
	require.ElementsMatch(t, []int{salesId}, salesGroupsById[engineeringGroup.Id].SharedOrgUnitIds)

	createSharedPolicyGroupPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Engineering Shared For Sales",
    "target_type": "policy_group",
    "target_id": `+strconv.Itoa(engineeringGroup.Id)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
  }`, salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, createSharedPolicyGroupPolicy).Success)

	engineeringShareRequestAudits := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs?target_type=policy_group_share_request&page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, engineeringShareRequestAudits).Success)
	engineeringShareRequestAuditPage := decodeEnterprisePageResponseForTest[enterpriseAuditLogItemForTest](t, engineeringShareRequestAudits)
	require.Len(t, engineeringShareRequestAuditPage.Data.Items, 2)
	salesShareRequestAudits := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs?target_type=policy_group_share_request&page_size=20", "", salesDepartmentCookies, salesDepartmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, salesShareRequestAudits).Success)
	salesShareRequestAuditPage := decodeEnterprisePageResponseForTest[enterpriseAuditLogItemForTest](t, salesShareRequestAudits)
	require.Len(t, salesShareRequestAuditPage.Data.Items, 2)

	shareExpiresAt := common.GetTimestamp() + 3600
	shareSalesGroup := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/policy-groups/"+strconv.Itoa(salesGroup.Id), `{
    "name": "Sales Pilot",
    "slug": "sales-pilot",
    "description": "shared with engineering",
    "shared_org_unit_ids": [`+strconv.Itoa(engineeringId)+`],
    "shared_expires_at": `+strconv.FormatInt(shareExpiresAt, 10)+`,
    "status": 1
  }`, enterpriseAdminCookies, 1)
	require.True(t, decodeEnterpriseAuthResponse(t, shareSalesGroup).Success)

	sharedPolicyGroups := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, sharedPolicyGroups).Success)
	sharedPolicyGroupPage := decodeEnterprisePageResponseForTest[enterprisePolicyGroupItemForTest](t, sharedPolicyGroups)
	require.ElementsMatch(t, []int{engineeringGroup.Id, salesGroup.Id}, enterprisePolicyGroupIdsForTest(sharedPolicyGroupPage.Data.Items))
	sharedGroupsById := enterprisePolicyGroupsByIdForTest(sharedPolicyGroupPage.Data.Items)
	require.True(t, sharedGroupsById[engineeringGroup.Id].CanManage)
	require.False(t, sharedGroupsById[salesGroup.Id].CanManage)
	require.ElementsMatch(t, []int{engineeringId}, sharedGroupsById[salesGroup.Id].SharedOrgUnitIds)
	require.Equal(t, shareExpiresAt, sharedGroupsById[salesGroup.Id].SharedExpiresAt)

	createPolicyGroup := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups", `{
    "name": "Department Created",
    "slug": "department-created",
    "description": "created by department admin",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, createPolicyGroup).Success)
	var departmentGroup model.EnterprisePolicyGroup
	require.NoError(t, model.DB.Where("slug = ?", "department-created").First(&departmentGroup).Error)
	require.Equal(t, engineeringId, departmentGroup.OrgUnitId)

	updateScopedGroup := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/policy-groups/"+strconv.Itoa(departmentGroup.Id), `{
    "name": "Department Created Updated",
    "slug": "department-created",
    "description": "updated by department admin",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, updateScopedGroup).Success)

	addScopedMember := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(departmentGroup.Id)+"/members", `{
	    "user_ids": [`+strconv.Itoa(engineerId)+`, `+strconv.Itoa(platformUserId)+`]
	  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, addScopedMember).Success)
	var viewerMember model.EnterprisePolicyGroupMember
	require.NoError(t, model.DB.Where("enterprise_id = ? AND policy_group_id = ? AND user_id = ?", enterprise.Id, departmentGroup.Id, engineerId).First(&viewerMember).Error)
	require.Equal(t, model.PolicyGroupMemberRoleViewer, viewerMember.Role)

	updateScopedMemberRole := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(departmentGroup.Id)+"/members", `{
    "user_ids": [`+strconv.Itoa(engineerId)+`],
    "role": "editor"
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, updateScopedMemberRole).Success)
	require.NoError(t, model.DB.Where("enterprise_id = ? AND policy_group_id = ? AND user_id = ?", enterprise.Id, departmentGroup.Id, engineerId).First(&viewerMember).Error)
	require.Equal(t, model.PolicyGroupMemberRoleEditor, viewerMember.Role)

	scopedMembers := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups/"+strconv.Itoa(departmentGroup.Id)+"/members?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, scopedMembers).Success)
	scopedMemberPage := decodeEnterprisePageResponseForTest[enterpriseMemberItemForTest](t, scopedMembers)
	scopedMembersById := enterpriseMembersByIdForTest(scopedMemberPage.Data.Items)
	require.Equal(t, model.PolicyGroupMemberRoleEditor, scopedMembersById[engineerId].Role)
	require.Equal(t, model.PolicyGroupMemberRoleViewer, scopedMembersById[platformUserId].Role)

	addCrossMember := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(departmentGroup.Id)+"/members", `{
		    "user_ids": [`+strconv.Itoa(salesUserId)+`]
	  }`, departmentCookies, departmentAdminId)
	addCrossMemberResponse := decodeEnterpriseAuthResponse(t, addCrossMember)
	require.False(t, addCrossMemberResponse.Success)
	require.Contains(t, addCrossMemberResponse.Message, "权限范围外")

	updateCrossGroup := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/policy-groups/"+strconv.Itoa(salesGroup.Id), `{
    "name": "Sales Cross Update",
    "slug": "sales-pilot",
    "description": "cross department",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	updateCrossGroupResponse := decodeEnterpriseAuthResponse(t, updateCrossGroup)
	require.False(t, updateCrossGroupResponse.Success)
	require.Contains(t, updateCrossGroupResponse.Message, "权限范围外")

	addSharedGroupMember := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(salesGroup.Id)+"/members", `{
    "user_ids": [`+strconv.Itoa(engineerId)+`],
    "role": "viewer"
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, addSharedGroupMember).Success)

	addSharedGroupCrossMember := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/policy-groups/"+strconv.Itoa(salesGroup.Id)+"/members", `{
    "user_ids": [`+strconv.Itoa(salesUserId)+`]
  }`, departmentCookies, departmentAdminId)
	addSharedGroupCrossMemberResponse := decodeEnterpriseAuthResponse(t, addSharedGroupCrossMember)
	require.False(t, addSharedGroupCrossMemberResponse.Success)
	require.Contains(t, addSharedGroupCrossMemberResponse.Message, "权限范围外")

	updateGlobalGroup := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/policy-groups/"+strconv.Itoa(globalGroup.Id), `{
    "name": "Global Cross Update",
    "slug": "global-pilot",
    "description": "global group",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	updateGlobalGroupResponse := decodeEnterpriseAuthResponse(t, updateGlobalGroup)
	require.False(t, updateGlobalGroupResponse.Success)
	require.Contains(t, updateGlobalGroupResponse.Message, "权限范围外")

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

	createPolicyGroupPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Department Group Scoped",
    "target_type": "policy_group",
    "target_id": `+strconv.Itoa(departmentGroup.Id)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, createPolicyGroupPolicy).Success)

	createCrossPolicyGroupPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Sales Group Shared",
    "target_type": "policy_group",
    "target_id": `+strconv.Itoa(salesGroup.Id)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
	  }`, departmentCookies, departmentAdminId)
	createCrossPolicyGroupPolicyResponse := decodeEnterpriseAuthResponse(t, createCrossPolicyGroupPolicy)
	require.True(t, createCrossPolicyGroupPolicyResponse.Success, createCrossPolicyGroupPolicyResponse.Message)

	expireSalesGroupShare := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/policy-groups/"+strconv.Itoa(salesGroup.Id), `{
    "name": "Sales Pilot",
    "slug": "sales-pilot",
    "description": "expired share with engineering",
    "shared_org_unit_ids": [`+strconv.Itoa(engineeringId)+`],
    "shared_expires_at": `+strconv.FormatInt(common.GetTimestamp()-60, 10)+`,
    "status": 1
  }`, enterpriseAdminCookies, 1)
	require.True(t, decodeEnterpriseAuthResponse(t, expireSalesGroupShare).Success)

	expiredSharedPolicyGroups := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/policy-groups?page_size=20", "", departmentCookies, departmentAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, expiredSharedPolicyGroups).Success)
	expiredSharedPolicyGroupPage := decodeEnterprisePageResponseForTest[enterprisePolicyGroupItemForTest](t, expiredSharedPolicyGroups)
	require.NotContains(t, enterprisePolicyGroupIdsForTest(expiredSharedPolicyGroupPage.Data.Items), salesGroup.Id)

	createExpiredSharedPolicyGroupPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Sales Group Expired Share",
    "target_type": "policy_group",
    "target_id": `+strconv.Itoa(salesGroup.Id)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
	  }`, departmentCookies, departmentAdminId)
	createExpiredSharedPolicyGroupPolicyResponse := decodeEnterpriseAuthResponse(t, createExpiredSharedPolicyGroupPolicy)
	require.False(t, createExpiredSharedPolicyGroupPolicyResponse.Success)
	require.Contains(t, createExpiredSharedPolicyGroupPolicyResponse.Message, "权限范围外")

	createGlobalPolicyGroupPolicy := requestEnterpriseForTest(t, router, http.MethodPost, "/api/enterprise/quota-policies", `{
    "name": "Global Group Cross",
    "target_type": "policy_group",
    "target_id": `+strconv.Itoa(globalGroup.Id)+`,
    "metric": "request_count",
    "period": "day",
    "limit_value": 5,
    "timezone": "Asia/Shanghai",
    "model_scope": "all",
    "action": "reject",
    "status": 1
  }`, departmentCookies, departmentAdminId)
	createGlobalPolicyGroupPolicyResponse := decodeEnterpriseAuthResponse(t, createGlobalPolicyGroupPolicy)
	require.False(t, createGlobalPolicyGroupPolicyResponse.Success)
	require.Contains(t, createGlobalPolicyGroupPolicyResponse.Message, "权限范围外")

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
	require.True(t, projectPage.Data.Items[0].CanManage)

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

func TestEnterpriseRBACProjectAdminMemberScope(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	projectAdminId := 7451
	projectMemberId := 7452
	ownerUserId := 7453
	otherProjectOwnerId := 7454
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectAdminId, service.EnterpriseRoleProjectAdmin)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectMemberId, service.EnterpriseRoleProjectAdmin)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, ownerUserId, "")
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, otherProjectOwnerId, "")
	adminProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Member Admin Project",
		Slug:         "member-admin-project",
		OwnerUserId:  ownerUserId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	otherProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Other Member Project",
		Slug:         "other-member-project",
		OwnerUserId:  otherProjectOwnerId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&adminProject).Error)
	require.NoError(t, model.DB.Create(&otherProject).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseProjectMember{
		{EnterpriseId: enterprise.Id, ProjectId: adminProject.Id, UserId: projectAdminId, Role: model.EnterpriseProjectMemberRoleAdmin},
		{EnterpriseId: enterprise.Id, ProjectId: otherProject.Id, UserId: projectMemberId, Role: model.EnterpriseProjectMemberRoleMember},
	}).Error)

	router := newEnterpriseRouterForTest(t)
	projectAdminCookies := loginEnterpriseRouterUserForTest(t, router, projectAdminId, common.RoleCommonUser)
	projectMemberCookies := loginEnterpriseRouterUserForTest(t, router, projectMemberId, common.RoleCommonUser)

	projects := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects?page_size=20", "", projectAdminCookies, projectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, projects).Success)
	projectPage := decodeEnterprisePageResponseForTest[enterpriseProjectItemForTest](t, projects)
	require.Len(t, projectPage.Data.Items, 1)
	require.Equal(t, adminProject.Id, projectPage.Data.Items[0].Id)
	require.EqualValues(t, 1, projectPage.Data.Items[0].MemberCount)

	members := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects/"+strconv.Itoa(adminProject.Id)+"/members", "", projectAdminCookies, projectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, members).Success)
	memberPage := decodeEnterprisePageResponseForTest[enterpriseProjectMemberItemForTest](t, members)
	require.Len(t, memberPage.Data.Items, 1)
	require.Equal(t, projectAdminId, memberPage.Data.Items[0].UserId)
	require.Equal(t, model.EnterpriseProjectMemberRoleAdmin, memberPage.Data.Items[0].Role)

	upsertMember := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/projects/"+strconv.Itoa(adminProject.Id)+"/members", `{
    "user_id": `+strconv.Itoa(projectMemberId)+`,
    "role": "admin"
  }`, projectAdminCookies, projectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, upsertMember).Success)

	var promotedMember model.EnterpriseProjectMember
	require.NoError(t, model.DB.Where("enterprise_id = ? AND project_id = ? AND user_id = ?", enterprise.Id, adminProject.Id, projectMemberId).First(&promotedMember).Error)
	require.Equal(t, model.EnterpriseProjectMemberRoleAdmin, promotedMember.Role)

	crossMembers := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects/"+strconv.Itoa(otherProject.Id)+"/members", "", projectAdminCookies, projectAdminId)
	crossMembersResponse := decodeEnterpriseAuthResponse(t, crossMembers)
	require.False(t, crossMembersResponse.Success)
	require.Contains(t, crossMembersResponse.Message, "权限范围外")

	memberOnlyProjects := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects?page_size=20", "", projectMemberCookies, projectMemberId)
	require.True(t, decodeEnterpriseAuthResponse(t, memberOnlyProjects).Success)
	memberOnlyProjectPage := decodeEnterprisePageResponseForTest[enterpriseProjectItemForTest](t, memberOnlyProjects)
	require.Len(t, memberOnlyProjectPage.Data.Items, 2)
	memberProjectsById := enterpriseProjectItemsByIdForTest(memberOnlyProjectPage.Data.Items)
	require.True(t, memberProjectsById[adminProject.Id].CanManage)
	require.Equal(t, model.EnterpriseProjectMemberRoleAdmin, memberProjectsById[adminProject.Id].MemberRole)
	require.False(t, memberProjectsById[otherProject.Id].CanManage)
	require.Equal(t, model.EnterpriseProjectMemberRoleMember, memberProjectsById[otherProject.Id].MemberRole)
}

func TestEnterpriseRBACProjectMemberReadOnlyScope(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	projectReaderId := 7461
	projectAdminId := 7462
	ownerUserId := 7463
	otherOwnerId := 7464
	noProjectAdminId := 7465
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectReaderId, service.EnterpriseRoleProjectAdmin)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectAdminId, service.EnterpriseRoleProjectAdmin)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, ownerUserId, "")
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, otherOwnerId, "")
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, noProjectAdminId, service.EnterpriseRoleProjectAdmin)
	readOnlyProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Read Only Project",
		Slug:         "read-only-project",
		OwnerUserId:  ownerUserId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	otherProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Hidden Project",
		Slug:         "hidden-project",
		OwnerUserId:  otherOwnerId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&readOnlyProject).Error)
	require.NoError(t, model.DB.Create(&otherProject).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseProjectMember{
		{EnterpriseId: enterprise.Id, ProjectId: readOnlyProject.Id, UserId: projectReaderId, Role: model.EnterpriseProjectMemberRoleMember},
		{EnterpriseId: enterprise.Id, ProjectId: readOnlyProject.Id, UserId: projectAdminId, Role: model.EnterpriseProjectMemberRoleAdmin},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseUsageAttribution{
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "reader-project-usage",
			UserId:       projectReaderId,
			ProjectId:    readOnlyProject.Id,
			ModelName:    "gpt-4o",
			Quota:        33,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "hidden-project-usage",
			UserId:       otherOwnerId,
			ProjectId:    otherProject.Id,
			ModelName:    "gpt-4o",
			Quota:        77,
			Status:       "succeeded",
			CreatedAt:    1000,
		},
	}).Error)

	router := newEnterpriseRouterForTest(t)
	readerCookies := loginEnterpriseRouterUserForTest(t, router, projectReaderId, common.RoleCommonUser)
	adminCookies := loginEnterpriseRouterUserForTest(t, router, projectAdminId, common.RoleCommonUser)
	noProjectCookies := loginEnterpriseRouterUserForTest(t, router, noProjectAdminId, common.RoleCommonUser)

	readerProjects := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects?page_size=20", "", readerCookies, projectReaderId)
	require.True(t, decodeEnterpriseAuthResponse(t, readerProjects).Success)
	readerProjectPage := decodeEnterprisePageResponseForTest[enterpriseProjectItemForTest](t, readerProjects)
	require.Len(t, readerProjectPage.Data.Items, 1)
	require.Equal(t, readOnlyProject.Id, readerProjectPage.Data.Items[0].Id)
	require.Equal(t, model.EnterpriseProjectMemberRoleMember, readerProjectPage.Data.Items[0].MemberRole)
	require.False(t, readerProjectPage.Data.Items[0].CanManage)

	readerMembers := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects/"+strconv.Itoa(readOnlyProject.Id)+"/members", "", readerCookies, projectReaderId)
	require.True(t, decodeEnterpriseAuthResponse(t, readerMembers).Success)
	readerMembersPage := decodeEnterprisePageResponseForTest[enterpriseProjectMemberItemForTest](t, readerMembers)
	require.Len(t, readerMembersPage.Data.Items, 2)

	readerSummary := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100", "", readerCookies, projectReaderId)
	readerSummaryResponse := decodeEnterpriseUsageSummaryResponseForTest(t, readerSummary)
	require.True(t, readerSummaryResponse.Success, readerSummaryResponse.Message)
	require.EqualValues(t, 1, readerSummaryResponse.Data.Total.RequestCount)
	require.EqualValues(t, 33, readerSummaryResponse.Data.Total.Quota)

	readerCrossProject := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100&project_id="+strconv.Itoa(otherProject.Id), "", readerCookies, projectReaderId)
	require.False(t, decodeEnterpriseUsageSummaryResponseForTest(t, readerCrossProject).Success)

	readerUpdateMember := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/projects/"+strconv.Itoa(readOnlyProject.Id)+"/members", `{
    "user_id": `+strconv.Itoa(otherOwnerId)+`,
    "role": "member"
  }`, readerCookies, projectReaderId)
	require.False(t, decodeEnterpriseAuthResponse(t, readerUpdateMember).Success)

	readerEditProject := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/projects/"+strconv.Itoa(readOnlyProject.Id), `{
    "name": "Reader Cannot Edit",
    "slug": "reader-cannot-edit",
    "owner_user_id": `+strconv.Itoa(ownerUserId)+`,
    "status": 1
  }`, readerCookies, projectReaderId)
	require.False(t, decodeEnterpriseAuthResponse(t, readerEditProject).Success)

	adminUpdateMember := requestEnterpriseForTest(t, router, http.MethodPut, "/api/enterprise/projects/"+strconv.Itoa(readOnlyProject.Id)+"/members", `{
    "user_id": `+strconv.Itoa(otherOwnerId)+`,
    "role": "member"
  }`, adminCookies, projectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, adminUpdateMember).Success)

	noProjectList := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/projects?page_size=20", "", noProjectCookies, noProjectAdminId)
	require.True(t, decodeEnterpriseAuthResponse(t, noProjectList).Success)
	noProjectPage := decodeEnterprisePageResponseForTest[enterpriseProjectItemForTest](t, noProjectList)
	require.Empty(t, noProjectPage.Data.Items)

	noProjectSummary := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=1100", "", noProjectCookies, noProjectAdminId)
	noProjectSummaryResponse := decodeEnterpriseUsageSummaryResponseForTest(t, noProjectSummary)
	require.True(t, noProjectSummaryResponse.Success, noProjectSummaryResponse.Message)
	require.EqualValues(t, 0, noProjectSummaryResponse.Data.Total.RequestCount)
}

func TestEnterpriseRBACScopedAuditLogs(t *testing.T) {
	setupEnterpriseRouterTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	engineeringId := seedEnterpriseOrgUnitForTest(t, enterprise.Id, 0, "Engineering", "engineering")
	salesId := seedEnterpriseOrgUnitForTest(t, enterprise.Id, 0, "Sales", "sales")
	departmentAdminId := 7501
	engineerId := 7502
	salesUserId := 7503
	projectAdminId := 7504
	otherProjectOwnerId := 7505
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, departmentAdminId, service.EnterpriseRoleDepartmentAdmin, engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, engineerId, "", engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, salesUserId, "", salesId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, projectAdminId, service.EnterpriseRoleProjectAdmin, engineeringId)
	seedEnterpriseMembershipRoleForTest(t, enterprise.Id, otherProjectOwnerId, "", salesId)
	engineeringPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Engineering Policy", model.PolicyTargetOrgUnit, engineeringId)
	salesPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Sales Policy", model.PolicyTargetOrgUnit, salesId)
	ownedProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Owned Project Audit",
		Slug:         "owned-project-audit",
		OwnerUserId:  projectAdminId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	otherProject := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Other Project Audit",
		Slug:         "other-project-audit",
		OwnerUserId:  otherProjectOwnerId,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&ownedProject).Error)
	require.NoError(t, model.DB.Create(&otherProject).Error)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseProjectOrgUnit{
		{EnterpriseId: enterprise.Id, ProjectId: ownedProject.Id, OrgUnitId: engineeringId},
		{EnterpriseId: enterprise.Id, ProjectId: otherProject.Id, OrgUnitId: salesId},
	}).Error)
	ownedProjectPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Owned Project Policy", model.PolicyTargetProject, ownedProject.Id)
	otherProjectPolicy := seedEnterpriseQuotaPolicyForTest(t, enterprise.Id, "Other Project Policy", model.PolicyTargetProject, otherProject.Id)
	engineeringQuotaRequest := model.EnterpriseQuotaRequest{
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
	}
	salesQuotaRequest := model.EnterpriseQuotaRequest{
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
	}
	require.NoError(t, model.DB.Create(&engineeringQuotaRequest).Error)
	require.NoError(t, model.DB.Create(&salesQuotaRequest).Error)
	audits := []model.EnterpriseAuditInput{
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  departmentAdminId,
			Action:       "org_unit.update",
			TargetType:   "org_unit",
			TargetId:     engineeringId,
			After:        gin.H{"name": "Engineering"},
			RequestId:    "audit-engineering-org",
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  departmentAdminId,
			Action:       "member.update_org_unit",
			TargetType:   "user",
			TargetId:     engineerId,
			After:        gin.H{"org_unit_id": engineeringId},
			RequestId:    "audit-engineering-user",
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  departmentAdminId,
			Action:       "quota_request.submit",
			TargetType:   "quota_request",
			TargetId:     engineeringQuotaRequest.Id,
			After:        engineeringQuotaRequest,
			RequestId:    "audit-engineering-request",
		},
		{
			EnterpriseId:   enterprise.Id,
			ActorUserId:    engineerId,
			Action:         "enterprise_governance.hard_limit_reject",
			TargetType:     "quota_policy",
			TargetId:       engineeringPolicy.Id,
			ScopeUserId:    engineerId,
			ScopeOrgUnitId: engineeringId,
			ScopeProjectId: ownedProject.Id,
			After:          gin.H{"org_unit_id": engineeringId, "project_id": ownedProject.Id},
			RequestId:      "audit-engineering-relay",
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  salesUserId,
			Action:       "org_unit.update",
			TargetType:   "org_unit",
			TargetId:     salesId,
			After:        gin.H{"name": "Sales"},
			RequestId:    "audit-sales-org",
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  salesUserId,
			Action:       "quota_request.submit",
			TargetType:   "quota_request",
			TargetId:     salesQuotaRequest.Id,
			After:        salesQuotaRequest,
			RequestId:    "audit-sales-request",
		},
		{
			EnterpriseId:   enterprise.Id,
			ActorUserId:    salesUserId,
			Action:         "enterprise_governance.hard_limit_reject",
			TargetType:     "quota_policy",
			TargetId:       otherProjectPolicy.Id,
			ScopeUserId:    salesUserId,
			ScopeOrgUnitId: salesId,
			ScopeProjectId: otherProject.Id,
			After:          gin.H{"org_unit_id": salesId, "project_id": otherProject.Id},
			RequestId:      "audit-other-project-relay",
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  7199,
			Action:       "webhook.create",
			TargetType:   "enterprise_webhook",
			TargetId:     1,
			After:        gin.H{"name": "global webhook"},
			RequestId:    "audit-global-webhook",
		},
	}
	for _, audit := range audits {
		require.NoError(t, model.RecordEnterpriseAuditLog(audit))
	}
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceQueueAdmission{
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "queue-engineering-relay",
			UserId:       engineerId,
			OrgUnitId:    engineeringId,
			ProjectId:    ownedProject.Id,
			PolicyId:     engineeringPolicy.Id,
			ModelName:    "gpt-4o",
			QueueKey:     "enterprise:1",
			Status:       model.EnterpriseGovernanceQueueAdmissionStatusAdmitted,
			CreatedAt:    1000,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "queue-owned-project-policy",
			UserId:       projectAdminId,
			OrgUnitId:    0,
			ProjectId:    0,
			PolicyId:     ownedProjectPolicy.Id,
			ModelName:    "gpt-4o",
			QueueKey:     "enterprise:1",
			Status:       model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
			CreatedAt:    1001,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "queue-sales-relay",
			UserId:       salesUserId,
			OrgUnitId:    salesId,
			ProjectId:    otherProject.Id,
			PolicyId:     otherProjectPolicy.Id,
			ModelName:    "gpt-4o-mini",
			QueueKey:     "enterprise:1",
			Status:       model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
			CreatedAt:    1002,
		},
		{
			EnterpriseId: enterprise.Id,
			RequestId:    "queue-global",
			UserId:       7199,
			PolicyId:     0,
			ModelName:    "gpt-4o",
			QueueKey:     "global",
			Status:       model.EnterpriseGovernanceQueueAdmissionStatusAdmitted,
			CreatedAt:    1003,
		},
	}).Error)

	router := newEnterpriseRouterForTest(t)
	departmentCookies := loginEnterpriseRouterUserForTest(t, router, departmentAdminId, common.RoleCommonUser)
	projectCookies := loginEnterpriseRouterUserForTest(t, router, projectAdminId, common.RoleCommonUser)

	departmentAudit := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs?page_size=50", "", departmentCookies, departmentAdminId)
	require.Equal(t, http.StatusOK, departmentAudit.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, departmentAudit).Success)
	departmentAuditPage := decodeEnterprisePageResponseForTest[enterpriseAuditLogItemForTest](t, departmentAudit)
	require.ElementsMatch(t, []string{
		"audit-engineering-org",
		"audit-engineering-user",
		"audit-engineering-request",
		"audit-engineering-relay",
	}, enterpriseAuditRequestIdsForTest(departmentAuditPage.Data.Items))
	for _, item := range departmentAuditPage.Data.Items {
		require.NotEqual(t, salesId, item.ScopeOrgUnitId)
		require.NotEqual(t, otherProject.Id, item.ScopeProjectId)
	}

	projectAudit := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/audit-logs?page_size=50", "", projectCookies, projectAdminId)
	require.Equal(t, http.StatusOK, projectAudit.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, projectAudit).Success)
	projectAuditPage := decodeEnterprisePageResponseForTest[enterpriseAuditLogItemForTest](t, projectAudit)
	require.ElementsMatch(t, []string{
		"audit-engineering-relay",
	}, enterpriseAuditRequestIdsForTest(projectAuditPage.Data.Items))
	require.Equal(t, ownedProject.Id, projectAuditPage.Data.Items[0].ScopeProjectId)

	departmentAdmissions := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/queue-admissions?page_size=50", "", departmentCookies, departmentAdminId)
	require.Equal(t, http.StatusOK, departmentAdmissions.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, departmentAdmissions).Success)
	departmentAdmissionPage := decodeEnterprisePageResponseForTest[enterpriseQueueAdmissionItemForTest](t, departmentAdmissions)
	require.ElementsMatch(t, []string{
		"queue-engineering-relay",
		"queue-owned-project-policy",
	}, enterpriseQueueAdmissionRequestIdsForTest(departmentAdmissionPage.Data.Items))

	projectAdmissions := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/queue-admissions?page_size=50", "", projectCookies, projectAdminId)
	require.Equal(t, http.StatusOK, projectAdmissions.Code)
	require.True(t, decodeEnterpriseAuthResponse(t, projectAdmissions).Success)
	projectAdmissionPage := decodeEnterprisePageResponseForTest[enterpriseQueueAdmissionItemForTest](t, projectAdmissions)
	require.ElementsMatch(t, []string{
		"queue-engineering-relay",
		"queue-owned-project-policy",
	}, enterpriseQueueAdmissionRequestIdsForTest(projectAdmissionPage.Data.Items))

	projectOutbox := requestEnterpriseForTest(t, router, http.MethodGet, "/api/enterprise/notification-outbox", "", projectCookies, projectAdminId)
	require.Equal(t, http.StatusOK, projectOutbox.Code)
	require.False(t, decodeEnterpriseAuthResponse(t, projectOutbox).Success)
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
		&model.EnterprisePolicyGroupShare{},
		&model.EnterprisePolicyGroupShareRequest{},
		&model.EnterpriseProject{},
		&model.EnterpriseProjectOrgUnit{},
		&model.EnterpriseProjectMember{},
		&model.EnterpriseQuotaPolicy{},
		&model.EnterpriseQuotaCounter{},
		&model.EnterpriseQuotaRequest{},
		&model.EnterpriseWebhook{},
		&model.EnterpriseUsageAttribution{},
		&model.EnterpriseGovernanceQueueAdmission{},
		&model.EnterpriseGovernanceAnomalyProtection{},
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

func enterpriseMembersByIdForTest(items []enterpriseMemberItemForTest) map[int]enterpriseMemberItemForTest {
	members := map[int]enterpriseMemberItemForTest{}
	for _, item := range items {
		members[item.UserId] = item
	}
	return members
}

func enterprisePolicyGroupIdsForTest(items []enterprisePolicyGroupItemForTest) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	return ids
}

func enterprisePolicyGroupsByIdForTest(items []enterprisePolicyGroupItemForTest) map[int]enterprisePolicyGroupItemForTest {
	groups := map[int]enterprisePolicyGroupItemForTest{}
	for _, item := range items {
		groups[item.Id] = item
	}
	return groups
}

func enterpriseProjectItemsByIdForTest(items []enterpriseProjectItemForTest) map[int]enterpriseProjectItemForTest {
	projects := map[int]enterpriseProjectItemForTest{}
	for _, item := range items {
		projects[item.Id] = item
	}
	return projects
}

func enterpriseAuditRequestIdsForTest(items []enterpriseAuditLogItemForTest) []string {
	requestIds := make([]string, 0, len(items))
	for _, item := range items {
		requestIds = append(requestIds, item.RequestId)
	}
	return requestIds
}

func enterpriseQueueAdmissionRequestIdsForTest(items []enterpriseQueueAdmissionItemForTest) []string {
	requestIds := make([]string, 0, len(items))
	for _, item := range items {
		requestIds = append(requestIds, item.RequestId)
	}
	return requestIds
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
