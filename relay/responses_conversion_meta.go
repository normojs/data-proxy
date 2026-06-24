package relay

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

const (
	responsesUpstreamProtocolNative = "responses"
	responsesUpstreamProtocolChat   = "chat_completions"

	responsesChannelCapabilityNative  = "native_responses"
	responsesChannelCapabilityChat    = "chat_completions_compat"
	responsesChannelCapabilityUnknown = "unknown"
)

func recordResponsesProtocolDecision(info *relaycommon.RelayInfo, upstreamProtocol string, decision string) {
	if info == nil {
		return
	}
	protocol := openaicompat.NormalizeResponsesProtocol(info.ChannelOtherSettings.ResponsesProtocol)
	nativeSupported := openaicompat.ChannelSupportsNativeResponses(info.ChannelType)
	chatPreferred := openaicompat.ChannelPrefersChatResponsesCompatibility(info.ChannelType)

	info.SetRequestConversionMeta("responses_protocol", protocol)
	if upstreamProtocol != "" {
		info.SetRequestConversionMeta("upstream_protocol", upstreamProtocol)
	}
	if decision != "" {
		info.SetRequestConversionMeta("responses_protocol_decision", decision)
	}
	info.SetRequestConversionMeta("responses_channel_capability", responsesChannelCapability(nativeSupported, chatPreferred))
	if adapter := openaicompat.InferResponsesReasoningAdapter(info.ChannelType); adapter != openaicompat.ResponsesReasoningAdapterLegacy {
		info.SetRequestConversionMeta("responses_reasoning_adapter_recommended", adapter)
	}
	if protocol == openaicompat.ResponsesProtocolAuto {
		info.SetRequestConversionMeta("responses_native_supported", nativeSupported)
		info.SetRequestConversionMeta("responses_chat_preferred", chatPreferred)
	}
}

func responsesChannelCapability(nativeSupported bool, chatPreferred bool) string {
	if chatPreferred {
		return responsesChannelCapabilityChat
	}
	if nativeSupported {
		return responsesChannelCapabilityNative
	}
	return responsesChannelCapabilityUnknown
}

func mergeResponsesCompatMeta(info *relaycommon.RelayInfo, ctx *openaicompat.ResponsesToChatContext) {
	if info == nil || ctx == nil {
		return
	}
	info.MergeRequestConversionMeta(ctx.RequestConversionMeta())
}

func recordResponsesTerminalStatus(info *relaycommon.RelayInfo, response map[string]any) {
	if info == nil || response == nil {
		return
	}
	status := strings.TrimSpace(interfaceString(response["status"]))
	if status == "" {
		return
	}
	info.SetRequestConversionMeta("responses_terminal_status", status)
	if status == "incomplete" {
		if details, ok := response["incomplete_details"]; ok {
			info.SetRequestConversionMeta("responses_incomplete_details", details)
		}
	}
	if status == "failed" {
		if err, ok := response["error"]; ok {
			info.SetRequestConversionMeta("responses_terminal_error", err)
		}
	}
}

func interfaceString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
