package router

import (
	"expiro/backend/controllers"
	"expiro/backend/handlers/auth"
	"expiro/backend/handlers/common"
	v1 "expiro/backend/handlers/v1"
	"expiro/backend/models/configuration"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func LoggerMiddleware(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process the request
		c.Next()

		// Log the request details
		logger.Info().
			Str("remote", c.Request.RemoteAddr).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration", time.Since(start)).
			Msg("Request handled")
	}
}

func SetupRouter(logger *zerolog.Logger, configuration *configuration.ExpiroConfiguration, db *gorm.DB, cntrl controllers.OpenFoodFactsAPIController) *gin.Engine {
	r := gin.New()

	r.Use(LoggerMiddleware(logger), gin.Recovery())

	// Setup custom middleware
	// Logging
	r.Use(func(c *gin.Context) {
		c.Set("logger", logger)
		c.Next()
	})
	// Database
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	// JWT Authorization
	jwtMiddleware, jwtAuthSetupErr := auth.JWTMiddleware(configuration, db)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up authorization middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up authorization middleware")
	}
	jwtAuthMiddlewareInitErr := jwtMiddleware.MiddlewareInit()
	if jwtAuthMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing authorization middleware")
		panic("Error initializing authorization middleware")
	}
	// OpenFoodFactsAPI Controller
	r.Use(func(c *gin.Context) {
		c.Set("cntrl", cntrl)
		c.Next()
	})

	// Health routes
	r.GET("/health", common.GetHealth)
	// Authentication routes
	r.POST("/auth/login", jwtMiddleware.LoginHandler)
	// Signup routes
	r.POST("/auth/signup", auth.Signup)
	r.GET("/auth/refresh_token", jwtMiddleware.RefreshHandler)
	// Product routes
	productAPI := r.Group("/api/v1/products")
	productAPI.Use(jwtMiddleware.MiddlewareFunc())
	productAPI.GET("", v1.GetProducts)
	productAPI.GET("/:id", v1.GetProduct)
	productAPI.POST("", v1.CreateProduct)
	productAPI.PATCH("/:id", v1.UpdateProduct)
	productAPI.DELETE("/:id", v1.DeleteProduct)
	productAPI.POST("/:id/expire", v1.SetExpireAt)
	productAPI.GET("/expired", v1.GetExpired)
	// Catch-All handler
	r.NoRoute(jwtMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		logger.Error().Msgf("NoRoute claims: %#v\n", claims)
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	return r
}
