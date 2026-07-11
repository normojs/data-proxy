package router

import (
	"net/http"
	"reflect"
	"runtime"
	"testing"

	"github.com/QuantumNous/new-api/controller"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIManagementProxyRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)
	managementHandler := runtime.FuncForPC(reflect.ValueOf(controller.RelayOpenAIManagement).Pointer()).Name()

	want := map[string]string{
		http.MethodPost + " /v1/images/variations":             managementHandler,
		http.MethodGet + " /v1/files":                          managementHandler,
		http.MethodPost + " /v1/files":                         managementHandler,
		http.MethodDelete + " /v1/files/:id":                   managementHandler,
		http.MethodGet + " /v1/files/:id":                      managementHandler,
		http.MethodGet + " /v1/files/:id/content":              managementHandler,
		http.MethodPost + " /v1/fine-tunes":                    managementHandler,
		http.MethodGet + " /v1/fine-tunes":                     managementHandler,
		http.MethodGet + " /v1/fine-tunes/:id":                 managementHandler,
		http.MethodPost + " /v1/fine-tunes/:id/cancel":         managementHandler,
		http.MethodGet + " /v1/fine-tunes/:id/events":          managementHandler,
		http.MethodDelete + " /v1/models/:model":               managementHandler,
		http.MethodPost + " /s/:slug/v1/images/variations":     managementHandler,
		http.MethodGet + " /s/:slug/v1/files":                  managementHandler,
		http.MethodPost + " /s/:slug/v1/files":                 managementHandler,
		http.MethodDelete + " /s/:slug/v1/files/:id":           managementHandler,
		http.MethodGet + " /s/:slug/v1/files/:id":              managementHandler,
		http.MethodGet + " /s/:slug/v1/files/:id/content":      managementHandler,
		http.MethodPost + " /s/:slug/v1/fine-tunes":            managementHandler,
		http.MethodGet + " /s/:slug/v1/fine-tunes":             managementHandler,
		http.MethodGet + " /s/:slug/v1/fine-tunes/:id":         managementHandler,
		http.MethodPost + " /s/:slug/v1/fine-tunes/:id/cancel": managementHandler,
		http.MethodGet + " /s/:slug/v1/fine-tunes/:id/events":  managementHandler,
		http.MethodDelete + " /s/:slug/v1/models/:model":       managementHandler,
	}

	registered := make(map[string]string)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}

	for route, handler := range want {
		require.Equal(t, handler, registered[route], "route %s should use OpenAI management proxy", route)
	}
}
