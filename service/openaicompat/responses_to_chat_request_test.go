package openaicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func TestShouldConvertResponsesToChat(t *testing.T) {
	chatCompatChannels := []int{
		constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeDeepSeek,
		constant.ChannelTypeMoonshot,
		constant.ChannelTypeMistral,
		constant.ChannelTypeOpenRouter,
		constant.ChannelTypeZhipu_v4,
		constant.ChannelTypeBaiduV2,
	}
	for _, channelType := range chatCompatChannels {
		require.True(t, ShouldConvertResponsesToChat(channelType, ""), "channel type %d should auto-convert Responses to Chat Completions", channelType)
		require.False(t, ChannelSupportsNativeResponses(channelType), "channel type %d should not be marked native", channelType)
		require.True(t, ChannelPrefersChatResponsesCompatibility(channelType), "channel type %d should prefer chat compatibility", channelType)
	}

	nativeChannels := []int{
		constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeAli,
		constant.ChannelCloudflare,
		constant.ChannelTypeCodex,
		constant.ChannelTypePerplexity,
		constant.ChannelTypeSubmodel,
		constant.ChannelTypeVolcEngine,
		constant.ChannelTypeXai,
	}
	for _, channelType := range nativeChannels {
		require.False(t, ShouldConvertResponsesToChat(channelType, ""), "channel type %d should prefer native Responses", channelType)
		require.True(t, ChannelSupportsNativeResponses(channelType), "channel type %d should be marked native", channelType)
		require.False(t, ChannelPrefersChatResponsesCompatibility(channelType), "channel type %d should not prefer chat compatibility", channelType)
	}

	providerNativeResponseChannels := []int{
		constant.ChannelTypeMiniMax,
		constant.ChannelTypeOllama,
		constant.ChannelTypeTencent,
		constant.ChannelTypeZhipu,
		constant.ChannelTypeXunfei,
	}
	for _, channelType := range providerNativeResponseChannels {
		require.False(t, ShouldConvertResponsesToChat(channelType, ""), "channel type %d has provider-native chat responses and needs a dedicated Responses adapter", channelType)
		require.False(t, ChannelSupportsNativeResponses(channelType), "channel type %d should not be marked native", channelType)
		require.False(t, ChannelPrefersChatResponsesCompatibility(channelType), "channel type %d should not auto-convert through OpenAI Chat JSON", channelType)
	}

	require.True(t, ShouldConvertResponsesToChat(constant.ChannelTypeOpenAI, ResponsesProtocolChatCompletions))
	require.False(t, ShouldConvertResponsesToChat(constant.ChannelTypeSiliconFlow, ResponsesProtocolNative))
	require.True(t, IsResponsesProtocolDisabled(ResponsesProtocolDisabled))
}

func TestResponsesRequestToChatCompletionsRequest(t *testing.T) {
	stream := true
	req := &dto.OpenAIResponsesRequest{
		Model:        "deepseek-chat",
		Instructions: rawJSON(t, "You are terse."),
		Input: rawJSON(t, []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "hello"},
				},
			},
		}),
		Tools: rawJSON(t, []any{
			map[string]any{
				"type":        "function",
				"name":        "lookup",
				"description": "Lookup data",
				"parameters":  map[string]any{"type": "object"},
			},
		}),
		ToolChoice:      rawJSON(t, map[string]any{"type": "function", "name": "lookup"}),
		Stream:          &stream,
		MaxOutputTokens: uintPtr(128),
		Reasoning:       &dto.Reasoning{Effort: "high"},
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Equal(t, "deepseek-chat", chatReq.Model)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Equal(t, "You are terse.", chatReq.Messages[0].Content)
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
	require.Equal(t, "high", chatReq.ReasoningEffort)
	require.NotNil(t, chatReq.StreamOptions)
	require.True(t, chatReq.StreamOptions.IncludeUsage)
	require.Contains(t, ctx.ToolsByChatName, "lookup")
}

