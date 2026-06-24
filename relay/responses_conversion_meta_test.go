package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/stretchr/testify/require"
)

func TestRecordResponsesProtocolDecisionRecordsNativeCapability(t *testing.T) {
	info := responsesDecisionTestInfo(constant.ChannelTypeOpenAI, "")

	recordResponsesProtocolDecision(info, responsesUpstreamProtocolNative, "native_responses")

	require.Equal(t, openaicompat.ResponsesProtocolAuto, info.RequestConversionMeta["responses_protocol"])
	require.Equal(t, responsesUpstreamProtocolNative, info.RequestConversionMeta["upstream_protocol"])
	require.Equal(t, "native_responses", info.RequestConversionMeta["responses_protocol_decision"])
	require.Equal(t, responsesChannelCapabilityNative, info.RequestConversionMeta["responses_channel_capability"])
	require.Equal(t, true, info.RequestConversionMeta["responses_native_supported"])
	require.Equal(t, false, info.RequestConversionMeta["responses_chat_preferred"])
	require.NotContains(t, info.RequestConversionMeta, "responses_reasoning_adapter_recommended")
}

func TestRecordResponsesProtocolDecisionRecordsChatCapability(t *testing.T) {
	info := responsesDecisionTestInfo(constant.ChannelTypeSiliconFlow, "")

	recordResponsesProtocolDecision(info, responsesUpstreamProtocolChat, "convert_to_chat_completions")

	require.Equal(t, openaicompat.ResponsesProtocolAuto, info.RequestConversionMeta["responses_protocol"])
	require.Equal(t, responsesUpstreamProtocolChat, info.RequestConversionMeta["upstream_protocol"])
	require.Equal(t, "convert_to_chat_completions", info.RequestConversionMeta["responses_protocol_decision"])
	require.Equal(t, responsesChannelCapabilityChat, info.RequestConversionMeta["responses_channel_capability"])
	require.Equal(t, false, info.RequestConversionMeta["responses_native_supported"])
	require.Equal(t, true, info.RequestConversionMeta["responses_chat_preferred"])
}

func TestRecordResponsesProtocolDecisionRecordsRecommendedReasoningAdapter(t *testing.T) {
	info := responsesDecisionTestInfo(constant.ChannelTypeDeepSeek, "")

	recordResponsesProtocolDecision(info, responsesUpstreamProtocolChat, "convert_to_chat_completions")

	require.Equal(t, responsesChannelCapabilityChat, info.RequestConversionMeta["responses_channel_capability"])
	require.Equal(t, openaicompat.ResponsesReasoningAdapterDeepSeek, info.RequestConversionMeta["responses_reasoning_adapter_recommended"])
}

func TestRecordResponsesProtocolDecisionSkipsAutoChecksForExplicitProtocol(t *testing.T) {
	info := responsesDecisionTestInfo(999999, openaicompat.ResponsesProtocolChatCompletions)

	recordResponsesProtocolDecision(info, responsesUpstreamProtocolChat, "convert_to_chat_completions")

	require.Equal(t, openaicompat.ResponsesProtocolChatCompletions, info.RequestConversionMeta["responses_protocol"])
	require.Equal(t, responsesChannelCapabilityUnknown, info.RequestConversionMeta["responses_channel_capability"])
	require.NotContains(t, info.RequestConversionMeta, "responses_native_supported")
	require.NotContains(t, info.RequestConversionMeta, "responses_chat_preferred")
	require.NotContains(t, info.RequestConversionMeta, "responses_reasoning_adapter_recommended")
}

func responsesDecisionTestInfo(channelType int, protocol string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: channelType,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ResponsesProtocol: protocol,
			},
		},
	}
}
