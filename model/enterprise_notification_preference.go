package model

const (
	EnterpriseNotificationPreferenceChannelEmail   = "email"
	EnterpriseNotificationPreferenceChannelWebhook = "webhook"
)

type EnterpriseNotificationPreference struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	EnterpriseId       int    `json:"enterprise_id" gorm:"not null;uniqueIndex:idx_enterprise_notification_preferences_channel_event,priority:1"`
	Channel            string `json:"channel" gorm:"size:32;not null;uniqueIndex:idx_enterprise_notification_preferences_channel_event,priority:2"`
	EventType          string `json:"event_type" gorm:"size:96;not null;uniqueIndex:idx_enterprise_notification_preferences_channel_event,priority:3"`
	Enabled            bool   `json:"enabled" gorm:"not null;default:false;index"`
	RecipientScopeJson string `json:"recipient_scope_json" gorm:"type:text;not null"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseNotificationPreference) TableName() string {
	return "enterprise_notification_preferences"
}
