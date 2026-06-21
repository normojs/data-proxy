package model

const (
	EnterpriseGovernanceSharedPoolBorrowStatusReserved = "reserved"
	EnterpriseGovernanceSharedPoolBorrowStatusSettled  = "settled"
	EnterpriseGovernanceSharedPoolBorrowStatusRefunded = "refunded"
)

type EnterpriseGovernanceSharedPool struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	EnterpriseId  int    `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_enterprise_governance_shared_pools_scope,priority:1;index"`
	PolicyId      int    `json:"policy_id" gorm:"not null;uniqueIndex:idx_enterprise_governance_shared_pools_scope,priority:2;index"`
	Metric        string `json:"metric" gorm:"type:varchar(32);not null;uniqueIndex:idx_enterprise_governance_shared_pools_scope,priority:3;index"`
	PeriodStart   int64  `json:"period_start" gorm:"not null;uniqueIndex:idx_enterprise_governance_shared_pools_scope,priority:4;index"`
	PeriodEnd     int64  `json:"period_end" gorm:"not null;index"`
	CapacityValue int64  `json:"capacity_value" gorm:"not null;default:0"`
	UsedValue     int64  `json:"used_value" gorm:"not null;default:0"`
	ReservedValue int64  `json:"reserved_value" gorm:"not null;default:0"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseGovernanceSharedPool) TableName() string {
	return "enterprise_governance_shared_pools"
}

type EnterpriseGovernanceSharedPoolBorrow struct {
	Id                    int64  `json:"id" gorm:"primaryKey"`
	RequestId             string `json:"request_id" gorm:"type:varchar(128);not null;index"`
	PoolId                int64  `json:"pool_id" gorm:"not null;index"`
	EnterpriseId          int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_governance_shared_pool_borrows_created_at,priority:1"`
	UserId                int    `json:"user_id" gorm:"index"`
	TokenId               int    `json:"token_id" gorm:"index"`
	OrgUnitId             int    `json:"org_unit_id" gorm:"index"`
	ProjectId             int    `json:"project_id" gorm:"index"`
	PolicyId              int    `json:"policy_id" gorm:"not null;index"`
	PolicyGroupIdsJson    string `json:"policy_group_ids_json" gorm:"type:text"`
	ModelName             string `json:"model_name" gorm:"type:varchar(128);index"`
	ChannelId             int    `json:"channel_id" gorm:"index"`
	RelayMode             int    `json:"relay_mode" gorm:"index"`
	Metric                string `json:"metric" gorm:"type:varchar(32);not null;index"`
	CapacityValue         int64  `json:"capacity_value" gorm:"not null;default:0"`
	ReservedBorrowedValue int64  `json:"reserved_borrowed_value" gorm:"not null;default:0"`
	SettledBorrowedValue  int64  `json:"settled_borrowed_value" gorm:"not null;default:0"`
	ReturnedValue         int64  `json:"returned_value" gorm:"not null;default:0"`
	PeriodStart           int64  `json:"period_start" gorm:"not null;index"`
	PeriodEnd             int64  `json:"period_end" gorm:"not null;index"`
	Status                string `json:"status" gorm:"type:varchar(32);not null;index"`
	DryRun                bool   `json:"dry_run" gorm:"not null;default:false"`
	PolicyActionsJson     string `json:"policy_actions_json" gorm:"type:text"`
	UserMessageKey        string `json:"user_message_key" gorm:"type:varchar(128);not null;default:''"`
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_governance_shared_pool_borrows_created_at,priority:2"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseGovernanceSharedPoolBorrow) TableName() string {
	return "enterprise_governance_shared_pool_borrows"
}
