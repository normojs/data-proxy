package model

const (
	EnterpriseWebhookStatusEnabled  = 1
	EnterpriseWebhookStatusDisabled = 2
)

type EnterpriseWebhook struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	EnterpriseId   int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_webhooks_status,priority:1"`
	Name           string `json:"name" gorm:"type:varchar(128);not null"`
	Url            string `json:"url" gorm:"type:text;not null"`
	Secret         string `json:"-" gorm:"type:varchar(191)"`
	EventTypesJson string `json:"event_types_json" gorm:"type:text;not null;default:'[]'"`
	Status         int    `json:"status" gorm:"not null;default:1;index:idx_enterprise_webhooks_status,priority:2"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseWebhook) TableName() string {
	return "enterprise_webhooks"
}
