package service

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultPricingActualSampleLimit  int64 = 50
	pricingActualCacheTokenThreshold int64 = 1000
	pricingActualFallbackBatchSize         = 500
	pricingActualCacheTTL                  = 30 * time.Second
)

var pricingActualMaxScan = 20000

type pricingActualAccumulator struct {
	model.PricingActualPrice
	cached  *pricingActualAccumulator
	noCache *pricingActualAccumulator
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

type pricingActualCacheEntry struct {
	expiresAt time.Time
	byModel   map[string]model.PricingActualPrice
	byGroup   map[string]map[string]model.PricingActualPrice
}

var (
	pricingActualCacheMu sync.Mutex
	pricingActualCache   = map[string]pricingActualCacheEntry{}
)

func GetPlatformPricingActualPrices(sampleLimit int64) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	return getPlatformPricingActualPrices(sampleLimit, nil)
}

func GetPlatformPricingActualPricesForPricing(sampleLimit int64, pricing []model.Pricing) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	return getPlatformPricingActualPrices(sampleLimit, pricingActualTargetsFromPricing(pricing))
}

func getPlatformPricingActualPrices(sampleLimit int64, targets *pricingActualTargets) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, error) {
	if sampleLimit <= 0 {
		sampleLimit = defaultPricingActualSampleLimit
	}

	cacheKey := pricingActualCacheKey(sampleLimit, targets)
	if byModel, byGroup, ok := getPricingActualCache(cacheKey); ok {
		return byModel, byGroup, nil
	}

	var events []model.BillingEvent
	err := model.DB.
		Model(&model.BillingEvent{}).
		Select(pricingActualBillingEventSelect()).
		Where("source = ? AND event_type = ? AND status = ?",
			model.BillingEventSourceModelRequest,
			model.BillingEventTypeDebit,
			model.BillingEventStatusSettled,
		).
		Order("created_at desc, id desc").
		Limit(pricingActualMaxScan).
		Find(&events).Error
	if err != nil {
		return nil, nil, err
	}

	byModelAcc := map[string]*pricingActualAccumulator{}
	byGroupAcc := map[string]map[string]*pricingActualAccumulator{}
	var cursorCreatedAt int64
	var cursorId int64

	for _, event := range events {
		cursorCreatedAt = event.CreatedAt
		cursorId = event.Id
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

		if targets == nil || targetHasModel(targets, modelName) {
			observePricingActual(byModelAcc, modelName, sampleLimit, event, metadata)
		}
		if targets == nil || targetHasGroup(targets, modelName, group) {
			if _, ok := byGroupAcc[modelName]; !ok {
				byGroupAcc[modelName] = map[string]*pricingActualAccumulator{}
			}
			observePricingActual(byGroupAcc[modelName], group, sampleLimit, event, metadata)
		}

		if pricingActualTargetsFilled(byModelAcc, byGroupAcc, targets, sampleLimit) {
			break
		}
	}

	byModel := make(map[string]model.PricingActualPrice, len(byModelAcc))
	for modelName, acc := range byModelAcc {
		byModel[modelName] = finalizePricingActualAccumulator(acc)
	}

	byGroup := make(map[string]map[string]model.PricingActualPrice, len(byGroupAcc))
	for modelName, groups := range byGroupAcc {
		byGroup[modelName] = make(map[string]model.PricingActualPrice, len(groups))
		for group, acc := range groups {
			byGroup[modelName][group] = finalizePricingActualAccumulator(acc)
		}
	}

	if !targets.empty() {
		if err := attachFallbackPricingActuals(byModel, byGroup, targets, sampleLimit, cursorCreatedAt, cursorId); err != nil {
			return nil, nil, err
		}
	}

	setPricingActualCache(cacheKey, byModel, byGroup)
	return byModel, byGroup, nil
}

func targetHasModel(targets *pricingActualTargets, modelName string) bool {
	if targets == nil || len(targets.models) == 0 {
		return true
	}
	_, ok := targets.models[modelName]
	return ok
}

func targetHasGroup(targets *pricingActualTargets, modelName, group string) bool {
	if targets == nil || len(targets.groups) == 0 {
		return true
	}
	groups := targets.groups[modelName]
	if len(groups) == 0 {
		return false
	}
	_, ok := groups[group]
	return ok
}

