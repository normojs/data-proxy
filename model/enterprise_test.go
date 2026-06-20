package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnterpriseTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM enterprise_audit_logs").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_usage_attributions").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_quota_counters").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_quota_policies").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_quota_requests").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_policy_group_members").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_policy_groups").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_project_org_units").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_projects").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_org_memberships").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_org_units").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprises").Error)
}

func TestEnsureDefaultEnterpriseCreatesDefault(t *testing.T) {
	clearEnterpriseTables(t)

	require.NoError(t, EnsureDefaultEnterprise())

	enterprise, err := GetDefaultEnterprise()
	require.NoError(t, err)
	require.NotNil(t, enterprise)
	assert.Equal(t, DefaultEnterpriseName, enterprise.Name)
	assert.Equal(t, DefaultEnterpriseSlug, enterprise.Slug)
	assert.Equal(t, EnterpriseStatusEnabled, enterprise.Status)
	assert.Equal(t, DefaultEnterpriseTimezone, enterprise.Timezone)
}

func TestEnterpriseGovernanceTablesMigrated(t *testing.T) {
	models := []any{
		&Enterprise{},
		&EnterpriseOrgUnit{},
		&EnterpriseOrgMembership{},
		&EnterpriseProject{},
		&EnterpriseProjectOrgUnit{},
		&EnterprisePolicyGroup{},
		&EnterprisePolicyGroupMember{},
		&EnterpriseQuotaPolicy{},
		&EnterpriseQuotaCounter{},
		&EnterpriseQuotaRequest{},
		&EnterpriseUsageAttribution{},
		&EnterpriseAuditLog{},
	}
	for _, model := range models {
		assert.Truef(t, DB.Migrator().HasTable(model), "expected migrated table for %T", model)
	}
}

func TestEnsureDefaultEnterpriseIsIdempotent(t *testing.T) {
	clearEnterpriseTables(t)

	require.NoError(t, EnsureDefaultEnterprise())
	require.NoError(t, EnsureDefaultEnterprise())

	var count int64
	require.NoError(t, DB.Model(&Enterprise{}).Where("slug = ?", DefaultEnterpriseSlug).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestRecordEnterpriseAuditLog(t *testing.T) {
	clearEnterpriseTables(t)
	require.NoError(t, EnsureDefaultEnterprise())
	enterprise, err := GetDefaultEnterprise()
	require.NoError(t, err)

	err = RecordEnterpriseAuditLog(EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  7,
		Action:       "org_unit.update",
		TargetType:   "org_unit",
		TargetId:     12,
		Before:       map[string]any{"name": "old"},
		After:        map[string]any{"name": "new"},
		RequestId:    "req-enterprise-audit",
	})
	require.NoError(t, err)

	var log EnterpriseAuditLog
	require.NoError(t, DB.Where("request_id = ?", "req-enterprise-audit").First(&log).Error)
	assert.Equal(t, enterprise.Id, log.EnterpriseId)
	assert.Equal(t, 7, log.ActorUserId)
	assert.Equal(t, "org_unit.update", log.Action)
	assert.JSONEq(t, `{"name":"old"}`, log.BeforeJson)
	assert.JSONEq(t, `{"name":"new"}`, log.AfterJson)
}

func TestRecordEnterpriseAuditLogFillsScope(t *testing.T) {
	clearEnterpriseTables(t)
	require.NoError(t, EnsureDefaultEnterprise())
	enterprise, err := GetDefaultEnterprise()
	require.NoError(t, err)
	orgUnit := EnterpriseOrgUnit{
		EnterpriseId: enterprise.Id,
		Name:         "Scoped Audit Engineering",
		Slug:         "scoped-audit-engineering",
		Path:         "/1/",
		Status:       OrgUnitStatusEnabled,
	}
	require.NoError(t, DB.Create(&orgUnit).Error)
	require.NoError(t, DB.Create(&EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       9701,
		OrgUnitId:    orgUnit.Id,
		IsPrimary:    true,
	}).Error)
	project := EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         "Scoped Audit Project",
		Slug:         "scoped-audit-project",
		OwnerUserId:  9701,
		Status:       EnterpriseProjectStatusEnabled,
	}
	require.NoError(t, DB.Create(&project).Error)
	policy := EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "Scoped Audit Project Policy",
		TargetType:   PolicyTargetProject,
		TargetId:     project.Id,
		Metric:       PolicyMetricRequestCount,
		Period:       PolicyPeriodDay,
		LimitValue:   10,
		Timezone:     DefaultEnterpriseTimezone,
		ModelScope:   PolicyModelScopeAll,
		ModelsJson:   "[]",
		Action:       PolicyActionReject,
		Status:       QuotaPolicyStatusEnabled,
	}
	require.NoError(t, DB.Create(&policy).Error)
	quotaRequest := EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9701,
		PolicyId:        policy.Id,
		TargetType:      policy.TargetType,
		TargetId:        policy.TargetId,
		Metric:          policy.Metric,
		Period:          policy.Period,
		LimitDelta:      1,
		Status:          EnterpriseQuotaRequestStatusPending,
		ExpiresAt:       common.GetTimestamp() + 3600,
	}
	require.NoError(t, DB.Create(&quotaRequest).Error)

	require.NoError(t, RecordEnterpriseAuditLog(EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  9701,
		Action:       "quota_policy.update",
		TargetType:   "quota_policy",
		TargetId:     policy.Id,
		After:        policy,
		RequestId:    "scope-policy",
	}))
	require.NoError(t, RecordEnterpriseAuditLog(EnterpriseAuditInput{
		EnterpriseId: enterprise.Id,
		ActorUserId:  9701,
		Action:       "quota_request.submit",
		TargetType:   "quota_request",
		TargetId:     quotaRequest.Id,
		After:        quotaRequest,
		RequestId:    "scope-request",
	}))

	var policyAudit EnterpriseAuditLog
	require.NoError(t, DB.Where("request_id = ?", "scope-policy").First(&policyAudit).Error)
	assert.Equal(t, project.Id, policyAudit.ScopeProjectId)
	assert.Equal(t, 0, policyAudit.ScopeUserId)
	assert.Equal(t, 0, policyAudit.ScopeOrgUnitId)

	var requestAudit EnterpriseAuditLog
	require.NoError(t, DB.Where("request_id = ?", "scope-request").First(&requestAudit).Error)
	assert.Equal(t, 9701, requestAudit.ScopeUserId)
	assert.Equal(t, orgUnit.Id, requestAudit.ScopeOrgUnitId)
	assert.Equal(t, project.Id, requestAudit.ScopeProjectId)
}

