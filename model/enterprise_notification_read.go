package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

type EnterpriseNotificationRead struct {
	ID              uint   `json:"id" gorm:"primaryKey"`
	UserID          int    `json:"user_id" gorm:"index:idx_enterprise_notification_read,unique;not null"`
	NotificationKey string `json:"notification_key" gorm:"size:160;index:idx_enterprise_notification_read,unique;not null"`
	ReadAt          int64  `json:"read_at" gorm:"not null;default:0"`
}

func (EnterpriseNotificationRead) TableName() string {
	return "enterprise_notification_reads"
}

func ListEnterpriseNotificationReadKeys(userID int) ([]string, error) {
	if userID <= 0 {
		return []string{}, nil
	}
	var rows []EnterpriseNotificationRead
	if err := DB.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.NotificationKey != "" {
			keys = append(keys, row.NotificationKey)
		}
	}
	return keys, nil
}

func MarkEnterpriseNotificationsRead(userID int, keys []string) ([]string, error) {
	if userID <= 0 || len(keys) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(keys))
	rows := make([]EnterpriseNotificationRead, 0, len(keys))
	readAt := common.GetTimestamp()
	for _, key := range keys {
		normalized := strings.TrimSpace(key)
		if normalized == "" || len(normalized) > 160 {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		rows = append(rows, EnterpriseNotificationRead{
			UserID:          userID,
			NotificationKey: normalized,
			ReadAt:          readAt,
		})
	}
	if len(rows) == 0 {
		return []string{}, nil
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "notification_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"read_at"}),
	}).Create(&rows).Error; err != nil {
		return nil, err
	}
	readKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		readKeys = append(readKeys, row.NotificationKey)
	}
	return readKeys, nil
}
