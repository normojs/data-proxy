package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestPreservesAssistantReasoning(t *testing.T) {
	reasoning := "Need lookup."
	msg := dto.Message{
		Role:             "assistant",
		Content:          nil,
		ReasoningContent: &reasoning,
	}
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

	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-5",
		Messages: []dto.Message{msg},
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var items []map[string]any
	require.NoError(t, common.Unmarshal(responsesReq.Input, &items))
	require.Len(t, items, 3)
	require.Equal(t, "reasoning", items[0]["type"])
	require.Equal(t, reasoning, responseItemReasoningText(items[0]))
	require.Equal(t, "assistant", items[1]["role"])
	require.Equal(t, "function_call", items[2]["type"])
	require.Equal(t, "call_1", items[2]["call_id"])
	require.Equal(t, "lookup", items[2]["name"])
	require.Equal(t, reasoning, items[2]["reasoning_content"])
}
