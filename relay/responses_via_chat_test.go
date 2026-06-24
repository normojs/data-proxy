package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsStreamToResponsesEmitsToolCallLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_lookup","type":"function","function":{"name":"lookup","arguments":"{\"query\""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"usd cny\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":123,"model":"deepseek-chat","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	stream := true
	usage, newAPIError := chatCompletionsStreamToResponsesHandler(
		c,
		&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-chat"},
		},
		&http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		},
		&dto.OpenAIResponsesRequest{Model: "deepseek-chat", Stream: &stream},
		&openaicompat.ResponsesToChatContext{
			ToolsByChatName: map[string]openaicompat.ResponseToolSpec{
				"lookup": {
					Kind:     openaicompat.ResponseToolKindFunction,
					Name:     "lookup",
					ChatName: "lookup",
				},
			},
		},
	)

	require.Nil(t, newAPIError)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)

	out := w.Body.String()
	require.Contains(t, out, "event: response.output_item.added")
	require.Contains(t, out, "event: response.function_call_arguments.delta")
	require.Contains(t, out, "event: response.function_call_arguments.done")
	require.Contains(t, out, "event: response.output_item.done")
	require.Contains(t, out, "event: response.completed")
	require.Contains(t, out, `"name":"lookup"`)
	require.Contains(t, out, `"arguments":"{\"query\":\"usd cny\"}"`)
}

func TestCompactTextAndUsageFromChatBody(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl-compact",
		"object":"chat.completion",
		"created":123,
		"model":"deepseek-chat",
		"choices":[
			{"index":0,"message":{"role":"assistant","content":"compact summary"},"finish_reason":"stop"}
		],
		"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}
	}`)

	text, usage, newAPIError := compactTextAndUsageFromChatBody(body, &dto.OpenAIResponsesRequest{Model: "deepseek-chat"}, nil, http.StatusOK)

	require.Nil(t, newAPIError)
	require.Equal(t, "compact summary", text)
	require.NotNil(t, usage)
	require.Equal(t, 7, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, 10, usage.TotalTokens)
}

func TestCompactOutputMessage(t *testing.T) {
	raw := compactOutputMessage("compact summary")

	var output []map[string]any
	require.NoError(t, json.Unmarshal(raw, &output))
	require.Len(t, output, 1)
	require.Equal(t, "message", output[0]["type"])
	require.Equal(t, "assistant", output[0]["role"])
	require.Equal(t, "completed", output[0]["status"])

	content, ok := output[0]["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	part, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "output_text", part["type"])
	require.Equal(t, "compact summary", part["text"])
}
