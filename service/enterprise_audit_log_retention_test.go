package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupEnterpriseAuditLogsByRetentionDryRunDoesNotDelete(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := time.Unix(2000000000, 0)
	oldCreatedAt := now.Add(-40 * 24 * time.Hour).Unix()
	freshCreatedAt := now.Add(-5 * 24 * time.Hour).Unix()
	require.NoError(t, model.DB.Create(&[]model.EnterpriseAuditLog{
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  1001,
			Action:       "quota_policy.update",
			TargetType:   "quota_policy",
			TargetId:     11,
			AfterJson:    `{"token":"secret-value"}`,
			RequestId:    "audit-retention-old",
			CreatedAt:    oldCreatedAt,
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  1001,
			Action:       "quota_policy.update",
			TargetType:   "quota_policy",
			TargetId:     12,
			AfterJson:    `{}`,
			RequestId:    "audit-retention-fresh",
			CreatedAt:    freshCreatedAt,
		},
	}).Error)

	result, err := CleanupEnterpriseAuditLogsByRetention(EnterpriseAuditLogRetentionCleanupParams{
		RetentionDays: 30,
		Limit:         100,
		DryRun:        true,
		Now:           now,
	})
	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, 1, result.Scanned)
	assert.Equal(t, 1, result.WouldDelete)
	assert.Equal(t, 0, result.Deleted)
	assert.Equal(t, map[int]int{enterprise.Id: 1}, result.EnterpriseCounts)

	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).Count(&count).Error)
	assert.EqualValues(t, 2, count)
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("action = ?", "audit_log.retention_cleanup").
		Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestCleanupEnterpriseAuditLogsByRetentionDeletesAndAudits(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	otherEnterprise := model.Enterprise{
		Name:     "Other Enterprise",
		Slug:     "other-enterprise",
		Status:   model.EnterpriseStatusEnabled,
		Timezone: "Asia/Shanghai",
	}
	require.NoError(t, model.DB.Create(&otherEnterprise).Error)
	now := time.Unix(2000000000, 0)
	oldCreatedAt := now.Add(-45 * 24 * time.Hour).Unix()
	freshCreatedAt := now.Add(-2 * 24 * time.Hour).Unix()
	require.NoError(t, model.DB.Create(&[]model.EnterpriseAuditLog{
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  1001,
			Action:       "quota_policy.update",
			TargetType:   "quota_policy",
			TargetId:     11,
			AfterJson:    `{"client_secret":"secret-value"}`,
			RequestId:    "audit-retention-old-default",
			CreatedAt:    oldCreatedAt,
		},
		{
			EnterpriseId: otherEnterprise.Id,
			ActorUserId:  1002,
			Action:       "quota_policy.update",
			TargetType:   "quota_policy",
			TargetId:     12,
			AfterJson:    `{}`,
			RequestId:    "audit-retention-old-other",
			CreatedAt:    oldCreatedAt + 10,
		},
		{
			EnterpriseId: enterprise.Id,
			ActorUserId:  1001,
			Action:       "quota_policy.update",
			TargetType:   "quota_policy",
			TargetId:     13,
			AfterJson:    `{}`,
			RequestId:    "audit-retention-fresh-default",
			CreatedAt:    freshCreatedAt,
		},
	}).Error)

	result, err := CleanupEnterpriseAuditLogsByRetention(EnterpriseAuditLogRetentionCleanupParams{
		RetentionDays: 30,
		Limit:         100,
		ActorUserId:   9001,
		RequestId:     "retention-run-1",
		Now:           now,
	})
	require.NoError(t, err)
	assert.False(t, result.DryRun)
	assert.Equal(t, 2, result.WouldDelete)
	assert.Equal(t, 2, result.Deleted)
	assert.Equal(t, map[int]int{enterprise.Id: 1, otherEnterprise.Id: 1}, result.EnterpriseCounts)

	var oldCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("request_id IN ?", []string{"audit-retention-old-default", "audit-retention-old-other"}).
		Count(&oldCount).Error)
	assert.EqualValues(t, 0, oldCount)
	var freshCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("request_id = ?", "audit-retention-fresh-default").
		Count(&freshCount).Error)
	assert.EqualValues(t, 1, freshCount)

	var cleanupAudits []model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ?", "audit_log.retention_cleanup").
		Order("enterprise_id asc").
		Find(&cleanupAudits).Error)
	require.Len(t, cleanupAudits, 2)
	for _, audit := range cleanupAudits {
		assert.Equal(t, 9001, audit.ActorUserId)
		assert.Equal(t, "enterprise", audit.TargetType)
		assert.Equal(t, audit.EnterpriseId, audit.TargetId)
		assert.Equal(t, "retention-run-1", audit.RequestId)
		var payload map[string]any
		require.NoError(t, common.UnmarshalJsonStr(audit.AfterJson, &payload))
		assert.EqualValues(t, 30, payload["retention_days"])
		assert.EqualValues(t, 1, payload["deleted_count"])
		assert.EqualValues(t, false, payload["dry_run"])
	}
}

func TestCleanupEnterpriseAuditLogsByRetentionLimitHasMore(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := time.Unix(2000000000, 0)
	oldCreatedAt := now.Add(-90 * 24 * time.Hour).Unix()
	require.NoError(t, model.DB.Create(&[]model.EnterpriseAuditLog{
		{EnterpriseId: enterprise.Id, Action: "audit.one", TargetType: "enterprise", TargetId: enterprise.Id, CreatedAt: oldCreatedAt},
		{EnterpriseId: enterprise.Id, Action: "audit.two", TargetType: "enterprise", TargetId: enterprise.Id, CreatedAt: oldCreatedAt + 1},
	}).Error)

	result, err := CleanupEnterpriseAuditLogsByRetention(EnterpriseAuditLogRetentionCleanupParams{
		RetentionDays: 30,
		Limit:         1,
		Now:           now,
	})
	require.NoError(t, err)
	assert.True(t, result.HasMore)
	assert.Equal(t, 1, result.Deleted)

	var remainingOld int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).
		Where("action IN ?", []string{"audit.one", "audit.two"}).
		Count(&remainingOld).Error)
	assert.EqualValues(t, 1, remainingOld)
}
