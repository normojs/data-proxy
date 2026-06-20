package model

const (
	OrgUnitStatusEnabled  = 1
	OrgUnitStatusDisabled = 2
)

type EnterpriseOrgUnit struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_org_units_status,priority:1;index:idx_enterprise_org_units_parent,priority:1;uniqueIndex:idx_enterprise_org_units_slug,priority:1"`
	ParentId     int    `json:"parent_id" gorm:"index:idx_enterprise_org_units_parent,priority:2"`
	Name         string `json:"name" gorm:"type:varchar(128);not null"`
	Slug         string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex:idx_enterprise_org_units_slug,priority:2"`
	Description  string `json:"description" gorm:"type:text"`
	Path         string `json:"path" gorm:"type:varchar(512);not null;default:'';index"`
	Depth        int    `json:"depth" gorm:"not null;default:0"`
	Sort         int    `json:"sort" gorm:"not null;default:0"`
	Status       int    `json:"status" gorm:"not null;default:1;index:idx_enterprise_org_units_status,priority:2"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseOrgUnit) TableName() string {
	return "enterprise_org_units"
}

type EnterpriseOrgMembership struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_enterprise_org_memberships_user,priority:1;index:idx_enterprise_org_memberships_org_unit,priority:1"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_enterprise_org_memberships_user,priority:2;index"`
	OrgUnitId    int    `json:"org_unit_id" gorm:"index:idx_enterprise_org_memberships_org_unit,priority:2"`
	Role         string `json:"role" gorm:"type:varchar(64);not null;default:''"`
	IsPrimary    bool   `json:"is_primary" gorm:"not null;default:true"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseOrgMembership) TableName() string {
	return "enterprise_org_memberships"
}
