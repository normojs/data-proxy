package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetTunnelRouter(router *gin.Engine) {
	tunnelHTTPHandlers := []gin.HandlerFunc{
		middleware.RouteTag("tunnel_http"),
		middleware.SystemPerformanceCheck(),
		controller.TunnelHTTP,
	}
	router.Any("/t/:connection_key/tunnel/http/:slug", tunnelHTTPHandlers...)
	router.Any("/t/:connection_key/tunnel/http/:slug/*proxy_path", tunnelHTTPHandlers...)

	tunnelTCPHandlers := []gin.HandlerFunc{
		middleware.RouteTag("tunnel_tcp"),
		middleware.SystemPerformanceCheck(),
		controller.TunnelTCP,
	}
	router.GET("/t/:connection_key/tunnel/tcp/:slug", tunnelTCPHandlers...)
}
