package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BillingEventRelationTypeReconciliationRepair          = "reconciliation_repair"
	BillingEventRelationTypeReconciliationBackfillMissing = "reconciliation_backfill_missing"
)

type BillingEventRelation struct {
	Id int64 `json:"id"`

	SourceEventId int64  `json:"source_event_id" gorm:"not null;index;uniqueIndex:idx_billing_event_relations_unique,priority:1"`
	TargetEventId int64  `json:"target_event_id" gorm:"not null;index;uniqueIndex:idx_billing_event_relations_unique,priority:2"`
	RelationType  string `json:"relation_type" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_billing_event_relations_unique,priority:3"`

	Reason  string `json:"reason" gorm:"type:varchar(512);default:''"`
	Label   string `json:"label" gorm:"type:varchar(256);default:'';index"`
	AdminId int    `json:"admin_id" gorm:"not null;default:0;index"`

	CreatedAt int64 `json:"created_at" gorm:"bigint;index"`
}

func (BillingEventRelation) TableName() string {
	return "billing_event_relations"
}

func (relation *BillingEventRelation) BeforeCreate(tx *gorm.DB) error {
	if relation.CreatedAt == 0 {
		relation.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func CreateBillingEventRelationIfNotExists(tx *gorm.DB, relation *BillingEventRelation) (bool, error) {
	if tx == nil {
		tx = DB
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(relation)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ListBillingEventRelationsByTargetIds(ids []int64) ([]BillingEventRelation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var relations []BillingEventRelation
	err := DB.Where("target_event_id IN ?", ids).
		Order("created_at desc, id desc").
		Find(&relations).Error
	return relations, err
}

func ListBillingEventRelationsBySourceIds(ids []int64) ([]BillingEventRelation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var relations []BillingEventRelation
	err := DB.Where("source_event_id IN ?", ids).
		Order("created_at desc, id desc").
		Find(&relations).Error
	return relations, err
}

func BillingEventRelationExists(tx *gorm.DB, sourceEventId int64, targetEventId int64, relationType string) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if sourceEventId <= 0 || targetEventId <= 0 || relationType == "" {
		return false, nil
	}
	var count int64
	err := tx.Model(&BillingEventRelation{}).
		Where("source_event_id = ? AND target_event_id = ? AND relation_type = ?", sourceEventId, targetEventId, relationType).
		Count(&count).Error
	return count > 0, err
}

func CountBillingEventRelations() (int64, error) {
	var count int64
	err := DB.Model(&BillingEventRelation{}).Count(&count).Error
	return count, err
}

func DeleteBillingEventRelationsByIds(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Where("id IN ?", ids).Delete(&BillingEventRelation{})
	return result.RowsAffected, result.Error
}
