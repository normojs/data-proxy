package model

const (
	EnterpriseNotificationOutboxStatusPending         = "pending"
	EnterpriseNotificationOutboxStatusProcessing      = "processing"
	EnterpriseNotificationOutboxStatusSent            = "sent"
	EnterpriseNotificationOutboxStatusFailed          = "failed"
	EnterpriseNotificationOutboxStatusPermanentFailed = "permanent_failed"

	EnterpriseNotificationOutboxChannelInApp   = "in_app"
	EnterpriseNotificationOutboxChannelEmail   = "email"
	EnterpriseNotificationOutboxChannelWebhook = "webhook"
)

type EnterpriseNotificationOutbox struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	EventKey        string `json:"event_key" gorm:"size:191;not null;uniqueIndex"`
	EventType       string `json:"event_type" gorm:"size:96;not null;index"`
	EnterpriseId    int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_notification_outbox_status,priority:1"`
	RecipientUserId int    `json:"recipient_user_id" gorm:"index"`
	RecipientEmail  string `json:"recipient_email" gorm:"size:191;index"`
	Channel         string `json:"channel" gorm:"size:32;not null;index:idx_enterprise_notification_outbox_status,priority:2"`
	TargetType      string `json:"target_type" gorm:"size:64;not null;index:idx_enterprise_notification_outbox_target,priority:1"`
	TargetId        int    `json:"target_id" gorm:"not null;index:idx_enterprise_notification_outbox_target,priority:2"`
	PayloadJson     string `json:"payload_json" gorm:"type:text;not null"`
	Status          string `json:"status" gorm:"size:32;not null;default:'pending';index:idx_enterprise_notification_outbox_status,priority:3"`
	RetryCount      int    `json:"retry_count" gorm:"not null;default:0"`
	NextRetryAt     int64  `json:"next_retry_at" gorm:"index"`
	LastError       string `json:"last_error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseNotificationOutbox) TableName() string {
	return "enterprise_notification_outbox"
}
