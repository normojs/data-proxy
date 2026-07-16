package service

import (
	"encoding/base64"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_ratio"] = modelRatio
	other["group_ratio"] = groupRatio
	other["completion_ratio"] = completionRatio
	other["cache_tokens"] = cacheTokens
	other["cache_ratio"] = cacheRatio
	other["model_price"] = modelPrice
	other["user_group_ratio"] = userGroupRatio
	other["frt"] = float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = ctx.GetStringSlice("use_channel")
	isMultiKey := common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}

	isLocalCountTokens := common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	AppendChannelAffinityAdminInfo(ctx, adminInfo)
	AppendMultiKeyAffinityAdminInfo(ctx, adminInfo)
	AppendChannelFailoverAdminInfo(ctx, adminInfo)

	other["admin_info"] = adminInfo
	appendUserFacingRetrySummary(ctx, other)
	appendRequestPath(ctx, relayInfo, other)
	appendRequestConversionChain(relayInfo, other)
	appendRequestConversionMeta(relayInfo, other)
	appendFinalRequestFormat(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	appendParamOverrideInfo(relayInfo, other)
	AppendStreamStatus(relayInfo, other)
	return other
}

// appendUserFacingRetrySummary writes a sanitized retry note when channel failover
// actually retried at least once. It never includes channel names/ids.
func appendUserFacingRetrySummary(ctx *gin.Context, other map[string]interface{}) {
	if ctx == nil || other == nil {
		return
	}
	events, ok := common.GetContextKeyType[[]map[string]interface{}](ctx, constant.ContextKeyChannelFailoverTrace)
	if !ok || len(events) == 0 {
		return
	}
	retryPlanned := false
	selectedAfterFail := 0
	for _, event := range events {
		if planned, ok := event["retry_planned"].(bool); ok && planned {
			retryPlanned = true
		}
		if eventType, _ := event["event"].(string); eventType == "selected" {
			if idx, ok := event["retry_index"].(int); ok && idx > 0 {
				selectedAfterFail++
			} else if idxF, ok := event["retry_index"].(float64); ok && idxF > 0 {
				selectedAfterFail++
			}
		}
	}
	if !retryPlanned && selectedAfterFail == 0 {
		return
	}
	other["user_retry_summary"] = map[string]interface{}{
		"retried": true,
		// Stable English key for clients; UI translates via i18n.
		"message_key": "upstream_busy_retried",
		"message":     "Upstream was busy; the request was retried on another route.",
	}
}

func appendParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || len(relayInfo.ParamOverrideAudit) == 0 {
		return
	}
	other["po"] = relayInfo.ParamOverrideAudit
}

