package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type EnterpriseAuditLog struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	EnterpriseId   int    `json:"enterprise_id" gorm:"index:idx_enterprise_audit_logs_created_at,priority:1"`
	ActorUserId    int    `json:"actor_user_id" gorm:"index"`
	Action         string `json:"action" gorm:"type:varchar(64);not null;index"`
	TargetType     string `json:"target_type" gorm:"type:varchar(64);not null;index:idx_enterprise_audit_logs_target,priority:1"`
	TargetId       int    `json:"target_id" gorm:"index:idx_enterprise_audit_logs_target,priority:2"`
	ScopeUserId    int    `json:"scope_user_id" gorm:"index"`
	ScopeOrgUnitId int    `json:"scope_org_unit_id" gorm:"index"`
	ScopeProjectId int    `json:"scope_project_id" gorm:"index"`
	BeforeJson     string `json:"before_json" gorm:"type:text"`
	AfterJson      string `json:"after_json" gorm:"type:text"`
	RequestId      string `json:"request_id" gorm:"type:varchar(128);index"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index:idx_enterprise_audit_logs_created_at,priority:2"`
}

func (EnterpriseAuditLog) TableName() string {
	return "enterprise_audit_logs"
}

type EnterpriseAuditInput struct {
	EnterpriseId   int
	ActorUserId    int
	Action         string
	TargetType     string
	TargetId       int
	ScopeUserId    int
	ScopeOrgUnitId int
	ScopeProjectId int
	Before         any
	After          any
	RequestId      string
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
		EnterpriseId:   input.EnterpriseId,
		ActorUserId:    input.ActorUserId,
		Action:         input.Action,
		TargetType:     input.TargetType,
		TargetId:       input.TargetId,
		ScopeUserId:    input.ScopeUserId,
		ScopeOrgUnitId: input.ScopeOrgUnitId,
		ScopeProjectId: input.ScopeProjectId,
		BeforeJson:     beforeJson,
		AfterJson:      afterJson,
		RequestId:      input.RequestId,
	}
	if err := fillEnterpriseAuditScope(db, &log); err != nil {
		return EnterpriseAuditLog{}, err
	}
	return log, db.Create(&log).Error
}

func fillEnterpriseAuditScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if db == nil || log == nil || log.EnterpriseId <= 0 {
		return nil
	}
	if log.ScopeUserId > 0 {
		if err := fillEnterpriseAuditUserOrgScope(db, log); err != nil {
			return err
		}
		return nil
	}
	switch log.TargetType {
	case "user":
		log.ScopeUserId = log.TargetId
		return fillEnterpriseAuditUserOrgScope(db, log)
	case "org_unit":
		log.ScopeOrgUnitId = log.TargetId
	case "project":
		log.ScopeProjectId = log.TargetId
	case "policy_group":
		return fillEnterpriseAuditPolicyGroupScope(db, log, log.TargetId)
	case "policy_group_share_request":
		return fillEnterpriseAuditPolicyGroupShareRequestScope(db, log)
	case "quota_policy":
		return fillEnterpriseAuditQuotaPolicyScope(db, log)
	case "quota_counter":
		return fillEnterpriseAuditQuotaCounterScope(db, log)
	case "quota_request":
		return fillEnterpriseAuditQuotaRequestScope(db, log)
	case "token":
		return fillEnterpriseAuditTokenScope(db, log)
	}
	return nil
}

func fillEnterpriseAuditUserOrgScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.ScopeUserId <= 0 || log.ScopeOrgUnitId > 0 {
		return nil
	}
	var membership EnterpriseOrgMembership
	if err := db.Select("org_unit_id").
		Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", log.EnterpriseId, log.ScopeUserId, true).
		First(&membership).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	log.ScopeOrgUnitId = membership.OrgUnitId
	return nil
}

func fillEnterpriseAuditQuotaPolicyScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.TargetId <= 0 {
		return nil
	}
	var policy EnterpriseQuotaPolicy
	if err := db.Select("target_type, target_id").
		Where("enterprise_id = ? AND id = ?", log.EnterpriseId, log.TargetId).
		First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	return fillEnterpriseAuditPolicyTargetScope(db, log, policy.TargetType, policy.TargetId)
}

func fillEnterpriseAuditQuotaCounterScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.TargetId <= 0 {
		return nil
	}
	var counter EnterpriseQuotaCounter
	if err := db.Select("id, target_type, target_id, policy_id").
		Where("enterprise_id = ? AND id = ?", log.EnterpriseId, log.TargetId).
		First(&counter).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if err := fillEnterpriseAuditPolicyTargetScope(db, log, counter.TargetType, counter.TargetId); err != nil {
		return err
	}
	if log.ScopeUserId == 0 && log.ScopeOrgUnitId == 0 && log.ScopeProjectId == 0 && counter.PolicyId > 0 {
		log.TargetId = counter.PolicyId
		err := fillEnterpriseAuditQuotaPolicyScope(db, log)
		log.TargetId = int(counter.Id)
		return err
	}
	return nil
}

func fillEnterpriseAuditQuotaRequestScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.TargetId <= 0 {
		return nil
	}
	var request EnterpriseQuotaRequest
	if err := db.Select("applicant_user_id, project_id, target_type, target_id").
		Where("enterprise_id = ? AND id = ?", log.EnterpriseId, log.TargetId).
		First(&request).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	log.ScopeUserId = request.ApplicantUserId
	if request.ProjectId > 0 {
		log.ScopeProjectId = request.ProjectId
	}
	if err := fillEnterpriseAuditUserOrgScope(db, log); err != nil {
		return err
	}
	return fillEnterpriseAuditPolicyTargetScope(db, log, request.TargetType, request.TargetId)
}

func fillEnterpriseAuditTokenScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.TargetId <= 0 {
		return nil
	}
	var token Token
	if err := db.Select("user_id, default_project_id").Where("id = ?", log.TargetId).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	log.ScopeUserId = token.UserId
	log.ScopeProjectId = token.DefaultProjectId
	return fillEnterpriseAuditUserOrgScope(db, log)
}

func fillEnterpriseAuditPolicyTargetScope(db *gorm.DB, log *EnterpriseAuditLog, targetType string, targetId int) error {
	switch targetType {
	case PolicyTargetOrgUnit:
		log.ScopeOrgUnitId = targetId
	case PolicyTargetProject:
		log.ScopeProjectId = targetId
	case PolicyTargetPolicyGroup:
		return fillEnterpriseAuditPolicyGroupScope(db, log, targetId)
	case PolicyTargetUser:
		log.ScopeUserId = targetId
		return fillEnterpriseAuditUserOrgScope(db, log)
	}
	return nil
}

func fillEnterpriseAuditPolicyGroupScope(db *gorm.DB, log *EnterpriseAuditLog, groupId int) error {
	if groupId <= 0 || log.ScopeOrgUnitId > 0 {
		return nil
	}
	var group EnterprisePolicyGroup
	if err := db.Select("org_unit_id").
		Where("enterprise_id = ? AND id = ?", log.EnterpriseId, groupId).
		First(&group).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	log.ScopeOrgUnitId = group.OrgUnitId
	return nil
}

func fillEnterpriseAuditPolicyGroupShareRequestScope(db *gorm.DB, log *EnterpriseAuditLog) error {
	if log.TargetId <= 0 || log.ScopeOrgUnitId > 0 {
		return nil
	}
	var request EnterprisePolicyGroupShareRequest
	if err := db.Select("requester_org_unit_id, target_org_unit_id").
		Where("enterprise_id = ? AND id = ?", log.EnterpriseId, log.TargetId).
		First(&request).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if request.RequesterOrgUnitId > 0 {
		log.ScopeOrgUnitId = request.RequesterOrgUnitId
		return nil
	}
	log.ScopeOrgUnitId = request.TargetOrgUnitId
	return nil
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
