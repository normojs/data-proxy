package model

const (
	QuotaPolicyStatusEnabled  = 1
	QuotaPolicyStatusDisabled = 2

	PolicyTargetEnterprise  = "enterprise"
	PolicyTargetOrgUnit     = "org_unit"
	PolicyTargetProject     = "project"
	PolicyTargetPolicyGroup = "policy_group"
	PolicyTargetUser        = "user"

	PolicyMetricRequestCount = "request_count"
	PolicyMetricQuota        = "quota"

	PolicyPeriodDay   = "day"
	PolicyPeriodMonth = "month"

	PolicyModelScopeAll      = "all"
	PolicyModelScopeSpecific = "specific"

	PolicyConditionModeStructured = "structured"
	PolicyConditionModeCEL        = "cel"

	PolicyActionReject        = "reject"
	PolicyActionAlert         = "alert"
	PolicyActionFallbackModel = "fallback_model"
	PolicyActionQueue         = "queue"
	PolicyActionSharedPool    = "shared_pool"
)

func IsEnterpriseQuotaPolicyAction(action string) bool {
	switch action {
	case PolicyActionReject,
		PolicyActionAlert,
		PolicyActionFallbackModel,
		PolicyActionQueue,
		PolicyActionSharedPool:
		return true
	default:
		return false
	}
}

func IsEnterpriseQuotaPolicyBlockingAction(action string) bool {
	return action == "" || action == PolicyActionReject
}

type EnterpriseQuotaPolicy struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	EnterpriseId  int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_quota_policies_status,priority:1"`
	Name          string `json:"name" gorm:"type:varchar(128);not null"`
	Description   string `json:"description" gorm:"type:text"`
	TargetType    string `json:"target_type" gorm:"type:varchar(32);not null;index:idx_enterprise_quota_policies_target,priority:1"`
	TargetId      int    `json:"target_id" gorm:"not null;index:idx_enterprise_quota_policies_target,priority:2"`
	Metric        string `json:"metric" gorm:"type:varchar(32);not null;index"`
	Period        string `json:"period" gorm:"type:varchar(32);not null;index"`
	LimitValue    int64  `json:"limit_value" gorm:"not null"`
	Timezone      string `json:"timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	ModelScope    string `json:"model_scope" gorm:"type:varchar(32);not null;default:'all'"`
	ModelsJson    string `json:"models_json" gorm:"type:text"`
	ConditionMode string `json:"condition_mode" gorm:"type:varchar(16);not null;default:'structured'"`
	ConditionJson string `json:"condition_json" gorm:"type:text"`
	ConditionExpr string `json:"condition_expr" gorm:"type:text"`
	ConditionHash string `json:"condition_hash" gorm:"type:varchar(64);index"`
	Action        string `json:"action" gorm:"type:varchar(32);not null;default:'reject'"`
	Priority      int    `json:"priority" gorm:"not null;default:0"`
	Status        int    `json:"status" gorm:"not null;default:1;index:idx_enterprise_quota_policies_status,priority:2"`
	EffectiveAt   int64  `json:"effective_at" gorm:"index"`
	ExpiresAt     int64  `json:"expires_at" gorm:"index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseQuotaPolicy) TableName() string {
	return "enterprise_quota_policies"
}
