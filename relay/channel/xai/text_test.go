package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestXAIStreamHandlerStopsOnNullChunk(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("data: null\n\ndata: [DONE]\n\n")),
	}
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-test",
		},
	}

	usage, apiErr := xAIStreamHandler(ctx, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	require.NotEqual(t, relaycommon.StreamEndReasonPanic, info.StreamStatus.EndReason)
	messages, count := info.StreamStatus.ErrorMessages()
	require.Equal(t, 1, count)
	require.Len(t, messages, 1)
	require.Contains(t, messages[0], "xai stream response is empty")
}
