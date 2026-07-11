package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestPricingActualBillingEventSelectQuotesGroupForDatabase(t *testing.T) {
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.UsingPostgreSQL = false
	require.Contains(t, pricingActualBillingEventSelect(), "`group`")
	require.NotContains(t, pricingActualBillingEventSelect(), `"group"`)

	common.UsingPostgreSQL = true
	selection := pricingActualBillingEventSelect()
	require.Contains(t, selection, `"group"`)
	require.NotContains(t, selection, "`group`")
	require.True(t, strings.HasPrefix(selection, `id, "group",`))
}

func TestGetPlatformPricingActualPricesAggregatesModelAndGroups(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	seedPricingActualBillingEvent(t, "actual-1", "gpt-actual", "default", 1000, 0.002, now-60, map[string]any{
		"prompt_tokens":            1000,
		"completion_tokens":        500,
		"input_tokens":             1000,
		"output_tokens":            500,
		"cache_tokens":             1200,
		"cache_creation_tokens":    50,
		"cache_creation_tokens_5m": 20,
		"cache_creation_tokens_1h": 30,
	})
	seedPricingActualBillingEvent(t, "actual-2", "gpt-actual", "vip", 2000, 0.004, now-120, map[string]any{
		"prompt_tokens":     2000,
		"completion_tokens": 1000,
	})

	byModel, byGroup, err := GetPlatformPricingActualPrices(0)
	require.NoError(t, err)

	actual := byModel["gpt-actual"]
	require.EqualValues(t, 2, actual.RequestCount)
	require.EqualValues(t, defaultPricingActualSampleLimit, actual.SampleLimit)
	require.EqualValues(t, pricingActualCacheTokenThreshold, actual.CacheTokenThreshold)
	require.EqualValues(t, 3000, actual.AmountQuota)
	require.InDelta(t, 0.006, actual.Cost, 0.0000001)
	require.EqualValues(t, 3000, actual.PromptTokens)
	require.EqualValues(t, 1500, actual.CompletionTokens)
	require.EqualValues(t, 3000, actual.InputTokens)
	require.EqualValues(t, 1500, actual.OutputTokens)
	require.EqualValues(t, 1200, actual.CacheTokens)
	require.EqualValues(t, 50, actual.CacheCreationTokens)
	require.EqualValues(t, 5750, actual.TotalBillableTokens)
	require.InDelta(t, 1.04347826, actual.EffectivePricePer1MTokens, 0.0000001)
	require.InDelta(t, 0.00104348, actual.EffectivePricePer1KTokens, 0.0000001)
	require.InDelta(t, 0.003, actual.EffectivePricePerRequest, 0.0000001)
	require.NotNil(t, actual.CachedPrice)
	require.EqualValues(t, 1, actual.CachedPrice.RequestCount)
	require.EqualValues(t, 2750, actual.CachedPrice.TotalBillableTokens)
	require.InDelta(t, 0.72727273, actual.CachedPrice.EffectivePricePer1MTokens, 0.0000001)
	require.NotNil(t, actual.NoCachePrice)
	require.EqualValues(t, 1, actual.NoCachePrice.RequestCount)
	require.EqualValues(t, 3000, actual.NoCachePrice.TotalBillableTokens)
	require.InDelta(t, 1.33333333, actual.NoCachePrice.EffectivePricePer1MTokens, 0.0000001)

	defaultGroup := byGroup["gpt-actual"]["default"]
	require.EqualValues(t, 1, defaultGroup.RequestCount)
	require.EqualValues(t, 2750, defaultGroup.TotalBillableTokens)
	require.NotNil(t, defaultGroup.CachedPrice)
	require.Nil(t, defaultGroup.NoCachePrice)

	vipGroup := byGroup["gpt-actual"]["vip"]
	require.EqualValues(t, 1, vipGroup.RequestCount)
	require.EqualValues(t, 3000, vipGroup.TotalBillableTokens)
	require.Nil(t, vipGroup.CachedPrice)
	require.NotNil(t, vipGroup.NoCachePrice)
}

func TestGetPlatformPricingActualPricesUsesTotalTokensFallback(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	seedPricingActualBillingEvent(t, "actual-total-only", "gpt-total-only", "default", 1500, 0.003, now-30, map[string]any{
		"total_tokens": 3000,
	})

	byModel, byGroup, err := GetPlatformPricingActualPrices(0)
	require.NoError(t, err)

	actual := byModel["gpt-total-only"]
	require.EqualValues(t, 1, actual.RequestCount)
	require.EqualValues(t, 3000, actual.TotalTokens)
	require.EqualValues(t, 3000, actual.TotalBillableTokens)
	require.InDelta(t, 1.0, actual.EffectivePricePer1MTokens, 0.0000001)
	require.InDelta(t, 0.001, actual.EffectivePricePer1KTokens, 0.0000001)

	defaultGroup := byGroup["gpt-total-only"]["default"]
	require.EqualValues(t, 3000, defaultGroup.TotalBillableTokens)
}

