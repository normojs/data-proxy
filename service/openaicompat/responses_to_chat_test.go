package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatCompletionsResponsePreservesReasoning(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_reasoning",
		CreatedAt: 123,
		Model:     "gpt-5",
		Output: []dto.ResponsesOutput{
			{
				Type: "reasoning",
				ID:   "rs_1",
				Summary: []dto.ResponsesReasoningSummaryPart{
					{Type: "summary_text", Text: "Need to inspect the request."},
				},
			},
			{
				Type:   "message",
				ID:     "msg_1",
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "done"},
				},
			},
		},
	}

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_reasoning")
	require.NoError(t, err)
	require.Len(t, chat.Choices, 1)
	require.Equal(t, "done", chat.Choices[0].Message.Content)
	require.Equal(t, "Need to inspect the request.", chat.Choices[0].Message.GetReasoningContent())
}

func TestResponsesResponseToChatCompletionsResponsePreservesTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_text_tool",
		CreatedAt: 123,
		Model:     "gpt-5",
		Output: []dto.ResponsesOutput{
			{
				Type:   "message",
				ID:     "msg_1",
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "Let me check."},
				},
			},
			{
				Type:      "function_call",
				ID:        "fc_call_1",
				CallId:    "call_1",
				Name:      "lookup",
				Arguments: []byte(`{"q":"usd"}`),
			},
		},
	}

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_text_tool")
	require.NoError(t, err)
	msg := chat.Choices[0].Message
	require.Equal(t, "Let me check.", msg.Content)
	toolCalls := msg.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "lookup", toolCalls[0].Function.Name)
	require.JSONEq(t, `{"q":"usd"}`, toolCalls[0].Function.Arguments)
	require.Equal(t, "tool_calls", chat.Choices[0].FinishReason)
}
