package model

const (
	EnterpriseGovernanceQueueAdmissionStatusQueued   = "queued"
	EnterpriseGovernanceQueueAdmissionStatusAdmitted = "admitted"
	EnterpriseGovernanceQueueAdmissionStatusReleased = "released"
	EnterpriseGovernanceQueueAdmissionStatusTimeout  = "timeout"
	EnterpriseGovernanceQueueAdmissionStatusCanceled = "canceled"
)

type EnterpriseGovernanceQueueAdmission struct {
	Id                 int64  `json:"id" gorm:"primaryKey"`
	RequestId          string `json:"request_id" gorm:"type:varchar(128);not null;index"`
	EnterpriseId       int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_governance_queue_admissions_created_at,priority:1"`
	UserId             int    `json:"user_id" gorm:"index"`
	TokenId            int    `json:"token_id" gorm:"index"`
	OrgUnitId          int    `json:"org_unit_id" gorm:"index"`
	ProjectId          int    `json:"project_id" gorm:"index"`
	PolicyId           int    `json:"policy_id" gorm:"index"`
	PolicyIdsJson      string `json:"policy_ids_json" gorm:"type:text"`
	PolicyGroupIdsJson string `json:"policy_group_ids_json" gorm:"type:text"`
	ModelName          string `json:"model_name" gorm:"type:varchar(128);index"`
	ChannelId          int    `json:"channel_id" gorm:"index"`
	RelayMode          int    `json:"relay_mode" gorm:"index"`
	QueueKey           string `json:"queue_key" gorm:"type:varchar(128);not null;index"`
	Status             string `json:"status" gorm:"type:varchar(32);not null;index"`
	WaitMs             int64  `json:"wait_ms" gorm:"not null;default:0"`
	TimeoutMs          int64  `json:"timeout_ms" gorm:"not null;default:0"`
	AdmittedAt         int64  `json:"admitted_at" gorm:"not null;default:0;index"`
	ReleasedAt         int64  `json:"released_at" gorm:"not null;default:0;index"`
	CanceledAt         int64  `json:"canceled_at" gorm:"not null;default:0;index"`
	RunMs              int64  `json:"run_ms" gorm:"not null;default:0"`
	DryRun             bool   `json:"dry_run" gorm:"not null;default:false"`
	PolicyActionsJson  string `json:"policy_actions_json" gorm:"type:text"`
	UserMessageKey     string `json:"user_message_key" gorm:"type:varchar(128);not null;default:''"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_governance_queue_admissions_created_at,priority:2"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime;index"`
}

func (EnterpriseGovernanceQueueAdmission) TableName() string {
	return "enterprise_governance_queue_admissions"
}