func TestResponsesRequestToChatReasoningAdapterDeepSeek(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "deepseek-ai/DeepSeek-V4-Flash",
		Input:     rawJSON(t, "hello"),
		Reasoning: &dto.Reasoning{Effort: "xhigh"},
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ReasoningAdapter: ResponsesReasoningAdapterDeepSeek,
	})
	require.NoError(t, err)
	require.Equal(t, "max", chatReq.ReasoningEffort)
	require.JSONEq(t, `{"type":"enabled"}`, string(chatReq.THINKING))
}

func TestResponsesRequestToChatReasoningAdapterOpenRouter(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "openrouter/model",
		Input:     rawJSON(t, "hello"),
		Reasoning: &dto.Reasoning{Effort: "max"},
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ReasoningAdapter: ResponsesReasoningAdapterOpenRouter,
	})
	require.NoError(t, err)
	require.Empty(t, chatReq.ReasoningEffort)
	require.JSONEq(t, `{"effort":"xhigh"}`, string(chatReq.Reasoning))
}

func TestResponsesRequestToChatReasoningAdapterOpenRouterNone(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "openrouter/model",
		Input:     rawJSON(t, "hello"),
		Reasoning: &dto.Reasoning{Effort: "none"},
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ReasoningAdapter: ResponsesReasoningAdapterOpenRouter,
	})
	require.NoError(t, err)
	require.Empty(t, chatReq.ReasoningEffort)
	require.JSONEq(t, `{"effort":"none"}`, string(chatReq.Reasoning))
}

func TestResponsesRequestToChatReasoningAdapterThinkingFlags(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "qwen",
		Input:     rawJSON(t, "hello"),
		Reasoning: &dto.Reasoning{Effort: "medium"},
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ReasoningAdapter: ResponsesReasoningAdapterQwenEnableThinking,
	})
	require.NoError(t, err)
	require.Empty(t, chatReq.ReasoningEffort)
	require.JSONEq(t, `true`, string(chatReq.EnableThinking))

	req.Reasoning.Effort = "off"
	chatReq, _, err = ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ReasoningAdapter: ResponsesReasoningAdapterMiniMaxReasoningSplit,
	})
	require.NoError(t, err)
	require.JSONEq(t, `false`, string(chatReq.ReasoningSplit))
}

func TestResponsesRequestToChatReasoningAdapterAutoInfersChannel(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:     "deepseek-reasoner",
		Input:     rawJSON(t, "hello"),
		Reasoning: &dto.Reasoning{Effort: "xhigh"},
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequestWithOptions(req, &ResponsesToChatOptions{
		ChannelType:      constant.ChannelTypeDeepSeek,
		ReasoningAdapter: ResponsesReasoningAdapterAuto,
	})
	require.NoError(t, err)
	require.Equal(t, "max", chatReq.ReasoningEffort)
}

func TestResponsesRequestToChatFiltersHostedTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: rawJSON(t, "hello"),
		Tools: rawJSON(t, []any{
			map[string]any{"type": "web_search"},
			map[string]any{
				"type":        "function",
				"name":        "lookup",
				"description": "Lookup data",
				"parameters":  map[string]any{"type": "object"},
			},
		}),
		ToolChoice: rawJSON(t, map[string]any{"type": "web_search"}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
	require.Nil(t, chatReq.ToolChoice)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Contains(t, common.Interface2String(chatReq.Messages[0].Content), "hosted tools (web_search)")
	requireJSONSubset(t, map[string]any{
		"hosted_tools_filtered":           []any{"web_search"},
		"hosted_tools_direct_answer_hint": true,
	}, ctx.RequestConversionMeta())
}

func TestResponsesRequestToChatClearsHostedToolChoice(t *testing.T) {
	parallelToolCalls := rawJSON(t, true)
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: rawJSON(t, "hello"),
		Tools: rawJSON(t, []any{
			map[string]any{"type": "web_search"},
		}),
		ToolChoice:        rawJSON(t, "auto"),
		ParallelToolCalls: parallelToolCalls,
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, chatReq.Tools)
	require.Nil(t, chatReq.ToolChoice)
	require.Nil(t, chatReq.ParallelTooCalls)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	requireJSONSubset(t, map[string]any{
		"hosted_tools_filtered":           []any{"web_search"},
		"hosted_tools_direct_answer_hint": true,
	}, ctx.RequestConversionMeta())
}

