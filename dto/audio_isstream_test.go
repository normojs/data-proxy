package dto

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAudioRequestIsStreamUsesStreamFormatSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newAudioRequestTestContext()

	tests := []struct {
		name         string
		streamFormat string
		expected     bool
	}{
		{name: "sse stream format", streamFormat: "sse", expected: true},
		{name: "empty stream format", streamFormat: "", expected: false},
		{name: "json stream format", streamFormat: "json", expected: false},
		{name: "case sensitive stream format", streamFormat: "SSE", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AudioRequest{StreamFormat: tt.streamFormat}
			require.Equal(t, tt.expected, req.IsStream(c))
		})
	}
}

func TestAudioRequestIsStreamIgnoresBooleanStreamField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newAudioRequestTestContext()

	var req AudioRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"tts-1","input":"hello","voice":"alloy","stream":true}`), &req))

	require.Empty(t, req.StreamFormat)
	require.False(t, req.IsStream(c))
}

func newAudioRequestTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	return c
}
