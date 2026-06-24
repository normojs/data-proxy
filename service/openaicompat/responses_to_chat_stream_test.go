package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesToChatStreamConverterStreamsTextAndUsage(t *testing.T) {
	converter := NewResponsesToChatStreamConverter("chatcmpl_1", 100, "gpt-5")

	chunks := converter.HandleEvent(dto.ResponsesStreamResponse{
		Type:  "response.output_text.delta",
		Delta: "hello",
	})
	require.Len(t, chunks, 2)
	require.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	require.Equal(t, "", chunks[0].Choices[0].Delta.GetContentString())
	require.Equal(t, "hello", chunks[1].Choices[0].Delta.GetContentString())

	chunks = converter.HandleEvent(dto.ResponsesStreamResponse{
		Type: "response.completed",
		Response: &dto.OpenAIResponsesResponse{
			Model:     "gpt-5",
			CreatedAt: 123,
			Usage: &dto.Usage{
				InputTokens:  4,
				OutputTokens: 2,
				TotalTokens:  6,
			},
		},
	})
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	require.Equal(t, "stop", *chunks[0].Choices[0].FinishReason)
	require.Equal(t, int64(123), chunks[0].Created)
	require.Equal(t, 4, converter.Usage().PromptTokens)
	require.Equal(t, 2, converter.Usage().CompletionTokens)
	require.Equal(t, 6, converter.Usage().TotalTokens)
}

func TestResponsesToChatStreamConverterStreamsFunctionCall(t *testing.T) {
	converter := NewResponsesToChatStreamConverter("chatcmpl_tool", 100, "gpt-5")

	chunks := converter.HandleEvent(dto.ResponsesStreamResponse{
		Type: "response.output_item.added",
		Item: &dto.ResponsesOutput{
			Type:   "function_call",
			ID:     "fc_1",
			CallId: "call_1",
			Name:   "lookup",
		},
	})
	require.Len(t, chunks, 2)
	require.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	tool := chunks[1].Choices[0].Delta.ToolCalls[0]
	require.Equal(t, "call_1", tool.ID)
	require.Equal(t, "lookup", tool.Function.Name)
	require.Equal(t, "", tool.Function.Arguments)

	chunks = converter.HandleEvent(dto.ResponsesStreamResponse{
		Type:   "response.function_call_arguments.delta",
		ItemID: "fc_1",
		Delta:  `{"q":"usd"}`,
	})
	require.Len(t, chunks, 1)
	tool = chunks[0].Choices[0].Delta.ToolCalls[0]
	require.Equal(t, "call_1", tool.ID)
	require.Empty(t, tool.Function.Name)
	require.JSONEq(t, `{"q":"usd"}`, tool.Function.Arguments)

	chunks = converter.HandleEvent(dto.ResponsesStreamResponse{Type: "response.completed"})
	require.Len(t, chunks, 1)
	require.NotNil(t, chunks[0].Choices[0].FinishReason)
	require.Equal(t, "tool_calls", *chunks[0].Choices[0].FinishReason)
}
