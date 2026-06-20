package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.POST("/setup/runtime-config", anonymousRequestBodyLimit, controller.PostSetupRuntimeConfig)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
		}
		serviceStatusRoute := apiRouter.Group("/service-status")
		serviceStatusRoute.Use(middleware.TryUserAuth())
		{
			serviceStatusRoute.GET("/summary", controller.GetServiceStatusSummary)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), controller.TelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		// :env separates test vs prod URLs so the operator can register each
		// in Pancake's matching webhook slot; handler enforces env match.
		apiRouter.POST("/waffo-pancake/webhook/:env", anonymousRequestBodyLimit, controller.WaffoPancakeWebhook)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.UniversalVerify)

		notificationRoute := apiRouter.Group("/notifications")
		notificationRoute.Use(middleware.UserAuth())
		{
			notificationRoute.GET("/read-state", controller.GetNotificationReadState)
			notificationRoute.POST("/read", controller.MarkNotificationsRead)
			notificationRoute.GET("/enterprise-quota-requests", controller.ListEnterpriseQuotaRequestNotifications)
			notificationRoute.POST("/enterprise-quota-requests/read", controller.MarkEnterpriseQuotaRequestNotificationsRead)
		}

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.GET("/logout", controller.Logout)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				selfRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", controller.Setup2FA)
				selfRoute.POST("/2fa/enable", controller.Enable2FA)
				selfRoute.POST("/2fa/disable", controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
				selfRoute.DELETE("/bindings/:binding_type", controller.ClearSelfUserBinding)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/topup", controller.GetAllTopUps)
				adminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/balance/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestBalancePay)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestWaffoPancakePay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.POST("/payment_compliance", controller.ConfirmPaymentCompliance)
			optionRoute.POST("/exchange-rate/fetch", controller.FetchExchangeRate)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.POST("/migrate_console_setting", controller.MigrateConsoleSetting) // 用于迁移检测的旧键，下个版本会删除
			optionRoute.POST("/waffo-pancake/catalog", controller.ListWaffoPancakeCatalog)
			optionRoute.POST("/waffo-pancake/pair", controller.CreateWaffoPancakePair)
			optionRoute.POST("/waffo-pancake/save", controller.SaveWaffoPancake)
			optionRoute.POST("/waffo-pancake/subscription-product", controller.CreateWaffoPancakeSubscriptionProduct)
			optionRoute.POST("/waffo-pancake/subscription-product-options", controller.ListWaffoPancakeSubscriptionProductOptions)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", controller.GetAllChannels)
			channelRoute.GET("/search", controller.SearchChannels)
			channelRoute.GET("/models", controller.ChannelListModels)
			channelRoute.GET("/models_enabled", controller.EnabledListModels)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), controller.GetChannelKey)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", controller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", controller.EnableTagChannels)
			channelRoute.PUT("/tag", controller.EditTagChannels)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.POST("/batch", controller.DeleteChannelBatch)
			channelRoute.POST("/fix", controller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", controller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", middleware.RootAuth(), controller.FetchModels)
			channelRoute.POST("/codex/oauth/start", controller.StartCodexOAuth)
			channelRoute.POST("/codex/oauth/complete", controller.CompleteCodexOAuth)
			channelRoute.POST("/:id/codex/oauth/start", controller.StartCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/oauth/complete", controller.CompleteCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/refresh", controller.RefreshCodexChannelCredential)
			channelRoute.GET("/:id/codex/usage", controller.GetCodexChannelUsage)
			channelRoute.POST("/ollama/pull", controller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", controller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", controller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", controller.OllamaVersion)
			channelRoute.POST("/batch/tag", controller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", controller.GetTagModels)
			channelRoute.POST("/copy/:id", controller.CopyChannel)
			channelRoute.POST("/multi_key/manage", controller.ManageMultiKeys)
			channelRoute.POST("/upstream_updates/apply", controller.ApplyChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/apply_all", controller.ApplyAllChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect", controller.DetectChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect_all", controller.DetectAllChannelUpstreamModelUpdates)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/enterprise-projects", controller.ListTokenEnterpriseProjects)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)

		enterpriseRoute := apiRouter.Group("/enterprise")
		enterpriseRoute.Use(middleware.UserAuth())
		{
			enterpriseRoute.GET("/quota-requests", controller.ListEnterpriseQuotaRequests)
			enterpriseRoute.GET("/quota-requests/policies", controller.ListEnterpriseQuotaRequestPolicies)
			enterpriseRoute.POST("/quota-requests", controller.SubmitEnterpriseQuotaRequest)
			enterpriseRoute.POST("/quota-requests/:id/withdraw", controller.WithdrawEnterpriseQuotaRequest)

			readEnterpriseRoute := enterpriseRoute.Group("")
			readEnterpriseRoute.Use(middleware.EnterpriseCapabilityAuth(service.EnterpriseCapabilityRead))
			{
				readEnterpriseRoute.GET("/current", controller.GetCurrentEnterprise)
				readEnterpriseRoute.GET("/org-units", controller.ListEnterpriseOrgUnits)
				readEnterpriseRoute.GET("/projects", controller.ListEnterpriseProjects)
				readEnterpriseRoute.GET("/quota-policies", controller.ListEnterpriseQuotaPolicies)
			}

			manageEnterpriseRoute := enterpriseRoute.Group("")
			manageEnterpriseRoute.Use(middleware.EnterpriseCapabilityAuth(service.EnterpriseCapabilityManage))
			{
				manageEnterpriseRoute.PUT("/current", controller.UpdateCurrentEnterprise)
				manageEnterpriseRoute.POST("/org-units", controller.CreateEnterpriseOrgUnit)
				manageEnterpriseRoute.PUT("/org-units/:id", controller.UpdateEnterpriseOrgUnit)
				manageEnterpriseRoute.DELETE("/org-units/:id", controller.DeleteEnterpriseOrgUnit)
				manageEnterpriseRoute.POST("/org-sync/preview", controller.PreviewEnterpriseOrgSync)
				manageEnterpriseRoute.POST("/org-sync/apply", controller.ApplyEnterpriseOrgSync)
				manageEnterpriseRoute.POST("/quota-counters/reconcile", controller.ReconcileEnterpriseQuotaCounters)
				manageEnterpriseRoute.GET("/webhooks", controller.ListEnterpriseWebhooks)
				manageEnterpriseRoute.POST("/webhooks", controller.CreateEnterpriseWebhook)
				manageEnterpriseRoute.PUT("/webhooks/:id", controller.UpdateEnterpriseWebhook)
				manageEnterpriseRoute.DELETE("/webhooks/:id", controller.DeleteEnterpriseWebhook)
				manageEnterpriseRoute.POST("/webhooks/:id/test", controller.TestEnterpriseWebhook)
				manageEnterpriseRoute.GET("/notification-preferences", controller.ListEnterpriseNotificationPreferences)
				manageEnterpriseRoute.PUT("/notification-preferences", controller.UpdateEnterpriseNotificationPreference)
				manageEnterpriseRoute.POST("/notification-outbox/:id/retry", controller.RetryEnterpriseNotificationOutbox)
			}

			departmentManageEnterpriseRoute := enterpriseRoute.Group("")
			departmentManageEnterpriseRoute.Use(middleware.EnterpriseAnyCapabilityAuth(service.EnterpriseCapabilityManage, service.EnterpriseCapabilityDepartmentManage))
			{
				departmentManageEnterpriseRoute.GET("/members", controller.ListEnterpriseMembers)
				departmentManageEnterpriseRoute.PUT("/members/:user_id/org-unit", controller.UpdateEnterpriseMemberOrgUnit)
				departmentManageEnterpriseRoute.GET("/policy-groups", controller.ListEnterprisePolicyGroups)
				departmentManageEnterpriseRoute.POST("/policy-groups", controller.CreateEnterprisePolicyGroup)
				departmentManageEnterpriseRoute.PUT("/policy-groups/:id", controller.UpdateEnterprisePolicyGroup)
				departmentManageEnterpriseRoute.DELETE("/policy-groups/:id", controller.DeleteEnterprisePolicyGroup)
				departmentManageEnterpriseRoute.GET("/policy-groups/:id/members", controller.ListEnterprisePolicyGroupMembers)
				departmentManageEnterpriseRoute.POST("/policy-groups/:id/members", controller.AddEnterprisePolicyGroupMembers)
				departmentManageEnterpriseRoute.DELETE("/policy-groups/:id/members/:user_id", controller.DeleteEnterprisePolicyGroupMember)
				departmentManageEnterpriseRoute.POST("/quota-policies", controller.CreateEnterpriseQuotaPolicy)
				departmentManageEnterpriseRoute.PUT("/quota-policies/:id", controller.UpdateEnterpriseQuotaPolicy)
				departmentManageEnterpriseRoute.DELETE("/quota-policies/:id", controller.DeleteEnterpriseQuotaPolicy)
			}

			projectEnterpriseRoute := enterpriseRoute.Group("")
			projectEnterpriseRoute.Use(middleware.EnterpriseCapabilityAuth(service.EnterpriseCapabilityProjectManage))
			{
				projectEnterpriseRoute.POST("/projects", controller.CreateEnterpriseProject)
				projectEnterpriseRoute.PUT("/projects/:id", controller.UpdateEnterpriseProject)
				projectEnterpriseRoute.DELETE("/projects/:id", controller.DeleteEnterpriseProject)
				projectEnterpriseRoute.PUT("/projects/:id/members", controller.UpsertEnterpriseProjectMember)
				projectEnterpriseRoute.DELETE("/projects/:id/members/:user_id", controller.DeleteEnterpriseProjectMember)
			}

			projectReadEnterpriseRoute := enterpriseRoute.Group("")
			projectReadEnterpriseRoute.Use(middleware.EnterpriseAnyCapabilityAuth(service.EnterpriseCapabilityManage, service.EnterpriseCapabilityProjectRead, service.EnterpriseCapabilityProjectManage))
			{
				projectReadEnterpriseRoute.GET("/projects/:id/members", controller.ListEnterpriseProjectMembers)
			}

			quotaApprovalEnterpriseRoute := enterpriseRoute.Group("")
			quotaApprovalEnterpriseRoute.Use(middleware.EnterpriseCapabilityAuth(service.EnterpriseCapabilityQuotaApprove))
			{
				quotaApprovalEnterpriseRoute.POST("/quota-requests/:id/approve", controller.ApproveEnterpriseQuotaRequest)
				quotaApprovalEnterpriseRoute.POST("/quota-requests/:id/reject", controller.RejectEnterpriseQuotaRequest)
			}

			financeEnterpriseRoute := enterpriseRoute.Group("")
			financeEnterpriseRoute.Use(middleware.EnterpriseAnyCapabilityAuth(service.EnterpriseCapabilityFinanceRead, service.EnterpriseCapabilityProjectRead))
			{
				financeEnterpriseRoute.GET("/usage/summary", controller.GetEnterpriseUsageSummary)
				financeEnterpriseRoute.GET("/usage/breakdown", controller.GetEnterpriseUsageBreakdown)
				financeEnterpriseRoute.GET("/usage/breakdown/export", controller.ExportEnterpriseUsageBreakdown)
			}

			auditLogEnterpriseRoute := enterpriseRoute.Group("")
			auditLogEnterpriseRoute.Use(middleware.EnterpriseAnyCapabilityAuth(service.EnterpriseCapabilityAuditRead, service.EnterpriseCapabilityDepartmentManage, service.EnterpriseCapabilityProjectRead, service.EnterpriseCapabilityProjectManage))
			{
				auditLogEnterpriseRoute.GET("/audit-logs", controller.ListEnterpriseAuditLogs)
			}

			auditEnterpriseRoute := enterpriseRoute.Group("")
			auditEnterpriseRoute.Use(middleware.EnterpriseCapabilityAuth(service.EnterpriseCapabilityAuditRead))
			{
				auditEnterpriseRoute.GET("/notification-outbox", controller.ListEnterpriseNotificationOutbox)
				auditEnterpriseRoute.GET("/notification-outbox/worker-metrics", controller.GetEnterpriseNotificationOutboxWorkerMetrics)
			}
		}

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mcpToolsRoute := apiRouter.Group("/mcp/tools")
		{
			mcpToolsRoute.GET("/", middleware.UserAuth(), controller.GetMCPTools)
			mcpToolsRoute.POST("/", middleware.AdminAuth(), controller.CreateMCPTool)
			mcpToolsRoute.POST("/seed", middleware.RootAuth(), controller.SeedMCPTools)
			mcpToolsRoute.GET("/:id", middleware.UserAuth(), controller.GetMCPTool)
			mcpToolsRoute.PATCH("/:id", middleware.AdminAuth(), controller.UpdateMCPTool)
			mcpToolsRoute.POST("/:id/archive", middleware.AdminAuth(), controller.ArchiveMCPTool)
			mcpToolsRoute.DELETE("/:id", middleware.AdminAuth(), controller.DeleteMCPTool)
		}
		mcpSummaryRoute := apiRouter.Group("/mcp/summary")
		mcpSummaryRoute.Use(middleware.UserAuth())
		{
			mcpSummaryRoute.GET("/", controller.GetMCPSummary)
		}
		mcpToolCallsRoute := apiRouter.Group("/mcp/tool-calls")
		mcpToolCallsRoute.Use(middleware.UserAuth())
		{
			mcpToolCallsRoute.GET("/", controller.GetMCPToolCalls)
		}
		mcpOpenAPIBinaryRoute := apiRouter.Group("/mcp/openapi/binary")
		{
			mcpOpenAPIBinaryRoute.GET("/", middleware.AdminAuth(), controller.GetMCPOpenAPIBinaryObjects)
			mcpOpenAPIBinaryRoute.GET("/summary", middleware.AdminAuth(), controller.GetMCPOpenAPIBinaryObjectSummary)
			mcpOpenAPIBinaryRoute.POST("/cleanup", middleware.AdminAuth(), controller.CleanupMCPOpenAPIBinaryObjects)
			mcpOpenAPIBinaryRoute.GET("/:object_id/download", middleware.UserAuth(), controller.DownloadMCPOpenAPIBinaryObject)
		}
		mcpOpenAPIRoute := apiRouter.Group("/mcp/openapi")
		mcpOpenAPIRoute.Use(middleware.AdminAuth())
		{
			mcpOpenAPIRoute.POST("/preview", controller.PreviewMCPOpenAPI)
			mcpOpenAPIRoute.POST("/diff", controller.DiffMCPOpenAPI)
			mcpOpenAPIRoute.POST("/import", controller.ImportMCPOpenAPI)
			mcpOpenAPIRoute.POST("/disable", controller.DisableMCPOpenAPI)
			mcpOpenAPIRoute.DELETE("/", controller.DeleteMCPOpenAPI)
		}
		mcpProxyServersRoute := apiRouter.Group("/mcp/proxy/servers")
		mcpProxyServersRoute.Use(middleware.AdminAuth())
		{
			mcpProxyServersRoute.GET("/", controller.GetMCPProxyServers)
			mcpProxyServersRoute.POST("/", controller.CreateMCPProxyServer)
			mcpProxyServersRoute.GET("/trends", controller.GetMCPProxyTrends)
			mcpProxyServersRoute.GET("/health-check", controller.GetMCPProxyHealthCheck)
			mcpProxyServersRoute.PUT("/health-check", controller.UpdateMCPProxyHealthCheck)
			mcpProxyServersRoute.POST("/health-check/run", controller.RunMCPProxyHealthCheck)
			mcpProxyServersRoute.GET("/heartbeat", controller.GetMCPProxyHeartbeat)
			mcpProxyServersRoute.PUT("/heartbeat", controller.UpdateMCPProxyHeartbeat)
			mcpProxyServersRoute.POST("/heartbeat/run", controller.RunMCPProxyHeartbeat)
			mcpProxyServersRoute.GET("/:id", controller.GetMCPProxyServer)
			mcpProxyServersRoute.PATCH("/:id", controller.UpdateMCPProxyServer)
			mcpProxyServersRoute.DELETE("/:id", controller.DeleteMCPProxyServer)
			mcpProxyServersRoute.POST("/:id/test", controller.TestMCPProxyServer)
			mcpProxyServersRoute.POST("/:id/discover", controller.DiscoverMCPProxyServerTools)
			mcpProxyServersRoute.GET("/:id/tools", controller.GetMCPProxyServerTools)
			mcpProxyServersRoute.GET("/:id/health", controller.GetMCPProxyServerHealth)
			mcpProxyServersRoute.GET("/:id/discovery-events", controller.GetMCPProxyServerDiscoveryEvents)
		}
		mcpProxyToolsRoute := apiRouter.Group("/mcp/proxy/tools")
		mcpProxyToolsRoute.Use(middleware.AdminAuth())
		{
			mcpProxyToolsRoute.GET("/", controller.GetMCPProxyTools)
			mcpProxyToolsRoute.GET("/:id", controller.GetMCPProxyTool)
			mcpProxyToolsRoute.GET("/:id/health", controller.GetMCPProxyToolHealth)
			mcpProxyToolsRoute.PATCH("/:id", controller.UpdateMCPProxyTool)
		}
		billingEventsRoute := apiRouter.Group("/billing/events")
		billingEventsRoute.Use(middleware.UserAuth())
		{
			billingEventsRoute.GET("/", controller.GetBillingEvents)
			billingEventsRoute.GET("/summary", controller.GetBillingEventSummary)
			billingEventsRoute.GET("/health", middleware.AdminAuth(), controller.GetBillingEventHealth)
			billingEventsRoute.GET("/source-matrix", middleware.AdminAuth(), controller.GetBillingEventSourceMatrix)
			billingEventsRoute.GET("/relation-health", middleware.AdminAuth(), controller.GetBillingEventRelationHealth)
			billingEventsRoute.POST("/relation-backfill", middleware.AdminAuth(), controller.BackfillBillingEventRelations)
			billingEventsRoute.POST("/relation-repair", middleware.AdminAuth(), controller.RepairBillingEventRelations)
			billingEventsRoute.POST("/relation-orphans/cleanup", middleware.AdminAuth(), controller.CleanupBillingEventRelationOrphans)
			billingEventsRoute.GET("/relation-inspection", middleware.AdminAuth(), controller.GetBillingEventRelationInspection)
			billingEventsRoute.GET("/relation-inspection/runs", middleware.AdminAuth(), controller.GetBillingEventRelationInspectionRuns)
			billingEventsRoute.PUT("/relation-inspection", middleware.AdminAuth(), controller.UpdateBillingEventRelationInspection)
			billingEventsRoute.POST("/relation-inspection/run", middleware.AdminAuth(), controller.RunBillingEventRelationInspection)
			billingEventsRoute.POST("/reconciliation", middleware.AdminAuth(), controller.ReconcileBillingEvents)
			billingEventsRoute.POST("/reconciliation/mismatches", middleware.AdminAuth(), controller.GetBillingEventReconciliationMismatches)
			billingEventsRoute.POST("/reconciliation/missing", middleware.AdminAuth(), controller.GetBillingEventReconciliationMissing)
			billingEventsRoute.POST("/reconciliation/repair", middleware.AdminAuth(), controller.RepairBillingEventReconciliationMismatch)
			billingEventsRoute.POST("/reconciliation/backfill-missing", middleware.AdminAuth(), controller.BackfillBillingEventReconciliationMissing)
			billingEventsRoute.POST("/backfill", middleware.AdminAuth(), controller.BackfillBillingEvents)
		}

		bridgeRoute := apiRouter.Group("/bridge")
		bridgeRoute.Use(middleware.UserAuth())
		{
			bridgeRoute.GET("/clients", controller.GetBridgeClients)
			bridgeRoute.GET("/clients/:client_id/health", controller.GetBridgeClientHealth)
			bridgeRoute.GET("/clients/:client_id", controller.GetBridgeClient)
			bridgeRoute.PATCH("/clients/:client_id", middleware.AdminAuth(), controller.UpdateBridgeClient)
			bridgeRoute.DELETE("/clients/:client_id", middleware.AdminAuth(), controller.DeleteBridgeClient)
			bridgeRoute.POST("/sessions/:session_id/close", middleware.AdminAuth(), controller.CloseBridgeSession)
			bridgeRoute.GET("/audit-logs", controller.GetBridgeAuditLogs)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
