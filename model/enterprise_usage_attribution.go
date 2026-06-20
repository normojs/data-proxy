package model

type EnterpriseUsageAttribution struct {
	Id                 int64  `json:"id" gorm:"primaryKey"`
	RequestId          string `json:"request_id" gorm:"type:varchar(128);not null;index"`
	UserId             int    `json:"user_id" gorm:"index"`
	TokenId            int    `json:"token_id" gorm:"index"`
	EnterpriseId       int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_usage_attributions_created_at,priority:1"`
	OrgUnitId          int    `json:"org_unit_id" gorm:"index"`
	ProjectId          int    `json:"project_id" gorm:"index"`
	PolicyGroupIdsJson string `json:"policy_group_ids_json" gorm:"type:text"`
	PolicyIdsJson      string `json:"policy_ids_json" gorm:"type:text"`
	ModelName          string `json:"model_name" gorm:"type:varchar(128);index"`
	ChannelId          int    `json:"channel_id" gorm:"index"`
	PromptTokens       int    `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens   int    `json:"completion_tokens" gorm:"not null;default:0"`
	TotalTokens        int    `json:"total_tokens" gorm:"not null;default:0"`
	Quota              int    `json:"quota" gorm:"not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);not null;default:'';index"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_usage_attributions_created_at,priority:2"`
}

func (EnterpriseUsageAttribution) TableName() string {
	return "enterprise_usage_attributions"
}
