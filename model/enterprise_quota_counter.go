package model

type EnterpriseQuotaCounter struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	EnterpriseId  int    `json:"enterprise_id" gorm:"not null;index"`
	PolicyId      int    `json:"policy_id" gorm:"not null;uniqueIndex:idx_enterprise_quota_counters_period,priority:1;index"`
	TargetType    string `json:"target_type" gorm:"type:varchar(32);not null;uniqueIndex:idx_enterprise_quota_counters_period,priority:2"`
	TargetId      int    `json:"target_id" gorm:"not null;uniqueIndex:idx_enterprise_quota_counters_period,priority:3"`
	Metric        string `json:"metric" gorm:"type:varchar(32);not null;uniqueIndex:idx_enterprise_quota_counters_period,priority:4"`
	PeriodStart   int64  `json:"period_start" gorm:"not null;uniqueIndex:idx_enterprise_quota_counters_period,priority:5;index"`
	PeriodEnd     int64  `json:"period_end" gorm:"not null;index"`
	UsedValue     int64  `json:"used_value" gorm:"not null;default:0"`
	ReservedValue int64  `json:"reserved_value" gorm:"not null;default:0"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseQuotaCounter) TableName() string {
	return "enterprise_quota_counters"
}
