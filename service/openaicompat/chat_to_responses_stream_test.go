package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatToResponsesStreamConverterCapturesUsageOnlyChunk(t *testing.T) {
	content := "hello"
	finishReason := "stop"
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-usage",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
	)
	item := events[2].Payload["item"].(map[string]any)
	itemID := item["id"].(string)
	require.NotEmpty(t, itemID)
	require.Equal(t, itemID, events[3].Payload["item_id"])
	require.Equal(t, itemID, events[4].Payload["item_id"])

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-usage",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				FinishReason: &finishReason,
			},
		},
	})
	require.Empty(t, events)

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-usage",
		Created: 123,
		Model:   "deepseek-chat",
		Usage: &dto.Usage{
			PromptTokens:     7,
			CompletionTokens: 3,
			TotalTokens:      10,
		},
	})
	require.Empty(t, events)

	events = converter.Finish()
	requireEventNames(t, events,
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	)
	require.Equal(t, itemID, events[0].Payload["item_id"])
	require.Equal(t, itemID, events[1].Payload["item_id"])
	completed := events[len(events)-1]
	response, ok := completed.Payload["response"].(map[string]any)
	require.True(t, ok)
	usage, ok := response["usage"].(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 7, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, 10, usage.TotalTokens)
}

func TestChatToResponsesStreamConverterMapsLengthToIncomplete(t *testing.T) {
	content := "partial"
	finishReason := "length"
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-length",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
		}},
	})
	require.NotEmpty(t, events)

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-length",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index:        0,
			Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
			FinishReason: &finishReason,
		}},
	})
	require.Empty(t, events)

	events = converter.Finish()
	completed := events[len(events)-1]
	require.Equal(t, "response.completed", completed.Event)
	response := completed.Payload["response"].(map[string]any)
	require.Equal(t, "incomplete", response["status"])
	require.Equal(t, map[string]any{"reason": "max_output_tokens"}, response["incomplete_details"])
}

func TestChatToResponsesStreamConverterTreatsMissingFinishWithOutputAsIncomplete(t *testing.T) {
	content := "partial"
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-missing-finish",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
		}},
	})
	require.NotEmpty(t, events)

	events = converter.Finish()
	completed := events[len(events)-1]
	require.Equal(t, "response.completed", completed.Event)
	response := completed.Payload["response"].(map[string]any)
	require.Equal(t, "incomplete", response["status"])
	require.Equal(t, map[string]any{"reason": "max_output_tokens"}, response["incomplete_details"])
}

func TestChatToResponsesStreamConverterFailsMissingFinishWithoutOutput(t *testing.T) {
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.Finish()
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.failed",
	)
	response := events[len(events)-1].Payload["response"].(map[string]any)
	require.Equal(t, "failed", response["status"])
	errObj := response["error"].(map[string]any)
	require.Equal(t, "stream_truncated", errObj["type"])
}

func TestChatToResponsesStreamConverterUsesReasoningFallbackAsText(t *testing.T) {
	reasoning := "I need to inspect the current question first."
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-reasoning",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: &reasoning},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
	)
	require.Equal(t, reasoning, converter.EstimatedOutputText())

	events = converter.Finish()
	requireEventNames(t, events,
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	)
	require.Equal(t, reasoning, events[5].Payload["delta"])
	completed := events[len(events)-1]
	response, ok := completed.Payload["response"].(map[string]any)
	require.True(t, ok)
	output := response["output"].([]any)
	require.Len(t, output, 2)
	require.Equal(t, "reasoning", output[0].(map[string]any)["type"])
	require.Equal(t, "message", output[1].(map[string]any)["type"])
}

