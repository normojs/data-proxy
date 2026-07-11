package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIOCopyBytesGracefullySkipsEmptyHeaderValues(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Header: http.Header{
			"X-Empty":  []string{},
			"X-Filled": []string{"ok"},
		},
	}

	require.NotPanics(t, func() {
		IOCopyBytesGracefully(ctx, resp, []byte(`{"ok":true}`))
	})
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, recorder.Header().Values("X-Empty"))
	require.Equal(t, "ok", recorder.Header().Get("X-Filled"))
}
