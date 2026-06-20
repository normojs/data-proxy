package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const (
	EnterpriseRoleOwner           = "owner"
	EnterpriseRoleEnterpriseAdmin = "enterprise_admin"
	EnterpriseRoleAdmin           = "admin"
	EnterpriseRoleDepartmentAdmin = "department_admin"
	EnterpriseRoleFinanceViewer   = "finance_viewer"
	EnterpriseRoleAuditor         = "auditor"
	EnterpriseRoleProjectAdmin    = "project_admin"
)

const (
	EnterpriseCapabilityRead             = "enterprise.read"
	EnterpriseCapabilityManage           = "enterprise.manage"
	EnterpriseCapabilityFinanceRead      = "enterprise.finance.read"
	EnterpriseCapabilityAuditRead        = "enterprise.audit.read"
	EnterpriseCapabilityQuotaApprove     = "enterprise.quota.approve"
	EnterpriseCapabilityProjectManage    = "enterprise.project.manage"
	EnterpriseCapabilityDepartmentManage = "enterprise.department.manage"
)

type EnterpriseUserPermissions struct {
	Read             bool `json:"read"`
	Manage           bool `json:"manage"`
	FinanceRead      bool `json:"finance_read"`
	AuditRead        bool `json:"audit_read"`
	QuotaApprove     bool `json:"quota_approve"`
	ProjectManage    bool `json:"project_manage"`
	DepartmentManage bool `json:"department_manage"`
}

type EnterpriseAccess struct {
	EnterpriseId     int
	Role             string
	OrgUnitId        int
	ScopedOrgUnitIds []int
	ScopedProjectIds []int
	Permissions      EnterpriseUserPermissions
	SystemAdmin      bool
}

func UserHasEnterpriseCapability(userId int, systemRole int, capability string) (bool, error) {
	if userId <= 0 || strings.TrimSpace(capability) == "" {
		return false, nil
	}
	access, err := EnterpriseAccessForUser(userId, systemRole)
	if err != nil {
		return false, err
	}
	return access.HasCapability(capability), nil
}

func EnterprisePermissionsForUser(userId int, systemRole int) (EnterpriseUserPermissions, error) {
	access, err := EnterpriseAccessForUser(userId, systemRole)
	if err != nil {
		return EnterpriseUserPermissions{}, err
	}
	return access.Permissions, nil
}

func EnterpriseAccessForUser(userId int, systemRole int) (EnterpriseAccess, error) {
	if systemRole >= common.RoleAdminUser {
		return EnterpriseAccess{
			Permissions: EnterpriseUserPermissions{
				Read:             true,
				Manage:           true,
				FinanceRead:      true,
				AuditRead:        true,
				QuotaApprove:     true,
				ProjectManage:    true,
				DepartmentManage: true,
			},
			SystemAdmin: true,
		}, nil
	}
	if !common.EnterpriseGovernanceEnabled || userId <= 0 {
		return EnterpriseAccess{}, nil
	}
	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnterpriseAccess{}, nil
		}
		return EnterpriseAccess{}, err
	}
	var membership model.EnterpriseOrgMembership
	err = model.DB.Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", enterprise.Id, userId, true).First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnterpriseAccess{}, nil
		}
		return EnterpriseAccess{}, err
	}
	permissions := EnterprisePermissionsForRole(membership.Role)
	access := EnterpriseAccess{
		EnterpriseId: enterprise.Id,
		Role:         normalizeEnterpriseRole(membership.Role),
		OrgUnitId:    membership.OrgUnitId,
		Permissions:  permissions,
	}
	if normalizeEnterpriseRole(membership.Role) == EnterpriseRoleDepartmentAdmin {
		if membership.OrgUnitId <= 0 {
			access.Permissions.DepartmentManage = false
			access.Permissions.FinanceRead = false
			access.Permissions.QuotaApprove = false
			return access, nil
		}
		scopeIds, err := EnterpriseOrgUnitScopeIds(enterprise.Id, membership.OrgUnitId)
		if err != nil {
			return EnterpriseAccess{}, err
		}
		if len(scopeIds) == 0 {
			access.Permissions.DepartmentManage = false
			access.Permissions.FinanceRead = false
			access.Permissions.QuotaApprove = false
			return access, nil
		}
		access.ScopedOrgUnitIds = scopeIds
	}
	if normalizeEnterpriseRole(membership.Role) == EnterpriseRoleProjectAdmin {
		projectIds, err := EnterpriseOwnedProjectIds(enterprise.Id, userId)
		if err != nil {
			return EnterpriseAccess{}, err
		}
		access.ScopedProjectIds = projectIds
	}
	return access, nil
}

func EnterprisePermissionsForRole(role string) EnterpriseUserPermissions {
	return EnterpriseUserPermissions{
		Read:             EnterpriseRoleHasCapability(role, EnterpriseCapabilityRead),
		Manage:           EnterpriseRoleHasCapability(role, EnterpriseCapabilityManage),
		FinanceRead:      EnterpriseRoleHasCapability(role, EnterpriseCapabilityFinanceRead),
		AuditRead:        EnterpriseRoleHasCapability(role, EnterpriseCapabilityAuditRead),
		QuotaApprove:     EnterpriseRoleHasCapability(role, EnterpriseCapabilityQuotaApprove),
		ProjectManage:    EnterpriseRoleHasCapability(role, EnterpriseCapabilityProjectManage),
		DepartmentManage: EnterpriseRoleHasCapability(role, EnterpriseCapabilityDepartmentManage),
	}
}

