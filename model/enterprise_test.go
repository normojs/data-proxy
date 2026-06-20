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
	require.NoError(t, DB.Exec("DELETE FROM enterprise_policy_group_members").Error)
	require.NoError(t, DB.Exec("DELETE FROM enterprise_policy_groups").Error)
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
		&EnterprisePolicyGroup{},
		&EnterprisePolicyGroupMember{},
		&EnterpriseQuotaPolicy{},
		&EnterpriseQuotaCounter{},
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

func TestEnterpriseGovernanceOptionsDefaultDisabled(t *testing.T) {
	require.NoError(t, DB.Where("key IN ?", []string{
		"EnterpriseGovernanceEnabled",
		"EnterpriseGovernanceDryRunEnabled",
	}).Delete(&Option{}).Error)
	common.EnterpriseGovernanceEnabled = false
	common.EnterpriseGovernanceDryRunEnabled = false

	InitOptionMap()

	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "false", common.OptionMap["EnterpriseGovernanceEnabled"])
	assert.Equal(t, "false", common.OptionMap["EnterpriseGovernanceDryRunEnabled"])
	common.OptionMapRWMutex.RUnlock()
	assert.False(t, common.EnterpriseGovernanceEnabled)
	assert.False(t, common.EnterpriseGovernanceDryRunEnabled)

	require.NoError(t, UpdateOption("EnterpriseGovernanceEnabled", "true"))
	require.NoError(t, UpdateOption("EnterpriseGovernanceDryRunEnabled", "true"))
	assert.True(t, common.EnterpriseGovernanceEnabled)
	assert.True(t, common.EnterpriseGovernanceDryRunEnabled)

	t.Cleanup(func() {
		_ = UpdateOption("EnterpriseGovernanceEnabled", "false")
		_ = UpdateOption("EnterpriseGovernanceDryRunEnabled", "false")
	})
}
