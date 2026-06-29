package router

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func SetRelayRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	router.Use(middleware.DecompressRequestMiddleware())
	router.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	router.Use(middleware.StatsMiddleware())
	// https://platform.openai.com/docs/api-reference/introduction
	modelsRouter := router.Group("/v1/models")
	modelsRouter.Use(middleware.RouteTag("relay"))
	modelsRouter.Use(middleware.TokenAuth())
	modelsRouter.Use(middleware.MainSiteTokenScopeAuth())
	modelsRouter.Use(middleware.ConnectedAppScopeAuth())
	{
		modelsRouter.GET("", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.ListModels(c, constant.ChannelTypeAnthropic)
			case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "": // 单独的适配
				controller.RetrieveModel(c, constant.ChannelTypeGemini)
			default:
				controller.ListModels(c, constant.ChannelTypeOpenAI)
			}
		})

		modelsRouter.GET("/:model", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
			default:
				controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
			}
		})
	}

	geminiRouter := router.Group("/v1beta/models")
	geminiRouter.Use(middleware.RouteTag("relay"))
	geminiRouter.Use(middleware.TokenAuth())
	geminiRouter.Use(middleware.MainSiteTokenScopeAuth())
	geminiRouter.Use(middleware.ConnectedAppScopeAuth())
	{
		geminiRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeGemini)
		})
	}

	geminiCompatibleRouter := router.Group("/v1beta/openai/models")
	geminiCompatibleRouter.Use(middleware.RouteTag("relay"))
	geminiCompatibleRouter.Use(middleware.TokenAuth())
	geminiCompatibleRouter.Use(middleware.MainSiteTokenScopeAuth())
	geminiCompatibleRouter.Use(middleware.ConnectedAppScopeAuth())
	{
		geminiCompatibleRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeOpenAI)
		})
	}

	playgroundRouter := router.Group("/pg")
	playgroundRouter.Use(middleware.RouteTag("relay"))
	playgroundRouter.Use(middleware.SystemPerformanceCheck())
	playgroundRouter.Use(middleware.UserAuth(), middleware.Distribute())
	{
		playgroundRouter.POST("/chat/completions", controller.Playground)
	}
	relayV1Router := router.Group("/v1")
	relayV1Router.Use(middleware.RouteTag("relay"))
	relayV1Router.Use(middleware.SystemPerformanceCheck())
	relayV1Router.Use(middleware.TokenAuth())
	relayV1Router.Use(middleware.MainSiteTokenScopeAuth())
	relayV1Router.Use(middleware.ConnectedAppScopeAuth())
	relayV1Router.Use(middleware.ModelRequestRateLimit())
	{
		// WebSocket 路由（统一到 Relay）
		wsRouter := relayV1Router.Group("")
		wsRouter.Use(middleware.Distribute())
		wsRouter.GET("/realtime", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIRealtime)
		})
	}
	{
		//http router
		httpRouter := relayV1Router.Group("")
		httpRouter.Use(middleware.Distribute())

		// claude related routes
		httpRouter.POST("/messages", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatClaude)
		})

		// chat related routes
		httpRouter.POST("/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})
		httpRouter.POST("/chat/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// response related routes
		httpRouter.POST("/responses", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponses)
		})
		httpRouter.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})

		// image related routes
		httpRouter.POST("/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		httpRouter.POST("/images/generations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		httpRouter.POST("/images/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})

		// embedding related routes
		httpRouter.POST("/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatEmbedding)
		})

		// audio related routes
		httpRouter.POST("/audio/transcriptions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/translations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/speech", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})

		// rerank related routes
		httpRouter.POST("/rerank", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatRerank)
		})

		// gemini relay routes
		httpRouter.POST("/engines/:model/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
		httpRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})

		// other relay routes
		httpRouter.POST("/moderations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// not implemented
		httpRouter.POST("/images/variations", controller.RelayNotImplemented)
		httpRouter.GET("/files", controller.RelayNotImplemented)
		httpRouter.POST("/files", controller.RelayNotImplemented)
		httpRouter.DELETE("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id/content", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes/:id/cancel", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id/events", controller.RelayNotImplemented)
		httpRouter.DELETE("/models/:model", controller.RelayNotImplemented)
	}

	subsiteModelsRouter := router.Group("/s/:slug/v1/models")
	subsiteModelsRouter.Use(middleware.RouteTag("subsite_relay"))
	subsiteModelsRouter.Use(middleware.SubsiteContext(true))
	subsiteModelsRouter.Use(middleware.TokenAuth())
	subsiteModelsRouter.Use(middleware.SubsiteTokenScopeAuth())
	subsiteModelsRouter.Use(middleware.ConnectedAppScopeAuth())
	{
		subsiteModelsRouter.GET("", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.ListModels(c, constant.ChannelTypeAnthropic)
			case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "":
				controller.RetrieveModel(c, constant.ChannelTypeGemini)
			default:
				controller.ListModels(c, constant.ChannelTypeOpenAI)
			}
		})

		subsiteModelsRouter.GET("/:model", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
			default:
				controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
			}
		})
	}

	subsiteV1Router := router.Group("/s/:slug/v1")
	subsiteV1Router.Use(middleware.RouteTag("subsite_relay"))
	subsiteV1Router.Use(middleware.SubsiteContext(true))
	subsiteV1Router.Use(middleware.SystemPerformanceCheck())
	subsiteV1Router.Use(middleware.TokenAuth())
	subsiteV1Router.Use(middleware.SubsiteTokenScopeAuth())
	subsiteV1Router.Use(middleware.ConnectedAppScopeAuth())
	subsiteV1Router.Use(middleware.ModelRequestRateLimit())
	{
		subsiteWSRouter := subsiteV1Router.Group("")
		subsiteWSRouter.Use(middleware.Distribute())
		subsiteWSRouter.GET("/realtime", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIRealtime)
		})
	}
	{
		subsiteHTTPRouter := subsiteV1Router.Group("")
		subsiteHTTPRouter.Use(middleware.Distribute())

		subsiteHTTPRouter.POST("/messages", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatClaude)
		})
		subsiteHTTPRouter.POST("/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})
		subsiteHTTPRouter.POST("/chat/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})
		subsiteHTTPRouter.POST("/responses", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponses)
		})
		subsiteHTTPRouter.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})
		subsiteHTTPRouter.POST("/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		subsiteHTTPRouter.POST("/images/generations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		subsiteHTTPRouter.POST("/images/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		subsiteHTTPRouter.POST("/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatEmbedding)
		})
		subsiteHTTPRouter.POST("/audio/transcriptions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		subsiteHTTPRouter.POST("/audio/translations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		subsiteHTTPRouter.POST("/audio/speech", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		subsiteHTTPRouter.POST("/rerank", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatRerank)
		})
		subsiteHTTPRouter.POST("/engines/:model/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
		subsiteHTTPRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
		subsiteHTTPRouter.POST("/moderations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		subsiteHTTPRouter.POST("/images/variations", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/files", controller.RelayNotImplemented)
		subsiteHTTPRouter.POST("/files", controller.RelayNotImplemented)
		subsiteHTTPRouter.DELETE("/files/:id", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/files/:id", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/files/:id/content", controller.RelayNotImplemented)
		subsiteHTTPRouter.POST("/fine-tunes", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/fine-tunes", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/fine-tunes/:id", controller.RelayNotImplemented)
		subsiteHTTPRouter.POST("/fine-tunes/:id/cancel", controller.RelayNotImplemented)
		subsiteHTTPRouter.GET("/fine-tunes/:id/events", controller.RelayNotImplemented)
		subsiteHTTPRouter.DELETE("/models/:model", controller.RelayNotImplemented)
	}

	relayMjRouter := router.Group("/mj")
	relayMjRouter.Use(middleware.RouteTag("relay"))
	relayMjRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjRouter)

	relayMjModeRouter := router.Group("/:mode/mj")
	relayMjModeRouter.Use(middleware.RouteTag("relay"))
	relayMjModeRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjModeRouter)
	//relayMjRouter.Use()

	relaySunoRouter := router.Group("/suno")
	relaySunoRouter.Use(middleware.RouteTag("relay"))
	relaySunoRouter.Use(middleware.SystemPerformanceCheck())
	relaySunoRouter.Use(middleware.TokenAuth(), middleware.MainSiteTokenScopeAuth(), middleware.ConnectedAppScopeAuth(), middleware.Distribute())
	{
		relaySunoRouter.POST("/submit/:action", controller.RelayTask)
		relaySunoRouter.POST("/fetch", controller.RelayTaskFetch)
		relaySunoRouter.GET("/fetch/:id", controller.RelayTaskFetch)
	}

	relayGeminiRouter := router.Group("/v1beta")
	relayGeminiRouter.Use(middleware.RouteTag("relay"))
	relayGeminiRouter.Use(middleware.SystemPerformanceCheck())
	relayGeminiRouter.Use(middleware.TokenAuth())
	relayGeminiRouter.Use(middleware.MainSiteTokenScopeAuth())
	relayGeminiRouter.Use(middleware.ConnectedAppScopeAuth())
	relayGeminiRouter.Use(middleware.ModelRequestRateLimit())
	relayGeminiRouter.Use(middleware.Distribute())
	{
		// Gemini API 路径格式: /v1beta/models/{model_name}:{action}
		relayGeminiRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
	}
}

func registerMjRouterGroup(relayMjRouter *gin.RouterGroup) {
	relayMjRouter.GET("/image/:id", relay.RelayMidjourneyImage)
	relayMjRouter.Use(middleware.TokenAuth(), middleware.MainSiteTokenScopeAuth(), middleware.ConnectedAppScopeAuth(), middleware.Distribute())
	{
		relayMjRouter.POST("/submit/action", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/shorten", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/modal", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/imagine", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/simple-change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/describe", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/blend", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/edits", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/video", controller.RelayMidjourney)
		//relayMjRouter.POST("/notify", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/fetch", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/image-seed", controller.RelayMidjourney)
		relayMjRouter.POST("/task/list-by-condition", controller.RelayMidjourney)
		relayMjRouter.POST("/insight-face/swap", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/upload-discord-images", controller.RelayMidjourney)
	}
}
