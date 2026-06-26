package service

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const defaultPricingActualWindowSeconds int64 = 3600

type pricingActualAccumulator struct {
	model.PricingActualPrice
}

func GetPlatformPricingActualPrices(windowSeconds int64) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
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

	return byModel, byGroup, nil
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
	acc.InputTokens += inputTokens
	acc.OutputTokens += outputTokens
	acc.CacheTokens += cacheTokens
	acc.CacheCreationTokens += cacheCreationTokens
	acc.TotalBillableTokens += inputTokens + outputTokens + cacheTokens + cacheCreationTokens
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
