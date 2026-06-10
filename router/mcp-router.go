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
}
