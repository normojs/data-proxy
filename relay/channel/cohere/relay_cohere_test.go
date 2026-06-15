package cohere

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyCohereStreamUsageSetsTotalTokens(t *testing.T) {
	t.Parallel()

	usage := &dto.Usage{}
	ok := applyCohereStreamUsage(usage, &CohereResponseResult{
		Meta: CohereMeta{
			BilledUnits: CohereBilledUnits{
				InputTokens:  12,
				OutputTokens: 5,
			},
		},
	})

	require.True(t, ok)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 17, usage.TotalTokens)
}

func TestFinalizeCohereStreamUsageFallsBackWithoutMetadata(t *testing.T) {
	t.Parallel()

	c := newCohereTestContext(t)
	usage := finalizeCohereStreamUsage(c, &dto.Usage{}, "hello from cohere", "command-r", 9)

	require.Equal(t, 9, usage.PromptTokens)
	require.Greater(t, usage.CompletionTokens, 0)
	require.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
	require.True(t, c.GetBool(string(constant.ContextKeyLocalCountTokens)))
}

func TestFinalizeCohereStreamUsageFillsMissingPromptTokens(t *testing.T) {
	t.Parallel()

	c := newCohereTestContext(t)
	usage := finalizeCohereStreamUsage(c, &dto.Usage{CompletionTokens: 4}, "partial metadata", "command-r", 11)

	require.Equal(t, 11, usage.PromptTokens)
	require.Equal(t, 4, usage.CompletionTokens)
	require.Equal(t, 15, usage.TotalTokens)
}

func TestCohereStreamHandlerUsesFinalUsageAndEmitsUsageChunk(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := newCohereStreamRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "cohere-stream-test")

	body := strings.Join([]string{
		`{"event_type":"text-generation","is_finished":false,"text":"hello"}`,
		`{"event_type":"text-generation","is_finished":false,"text":" world"}`,
		`{"event_type":"stream-end","is_finished":true,"finish_reason":"COMPLETE","response":{"response_id":"chat_123","finish_reason":"COMPLETE","text":"hello world","meta":{"billed_units":{"input_tokens":7,"output_tokens":3}}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ShouldIncludeUsage: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "command-r",
		},
	}

	usage, apiErr := cohereStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.Equal(t, &dto.Usage{
		PromptTokens:     7,
		CompletionTokens: 3,
		TotalTokens:      10,
	}, usage)

	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"content":"hello"`)
	require.Contains(t, responseBody, `"content":" world"`)
	require.Contains(t, responseBody, `"finish_reason":"stop"`)
	require.Contains(t, responseBody, `"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10`)
	require.Contains(t, responseBody, "data: [DONE]")
	require.Less(t, strings.Index(responseBody, `"usage":{"prompt_tokens":7`), strings.Index(responseBody, "data: [DONE]"))
}

type cohereStreamRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func newCohereStreamRecorder() *cohereStreamRecorder {
	return &cohereStreamRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify:      make(chan bool),
	}
}

func (r *cohereStreamRecorder) CloseNotify() <-chan bool {
	return r.closeNotify
}

func newCohereTestContext(t *testing.T) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}
