// router contains the gin router definitions and maps requests to handlers
package router

import (
	"net/http"
	"strings"

	"gitlab.com/Isotop7/proviant/api/auth"
	"gitlab.com/Isotop7/proviant/api/common"
	v1 "gitlab.com/Isotop7/proviant/api/v1"
	"gitlab.com/Isotop7/proviant/assets"
	"gitlab.com/Isotop7/proviant/controllers"
	"gitlab.com/Isotop7/proviant/errors"
	"gitlab.com/Isotop7/proviant/models/configuration"
	"gitlab.com/Isotop7/proviant/templates"
	"gitlab.com/Isotop7/proviant/web"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SetupRouter creates the gin engine and associated middleware
func SetupRouter(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB, offacntrl controllers.OpenFoodFactsAPIController) *gin.Engine {
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

	// Pass references to gin context
	// Logging
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("logger", logger)
		ctx.Next()
	})

	// Database
	engine.Use(func(ctx *gin.Context) {
		ctx.Set("dbHandle", dbHandle)
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

	// Setup JWT authentication middleware for API
	jwtAPIMiddleware, jwtAPIAuthSetupErr := JWTMiddleware(proviantConfiguration, dbHandle, AuthorizatorNotUserAware, UnauthorizedAPIFunc)
	if jwtAPIAuthSetupErr != nil {
		logger.Error().Msg(jwtAPIAuthSetupErr.Error())
		panic(jwtAPIAuthSetupErr.Error())
	}
	// Initialize JWT authentication middleware
	jwtAuthMiddlewareInitErr := jwtAPIMiddleware.MiddlewareInit()
	if jwtAuthMiddlewareInitErr != nil {
		logger.Error().Msg(jwtAuthMiddlewareInitErr.Error())
		panic(jwtAuthMiddlewareInitErr.Error())
	}

	// Setup JWT authentication and authorization middleware, aka user-aware
	jwtAPIUserAwareMiddleware, jwtAPIAuthSetupErr := JWTMiddleware(proviantConfiguration, dbHandle, AuthorizatorUserAware, UnauthorizedAPIFunc)
	if jwtAPIAuthSetupErr != nil {
		logger.Error().Msg(jwtAPIAuthSetupErr.Error())
		panic(jwtAPIAuthSetupErr.Error())
	}
	// Initialize JWT authentication and authorization middleware
	jwtAuthUserAwareMiddlewareInitErr := jwtAPIUserAwareMiddleware.MiddlewareInit()
	if jwtAuthUserAwareMiddlewareInitErr != nil {
		logger.Error().Msg(jwtAuthUserAwareMiddlewareInitErr.Error())
		panic(jwtAuthUserAwareMiddlewareInitErr.Error())
	}

	// Setup JWT authentication middleware for Frontend
	jwtFrontendMiddleware, jwtFrontendAuthSetupErr := JWTMiddleware(proviantConfiguration, dbHandle, AuthorizatorNotUserAware, UnauthorizedFrontendFunc)
	if jwtFrontendAuthSetupErr != nil {
		logger.Error().Msg(jwtFrontendAuthSetupErr.Error())
		panic(jwtFrontendAuthSetupErr.Error())
	}
	// Initialize JWT authentication middleware
	jwtFrontendAuthMiddlewareInitErr := jwtFrontendMiddleware.MiddlewareInit()
	if jwtFrontendAuthMiddlewareInitErr != nil {
		logger.Error().Msg(errors.ErrAuthMiddlewareInit.Error())
		panic(errors.ErrAuthMiddlewareInit.Error())
	}

	// Setup JWT authentication and authorization middleware, aka user-aware
	jwtFrontendUserAwareMiddleware, jwtFrontendAuthSetupErr := JWTMiddleware(proviantConfiguration, dbHandle, AuthorizatorUserAware, UnauthorizedFrontendFunc)
	if jwtFrontendAuthSetupErr != nil {
		logger.Error().Msgf("%s: %s", errors.ErrUserAwareAuthMiddlewareInit.Error(), jwtFrontendAuthSetupErr.Error())
		panic(errors.ErrUserAwareAuthMiddlewareInit.Error())
	}
	// Initialize JWT authentication and authorization middleware
	jwtFrontendAuthUserAwareMiddlewareInitErr := jwtFrontendUserAwareMiddleware.MiddlewareInit()
	if jwtFrontendAuthUserAwareMiddlewareInitErr != nil {
		logger.Error().Msg(errors.ErrUserAwareAuthMiddlewareInit.Error())
		panic(errors.ErrUserAwareAuthMiddlewareInit.Error())
	}

	// Map routes to handlers
	// Health routes
	engine.GET("/health", common.GetHealth)

	// Favicon redirect
	engine.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusPermanentRedirect, "/assets/icons/favicon.ico")
	})

	// Authentication routes
	engine.POST("/auth/login", jwtAPIMiddleware.LoginHandler)

	// Signup routes
	engine.POST("/auth/signup", auth.Signup)
	engine.GET("/auth/refresh_token", jwtAPIMiddleware.RefreshHandler)

	// Public product routes
	publicProductAPI := engine.Group("/api/v1/products")
	publicProductAPI.Use(jwtAPIMiddleware.MiddlewareFunc())
	publicProductAPI.GET("", v1.GetProducts)
	publicProductAPI.GET("/archived", v1.GetArchivedProducts)
	publicProductAPI.GET("/expired", v1.GetExpired)
	publicProductAPI.POST("", v1.CreateProduct)
	publicProductAPI.POST("/scan", v1.ScanProduct)
	publicProductAPI.GET("/byBarcode/:barcode", v1.GetProductsByBarcode)
	publicProductAPI.GET("/search", v1.SearchProducts)

	// Protected user routes
	protectedUserAPI := engine.Group("/api/v1/user")
	protectedUserAPI.Use(jwtAPIMiddleware.MiddlewareFunc())
	protectedUserAPI.PATCH("", v1.UpdateUser)
	protectedUserAPI.POST("/password", v1.UpdateUserPassword)

	// Protected product routes
	protectedProductAPI := engine.Group("/api/v1/products")
	protectedProductAPI.Use(jwtAPIUserAwareMiddleware.MiddlewareFunc())
	protectedProductAPI.GET("/:id", v1.GetProduct)
	protectedProductAPI.PATCH("/:id", v1.UpdateProduct)
	protectedProductAPI.DELETE("/:id", v1.DeleteProduct)
	protectedProductAPI.POST("/:id/restore", v1.RestoreProduct)
	protectedProductAPI.POST("/:id/expire", v1.SetExpireAt)

	// Web frontend routes
	// Serve asset files
	engine.StaticFS("/assets", http.FS(assets.AssetFiles))
	// Create frontend handler with template cache
	webFrontendHandler := web.Frontend{TemplateCache: proviantConfiguration.TemplateCache}
	webFrontend := engine.Group("/web")
	webFrontend.GET("/auth", webFrontendHandler.Auth)

	// Public web frontend routes
	publicWebFrontend := engine.Group("/web")
	publicWebFrontend.Use(jwtFrontendMiddleware.MiddlewareFunc())
	publicWebFrontend.GET("/", webFrontendHandler.Root)
	publicWebFrontend.GET("/user", webFrontendHandler.User)
	publicWebFrontend.GET("/user/settings", webFrontendHandler.UserSettings)
	publicWebFrontend.GET("/products", webFrontendHandler.Products)
	publicWebFrontend.GET("/products/archived", webFrontendHandler.ProductsArchived)
	publicWebFrontend.GET("/products/create", webFrontendHandler.ProductsCreate)
	publicWebFrontend.GET("/products/search", webFrontendHandler.Search)

	// Protected web frontend routes
	protectedWebFrontend := engine.Group("/web")
	protectedWebFrontend.Use(jwtFrontendUserAwareMiddleware.MiddlewareFunc())
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