func TestResponsesRequestToChatRecordsUnsupportedFilteredTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: rawJSON(t, "hello"),
		Tools: rawJSON(t, []any{
			map[string]any{"type": "unsupported_tool"},
			map[string]any{
				"type": "namespace",
				"name": "workspace",
				"tools": []any{
					map[string]any{"type": "prompt", "name": "build_plan"},
				},
			},
		}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, chatReq.Tools)
	require.Equal(t, []string{"namespace.prompt", "unsupported_tool"}, ctx.RequestConversionMeta()["unsupported_tools_filtered"])
}

func TestResponsesRequestToChatUsesToolSearchOutputTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: rawJSON(t, []any{
			map[string]any{
				"type":    "tool_search_output",
				"call_id": "call_search",
				"tools": []any{
					map[string]any{
						"type":        "function",
						"name":        "search_docs",
						"description": "Search documentation.",
						"parameters":  map[string]any{"type": "object"},
					},
				},
			},
		}),
		ToolChoice: rawJSON(t, "auto"),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "search_docs", chatReq.Tools[0].Function.Name)
	require.Equal(t, "auto", chatReq.ToolChoice)
}

func TestResponsesRequestToChatRestoresPreviousToolCallFromHistory(t *testing.T) {
	ResetDefaultResponsesChatHistoryForTest()
	defer ResetDefaultResponsesChatHistoryForTest()
	DefaultResponsesChatHistory().RecordResponseMap(map[string]any{
		"id": "resp_1",
		"output": []any{
			map[string]any{
				"type":              "function_call",
				"call_id":           "call_1",
				"name":              "lookup",
				"arguments":         `{"q":"usd"}`,
				"reasoning_content": "Need lookup.",
			},
		},
	})

	req := &dto.OpenAIResponsesRequest{
		Model:              "deepseek-chat",
		PreviousResponseID: "resp_1",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
		}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Equal(t, "Need lookup.", chatReq.Messages[0].GetReasoningContent())
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "lookup", toolCalls[0].Function.Name)
	require.Equal(t, `{"q":"usd"}`, toolCalls[0].Function.Arguments)
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, "call_1", chatReq.Messages[1].ToolCallId)
	require.Equal(t, []string{"previous_response_id"}, ctx.RequestConversionMeta()["history_restore_sources"])
}

func TestResponsesRequestToChatRestoresPreviousToolCallByUniqueCallID(t *testing.T) {
	ResetDefaultResponsesChatHistoryForTest()
	defer ResetDefaultResponsesChatHistoryForTest()
	DefaultResponsesChatHistory().RecordResponseMap(map[string]any{
		"id": "resp_1",
		"output": []any{
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_unique",
				"name":      "lookup",
				"arguments": `{"q":"usd"}`,
			},
		},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call_output", "call_id": "call_unique", "output": "ok"},
		}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, []string{"unique_call_id"}, ctx.RequestConversionMeta()["history_restore_sources"])
}

func TestResponsesRequestToChatBackfillsToolCallReasoning(t *testing.T) {
	ResetDefaultResponsesChatHistoryForTest()
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": `{"q":"usd"}`},
		}),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "tool call", chatReq.Messages[0].GetReasoningContent())
}

func TestResponsesRequestToChatExtractsToolCallReasoningDetails(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": `{}`,
				"reasoning_details": []any{
					map[string]any{"type": "reasoning.text", "text": "Need lookup."},
				},
			},
		}),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "Need lookup.", chatReq.Messages[0].GetReasoningContent())
}

