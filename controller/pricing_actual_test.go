package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pricingResponseForTest struct {
	Success bool            `json:"success"`
	Data    []model.Pricing `json:"data"`
}

func TestGetPricingIncludesCachedActualPriceBreakdown(t *testing.T) {
	withSelfUseModeDisabled(t)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.BillingEvent{}))
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)

	const modelName = "gpt-4o-mini"
	seedModelListChannelAbility(t, db, 9201, 0, "default", modelName, constant.ChannelTypeOpenAI)
	seedPricingActualBillingEventForController(t, "controller-actual-cached", modelName, "default", 1000, 0.002, map[string]any{
		"input_tokens":          1000,
		"output_tokens":         500,
		"cache_tokens":          1200,
		"cache_creation_tokens": 50,
	})
	seedPricingActualBillingEventForController(t, "controller-actual-no-cache", modelName, "default", 500, 0.001, map[string]any{
		"total_tokens": 1000,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload pricingResponseForTest
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	pricingByName := pricingByModelName(payload.Data)
	pricing, ok := pricingByName[modelName]
	require.True(t, ok)
	require.NotNil(t, pricing.ActualPrice)
	require.NotNil(t, pricing.ActualPrice.CachedPrice)
	require.NotNil(t, pricing.ActualPrice.NoCachePrice)
	require.EqualValues(t, 2, pricing.ActualPrice.RequestCount)
	require.EqualValues(t, 1, pricing.ActualPrice.CachedPrice.RequestCount)
	require.EqualValues(t, 1, pricing.ActualPrice.NoCachePrice.RequestCount)

	defaultActual := pricing.ActualPriceByGroup["default"]
	require.EqualValues(t, 2, defaultActual.RequestCount)
	require.NotNil(t, defaultActual.CachedPrice)
	require.NotNil(t, defaultActual.NoCachePrice)
}

func seedPricingActualBillingEventForController(t *testing.T, requestId string, modelName string, group string, quota int, cost float64, metadata map[string]any) {
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
		CreatedAt:   common.GetTimestamp(),
	}).Error)
}
