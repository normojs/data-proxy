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

func TestRelayNotImplementedRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)
	notImplementedHandler := runtime.FuncForPC(reflect.ValueOf(controller.RelayNotImplemented).Pointer()).Name()

	want := map[string]string{
		http.MethodPost + " /v1/images/variations":     notImplementedHandler,
		http.MethodGet + " /v1/files":                  notImplementedHandler,
		http.MethodPost + " /v1/files":                 notImplementedHandler,
		http.MethodDelete + " /v1/files/:id":           notImplementedHandler,
		http.MethodGet + " /v1/files/:id":              notImplementedHandler,
		http.MethodGet + " /v1/files/:id/content":      notImplementedHandler,
		http.MethodPost + " /v1/fine-tunes":            notImplementedHandler,
		http.MethodGet + " /v1/fine-tunes":             notImplementedHandler,
		http.MethodGet + " /v1/fine-tunes/:id":         notImplementedHandler,
		http.MethodPost + " /v1/fine-tunes/:id/cancel": notImplementedHandler,
		http.MethodGet + " /v1/fine-tunes/:id/events":  notImplementedHandler,
		http.MethodDelete + " /v1/models/:model":       notImplementedHandler,
	}

	registered := make(map[string]string)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}

	for route, handler := range want {
		require.Equal(t, handler, registered[route], "route %s should stay explicitly not implemented", route)
	}
}
