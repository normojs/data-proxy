package model

import (
	"errors"

	"gorm.io/gorm"
)

const (
	EnterpriseStatusEnabled  = 1
	EnterpriseStatusDisabled = 2

	DefaultEnterpriseSlug     = "default"
	DefaultEnterpriseName     = "Default Enterprise"
	DefaultEnterpriseTimezone = "Asia/Shanghai"
)

type Enterprise struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Name      string `json:"name" gorm:"type:varchar(128);not null"`
	Slug      string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex"`
	Status    int    `json:"status" gorm:"not null;default:1;index"`
	Timezone  string `json:"timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Enterprise) TableName() string {
	return "enterprises"
}

func EnsureDefaultEnterprise() error {
	_, err := GetDefaultEnterprise()
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	enterprise := Enterprise{
		Name:     DefaultEnterpriseName,
		Slug:     DefaultEnterpriseSlug,
		Status:   EnterpriseStatusEnabled,
		Timezone: DefaultEnterpriseTimezone,
	}
	return DB.Create(&enterprise).Error
}

func GetDefaultEnterprise() (*Enterprise, error) {
	var enterprise Enterprise
	err := DB.Where("slug = ?", DefaultEnterpriseSlug).First(&enterprise).Error
	if err != nil {
		return nil, err
	}
	return &enterprise, nil
}