func (access EnterpriseAccess) HasCapability(capability string) bool {
	switch strings.TrimSpace(capability) {
	case EnterpriseCapabilityRead:
		return access.Permissions.Read
	case EnterpriseCapabilityManage:
		return access.Permissions.Manage
	case EnterpriseCapabilityFinanceRead:
		return access.Permissions.FinanceRead
	case EnterpriseCapabilityAuditRead:
		return access.Permissions.AuditRead
	case EnterpriseCapabilityQuotaApprove:
		return access.Permissions.QuotaApprove
	case EnterpriseCapabilityProjectManage:
		return access.Permissions.ProjectManage
	case EnterpriseCapabilityDepartmentManage:
		return access.Permissions.DepartmentManage
	default:
		return false
	}
}

func (access EnterpriseAccess) HasDepartmentScope() bool {
	return !access.SystemAdmin && access.Permissions.DepartmentManage && !access.Permissions.Manage && len(access.ScopedOrgUnitIds) > 0
}

func (access EnterpriseAccess) HasProjectScope() bool {
	return !access.SystemAdmin && access.Role == EnterpriseRoleProjectAdmin && access.Permissions.ProjectManage && !access.Permissions.Manage
}

func (access EnterpriseAccess) OrgUnitInScope(orgUnitId int) bool {
	return EnterpriseOrgUnitInScope(orgUnitId, access.ScopedOrgUnitIds)
}

func (access EnterpriseAccess) ProjectInScope(projectId int) bool {
	return EnterpriseProjectInScope(projectId, access.ScopedProjectIds)
}

func EnterpriseRoleHasCapability(role string, capability string) bool {
	role = normalizeEnterpriseRole(role)
	capability = strings.TrimSpace(capability)
	if role == "" || capability == "" {
		return false
	}
	for _, allowed := range enterpriseCapabilitiesForRole(role) {
		if allowed == capability {
			return true
		}
	}
	return false
}

func enterpriseCapabilitiesForRole(role string) []string {
	switch normalizeEnterpriseRole(role) {
	case EnterpriseRoleOwner, EnterpriseRoleEnterpriseAdmin, EnterpriseRoleAdmin:
		return []string{
			EnterpriseCapabilityRead,
			EnterpriseCapabilityManage,
			EnterpriseCapabilityFinanceRead,
			EnterpriseCapabilityAuditRead,
			EnterpriseCapabilityQuotaApprove,
			EnterpriseCapabilityProjectManage,
		}
	case EnterpriseRoleFinanceViewer:
		return []string{
			EnterpriseCapabilityRead,
			EnterpriseCapabilityFinanceRead,
		}
	case EnterpriseRoleAuditor:
		return []string{
			EnterpriseCapabilityRead,
			EnterpriseCapabilityAuditRead,
		}
	case EnterpriseRoleProjectAdmin:
		return []string{
			EnterpriseCapabilityRead,
			EnterpriseCapabilityFinanceRead,
			EnterpriseCapabilityProjectManage,
		}
	case EnterpriseRoleDepartmentAdmin:
		return []string{
			EnterpriseCapabilityRead,
			EnterpriseCapabilityFinanceRead,
			EnterpriseCapabilityQuotaApprove,
			EnterpriseCapabilityDepartmentManage,
		}
	default:
		return nil
	}
}

func EnterpriseOrgUnitScopeIds(enterpriseId int, orgUnitId int) ([]int, error) {
	if enterpriseId <= 0 || orgUnitId <= 0 {
		return []int{}, nil
	}
	var root model.EnterpriseOrgUnit
	if err := model.DB.Where("enterprise_id = ? AND id = ?", enterpriseId, orgUnitId).First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []int{}, nil
		}
		return nil, err
	}
	var units []model.EnterpriseOrgUnit
	if err := model.DB.Select("id").Where("enterprise_id = ? AND path LIKE ?", enterpriseId, root.Path+"%").Find(&units).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(units))
	for _, unit := range units {
		ids = append(ids, unit.Id)
	}
	sort.Ints(ids)
	return ids, nil
}

func EnterpriseOwnedProjectIds(enterpriseId int, ownerUserId int) ([]int, error) {
	if enterpriseId <= 0 || ownerUserId <= 0 {
		return []int{}, nil
	}
	var projects []model.EnterpriseProject
	if err := model.DB.Select("id").Where("enterprise_id = ? AND owner_user_id = ? AND status = ?", enterpriseId, ownerUserId, model.EnterpriseProjectStatusEnabled).Find(&projects).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.Id)
	}
	sort.Ints(ids)
	return ids, nil
}

func EnterpriseOrgUnitInScope(orgUnitId int, scopeIds []int) bool {
	if orgUnitId <= 0 {
		return false
	}
	for _, scopeId := range scopeIds {
		if scopeId == orgUnitId {
			return true
		}
	}
	return false
}

func EnterpriseProjectInScope(projectId int, scopeIds []int) bool {
	if projectId <= 0 {
		return false
	}
	for _, scopeId := range scopeIds {
		if scopeId == projectId {
			return true
		}
	}
	return false
}

func EnterpriseUserInOrgUnitScope(enterpriseId int, userId int, scopeIds []int) (bool, error) {
	if enterpriseId <= 0 || userId <= 0 || len(scopeIds) == 0 {
		return false, nil
	}
	var count int64
	err := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ? AND user_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, userId, true, scopeIds).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func normalizeEnterpriseRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	role = strings.ReplaceAll(role, "-", "_")
	role = strings.ReplaceAll(role, " ", "_")
	return role
}
