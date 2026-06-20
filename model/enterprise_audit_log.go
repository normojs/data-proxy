package model

import "github.com/QuantumNous/new-api/common"

import "gorm.io/gorm"

type EnterpriseAuditLog struct {
	Id           int64  `json:"id" gorm:"primaryKey"`
	EnterpriseId int    `json:"enterprise_id" gorm:"index:idx_enterprise_audit_logs_created_at,priority:1"`
	ActorUserId  int    `json:"actor_user_id" gorm:"index"`
	Action       string `json:"action" gorm:"type:varchar(64);not null;index"`
	TargetType   string `json:"target_type" gorm:"type:varchar(64);not null;index:idx_enterprise_audit_logs_target,priority:1"`
	TargetId     int    `json:"target_id" gorm:"index:idx_enterprise_audit_logs_target,priority:2"`
	BeforeJson   string `json:"before_json" gorm:"type:text"`
	AfterJson    string `json:"after_json" gorm:"type:text"`
	RequestId    string `json:"request_id" gorm:"type:varchar(128);index"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_audit_logs_created_at,priority:2"`
}

func (EnterpriseAuditLog) TableName() string {
	return "enterprise_audit_logs"
}

type EnterpriseAuditInput struct {
	EnterpriseId int
	ActorUserId  int
	Action       string
	TargetType   string
	TargetId     int
	Before       any
	After        any
	RequestId    string
}

func RecordEnterpriseAuditLog(input EnterpriseAuditInput) error {
	_, err := RecordEnterpriseAuditLogWithDB(DB, input)
	return err
}

func RecordEnterpriseAuditLogWithDB(db *gorm.DB, input EnterpriseAuditInput) (EnterpriseAuditLog, error) {
	beforeJson, err := marshalEnterpriseAuditValue(input.Before)
	if err != nil {
		return EnterpriseAuditLog{}, err
	}
	afterJson, err := marshalEnterpriseAuditValue(input.After)
	if err != nil {
		return EnterpriseAuditLog{}, err
	}
	log := EnterpriseAuditLog{
		EnterpriseId: input.EnterpriseId,
		ActorUserId:  input.ActorUserId,
		Action:       input.Action,
		TargetType:   input.TargetType,
		TargetId:     input.TargetId,
		BeforeJson:   beforeJson,
		AfterJson:    afterJson,
		RequestId:    input.RequestId,
	}
	return log, db.Create(&log).Error
}

func marshalEnterpriseAuditValue(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