func TestChatToResponsesStreamConverterExtractsReasoningDetails(t *testing.T) {
	reasoningDetails := []any{
		map[string]any{"type": "reasoning.text", "text": "Inspect provider context."},
		map[string]any{"type": "reasoning.text", "delta": "Pick compatible answer path."},
	}
	content := "done"
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "openrouter/kimi"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-openrouter-reasoning-details",
		Created: 123,
		Model:   "openrouter/kimi",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningDetails: reasoningDetails,
				},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
	)
	require.Equal(t, "Inspect provider context.\nPick compatible answer path.", events[4].Payload["delta"])

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-openrouter-reasoning-details",
		Created: 123,
		Model:   "openrouter/kimi",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
			},
		},
	})
	requireEventNames(t, events,
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
	)

	events = converter.Finish()
	completed := events[len(events)-1]
	response := completed.Payload["response"].(map[string]any)
	output := response["output"].([]any)
	require.Len(t, output, 2)
	require.Equal(t, "reasoning", output[0].(map[string]any)["type"])
	require.Equal(t, "message", output[1].(map[string]any)["type"])
}

func TestChatToResponsesStreamConverterSplitsInlineThinkContent(t *testing.T) {
	content := "<think>plan</think>answer"
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-think",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
	)
	require.Equal(t, "plan", events[4].Payload["delta"])
	require.Equal(t, "answer", events[10].Payload["delta"])

	events = converter.Finish()
	completed := events[len(events)-1]
	response := completed.Payload["response"].(map[string]any)
	output := response["output"].([]any)
	require.Len(t, output, 2)
	require.Equal(t, "reasoning", output[0].(map[string]any)["type"])
	require.Equal(t, "message", output[1].(map[string]any)["type"])
}

func TestChatToResponsesStreamConverterSplitsPartialInlineThinkContent(t *testing.T) {
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	part1 := "<thi"
	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-think-partial",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &part1},
		}},
	})
	require.Empty(t, events)

	part2 := "nk>plan"
	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-think-partial",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &part2},
		}},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
	)
	require.Equal(t, "plan", events[4].Payload["delta"])

	part3 := "</think>answer"
	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-think-partial",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &part3},
		}},
	})
	requireEventNames(t, events,
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
	)
	require.Equal(t, "answer", events[5].Payload["delta"])
}

func TestChatToResponsesStreamConverterEmitsCustomToolDone(t *testing.T) {
	idx := 0
	args := `{"input":"hello"}`
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, &ResponsesToChatContext{
		ToolsByChatName: map[string]ResponseToolSpec{
			"run": {Kind: ResponseToolKindCustom, Name: "run", ChatName: "run"},
		},
	})

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-custom",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: &idx,
							ID:    "call_custom",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "run",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
	)

	events = converter.Finish()
	requireEventNames(t, events,
		"response.custom_tool_call_input.delta",
		"response.custom_tool_call_input.done",
		"response.output_item.done",
		"response.completed",
	)
	require.Equal(t, "hello", events[0].Payload["delta"])
	done := events[1]
	require.Equal(t, "hello", done.Payload["input"])
	response := events[len(events)-1].Payload["response"].(map[string]any)
	output := response["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	require.Equal(t, "custom_tool_call", item["type"])
	require.Equal(t, "hello", item["input"])
}

func TestChatToResponsesStreamConverterRestoresHostedToolCall(t *testing.T) {
	idx := 0
	args := `{"query":"人民币 美元 汇率"}`
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, &ResponsesToChatContext{
		ToolsByChatName: map[string]ResponseToolSpec{
			"web_search": {
				Kind:       ResponseToolKindHosted,
				Name:       "web_search",
				ChatName:   "web_search",
				HostedType: "web_search",
			},
		},
	})

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-hosted",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: &idx,
							ID:    "call_ws_1",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "web_search",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
	)
	added := events[2].Payload["item"].(map[string]any)
	require.Equal(t, "web_search_call", added["type"])

	events = converter.Finish()
	requireEventNames(t, events,
		"response.output_item.done",
		"response.completed",
	)
	done := events[0].Payload["item"].(map[string]any)
	require.Equal(t, "web_search_call", done["type"])
	require.Equal(t, "call_ws_1", done["call_id"])
	action := done["action"].(map[string]any)
	require.Equal(t, "人民币 美元 汇率", action["query"])
}

