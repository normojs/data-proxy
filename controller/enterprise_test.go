package controller

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type enterpriseControllerResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func TestEnterpriseWebhookManagementAndTestSend(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	disableEnterpriseControllerWebhookSSRFProtection(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	var receivedSignature string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Enterprise-Webhook-Signature")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/webhooks", `{
    "name": "Approval Hook",
    "url": "`+server.URL+`?token=secret",
    "secret": "hook-secret",
    "event_types": ["quota_request.approve", "quota_request.approve", ""],
    "status": 1
  }`)
	CreateEnterpriseWebhook(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	webhookId := int(response.Data["id"].(float64))
	assert.Equal(t, "Approval Hook", response.Data["name"])
	assert.Equal(t, true, response.Data["has_secret"])
	assert.NotContains(t, recorder.Body.String(), "hook-secret")

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/webhooks", "")
	ListEnterpriseWebhooks(ctx)
	listResponse := decodeEnterpriseControllerListResponse(t, recorder)
	require.True(t, listResponse.Success, listResponse.Message)
	require.Len(t, listResponse.Data, 1)
	assert.NotContains(t, recorder.Body.String(), "hook-secret")

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/webhooks/"+itoaForEnterpriseTest(webhookId)+"/test", "{}")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(webhookId)}}
	TestEnterpriseWebhook(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.Equal(t, true, response.Data["success"])
	assert.EqualValues(t, http.StatusNoContent, response.Data["status_code"])
	assert.NotEmpty(t, receivedBody)
	assert.Equal(t, service.EnterpriseWebhookSignature("hook-secret", receivedBody), receivedSignature)
	assert.Contains(t, string(receivedBody), "enterprise.webhook.test")

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPut, "/api/enterprise/webhooks/"+itoaForEnterpriseTest(webhookId), `{
    "name": "Approval Hook Updated",
    "url": "`+server.URL+`/updated",
    "event_types": ["quota_request.reject"],
    "status": 2
  }`)
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(webhookId)}}
	UpdateEnterpriseWebhook(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.Equal(t, "Approval Hook Updated", response.Data["name"])
	assert.Equal(t, true, response.Data["has_secret"])
	assert.EqualValues(t, model.EnterpriseWebhookStatusDisabled, response.Data["status"])

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodDelete, "/api/enterprise/webhooks/"+itoaForEnterpriseTest(webhookId), "")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(webhookId)}}
	DeleteEnterpriseWebhook(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, model.EnterpriseWebhookStatusDisabled, response.Data["status"])

	var audits []model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("enterprise_id = ? AND target_type = ? AND target_id = ?", enterprise.Id, "enterprise_webhook", webhookId).Order("id asc").Find(&audits).Error)
	require.Len(t, audits, 4)
	assert.Equal(t, "webhook.create", audits[0].Action)
	assert.Equal(t, "webhook.test", audits[1].Action)
	assert.Equal(t, "webhook.update", audits[2].Action)
	assert.Equal(t, "webhook.disable", audits[3].Action)
	for _, audit := range audits {
		assert.NotContains(t, audit.BeforeJson, "hook-secret")
		assert.NotContains(t, audit.AfterJson, "hook-secret")
		assert.NotContains(t, audit.AfterJson, "token=secret")
	}
}

type enterpriseControllerListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    []any  `json:"data"`
}

type enterpriseOrgSyncControllerResponse struct {
	Success bool                            `json:"success"`
	Message string                          `json:"message"`
	Data    service.EnterpriseOrgSyncResult `json:"data"`
}

