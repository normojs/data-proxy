package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ModelTokenPackageStatusActive    = "active"
	ModelTokenPackageStatusExhausted = "exhausted"
	ModelTokenPackageStatusExpired   = "expired"
	ModelTokenPackageStatusDisabled  = "disabled"

	ModelTokenPackageSourceAdminGrant = "admin_grant"
	ModelTokenPackageSourcePurchase   = "purchase"
	ModelTokenPackageSourceRedeem     = "redeem"

	ModelTokenPackageLedgerReasonConsume = "consume"
	ModelTokenPackageLedgerReasonRefund  = "refund"
	ModelTokenPackageLedgerReasonGrant   = "grant"
	ModelTokenPackageLedgerReasonAdjust  = "adjust"
	ModelTokenPackageLedgerReasonDisable = "disable"
)

var (
	ErrModelTokenPackageNotFound      = errors.New("model token package not found")
	ErrModelTokenPackageInsufficient  = errors.New("model token package remaining is insufficient")
	ErrModelTokenPackageNotActive     = errors.New("model token package is not active")
	ErrModelTokenPackageInvalidModels = errors.New("model token package models are invalid")
	ErrModelTokenPackageInvalidAmount = errors.New("model token package token amount is invalid")
)

// ModelTokenPackage stores a user-owned model token usage balance (LLM tokens, not money quota).
type ModelTokenPackage struct {
	Id              int      `json:"id" gorm:"primaryKey"`
	UserId          int      `json:"user_id" gorm:"not null;index:idx_model_token_packages_user_status,priority:1;index:idx_model_token_packages_user_priority,priority:1"`
	Name            string   `json:"name" gorm:"type:varchar(128);not null;default:''"`
	ModelsJson      string   `json:"models_json" gorm:"type:text;not null"`
	TotalTokens     int64    `json:"total_tokens" gorm:"not null;default:0"`
	RemainingTokens int64    `json:"remaining_tokens" gorm:"not null;default:0"`
	UsedTokens      int64    `json:"used_tokens" gorm:"not null;default:0"`
	InputRatio      float64  `json:"input_ratio" gorm:"not null;default:1"`
	OutputRatio     float64  `json:"output_ratio" gorm:"not null;default:1"`
	CacheRatio      float64  `json:"cache_ratio" gorm:"not null;default:1"`
	Priority        int      `json:"priority" gorm:"not null;default:0;index:idx_model_token_packages_user_priority,priority:2"`
	Status          string   `json:"status" gorm:"type:varchar(32);not null;default:'active';index:idx_model_token_packages_user_status,priority:2"`
	ExpiredAt       int64    `json:"expired_at" gorm:"not null;default:0;index"` // 0 = never expires
	Source          string   `json:"source" gorm:"type:varchar(32);not null;default:''"`
	CreatedBy       int      `json:"created_by" gorm:"not null;default:0"`
	Remark          string   `json:"remark" gorm:"type:text"`
	CreatedAt       int64    `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64    `json:"updated_at" gorm:"autoUpdateTime"`
	Models          []string `json:"models" gorm:"-"`
}

func (ModelTokenPackage) TableName() string {
	return "model_token_packages"
}

type ModelTokenPackageLedger struct {
	Id               int64   `json:"id" gorm:"primaryKey"`
	PackageId        int     `json:"package_id" gorm:"not null;index"`
	UserId           int     `json:"user_id" gorm:"not null;index"`
	RequestId        string  `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	Model            string  `json:"model" gorm:"type:varchar(128);not null;default:''"`
	PromptTokens     int     `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens int     `json:"completion_tokens" gorm:"not null;default:0"`
	CacheTokens      int     `json:"cache_tokens" gorm:"not null;default:0"`
	InputRatio       float64 `json:"input_ratio" gorm:"not null;default:1"`
	OutputRatio      float64 `json:"output_ratio" gorm:"not null;default:1"`
	CacheRatio       float64 `json:"cache_ratio" gorm:"not null;default:1"`
	DeltaTokens      int64   `json:"delta_tokens" gorm:"not null;default:0"`
	Reason           string  `json:"reason" gorm:"type:varchar(32);not null;default:''"`
	CreatedAt        int64   `json:"created_at" gorm:"autoCreateTime;index"`
}

func (ModelTokenPackageLedger) TableName() string {
	return "model_token_package_ledgers"
}

type ModelTokenPackageCreateInput struct {
	UserId      int
	Name        string
	Models      []string
	TotalTokens int64
	InputRatio  float64
	OutputRatio float64
	CacheRatio  float64
	Priority    int
	ExpiredAt   int64
	Source      string
	CreatedBy   int
	Remark      string
}

type ModelTokenPackageConsumeInput struct {
	PackageId        int
	UserId           int
	RequestId        string
	Model            string
	PromptTokens     int
	CompletionTokens int
	CacheTokens      int
	ConsumeTokens    int64
}

func (p *ModelTokenPackage) AfterFind(tx *gorm.DB) error {
	if p == nil {
		return nil
	}
	p.Models = ParseModelTokenPackageModels(p.ModelsJson)
	return nil
}

func ParseModelTokenPackageModels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var models []string
	if err := common.UnmarshalJsonStr(raw, &models); err != nil {
		// fallback: comma-separated
		parts := strings.Split(raw, ",")
		models = make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				models = append(models, part)
			}
		}
		return uniqueNonEmptyStrings(models)
	}
	return uniqueNonEmptyStrings(models)
}

func EncodeModelTokenPackageModels(models []string) (string, error) {
	normalized := uniqueNonEmptyStrings(models)
	if len(normalized) == 0 {
		return "", ErrModelTokenPackageInvalidModels
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func NormalizeModelTokenPackageRatio(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	if value < 0 {
		return 0
	}
	if value > 10 {
		return 10
	}
	return value
}

// ResolveModelTokenPackageRatio treats negative as default 1, then clamps to [0, 10].
func ResolveModelTokenPackageRatio(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 1
	}
	return NormalizeModelTokenPackageRatio(value)
}

func CreateModelTokenPackage(input ModelTokenPackageCreateInput) (*ModelTokenPackage, error) {
	if input.UserId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if input.TotalTokens <= 0 {
		return nil, ErrModelTokenPackageInvalidAmount
	}
	modelsJson, err := EncodeModelTokenPackageModels(input.Models)
	if err != nil {
		return nil, err
	}
	// If all ratios are left as Go zero (typical create defaults), use 1/1/1.
	// Otherwise keep explicit zeros (e.g. cache_ratio=0 means free cache).
	inputRatio, outputRatio, cacheRatio := input.InputRatio, input.OutputRatio, input.CacheRatio
	if inputRatio == 0 && outputRatio == 0 && cacheRatio == 0 {
		inputRatio, outputRatio, cacheRatio = 1, 1, 1
	} else {
		inputRatio = NormalizeModelTokenPackageRatio(inputRatio)
		outputRatio = NormalizeModelTokenPackageRatio(outputRatio)
		cacheRatio = NormalizeModelTokenPackageRatio(cacheRatio)
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Model Token Package"
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = ModelTokenPackageSourceAdminGrant
	}
	pkg := &ModelTokenPackage{
		UserId:          input.UserId,
		Name:            name,
		ModelsJson:      modelsJson,
		TotalTokens:     input.TotalTokens,
		RemainingTokens: input.TotalTokens,
		UsedTokens:      0,
		InputRatio:      inputRatio,
		OutputRatio:     outputRatio,
		CacheRatio:      cacheRatio,
		Priority:        input.Priority,
		Status:          ModelTokenPackageStatusActive,
		ExpiredAt:       input.ExpiredAt,
		Source:          source,
		CreatedBy:       input.CreatedBy,
		Remark:          strings.TrimSpace(input.Remark),
		Models:          ParseModelTokenPackageModels(modelsJson),
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pkg).Error; err != nil {
			return err
		}
		ledger := ModelTokenPackageLedger{
			PackageId:   pkg.Id,
			UserId:      pkg.UserId,
			DeltaTokens: pkg.TotalTokens,
			Reason:      ModelTokenPackageLedgerReasonGrant,
			InputRatio:  pkg.InputRatio,
			OutputRatio: pkg.OutputRatio,
			CacheRatio:  pkg.CacheRatio,
		}
		return tx.Create(&ledger).Error
	})
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func GetModelTokenPackageById(id int) (*ModelTokenPackage, error) {
	if id <= 0 {
		return nil, ErrModelTokenPackageNotFound
	}
	var pkg ModelTokenPackage
	if err := DB.Where("id = ?", id).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelTokenPackageNotFound
		}
		return nil, err
	}
	return &pkg, nil
}

type UserModelTokenPackageSummary struct {
	ActiveCount     int   `json:"active_count"`
	RemainingTokens int64 `json:"remaining_tokens"`
	UsedTokens      int64 `json:"used_tokens"`
	TotalPackages   int   `json:"total_packages"`
}

// FillUserModelTokenPackageSummaries attaches package summaries onto users (non-DB fields).
func FillUserModelTokenPackageSummaries(users []*User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]int, 0, len(users))
	index := map[int]*User{}
	for _, user := range users {
		if user == nil || user.Id <= 0 {
			continue
		}
		ids = append(ids, user.Id)
		index[user.Id] = user
		user.ModelTokenPackageActiveCount = 0
		user.ModelTokenPackageRemaining = 0
		user.ModelTokenPackageUsed = 0
		user.ModelTokenPackageTotal = 0
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		UserId          int   `gorm:"column:user_id"`
		ActiveCount     int64 `gorm:"column:active_count"`
		RemainingTokens int64 `gorm:"column:remaining_tokens"`
		UsedTokens      int64 `gorm:"column:used_tokens"`
		TotalPackages   int64 `gorm:"column:total_packages"`
	}
	err := DB.Model(&ModelTokenPackage{}).
		Select(`user_id,
			COUNT(*) AS total_packages,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN status = ? THEN remaining_tokens ELSE 0 END), 0) AS remaining_tokens,
			COALESCE(SUM(used_tokens), 0) AS used_tokens`,
			ModelTokenPackageStatusActive, ModelTokenPackageStatusActive).
		Where("user_id IN ?", ids).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		user := index[row.UserId]
		if user == nil {
			continue
		}
		user.ModelTokenPackageActiveCount = int(row.ActiveCount)
		user.ModelTokenPackageRemaining = row.RemainingTokens
		user.ModelTokenPackageUsed = row.UsedTokens
		user.ModelTokenPackageTotal = int(row.TotalPackages)
	}
	return nil
}

func ListModelTokenPackagesByUser(userId int, includeInactive bool) ([]ModelTokenPackage, error) {
	if userId <= 0 {
		return []ModelTokenPackage{}, nil
	}
	query := DB.Where("user_id = ?", userId)
	if !includeInactive {
		query = query.Where("status = ?", ModelTokenPackageStatusActive)
	}
	var rows []ModelTokenPackage
	if err := query.Order("priority desc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListActiveModelTokenPackagesForUser(userId int, now int64) ([]ModelTokenPackage, error) {
	if userId <= 0 {
		return []ModelTokenPackage{}, nil
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	var rows []ModelTokenPackage
	err := DB.Where("user_id = ? AND status = ? AND remaining_tokens > 0 AND (expired_at = 0 OR expired_at > ?)",
		userId, ModelTokenPackageStatusActive, now).
		Order("priority desc, CASE WHEN expired_at = 0 THEN 1 ELSE 0 END asc, expired_at asc, id asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func ConsumeModelTokenPackage(input ModelTokenPackageConsumeInput) (*ModelTokenPackage, error) {
	if input.PackageId <= 0 || input.UserId <= 0 {
		return nil, ErrModelTokenPackageNotFound
	}
	if input.ConsumeTokens < 0 {
		return nil, ErrModelTokenPackageInvalidAmount
	}
	if input.ConsumeTokens == 0 {
		pkg, err := GetModelTokenPackageById(input.PackageId)
		if err != nil {
			return nil, err
		}
		if pkg.UserId != input.UserId {
			return nil, ErrModelTokenPackageNotFound
		}
		return pkg, nil
	}

	var updated ModelTokenPackage
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pkg ModelTokenPackage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", input.PackageId, input.UserId).
			First(&pkg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelTokenPackageNotFound
			}
			return err
		}
		if pkg.Status != ModelTokenPackageStatusActive {
			return ErrModelTokenPackageNotActive
		}
		now := common.GetTimestamp()
		if pkg.ExpiredAt > 0 && pkg.ExpiredAt <= now {
			_ = tx.Model(&ModelTokenPackage{}).Where("id = ?", pkg.Id).Update("status", ModelTokenPackageStatusExpired).Error
			return ErrModelTokenPackageNotActive
		}
		if pkg.RemainingTokens < input.ConsumeTokens {
			return ErrModelTokenPackageInsufficient
		}
		remaining := pkg.RemainingTokens - input.ConsumeTokens
		used := pkg.UsedTokens + input.ConsumeTokens
		status := pkg.Status
		if remaining <= 0 {
			remaining = 0
			status = ModelTokenPackageStatusExhausted
		}
		result := tx.Model(&ModelTokenPackage{}).
			Where("id = ? AND user_id = ? AND status = ? AND remaining_tokens >= ?", pkg.Id, input.UserId, ModelTokenPackageStatusActive, input.ConsumeTokens).
			Updates(map[string]any{
				"remaining_tokens": remaining,
				"used_tokens":      used,
				"status":           status,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrModelTokenPackageInsufficient
		}
		ledger := ModelTokenPackageLedger{
			PackageId:        pkg.Id,
			UserId:           input.UserId,
			RequestId:        strings.TrimSpace(input.RequestId),
			Model:            strings.TrimSpace(input.Model),
			PromptTokens:     input.PromptTokens,
			CompletionTokens: input.CompletionTokens,
			CacheTokens:      input.CacheTokens,
			InputRatio:       pkg.InputRatio,
			OutputRatio:      pkg.OutputRatio,
			CacheRatio:       pkg.CacheRatio,
			DeltaTokens:      -input.ConsumeTokens,
			Reason:           ModelTokenPackageLedgerReasonConsume,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", pkg.Id).First(&updated).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func AdjustModelTokenPackage(packageId int, userId int, delta int64, reason string, actorUserId int) (*ModelTokenPackage, error) {
	if packageId <= 0 || userId <= 0 {
		return nil, ErrModelTokenPackageNotFound
	}
	if delta == 0 {
		return GetModelTokenPackageById(packageId)
	}
	if reason == "" {
		reason = ModelTokenPackageLedgerReasonAdjust
	}
	var updated ModelTokenPackage
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pkg ModelTokenPackage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", packageId, userId).
			First(&pkg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelTokenPackageNotFound
			}
			return err
		}
		remaining := pkg.RemainingTokens + delta
		if remaining < 0 {
			return ErrModelTokenPackageInsufficient
		}
		total := pkg.TotalTokens
		used := pkg.UsedTokens
		if delta > 0 {
			total += delta
		} else {
			used -= delta // delta negative => used increases? No: admin reduce remaining without consume
			// For negative delta: reduce remaining only; total unchanged; used unchanged
			used = pkg.UsedTokens
		}
		status := pkg.Status
		if status == ModelTokenPackageStatusDisabled {
			return ErrModelTokenPackageNotActive
		}
		if remaining == 0 {
			status = ModelTokenPackageStatusExhausted
		} else if status == ModelTokenPackageStatusExhausted {
			status = ModelTokenPackageStatusActive
		}
		updates := map[string]any{
			"remaining_tokens": remaining,
			"status":           status,
		}
		if delta > 0 {
			updates["total_tokens"] = total
		}
		if err := tx.Model(&ModelTokenPackage{}).Where("id = ?", pkg.Id).Updates(updates).Error; err != nil {
			return err
		}
		ledger := ModelTokenPackageLedger{
			PackageId:   pkg.Id,
			UserId:      userId,
			DeltaTokens: delta,
			Reason:      reason,
			InputRatio:  pkg.InputRatio,
			OutputRatio: pkg.OutputRatio,
			CacheRatio:  pkg.CacheRatio,
			Model:       fmt.Sprintf("actor:%d", actorUserId),
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", pkg.Id).First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func DisableModelTokenPackage(packageId int, userId int) (*ModelTokenPackage, error) {
	if packageId <= 0 || userId <= 0 {
		return nil, ErrModelTokenPackageNotFound
	}
	var updated ModelTokenPackage
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ModelTokenPackage{}).
			Where("id = ? AND user_id = ? AND status <> ?", packageId, userId, ModelTokenPackageStatusDisabled).
			Update("status", ModelTokenPackageStatusDisabled)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("id = ? AND user_id = ?", packageId, userId).First(&updated).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrModelTokenPackageNotFound
				}
				return err
			}
			return nil
		}
		ledger := ModelTokenPackageLedger{
			PackageId:   packageId,
			UserId:      userId,
			DeltaTokens: 0,
			Reason:      ModelTokenPackageLedgerReasonDisable,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", packageId).First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func ListModelTokenPackageLedger(packageId int, userId int, offset int, limit int) ([]ModelTokenPackageLedger, int64, error) {
	if packageId <= 0 || userId <= 0 {
		return []ModelTokenPackageLedger{}, 0, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := DB.Model(&ModelTokenPackageLedger{}).Where("package_id = ? AND user_id = ?", packageId, userId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ModelTokenPackageLedger
	if err := query.Order("id desc").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func PackageCoversModel(pkg ModelTokenPackage, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	models := pkg.Models
	if len(models) == 0 {
		models = ParseModelTokenPackageModels(pkg.ModelsJson)
	}
	for _, item := range models {
		if strings.EqualFold(strings.TrimSpace(item), modelName) {
			return true
		}
	}
	return false
}

func uniqueNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
