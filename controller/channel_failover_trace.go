package controller

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ginKeyChannelFailoverAttempt = "channel_failover_attempt"

func recordChannelFailoverSelection(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam, channel *model.Channel, selectedGroup string) {
	attempt := buildChannelFailoverAttempt(c, info, retryParam, channel, selectedGroup)
	if len(attempt) == 0 {
		return
	}
	c.Set(ginKeyChannelFailoverAttempt, attempt)
	event := cloneChannelFailoverMap(attempt)
	event["event"] = "selected"
	appendChannelFailoverEvent(c, event)
}

func updateChannelFailoverRetryDecision(c *gin.Context, retryParam *service.RetryParam, retryPlanned bool) {
	if c == nil {
		return
	}
	attempt, ok := currentChannelFailoverAttempt(c)
	if !ok {
		return
	}
	attempt["retry_planned"] = retryPlanned
	attempt["remaining_retries"] = remainingChannelRetries(retryParam)
	c.Set(ginKeyChannelFailoverAttempt, attempt)
}

func recordChannelFailoverFailure(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, healthDecision service.ChannelHealthDecision) {
	if c == nil || err == nil {
		return
	}
	attempt, _ := currentChannelFailoverAttempt(c)
	event := cloneChannelFailoverMap(attempt)
	event["event"] = "failed"
	event["channel_id"] = channelError.ChannelId
	event["channel_name"] = channelError.ChannelName
	event["channel_type"] = channelError.ChannelType
	event["auto_ban"] = channelError.AutoBan
	if channelError.IsMultiKey {
		event["is_multi_key"] = true
	}
	if code := err.GetErrorCode(); code != "" {
		event["error_code"] = string(code)
	}
	if errorType := strings.TrimSpace(string(err.GetErrorType())); errorType != "" {
		event["error_type"] = errorType
	}
	if err.StatusCode != 0 {
		event["status_code"] = err.StatusCode
	}
	if reason := strings.TrimSpace(err.MaskSensitiveErrorWithStatusCode()); reason != "" {
		event["reason"] = common.LocalLogPreview(reason)
	}
	if healthDecision.Action != "" && healthDecision.Action != service.ChannelErrorActionNone {
		event["health_action"] = string(healthDecision.Action)
	}
	if healthDecision.FailureCount > 0 {
		event["health_failure_count"] = healthDecision.FailureCount
	}
	if !healthDecision.CooldownUntil.IsZero() {
		event["health_cooldown_until"] = healthDecision.CooldownUntil.Unix()
	}
	if healthDecision.Reason != "" {
		event["health_reason"] = common.LocalLogPreview(healthDecision.Reason)
	}
	runtimeHealth := service.ChannelRuntimeHealthSnapshot(channelError.ChannelId)
	if runtimeHealth.Status != "" && runtimeHealth.Status != "healthy" {
		event["runtime_status"] = runtimeHealth.Status
		event["temporarily_unavailable"] = runtimeHealth.TemporarilyUnavailable
		if runtimeHealth.ConsecutiveFailures > 0 {
			event["consecutive_failures"] = runtimeHealth.ConsecutiveFailures
		}
		if runtimeHealth.CooldownUntil > 0 {
			event["cooldown_until"] = runtimeHealth.CooldownUntil
		}
	}
	appendChannelFailoverEvent(c, event)
}

func buildChannelFailoverAttempt(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam, channel *model.Channel, selectedGroup string) map[string]interface{} {
	if c == nil || channel == nil {
		return nil
	}
	attempt := map[string]interface{}{
		"retry_index":       channelFailoverRetryIndex(retryParam),
		"remaining_retries": remainingChannelRetries(retryParam),
		"channel_id":        channel.Id,
		"channel_name":      channel.Name,
		"channel_type":      channel.Type,
		"auto_ban":          channel.GetAutoBan(),
	}
	if info != nil {
		if info.TokenGroup != "" {
			attempt["token_group"] = info.TokenGroup
		}
		if info.OriginModelName != "" {
			attempt["model_name"] = info.OriginModelName
		}
		if selectedGroup == "" {
			selectedGroup = info.UsingGroup
		}
	}
	if selectedGroup == "" {
		selectedGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if selectedGroup != "" {
		attempt["selected_group"] = selectedGroup
	}
	if retryParam != nil && len(retryParam.ExcludeChannelIds) > 0 {
		attempt["excluded_channel_ids"] = sortedExcludedChannelIds(retryParam.ExcludeChannelIds)
	}
	if common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) {
		attempt["is_multi_key"] = true
		attempt["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
	}
	return attempt
}

func currentChannelFailoverAttempt(c *gin.Context) (map[string]interface{}, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(ginKeyChannelFailoverAttempt)
	if !ok || value == nil {
		return nil, false
	}
	attempt, ok := value.(map[string]interface{})
	if !ok || len(attempt) == 0 {
		return nil, false
	}
	return cloneChannelFailoverMap(attempt), true
}

func appendChannelFailoverEvent(c *gin.Context, event map[string]interface{}) {
	if c == nil || len(event) == 0 {
		return
	}
	event["ts"] = common.GetTimestamp()
	events, _ := common.GetContextKeyType[[]map[string]interface{}](c, constant.ContextKeyChannelFailoverTrace)
	events = append(events, compactChannelFailoverMap(event))
	common.SetContextKey(c, constant.ContextKeyChannelFailoverTrace, events)
}

func cloneChannelFailoverMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func compactChannelFailoverMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		case int:
			if typed == 0 {
				continue
			}
		case int64:
			if typed == 0 {
				continue
			}
		case []int:
			if len(typed) == 0 {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		case nil:
			continue
		}
		out[key] = value
	}
	return out
}

func sortedExcludedChannelIds(excluded map[int]bool) []int {
	ids := make([]int, 0, len(excluded))
	for id, ok := range excluded {
		if ok && id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

func remainingChannelRetries(retryParam *service.RetryParam) int {
	if retryParam == nil {
		return 0
	}
	remaining := common.RetryTimes - retryParam.GetRetry()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func channelFailoverRetryIndex(retryParam *service.RetryParam) int {
	if retryParam == nil {
		return 0
	}
	return retryParam.GetRetry()
}
