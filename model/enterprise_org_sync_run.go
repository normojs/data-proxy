package model

const (
	EnterpriseOrgSyncRunStatusApplied    = "applied"
	EnterpriseOrgSyncRunStatusRolledBack = "rolled_back"
)

type EnterpriseOrgSyncRun struct {
	Id                  int64  `json:"id" gorm:"primaryKey"`
	EnterpriseId        int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_org_sync_runs_created_at,priority:1;uniqueIndex:idx_enterprise_org_sync_runs_batch,priority:1"`
	BatchId             string `json:"batch_id" gorm:"type:varchar(96);not null;uniqueIndex:idx_enterprise_org_sync_runs_batch,priority:2"`
	Provider            string `json:"provider" gorm:"type:varchar(64);not null;index"`
	SnapshotAt          int64  `json:"snapshot_at" gorm:"index"`
	Status              string `json:"status" gorm:"type:varchar(32);not null;index"`
	SummaryJson         string `json:"summary_json" gorm:"type:text"`
	OperationsJson      string `json:"operations_json" gorm:"type:text"`
	AppliedByUserId     int    `json:"applied_by_user_id" gorm:"index"`
	AppliedAt           int64  `json:"applied_at" gorm:"index"`
	RolledBackByUserId  int    `json:"rolled_back_by_user_id" gorm:"index"`
	RolledBackAt        int64  `json:"rolled_back_at" gorm:"index"`
	RollbackSummaryJson string `json:"rollback_summary_json" gorm:"type:text"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_org_sync_runs_created_at,priority:2"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseOrgSyncRun) TableName() string {
	return "enterprise_org_sync_runs"
}
