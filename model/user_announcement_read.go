package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

type UserAnnouncementRead struct {
	ID              uint   `json:"id" gorm:"primaryKey"`
	UserID          int    `json:"user_id" gorm:"index:idx_user_announcement_read,unique;not null"`
	AnnouncementKey string `json:"announcement_key" gorm:"size:128;index:idx_user_announcement_read,unique;not null"`
	ReadAt          int64  `json:"read_at" gorm:"not null;default:0"`
}

func ListUserAnnouncementReadKeys(userID int) ([]string, error) {
	if userID <= 0 {
		return []string{}, nil
	}
	var rows []UserAnnouncementRead
	if err := DB.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.AnnouncementKey != "" {
			keys = append(keys, row.AnnouncementKey)
		}
	}
	return keys, nil
}

func MarkUserAnnouncementsRead(userID int, keys []string) ([]string, error) {
	if userID <= 0 || len(keys) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(keys))
	rows := make([]UserAnnouncementRead, 0, len(keys))
	readAt := common.GetTimestamp()
	for _, key := range keys {
		normalized := strings.TrimSpace(key)
		if normalized == "" || len(normalized) > 128 {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		rows = append(rows, UserAnnouncementRead{
			UserID:          userID,
			AnnouncementKey: normalized,
			ReadAt:          readAt,
		})
	}
	if len(rows) == 0 {
		return []string{}, nil
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "announcement_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"read_at"}),
	}).Create(&rows).Error; err != nil {
		return nil, err
	}
	readKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		readKeys = append(readKeys, row.AnnouncementKey)
	}
	return readKeys, nil
}
