package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ViolationFeeRecord struct {
	Id int64 `json:"id"`

	SourceId string `json:"source_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_violation_fee_records_source_phase,priority:1"`
	Phase    string `json:"phase" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_violation_fee_records_source_phase,priority:2"`

	UserId  int    `json:"user_id" gorm:"not null;index"`
	TokenId int    `json:"token_id" gorm:"index"`
	Group   string `json:"group" gorm:"type:varchar(64);default:'';index"`

	BillingSource string `json:"billing_source" gorm:"type:varchar(64);default:'';index"`
	PriceUnit     string `json:"price_unit" gorm:"type:varchar(32);default:''"`
	EventType     string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	AmountQuota   int    `json:"amount_quota" gorm:"not null;default:0"`
	QuotaDelta    int    `json:"quota_delta" gorm:"not null;default:0"`

	RequestId string `json:"request_id" gorm:"type:varchar(128);default:'';index"`
	Metadata  string `json:"metadata" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

func (ViolationFeeRecord) TableName() string {
	return "violation_fee_records"
}

func (record *ViolationFeeRecord) BeforeCreate(tx *gorm.DB) error {
	record.SourceId = modelBillingEventSourceId(record.SourceId)
	if strings.TrimSpace(record.RequestId) != "" {
		record.RequestId = modelBillingEventSourceId(record.RequestId)
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func CreateViolationFeeRecordIfNotExists(tx *gorm.DB, record *ViolationFeeRecord) (bool, error) {
	if tx == nil {
		tx = DB
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ListViolationFeeRecordsAfterId(lastId int64, limit int) ([]ViolationFeeRecord, error) {
	items := make([]ViolationFeeRecord, 0, limit)
	err := DB.Where("id > ?", lastId).
		Order("id asc").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func ViolationFeeRecordsCountAfterId(lastId int64) (int64, error) {
	var count int64
	err := DB.Model(&ViolationFeeRecord{}).Where("id > ?", lastId).Limit(1).Count(&count).Error
	return count, err
}

func GetViolationFeeRecordBySourcePhase(tx *gorm.DB, sourceId string, phase string) (ViolationFeeRecord, bool, error) {
	if tx == nil {
		tx = DB
	}
	var record ViolationFeeRecord
	sourceId = strings.TrimSpace(sourceId)
	phase = strings.TrimSpace(phase)
	if sourceId == "" || phase == "" {
		return record, false, nil
	}
	err := tx.Where("source_id = ? AND phase = ?", modelBillingEventSourceId(sourceId), phase).First(&record).Error
	if err == nil {
		return record, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return record, false, nil
	}
	return record, false, err
}
