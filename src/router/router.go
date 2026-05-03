// router contains the gin router definitions and maps requests to handlers
package router

import (
	"net/http"
	"strings"

	"time"

	"codeberg.org/isotop7/proviant/api/auth"
	"codeberg.org/isotop7/proviant/api/common"
	"codeberg.org/isotop7/proviant/api/onboarding"
	v1 "codeberg.org/isotop7/proviant/api/v1"
	"codeberg.org/isotop7/proviant/assets"
	"codeberg.org/isotop7/proviant/controllers"
	dbcontroller "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/util"
	"codeberg.org/isotop7/proviant/web"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type zerologWriter struct {
	logger *zerolog.Logger
	level  zerolog.Level
}

func (w zerologWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	w.logger.WithLevel(w.level).Msg(msg)
	return len(p), nil
}

func cleanupRevokedTokens(db *gorm.DB, logger *zerolog.Logger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		result := db.Where("expires_at < ?", time.Now()).Delete(&authentication.RevokedToken{})
		logger.Info().Int64("deleted", result.RowsAffected).Msg("Cleaned up expired revoked tokens")
	}
}

// mustInitJWT creates and fully initializes a GinJWTMiddleware; panics on any error.
func mustInitJWT(
	logger *zerolog.Logger,
	config *configuration.ProviantConfiguration,
	db *gorm.DB,
	authorizatorFunc func(data any, ctx *gin.Context) bool,
	unauthorizedFunc func(ctx *gin.Context, code int, message string),
) *jwt.GinJWTMiddleware {
	middleware, err := JWTMiddleware(config, db, authorizatorFunc, unauthorizedFunc)
	if err != nil {
		logger.Error().Msg(err.Error())
		panic(err.Error())
	}
	if err := middleware.MiddlewareInit(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err.Error())
	}
	return middleware
}

