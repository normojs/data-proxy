package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestRecordModelRequestBillingEventIdempotent(t *testing.T) {
	truncate(t)
	common.QuotaPerUnit = 500000

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "model-billing-event-smoke",
		UserId:          11,
		TokenId:         22,
		UsingGroup:      "default",
		OriginModelName: "gpt-test",
		RequestURLPath:  "/v1/chat/completions",
		RelayMode:       1,
		RelayFormat:     types.RelayFormatOpenAI,
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			ModelRatio: 1.5,
			ModelPrice: 0.002,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 2,
			},
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   33,
			ChannelType: 1,
		},
	}

	input := ModelRequestBillingEventInput{
		UsageKind:        "text",
		ModelName:        "gpt-test",
		TokenName:        "unit-token",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		InputTokens:      100,
		OutputTokens:     50,
		CacheTokens:      10,
		Quota:            1200,
	}

	require.NoError(t, RecordModelRequestBillingEvent(relayInfo, input))
	require.NoError(t, RecordModelRequestBillingEvent(relayInfo, input))

	var events []model.BillingEvent
	require.NoError(t, model.DB.Where("request_id = ?", relayInfo.RequestId).Find(&events).Error)
	require.Len(t, events, 1)

	event := events[0]
	require.Equal(t, model.BillingEventSourceModelRequest, event.Source)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, "model_request:model-billing-event-smoke:settlement", event.EventId)
	require.Equal(t, "model-billing-event-smoke", event.SourceId)
	require.Equal(t, "wallet", event.BillingSource)
	require.Equal(t, "token_usage", event.PriceUnit)
	require.Equal(t, 1200, event.AmountQuota)
	require.Equal(t, -1200, event.QuotaDelta)
	require.InDelta(t, 0.0024, event.Cost, 0.000001)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, "text", metadata["usage_kind"])
	require.Equal(t, "gpt-test", metadata["model_name"])
	require.Equal(t, float64(100), metadata["prompt_tokens"])
	require.Equal(t, float64(50), metadata["completion_tokens"])
	require.Equal(t, float64(33), metadata["channel_id"])
}

func TestRecordModelRequestBillingEventTieredBillingMetadata(t *testing.T) {
	truncate(t)

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "model-billing-event-tiered",
		UserId:          12,
		TokenId:         23,
		UsingGroup:      "vip",
		OriginModelName: "gpt-tiered",
		RequestURLPath:  "/v1/responses",
		RelayMode:       1,
		RelayFormat:     types.RelayFormatOpenAI,
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.25},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			ExprHash:                  "hash-123",
			ExprVersion:               4,
			EstimatedTier:             "standard",
			EstimatedQuotaBeforeGroup: 100,
			EstimatedQuotaAfterGroup:  125,
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   44,
			ChannelType: 55,
		},
	}

	require.NoError(t, RecordModelRequestBillingEvent(relayInfo, ModelRequestBillingEventInput{
		UsageKind: "text",
		ModelName: "gpt-tiered",
		Quota:     250,
		TieredResult: &billingexpr.TieredResult{
			ActualQuotaBeforeGroup: 200,
			ActualQuotaAfterGroup:  250,
			MatchedTier:            "long_context",
			CrossedTier:            true,
		},
	}))

	var event model.BillingEvent
	require.NoError(t, model.DB.Where("request_id = ?", relayInfo.RequestId).First(&event).Error)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, float64(44), metadata["channel_id"])
	require.Equal(t, float64(55), metadata["channel_type"])
	require.Equal(t, "tiered_expr", metadata["billing_mode"])
	require.Equal(t, "hash-123", metadata["tiered_expr_hash"])
	require.Equal(t, float64(4), metadata["tiered_expr_version"])
	require.Equal(t, "standard", metadata["tiered_estimated_tier"])
	require.Equal(t, float64(100), metadata["tiered_estimated_quota_before_group"])
	require.Equal(t, float64(125), metadata["tiered_estimated_quota_after_group"])
	require.Equal(t, "long_context", metadata["tiered_matched_tier"])
	require.Equal(t, true, metadata["tiered_crossed_tier"])
	require.Equal(t, float64(200), metadata["tiered_actual_quota_before_group"])
	require.Equal(t, float64(250), metadata["tiered_actual_quota_after_group"])
}