func pricingActualTargetsFilled(byModel map[string]*pricingActualAccumulator, byGroup map[string]map[string]*pricingActualAccumulator, targets *pricingActualTargets, sampleLimit int64) bool {
	if targets == nil || targets.empty() {
		return false
	}
	for modelName := range targets.models {
		if byModel[modelName] == nil || byModel[modelName].RequestCount < sampleLimit {
			return false
		}
	}
	for modelName, groups := range targets.groups {
		for group := range groups {
			if byGroup[modelName] == nil || byGroup[modelName][group] == nil || byGroup[modelName][group].RequestCount < sampleLimit {
				return false
			}
		}
	}
	return true
}

func pricingActualBillingEventSelect() string {
	groupColumn := "`group`"
	if common.UsingPostgreSQL {
		groupColumn = `"group"`
	}
	return "id, " + groupColumn + ", price_unit, amount_quota, cost, metadata, created_at"
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

func attachFallbackPricingActuals(
	byModel map[string]model.PricingActualPrice,
	byGroup map[string]map[string]model.PricingActualPrice,
	targets *pricingActualTargets,
	sampleLimit int64,
	cursorCreatedAt int64,
	cursorId int64,
) error {
	missing := missingPricingActuals(byModel, byGroup, targets)
	if missing.count == 0 {
		return nil
	}

	scanned := 0
	for missing.count > 0 && scanned < pricingActualMaxScan {
		var events []model.BillingEvent
		query := model.DB.
			Model(&model.BillingEvent{}).
			Select(pricingActualBillingEventSelect()).
			Where("source = ? AND event_type = ? AND status = ?",
				model.BillingEventSourceModelRequest,
				model.BillingEventTypeDebit,
				model.BillingEventStatusSettled,
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

			actual := fallbackPricingActual(event, metadata, sampleLimit)
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
			if missing.count == 0 || scanned >= pricingActualMaxScan {
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

func fallbackPricingActual(event model.BillingEvent, metadata map[string]any, sampleLimit int64) model.PricingActualPrice {
	acc := map[string]*pricingActualAccumulator{}
	observePricingActual(acc, "last", sampleLimit, event, metadata)
	actual := finalizePricingActualAccumulator(acc["last"])
	markPricingActualFallback(&actual)
	return actual
}

func markPricingActualFallback(actual *model.PricingActualPrice) {
	if actual == nil {
		return
	}
	actual.IsFallback = true
	actual.PriceMayHaveChanged = true
	if actual.CachedPrice != nil {
		markPricingActualFallback(actual.CachedPrice)
	}
	if actual.NoCachePrice != nil {
		markPricingActualFallback(actual.NoCachePrice)
	}
}

func observePricingActual(target map[string]*pricingActualAccumulator, key string, sampleLimit int64, event model.BillingEvent, metadata map[string]any) {
	acc := target[key]
	if acc == nil {
		acc = &pricingActualAccumulator{}
		target[key] = acc
	}
	if acc.RequestCount >= sampleLimit {
		return
	}
	observePricingActualValue(&acc.PricingActualPrice, sampleLimit, event, metadata)

	if metadataInt64(metadata, "cache_tokens") > pricingActualCacheTokenThreshold {
		if acc.cached == nil {
			acc.cached = &pricingActualAccumulator{}
		}
		observePricingActualValue(&acc.cached.PricingActualPrice, sampleLimit, event, metadata)
	} else {
		if acc.noCache == nil {
			acc.noCache = &pricingActualAccumulator{}
		}
		observePricingActualValue(&acc.noCache.PricingActualPrice, sampleLimit, event, metadata)
	}
}

func observePricingActualValue(value *model.PricingActualPrice, sampleLimit int64, event model.BillingEvent, metadata map[string]any) {
	if value.RequestCount == 0 {
		value.SampleLimit = sampleLimit
		value.CacheTokenThreshold = pricingActualCacheTokenThreshold
		value.StartedAt = event.CreatedAt
		value.EndedAt = event.CreatedAt
		value.LastTransactionAt = event.CreatedAt
		value.PriceUnit = strings.TrimSpace(event.PriceUnit)
	}
	if event.CreatedAt < value.StartedAt {
		value.StartedAt = event.CreatedAt
	}
	if event.CreatedAt > value.EndedAt {
		value.EndedAt = event.CreatedAt
		value.LastTransactionAt = event.CreatedAt
	}
	value.RequestCount++
	value.AmountQuota += int64(event.AmountQuota)
	value.Cost += event.Cost
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

	value.PromptTokens += promptTokens
	value.CompletionTokens += completionTokens
	value.TotalTokens += totalTokens
	value.InputTokens += inputTokens
	value.OutputTokens += outputTokens
	value.CacheTokens += cacheTokens
	value.CacheCreationTokens += cacheCreationTokens
	billableTokens := inputTokens + outputTokens + cacheTokens + cacheCreationTokens
	if billableTokens == 0 {
		billableTokens = totalTokens
	}
	value.TotalBillableTokens += billableTokens
	if value.PriceUnit == "" {
		value.PriceUnit = strings.TrimSpace(event.PriceUnit)
	} else if event.PriceUnit != "" && value.PriceUnit != event.PriceUnit {
		value.PriceUnit = "mixed"
	}
}

func finalizePricingActualAccumulator(acc *pricingActualAccumulator) model.PricingActualPrice {
	if acc == nil {
		return model.PricingActualPrice{}
	}
	value := finalizePricingActual(acc.PricingActualPrice)
	if acc.cached != nil && acc.cached.RequestCount > 0 {
		cached := finalizePricingActual(acc.cached.PricingActualPrice)
		value.CachedPrice = &cached
	}
	if acc.noCache != nil && acc.noCache.RequestCount > 0 {
		noCache := finalizePricingActual(acc.noCache.PricingActualPrice)
		value.NoCachePrice = &noCache
	}
	return value
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

func InvalidatePricingActualCache() {
	pricingActualCacheMu.Lock()
	defer pricingActualCacheMu.Unlock()
	pricingActualCache = map[string]pricingActualCacheEntry{}
}

func getPricingActualCache(key string) (map[string]model.PricingActualPrice, map[string]map[string]model.PricingActualPrice, bool) {
	pricingActualCacheMu.Lock()
	defer pricingActualCacheMu.Unlock()
	entry, ok := pricingActualCache[key]
	if !ok {
		return nil, nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(pricingActualCache, key)
		return nil, nil, false
	}
	return clonePricingActualByModel(entry.byModel), clonePricingActualByGroup(entry.byGroup), true
}

func setPricingActualCache(key string, byModel map[string]model.PricingActualPrice, byGroup map[string]map[string]model.PricingActualPrice) {
	pricingActualCacheMu.Lock()
	defer pricingActualCacheMu.Unlock()
	pricingActualCache[key] = pricingActualCacheEntry{
		expiresAt: time.Now().Add(pricingActualCacheTTL),
		byModel:   clonePricingActualByModel(byModel),
		byGroup:   clonePricingActualByGroup(byGroup),
	}
}

func pricingActualCacheKey(sampleLimit int64, targets *pricingActualTargets) string {
	if targets == nil || targets.empty() {
		return "sample:" + strconv.FormatInt(sampleLimit, 10) + "|all"
	}
	models := make([]string, 0, len(targets.models))
	for modelName := range targets.models {
		models = append(models, modelName)
	}
	sort.Strings(models)

	groupPairs := make([]string, 0)
	for modelName, groups := range targets.groups {
		for group := range groups {
			groupPairs = append(groupPairs, modelName+"/"+group)
		}
	}
	sort.Strings(groupPairs)
	return "sample:" + strconv.FormatInt(sampleLimit, 10) + "|models:" + strings.Join(models, ",") + "|groups:" + strings.Join(groupPairs, ",")
}

func clonePricingActualByModel(source map[string]model.PricingActualPrice) map[string]model.PricingActualPrice {
	if len(source) == 0 {
		return map[string]model.PricingActualPrice{}
	}
	cloned := make(map[string]model.PricingActualPrice, len(source))
	for key, value := range source {
		cloned[key] = clonePricingActualPrice(value)
	}
	return cloned
}

func clonePricingActualByGroup(source map[string]map[string]model.PricingActualPrice) map[string]map[string]model.PricingActualPrice {
	if len(source) == 0 {
		return map[string]map[string]model.PricingActualPrice{}
	}
	cloned := make(map[string]map[string]model.PricingActualPrice, len(source))
	for modelName, groups := range source {
		cloned[modelName] = make(map[string]model.PricingActualPrice, len(groups))
		for group, value := range groups {
			cloned[modelName][group] = clonePricingActualPrice(value)
		}
	}
	return cloned
}

func clonePricingActualPrice(value model.PricingActualPrice) model.PricingActualPrice {
	if value.CachedPrice != nil {
		cached := clonePricingActualPrice(*value.CachedPrice)
		value.CachedPrice = &cached
	}
	if value.NoCachePrice != nil {
		noCache := clonePricingActualPrice(*value.NoCachePrice)
		value.NoCachePrice = &noCache
	}
	return value
}
