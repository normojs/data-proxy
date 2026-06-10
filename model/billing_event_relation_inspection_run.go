package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	BillingEventRelationInspectionTriggerManual    = "manual"
	BillingEventRelationInspectionTriggerScheduled = "scheduled"

	BillingEventRelationInspectionStatusRunning = "running"
	BillingEventRelationInspectionStatusSuccess = "success"
	BillingEventRelationInspectionStatusBlocked = "blocked"
	BillingEventRelationInspectionStatusFailed  = "failed"
)

type BillingEventRelationInspectionRun struct {
	Id int64 `json:"id"`

	Trigger string `json:"trigger" gorm:"type:varchar(32);not null;index"`
	Status  string `json:"status" gorm:"type:varchar(32);not null;index"`
	Message string `json:"message" gorm:"type:varchar(1024);default:''"`

	Limit              int   `json:"limit" gorm:"not null;default:0"`
	Cursor             int64 `json:"cursor" gorm:"bigint;not null;default:0;index"`
	NextCursor         int64 `json:"next_cursor" gorm:"bigint;not null;default:0;index"`
	AutoBackfill       bool  `json:"auto_backfill" gorm:"not null;default:false"`
	AutoCleanupOrphans bool  `json:"auto_cleanup_orphans" gorm:"not null;default:false"`

	MaxAutoBackfill       int `json:"max_auto_backfill" gorm:"not null;default:0"`
	MaxAutoCleanupOrphans int `json:"max_auto_cleanup_orphans" gorm:"not null;default:0"`

	ScannedAuditEvents    int `json:"scanned_audit_events" gorm:"not null;default:0"`
	MissingRelations      int `json:"missing_relations" gorm:"not null;default:0"`
	InvalidAuditEvents    int `json:"invalid_audit_events" gorm:"not null;default:0"`
	OrphanSourceRelations int `json:"orphan_source_relations" gorm:"not null;default:0"`
	OrphanTargetRelations int `json:"orphan_target_relations" gorm:"not null;default:0"`

	BackfillCreated        int  `json:"backfill_created" gorm:"not null;default:0"`
	BackfillWouldCreate    int  `json:"backfill_would_create" gorm:"not null;default:0"`
	BackfillSkippedInvalid int  `json:"backfill_skipped_invalid" gorm:"not null;default:0"`
	BackfillErrorCount     int  `json:"backfill_error_count" gorm:"not null;default:0"`
	BackfillBlocked        bool `json:"backfill_blocked" gorm:"not null;default:false"`

	CleanupDeleted     int  `json:"cleanup_deleted" gorm:"not null;default:0"`
	CleanupWouldDelete int  `json:"cleanup_would_delete" gorm:"not null;default:0"`
	CleanupBlocked     bool `json:"cleanup_blocked" gorm:"not null;default:false"`

	ResultJson string `json:"result_json" gorm:"type:text"`

	StartedAt  int64 `json:"started_at" gorm:"bigint;index"`
	FinishedAt int64 `json:"finished_at" gorm:"bigint;index"`
	CreatedAt  int64 `json:"created_at" gorm:"bigint;index"`
}

func (BillingEventRelationInspectionRun) TableName() string {
	return "billing_event_relation_inspection_runs"
}

func (run *BillingEventRelationInspectionRun) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if run.CreatedAt == 0 {
		run.CreatedAt = now
	}
	if run.StartedAt == 0 {
		run.StartedAt = now
	}
	if run.Status == "" {
		run.Status = BillingEventRelationInspectionStatusRunning
	}
	return nil
}

func CreateBillingEventRelationInspectionRun(run *BillingEventRelationInspectionRun) error {
	return DB.Create(run).Error
}

func UpdateBillingEventRelationInspectionRun(run *BillingEventRelationInspectionRun) error {
	return DB.Save(run).Error
}

func ListBillingEventRelationInspectionRuns(limit int) ([]BillingEventRelationInspectionRun, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var runs []BillingEventRelationInspectionRun
	err := DB.Order("started_at desc, id desc").Limit(limit).Find(&runs).Error
	return runs, err
}

func ListBillingEventRelationInspectionRunsPage(offset int, limit int) ([]BillingEventRelationInspectionRun, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	var total int64
	if err := DB.Model(&BillingEventRelationInspectionRun{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var runs []BillingEventRelationInspectionRun
	err := DB.Model(&BillingEventRelationInspectionRun{}).Order("started_at desc, id desc").Offset(offset).Limit(limit).Find(&runs).Error
	return runs, total, err
}