func TestEnterpriseGovernanceOptionsDefaultDisabled(t *testing.T) {
	require.NoError(t, DB.Where("key IN ?", []string{
		"EnterpriseGovernanceEnabled",
		"EnterpriseGovernanceDryRunEnabled",
		"EnterpriseQuotaRedisCounterEnabled",
	}).Delete(&Option{}).Error)
	common.EnterpriseGovernanceEnabled = false
	common.EnterpriseGovernanceDryRunEnabled = false
	common.EnterpriseQuotaRedisCounterEnabled = false

	InitOptionMap()

	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "false", common.OptionMap["EnterpriseGovernanceEnabled"])
	assert.Equal(t, "false", common.OptionMap["EnterpriseGovernanceDryRunEnabled"])
	assert.Equal(t, "false", common.OptionMap["EnterpriseQuotaRedisCounterEnabled"])
	common.OptionMapRWMutex.RUnlock()
	assert.False(t, common.EnterpriseGovernanceEnabled)
	assert.False(t, common.EnterpriseGovernanceDryRunEnabled)
	assert.False(t, common.EnterpriseQuotaRedisCounterEnabled)

	require.NoError(t, UpdateOption("EnterpriseGovernanceEnabled", "true"))
	require.NoError(t, UpdateOption("EnterpriseGovernanceDryRunEnabled", "true"))
	require.NoError(t, UpdateOption("EnterpriseQuotaRedisCounterEnabled", "true"))
	assert.True(t, common.EnterpriseGovernanceEnabled)
	assert.True(t, common.EnterpriseGovernanceDryRunEnabled)
	assert.True(t, common.EnterpriseQuotaRedisCounterEnabled)

	t.Cleanup(func() {
		_ = UpdateOption("EnterpriseGovernanceEnabled", "false")
		_ = UpdateOption("EnterpriseGovernanceDryRunEnabled", "false")
		_ = UpdateOption("EnterpriseQuotaRedisCounterEnabled", "false")
	})
}