func AppendStreamStatus(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	hasFirstResponse := relayInfo.HasSendResponse() || relayInfo.ReceivedResponseCount > 0
	classification := ss.ClassifyFailure(hasFirstResponse)
	errorMessages, errorCount := ss.ErrorMessages()
	streamInfo := map[string]interface{}{
		"status":                    status,
		"end_reason":                string(ss.EndReason),
		"failure_category":          string(classification.Category),
		"failure_source":            string(classification.Source),
		"failure_stage":             string(classification.Stage),
		"channel_failure_candidate": classification.ChannelFailureCandidate,
		"has_first_response":        hasFirstResponse,
		"received_response_count":   relayInfo.ReceivedResponseCount,
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.MappedErrorCode != "" {
		streamInfo["mapped_error_code"] = ss.MappedErrorCode
	}
	if ss.MappedErrorStatusCode > 0 {
		streamInfo["mapped_status_code"] = ss.MappedErrorStatusCode
	}
	if ss.MappedErrorMessage != "" {
		streamInfo["mapped_message"] = ss.MappedErrorMessage
	}
	if ss.MappedErrorRuleName != "" {
		streamInfo["mapped_rule"] = ss.MappedErrorRuleName
	}
	if errorCount > 0 {
		streamInfo["error_count"] = errorCount
		streamInfo["errors"] = errorMessages
	}
	other["stream_status"] = streamInfo
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	// Prefer package funding metadata when the request settled against a token package.
	if packageInfo := ModelTokenPackageBillingOtherInfo(relayInfo); len(packageInfo) > 0 {
		for key, value := range packageInfo {
			other[key] = value
		}
		// Keep billing_source aligned for legacy UI that only checks that field.
		other["billing_source"] = BillingSourceModelTokenPackage
		other["funding_source"] = BillingSourceModelTokenPackage
		other["wallet_quota_deducted"] = 0
	} else {
		source := relayInfo.BillingSource
		if source == "" {
			source = BillingSourceWallet
		}
		other["billing_source"] = source
		other["funding_source"] = source
	}
	if relayInfo.UserSetting.BillingPreference != "" {
		other["billing_preference"] = relayInfo.UserSetting.BillingPreference
	}
	if relayInfo.BillingSource == BillingSourceSubscription || other["funding_source"] == BillingSourceSubscription {
		if relayInfo.SubscriptionId != 0 {
			other["subscription_id"] = relayInfo.SubscriptionId
		}
		if relayInfo.SubscriptionPreConsumed > 0 {
			other["subscription_pre_consumed"] = relayInfo.SubscriptionPreConsumed
		}
		// post_delta: settlement delta applied after actual usage is known (can be negative for refund)
		if relayInfo.SubscriptionPostDelta != 0 {
			other["subscription_post_delta"] = relayInfo.SubscriptionPostDelta
		}
		if relayInfo.SubscriptionPlanId != 0 {
			other["subscription_plan_id"] = relayInfo.SubscriptionPlanId
		}
		if relayInfo.SubscriptionPlanTitle != "" {
			other["subscription_plan_title"] = relayInfo.SubscriptionPlanTitle
		}
		// Compute "this request" subscription consumed + remaining
		consumed := relayInfo.SubscriptionPreConsumed + relayInfo.SubscriptionPostDelta
		usedFinal := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		if consumed < 0 {
			consumed = 0
		}
		if usedFinal < 0 {
			usedFinal = 0
		}
		if relayInfo.SubscriptionAmountTotal > 0 {
			remain := relayInfo.SubscriptionAmountTotal - usedFinal
			if remain < 0 {
				remain = 0
			}
			other["subscription_total"] = relayInfo.SubscriptionAmountTotal
			other["subscription_used"] = usedFinal
			other["subscription_remain"] = remain
		}
		if consumed > 0 {
			other["subscription_consumed"] = consumed
		}
		// Wallet quota is not deducted when billed from subscription.
		other["wallet_quota_deducted"] = 0
	}
}

// ApplyWalletFundingAmount records the final wallet debit on consume logs.
// Call after settle with the actual quota charged to the wallet (0 for package/subscription).
func ApplyWalletFundingAmount(other map[string]interface{}, finalQuota int) {
	if other == nil {
		return
	}
	source, _ := other["funding_source"].(string)
	if source == "" {
		if v, ok := other["billing_source"].(string); ok {
			source = v
		}
	}
	switch source {
	case BillingSourceModelTokenPackage, BillingSourceSubscription:
		other["wallet_quota_deducted"] = 0
	default:
		if finalQuota < 0 {
			finalQuota = 0
		}
		other["wallet_quota_deducted"] = finalQuota
		if source == "" {
			other["funding_source"] = BillingSourceWallet
			other["billing_source"] = BillingSourceWallet
		}
	}
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other["request_conversion"] = chain
}

func appendRequestConversionMeta(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	meta := make(map[string]interface{}, len(relayInfo.RequestConversionMeta)+1)
	for key, value := range relayInfo.RequestConversionMeta {
		key = strings.TrimSpace(key)
		if key == "" || isEmptyRequestConversionMetaValue(value) {
			continue
		}
		meta[key] = value
	}
	if len(relayInfo.RequestConversionNotes) > 0 {
		notes := make([]string, 0, len(relayInfo.RequestConversionNotes))
		for _, note := range relayInfo.RequestConversionNotes {
			if note = strings.TrimSpace(note); note != "" {
				notes = append(notes, note)
			}
		}
		if len(notes) > 0 {
			meta["notes"] = notes
		}
	}
	if len(meta) == 0 {
		return
	}
	other["request_conversion_meta"] = meta
}

func isEmptyRequestConversionMetaValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func appendFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		// claude indicates the final upstream request format is Claude Messages.
		// Frontend log rendering uses this to keep the original Claude input display.
		other["claude"] = true
	}
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	info["cache_creation_ratio"] = cacheCreationRatio
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
		info["cache_creation_ratio_5m"] = cacheCreationRatio5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
		info["cache_creation_ratio_1h"] = cacheCreationRatio1h
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData types.PriceData, action string, upstreamTaskId string) map[string]interface{} {
	other := make(map[string]interface{})
	other["usage_kind"] = "midjourney"
	other["model_price"] = priceData.ModelPrice
	other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio
	if action != "" {
		other["midjourney_action"] = action
	}
	if upstreamTaskId != "" {
		other["midjourney_upstream_task_id"] = upstreamTaskId
	}
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = priceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendRequestPath(nil, relayInfo, other)
	appendBillingInfo(relayInfo, other)
	return other
}

// InjectTieredBillingInfo overlays tiered billing fields onto an existing
// module-specific other map. Call this after GenerateTextOtherInfo /
// GenerateClaudeOtherInfo / etc. when the request used tiered_expr billing.
func InjectTieredBillingInfo(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if relayInfo == nil || other == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil {
		return
	}
	other["billing_mode"] = "tiered_expr"
	other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
	if result != nil {
		other["matched_tier"] = result.MatchedTier
	}
}
