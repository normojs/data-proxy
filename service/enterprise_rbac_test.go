package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseRoleCapabilityMapping(t *testing.T) {
	require.True(t, EnterpriseRoleHasCapability(EnterpriseRoleEnterpriseAdmin, EnterpriseCapabilityManage))
	require.True(t, EnterpriseRoleHasCapability(EnterpriseRoleEnterpriseAdmin, EnterpriseCapabilityQuotaApprove))
	require.True(t, EnterpriseRoleHasCapability(EnterpriseRoleFinanceViewer, EnterpriseCapabilityFinanceRead))
	require.False(t, EnterpriseRoleHasCapability(EnterpriseRoleFinanceViewer, EnterpriseCapabilityAuditRead))
	require.True(t, EnterpriseRoleHasCapability(EnterpriseRoleAuditor, EnterpriseCapabilityAuditRead))
	require.False(t, EnterpriseRoleHasCapability(EnterpriseRoleAuditor, EnterpriseCapabilityManage))
	require.True(t, EnterpriseRoleHasCapability("project-admin", EnterpriseCapabilityProjectManage))
	require.True(t, EnterpriseRoleHasCapability("project-admin", EnterpriseCapabilityProjectRead))
	require.False(t, EnterpriseRoleHasCapability("project-admin", EnterpriseCapabilityFinanceRead))
	require.True(t, EnterpriseRoleHasCapability(EnterpriseRoleDepartmentAdmin, EnterpriseCapabilityRead))
	require.False(t, EnterpriseRoleHasCapability(EnterpriseRoleDepartmentAdmin, EnterpriseCapabilityManage))
}

func TestEnterprisePermissionsRequirePrimaryMembership(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)

	userId := 8301
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       userId,
		Role:         EnterpriseRoleFinanceViewer,
	}).Error)
	require.NoError(t, model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ? AND user_id = ?", enterprise.Id, userId).
		UpdateColumn("is_primary", false).Error)

	allowed, err := UserHasEnterpriseCapability(userId, common.RoleCommonUser, EnterpriseCapabilityFinanceRead)
	require.NoError(t, err)
	require.False(t, allowed)

	permissions, err := EnterprisePermissionsForUser(userId, common.RoleCommonUser)
	require.NoError(t, err)
	require.False(t, permissions.Read)
	require.False(t, permissions.FinanceRead)

	require.NoError(t, model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ? AND user_id = ?", enterprise.Id, userId).
		Update("is_primary", true).Error)

	allowed, err = UserHasEnterpriseCapability(userId, common.RoleCommonUser, EnterpriseCapabilityFinanceRead)
	require.NoError(t, err)
	require.True(t, allowed)

	permissions, err = EnterprisePermissionsForUser(userId, common.RoleCommonUser)
	require.NoError(t, err)
	require.True(t, permissions.Read)
	require.True(t, permissions.FinanceRead)
}
