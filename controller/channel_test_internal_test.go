package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestResolveChannelTestUserIDUsesRequestUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", 2)

	userID, err := resolveChannelTestUserID(ctx)

	require.NoError(t, err)
	require.Equal(t, 2, userID)
}

func TestResolveChannelTestModelPriority(t *testing.T) {
	channelTestModel := " channel-test-model "
	channel := &model.Channel{
		Type:      constant.ChannelTypeSiliconFlow,
		TestModel: &channelTestModel,
		Models:    " first-model ,second-model",
	}

	testModel, err := resolveChannelTestModel(channel, " requested-model ")
	require.NoError(t, err)
	require.Equal(t, "requested-model", testModel)

	testModel, err = resolveChannelTestModel(channel, "")
	require.NoError(t, err)
	require.Equal(t, "channel-test-model", testModel)

	channel.TestModel = nil
	testModel, err = resolveChannelTestModel(channel, "")
	require.NoError(t, err)
	require.Equal(t, "first-model", testModel)
}

func TestResolveChannelTestModelDefaultsOnlyForOfficialOpenAI(t *testing.T) {
	testModel, err := resolveChannelTestModel(&model.Channel{
		Type: constant.ChannelTypeOpenAI,
	}, "")
	require.NoError(t, err)
	require.Equal(t, defaultOpenAIChannelTestModel, testModel)

	customBaseURL := "https://api.siliconflow.cn"
	_, err = resolveChannelTestModel(&model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		BaseURL: &customBaseURL,
	}, "")
	require.ErrorContains(t, err, "test model is empty")

	_, err = resolveChannelTestModel(&model.Channel{
		Type: constant.ChannelTypeSiliconFlow,
	}, "")
	require.ErrorContains(t, err, "test model is empty")
}
