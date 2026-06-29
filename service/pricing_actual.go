package service

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultPricingActualWindowSeconds int64 = 3600
	pricingActualFallbackBatchSize          = 500
	pricingActualFallbackMaxScan            = 20000
)

type pricingActualAccumulator struct {
	model.PricingActualPrice
}

type pricingActualTargets struct {
	models map[string]struct{}
	groups map[string]map[string]struct{}
}

type pricingActualMissing struct {
	models map[string]struct{}
	groups map[string]map[string]struct{}
	count  int
}

func GetPlatformPricingActualPrices(windowSeconds int64) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	return getPlatformPricingActualPrices(windowSeconds, nil)
}

func GetPlatformPricingActualPricesForPricing(windowSeconds int64, pricing []model.Pricing) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	return getPlatformPricingActualPrices(windowSeconds, pricingActualTargetsFromPricing(pricing))
}

func getPlatformPricingActualPrices(windowSeconds int64, targets *pricingActualTargets) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	if windowSeconds <= 0 {
		windowSeconds = defaultPricingActualWindowSeconds
	}
	endAt := common.GetTimestamp()
	startAt := endAt - windowSeconds

	var events []model.BillingEvent
	err := model.DB.
		Model(&model.BillingEvent{}).
		Select("id, `group`, price_unit, amount_quota, cost, metadata, created_at").
		Where("source = ? AND event_type = ? AND status = ? AND created_at >= ? AND created_at <= ?",
			model.BillingEventSourceModelRequest,
			model.BillingEventTypeDebit,
			model.BillingEventStatusSettled,
			startAt,
			endAt,
		).
		Find(&events).Error
	if err != nil {
		return nil, nil, err
	}

	byModelAcc := map[string]*pricingActualAccumulator{}
	byGroupAcc := map[string]map[string]*pricingActualAccumulator{}

	for _, event := range events {
		metadata := map[string]any{}
		_ = common.UnmarshalJsonStr(event.Metadata, &metadata)

		modelName := strings.TrimSpace(metadataString(metadata, "model_name"))
		if modelName == "" {
			continue
		}
		group := strings.TrimSpace(event.Group)
		if group == "" {
			group = "default"
		}

		observePricingActual(byModelAcc, modelName, windowSeconds, startAt, endAt, event, metadata)
		if _, ok := byGroupAcc[modelName]; !ok {
			byGroupAcc[modelName] = map[string]*pricingActualAccumulator{}
		}
		observePricingActual(byGroupAcc[modelName], group, windowSeconds, startAt, endAt, event, metadata)
	}

	byModel := make(map[string]model.PricingActualPrice, len(byModelAcc))
	for modelName, acc := range byModelAcc {
		byModel[modelName] = finalizePricingActual(acc.PricingActualPrice)
	}

	byGroup := make(map[string]map[string]model.PricingActualPrice, len(byGroupAcc))
	for modelName, groups := range byGroupAcc {
		byGroup[modelName] = make(map[string]model.PricingActualPrice, len(groups))
		for group, acc := range groups {
			byGroup[modelName][group] = finalizePricingActual(acc.PricingActualPrice)
		}
	}

	if !targets.empty() {
		if err := attachFallbackPricingActuals(byModel, byGroup, targets, startAt); err != nil {
			return nil, nil, err
		}
	}

	return byModel, byGroup, nil
}

func pricingActualTargetsFromPricing(pricing []model.Pricing) *pricingActualTargets {
	targets := &pricingActualTargets{
		models: map[string]struct{}{},
		groups: map[string]map[string]struct{}{},
	}
	for _, item := range pricing {
		modelName := strings.TrimSpace(item.ModelName)
		if modelName == "" {
			continue
		}
		targets.models[modelName] = struct{}{}
		for _, group := range item.EnableGroup {
			group = strings.TrimSpace(group)
			if group == "" || group == "all" {
				continue
			}
			if _, ok := targets.groups[modelName]; !ok {
				targets.groups[modelName] = map[string]struct{}{}
			}
			targets.groups[modelName][group] = struct{}{}
		}
	}
	return targets
}

