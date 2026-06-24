package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetTunnelRouterDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetTunnelRouter(engine)

	routes := engine.Routes()
	if len(routes) == 0 {
		t.Fatal("expected tunnel routes to be registered")
	}
}
