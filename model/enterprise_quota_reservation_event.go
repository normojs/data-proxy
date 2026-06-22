package model

const (
	EnterpriseQuotaReservationStatusReserved    = "reserved"
	EnterpriseQuotaReservationStatusSettled     = "settled"
	EnterpriseQuotaReservationStatusRefunded    = "refunded"
	EnterpriseQuotaReservationStatusCompensated = "compensated"
	EnterpriseQuotaReservationStatusFailed      = "failed"
)

type EnterpriseQuotaReservationEvent struct {
	Id                     int    `json:"id" gorm:"primaryKey"`
	EnterpriseId           int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_quota_reservation_events_status,priority:1"`
	RequestId              string `json:"request_id" gorm:"type:varchar(128);index"`
	UserId                 int    `json:"user_id" gorm:"not null;index"`
	PolicyId               int    `json:"policy_id" gorm:"not null;index"`
	CounterId              int    `json:"counter_id" gorm:"not null;index"`
	Metric                 string `json:"metric" gorm:"type:varchar(32);not null;index"`
	ReservedValue          int64  `json:"reserved_value" gorm:"not null;default:0"`
	ActualValue            int64  `json:"actual_value" gorm:"not null;default:0"`
	Status                 string `json:"status" gorm:"type:varchar(32);not null;default:'reserved';index:idx_enterprise_quota_reservation_events_status,priority:2"`
	RedisCounterKey        string `json:"redis_counter_key" gorm:"type:varchar(255)"`
	RedisCounterTTLSeconds int64  `json:"redis_counter_ttl_seconds" gorm:"not null;default:0"`
	ErrorMessage           string `json:"error_message" gorm:"type:text"`
	ReservedAt             int64  `json:"reserved_at" gorm:"not null;index"`
	SettledAt              int64  `json:"settled_at" gorm:"index"`
	RefundedAt             int64  `json:"refunded_at" gorm:"index"`
	CreatedAt              int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt              int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseQuotaReservationEvent) TableName() string {
	return "enterprise_quota_reservation_events"
}
