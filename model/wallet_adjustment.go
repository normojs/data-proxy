package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletAdjustment struct {
	Id int64 `json:"id"`

	SourceId string `json:"source_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserId   int    `json:"user_id" gorm:"not null;index"`
	AdminId  int    `json:"admin_id" gorm:"not null;index"`

	Mode       string `json:"mode" gorm:"type:varchar(32);not null;index"`
	EventType  string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Amount     int    `json:"amount" gorm:"not null;default:0"`
	OldQuota   int    `json:"old_quota" gorm:"not null;default:0"`
	NewQuota   int    `json:"new_quota" gorm:"not null;default:0"`
	Metadata   string `json:"metadata" gorm:"type:text"`
	LedgeredAt int64  `json:"ledgered_at" gorm:"bigint;index"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

func (WalletAdjustment) TableName() string {
	return "wallet_adjustments"
}

func (adjustment *WalletAdjustment) BeforeCreate(tx *gorm.DB) error {
	adjustment.SourceId = modelBillingEventSourceId(adjustment.SourceId)
	if adjustment.CreatedAt == 0 {
		adjustment.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func CreateWalletAdjustmentIfNotExists(tx *gorm.DB, adjustment *WalletAdjustment) (bool, error) {
	if tx == nil {
		tx = DB
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(adjustment)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ListWalletAdjustmentsAfterId(lastId int64, limit int) ([]WalletAdjustment, error) {
	items := make([]WalletAdjustment, 0, limit)
	err := DB.Where("id > ?", lastId).
		Order("id asc").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func WalletAdjustmentsCountAfterId(lastId int64) (int64, error) {
	var count int64
	err := DB.Model(&WalletAdjustment{}).Where("id > ?", lastId).Limit(1).Count(&count).Error
	return count, err
}

func GetWalletAdjustmentBySourceId(tx *gorm.DB, sourceId string) (WalletAdjustment, bool, error) {
	if tx == nil {
		tx = DB
	}
	var adjustment WalletAdjustment
	sourceId = strings.TrimSpace(sourceId)
	if sourceId == "" {
		return adjustment, false, nil
	}
	err := tx.Where("source_id = ?", modelBillingEventSourceId(sourceId)).First(&adjustment).Error
	if err == nil {
		return adjustment, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return adjustment, false, nil
	}
	return adjustment, false, err
}
