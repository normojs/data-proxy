package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RedemptionRewardTypeQuota             = "quota"
	RedemptionRewardTypeModelTokenPackage = "model_token_package"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期

	// RewardType: quota (wallet points) or model_token_package (LLM tokens).
	// Empty means quota for backward compatibility.
	RewardType string `json:"reward_type" gorm:"type:varchar(32);not null;default:'quota'"`
	// Package snapshot fields (used when reward_type = model_token_package)
	PackageModelsJson  string  `json:"package_models_json" gorm:"type:text"`
	PackageTokens      int64   `json:"package_tokens" gorm:"not null;default:0"`
	PackageInputRatio  float64 `json:"package_input_ratio" gorm:"not null;default:1"`
	PackageOutputRatio float64 `json:"package_output_ratio" gorm:"not null;default:1"`
	PackageCacheRatio  float64 `json:"package_cache_ratio" gorm:"not null;default:1"`
	// PackageExpiredAt is absolute package expiry (unix). 0 = never.
	// If 0 and PackageDurationSeconds > 0, expiry = redeem_time + duration.
	PackageExpiredAt       int64 `json:"package_expired_at" gorm:"not null;default:0"`
	PackageDurationSeconds int64 `json:"package_duration_seconds" gorm:"not null;default:0"`
	// ResultPackageId filled after successful package redeem (audit).
	ResultPackageId int `json:"result_package_id" gorm:"not null;default:0"`
	// Non-DB helper for create API
	PackageModels []string `json:"package_models" gorm:"-"`
}

// RedeemResult is returned to the client after a successful redeem.
type RedeemResult struct {
	RewardType  string `json:"reward_type"`
	Quota       int    `json:"quota,omitempty"`
	PackageId   int    `json:"package_id,omitempty"`
	TotalTokens int64  `json:"total_tokens,omitempty"`
	Name        string `json:"name,omitempty"`
}

func (r *Redemption) NormalizedRewardType() string {
	if r == nil {
		return RedemptionRewardTypeQuota
	}
	switch strings.TrimSpace(r.RewardType) {
	case RedemptionRewardTypeModelTokenPackage:
		return RedemptionRewardTypeModelTokenPackage
	default:
		return RedemptionRewardTypeQuota
	}
}

func (r *Redemption) IsPackageReward() bool {
	return r.NormalizedRewardType() == RedemptionRewardTypeModelTokenPackage
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Build query based on keyword type
	query := tx.Model(&Redemption{})

	// Only try to convert to ID if the string represents a valid integer
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (*RedeemResult, error) {
	if key == "" {
		return nil, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return nil, errors.New("无效的 user id")
	}
	redemption := &Redemption{}
	result := &RedeemResult{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}
	common.RandomSleep()
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}

		now := common.GetTimestamp()
		redemption.RedeemedTime = now
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId

		if redemption.IsPackageReward() {
			if redemption.PackageTokens <= 0 {
				return errors.New("兑换码 Token 包配置无效")
			}
			models := ParseModelTokenPackageModels(redemption.PackageModelsJson)
			if len(models) == 0 && strings.TrimSpace(redemption.PackageModelsJson) != "" {
				// keep empty models meaning all models if intentionally empty array
			}
			expiredAt := redemption.PackageExpiredAt
			if expiredAt == 0 && redemption.PackageDurationSeconds > 0 {
				expiredAt = now + redemption.PackageDurationSeconds
			}
			pkg, err := createModelTokenPackageInTx(tx, ModelTokenPackageCreateInput{
				UserId:      userId,
				Name:        redemption.Name,
				Models:      models,
				TotalTokens: redemption.PackageTokens,
				InputRatio:  redemption.PackageInputRatio,
				OutputRatio: redemption.PackageOutputRatio,
				CacheRatio:  redemption.PackageCacheRatio,
				ExpiredAt:   expiredAt,
				Source:      ModelTokenPackageSourceRedeem,
				CreatedBy:   0,
				Remark:      fmt.Sprintf("redemption:%d", redemption.Id),
			})
			if err != nil {
				return err
			}
			redemption.ResultPackageId = pkg.Id
			result.RewardType = RedemptionRewardTypeModelTokenPackage
			result.PackageId = pkg.Id
			result.TotalTokens = pkg.TotalTokens
			result.Name = pkg.Name
			if err := tx.Save(redemption).Error; err != nil {
				return err
			}
			return RecordFundingBillingEvent(tx, FundingBillingEventInput{
				Source:        BillingEventSourceWalletTopUp,
				SourceId:      fmt.Sprintf("redemption:%d", redemption.Id),
				Phase:         "redemption",
				UserId:        userId,
				RequestId:     fmt.Sprintf("redemption:%d", redemption.Id),
				BillingSource: BillingSourceModelTokenPackageLabel,
				PriceUnit:     "model_token_package",
				EventType:     BillingEventTypeCredit,
				AmountQuota:   0,
				Metadata: map[string]any{
					"channel":        "redemption",
					"reward_type":    RedemptionRewardTypeModelTokenPackage,
					"redemption_id":  redemption.Id,
					"package_id":     pkg.Id,
					"name":           redemption.Name,
					"package_tokens": redemption.PackageTokens,
				},
			})
		}

		err = tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		if err != nil {
			return err
		}
		if err := tx.Save(redemption).Error; err != nil {
			return err
		}
		result.RewardType = RedemptionRewardTypeQuota
		result.Quota = redemption.Quota
		result.Name = redemption.Name
		return RecordFundingBillingEvent(tx, FundingBillingEventInput{
			Source:        BillingEventSourceWalletTopUp,
			SourceId:      fmt.Sprintf("redemption:%d", redemption.Id),
			Phase:         "redemption",
			UserId:        userId,
			RequestId:     fmt.Sprintf("redemption:%d", redemption.Id),
			BillingSource: "wallet",
			PriceUnit:     "redemption",
			EventType:     BillingEventTypeCredit,
			AmountQuota:   redemption.Quota,
			Metadata: map[string]any{
				"channel":       "redemption",
				"reward_type":   RedemptionRewardTypeQuota,
				"redemption_id": redemption.Id,
				"name":          redemption.Name,
				"quota":         redemption.Quota,
			},
		})
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		// Surface known user-facing errors without wrapping as generic redeem failed
		if err.Error() == "无效的兑换码" || err.Error() == "该兑换码已被使用" || err.Error() == "该兑换码已过期" || err.Error() == "兑换码 Token 包配置无效" {
			return nil, err
		}
		return nil, ErrRedeemFailed
	}
	if result.RewardType == RedemptionRewardTypeModelTokenPackage {
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码获得模型 Token 包 %s（%d tokens），兑换码ID %d，包ID %d",
			result.Name, result.TotalTokens, redemption.Id, result.PackageId))
	} else {
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	}
	return result, nil
}

// BillingSourceModelTokenPackageLabel is stored in funding billing metadata only.
const BillingSourceModelTokenPackageLabel = "model_token_package"

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select(
		"name", "status", "quota", "redeemed_time", "expired_time",
		"reward_type", "package_models_json", "package_tokens",
		"package_input_ratio", "package_output_ratio", "package_cache_ratio",
		"package_expired_at", "package_duration_seconds",
	).Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
