package router

import (
	"expiro/backend/controllers"
	common "expiro/backend/handlers"
	v1 "expiro/backend/handlers/v1"
	"time"

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

	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Set("cntrl", cntrl)
		c.Next()
	})

	// Health routes
	r.GET("/health", common.GetHealth)
	// Product routes
	r.GET("/api/v1/products", v1.GetProducts)
	r.GET("/api/v1/products/:id", v1.GetProduct)
	r.POST("/api/v1/products", v1.CreateProduct)
	r.PATCH("/api/v1/products/:id", v1.UpdateProduct)
	r.DELETE("/api/v1/products/:id", v1.DeleteProduct)
	r.POST("/api/v1/products/:id/expire", v1.SetExpireAt)

	return r
}
