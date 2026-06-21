package model

const (
	ConnectedAppNotificationOutboxStatusPending         = "pending"
	ConnectedAppNotificationOutboxStatusProcessing      = "processing"
	ConnectedAppNotificationOutboxStatusSent            = "sent"
	ConnectedAppNotificationOutboxStatusFailed          = "failed"
	ConnectedAppNotificationOutboxStatusPermanentFailed = "permanent_failed"

	ConnectedAppNotificationOutboxChannelInApp   = "in_app"
	ConnectedAppNotificationOutboxChannelEmail   = "email"
	ConnectedAppNotificationOutboxChannelWebhook = "webhook"

	ConnectedAppNotificationEventDeviceAuthorized = "connected_app_device.authorized"
	ConnectedAppNotificationEventDeviceDenied     = "connected_app_device.denied"
	ConnectedAppNotificationEventDeviceRevoked    = "connected_app_device.revoked"
	ConnectedAppNotificationEventGrantRevoked     = "connected_app_grant.revoked"
	ConnectedAppNotificationEventHealthWarning    = "connected_app.health.warning"
	ConnectedAppNotificationEventTokenRevoked     = "connected_app_token.revoked"
	ConnectedAppNotificationEventTokenRotated     = "connected_app_token.rotated"

	ConnectedAppWebhookStatusEnabled  = 1
	ConnectedAppWebhookStatusDisabled = 2
)

type ConnectedAppNotificationPreference struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	AppId              int    `json:"app_id" gorm:"not null;default:0;uniqueIndex:idx_connected_app_notification_preferences_channel_event,priority:1"`
	Channel            string `json:"channel" gorm:"size:32;not null;uniqueIndex:idx_connected_app_notification_preferences_channel_event,priority:2"`
	EventType          string `json:"event_type" gorm:"size:96;not null;uniqueIndex:idx_connected_app_notification_preferences_channel_event,priority:3"`
	Enabled            bool   `json:"enabled" gorm:"not null;default:false;index"`
	RecipientScopeJson string `json:"recipient_scope_json" gorm:"type:text;not null"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppNotificationPreference) TableName() string {
	return "connected_app_notification_preferences"
}

type ConnectedAppWebhook struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	AppId          int    `json:"app_id" gorm:"not null;default:0;index:idx_connected_app_webhooks_app_status,priority:1"`
	Name           string `json:"name" gorm:"type:varchar(128);not null"`
	Url            string `json:"url" gorm:"type:text;not null"`
	Secret         string `json:"-" gorm:"type:varchar(191)"`
	EventTypesJson string `json:"event_types_json" gorm:"type:text;not null"`
	Status         int    `json:"status" gorm:"not null;default:1;index:idx_connected_app_webhooks_app_status,priority:2"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppWebhook) TableName() string {
	return "connected_app_webhooks"
}

type ConnectedAppNotificationOutbox struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	EventKey        string `json:"event_key" gorm:"size:191;not null;uniqueIndex"`
	EventType       string `json:"event_type" gorm:"size:96;not null;index"`
	AppId           int    `json:"app_id" gorm:"not null;default:0;index:idx_connected_app_notification_outbox_status,priority:1"`
	RecipientUserId int    `json:"recipient_user_id" gorm:"index"`
	RecipientEmail  string `json:"recipient_email" gorm:"size:191;index"`
	Channel         string `json:"channel" gorm:"size:32;not null;index:idx_connected_app_notification_outbox_status,priority:2"`
	TargetType      string `json:"target_type" gorm:"size:64;not null;index:idx_connected_app_notification_outbox_target,priority:1"`
	TargetId        int    `json:"target_id" gorm:"not null;index:idx_connected_app_notification_outbox_target,priority:2"`
	PayloadJson     string `json:"payload_json" gorm:"type:text;not null"`
	Status          string `json:"status" gorm:"size:32;not null;default:'pending';index:idx_connected_app_notification_outbox_status,priority:3"`
	RetryCount      int    `json:"retry_count" gorm:"not null;default:0"`
	NextRetryAt     int64  `json:"next_retry_at" gorm:"index"`
	LastError       string `json:"last_error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppNotificationOutbox) TableName() string {
	return "connected_app_notification_outbox"
}
