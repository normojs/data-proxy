package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	ConnectedAppRequestStatusPending  = "pending"
	ConnectedAppRequestStatusApproved = "approved"
	ConnectedAppRequestStatusRejected = "rejected"

	ConnectedAppAuditTargetRequest = "connected_app_request"
	ConnectedAppAuditTargetApp     = "connected_app"

	ConnectedAppAuditActionSubmit  = "connected_app_request.submit"
	ConnectedAppAuditActionApprove = "connected_app_request.approve"
	ConnectedAppAuditActionReject  = "connected_app_request.reject"
)

type ConnectedAppRequest struct {
	Id                int    `json:"id" gorm:"primaryKey"`
	ApplicantUserId   int    `json:"applicant_user_id" gorm:"not null;index"`
	AppId             int    `json:"app_id" gorm:"not null;default:0;index"`
	Slug              string `json:"slug" gorm:"type:varchar(64);not null;index"`
	Name              string `json:"name" gorm:"type:varchar(128);not null"`
	Description       string `json:"description" gorm:"type:varchar(512);not null;default:''"`
	RequestedScopes   string `json:"requested_scopes" gorm:"type:varchar(512);not null;default:''"`
	DefaultScopes     string `json:"default_scopes" gorm:"type:varchar(512);not null;default:''"`
	AuthorizationFlow string `json:"authorization_flow" gorm:"type:varchar(32);not null;default:'device_code'"`
	HomepageURL       string `json:"homepage_url" gorm:"type:varchar(512);not null;default:''"`
	CallbackURL       string `json:"callback_url" gorm:"type:varchar(512);not null;default:''"`
	Reason            string `json:"reason" gorm:"type:text"`
	Status            string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ReviewerUserId    int    `json:"reviewer_user_id" gorm:"not null;default:0;index"`
	ReviewNote        string `json:"review_note" gorm:"type:text"`
	ReviewedAt        int64  `json:"reviewed_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ConnectedAppRequest) TableName() string {
	return "connected_app_requests"
}

func (request *ConnectedAppRequest) ScopeList() []string {
	return splitConnectedAppScopes(request.RequestedScopes)
}

func (request *ConnectedAppRequest) DefaultScopeList() []string {
	return splitConnectedAppScopes(request.DefaultScopes)
}

type ConnectedAppAuditLog struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	ActorUserId int    `json:"actor_user_id" gorm:"index"`
	Action      string `json:"action" gorm:"type:varchar(64);not null;index"`
	TargetType  string `json:"target_type" gorm:"type:varchar(64);not null;index:idx_connected_app_audit_target,priority:1"`
	TargetId    int    `json:"target_id" gorm:"index:idx_connected_app_audit_target,priority:2"`
	BeforeJson  string `json:"before_json" gorm:"type:text"`
	AfterJson   string `json:"after_json" gorm:"type:text"`
	RequestId   string `json:"request_id" gorm:"type:varchar(128);index"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (ConnectedAppAuditLog) TableName() string {
	return "connected_app_audit_logs"
}

type ConnectedAppAuditInput struct {
	ActorUserId int
	Action      string
	TargetType  string
	TargetId    int
	Before      any
	After       any
	RequestId   string
}

func RecordConnectedAppAuditLog(input ConnectedAppAuditInput) error {
	_, err := RecordConnectedAppAuditLogWithDB(DB, input)
	return err
}

func RecordConnectedAppAuditLogWithDB(db *gorm.DB, input ConnectedAppAuditInput) (ConnectedAppAuditLog, error) {
	beforeJson, err := marshalConnectedAppAuditValue(input.Before)
	if err != nil {
		return ConnectedAppAuditLog{}, err
	}
	afterJson, err := marshalConnectedAppAuditValue(input.After)
	if err != nil {
		return ConnectedAppAuditLog{}, err
	}
	log := ConnectedAppAuditLog{
		ActorUserId: input.ActorUserId,
		Action:      strings.TrimSpace(input.Action),
		TargetType:  strings.TrimSpace(input.TargetType),
		TargetId:    input.TargetId,
		BeforeJson:  beforeJson,
		AfterJson:   afterJson,
		RequestId:   strings.TrimSpace(input.RequestId),
	}
	return log, db.Create(&log).Error
}

func marshalConnectedAppAuditValue(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