func TestChatToResponsesStreamConverterRestoresNamespaceToolCall(t *testing.T) {
	idx := 0
	args := `{"query":"usd cny"}`
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, &ResponsesToChatContext{
		ToolsByChatName: map[string]ResponseToolSpec{
			"web__search": {
				Kind:      ResponseToolKindFunction,
				Name:      "search",
				ChatName:  "web__search",
				Namespace: "web",
			},
		},
	})

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-namespace",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: &idx,
							ID:    "019e_ns_1",
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      "web__search",
								Arguments: args,
							},
						},
					},
				},
			},
		},
	})
	requireEventNames(t, events,
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.function_call_arguments.delta",
	)
	added := events[2].Payload["item"].(map[string]any)
	require.Equal(t, "function_call", added["type"])
	require.Equal(t, "web", added["namespace"])
	require.Equal(t, "search", added["name"])
	require.Equal(t, "call_019e_ns_1", added["call_id"])
	require.Equal(t, "", added["arguments"])

	events = converter.Finish()
	requireEventNames(t, events,
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	)
	done := events[1].Payload["item"].(map[string]any)
	require.Equal(t, "function_call", done["type"])
	require.Equal(t, "web", done["namespace"])
	require.Equal(t, "search", done["name"])
	require.Equal(t, "call_019e_ns_1", done["call_id"])
	require.JSONEq(t, args, done["arguments"].(string))
}

func TestChatToResponsesStreamConverterWaitsForStreamingToolName(t *testing.T) {
	idx := 0
	firstArgs := `{"q"`
	secondArgs := `:"usd"}`
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tool-late-name",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: &idx,
					ID:    "call_1",
					Type:  "function",
					Function: dto.FunctionResponse{
						Arguments: firstArgs,
					},
				}},
			},
		}},
	})
	requireEventNames(t, events, "response.created", "response.in_progress")

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tool-late-name",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: &idx,
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "lookup",
						Arguments: secondArgs,
					},
				}},
			},
		}},
	})
	requireEventNames(t, events,
		"response.output_item.added",
		"response.function_call_arguments.delta",
	)
	require.Equal(t, firstArgs+secondArgs, events[1].Payload["delta"])

	events = converter.Finish()
	requireEventNames(t, events,
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	)
	response := events[len(events)-1].Payload["response"].(map[string]any)
	output := response["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	require.Equal(t, "lookup", item["name"])
	require.JSONEq(t, firstArgs+secondArgs, item["arguments"].(string))
}

func TestChatToResponsesStreamConverterWaitsForStreamingToolID(t *testing.T) {
	idx := 0
	args := `{"q":"usd"}`
	converter := NewChatToResponsesStreamConverter(&dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)

	events := converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tool-late-id",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: &idx,
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "lookup",
						Arguments: args,
					},
				}},
			},
		}},
	})
	requireEventNames(t, events, "response.created", "response.in_progress")

	events = converter.HandleChunk(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tool-late-id",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: &idx,
					ID:    "call_late",
					Type:  "function",
				}},
			},
		}},
	})
	requireEventNames(t, events,
		"response.output_item.added",
		"response.function_call_arguments.delta",
	)
	added := events[0].Payload["item"].(map[string]any)
	require.Equal(t, "call_late", added["call_id"])
	require.Equal(t, args, events[1].Payload["delta"])

	events = converter.Finish()
	requireEventNames(t, events,
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	)
	doneItem := events[1].Payload["item"].(map[string]any)
	require.Equal(t, "call_late", doneItem["call_id"])
}

func requireEventNames(t *testing.T, events []ChatToResponsesStreamEvent, names ...string) {
	t.Helper()
	require.Len(t, events, len(names))
	for i, name := range names {
		require.Equal(t, name, events[i].Event)
	}
}
