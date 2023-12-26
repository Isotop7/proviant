package router

import (
	"expiro/backend/controllers"
	common "expiro/backend/handlers"
	"expiro/backend/handlers/auth"
	v1 "expiro/backend/handlers/v1"
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

func SetupRouter(logger *zerolog.Logger, db *gorm.DB, cntrl controllers.OpenFoodFactsAPIController) *gin.Engine {
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
	jwtAuthMiddleware, jwtAuthSetupErr := auth.AuthorizationMiddleware(db)
	if jwtAuthSetupErr != nil {
		logger.Error().Msg("Error setting up authorization middleware")
		panic("Error setting up authorization middleware")
	}
	jwtAuthMiddlewareInitErr := jwtAuthMiddleware.MiddlewareInit()
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
	r.POST("/auth/login", jwtAuthMiddleware.LoginHandler)
	r.NoRoute(jwtAuthMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		logger.Error().Msgf("NoRoute claims: %#v\n", claims)
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})
	// Signup routes
	r.POST("/auth/signup", auth.Signup)
	r.GET("/auth/refresh_token", jwtAuthMiddleware.RefreshHandler)
	// Product routes
	productAPI := r.Group("/api/v1/products")
	productAPI.Use(jwtAuthMiddleware.MiddlewareFunc())
	productAPI.GET("", v1.GetProducts)
	productAPI.GET("/:id", v1.GetProduct)
	productAPI.POST("", v1.CreateProduct)
	productAPI.PATCH("/:id", v1.UpdateProduct)
	productAPI.DELETE("/:id", v1.DeleteProduct)
	productAPI.POST("/:id/expire", v1.SetExpireAt)
	productAPI.GET("/expired", v1.GetExpired)

	return r
}
