package router

import (
	"strings"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func PATMiddleware(jwtMiddleware *jwt.GinJWTMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, controllers.TokenPrefix) {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			dbHandle, ok := c.MustGet("dbHandle").(*gorm.DB)
			if !ok {
				c.AbortWithStatus(401)
				return
			}

			pat, err := controllers.ValidateAndLookupPAT(token, dbHandle)
			if err == nil {
				c.Set("pat", pat)
				c.Set("userID", pat.UserID)
				c.Set(static.UserIDContextKey, pat.UserID)
				go func() {
					patRepo := database.NewPATRepository(dbHandle)
					_ = patRepo.UpdateLastUsed(pat.ID)
				}()

				// Enrich logger with user_id
				if existing, ok := c.MustGet("logger").(*zerolog.Logger); ok {
					enriched := existing.With().Uint("user_id", pat.UserID).Logger()
					c.Set("logger", &enriched)
				}

				c.Next()
				return
			}
		}
		jwtMiddleware.MiddlewareFunc()(c)
	}
}
