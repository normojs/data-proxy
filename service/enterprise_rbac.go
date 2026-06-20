package service

import (
	"errors"
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
	EnterpriseCapabilityRead          = "enterprise.read"
	EnterpriseCapabilityManage        = "enterprise.manage"
	EnterpriseCapabilityFinanceRead   = "enterprise.finance.read"
	EnterpriseCapabilityAuditRead     = "enterprise.audit.read"
	EnterpriseCapabilityQuotaApprove  = "enterprise.quota.approve"
	EnterpriseCapabilityProjectManage = "enterprise.project.manage"
)

type EnterpriseUserPermissions struct {
	Read          bool `json:"read"`
	Manage        bool `json:"manage"`
	FinanceRead   bool `json:"finance_read"`
	AuditRead     bool `json:"audit_read"`
	QuotaApprove  bool `json:"quota_approve"`
	ProjectManage bool `json:"project_manage"`
}

func UserHasEnterpriseCapability(userId int, systemRole int, capability string) (bool, error) {
	if systemRole >= common.RoleAdminUser {
		return true, nil
	}
	if !common.EnterpriseGovernanceEnabled {
		return false, nil
	}
	if userId <= 0 || strings.TrimSpace(capability) == "" {
		return false, nil
	}
	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	var membership model.EnterpriseOrgMembership
	err = model.DB.Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", enterprise.Id, userId, true).First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return EnterpriseRoleHasCapability(membership.Role, capability), nil
}

func EnterprisePermissionsForUser(userId int, systemRole int) (EnterpriseUserPermissions, error) {
	if systemRole >= common.RoleAdminUser {
		return EnterpriseUserPermissions{
			Read:          true,
			Manage:        true,
			FinanceRead:   true,
			AuditRead:     true,
			QuotaApprove:  true,
			ProjectManage: true,
		}, nil
	}
	if !common.EnterpriseGovernanceEnabled {
		return EnterpriseUserPermissions{}, nil
	}
	if userId <= 0 {
		return EnterpriseUserPermissions{}, nil
	}
	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnterpriseUserPermissions{}, nil
		}
		return EnterpriseUserPermissions{}, err
	}
	var membership model.EnterpriseOrgMembership
	err = model.DB.Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", enterprise.Id, userId, true).First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnterpriseUserPermissions{}, nil
		}
		return EnterpriseUserPermissions{}, err
	}
	return EnterprisePermissionsForRole(membership.Role), nil
}

func EnterprisePermissionsForRole(role string) EnterpriseUserPermissions {
	return EnterpriseUserPermissions{
		Read:          EnterpriseRoleHasCapability(role, EnterpriseCapabilityRead),
		Manage:        EnterpriseRoleHasCapability(role, EnterpriseCapabilityManage),
		FinanceRead:   EnterpriseRoleHasCapability(role, EnterpriseCapabilityFinanceRead),
		AuditRead:     EnterpriseRoleHasCapability(role, EnterpriseCapabilityAuditRead),
		QuotaApprove:  EnterpriseRoleHasCapability(role, EnterpriseCapabilityQuotaApprove),
		ProjectManage: EnterpriseRoleHasCapability(role, EnterpriseCapabilityProjectManage),
	}
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
		}
	default:
		return nil
	}
}

func normalizeEnterpriseRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	role = strings.ReplaceAll(role, "-", "_")
	role = strings.ReplaceAll(role, " ", "_")
	return role
}