// SetupRouter creates the gin engine and associated middleware
func SetupRouter(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB, offacntrl *controllers.OpenFoodFactsAPIController, notificationController *controllers.NotificationController, ocrController *controllers.OCRControllerImpl) *gin.Engine {
	go cleanupRevokedTokens(dbHandle, logger)

	repos := dbcontroller.NewRepositoryContainer(dbHandle)

	gin.DefaultWriter = zerologWriter{logger: logger, level: zerolog.DebugLevel}
	gin.DefaultErrorWriter = zerologWriter{logger: logger, level: zerolog.WarnLevel}

	// Generate new gin instance
	engine := gin.New()

	// Inject logging middleware
	engine.Use(ZerologMiddleware(logger), gin.Recovery())

	// Setup cors
	corsConfig := cors.DefaultConfig()
	// Check if any origin is allowed or set list
	if proviantConfiguration.Server.CORS.AllowAllOrigins {
		corsConfig.AllowAllOrigins = true
		logger.Info().Msg("Allowed all CORS origins")
	} else {
		corsConfig.AllowAllOrigins = false
		corsConfig.AllowOrigins = proviantConfiguration.Server.CORS.AllowedOrigins
		logger.Info().Msgf("Allowed CORS origins: %s", strings.Join(proviantConfiguration.Server.CORS.AllowedOrigins, "; "))
	}
	corsConfig.AllowCredentials = true
	engine.Use(cors.New(corsConfig))

	// Setup security headers
	engine.Use(SecurityHeadersMiddleware(proviantConfiguration))

	// RequestID middleware - must run before other context injectors
	engine.Use(RequestIDMiddleware(logger))

	// Database
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(util.ContextKeyDBHandle, dbHandle)
		ctx.Next()
	})

	// Repository container
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(util.ContextKeyRepos, repos)
		ctx.Next()
	})

	// OpenFoodFactsAPI Controller
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("offacntrl", offacntrl)
		ctx.Next()
	})

	// Template cache
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("templateCache", proviantConfiguration.TemplateCache)
		ctx.Next()
	})

	// Notification controller for invitation emails
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(util.ContextKeyNotificationController, notificationController)
		ctx.Next()
	})

	// OCR controller for expiry date detection
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("ocrController", ocrController)
		ctx.Next()
	})

	// Proviant configuration for access in handlers
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(util.ContextKeyProviantConfig, proviantConfiguration)
		ctx.Next()
	})

	// Recipe controller for recipe suggestions
	engine.Use(func(ctx *gin.Context) {
		recipeCtrl := controllers.NewRecipeController(
			proviantConfiguration.RecipeAPI,
			logger,
			dbHandle,
		)
		ctx.Set("recipeController", recipeCtrl)
		ctx.Next()
	})

	jwtAPIMiddleware := mustInitJWT(logger, proviantConfiguration, dbHandle, AuthorizatorNotUserAware, UnauthorizedAPIFunc)
	jwtAPIUserAwareMiddleware := mustInitJWT(logger, proviantConfiguration, dbHandle, AuthorizatorUserAware, UnauthorizedAPIFunc)
	jwtFrontendMiddleware := mustInitJWT(logger, proviantConfiguration, dbHandle, AuthorizatorNotUserAware, UnauthorizedFrontendFunc)
	jwtFrontendUserAwareMiddleware := mustInitJWT(logger, proviantConfiguration, dbHandle, AuthorizatorUserAware, UnauthorizedFrontendFunc)

	// Wrap JWT middlewares with PAT support
	jwtAPIMiddlewareWithPAT := PATMiddleware(jwtAPIMiddleware)
	jwtAPIUserAwareMiddlewareWithPAT := PATMiddleware(jwtAPIUserAwareMiddleware)

	// Map routes to handlers
	// Health routes
	engine.GET("/health", common.GetHealth)

	// Favicon redirect
	engine.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusPermanentRedirect, "/assets/icons/favicon.ico")
	})

	// Authentication routes
	engine.POST("/auth/login", loginRateLimitMiddleware, jwtAPIMiddleware.LoginHandler)

	// Signup routes
	engine.POST("/auth/signup", signupRateLimitMiddleware, auth.Signup)
	engine.POST("/auth/verify-email", auth.VerifyEmail)
	engine.POST("/auth/invite/accept", auth.AcceptInvitation)
	engine.GET("/auth/refresh_token", jwtAPIMiddleware.RefreshHandler)

	// Logout route (requires authentication)
	logoutAuth := engine.Group("/auth")
	logoutAuth.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	logoutAuth.POST("/logout", auth.Logout)

	// Public product routes
	publicProductAPI := engine.Group("/api/v1/products")
	publicProductAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	publicProductAPI.GET("", v1.GetProducts)
	publicProductAPI.GET("/archived", v1.GetArchivedProducts)
	publicProductAPI.GET("/expired", v1.GetExpired)
	publicProductAPI.POST("", v1.CreateProduct)
	publicProductAPI.POST("/scan", v1.ScanProduct)
	publicProductAPI.POST("/scan-date", v1.ScanExpiryDate)
	publicProductAPI.GET("/byBarcode/:barcode", v1.GetProductsByBarcode)
	publicProductAPI.GET("/openfoodfacts/:barcode", v1.GetOpenFoodFactsData)
	publicProductAPI.GET("/search", v1.SearchProducts)
	publicProductAPI.DELETE("/bulkDelete", v1.BulkDeleteProducts)
	publicProductAPI.DELETE("/bulkArchive", v1.BulkArchiveProducts)
	publicProductAPI.POST("/bulkRestore", v1.BulkRestoreProducts)
	publicProductAPI.GET("/stats", v1.GetProductStats)
	publicProductAPI.GET("/summary", v1.GetProductSummary)
	publicProductAPI.GET("/export/products.csv", exportRateLimitMiddleware, v1.ExportProductsCSV)
	publicProductAPI.GET("/export/products.json", exportRateLimitMiddleware, v1.ExportProductsJSON)
	publicProductAPI.GET("/export/archive.csv", exportRateLimitMiddleware, v1.ExportArchiveCSV)
	publicProductAPI.GET("/export/full.json", exportRateLimitMiddleware, v1.ExportFullJSON)

	// Protected user routes
	protectedUserAPI := engine.Group("/api/v1/user")
	protectedUserAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	protectedUserAPI.PATCH("", v1.UpdateUser)
	protectedUserAPI.POST("/password", v1.UpdateUserPassword)
	protectedUserAPI.GET("/notification-preferences", v1.GetUserNotificationPreferences)
	protectedUserAPI.POST("/notification-preferences", v1.UpdateUserNotificationPreferences)
	protectedUserAPI.POST("/telegram-link-token", v1.GenerateTelegramLinkToken)
	protectedUserAPI.POST("/household/leave", v1.LeaveHousehold)
	protectedUserAPI.POST("/household/create", v1.CreateHousehold)
	protectedUserAPI.POST("/tokens", v1.CreateUserToken)
	protectedUserAPI.GET("/tokens", v1.ListUserTokens)
	protectedUserAPI.DELETE("/tokens/:id", v1.DeleteUserToken)

	// Onboarding routes (require authentication)
	onboardingAPI := engine.Group("/api/v1/onboarding")
	onboardingAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	onboardingAPI.GET("/state", onboarding.GetOnboardingState)
	onboardingAPI.GET("/households", onboarding.GetAvailableHouseholds)
	onboardingAPI.PATCH("/profile", onboarding.UpdateOnboardingProfile)
	onboardingAPI.POST("/apply-household", onboarding.ApplyForHousehold)
	onboardingAPI.POST("/create-household", onboarding.CreateOnboardingHousehold)
	onboardingAPI.POST("/join-invite", onboarding.JoinOnboardingByInvite)
	onboardingAPI.POST("/complete", onboarding.CompleteOnboarding)

	// Streak routes
	streakAPI := engine.Group("/api/v1/streak")
	streakAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	streakAPI.GET("", v1.GetStreak)

	// Savings routes
	savingsAPI := engine.Group("/api/v1/savings")
	savingsAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	savingsAPI.GET("/stats", v1.GetSavingsStats)

	// Recipe suggestion routes
	recipeAPI := engine.Group("/api/v1/recipes")
	recipeAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	recipeAPI.GET("/suggestions", v1.GetRecipeSuggestions)

	// Notification routes
	notificationAPI := engine.Group("/api/v1/notifications")
	notificationAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	notificationAPI.GET("", v1.GetNotifications)

	// Household application routes
	householdAPI := engine.Group("/api/v1/household")
	householdAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	householdAPI.POST("/:id/apply", v1.ApplyForHousehold)
	householdAPI.GET("/applications", v1.GetHouseholdApplications)
	householdAPI.POST("/applications/:id/approve", v1.ApproveHouseholdApplication)
	householdAPI.POST("/applications/:id/reject", v1.RejectHouseholdApplication)
	householdAPI.DELETE("/applications/:id", v1.CancelHouseholdApplication)
	householdAPI.PATCH("/name", v1.UpdateHouseholdName)
	householdAPI.DELETE("/members/:userId", v1.RemoveHouseholdMember)

	// Household invitation routes
	householdAPI.POST("/invitations", v1.CreateInvitation)
	householdAPI.GET("/invitations", v1.GetInvitations)
	householdAPI.DELETE("/invitations/:id", v1.CancelInvitation)

	// Storage location routes
	householdAPI.GET("/storage-locations", v1.ListStorageLocations)
	householdAPI.POST("/storage-locations", v1.CreateStorageLocation)
	householdAPI.PATCH("/storage-locations/:id", v1.UpdateStorageLocation)
	householdAPI.DELETE("/storage-locations/:id", v1.DeleteStorageLocation)

	// Admin user management routes
	adminAPI := engine.Group("/api/v1/admin/users")
	adminAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	adminAPI.GET("", v1.GetHouseholdUsers)
	adminAPI.PATCH("/:id", v1.UpdateHouseholdUser)
	adminAPI.DELETE("/:id", v1.DeleteHouseholdUser)
	adminAPI.POST("/:id/reset-password", v1.AdminResetUserPassword)

	// Protected product routes
	protectedProductAPI := engine.Group("/api/v1/products")
	protectedProductAPI.Use(jwtAPIUserAwareMiddlewareWithPAT, UserContextLoggerMiddleware())
	protectedProductAPI.GET("/:id", v1.GetProduct)
	protectedProductAPI.PATCH("/:id", v1.UpdateProduct)
	protectedProductAPI.PATCH("/:id/amount", v1.UpdateProductAmount)
	protectedProductAPI.DELETE("/:id", v1.DeleteProduct)
	protectedProductAPI.POST("/:id/restore", v1.RestoreProduct)
	protectedProductAPI.POST("/:id/expire", v1.SetExpireAt)
	protectedProductAPI.POST("/:id/consume", v1.ConsumeProduct)
	protectedProductAPI.POST("/:id/waste", v1.WasteProduct)

	// Webhook routes
	webhookAPI := engine.Group("/api/v1/webhooks")
	webhookAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	webhookAPI.POST("", v1.CreateWebhook)
	webhookAPI.GET("", v1.ListWebhooks)
	webhookAPI.GET("/:id", v1.GetWebhook)
	webhookAPI.PATCH("/:id", v1.UpdateWebhook)
	webhookAPI.DELETE("/:id", v1.DeleteWebhook)
	webhookAPI.GET("/:id/deliveries", v1.GetWebhookDeliveries)

	// Calendar routes (export uses token query param, token management uses JWT)
	calendarAPI := engine.Group("/api/v1/calendar")
	calendarAPI.GET("/export.ics", v1.ExportICalendar)
	calendarAPI.Use(jwtAPIMiddlewareWithPAT, UserContextLoggerMiddleware())
	calendarAPI.POST("/token", v1.CreateCalendarToken)
	calendarAPI.DELETE("/token", v1.DeleteCalendarToken)
	calendarAPI.GET("/token", v1.GetCalendarTokenStatus)

	// PWA — serve manifest and service worker at root scope (no auth required)
	engine.GET("/manifest.json", func(ctx *gin.Context) {
		content, readErr := assets.AssetFiles.ReadFile("manifest.json")
		if readErr != nil {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Data(http.StatusOK, "application/manifest+json", content)
	})
	engine.GET("/sw.js", func(ctx *gin.Context) {
		content, readErr := assets.AssetFiles.ReadFile("js/sw.js")
		if readErr != nil {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Header("Service-Worker-Allowed", "/")
		ctx.Data(http.StatusOK, "application/javascript", content)
	})

	// Web frontend routes
	// Serve asset files
	engine.StaticFS("/assets", http.FS(assets.AssetFiles))
	if proviantConfiguration.OpenFoodFacts.ImageCacheEnabled {
		productImages := engine.Group("/product-images")
		productImages.Use(func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=31536000")
		})
		productImages.Static("/", proviantConfiguration.OpenFoodFacts.ImageCachePath)
	}
	// Create frontend handler with template cache
	webFrontendHandler := web.Frontend{TemplateCache: proviantConfiguration.TemplateCache}
	webFrontend := engine.Group("/web")
	webFrontend.GET("/auth", webFrontendHandler.Auth)

	// Public web frontend routes
	publicWebFrontend := engine.Group("/web")
	publicWebFrontend.Use(jwtFrontendMiddleware.MiddlewareFunc(), UserContextLoggerMiddleware())
	publicWebFrontend.GET("/", webFrontendHandler.Root)
	publicWebFrontend.GET("/user", webFrontendHandler.User)
	publicWebFrontend.GET("/user/settings", webFrontendHandler.UserSettings)
	publicWebFrontend.GET("/products", webFrontendHandler.Products)
	publicWebFrontend.GET("/products/scan", webFrontendHandler.ProductsScan)
	publicWebFrontend.GET("/onboarding", webFrontendHandler.Onboarding)
	publicWebFrontend.GET("/recipes", webFrontendHandler.Recipes)

	// Public invite acceptance page (no auth required)
	engine.GET("/web/invite/accept", webFrontendHandler.AcceptInvite)

	// Public email verification page (no auth required)
	engine.GET("/web/verify-email", webFrontendHandler.VerifyEmail)

	// Protected web frontend routes
	protectedWebFrontend := engine.Group("/web")
	protectedWebFrontend.Use(jwtFrontendUserAwareMiddleware.MiddlewareFunc(), UserContextLoggerMiddleware())
	protectedWebFrontend.GET("/products/:id/view", webFrontendHandler.ProductsView)
	protectedWebFrontend.GET("/products/:id/edit", webFrontendHandler.ProductsEdit)

	// Static redirects
	engine.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusPermanentRedirect, "/web")
	})

	// Catch-All handler
	engine.NoRoute(func(ctx *gin.Context) {
		logger.Error().Msgf("NoRoute ('%s')", ctx.Request.RequestURI)

		if strings.HasPrefix(ctx.Request.URL.Path, "/web") {
			templates.RenderError(ctx, webFrontendHandler.TemplateCache, http.StatusNotFound, "Page not found")
		} else {
			ctx.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
		}
	})

	// Return engine to caller
	return engine
}
