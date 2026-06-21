package model

const (
	PolicyGroupStatusEnabled  = 1
	PolicyGroupStatusDisabled = 2

	PolicyGroupMemberRoleEditor = "editor"
	PolicyGroupMemberRoleViewer = "viewer"

	PolicyGroupShareRequestStatusPending   = "pending"
	PolicyGroupShareRequestStatusApproved  = "approved"
	PolicyGroupShareRequestStatusRejected  = "rejected"
	PolicyGroupShareRequestStatusWithdrawn = "withdrawn"
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
	ExpiresAt     int64 `json:"expires_at" gorm:"not null;default:0;index"`
	CreatedAt     int64 `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterprisePolicyGroupShare) TableName() string {
	return "enterprise_policy_group_shares"
}

type EnterprisePolicyGroupShareRequest struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	EnterpriseId       int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_policy_group_share_requests_status,priority:1;index"`
	PolicyGroupId      int    `json:"policy_group_id" gorm:"not null;index"`
	RequesterUserId    int    `json:"requester_user_id" gorm:"not null;index"`
	RequesterOrgUnitId int    `json:"requester_org_unit_id" gorm:"not null;index"`
	TargetOrgUnitId    int    `json:"target_org_unit_id" gorm:"not null;index"`
	SharedExpiresAt    int64  `json:"shared_expires_at" gorm:"not null;default:0;index"`
	Reason             string `json:"reason" gorm:"type:text"`
	Status             string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index:idx_enterprise_policy_group_share_requests_status,priority:2"`
	ApproverUserId     int    `json:"approver_user_id" gorm:"index"`
	DecisionReason     string `json:"decision_reason" gorm:"type:text"`
	DecidedAt          int64  `json:"decided_at" gorm:"index"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterprisePolicyGroupShareRequest) TableName() string {
	return "enterprise_policy_group_share_requests"
}
