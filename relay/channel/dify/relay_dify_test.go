package dify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func TestStreamResponseDify2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponseDify2OpenAI(DifyChunkChatCompletionResponse{Event: "message", Answer: "hello"}, "dify-app")

	if got.Model != "dify-app" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
	if len(got.Choices) != 1 || got.Choices[0].Delta.GetContentString() != "hello" {
		t.Fatalf("unexpected choices: %#v", got.Choices)
	}
}

func TestStreamResponseDify2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := streamResponseDify2OpenAI(DifyChunkChatCompletionResponse{}, "")

	if got.Model != "dify" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestDifyHandlerUsesUpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := `{"conversation_id":"conv-1","answer":"hello","metadata":{"usage":{"total_tokens":2}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := difyHandler(ctx, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "dify-app"},
	}, resp)

	if apiErr != nil {
		t.Fatalf("unexpected api error: %v", apiErr)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v, want total tokens 2", usage)
	}
	var got dto.OpenAITextResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Model != "dify-app" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
