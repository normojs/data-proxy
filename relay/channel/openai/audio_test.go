package openai

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

func TestOpenaiTTSHandlerSkipsEmptyHeaderValues(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Empty":  []string{},
			"X-Filled": []string{"ok"},
		},
		Body: io.NopCloser(strings.NewReader("audio")),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	require.NotPanics(t, func() {
		usage := OpenaiTTSHandler(ctx, resp, info)
		require.NotNil(t, usage)
	})
	require.Empty(t, recorder.Header().Values("X-Empty"))
	require.Equal(t, "ok", recorder.Header().Get("X-Filled"))
}
