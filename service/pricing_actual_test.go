package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestGetPlatformPricingActualPricesAggregatesModelAndGroups(t *testing.T) {
	truncate(t)

	now := common.GetTimestamp()
	seedPricingActualBillingEvent(t, "actual-1", "gpt-actual", "default", 1000, 0.002, now-60, map[string]any{
		"prompt_tokens":            1000,
		"completion_tokens":        500,
		"input_tokens":             1000,
		"output_tokens":            500,
		"cache_tokens":             200,
		"cache_creation_tokens":    50,
		"cache_creation_tokens_5m": 20,
		"cache_creation_tokens_1h": 30,
	})
	seedPricingActualBillingEvent(t, "actual-2", "gpt-actual", "vip", 2000, 0.004, now-120, map[string]any{
		"prompt_tokens":     2000,
		"completion_tokens": 1000,
	})
	seedPricingActualBillingEvent(t, "actual-old", "gpt-actual", "vip", 9999, 0.999, now-7200, map[string]any{
		"prompt_tokens": 1000,
	})

	byModel, byGroup, err := GetPlatformPricingActualPrices(3600)
	require.NoError(t, err)

	actual := byModel["gpt-actual"]
	require.EqualValues(t, 2, actual.RequestCount)
	require.EqualValues(t, 3000, actual.AmountQuota)
	require.InDelta(t, 0.006, actual.Cost, 0.0000001)
	require.EqualValues(t, 3000, actual.PromptTokens)
	require.EqualValues(t, 1500, actual.CompletionTokens)
	require.EqualValues(t, 3000, actual.InputTokens)
	require.EqualValues(t, 1500, actual.OutputTokens)
	require.EqualValues(t, 200, actual.CacheTokens)
	require.EqualValues(t, 50, actual.CacheCreationTokens)
	require.EqualValues(t, 4750, actual.TotalBillableTokens)
	require.InDelta(t, 1.26315789, actual.EffectivePricePer1MTokens, 0.0000001)
	require.InDelta(t, 0.00126316, actual.EffectivePricePer1KTokens, 0.0000001)
	require.InDelta(t, 0.003, actual.EffectivePricePerRequest, 0.0000001)

	defaultGroup := byGroup["gpt-actual"]["default"]
	require.EqualValues(t, 1, defaultGroup.RequestCount)
	require.EqualValues(t, 1750, defaultGroup.TotalBillableTokens)

	vipGroup := byGroup["gpt-actual"]["vip"]
	require.EqualValues(t, 1, vipGroup.RequestCount)
	require.EqualValues(t, 3000, vipGroup.TotalBillableTokens)
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
