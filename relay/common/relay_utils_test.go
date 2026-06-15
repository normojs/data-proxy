package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAPIVersionPrefersQueryParameter(t *testing.T) {
	c := newRelayUtilsTestContext("/v1/chat/completions?api-version=from-query")
	c.Set("api_version", "from-context")

	require.Equal(t, "from-query", GetAPIVersion(c))
}

func TestGetAPIVersionFallsBackToContext(t *testing.T) {
	c := newRelayUtilsTestContext("/v1/chat/completions")
	c.Set("api_version", "from-context")

	require.Equal(t, "from-context", GetAPIVersion(c))
}

func newRelayUtilsTestContext(target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, target, nil)
	return c
}
