package model

const (
	PolicyGroupStatusEnabled  = 1
	PolicyGroupStatusDisabled = 2

	PolicyGroupMemberRoleEditor = "editor"
	PolicyGroupMemberRoleViewer = "viewer"
)

type EnterprisePolicyGroup struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_policy_groups_status,priority:1;uniqueIndex:idx_enterprise_policy_groups_slug,priority:1"`
	OrgUnitId    int    `json:"org_unit_id" gorm:"index"`
	Name         string `json:"name" gorm:"type:varchar(128);not null"`
	Slug         string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex:idx_enterprise_policy_groups_slug,priority:2"`
	Description  string `json:"description" gorm:"type:text"`
	Status       int    `json:"status" gorm:"not null;default:1;index:idx_enterprise_policy_groups_status,priority:2"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterprisePolicyGroup) TableName() string {
	return "enterprise_policy_groups"
}

type EnterprisePolicyGroupMember struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	EnterpriseId  int    `json:"enterprise_id" gorm:"not null;index"`
	PolicyGroupId int    `json:"policy_group_id" gorm:"not null;uniqueIndex:idx_enterprise_policy_group_members_user,priority:1;index"`
	UserId        int    `json:"user_id" gorm:"not null;uniqueIndex:idx_enterprise_policy_group_members_user,priority:2;index"`
	Role          string `json:"role" gorm:"type:varchar(32);not null;default:'viewer';index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterprisePolicyGroupMember) TableName() string {
	return "enterprise_policy_group_members"
}

type EnterprisePolicyGroupShare struct {
	Id            int   `json:"id" gorm:"primaryKey"`
	EnterpriseId  int   `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_enterprise_policy_group_shares,priority:1;index"`
	PolicyGroupId int   `json:"policy_group_id" gorm:"not null;uniqueIndex:idx_enterprise_policy_group_shares,priority:2;index"`
	OrgUnitId     int   `json:"org_unit_id" gorm:"not null;uniqueIndex:idx_enterprise_policy_group_shares,priority:3;index"`
	CreatedAt     int64 `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterprisePolicyGroupShare) TableName() string {
	return "enterprise_policy_group_shares"
}
