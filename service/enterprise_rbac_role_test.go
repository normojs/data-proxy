package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateEnterpriseMemberRoleRejectsLastAdminRemoval(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       2201,
		Username: "only-admin",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff2201",
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       2201,
		OrgUnitId:    0,
		Role:         EnterpriseRoleEnterpriseAdmin,
		IsPrimary:    true,
	}).Error)

	_, _, err = UpdateEnterpriseMemberRole(enterprise.Id, 2201, EnterpriseRoleAuditor)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "last enterprise administrator")

	require.NoError(t, model.DB.Create(&model.User{
		Id:       2202,
		Username: "second-admin",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff2202",
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       2202,
		OrgUnitId:    0,
		Role:         EnterpriseRoleAdmin,
		IsPrimary:    true,
	}).Error)

	before, after, err := UpdateEnterpriseMemberRole(enterprise.Id, 2201, EnterpriseRoleAuditor)
	require.NoError(t, err)
	assert.Equal(t, EnterpriseRoleEnterpriseAdmin, before.Role)
	assert.Equal(t, EnterpriseRoleAuditor, after.Role)

	var membership model.EnterpriseOrgMembership
	require.NoError(t, model.DB.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, 2201).First(&membership).Error)
	assert.Equal(t, EnterpriseRoleAuditor, membership.Role)
}

func TestIsEnterpriseAssignableRole(t *testing.T) {
	assert.True(t, IsEnterpriseAssignableRole("enterprise_admin"))
	assert.True(t, IsEnterpriseAssignableRole("Department-Admin"))
	assert.True(t, IsEnterpriseAssignableRole(""))
	assert.False(t, IsEnterpriseAssignableRole("superuser"))
}
