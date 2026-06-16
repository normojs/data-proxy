package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetUploadRouter(router *gin.Engine) {
	router.StaticFS("/uploads/system", gin.Dir(controller.SystemUploadDir(), false))

	uploadRoute := router.Group("/api/uploads")
	uploadRoute.Use(middleware.RouteTag("api"))
	uploadRoute.Use(middleware.GlobalAPIRateLimit())
	uploadRoute.Use(middleware.RootAuth())
	uploadRoute.Use(middleware.UploadRateLimit())
	{
		uploadRoute.POST("/system/logo", controller.UploadSystemLogo)
	}
}
