package model

const (
	EnterpriseGovernanceAnomalyProtectionStatusActive  = "active"
	EnterpriseGovernanceAnomalyProtectionStatusExpired = "expired"
)

type EnterpriseGovernanceAnomalyProtection struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	EnterpriseId   int    `json:"enterprise_id" gorm:"not null;index:idx_enterprise_governance_anomaly_protections_enterprise,priority:1"`
	ProtectionKey  string `json:"protection_key" gorm:"type:varchar(128);not null;index:idx_enterprise_governance_anomaly_protections_key,priority:1"`
	Reason         string `json:"reason" gorm:"type:varchar(64);not null;index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;index:idx_enterprise_governance_anomaly_protections_key,priority:2"`
	DetectedAt     int64  `json:"detected_at" gorm:"not null;index"`
	ProtectedUntil int64  `json:"protected_until" gorm:"not null;index:idx_enterprise_governance_anomaly_protections_key,priority:3"`
	PayloadJson    string `json:"payload_json" gorm:"type:text;not null"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_governance_anomaly_protections_enterprise,priority:2"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseGovernanceAnomalyProtection) TableName() string {
	return "enterprise_governance_anomaly_protections"
}
