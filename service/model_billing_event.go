package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"gorm.io/gorm"
)

type ModelRequestBillingEventInput struct {
	UsageKind string

	ModelName string
	TokenName string
	PriceUnit string

	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	InputTokens      int
	OutputTokens     int

	CacheTokens            int
	CacheCreationTokens    int
	CacheCreationTokens5m  int
	CacheCreationTokens1h  int
	ImageTokens            int
	AudioTokens            int
	ReasoningTokens        int
	ToolCallSurchargeQuota int64

	Quota        int
	TieredResult *billingexpr.TieredResult
	Metadata     map[string]any
}

func RecordModelRequestBillingEvent(relayInfo *relaycommon.RelayInfo, input ModelRequestBillingEventInput) error {
	if relayInfo == nil || input.Quota <= 0 {
		return nil
	}

	requestId := strings.TrimSpace(relayInfo.RequestId)
	if requestId == "" {
		requestId = fallbackModelRequestBillingSourceId(relayInfo)
	}
	modelName := strings.TrimSpace(input.ModelName)
	if modelName == "" {
		modelName = relayInfo.OriginModelName
	}
	billingSource := strings.TrimSpace(relayInfo.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	priceUnit := strings.TrimSpace(input.PriceUnit)
	if priceUnit == "" {
		priceUnit = "token_usage"
	}

	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	metadata := map[string]any{
		"usage_kind":                 input.UsageKind,
		"model_name":                 modelName,
		"origin_model_name":          relayInfo.OriginModelName,
		"upstream_model_name":        upstreamModelName(relayInfo),
		"token_name":                 input.TokenName,
		"relay_mode":                 relayInfo.RelayMode,
		"relay_format":               relayInfo.RelayFormat,
		"final_request_relay_format": relayInfo.GetFinalRequestRelayFormat(),
		"request_url_path":           relayInfo.RequestURLPath,
		"channel_id":                 billingEventChannelId(relayInfo),
		"channel_type":               billingEventChannelType(relayInfo),
		"is_stream":                  relayInfo.IsStream,
		"is_playground":              relayInfo.IsPlayground,
		"prompt_tokens":              input.PromptTokens,
		"completion_tokens":          input.CompletionTokens,
		"total_tokens":               input.TotalTokens,
		"input_tokens":               input.InputTokens,
		"output_tokens":              input.OutputTokens,
		"cache_tokens":               input.CacheTokens,
		"cache_creation_tokens":      input.CacheCreationTokens,
		"cache_creation_tokens_5m":   input.CacheCreationTokens5m,
		"cache_creation_tokens_1h":   input.CacheCreationTokens1h,
		"image_tokens":               input.ImageTokens,
		"audio_tokens":               input.AudioTokens,
		"reasoning_tokens":           input.ReasoningTokens,
		"tool_call_surcharge_quota":  input.ToolCallSurchargeQuota,
		"model_ratio":                relayInfo.PriceData.ModelRatio,
		"group_ratio":                groupRatio,
		"model_price":                relayInfo.PriceData.ModelPrice,
		"use_price":                  relayInfo.PriceData.UsePrice,
		"billing_source":             billingSource,
		"subscription_id":            relayInfo.SubscriptionId,
		"subscription_plan_id":       relayInfo.SubscriptionPlanId,
		"subscription_plan_title":    relayInfo.SubscriptionPlanTitle,
	}
	for key, value := range input.Metadata {
		metadata[key] = value
	}
	appendTieredModelBillingMetadata(metadata, relayInfo, input.TieredResult)
	metadataBytes, err := common.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = model.CreateBillingEventIfNotExists(nil, &model.BillingEvent{
		EventId:       billingEventID(model.BillingEventSourceModelRequest, requestId, "settlement"),
		UserId:        relayInfo.UserId,
		TokenId:       relayInfo.TokenId,
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      truncateBillingEventString(requestId, 128),
		EventType:     model.BillingEventTypeDebit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     truncateBillingEventString(requestId, 128),
		Group:         relayInfo.UsingGroup,
		BillingSource: billingSource,
		PriceUnit:     priceUnit,
		Currency:      "quota",
		AmountQuota:   input.Quota,
		QuotaDelta:    -input.Quota,
		Cost:          modelRequestBillingCost(input.Quota),
		Metadata:      string(metadataBytes),
		CreatedAt:     common.GetTimestamp(),
	})
	return err
}

func RecordMidjourneyBillingEvent(relayInfo *relaycommon.RelayInfo, modelName string, tokenName string, action string, upstreamTaskId string, quota int) error {
	if relayInfo == nil || quota <= 0 {
		return nil
	}
	return RecordModelRequestBillingEvent(relayInfo, ModelRequestBillingEventInput{
		UsageKind: "midjourney",
		ModelName: modelName,
		TokenName: tokenName,
		PriceUnit: "per_call",
		Quota:     quota,
		Metadata: map[string]any{
			"midjourney_action":           action,
			"midjourney_upstream_task_id": upstreamTaskId,
		},
	})
}

func RecordMidjourneyRefundBillingEvent(task *model.Midjourney, quota int, reason string) error {
	if task == nil || quota <= 0 {
		return nil
	}
	sourceId := midjourneyBillingSourceId(task)
	return model.RecordFundingBillingEvent(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      sourceId,
		Phase:         "refund",
		UserId:        task.UserId,
		TokenId:       task.TokenId,
		RequestId:     sourceId,
		Group:         task.Group,
		BillingSource: midjourneyBillingSource(task),
		PriceUnit:     "per_call_refund",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   quota,
		Metadata: map[string]any{
			"usage_kind":                  "midjourney",
			"reason":                      reason,
			"subscription_id":             task.SubscriptionId,
			"midjourney_action":           task.Action,
			"midjourney_upstream_task_id": task.MjId,
			"midjourney_task_pk":          task.Id,
			"midjourney_fail_reason":      task.FailReason,
			"midjourney_status":           task.Status,
			"midjourney_progress":         task.Progress,
			"midjourney_submit_time":      task.SubmitTime,
			"midjourney_start_time":       task.StartTime,
			"midjourney_finish_time":      task.FinishTime,
			"midjourney_original_quota":   task.Quota,
			"midjourney_original_request": task.RequestId,
		},
	})
}

