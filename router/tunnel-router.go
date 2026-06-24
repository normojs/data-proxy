package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetTunnelRouter(router *gin.Engine) {
	tunnelHTTPRouter := router.Group("/t/:connection_key/tunnel/http/:slug")
	tunnelHTTPRouter.Use(middleware.RouteTag("tunnel_http"))
	tunnelHTTPRouter.Use(middleware.SystemPerformanceCheck())
	{
		tunnelHTTPRouter.Any("", controller.TunnelHTTP)
		tunnelHTTPRouter.Any("/", controller.TunnelHTTP)
		tunnelHTTPRouter.Any("/*proxy_path", controller.TunnelHTTP)
	}
}
