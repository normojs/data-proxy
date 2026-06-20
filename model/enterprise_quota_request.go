package model

const (
	EnterpriseQuotaRequestStatusPending   = "pending"
	EnterpriseQuotaRequestStatusApproved  = "approved"
	EnterpriseQuotaRequestStatusRejected  = "rejected"
	EnterpriseQuotaRequestStatusWithdrawn = "withdrawn"
	EnterpriseQuotaRequestStatusExpired   = "expired"
)

type EnterpriseQuotaRequest struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	EnterpriseId    int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_quota_requests_status,priority:1"`
	ApplicantUserId int    `json:"applicant_user_id" gorm:"not null;index"`
	ApproverUserId  int    `json:"approver_user_id" gorm:"index"`
	PolicyId        int    `json:"policy_id" gorm:"not null;index"`
	TargetType      string `json:"target_type" gorm:"type:varchar(32);not null;index:idx_enterprise_quota_requests_target,priority:1"`
	TargetId        int    `json:"target_id" gorm:"not null;index:idx_enterprise_quota_requests_target,priority:2"`
	Metric          string `json:"metric" gorm:"type:varchar(32);not null;index"`
	Period          string `json:"period" gorm:"type:varchar(32);not null;index"`
	LimitDelta      int64  `json:"limit_delta" gorm:"not null"`
	Reason          string `json:"reason" gorm:"type:text"`
	DecisionReason  string `json:"decision_reason" gorm:"type:text"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index:idx_enterprise_quota_requests_status,priority:2"`
	EffectiveAt     int64  `json:"effective_at" gorm:"index"`
	ExpiresAt       int64  `json:"expires_at" gorm:"not null;index"`
	DecidedAt       int64  `json:"decided_at" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseQuotaRequest) TableName() string {
	return "enterprise_quota_requests"
}