func TestResponsesRequestToChatDefersMessagesUntilToolOutput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": `{"q":"usd"}`},
			map[string]any{"type": "message", "role": "user", "content": "next question"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
		}),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 3)
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Len(t, chatReq.Messages[0].ParseToolCalls(), 1)
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, "call_1", chatReq.Messages[1].ToolCallId)
	require.Equal(t, "user", chatReq.Messages[2].Role)
	require.Equal(t, "next question", chatReq.Messages[2].Content)
}

func TestResponsesRequestToChatGroupsParallelToolCalls(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup_a", "arguments": `{ "a": 1 }`},
			map[string]any{"type": "function_call", "call_id": "call_2", "name": "lookup_b", "arguments": `{ "b": 2 }`},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "a"},
			map[string]any{"type": "function_call_output", "call_id": "call_2", "output": "b"},
		}),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 3)
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 2)
	require.Equal(t, "lookup_a", toolCalls[0].Function.Name)
	require.Equal(t, "lookup_b", toolCalls[1].Function.Name)
	require.JSONEq(t, `{"a":1}`, toolCalls[0].Function.Arguments)
	require.JSONEq(t, `{"b":2}`, toolCalls[1].Function.Arguments)
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, "tool", chatReq.Messages[2].Role)
}

func TestResponsesRequestToChatCanonicalizesArgumentsAndToolOutput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": `{ "b": 2, "a": 1 }`},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": `{ "z": 2, "a": 1 }`},
		}),
	}

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.JSONEq(t, `{"a":1,"b":2}`, toolCalls[0].Function.Arguments)
	require.JSONEq(t, `{"a":1,"z":2}`, chatReq.Messages[1].Content.(string))
}

func TestResponsesRequestToChatFlattensLongNamespaceToolName(t *testing.T) {
	namespace := strings.Repeat("namespace", 8)
	childName := strings.Repeat("lookup", 8)
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Tools: rawJSON(t, []any{
			map[string]any{
				"type": "namespace",
				"name": namespace,
				"tools": []any{
					map[string]any{
						"type":        "function",
						"name":        childName,
						"description": "Lookup data",
						"parameters":  map[string]any{"type": "object"},
					},
				},
			},
		}),
		Input: rawJSON(t, []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "namespace": namespace, "name": childName, "arguments": `{}`},
		}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	chatName := chatReq.Tools[0].Function.Name
	require.LessOrEqual(t, len(chatName), 64)
	require.NotEqual(t, namespace+"__"+childName, chatName)
	require.Contains(t, ctx.ToolsByChatName, chatName)
	require.Equal(t, namespace, ctx.ToolsByChatName[chatName].Namespace)
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, chatName, toolCalls[0].Function.Name)
}

func TestResponsesRequestToChatConvertsHostedCallInput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: rawJSON(t, []any{
			map[string]any{
				"type":    "web_search_call",
				"id":      "ws_1",
				"call_id": "call_ws_1",
				"status":  "completed",
				"action": map[string]any{
					"type":  "search",
					"query": "人民币 美元 汇率",
				},
			},
		}),
		Tools: rawJSON(t, []any{
			map[string]any{"type": "web_search"},
		}),
	}

	chatReq, ctx, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Equal(t, "assistant", chatReq.Messages[1].Role)
	require.Contains(t, common.Interface2String(chatReq.Messages[1].Content), "hosted Responses tool call (web_search)")
	require.Contains(t, common.Interface2String(chatReq.Messages[1].Content), "人民币 美元 汇率")
	require.Empty(t, chatReq.Messages[1].ParseToolCalls())
	requireJSONSubset(t, map[string]any{
		"hosted_tools_filtered":           []any{"web_search"},
		"hosted_tools_direct_answer_hint": true,
	}, ctx.RequestConversionMeta())
}

func TestChatCompletionResponseToResponses(t *testing.T) {
	ctx := &ResponsesToChatContext{ToolsByChatName: map[string]ResponseToolSpec{
		"lookup": {Kind: ResponseToolKindFunction, Name: "lookup", ChatName: "lookup"},
	}}
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl-test",
		Model:   "deepseek-chat",
		Created: float64(123),
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{Role: "assistant", Content: "done"},
			},
		},
		Usage: dto.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
	}

	resp, usage, err := ChatCompletionResponseToResponses(chat, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, ctx)
	require.NoError(t, err)
	require.Equal(t, "resp_test", resp["id"])
	require.Equal(t, 10, usage.InputTokens)
	require.Equal(t, 2, usage.OutputTokens)
	output := resp["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	require.Equal(t, "message", item["type"])
}

