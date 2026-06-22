package openaicompat

import (
	"encoding/json"
	"testing"

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
	require.True(t, ShouldConvertResponsesToChat(constant.ChannelTypeSiliconFlow, ""))
	require.True(t, ShouldConvertResponsesToChat(constant.ChannelTypeOpenAI, ResponsesProtocolChatCompletions))
	require.False(t, ShouldConvertResponsesToChat(constant.ChannelTypeOpenAI, ""))
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

func TestResponsesRequestToChatSkipsHostedTools(t *testing.T) {
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

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
	require.Nil(t, chatReq.ToolChoice)
}

func TestResponsesRequestToChatDropsToolChoiceWhenAllToolsFiltered(t *testing.T) {
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

	chatReq, _, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, chatReq.Tools)
	require.Nil(t, chatReq.ToolChoice)
	require.Nil(t, chatReq.ParallelTooCalls)
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

func uintPtr(v uint) *uint {
	return &v
}