func (targets *pricingActualTargets) empty() bool {
	return targets == nil || (len(targets.models) == 0 && len(targets.groups) == 0)
}

func attachFallbackPricingActuals(byModel map[string]model.PricingActualPrice, byGroup map[string]map[string]model.PricingActualPrice, targets *pricingActualTargets, before int64) error {
	missing := missingPricingActuals(byModel, byGroup, targets)
	if missing.count == 0 {
		return nil
	}

	var cursorCreatedAt int64
	var cursorId int64
	scanned := 0
	for missing.count > 0 && scanned < pricingActualFallbackMaxScan {
		var events []model.BillingEvent
		query := model.DB.
			Model(&model.BillingEvent{}).
			Select("id, `group`, price_unit, amount_quota, cost, metadata, created_at").
			Where("source = ? AND event_type = ? AND status = ? AND created_at < ?",
				model.BillingEventSourceModelRequest,
				model.BillingEventTypeDebit,
				model.BillingEventStatusSettled,
				before,
			)
		if cursorCreatedAt > 0 {
			query = query.Where("(created_at < ? OR (created_at = ? AND id < ?))", cursorCreatedAt, cursorCreatedAt, cursorId)
		}
		if err := query.Order("created_at desc, id desc").Limit(pricingActualFallbackBatchSize).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			break
		}

		for _, event := range events {
			scanned++
			metadata := map[string]any{}
			_ = common.UnmarshalJsonStr(event.Metadata, &metadata)

			modelName := strings.TrimSpace(metadataString(metadata, "model_name"))
			if modelName == "" {
				continue
			}
			group := strings.TrimSpace(event.Group)
			if group == "" {
				group = "default"
			}

			actual := fallbackPricingActual(event, metadata)
			if _, ok := missing.models[modelName]; ok {
				byModel[modelName] = actual
				delete(missing.models, modelName)
				missing.count--
			}
			if groups, ok := missing.groups[modelName]; ok {
				if _, ok := groups[group]; ok {
					if _, ok := byGroup[modelName]; !ok {
						byGroup[modelName] = map[string]model.PricingActualPrice{}
					}
					byGroup[modelName][group] = actual
					delete(groups, group)
					missing.count--
					if len(groups) == 0 {
						delete(missing.groups, modelName)
					}
				}
			}
			if missing.count == 0 || scanned >= pricingActualFallbackMaxScan {
				break
			}
		}

		last := events[len(events)-1]
		cursorCreatedAt = last.CreatedAt
		cursorId = last.Id
		if len(events) < pricingActualFallbackBatchSize {
			break
		}
	}

	return nil
}

func missingPricingActuals(byModel map[string]model.PricingActualPrice, byGroup map[string]map[string]model.PricingActualPrice, targets *pricingActualTargets) pricingActualMissing {
	missing := pricingActualMissing{
		models: map[string]struct{}{},
		groups: map[string]map[string]struct{}{},
	}
	if targets == nil {
		return missing
	}
	for modelName := range targets.models {
		if _, ok := byModel[modelName]; !ok {
			missing.models[modelName] = struct{}{}
			missing.count++
		}
	}
	for modelName, groups := range targets.groups {
		existingGroups := byGroup[modelName]
		for group := range groups {
			if _, ok := existingGroups[group]; ok {
				continue
			}
			if _, ok := missing.groups[modelName]; !ok {
				missing.groups[modelName] = map[string]struct{}{}
			}
			missing.groups[modelName][group] = struct{}{}
			missing.count++
		}
	}
	return missing
}

func fallbackPricingActual(event model.BillingEvent, metadata map[string]any) model.PricingActualPrice {
	acc := map[string]*pricingActualAccumulator{}
	observePricingActual(acc, "last", 0, event.CreatedAt, event.CreatedAt, event, metadata)
	actual := finalizePricingActual(acc["last"].PricingActualPrice)
	actual.LastTransactionAt = event.CreatedAt
	actual.IsFallback = true
	actual.PriceMayHaveChanged = true
	return actual
}

