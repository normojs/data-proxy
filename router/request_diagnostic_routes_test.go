package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestDiagnosticRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		http.MethodGet + " /api/log/request-diagnostic-candidates",
		http.MethodGet + " /api/log/request-diagnostic",
		http.MethodPost + " /api/log/request-diagnostic",
		http.MethodPost + " /api/log/request-capture/cleanup",
		http.MethodGet + " /api/log/request/:request_id/diagnostic",
		http.MethodPost + " /api/log/request/:request_id/diagnostic",
		http.MethodGet + " /api/log/request/:request_id/diagnostic/bundle",
	} {
		require.True(t, registered[route], "route %s should be registered", route)
	}
}
