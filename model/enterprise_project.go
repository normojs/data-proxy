package model

const (
	EnterpriseProjectStatusEnabled  = 1
	EnterpriseProjectStatusDisabled = 2

	EnterpriseProjectMemberRoleAdmin  = "admin"
	EnterpriseProjectMemberRoleMember = "member"
)

type EnterpriseProject struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_projects_status,priority:1;uniqueIndex:idx_enterprise_projects_slug,priority:1"`
	Name         string `json:"name" gorm:"type:varchar(128);not null"`
	Slug         string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex:idx_enterprise_projects_slug,priority:2"`
	Description  string `json:"description" gorm:"type:text"`
	OwnerUserId  int    `json:"owner_user_id" gorm:"index"`
	Status       int    `json:"status" gorm:"not null;default:1;index:idx_enterprise_projects_status,priority:2"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseProject) TableName() string {
	return "enterprise_projects"
}

type EnterpriseProjectOrgUnit struct {
	Id           int   `json:"id" gorm:"primaryKey"`
	EnterpriseId int   `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_enterprise_project_org_units,priority:1;index:idx_enterprise_project_org_units_org,priority:1"`
	ProjectId    int   `json:"project_id" gorm:"not null;uniqueIndex:idx_enterprise_project_org_units,priority:2;index"`
	OrgUnitId    int   `json:"org_unit_id" gorm:"not null;uniqueIndex:idx_enterprise_project_org_units,priority:3;index:idx_enterprise_project_org_units_org,priority:2"`
	CreatedAt    int64 `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseProjectOrgUnit) TableName() string {
	return "enterprise_project_org_units"
}

type EnterpriseProjectMember struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"not null;index"`
	ProjectId    int    `json:"project_id" gorm:"not null;uniqueIndex:idx_enterprise_project_members_user,priority:1;index"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_enterprise_project_members_user,priority:2;index"`
	Role         string `json:"role" gorm:"type:varchar(32);not null;default:'member';index"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseProjectMember) TableName() string {
	return "enterprise_project_members"
}
