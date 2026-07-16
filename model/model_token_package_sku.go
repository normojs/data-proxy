package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ModelTokenPackageSkuStatusEnabled  = "enabled"
	ModelTokenPackageSkuStatusDisabled = "disabled"
)

var (
	ErrModelTokenPackageSkuNotFound      = errors.New("model token package sku not found")
	ErrModelTokenPackageSkuDisabled      = errors.New("model token package sku is disabled")
	ErrModelTokenPackageSkuInvalid       = errors.New("model token package sku is invalid")
	ErrModelTokenPackageSkuPurchaseFail  = errors.New("model token package sku purchase failed")
	ErrModelTokenPackageSkuInsufficientQ = errors.New("wallet quota is insufficient for this package sku")
)

// ModelTokenPackageSku is a sellable catalog item that grants a model token package.
type ModelTokenPackageSku struct {
	Id              int      `json:"id" gorm:"primaryKey"`
	Name            string   `json:"name" gorm:"type:varchar(128);not null;default:''"`
	Description     string   `json:"description" gorm:"type:text"`
	ModelsJson      string   `json:"models_json" gorm:"type:text;not null"`
	TotalTokens     int64    `json:"total_tokens" gorm:"not null;default:0"`
	InputRatio      float64  `json:"input_ratio" gorm:"not null;default:1"`
	OutputRatio     float64  `json:"output_ratio" gorm:"not null;default:1"`
	CacheRatio      float64  `json:"cache_ratio" gorm:"not null;default:1"`
	Priority        int      `json:"priority" gorm:"not null;default:0"`
	DurationSeconds int64    `json:"duration_seconds" gorm:"not null;default:0"` // 0 = never expires after purchase
	PriceQuota      int      `json:"price_quota" gorm:"not null;default:0"`      // wallet quota points
	Status          string   `json:"status" gorm:"type:varchar(32);not null;default:enabled;index"`
	SortOrder       int      `json:"sort_order" gorm:"not null;default:0;index"`
	CreatedBy       int      `json:"created_by" gorm:"not null;default:0"`
	CreatedAt       int64    `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64    `json:"updated_at" gorm:"autoUpdateTime"`
	Models          []string `json:"models" gorm:"-"`
}

func (ModelTokenPackageSku) TableName() string {
	return "model_token_package_skus"
}

func (s *ModelTokenPackageSku) AfterFind(tx *gorm.DB) error {
	if s == nil {
		return nil
	}
	s.Models = ParseModelTokenPackageModels(s.ModelsJson)
	return nil
}

func (s *ModelTokenPackageSku) IsEnabled() bool {
	return s != nil && strings.TrimSpace(s.Status) == ModelTokenPackageSkuStatusEnabled
}

type ModelTokenPackageSkuInput struct {
	Name            string
	Description     string
	Models          []string
	TotalTokens     int64
	InputRatio      float64
	OutputRatio     float64
	CacheRatio      float64
	Priority        int
	DurationSeconds int64
	PriceQuota      int
	Status          string
	SortOrder       int
	CreatedBy       int
}

func normalizeSkuStatus(status string) string {
	switch strings.TrimSpace(status) {
	case ModelTokenPackageSkuStatusDisabled:
		return ModelTokenPackageSkuStatusDisabled
	default:
		return ModelTokenPackageSkuStatusEnabled
	}
}

func validateSkuInput(input ModelTokenPackageSkuInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrModelTokenPackageSkuInvalid)
	}
	if input.TotalTokens <= 0 {
		return fmt.Errorf("%w: total_tokens must be > 0", ErrModelTokenPackageSkuInvalid)
	}
	if input.PriceQuota < 0 {
		return fmt.Errorf("%w: price_quota must be >= 0", ErrModelTokenPackageSkuInvalid)
	}
	if input.DurationSeconds < 0 {
		return fmt.Errorf("%w: duration_seconds must be >= 0", ErrModelTokenPackageSkuInvalid)
	}
	if _, err := EncodeModelTokenPackageModels(input.Models); err != nil {
		return fmt.Errorf("%w: models are required", ErrModelTokenPackageSkuInvalid)
	}
	return nil
}

func CreateModelTokenPackageSku(input ModelTokenPackageSkuInput) (*ModelTokenPackageSku, error) {
	if err := validateSkuInput(input); err != nil {
		return nil, err
	}
	modelsJSON, err := EncodeModelTokenPackageModels(input.Models)
	if err != nil {
		return nil, err
	}
	sku := &ModelTokenPackageSku{
		Name:            strings.TrimSpace(input.Name),
		Description:     strings.TrimSpace(input.Description),
		ModelsJson:      modelsJSON,
		TotalTokens:     input.TotalTokens,
		InputRatio:      ResolveModelTokenPackageRatio(input.InputRatio),
		OutputRatio:     ResolveModelTokenPackageRatio(input.OutputRatio),
		CacheRatio:      ResolveModelTokenPackageRatio(input.CacheRatio),
		Priority:        input.Priority,
		DurationSeconds: input.DurationSeconds,
		PriceQuota:      input.PriceQuota,
		Status:          normalizeSkuStatus(input.Status),
		SortOrder:       input.SortOrder,
		CreatedBy:       input.CreatedBy,
	}
	if err := DB.Create(sku).Error; err != nil {
		return nil, err
	}
	sku.Models = ParseModelTokenPackageModels(sku.ModelsJson)
	return sku, nil
}

func UpdateModelTokenPackageSku(id int, input ModelTokenPackageSkuInput) (*ModelTokenPackageSku, error) {
	if id <= 0 {
		return nil, ErrModelTokenPackageSkuNotFound
	}
	if err := validateSkuInput(input); err != nil {
		return nil, err
	}
	modelsJSON, err := EncodeModelTokenPackageModels(input.Models)
	if err != nil {
		return nil, err
	}
	sku := &ModelTokenPackageSku{}
	if err := DB.First(sku, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelTokenPackageSkuNotFound
		}
		return nil, err
	}
	sku.Name = strings.TrimSpace(input.Name)
	sku.Description = strings.TrimSpace(input.Description)
	sku.ModelsJson = modelsJSON
	sku.TotalTokens = input.TotalTokens
	sku.InputRatio = ResolveModelTokenPackageRatio(input.InputRatio)
	sku.OutputRatio = ResolveModelTokenPackageRatio(input.OutputRatio)
	sku.CacheRatio = ResolveModelTokenPackageRatio(input.CacheRatio)
	sku.Priority = input.Priority
	sku.DurationSeconds = input.DurationSeconds
	sku.PriceQuota = input.PriceQuota
	sku.Status = normalizeSkuStatus(input.Status)
	sku.SortOrder = input.SortOrder
	if err := DB.Save(sku).Error; err != nil {
		return nil, err
	}
	sku.Models = ParseModelTokenPackageModels(sku.ModelsJson)
	return sku, nil
}

func GetModelTokenPackageSkuById(id int) (*ModelTokenPackageSku, error) {
	if id <= 0 {
		return nil, ErrModelTokenPackageSkuNotFound
	}
	sku := &ModelTokenPackageSku{}
	if err := DB.First(sku, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelTokenPackageSkuNotFound
		}
		return nil, err
	}
	return sku, nil
}

func ListModelTokenPackageSkus(enabledOnly bool) ([]ModelTokenPackageSku, error) {
	query := DB.Model(&ModelTokenPackageSku{}).Order("sort_order asc, id asc")
	if enabledOnly {
		query = query.Where("status = ?", ModelTokenPackageSkuStatusEnabled)
	}
	var rows []ModelTokenPackageSku
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// PurchaseModelTokenPackageSkuWithWallet deducts wallet quota and grants a package.
// Free SKUs (price_quota=0) still create a purchase package for auditability.
func PurchaseModelTokenPackageSkuWithWallet(userId, skuId int) (*ModelTokenPackage, *ModelTokenPackageSku, error) {
	if userId <= 0 || skuId <= 0 {
		return nil, nil, ErrModelTokenPackageSkuPurchaseFail
	}

	var pkg *ModelTokenPackage
	var skuOut *ModelTokenPackageSku
	err := DB.Transaction(func(tx *gorm.DB) error {
		sku := &ModelTokenPackageSku{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(sku, skuId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelTokenPackageSkuNotFound
			}
			return err
		}
		if !sku.IsEnabled() {
			return ErrModelTokenPackageSkuDisabled
		}
		if sku.TotalTokens <= 0 {
			return ErrModelTokenPackageSkuInvalid
		}

		user := &User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "quota").Where("id = ?", userId).First(user).Error; err != nil {
			return err
		}
		if sku.PriceQuota > 0 {
			if user.Quota < sku.PriceQuota {
				return ErrModelTokenPackageSkuInsufficientQ
			}
			result := tx.Model(&User{}).
				Where("id = ? AND quota >= ?", userId, sku.PriceQuota).
				Update("quota", gorm.Expr("quota - ?", sku.PriceQuota))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrModelTokenPackageSkuInsufficientQ
			}
		}

		var expiredAt int64
		if sku.DurationSeconds > 0 {
			expiredAt = common.GetTimestamp() + sku.DurationSeconds
		}
		created, err := createModelTokenPackageInTx(tx, ModelTokenPackageCreateInput{
			UserId:      userId,
			Name:        sku.Name,
			Models:      ParseModelTokenPackageModels(sku.ModelsJson),
			TotalTokens: sku.TotalTokens,
			InputRatio:  sku.InputRatio,
			OutputRatio: sku.OutputRatio,
			CacheRatio:  sku.CacheRatio,
			Priority:    sku.Priority,
			ExpiredAt:   expiredAt,
			Source:      ModelTokenPackageSourcePurchase,
			CreatedBy:   userId,
			Remark:      fmt.Sprintf("sku_id=%d price_quota=%d", sku.Id, sku.PriceQuota),
		})
		if err != nil {
			return err
		}
		if err := RecordModelTokenPackagePurchaseBillingEvent(tx, userId, sku, created); err != nil {
			return err
		}
		pkg = created
		skuOut = sku
		skuOut.Models = ParseModelTokenPackageModels(sku.ModelsJson)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return pkg, skuOut, nil
}