func observePricingActual(target map[string]*pricingActualAccumulator, key string, windowSeconds int64, startAt int64, endAt int64, event model.BillingEvent, metadata map[string]any) {
	acc := target[key]
	if acc == nil {
		acc = &pricingActualAccumulator{PricingActualPrice: model.PricingActualPrice{
			WindowSeconds: windowSeconds,
			StartedAt:     startAt,
			EndedAt:       endAt,
			PriceUnit:     strings.TrimSpace(event.PriceUnit),
		}}
		target[key] = acc
	}
	acc.RequestCount++
	acc.AmountQuota += int64(event.AmountQuota)
	acc.Cost += event.Cost
	promptTokens := metadataInt64(metadata, "prompt_tokens")
	completionTokens := metadataInt64(metadata, "completion_tokens")
	totalTokens := metadataInt64(metadata, "total_tokens")
	inputTokens := metadataInt64(metadata, "input_tokens")
	outputTokens := metadataInt64(metadata, "output_tokens")
	cacheTokens := metadataInt64(metadata, "cache_tokens")
	cacheCreationTokens := pricingActualCacheCreationTokens(metadata)
	if inputTokens == 0 {
		inputTokens = promptTokens
	}
	if outputTokens == 0 {
		outputTokens = completionTokens
	}

	acc.PromptTokens += promptTokens
	acc.CompletionTokens += completionTokens
	acc.TotalTokens += totalTokens
	acc.InputTokens += inputTokens
	acc.OutputTokens += outputTokens
	acc.CacheTokens += cacheTokens
	acc.CacheCreationTokens += cacheCreationTokens
	billableTokens := inputTokens + outputTokens + cacheTokens + cacheCreationTokens
	if billableTokens == 0 {
		billableTokens = totalTokens
	}
	acc.TotalBillableTokens += billableTokens
	if acc.PriceUnit == "" {
		acc.PriceUnit = strings.TrimSpace(event.PriceUnit)
	} else if event.PriceUnit != "" && acc.PriceUnit != event.PriceUnit {
		acc.PriceUnit = "mixed"
	}
}

func finalizePricingActual(value model.PricingActualPrice) model.PricingActualPrice {
	totalBillableTokens := value.TotalBillableTokens
	if totalBillableTokens == 0 {
		inputTokens := value.InputTokens
		if inputTokens == 0 {
			inputTokens = value.PromptTokens
		}
		outputTokens := value.OutputTokens
		if outputTokens == 0 {
			outputTokens = value.CompletionTokens
		}
		totalBillableTokens = inputTokens + outputTokens + value.CacheTokens + value.CacheCreationTokens
		if totalBillableTokens == 0 {
			totalBillableTokens = value.TotalTokens
		}
	}
	value.TotalBillableTokens = totalBillableTokens
	if totalBillableTokens > 0 {
		value.EffectivePricePer1MTokens = roundPricingActual(value.Cost / float64(totalBillableTokens) * 1000000)
		value.EffectivePricePer1KTokens = roundPricingActual(value.Cost / float64(totalBillableTokens) * 1000)
	}
	if value.RequestCount > 0 {
		value.EffectivePricePerRequest = roundPricingActual(value.Cost / float64(value.RequestCount))
	}
	if value.PriceUnit == "" {
		value.PriceUnit = "token_usage"
	}
	value.Cost = roundPricingActual(value.Cost)
	return value
}

func pricingActualCacheCreationTokens(metadata map[string]any) int64 {
	total := metadataInt64(metadata, "cache_creation_tokens")
	split := metadataInt64(metadata, "cache_creation_tokens_5m") + metadataInt64(metadata, "cache_creation_tokens_1h")
	if split > total {
		return split
	}
	return total
}

func roundPricingActual(value float64) float64 {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100000000) / 100000000
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func metadataInt64(metadata map[string]any, key string) int64 {
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case string:
		return int64(common.String2Int(strings.TrimSpace(v)))
	default:
		return 0
	}
}