func TestRecordMidjourneyBillingEventUsesPerCallUnit(t *testing.T) {
	truncate(t)
	common.QuotaPerUnit = 500000

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "mj-billing-event",
		UserId:          13,
		TokenId:         24,
		UsingGroup:      "default",
		OriginModelName: "mj_imagine",
		RequestURLPath:  "/mj/submit/imagine",
		RelayMode:       1,
		RelayFormat:     types.RelayFormatMjProxy,
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			ModelPrice: 0.02,
			UsePrice:   true,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1.5,
			},
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   34,
			ChannelType: 2,
		},
	}

	require.NoError(t, RecordMidjourneyBillingEvent(relayInfo, "mj_imagine", "mj-token", "IMAGINE", "mj-upstream-1", 15000))

	var event model.BillingEvent
	require.NoError(t, model.DB.Where("request_id = ?", relayInfo.RequestId).First(&event).Error)
	require.Equal(t, model.BillingEventSourceModelRequest, event.Source)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, "per_call", event.PriceUnit)
	require.Equal(t, 15000, event.AmountQuota)
	require.Equal(t, -15000, event.QuotaDelta)
	require.InDelta(t, 0.03, event.Cost, 0.000001)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, "midjourney", metadata["usage_kind"])
	require.Equal(t, "mj_imagine", metadata["model_name"])
	require.Equal(t, "IMAGINE", metadata["midjourney_action"])
	require.Equal(t, "mj-upstream-1", metadata["midjourney_upstream_task_id"])
	require.Equal(t, float64(34), metadata["channel_id"])
}

func TestRefundMidjourneyQuotaRecordsRefundEvent(t *testing.T) {
	truncate(t)
	common.QuotaPerUnit = 500000
	seedUser(t, 14, 1000)
	seedToken(t, 25, 14, "mj-token-key", 2000)

	task := &model.Midjourney{
		Id:            31,
		UserId:        14,
		Action:        "IMAGINE",
		MjId:          "mj-refund-upstream",
		Status:        "FAILURE",
		Progress:      "100%",
		FailReason:    "构图失败",
		ChannelId:     35,
		Quota:         700,
		RequestId:     "mj-refund-request",
		TokenId:       25,
		Group:         "default",
		BillingSource: BillingSourceWallet,
	}
	require.NoError(t, model.DB.Create(task).Error)

	RefundMidjourneyQuota(context.Background(), task, "构图失败")

	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 14).Error)
	require.Equal(t, 1700, user.Quota)

	var token model.Token
	require.NoError(t, model.DB.First(&token, "id = ?", 25).Error)
	require.Equal(t, 2700, token.RemainQuota)
	require.Equal(t, -700, token.UsedQuota)

	var event model.BillingEvent
	require.NoError(t, model.DB.Where("event_id = ?", "model_request:mj-refund-request:refund").First(&event).Error)
	require.Equal(t, model.BillingEventTypeCredit, event.EventType)
	require.Equal(t, "per_call_refund", event.PriceUnit)
	require.Equal(t, 700, event.AmountQuota)
	require.Equal(t, 700, event.QuotaDelta)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, "midjourney", metadata["usage_kind"])
	require.Equal(t, "构图失败", metadata["reason"])
	require.Equal(t, float64(0), metadata["subscription_id"])
	require.Equal(t, "mj-refund-upstream", metadata["midjourney_upstream_task_id"])
}

func TestRefundMidjourneyQuotaRefundsSubscriptionSource(t *testing.T) {
	truncate(t)
	common.QuotaPerUnit = 500000
	seedUser(t, 15, 1000)
	seedToken(t, 26, 15, "mj-sub-token-key", 3000)
	seedSubscription(t, 41, 15, 10000, 5000)

	task := &model.Midjourney{
		Id:             32,
		UserId:         15,
		Action:         "IMAGINE",
		MjId:           "mj-sub-refund-upstream",
		Status:         "FAILURE",
		Progress:       "100%",
		FailReason:     "构图失败",
		ChannelId:      36,
		Quota:          900,
		RequestId:      "mj-sub-refund-request",
		TokenId:        26,
		Group:          "default",
		BillingSource:  BillingSourceSubscription,
		SubscriptionId: 41,
	}
	require.NoError(t, model.DB.Create(task).Error)

	RefundMidjourneyQuota(context.Background(), task, "构图失败")

	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 15).Error)
	require.Equal(t, 1000, user.Quota)

	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, "id = ?", 41).Error)
	require.Equal(t, int64(4100), sub.AmountUsed)

	var token model.Token
	require.NoError(t, model.DB.First(&token, "id = ?", 26).Error)
	require.Equal(t, 3900, token.RemainQuota)
	require.Equal(t, -900, token.UsedQuota)

	var event model.BillingEvent
	require.NoError(t, model.DB.Where("event_id = ?", "model_request:mj-sub-refund-request:refund").First(&event).Error)
	require.Equal(t, model.BillingEventTypeCredit, event.EventType)
	require.Equal(t, BillingSourceSubscription, event.BillingSource)
	require.Equal(t, "per_call_refund", event.PriceUnit)
	require.Equal(t, 900, event.AmountQuota)
	require.Equal(t, 900, event.QuotaDelta)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, "midjourney", metadata["usage_kind"])
	require.Equal(t, "构图失败", metadata["reason"])
	require.Equal(t, float64(41), metadata["subscription_id"])
	require.Equal(t, "mj-sub-refund-upstream", metadata["midjourney_upstream_task_id"])
}