func TestGetPlatformPricingActualPricesUsesStrictCacheTokenThreshold(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	seedPricingActualBillingEvent(t, "actual-threshold-exact", "gpt-threshold", "default", 1000, 0.001, now-30, map[string]any{
		"cache_tokens": 1000,
	})
	seedPricingActualBillingEvent(t, "actual-threshold-hit", "gpt-threshold", "default", 1001, 0.001, now-60, map[string]any{
		"cache_tokens": 1001,
	})

	byModel, _, err := GetPlatformPricingActualPrices(0)
	require.NoError(t, err)

	actual := byModel["gpt-threshold"]
	require.NotNil(t, actual.CachedPrice)
	require.EqualValues(t, 1, actual.CachedPrice.RequestCount)
	require.EqualValues(t, 1001, actual.CachedPrice.CacheTokens)
	require.NotNil(t, actual.NoCachePrice)
	require.EqualValues(t, 1, actual.NoCachePrice.RequestCount)
	require.EqualValues(t, 1000, actual.NoCachePrice.CacheTokens)
}

func TestGetPlatformPricingActualPricesForPricingUsesRecentSampleLimit(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	seedPricingActualBillingEvent(t, "actual-newest", "gpt-sampled", "default", 1000, 0.002, now-30, map[string]any{
		"total_tokens": 2000,
	})
	seedPricingActualBillingEvent(t, "actual-second", "gpt-sampled", "default", 1500, 0.003, now-60, map[string]any{
		"total_tokens": 3000,
	})
	seedPricingActualBillingEvent(t, "actual-ignored", "gpt-sampled", "default", 9999, 0.999, now-90, map[string]any{
		"total_tokens": 9999,
	})
	seedPricingActualBillingEvent(t, "actual-vip", "gpt-sampled", "vip", 3000, 0.006, now-120, map[string]any{
		"total_tokens": 6000,
	})

	pricing := []model.Pricing{
		{ModelName: "gpt-sampled", EnableGroup: []string{"default", "vip"}},
	}
	byModel, byGroup, err := GetPlatformPricingActualPricesForPricing(2, pricing)
	require.NoError(t, err)

	actual := byModel["gpt-sampled"]
	require.EqualValues(t, 2, actual.RequestCount)
	require.EqualValues(t, 2, actual.SampleLimit)
	require.EqualValues(t, 5000, actual.TotalBillableTokens)
	require.InDelta(t, 1.0, actual.EffectivePricePer1MTokens, 0.0000001)

	defaultGroup := byGroup["gpt-sampled"]["default"]
	require.EqualValues(t, 2, defaultGroup.RequestCount)
	require.EqualValues(t, 5000, defaultGroup.TotalBillableTokens)

	vipGroup := byGroup["gpt-sampled"]["vip"]
	require.EqualValues(t, 1, vipGroup.RequestCount)
	require.EqualValues(t, 6000, vipGroup.TotalBillableTokens)
}

func TestGetPlatformPricingActualPricesForPricingFallsBackToOlderTarget(t *testing.T) {
	truncate(t)
	originalMaxScan := pricingActualMaxScan
	pricingActualMaxScan = 3
	t.Cleanup(func() {
		pricingActualMaxScan = originalMaxScan
	})

	now := common.GetTimestamp()
	for i := 0; i < 3; i++ {
		seedPricingActualBillingEvent(t, fmt.Sprintf("actual-recent-%d", i), "gpt-hot", "default", 1000, 0.001, now-int64(i+1), map[string]any{
			"total_tokens": 1000,
		})
	}
	seedPricingActualBillingEvent(t, "actual-long-tail", "gpt-long-tail", "vip", 2000, 0.004, now-100, map[string]any{
		"total_tokens": 4000,
	})

	pricing := []model.Pricing{
		{ModelName: "gpt-long-tail", EnableGroup: []string{"vip"}},
	}
	byModel, byGroup, err := GetPlatformPricingActualPricesForPricing(2, pricing)
	require.NoError(t, err)

	actual := byModel["gpt-long-tail"]
	require.EqualValues(t, 1, actual.RequestCount)
	require.True(t, actual.IsFallback)
	require.True(t, actual.PriceMayHaveChanged)
	require.EqualValues(t, 4000, actual.TotalBillableTokens)

	vipGroup := byGroup["gpt-long-tail"]["vip"]
	require.EqualValues(t, 1, vipGroup.RequestCount)
	require.True(t, vipGroup.IsFallback)
	require.True(t, vipGroup.PriceMayHaveChanged)
}

func seedPricingActualBillingEvent(t *testing.T, requestId string, modelName string, group string, quota int, cost float64, createdAt int64, metadata map[string]any) {
	t.Helper()
	metadata["model_name"] = modelName
	raw, err := common.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BillingEvent{
		EventId:     fmt.Sprintf("model_request:%s:settlement", requestId),
		UserId:      1,
		TokenId:     1,
		Source:      model.BillingEventSourceModelRequest,
		SourceId:    requestId,
		EventType:   model.BillingEventTypeDebit,
		Status:      model.BillingEventStatusSettled,
		RequestId:   requestId,
		Group:       group,
		PriceUnit:   "token_usage",
		AmountQuota: quota,
		QuotaDelta:  -quota,
		Cost:        cost,
		Metadata:    string(raw),
		CreatedAt:   createdAt,
	}).Error)
}