func TestChatCompletionResponseToResponsesMapsLengthToIncomplete(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl-length",
		Model: "deepseek-chat",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      dto.Message{Role: "assistant", Content: "partial"},
				FinishReason: "length",
			},
		},
	}

	resp, _, err := ChatCompletionResponseToResponses(chat, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	require.Equal(t, "incomplete", resp["status"])
	require.Equal(t, map[string]any{"reason": "max_output_tokens"}, resp["incomplete_details"])
}

func TestChatCompletionResponseToResponsesPreservesTextAndToolCalls(t *testing.T) {
	msg := dto.Message{Role: "assistant", Content: "Let me check."}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   "call_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "lookup",
				Arguments: `{"q":"usd"}`,
			},
		},
	})
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl-text-tool",
		Model:   "deepseek-chat",
		Created: float64(123),
		Choices: []dto.OpenAITextResponseChoice{
			{Message: msg},
		},
	}

	resp, _, err := ChatCompletionResponseToResponses(chat, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	output := resp["output"].([]any)
	require.Len(t, output, 2)
	require.Equal(t, "message", output[0].(map[string]any)["type"])
	require.Equal(t, "function_call", output[1].(map[string]any)["type"])
}

func TestChatCompletionResponseToResponsesRestoresHostedToolCall(t *testing.T) {
	msg := dto.Message{Role: "assistant", Content: nil}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   "call_ws_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "web_search",
				Arguments: `{"query":"人民币 美元 汇率"}`,
			},
		},
	})
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl-web-search",
		Model: "deepseek-chat",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: msg},
		},
	}
	ctx := &ResponsesToChatContext{ToolsByChatName: map[string]ResponseToolSpec{
		"web_search": {
			Kind:       ResponseToolKindHosted,
			Name:       "web_search",
			ChatName:   "web_search",
			HostedType: "web_search",
		},
	}}

	resp, _, err := ChatCompletionResponseToResponses(chat, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, ctx)
	require.NoError(t, err)
	output := resp["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	require.Equal(t, "web_search_call", item["type"])
	require.Equal(t, "call_ws_1", item["call_id"])
	action := item["action"].(map[string]any)
	require.Equal(t, "search", action["type"])
	require.Equal(t, "人民币 美元 汇率", action["query"])
}

func TestChatCompletionResponseToResponsesReasoningFallback(t *testing.T) {
	reasoning := "当前时间需要由客户端环境提供。"
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl-reasoning",
		Model:   "deepseek-chat",
		Created: float64(123),
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:             "assistant",
					ReasoningContent: &reasoning,
				},
			},
		},
	}

	resp, _, err := ChatCompletionResponseToResponses(chat, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	require.Equal(t, reasoning, responseOutputText(t, resp))
}

func TestChatStreamDeltaOutputTextReasoningFallback(t *testing.T) {
	reasoning := "现在是测试时间。"
	require.Equal(t, reasoning, ChatStreamDeltaOutputText(dto.ChatCompletionsStreamResponseChoiceDelta{
		ReasoningContent: &reasoning,
	}))

	content := "优先显示正文。"
	require.Equal(t, content, ChatStreamDeltaOutputText(dto.ChatCompletionsStreamResponseChoiceDelta{
		Content:          &content,
		ReasoningContent: &reasoning,
	}))
}

func responseOutputText(t *testing.T, resp map[string]any) string {
	t.Helper()
	output := resp["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	content := item["content"].([]map[string]any)
	require.Len(t, content, 1)
	text, ok := content[0]["text"].(string)
	require.True(t, ok)
	return text
}

func uintPtr(v uint) *uint {
	return &v
}
