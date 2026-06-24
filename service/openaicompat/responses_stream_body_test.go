package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesStreamBodyToResponseUsesCompletedEvent(t *testing.T) {
	body := []byte(`event: response.created
data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":123,"status":"in_progress","model":"gpt-5","output":[]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":123,"status":"completed","model":"gpt-5","output":[{"type":"message","id":"msg_1","status":"completed","role":"assistant","content":[{"type":"output_text","text":"done","annotations":[]}]}],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}

`)

	resp, ok, err := ResponsesStreamBodyToResponse(body)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "resp_1", resp.ID)
	require.Equal(t, "gpt-5", resp.Model)
	require.Len(t, resp.Output, 1)
	require.Equal(t, "done", resp.Output[0].Content[0].Text)
	require.NotNil(t, resp.Usage)
	require.Equal(t, 3, resp.Usage.TotalTokens)
}

func TestResponsesStreamBodyToResponseUsesFailedEvent(t *testing.T) {
	body := []byte(`event: response.failed
data: {"type":"response.failed","response":{"id":"resp_failed_fixture","object":"response","created_at":123,"status":"failed","model":"gpt-5","output":[],"error":{"type":"server_error","message":"fixture upstream failure"}}}

`)

	resp, ok, err := ResponsesStreamBodyToResponse(body)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "resp_failed_fixture", resp.ID)
	require.JSONEq(t, `"failed"`, string(resp.Status))
	require.NotNil(t, resp.Error)
}

func TestResponsesStreamBodyToResponseAggregatesTextWithoutCompletedEvent(t *testing.T) {
	body := []byte(`event: response.created
data: {"type":"response.created","response":{"id":"resp_2","object":"response","created_at":123,"status":"in_progress","model":"gpt-5","output":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"hello "}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"world"}

`)

	resp, ok, err := ResponsesStreamBodyToResponse(body)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "resp_2", resp.ID)
	require.Len(t, resp.Output, 1)
	require.Equal(t, "hello world", resp.Output[0].Content[0].Text)
}

func TestResponsesStreamBodyToResponseIgnoresProviderErrorJSON(t *testing.T) {
	body := []byte(`{"error":{"message":"fixture upstream error","type":"invalid_request_error"}}`)

	resp, ok, err := ResponsesStreamBodyToResponse(body)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, resp)
}

func TestChatCompletionsStreamBodyToResponsesAggregatesChatSSE(t *testing.T) {
	body := []byte(`data: {"id":"chatcmpl_1","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"hello "}}]}

data: {"id":"chatcmpl_1","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}

data: [DONE]

`)

	resp, usage, ok, err := ChatCompletionsStreamBodyToResponses(body, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "resp_chatcmpl_1", resp["id"])
	require.Equal(t, "completed", resp["status"])
	output := resp["output"].([]any)
	require.Len(t, output, 1)
	item := output[0].(map[string]any)
	require.Equal(t, "message", item["type"])
	content := item["content"].([]map[string]any)
	require.Equal(t, "hello world", content[0]["text"])
	require.Equal(t, 4, usage.InputTokens)
	require.Equal(t, 2, usage.OutputTokens)
	require.Equal(t, 6, usage.TotalTokens)
}

func TestChatCompletionsStreamBodyToResponsesHandlesEmptyStop(t *testing.T) {
	body := []byte(`data: {"id":"chatcmpl_empty","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":0,"total_tokens":4}}

data: [DONE]

`)

	resp, usage, ok, err := ChatCompletionsStreamBodyToResponses(body, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "completed", resp["status"])
	output := resp["output"].([]any)
	require.Empty(t, output)
	require.Equal(t, 4, usage.InputTokens)
	require.Equal(t, 0, usage.OutputTokens)
	require.Equal(t, 4, usage.TotalTokens)
}

func TestChatCompletionsStreamBodyToResponsesAggregatesDomesticToolCallSSE(t *testing.T) {
	body := []byte(`: keep-alive

data: {"id":"chatcmpl_tool_fixture","created":123,"model":"deepseek-ai/DeepSeek-V4-Flash","choices":[{"index":0,"delta":{"role":"assistant"}}]}

data: {"id":"chatcmpl_tool_fixture","created":123,"model":"deepseek-ai/DeepSeek-V4-Flash","choices":[{"index":0,"delta":{"reasoning_content":"Need a lookup before answering."}}]}

data: {"id":"chatcmpl_tool_fixture","created":123,"model":"deepseek-ai/DeepSeek-V4-Flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_lookup_rate","type":"function","function":{"name":"lookup_exchange_rate","arguments":"{\"pair\""}}]}}]}

data: {"id":"chatcmpl_tool_fixture","created":123,"model":"deepseek-ai/DeepSeek-V4-Flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"USD_CNY\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28}}

data: [DONE]

`)

	resp, usage, ok, err := ChatCompletionsStreamBodyToResponses(body, &dto.OpenAIResponsesRequest{Model: "deepseek-ai/DeepSeek-V4-Flash"}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, resp)
	require.Equal(t, "resp_chatcmpl_tool_fixture", resp["id"])
	require.Equal(t, "completed", resp["status"])
	output := resp["output"].([]any)
	require.Len(t, output, 2)

	reasoning := output[0].(map[string]any)
	require.Equal(t, "reasoning", reasoning["type"])
	summary := reasoning["summary"].([]map[string]any)
	require.Equal(t, "Need a lookup before answering.", summary[0]["text"])

	tool := output[1].(map[string]any)
	require.Equal(t, "function_call", tool["type"])
	require.Equal(t, "call_lookup_rate", tool["call_id"])
	require.Equal(t, "lookup_exchange_rate", tool["name"])
	require.JSONEq(t, `{"pair":"USD_CNY"}`, tool["arguments"].(string))
	require.Equal(t, 20, usage.InputTokens)
	require.Equal(t, 8, usage.OutputTokens)
	require.Equal(t, 28, usage.TotalTokens)
}

func TestChatCompletionsStreamBodyToResponsesIgnoresProviderErrorSSE(t *testing.T) {
	body := []byte(`event: error
data: {"error":{"message":"fixture provider overloaded","type":"server_error"}}

`)

	resp, usage, ok, err := ChatCompletionsStreamBodyToResponses(body, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, resp)
	require.Nil(t, usage)
}
