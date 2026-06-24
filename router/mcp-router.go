package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetMCPRouter(router *gin.Engine) {
	mcpRouter := router.Group("/mcp")
	mcpRouter.Use(middleware.RouteTag("mcp"))
	mcpRouter.Use(middleware.SystemPerformanceCheck())
	mcpRouter.Use(middleware.TokenAuth())
	{
		mcpRouter.POST("", controller.MCP)
		mcpRouter.POST("/", controller.MCP)
		mcpRouter.POST("/v1", controller.MCP)
	}

	tunnelMCPRouter := router.Group("/t/:connection_key/tunnel/mcp/:slug")
	tunnelMCPRouter.Use(middleware.RouteTag("tunnel_mcp"))
	tunnelMCPRouter.Use(middleware.SystemPerformanceCheck())
	tunnelMCPRouter.Use(middleware.TokenAuth())
	{
		tunnelMCPRouter.GET("", controller.TunnelMCPSSE)
		tunnelMCPRouter.GET("/", controller.TunnelMCPSSE)
		tunnelMCPRouter.GET("/v1", controller.TunnelMCPSSE)
		tunnelMCPRouter.POST("", controller.TunnelMCP)
		tunnelMCPRouter.POST("/", controller.TunnelMCP)
		tunnelMCPRouter.POST("/v1", controller.TunnelMCP)
		tunnelMCPRouter.DELETE("", controller.TunnelMCPDelete)
		tunnelMCPRouter.DELETE("/", controller.TunnelMCPDelete)
		tunnelMCPRouter.DELETE("/v1", controller.TunnelMCPDelete)
		tunnelMCPRouter.POST("/message", controller.TunnelMCPMessage)
		tunnelMCPRouter.POST("/v1/message", controller.TunnelMCPMessage)
	}
}