func setupEnterpriseControllerTestDB(t *testing.T) {
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
		&model.Token{},
		&model.Channel{},
		&model.Enterprise{},
		&model.EnterpriseOrgUnit{},
		&model.EnterpriseOrgMembership{},
		&model.EnterpriseProject{},
		&model.EnterpriseProjectOrgUnit{},
		&model.EnterpriseProjectMember{},
		&model.EnterprisePolicyGroup{},
		&model.EnterprisePolicyGroupMember{},
		&model.EnterprisePolicyGroupShare{},
		&model.EnterprisePolicyGroupShareRequest{},
		&model.EnterpriseQuotaPolicy{},
		&model.EnterpriseQuotaCounter{},
		&model.EnterpriseQuotaRequest{},
		&model.EnterpriseWebhook{},
		&model.EnterpriseUsageAttribution{},
		&model.EnterpriseGovernanceQueueAdmission{},
		&model.EnterpriseGovernanceQueuePayload{},
		&model.EnterpriseGovernanceSharedPoolConfig{},
		&model.EnterpriseGovernanceSharedPool{},
		&model.EnterpriseGovernanceSharedPoolBorrow{},
		&model.EnterpriseGovernanceAnomalyProtection{},
		&model.EnterpriseAuditLog{},
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

func newEnterpriseControllerContext(t *testing.T, method string, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set(common.RequestIdKey, "req-enterprise-controller-test")
	ctx.Set("id", 9001)
	return ctx, recorder
}

func decodeEnterpriseControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseControllerResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response enterpriseControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeEnterpriseControllerListResponse(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseControllerListResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response enterpriseControllerListResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeEnterpriseOrgSyncResponse(t *testing.T, recorder *httptest.ResponseRecorder) enterpriseOrgSyncControllerResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response enterpriseOrgSyncControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func disableEnterpriseControllerWebhookSSRFProtection(t *testing.T) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	originalEnabled := fetchSetting.EnableSSRFProtection
	originalAllowPrivate := fetchSetting.AllowPrivateIp
	fetchSetting.EnableSSRFProtection = false
	fetchSetting.AllowPrivateIp = true
	t.Cleanup(func() {
		fetchSetting.EnableSSRFProtection = originalEnabled
		fetchSetting.AllowPrivateIp = originalAllowPrivate
	})
}

func TestEnterpriseCurrentAnomalyThrottleConfigRoundTripAndAudit(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/current", "")
	GetCurrentEnterprise(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var current enterpriseCurrentItem
	decodeEnterpriseResponseData(t, response, &current)
	defaultConfig := service.DefaultEnterpriseAnomalyThrottleConfig()
	assert.True(t, current.AnomalyThrottleConfig.Enabled)
	assert.Equal(t, defaultConfig.CurrentWindowSeconds, current.AnomalyThrottleConfig.CurrentWindowSeconds)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPut, "/api/enterprise/current", `{
		"name": "Default Enterprise",
		"timezone": "UTC",
		"status": 1,
		"anomaly_throttle_config": {
			"enabled": false,
			"current_window_seconds": 0,
			"baseline_window_seconds": 600,
			"cooldown_seconds": 30,
			"request_spike_ratio": 3.5,
			"failure_rate": 1.7
		}
	}`)
	UpdateCurrentEnterprise(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var stored model.Enterprise
	require.NoError(t, model.DB.First(&stored, enterprise.Id).Error)
	storedConfig := service.EnterpriseAnomalyThrottleConfigFromJSON(stored.AnomalyThrottleConfigJson)
	assert.False(t, storedConfig.Enabled)
	assert.EqualValues(t, defaultConfig.CurrentWindowSeconds, storedConfig.CurrentWindowSeconds)
	assert.EqualValues(t, 600, storedConfig.BaselineWindowSeconds)
	assert.EqualValues(t, 30, storedConfig.CooldownSeconds)
	assert.Equal(t, 3.5, storedConfig.RequestSpikeRatio)
	assert.Equal(t, 1.0, storedConfig.FailureRate)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/current", "")
	GetCurrentEnterprise(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &current)
	assert.False(t, current.AnomalyThrottleConfig.Enabled)
	assert.EqualValues(t, 600, current.AnomalyThrottleConfig.BaselineWindowSeconds)
	assert.Equal(t, 3.5, current.AnomalyThrottleConfig.RequestSpikeRatio)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("enterprise_id = ? AND action = ?", enterprise.Id, "enterprise.update").First(&audit).Error)
	var auditAfter struct {
		AnomalyThrottleConfig service.EnterpriseAnomalyThrottleConfig `json:"anomaly_throttle_config"`
	}
	require.NoError(t, common.Unmarshal([]byte(audit.AfterJson), &auditAfter))
	assert.False(t, auditAfter.AnomalyThrottleConfig.Enabled)
	assert.Equal(t, 3.5, auditAfter.AnomalyThrottleConfig.RequestSpikeRatio)
}

func enterpriseResponseId(t *testing.T, response enterpriseControllerResponse) int {
	t.Helper()
	raw, ok := response.Data["id"].(float64)
	require.True(t, ok)
	return int(raw)
}

func createEnterpriseOrgUnitForTest(t *testing.T, parentId int, name string, slug string) int {
	t.Helper()
	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/org-units", `{
		"parent_id": `+itoaForEnterpriseTest(parentId)+`,
		"name": "`+name+`",
		"slug": "`+slug+`"
	}`)
	CreateEnterpriseOrgUnit(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	return enterpriseResponseId(t, response)
}

func TestEnterpriseOrgUnitRejectsMoveIntoDescendant(t *testing.T) {
	setupEnterpriseControllerTestDB(t)

	rootId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")
	childId := createEnterpriseOrgUnitForTest(t, rootId, "平台组", "platform")

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPut, "/api/enterprise/org-units/1", `{
		"parent_id": `+itoaForEnterpriseTest(childId)+`,
		"name": "研发部",
		"slug": "engineering"
	}`)
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(rootId)}}
	UpdateEnterpriseOrgUnit(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)

	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "子部门")
}

func TestEnterpriseOrgUnitDisableRejectsChildrenAndMembers(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 1003, Username: "carol", Status: common.UserStatusEnabled}).Error)

	rootId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")
	createEnterpriseOrgUnitForTest(t, rootId, "平台组", "platform")

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodDelete, "/api/enterprise/org-units/"+itoaForEnterpriseTest(rootId), "")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(rootId)}}
	DeleteEnterpriseOrgUnit(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "子部门")

	leafId := createEnterpriseOrgUnitForTest(t, 0, "销售部", "sales")
	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPut, "/api/enterprise/members/1003/org-unit", `{
		"org_unit_id": `+itoaForEnterpriseTest(leafId)+`
	}`)
	ctx.Params = gin.Params{{Key: "user_id", Value: "1003"}}
	UpdateEnterpriseMemberOrgUnit(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodDelete, "/api/enterprise/org-units/"+itoaForEnterpriseTest(leafId), "")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(leafId)}}
	DeleteEnterpriseOrgUnit(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "成员")
}

func TestEnterpriseMemberOrgUnitAssignment(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 1001, Username: "alice", Status: common.UserStatusEnabled}).Error)
	orgUnitId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPut, "/api/enterprise/members/1001/org-unit", `{
		"org_unit_id": `+itoaForEnterpriseTest(orgUnitId)+`
	}`)
	ctx.Params = gin.Params{{Key: "user_id", Value: "1001"}}
	UpdateEnterpriseMemberOrgUnit(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	var membership model.EnterpriseOrgMembership
	require.NoError(t, model.DB.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, 1001).First(&membership).Error)
	assert.Equal(t, orgUnitId, membership.OrgUnitId)
}

func TestEnterpriseOrgSyncPreviewAndApply(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 1101, Username: "alice", DisplayName: "Alice", Email: "alice@example.com", HStationId: "hs-alice", AffCode: "aff1101", Status: common.UserStatusEnabled},
		{Id: 1102, Username: "bob", DisplayName: "Bob", Email: "bob@example.com", AffCode: "aff1102", Status: common.UserStatusEnabled},
	}).Error)

	payload := `{
		"provider": "hstation",
		"snapshot_at": 1710000000,
		"org_units": [
			{"external_id": "eng", "name": "Engineering", "slug": "engineering", "sort": 10},
			{"external_id": "platform", "parent_external_id": "eng", "name": "Platform", "slug": "platform", "sort": 20}
		],
		"members": [
			{"provider_user_id": "hs-alice", "org_unit_external_id": "eng", "role": "owner"},
			{"username": "bob", "org_unit_external_id": "platform"}
		]
	}`

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/org-sync/preview", payload)
	PreviewEnterpriseOrgSync(ctx)
	preview := decodeEnterpriseOrgSyncResponse(t, recorder)
	require.True(t, preview.Success, preview.Message)
	assert.True(t, preview.Data.DryRun)
	assert.Equal(t, "hstation", preview.Data.Provider)
	assert.EqualValues(t, 2, preview.Data.Summary.CreateOrgUnits)
	assert.EqualValues(t, 2, preview.Data.Summary.AssignMembers)
	assert.Empty(t, preview.Data.Conflicts)

	var orgCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseOrgUnit{}).Where("enterprise_id = ?", enterprise.Id).Count(&orgCount).Error)
	assert.EqualValues(t, 0, orgCount)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/org-sync/apply", payload)
	ApplyEnterpriseOrgSync(ctx)
	applied := decodeEnterpriseOrgSyncResponse(t, recorder)
	require.True(t, applied.Success, applied.Message)
	assert.False(t, applied.Data.DryRun)
	assert.NotZero(t, applied.Data.AppliedAt)
	assert.EqualValues(t, 2, applied.Data.Summary.CreateOrgUnits)
	assert.EqualValues(t, 2, applied.Data.Summary.AssignMembers)

	var engineering model.EnterpriseOrgUnit
	require.NoError(t, model.DB.Where("enterprise_id = ? AND slug = ?", enterprise.Id, "engineering").First(&engineering).Error)
	var platform model.EnterpriseOrgUnit
	require.NoError(t, model.DB.Where("enterprise_id = ? AND slug = ?", enterprise.Id, "platform").First(&platform).Error)
	assert.Equal(t, engineering.Id, platform.ParentId)

	var aliceMembership model.EnterpriseOrgMembership
	require.NoError(t, model.DB.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, 1101).First(&aliceMembership).Error)
	assert.Equal(t, engineering.Id, aliceMembership.OrgUnitId)
	assert.Equal(t, "owner", aliceMembership.Role)
	var bobMembership model.EnterpriseOrgMembership
	require.NoError(t, model.DB.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, 1102).First(&bobMembership).Error)
	assert.Equal(t, platform.Id, bobMembership.OrgUnitId)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("enterprise_id = ? AND action = ?", enterprise.Id, "org_sync.apply").First(&audit).Error)
	assert.Contains(t, audit.AfterJson, `"provider":"hstation"`)
	assert.Contains(t, audit.AfterJson, `"create_org_units":2`)
}

func TestEnterpriseOrgSyncPreviewReportsConflicts(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	payload := `{
		"provider": "hstation",
		"org_units": [
			{"external_id": "platform", "parent_external_id": "missing", "name": "Platform", "slug": "platform"}
		],
		"members": [
			{"provider_user_id": "missing-user", "org_unit_external_id": "platform"}
		]
	}`

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/org-sync/preview", payload)
	PreviewEnterpriseOrgSync(ctx)
	preview := decodeEnterpriseOrgSyncResponse(t, recorder)
	require.True(t, preview.Success, preview.Message)
	assert.Len(t, preview.Data.Conflicts, 2)
	assert.EqualValues(t, 2, preview.Data.Summary.Conflicts)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/org-sync/apply", payload)
	ApplyEnterpriseOrgSync(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "冲突")
}

func TestEnterprisePolicyGroupDisableRejectsReferencedGroup(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	group := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		Name:         "高阶模型",
		Slug:         "advanced",
		Status:       model.PolicyGroupStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&group).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "高阶模型每日额度",
		TargetType:   model.PolicyTargetPolicyGroup,
		TargetId:     group.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodDelete, "/api/enterprise/policy-groups/"+itoaForEnterpriseTest(group.Id), "")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(group.Id)}}
	DeleteEnterprisePolicyGroup(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "引用")
}

func TestEnterpriseProjectCreateListAndAudit(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 1001, Username: "alice", DisplayName: "Alice", Status: common.UserStatusEnabled}).Error)
	engineeringId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")
	platformId := createEnterpriseOrgUnitForTest(t, engineeringId, "平台组", "platform")

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/projects", `{
		"name": "推理平台",
		"slug": "inference-platform",
		"description": "核心推理服务成本中心",
		"owner_user_id": 1001,
		"org_unit_ids": [`+itoaForEnterpriseTest(engineeringId)+`, `+itoaForEnterpriseTest(platformId)+`]
	}`)
	CreateEnterpriseProject(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	projectId := enterpriseResponseId(t, response)

	var project model.EnterpriseProject
	require.NoError(t, model.DB.First(&project, projectId).Error)
	assert.Equal(t, "推理平台", project.Name)
	assert.Equal(t, 1001, project.OwnerUserId)

	var bindings []model.EnterpriseProjectOrgUnit
	require.NoError(t, model.DB.Where("project_id = ?", projectId).Order("org_unit_id asc").Find(&bindings).Error)
	require.Len(t, bindings, 2)
	assert.Equal(t, engineeringId, bindings[0].OrgUnitId)
	assert.Equal(t, platformId, bindings[1].OrgUnitId)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/projects?keyword=inference&page_size=10", "")
	ListEnterpriseProjects(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page struct {
		Total int                     `json:"total"`
		Items []enterpriseProjectItem `json:"items"`
	}
	decodeEnterpriseResponseData(t, response, &page)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "推理平台", page.Items[0].Name)
	assert.Equal(t, "Alice", page.Items[0].OwnerName)
	assert.ElementsMatch(t, []int{engineeringId, platformId}, page.Items[0].OrgUnitIds)
	assert.ElementsMatch(t, []string{"研发部", "平台组"}, page.Items[0].OrgUnitNames)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("target_type = ? AND target_id = ? AND action = ?", "project", projectId, "project.create").
		Count(&auditCount).Error)
	assert.EqualValues(t, 1, auditCount)
}

func TestEnterpriseProjectDisableRejectsReferencedProject(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	project := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "成本中心 A",
		Slug:         "cost-center-a",
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&project).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "项目每日额度",
		TargetType:   model.PolicyTargetProject,
		TargetId:     project.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodDelete, "/api/enterprise/projects/"+itoaForEnterpriseTest(project.Id), "")
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(project.Id)}}
	DeleteEnterpriseProject(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "引用")
}

func TestEnterpriseQuotaPolicyCreateWritesAuditLog(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	orgUnitId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-policies", `{
		"name": "研发部每日额度",
		"target_type": "org_unit",
		"target_id": `+itoaForEnterpriseTest(orgUnitId)+`,
		"metric": "quota",
		"period": "day",
		"limit_value": 500000,
		"model_scope": "specific",
		"models": ["gpt-4o"],
		"action": "reject"
	}`)
	CreateEnterpriseQuotaPolicy(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	policyId := enterpriseResponseId(t, response)

	var policy model.EnterpriseQuotaPolicy
	require.NoError(t, model.DB.First(&policy, policyId).Error)
	assert.JSONEq(t, `["gpt-4o"]`, policy.ModelsJson)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("target_type = ? AND target_id = ? AND action = ?", "quota_policy", policyId, "quota_policy.create").
		Count(&auditCount).Error)
	assert.EqualValues(t, 1, auditCount)
}

func TestEnterpriseQuotaPolicyAdvancedActionsValidation(t *testing.T) {
	setupEnterpriseControllerTestDB(t)

	actions := []string{
		model.PolicyActionAlert,
		model.PolicyActionFallbackModel,
		model.PolicyActionQueue,
		model.PolicyActionSharedPool,
	}
	for index, action := range actions {
		ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-policies", `{
			"name": "enterprise action `+strconv.Itoa(index)+`",
			"target_type": "enterprise",
			"target_id": 0,
			"metric": "request_count",
			"period": "day",
			"limit_value": 10,
			"model_scope": "all",
			"action": "`+action+`"
		}`)
		CreateEnterpriseQuotaPolicy(ctx)
		response := decodeEnterpriseControllerResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		policyId := enterpriseResponseId(t, response)

		var policy model.EnterpriseQuotaPolicy
		require.NoError(t, model.DB.First(&policy, policyId).Error)
		assert.Equal(t, action, policy.Action)
		assert.Equal(t, model.PolicyTargetEnterprise, policy.TargetType)
		assert.Equal(t, policy.EnterpriseId, policy.TargetId)
	}

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-policies", `{
		"name": "enterprise invalid action",
		"target_type": "enterprise",
		"target_id": 0,
		"metric": "request_count",
		"period": "day",
		"limit_value": 10,
		"model_scope": "all",
		"action": "silently_do_magic"
	}`)
	CreateEnterpriseQuotaPolicy(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "不支持的策略动作")
}

func TestEnterpriseAuditLogFilters(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseAuditLog{
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  9001,
			Action:       "quota_policy.create",
			TargetType:   "quota_policy",
			TargetId:     11,
			RequestId:    "req-audit-1",
			CreatedAt:    1000,
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  9002,
			Action:       "org_unit.create",
			TargetType:   "org_unit",
			TargetId:     12,
			RequestId:    "req-audit-2",
			CreatedAt:    2000,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/audit-logs?action=quota_policy.create&request_id=req-audit-1&start_time=900&end_time=1100",
		"",
	)
	ListEnterpriseAuditLogs(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "quota_policy.create", item["action"])
	assert.Equal(t, "req-audit-1", item["request_id"])
}

func TestEnterpriseQueueAdmissionFilters(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceQueueAdmission{
		{
			EnterpriseId:   enterprise.Id,
			RequestId:      "req-queue-admitted",
			UserId:         9101,
			TokenId:        101,
			PolicyId:       11,
			ModelName:      "gpt-4o",
			ChannelId:      701,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusAdmitted,
			WaitMs:         5,
			TimeoutMs:      30000,
			UserMessageKey: "enterprise_governance.policy_action_observed",
			CreatedAt:      1000,
		},
		{
			EnterpriseId:   enterprise.Id,
			RequestId:      "req-queue-timeout",
			UserId:         9102,
			TokenId:        102,
			PolicyId:       12,
			ModelName:      "gpt-4o-mini",
			ChannelId:      702,
			QueueKey:       "enterprise:1",
			Status:         model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
			WaitMs:         30000,
			TimeoutMs:      30000,
			UserMessageKey: "enterprise_governance.queue_timeout",
			CreatedAt:      2000,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/queue-admissions?status=timeout&request_id=req-queue-timeout&start_time=1500&end_time=2500",
		"",
	)
	ListEnterpriseGovernanceQueueAdmissions(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "timeout", item["status"])
	assert.Equal(t, "req-queue-timeout", item["request_id"])
	assert.Equal(t, "gpt-4o-mini", item["model_name"])
	assert.EqualValues(t, 12, item["policy_id"])
}

func TestEnterpriseQueueAdmissionListRedactsRequestPayload(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	payload := service.EnterpriseGovernanceQueueRequestPayload{
		Method:      http.MethodPost,
		Path:        "/v1/chat/completions",
		RawQuery:    "stream=false&api_key=sk-controller-query-secret",
		ContentType: "application/json",
		Body:        `{"model":"gpt-4o","api_key":"sk-controller-body-secret","messages":[{"role":"user","content":"visible"}]}`,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	row := model.EnterpriseGovernanceQueueAdmission{
		EnterpriseId:       enterprise.Id,
		RequestId:          "req-queue-redact",
		UserId:             9104,
		TokenId:            104,
		PolicyId:           14,
		ModelName:          "gpt-4o",
		QueueKey:           "enterprise:1",
		Status:             model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
		RequestPayloadJson: string(payloadJson),
		UserMessageKey:     "enterprise_governance.queue_timeout",
		CreatedAt:          4000,
	}
	require.NoError(t, model.DB.Create(&row).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/queue-admissions?request_id=req-queue-redact", "")
	ListEnterpriseGovernanceQueueAdmissions(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	visibleJson, ok := item["request_payload_json"].(string)
	require.True(t, ok)
	assert.NotContains(t, visibleJson, "sk-controller-query-secret")
	assert.NotContains(t, visibleJson, "sk-controller-body-secret")
	var visiblePayload service.EnterpriseGovernanceQueueRequestPayload
	require.NoError(t, common.UnmarshalJsonStr(visibleJson, &visiblePayload))
	assert.Equal(t, "stream=false&api_key=[REDACTED]", visiblePayload.RawQuery)
	assert.Contains(t, visiblePayload.Body, `"api_key":"[REDACTED]"`)
	assert.Contains(t, visiblePayload.Body, "visible")

	var stored model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("id = ?", row.Id).First(&stored).Error)
	assert.Contains(t, stored.RequestPayloadJson, "sk-controller-query-secret")
	assert.Contains(t, stored.RequestPayloadJson, "sk-controller-body-secret")
}

func TestEnterpriseAuditLogsRedactHistoricalQueuePayload(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	payload := service.EnterpriseGovernanceQueueRequestPayload{
		Method:      http.MethodPost,
		Path:        "/v1/chat/completions",
		RawQuery:    "api_key=sk-history-query-secret&stream=false",
		ContentType: "application/json",
		Body:        `{"model":"gpt-4o","client_secret":"history-body-secret"}`,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	admission := model.EnterpriseGovernanceQueueAdmission{
		EnterpriseId:       enterprise.Id,
		RequestId:          "req-history-audit",
		RequestPayloadJson: string(payloadJson),
		Status:             model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
	}
	beforeJson, err := common.Marshal(admission)
	require.NoError(t, err)
	afterJson, err := common.Marshal(gin.H{
		"admission": admission,
		"replay": gin.H{
			"raw_query": "api_key=sk-history-replay-query",
		},
	})
	require.NoError(t, err)
	rawLog := model.EnterpriseAuditLog{
		EnterpriseId: enterprise.Id,
		Action:       "enterprise_governance.queue_admission.replay",
		TargetType:   "enterprise_governance_queue_admission",
		TargetId:     8801,
		BeforeJson:   string(beforeJson),
		AfterJson:    string(afterJson),
		RequestId:    "req-history-audit",
		CreatedAt:    4100,
	}
	require.NoError(t, model.DB.Create(&rawLog).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/audit-logs?request_id=req-history-audit", "")
	ListEnterpriseAuditLogs(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	beforeVisible, ok := item["before_json"].(string)
	require.True(t, ok)
	afterVisible, ok := item["after_json"].(string)
	require.True(t, ok)
	assert.NotContains(t, beforeVisible, "sk-history-query-secret")
	assert.NotContains(t, beforeVisible, "history-body-secret")
	assert.NotContains(t, afterVisible, "sk-history-replay-query")
	assert.Contains(t, beforeVisible, "api_key=[REDACTED]")
	assert.Contains(t, beforeVisible, "client_secret")
	assert.Contains(t, beforeVisible, "[REDACTED]")
	assert.Contains(t, afterVisible, "api_key=[REDACTED]")

	var stored model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("id = ?", rawLog.Id).First(&stored).Error)
	assert.Contains(t, stored.BeforeJson, "sk-history-query-secret")
	assert.Contains(t, stored.AfterJson, "sk-history-replay-query")
}

func TestRetryEnterpriseGovernanceQueueAdmission(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	payload := service.EnterpriseGovernanceQueueRequestPayload{
		Method:      http.MethodPost,
		Path:        "/v1/chat/completions",
		RawQuery:    "api_key=sk-retry-query-secret",
		ContentType: "application/json",
		Body:        `{"model":"gpt-4o","api_key":"sk-retry-body-secret"}`,
	}
	payloadJson, err := common.Marshal(payload)
	require.NoError(t, err)
	row := model.EnterpriseGovernanceQueueAdmission{
		EnterpriseId:       enterprise.Id,
		RequestId:          "req-queue-retry",
		UserId:             9103,
		TokenId:            103,
		PolicyId:           13,
		ModelName:          "gpt-4o",
		QueueKey:           "enterprise:1",
		Status:             model.EnterpriseGovernanceQueueAdmissionStatusTimeout,
		TimeoutMs:          30000,
		RequestPayloadJson: string(payloadJson),
		LastError:          service.ErrEnterpriseGovernanceQueueTimeout.Error(),
		UserMessageKey:     "enterprise_governance.queue_timeout",
		CreatedAt:          3000,
	}
	require.NoError(t, model.DB.Create(&row).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/queue-admissions/"+strconv.FormatInt(row.Id, 10)+"/retry", "{}")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(row.Id, 10)}}
	RetryEnterpriseGovernanceQueueAdmission(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var after model.EnterpriseGovernanceQueueAdmission
	decodeEnterpriseResponseData(t, response, &after)
	assert.Equal(t, model.EnterpriseGovernanceQueueAdmissionStatusRetryPending, after.Status)
	assert.EqualValues(t, 1, after.RetryCount)
	assert.Greater(t, after.NextRetryAt, int64(0))
	assert.Empty(t, after.LastError)
	assert.Equal(t, "enterprise_governance.queue_retry_pending", after.UserMessageKey)
	assert.NotContains(t, after.RequestPayloadJson, "sk-retry-query-secret")
	assert.NotContains(t, after.RequestPayloadJson, "sk-retry-body-secret")

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ? AND target_type = ? AND target_id = ?", "queue_admission.retry", "enterprise_governance_queue_admission", row.Id).First(&audit).Error)
	assert.Equal(t, "req-enterprise-controller-test", audit.RequestId)
	assert.NotContains(t, audit.BeforeJson, "sk-retry-query-secret")
	assert.NotContains(t, audit.BeforeJson, "sk-retry-body-secret")
	assert.NotContains(t, audit.AfterJson, "sk-retry-query-secret")
	assert.NotContains(t, audit.AfterJson, "sk-retry-body-secret")

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/audit-logs?action=queue_admission.retry", "")
	ListEnterpriseAuditLogs(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	beforeVisible, ok := item["before_json"].(string)
	require.True(t, ok)
	afterVisible, ok := item["after_json"].(string)
	require.True(t, ok)
	assert.NotContains(t, beforeVisible, "sk-retry-query-secret")
	assert.NotContains(t, afterVisible, "sk-retry-body-secret")

	var stored model.EnterpriseGovernanceQueueAdmission
	require.NoError(t, model.DB.Where("id = ?", row.Id).First(&stored).Error)
	assert.Contains(t, stored.RequestPayloadJson, "sk-retry-query-secret")
	assert.Contains(t, stored.RequestPayloadJson, "sk-retry-body-secret")
}

func TestEnterpriseSharedPoolFilters(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceSharedPool{
		{
			EnterpriseId:  enterprise.Id,
			PolicyId:      11,
			Metric:        model.PolicyMetricRequestCount,
			PeriodStart:   1000,
			PeriodEnd:     1999,
			CapacityValue: 100,
			UsedValue:     10,
			ReservedValue: 20,
		},
		{
			EnterpriseId:  enterprise.Id,
			PolicyId:      12,
			Metric:        model.PolicyMetricQuota,
			PeriodStart:   2000,
			PeriodEnd:     2999,
			CapacityValue: 1000,
			UsedValue:     100,
			ReservedValue: 200,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/shared-pools?metric=quota&policy_id=12&start_time=1500&end_time=2500",
		"",
	)
	ListEnterpriseGovernanceSharedPools(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.PolicyMetricQuota, item["metric"])
	assert.EqualValues(t, 12, item["policy_id"])
	assert.EqualValues(t, 1000, item["capacity_value"])
	assert.EqualValues(t, 200, item["reserved_value"])
}

func TestEnterpriseSharedPoolConfigAndTrendEndpoints(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "shared pool config policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		Timezone:     "Asia/Shanghai",
		ModelScope:   model.PolicyModelScopeAll,
		Action:       model.PolicyActionSharedPool,
		Status:       model.QuotaPolicyStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&policy).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodPut,
		"/api/enterprise/shared-pool-configs",
		`{"policy_id":`+strconv.Itoa(policy.Id)+`,"metric":"quota","capacity_value":500,"status":1}`,
	)
	UpsertEnterpriseGovernanceSharedPoolConfig(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, policy.Id, response.Data["policy_id"])
	assert.EqualValues(t, 500, response.Data["capacity_value"])

	ctx, recorder = newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/shared-pool-configs?metric=quota&policy_id="+strconv.Itoa(policy.Id)+"&status=1",
		"",
	)
	ListEnterpriseGovernanceSharedPoolConfigs(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ? AND target_type = ?", "shared_pool_config.upsert", "enterprise_governance_shared_pool_config").First(&audit).Error)
	assert.Equal(t, "req-enterprise-controller-test", audit.RequestId)

	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceSharedPoolBorrow{
		{
			EnterpriseId:          enterprise.Id,
			PoolId:                201,
			RequestId:             "req-shared-trend-1",
			PolicyId:              policy.Id,
			Metric:                model.PolicyMetricQuota,
			Status:                model.EnterpriseGovernanceSharedPoolBorrowStatusSettled,
			ReservedBorrowedValue: 10,
			SettledBorrowedValue:  7,
			ReturnedValue:         3,
			CapacityValue:         500,
			UserMessageKey:        "enterprise_governance.shared_pool_settled",
			CreatedAt:             1700000000,
		},
		{
			EnterpriseId:          enterprise.Id,
			PoolId:                201,
			RequestId:             "req-shared-trend-2",
			PolicyId:              policy.Id,
			Metric:                model.PolicyMetricQuota,
			Status:                model.EnterpriseGovernanceSharedPoolBorrowStatusSettled,
			ReservedBorrowedValue: 20,
			SettledBorrowedValue:  18,
			ReturnedValue:         2,
			CapacityValue:         500,
			UserMessageKey:        "enterprise_governance.shared_pool_settled",
			CreatedAt:             1700003600,
		},
	}).Error)

	ctx, recorder = newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/shared-pool-trends?metric=quota&policy_id="+strconv.Itoa(policy.Id)+"&start_time=1699990000&end_time=1700100000&bucket_seconds=86400",
		"",
	)
	ListEnterpriseGovernanceSharedPoolTrends(ctx)
	listResponse := decodeEnterpriseControllerListResponse(t, recorder)
	require.True(t, listResponse.Success, listResponse.Message)
	require.Len(t, listResponse.Data, 1)
	trend, ok := listResponse.Data[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.PolicyMetricQuota, trend["metric"])
	assert.EqualValues(t, 2, trend["borrow_count"])
	assert.EqualValues(t, 30, trend["reserved_borrowed_value"])
	assert.EqualValues(t, 25, trend["settled_borrowed_value"])
	assert.EqualValues(t, 5, trend["returned_value"])
}

func TestEnterpriseAnomalyProtectionListAndTrendEndpoints(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceAnomalyProtection{
		{
			EnterpriseId:   enterprise.Id,
			ProtectionKey:  "project:1:20",
			ScopeType:      model.EnterpriseGovernanceAnomalyProtectionScopeProject,
			ScopeId:        20,
			Reason:         "request_spike",
			Status:         model.EnterpriseGovernanceAnomalyProtectionStatusActive,
			DetectedAt:     1700000000,
			ProtectedUntil: 1700007200,
			PayloadJson:    `{"anomaly_reason":"request_spike","dry_run":false}`,
		},
		{
			EnterpriseId:   enterprise.Id,
			ProtectionKey:  "project:1:20",
			ScopeType:      model.EnterpriseGovernanceAnomalyProtectionScopeProject,
			ScopeId:        20,
			Reason:         "request_spike",
			Status:         model.EnterpriseGovernanceAnomalyProtectionStatusActive,
			DetectedAt:     1700003600,
			ProtectedUntil: 1700010800,
			PayloadJson:    `{"anomaly_reason":"request_spike","dry_run":false}`,
		},
		{
			EnterpriseId:   enterprise.Id,
			ProtectionKey:  "org_unit:1:30",
			ScopeType:      model.EnterpriseGovernanceAnomalyProtectionScopeOrgUnit,
			ScopeId:        30,
			Reason:         "cost_spike",
			Status:         model.EnterpriseGovernanceAnomalyProtectionStatusExpired,
			DetectedAt:     1700080000,
			ProtectedUntil: 1700083600,
			PayloadJson:    `{"anomaly_reason":"cost_spike","dry_run":false}`,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/anomaly-protections?status=active&reason=request_spike&protection_key=project:1:20&scope_type=project&scope_id=20&start_time=1699999000&end_time=1700001000",
		"",
	)
	ListEnterpriseGovernanceAnomalyProtections(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.EnterpriseGovernanceAnomalyProtectionStatusActive, item["status"])
	assert.Equal(t, "request_spike", item["reason"])
	assert.Equal(t, model.EnterpriseGovernanceAnomalyProtectionScopeProject, item["scope_type"])
	assert.EqualValues(t, 20, item["scope_id"])
	assert.EqualValues(t, 1700007200, item["protected_until"])

	ctx, recorder = newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/anomaly-protection-trends?status=active&reason=request_spike&scope_type=project&scope_id=20&start_time=1699990000&end_time=1700100000&bucket_seconds=86400",
		"",
	)
	ListEnterpriseGovernanceAnomalyProtectionTrends(ctx)
	listResponse := decodeEnterpriseControllerListResponse(t, recorder)
	require.True(t, listResponse.Success, listResponse.Message)
	require.Len(t, listResponse.Data, 1)
	trend, ok := listResponse.Data[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "request_spike", trend["reason"])
	assert.EqualValues(t, 2, trend["protection_count"])
	assert.EqualValues(t, 2, trend["active_count"])
	assert.EqualValues(t, 0, trend["expired_count"])
	assert.EqualValues(t, 1700010800, trend["max_protected_until"])
}

func TestEnterpriseSharedPoolBorrowFilters(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseGovernanceSharedPoolBorrow{
		{
			EnterpriseId:          enterprise.Id,
			PoolId:                101,
			RequestId:             "req-shared-pool-reserved",
			UserId:                9101,
			TokenId:               101,
			PolicyId:              11,
			Metric:                model.PolicyMetricRequestCount,
			ModelName:             "gpt-4o",
			ChannelId:             701,
			Status:                model.EnterpriseGovernanceSharedPoolBorrowStatusReserved,
			ReservedBorrowedValue: 5,
			CapacityValue:         100,
			UserMessageKey:        "enterprise_governance.shared_pool_reserved",
			CreatedAt:             1000,
		},
		{
			EnterpriseId:          enterprise.Id,
			PoolId:                102,
			RequestId:             "req-shared-pool-settled",
			UserId:                9102,
			TokenId:               102,
			OrgUnitId:             22,
			ProjectId:             32,
			PolicyId:              12,
			Metric:                model.PolicyMetricQuota,
			ModelName:             "gpt-4o-mini",
			ChannelId:             702,
			Status:                model.EnterpriseGovernanceSharedPoolBorrowStatusSettled,
			ReservedBorrowedValue: 50,
			SettledBorrowedValue:  30,
			ReturnedValue:         20,
			CapacityValue:         1000,
			UserMessageKey:        "enterprise_governance.shared_pool_settled",
			CreatedAt:             2000,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(
		t,
		http.MethodGet,
		"/api/enterprise/shared-pool-borrows?status=settled&metric=quota&request_id=req-shared-pool-settled&model_name=gpt-4o-mini&pool_id=102&policy_id=12&project_id=32&start_time=1500&end_time=2500",
		"",
	)
	ListEnterpriseGovernanceSharedPoolBorrows(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, model.EnterpriseGovernanceSharedPoolBorrowStatusSettled, item["status"])
	assert.Equal(t, "req-shared-pool-settled", item["request_id"])
	assert.Equal(t, "gpt-4o-mini", item["model_name"])
	assert.EqualValues(t, 12, item["policy_id"])
	assert.EqualValues(t, 30, item["settled_borrowed_value"])
	assert.EqualValues(t, 20, item["returned_value"])
}

func TestEnterpriseQuotaRequestSubmitApproveListAndAudit(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1001, Username: "alice", DisplayName: "Alice", Email: "alice@example.com", Status: common.UserStatusEnabled, AffCode: "aff-alice-quota-request"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 9001, Username: "admin", DisplayName: "Admin", Status: common.UserStatusEnabled, AffCode: "aff-admin-quota-request"}).Error)
	orgUnitId := createEnterpriseOrgUnitForTest(t, 0, "Engineering", "engineering-quota-request")
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{EnterpriseId: enterprise.Id, UserId: 1001, OrgUnitId: orgUnitId, IsPrimary: true}).Error)
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "daily launch quota",
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
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaCounter{
		EnterpriseId: enterprise.Id,
		PolicyId:     policy.Id,
		TargetType:   policy.TargetType,
		TargetId:     policy.TargetId,
		Metric:       policy.Metric,
		PeriodStart:  1000,
		PeriodEnd:    2000,
		UsedValue:    7,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaCounter{
		EnterpriseId: enterprise.Id,
		PolicyId:     policy.Id,
		TargetType:   policy.TargetType,
		TargetId:     policy.TargetId + 100,
		Metric:       policy.Metric,
		PeriodStart:  1000,
		PeriodEnd:    2000,
		UsedValue:    99,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaCounter{
		EnterpriseId: enterprise.Id + 100,
		PolicyId:     policy.Id,
		TargetType:   policy.TargetType,
		TargetId:     policy.TargetId,
		Metric:       policy.Metric,
		PeriodStart:  3000,
		PeriodEnd:    4000,
		UsedValue:    41,
	}).Error)
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  1001,
		Action:       "enterprise_governance.dry_run_reject",
		TargetType:   "quota_policy",
		TargetId:     policy.Id,
		After: gin.H{
			"policy_id":          policy.Id,
			"matched_policy_ids": []int{policy.Id},
			"dry_run":            true,
		},
		RequestId: "quota-risk-dry-run",
	}))
	require.NoError(t, model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  1001,
		Action:       "enterprise_governance.hard_limit_reject",
		TargetType:   "quota_policy",
		TargetId:     policy.Id,
		After: gin.H{
			"matched_policy_ids": []int{policy.Id},
			"dry_run":            false,
		},
		RequestId: "quota-risk-hard-limit",
	}))
	webhook := model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "approval webhook",
		Url:            "https://example.com/enterprise-webhook",
		Secret:         "secret",
		EventTypesJson: `["quota_request.approve"]`,
		Status:         model.EnterpriseWebhookStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&webhook).Error)
	applicantScope, err := common.Marshal(service.EnterpriseNotificationRecipientScope{Applicant: true})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelEmail,
		EventType:          "quota_request.approve",
		Enabled:            true,
		RecipientScopeJson: string(applicantScope),
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelWebhook,
		EventType:          "quota_request.approve",
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	inaccessiblePolicy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "bob private quota",
		TargetType:   model.PolicyTargetUser,
		TargetId:     2002,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		ModelsJson:   "[]",
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&inaccessiblePolicy).Error)
	project := model.EnterpriseProject{EnterpriseId: enterprise.Id, Name: "Inference Project", Slug: "inference-project", Status: model.EnterpriseProjectStatusEnabled}
	require.NoError(t, model.DB.Create(&project).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseProjectOrgUnit{EnterpriseId: enterprise.Id, ProjectId: project.Id, OrgUnitId: orgUnitId}).Error)
	projectPolicy := model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "project quota",
		TargetType:   model.PolicyTargetProject,
		TargetId:     project.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   20,
		Timezone:     model.DefaultEnterpriseTimezone,
		ModelScope:   model.PolicyModelScopeAll,
		ModelsJson:   "[]",
		Action:       model.PolicyActionReject,
		Status:       model.QuotaPolicyStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&projectPolicy).Error)
	expiresAt := common.GetTimestamp() + 3600

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/quota-requests/policies", "")
	ctx.Set("id", 1001)
	ListEnterpriseQuotaRequestPolicies(ctx)
	policyResponse := decodeEnterpriseControllerListResponse(t, recorder)
	require.True(t, policyResponse.Success, policyResponse.Message)
	require.Len(t, policyResponse.Data, 1)
	policyItem, ok := policyResponse.Data[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, policy.Id, policyItem["id"])

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/quota-requests/policies?project_id="+itoaForEnterpriseTest(project.Id), "")
	ctx.Set("id", 1001)
	ListEnterpriseQuotaRequestPolicies(ctx)
	projectPolicyResponse := decodeEnterpriseControllerListResponse(t, recorder)
	require.True(t, projectPolicyResponse.Success, projectPolicyResponse.Message)
	require.Len(t, projectPolicyResponse.Data, 2)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+itoaForEnterpriseTest(inaccessiblePolicy.Id)+`,
    "limit_delta": 5,
    "reason": "wrong scope",
    "expires_at": `+itoaForEnterpriseTest(int(expiresAt))+`
  }`)
	ctx.Set("id", 1001)
	SubmitEnterpriseQuotaRequest(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.False(t, response.Success)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+itoaForEnterpriseTest(projectPolicy.Id)+`,
    "limit_delta": 5,
    "reason": "missing project context",
    "expires_at": `+itoaForEnterpriseTest(int(expiresAt))+`
  }`)
	ctx.Set("id", 1001)
	SubmitEnterpriseQuotaRequest(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.False(t, response.Success)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+itoaForEnterpriseTest(projectPolicy.Id)+`,
    "project_id": `+itoaForEnterpriseTest(project.Id)+`,
    "limit_delta": 5,
    "reason": "project launch",
    "expires_at": `+itoaForEnterpriseTest(int(expiresAt))+`
  }`)
	ctx.Set("id", 1001)
	SubmitEnterpriseQuotaRequest(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	projectRequestId := int(response.Data["id"].(float64))

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests", `{
    "policy_id": `+itoaForEnterpriseTest(policy.Id)+`,
    "limit_delta": 5,
    "reason": "release day",
    "expires_at": `+itoaForEnterpriseTest(int(expiresAt))+`
  }`)
	ctx.Set("id", 1001)
	SubmitEnterpriseQuotaRequest(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	requestId := int(response.Data["id"].(float64))

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests/"+itoaForEnterpriseTest(requestId)+"/approve", `{
    "decision_reason": "approved for launch"
  }`)
	ctx.Set("id", 9001)
	ctx.Params = gin.Params{{Key: "id", Value: itoaForEnterpriseTest(requestId)}}
	ApproveEnterpriseQuotaRequest(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var quotaRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.First(&quotaRequest, requestId).Error)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusApproved, quotaRequest.Status)
	assert.Equal(t, 1001, quotaRequest.ApplicantUserId)
	assert.Equal(t, 9001, quotaRequest.ApproverUserId)
	assert.EqualValues(t, 5, quotaRequest.LimitDelta)
	assert.Equal(t, policy.TargetType, quotaRequest.TargetType)
	assert.Equal(t, policy.Metric, quotaRequest.Metric)

	var projectQuotaRequest model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.First(&projectQuotaRequest, projectRequestId).Error)
	assert.Equal(t, project.Id, projectQuotaRequest.ProjectId)
	assert.Equal(t, projectPolicy.TargetType, projectQuotaRequest.TargetType)
	assert.Equal(t, projectPolicy.TargetId, projectQuotaRequest.TargetId)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/quota-requests?status=approved&page_size=10", "")
	ctx.Set("id", 9001)
	ctx.Set("role", common.RoleAdminUser)
	ListEnterpriseQuotaRequests(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "daily launch quota", item["policy_name"])
	assert.Equal(t, "Alice", item["applicant_name"])
	assert.Equal(t, "Admin", item["approver_name"])
	assert.EqualValues(t, 10, item["policy_limit_value"])
	assert.EqualValues(t, 7, item["policy_used_value"])
	assert.EqualValues(t, 15, item["stacked_limit_value"])
	assert.EqualValues(t, 2, item["recent_policy_hits"])
	assert.EqualValues(t, 1, item["recent_dry_run_hits"])

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/quota-requests?project_id="+itoaForEnterpriseTest(project.Id)+"&target_type=project&target_id="+itoaForEnterpriseTest(project.Id)+"&applicant_user_id=1001&page_size=10", "")
	ctx.Set("id", 9001)
	ctx.Set("role", common.RoleAdminUser)
	ListEnterpriseQuotaRequests(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["total"])
	items, ok = response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok = items[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, projectRequestId, item["id"])
	assert.EqualValues(t, project.Id, item["project_id"])
	assert.Equal(t, "project quota", item["policy_name"])

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/quota-requests?project_id=999999&page_size=10", "")
	ctx.Set("id", 9001)
	ctx.Set("role", common.RoleAdminUser)
	ListEnterpriseQuotaRequests(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 0, response.Data["total"])

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/quota-requests/batch/reject", `{
    "ids": [`+itoaForEnterpriseTest(projectRequestId)+`, `+itoaForEnterpriseTest(requestId)+`],
    "decision_reason": "batch clean up"
  }`)
	ctx.Set("id", 9001)
	ctx.Set("role", common.RoleAdminUser)
	BatchRejectEnterpriseQuotaRequests(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["success_count"])
	assert.EqualValues(t, 1, response.Data["failure_count"])
	batchItems, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, batchItems, 2)
	firstBatchItem, ok := batchItems[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, projectRequestId, firstBatchItem["id"])
	assert.Equal(t, true, firstBatchItem["success"])
	secondBatchItem, ok := batchItems[1].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, requestId, secondBatchItem["id"])
	assert.Equal(t, false, secondBatchItem["success"])

	require.NoError(t, model.DB.First(&projectQuotaRequest, projectRequestId).Error)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusRejected, projectQuotaRequest.Status)
	assert.Equal(t, "batch clean up", projectQuotaRequest.DecisionReason)
	require.NoError(t, model.DB.Where("target_type = ? AND target_id = ? AND action = ?", "quota_request", projectRequestId, "quota_request.reject").First(&model.EnterpriseAuditLog{}).Error)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("target_type = ? AND target_id = ? AND action IN ?", "quota_request", requestId, []string{"quota_request.submit", "quota_request.approve"}).
		Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount)

	var outboxRows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("target_type = ? AND target_id = ?", "quota_request", requestId).Order("id asc").Find(&outboxRows).Error)
	require.Len(t, outboxRows, 4)
	assert.Equal(t, "quota_request.submit", outboxRows[0].EventType)
	assert.Equal(t, model.EnterpriseNotificationOutboxChannelInApp, outboxRows[0].Channel)
	assert.Equal(t, 0, outboxRows[0].RecipientUserId)
	assert.Equal(t, "quota_request.approve", outboxRows[1].EventType)
	assert.Equal(t, 1001, outboxRows[1].RecipientUserId)
	assert.Contains(t, outboxRows[1].PayloadJson, model.EnterpriseQuotaRequestStatusApproved)
	assert.Equal(t, "quota_request.approve", outboxRows[2].EventType)
	assert.Equal(t, model.EnterpriseNotificationOutboxChannelEmail, outboxRows[2].Channel)
	assert.Equal(t, "alice@example.com", outboxRows[2].RecipientEmail)
	assert.Equal(t, "quota_request.approve", outboxRows[3].EventType)
	assert.Equal(t, model.EnterpriseNotificationOutboxChannelWebhook, outboxRows[3].Channel)
	assert.Equal(t, "webhook:"+itoaForEnterpriseTest(webhook.Id), outboxRows[3].RecipientEmail)
}

func TestEnterpriseNotificationOutboxListFiltersWebhookDeliveries(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&[]model.EnterpriseNotificationOutbox{
		{
			EventKey:       "controller-webhook-failed",
			EventType:      "quota_request.approve",
			EnterpriseId:   enterprise.Id,
			RecipientEmail: "webhook:77",
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       7001,
			PayloadJson:    `{}`,
			Status:         model.EnterpriseNotificationOutboxStatusFailed,
			RetryCount:     1,
			NextRetryAt:    now + 60,
			LastError:      "failed url=https://example.com/hook?secret=abc",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			EventKey:       "controller-webhook-other",
			EventType:      "quota_request.reject",
			EnterpriseId:   enterprise.Id,
			RecipientEmail: "webhook:88",
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       7002,
			PayloadJson:    `{}`,
			Status:         model.EnterpriseNotificationOutboxStatusSent,
			CreatedAt:      now - 10,
			UpdatedAt:      now - 10,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/notification-outbox?webhook_id=77&status=failed", "")
	ListEnterpriseNotificationOutbox(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.EqualValues(t, 1, response.Data["total"])
	items, ok := response.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "controller-webhook-failed", item["event_key"])
	assert.Equal(t, "webhook:77", item["recipient_email"])
	assert.Contains(t, item["last_error"], "secret=redacted")
	assert.NotContains(t, item["last_error"], "abc")
}

func TestEnterpriseNotificationOutboxRetryResetsFailedRowAndAudits(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := common.GetTimestamp()
	row := model.EnterpriseNotificationOutbox{
		EventKey:       "controller-retry-row",
		EventType:      "quota_request.reject",
		EnterpriseId:   enterprise.Id,
		RecipientEmail: "retry-person@example.com",
		Channel:        model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:     "quota_request",
		TargetId:       7101,
		PayloadJson:    `{}`,
		Status:         model.EnterpriseNotificationOutboxStatusFailed,
		RetryCount:     2,
		NextRetryAt:    now + 3600,
		LastError:      "SMTP server is not configured",
		CreatedAt:      now - 10,
		UpdatedAt:      now - 5,
	}
	require.NoError(t, model.DB.Create(&row).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodPost, "/api/enterprise/notification-outbox/"+strconv.FormatInt(row.Id, 10)+"/retry", "{}")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(row.Id, 10)}}
	RetryEnterpriseNotificationOutbox(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPending, response.Data["status"])
	assert.EqualValues(t, 0, response.Data["retry_count"])
	assert.Equal(t, "r***@example.com", response.Data["recipient_email"])

	var reloaded model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.First(&reloaded, row.Id).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPending, reloaded.Status)
	assert.Equal(t, 0, reloaded.RetryCount)
	assert.Empty(t, reloaded.LastError)

	var audit model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ? AND target_type = ? AND target_id = ?", "notification_outbox.retry", "notification_outbox", row.Id).First(&audit).Error)
	assert.NotContains(t, audit.BeforeJson, "retry-person@example.com")
	assert.Contains(t, audit.BeforeJson, "r***@example.com")
	assert.Contains(t, audit.BeforeJson, model.EnterpriseNotificationOutboxStatusFailed)
	assert.Contains(t, audit.AfterJson, model.EnterpriseNotificationOutboxStatusPending)
}

func TestEnterpriseUsageSummaryAndBreakdown(t *testing.T) {
	setupEnterpriseControllerTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1001, Username: "alice", DisplayName: "Alice", Status: common.UserStatusEnabled, AffCode: "aff-alice"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 1002, Username: "bob", DisplayName: "Bob", Status: common.UserStatusEnabled, AffCode: "aff-bob"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 10, UserId: 1001, Key: "alice-key", Name: "Alice Key", Status: common.TokenStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 20, UserId: 1002, Key: "bob-key", Name: "Bob Key", Status: common.TokenStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 101, Name: "OpenAI Primary", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 202, Name: "Claude Backup", Status: common.ChannelStatusEnabled}).Error)
	engineeringId := createEnterpriseOrgUnitForTest(t, 0, "研发部", "engineering")
	salesId := createEnterpriseOrgUnitForTest(t, 0, "销售部", "sales")
	groupA := model.EnterprisePolicyGroup{EnterpriseId: enterprise.Id, Name: "高阶模型", Slug: "advanced", Status: model.PolicyGroupStatusEnabled}
	groupB := model.EnterprisePolicyGroup{EnterpriseId: enterprise.Id, Name: "试点用户", Slug: "pilot", Status: model.PolicyGroupStatusEnabled}
	require.NoError(t, model.DB.Create(&groupA).Error)
	require.NoError(t, model.DB.Create(&groupB).Error)
	groupAJson, err := common.Marshal([]int{groupA.Id})
	require.NoError(t, err)
	groupBJson, err := common.Marshal([]int{groupB.Id})
	require.NoError(t, err)
	bothGroupsJson, err := common.Marshal([]int{groupA.Id, groupB.Id})
	require.NoError(t, err)
	projectA := model.EnterpriseProject{EnterpriseId: enterprise.Id, Name: "研发成本中心", Slug: "engineering-cost", Status: model.EnterpriseProjectStatusEnabled}
	projectB := model.EnterpriseProject{EnterpriseId: enterprise.Id, Name: "销售成本中心", Slug: "sales-cost", Status: model.EnterpriseProjectStatusEnabled}
	require.NoError(t, model.DB.Create(&projectA).Error)
	require.NoError(t, model.DB.Create(&projectB).Error)

	require.NoError(t, model.DB.Create(&[]model.EnterpriseUsageAttribution{
		{
			RequestId:          "req-usage-1",
			UserId:             1001,
			TokenId:            10,
			EnterpriseId:       enterprise.Id,
			OrgUnitId:          engineeringId,
			ProjectId:          projectA.Id,
			PolicyGroupIdsJson: string(groupAJson),
			PolicyIdsJson:      "[]",
			ModelName:          "gpt-4o",
			ChannelId:          101,
			PromptTokens:       3,
			CompletionTokens:   2,
			TotalTokens:        5,
			Quota:              10,
			Status:             "success",
			CreatedAt:          1000,
		},
		{
			RequestId:          "req-usage-2",
			UserId:             1002,
			TokenId:            20,
			EnterpriseId:       enterprise.Id,
			OrgUnitId:          salesId,
			ProjectId:          projectB.Id,
			PolicyGroupIdsJson: string(groupBJson),
			PolicyIdsJson:      "[]",
			ModelName:          "claude-sonnet",
			ChannelId:          202,
			PromptTokens:       4,
			CompletionTokens:   2,
			TotalTokens:        6,
			Quota:              20,
			Status:             "success",
			CreatedAt:          1001,
		},
		{
			RequestId:          "req-usage-3",
			UserId:             1001,
			TokenId:            10,
			EnterpriseId:       enterprise.Id,
			OrgUnitId:          engineeringId,
			ProjectId:          projectA.Id,
			PolicyGroupIdsJson: string(bothGroupsJson),
			PolicyIdsJson:      "[]",
			ModelName:          "gpt-4o",
			ChannelId:          101,
			PromptTokens:       2,
			CompletionTokens:   1,
			TotalTokens:        3,
			Quota:              5,
			Status:             "success",
			CreatedAt:          2678400,
		},
		{
			RequestId:          "req-usage-outside",
			UserId:             1001,
			TokenId:            10,
			EnterpriseId:       enterprise.Id,
			OrgUnitId:          engineeringId,
			ProjectId:          projectA.Id,
			PolicyGroupIdsJson: string(groupAJson),
			PolicyIdsJson:      "[]",
			ModelName:          "gpt-4o",
			ChannelId:          101,
			PromptTokens:       100,
			CompletionTokens:   100,
			TotalTokens:        200,
			Quota:              999,
			Status:             "success",
			CreatedAt:          10,
		},
	}).Error)

	ctx, recorder := newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=2688500&org_unit_id="+itoaForEnterpriseTest(engineeringId), "")
	GetEnterpriseUsageSummary(ctx)
	response := decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var summary enterpriseUsageSummary
	decodeEnterpriseResponseData(t, response, &summary)
	assert.EqualValues(t, 2, summary.Total.RequestCount)
	assert.EqualValues(t, 15, summary.Total.Quota)
	assert.EqualValues(t, 8, summary.Total.TotalTokens)
	require.Len(t, summary.ByModel, 1)
	assert.Equal(t, "gpt-4o", summary.ByModel[0].ModelName)
	assert.EqualValues(t, 15, summary.ByModel[0].Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=2688500&policy_group_id="+itoaForEnterpriseTest(groupB.Id), "")
	GetEnterpriseUsageSummary(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &summary)
	assert.EqualValues(t, 2, summary.Total.RequestCount)
	assert.EqualValues(t, 25, summary.Total.Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=2688500&project_id="+itoaForEnterpriseTest(projectA.Id), "")
	GetEnterpriseUsageSummary(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &summary)
	assert.EqualValues(t, 2, summary.Total.RequestCount)
	assert.EqualValues(t, 15, summary.Total.Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/summary?start_time=900&end_time=2688500&channel_id=101&token_id=10&model_name=gpt-4o", "")
	GetEnterpriseUsageSummary(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &summary)
	assert.EqualValues(t, 2, summary.Total.RequestCount)
	assert.EqualValues(t, 15, summary.Total.Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=user&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page struct {
		Total int                            `json:"total"`
		Items []enterpriseUsageBreakdownItem `json:"items"`
	}
	decodeEnterpriseResponseData(t, response, &page)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, 1002, page.Items[0].TargetId)
	assert.Equal(t, "Bob", page.Items[0].TargetName)
	assert.EqualValues(t, 20, page.Items[0].Quota)
	assert.Equal(t, 1001, page.Items[1].TargetId)
	assert.EqualValues(t, 15, page.Items[1].Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=policy_group&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &page)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, groupB.Id, page.Items[0].TargetId)
	assert.Equal(t, "试点用户", page.Items[0].TargetName)
	assert.EqualValues(t, 25, page.Items[0].Quota)
	assert.Equal(t, groupA.Id, page.Items[1].TargetId)
	assert.EqualValues(t, 15, page.Items[1].Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=project&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &page)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, projectB.Id, page.Items[0].TargetId)
	assert.Equal(t, "销售成本中心", page.Items[0].TargetName)
	assert.EqualValues(t, 20, page.Items[0].Quota)
	assert.Equal(t, projectA.Id, page.Items[1].TargetId)
	assert.Equal(t, "研发成本中心", page.Items[1].TargetName)
	assert.EqualValues(t, 15, page.Items[1].Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=channel&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &page)
	require.Len(t, page.Items, 2)
	assert.Equal(t, 202, page.Items[0].TargetId)
	assert.Equal(t, "Claude Backup", page.Items[0].TargetName)
	assert.EqualValues(t, 20, page.Items[0].Quota)
	assert.Equal(t, 101, page.Items[1].TargetId)
	assert.Equal(t, "OpenAI Primary", page.Items[1].TargetName)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=api_key&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &page)
	require.Len(t, page.Items, 2)
	assert.Equal(t, 20, page.Items[0].TargetId)
	assert.Equal(t, "Bob Key", page.Items[0].TargetName)
	assert.EqualValues(t, 20, page.Items[0].Quota)
	assert.Equal(t, 10, page.Items[1].TargetId)
	assert.Equal(t, "Alice Key", page.Items[1].TargetName)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown?start_time=900&end_time=2688500&dimension=time&granularity=month&sort_by=quota&sort_order=desc&page_size=10", "")
	GetEnterpriseUsageBreakdown(ctx)
	response = decodeEnterpriseControllerResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	decodeEnterpriseResponseData(t, response, &page)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "1970-01", page.Items[0].TimeBucket)
	assert.EqualValues(t, 30, page.Items[0].Quota)
	assert.Equal(t, "1970-02", page.Items[1].TimeBucket)
	assert.EqualValues(t, 5, page.Items[1].Quota)

	ctx, recorder = newEnterpriseControllerContext(t, http.MethodGet, "/api/enterprise/usage/breakdown/export?start_time=900&end_time=2688500&dimension=project&sort_by=quota&sort_order=desc", "")
	ExportEnterpriseUsageBreakdown(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/csv")
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(recorder.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF})))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{"dimension", "target_id", "target_name", "model_name", "status", "time_bucket", "request_count", "quota", "prompt_tokens", "completion_tokens", "total_tokens"}, records[0])
	assert.Equal(t, "project", records[1][0])
	assert.Equal(t, "销售成本中心", records[1][2])
	assert.Equal(t, "20", records[1][7])
	assert.Equal(t, "研发成本中心", records[2][2])
	assert.Equal(t, "15", records[2][7])
}

func decodeEnterpriseResponseData(t *testing.T, response enterpriseControllerResponse, out any) {
	t.Helper()
	bytes, err := common.Marshal(response.Data)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(bytes, out))
}

func itoaForEnterpriseTest(value int) string {
	return strconv.Itoa(value)
}
