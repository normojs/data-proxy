package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetBridgeRouter(router *gin.Engine) {
	bridgeRouter := router.Group("/bridge")
	bridgeRouter.Use(middleware.RouteTag("bridge"))
	bridgeRouter.Use(middleware.TokenAuth())
	{
		bridgeRouter.GET("/ws", controller.BridgeWebSocket)
	}
}
