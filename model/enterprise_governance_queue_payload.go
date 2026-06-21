package model

const (
	EnterpriseGovernanceQueuePayloadStorageDB = "db"
)

type EnterpriseGovernanceQueuePayload struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	AdmissionId   int64  `json:"admission_id" gorm:"not null;default:0;index"`
	RequestId     string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	EnterpriseId  int    `json:"enterprise_id" gorm:"not null;default:0;index"`
	UserId        int    `json:"user_id" gorm:"not null;default:0;index"`
	TokenId       int    `json:"token_id" gorm:"not null;default:0;index"`
	ContentType   string `json:"content_type" gorm:"type:varchar(255);not null;default:''"`
	ContentLength int64  `json:"content_length" gorm:"not null;default:0"`
	Body          []byte `json:"-" gorm:"not null"`
	BodyBytes     int64  `json:"body_bytes" gorm:"not null;default:0"`
	SHA256        string `json:"sha256" gorm:"type:varchar(64);not null;default:'';index"`
	StorageKind   string `json:"storage_kind" gorm:"type:varchar(32);not null;default:'db';index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (EnterpriseGovernanceQueuePayload) TableName() string {
	return "enterprise_governance_queue_payloads"
}