func RefundMidjourneyQuota(ctx context.Context, task *model.Midjourney, reason string) {
	if task == nil || task.Quota <= 0 {
		return
	}
	quota := task.Quota
	billingSource := midjourneyBillingSource(task)
	switch billingSource {
	case BillingSourceSubscription:
		if task.SubscriptionId <= 0 {
			logger.LogError(ctx, fmt.Sprintf("fail to refund midjourney subscription quota: missing subscription id task %s", task.MjId))
			return
		}
		if err := model.PostConsumeUserSubscriptionDelta(task.SubscriptionId, -int64(quota)); err != nil {
			logger.LogError(ctx, "fail to refund midjourney subscription quota: "+err.Error())
			return
		}
	default:
		if err := model.IncreaseUserQuota(task.UserId, quota, false); err != nil {
			logger.LogError(ctx, "fail to increase user quota: "+err.Error())
			return
		}
	}
	if task.TokenId > 0 {
		token, err := model.GetTokenById(task.TokenId)
		if err != nil && err != gorm.ErrRecordNotFound {
			logger.LogWarn(ctx, fmt.Sprintf("failed to load midjourney token for refund tokenId=%d task=%s: %s", task.TokenId, task.MjId, err.Error()))
		} else if err == nil {
			if tokenErr := model.IncreaseTokenQuota(task.TokenId, token.Key, quota); tokenErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("failed to refund midjourney token quota tokenId=%d task=%s: %s", task.TokenId, task.MjId, tokenErr.Error()))
			}
		}
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     quota,
		TokenId:   task.TokenId,
		Group:     task.Group,
		Other: map[string]interface{}{
			"task_id":         task.MjId,
			"reason":          reason,
			"request_id":      midjourneyBillingSourceId(task),
			"billing_source":  billingSource,
			"subscription_id": task.SubscriptionId,
			"usage_kind":      "midjourney",
		},
	})
	if err := RecordMidjourneyRefundBillingEvent(task, quota, reason); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("record midjourney refund billing event failed task %s: %s", task.MjId, err.Error()))
	}
}

func midjourneyBillingSourceId(task *model.Midjourney) string {
	if task == nil {
		return ""
	}
	if sourceId := strings.TrimSpace(task.RequestId); sourceId != "" {
		return sourceId
	}
	if sourceId := strings.TrimSpace(task.MjId); sourceId != "" {
		return "midjourney:" + sourceId
	}
	return fmt.Sprintf("midjourney_pk:%d", task.Id)
}

func midjourneyBillingSource(task *model.Midjourney) string {
	if task == nil {
		return BillingSourceWallet
	}
	source := strings.TrimSpace(task.BillingSource)
	if source == BillingSourceSubscription {
		return BillingSourceSubscription
	}
	return BillingSourceWallet
}

func fallbackModelRequestBillingSourceId(relayInfo *relaycommon.RelayInfo) string {
	return fmt.Sprintf("user:%d:token:%d:start:%d:mode:%d", relayInfo.UserId, relayInfo.TokenId, relayInfo.StartTime.UnixNano(), relayInfo.RelayMode)
}

func modelRequestBillingCost(quota int) float64 {
	return billingEventCost(quota)
}

func upstreamModelName(relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil || relayInfo.ChannelMeta == nil {
		return ""
	}
	return relayInfo.ChannelMeta.UpstreamModelName
}

func billingEventChannelId(relayInfo *relaycommon.RelayInfo) int {
	if relayInfo == nil || relayInfo.ChannelMeta == nil {
		return 0
	}
	return relayInfo.ChannelMeta.ChannelId
}

func billingEventChannelType(relayInfo *relaycommon.RelayInfo) int {
	if relayInfo == nil || relayInfo.ChannelMeta == nil {
		return 0
	}
	return relayInfo.ChannelMeta.ChannelType
}

func appendTieredModelBillingMetadata(metadata map[string]any, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if metadata == nil || relayInfo == nil || relayInfo.TieredBillingSnapshot == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	metadata["billing_mode"] = snap.BillingMode
	metadata["tiered_expr_hash"] = snap.ExprHash
	metadata["tiered_expr_version"] = snap.ExprVersion
	metadata["tiered_estimated_tier"] = snap.EstimatedTier
	metadata["tiered_estimated_quota_before_group"] = snap.EstimatedQuotaBeforeGroup
	metadata["tiered_estimated_quota_after_group"] = snap.EstimatedQuotaAfterGroup
	if result == nil {
		return
	}
	metadata["tiered_matched_tier"] = result.MatchedTier
	metadata["tiered_crossed_tier"] = result.CrossedTier
	metadata["tiered_actual_quota_before_group"] = result.ActualQuotaBeforeGroup
	metadata["tiered_actual_quota_after_group"] = result.ActualQuotaAfterGroup
}
