// router contains the gin router definitions and maps requests to handlers
package router

import (
	"net/http"
	"strconv"
	"time"

	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/handlers/auth"
	"gitlab.com/Isotop7/expiro/handlers/common"
	v1 "gitlab.com/Isotop7/expiro/handlers/v1"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/configuration"
	"gitlab.com/Isotop7/expiro/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ZerologMiddleware implements a gin.HandlerFunc and logs the output from gin
func ZerologMiddleware(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get start of request
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

// JWTMiddleware implements a jwt.GinJWTMiddleware for authentication and authorization (optional)
func JWTMiddleware(configuration *configuration.ExpiroConfiguration, db *gorm.DB, userAware bool) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		// JWT configuration and timeouts
		Realm:         static.TokenRealm,
		Key:           []byte(configuration.Server.Authentication.TokenPassword),
		Timeout:       (time.Duration(configuration.Server.Authentication.TokenLifetime) * time.Hour),
		MaxRefresh:    (time.Duration(configuration.Server.Authentication.TokenLifetime) * time.Hour),
		IdentityKey:   static.TokenIdentityKey,
		TokenLookup:   static.TokenLookup,
		TokenHeadName: static.TokenHeadName,
		TimeFunc:      time.Now,
		// Generate claims and return it to payload
		PayloadFunc: func(data any) jwt.MapClaims {
			if v, ok := data.(authentication.User); ok {
				return jwt.MapClaims{
					static.TokenIdentityKey: v.ID,
					static.TokenUsernameKey: v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		// Extract claims from context
		IdentityHandler: func(c *gin.Context) any {
			claims := jwt.ExtractClaims(c)
			return &authentication.User{
				ID:       uint(claims[static.TokenIdentityKey].(float64)),
				Username: claims[static.TokenUsernameKey].(string),
			}
		},
		// Authenticate user from context
		Authenticator: func(c *gin.Context) (any, error) {
			// Get and parse login credentials
			var loginVals authentication.Login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			// Create database controller
			dbController := controllers.DatabaseController{DB: db}
			// Get user object by username
			user, err := dbController.GetUserByUsername(loginVals.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			// Compare supplied password with database hash
			// Return result of comparison
			authErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginVals.Password))
			if authErr != nil {
				return nil, jwt.ErrFailedAuthentication
			} else {
				return user, nil
			}
		},
		// Authorizator checks if user is authorized to emit operation
		Authorizator: func(data any, c *gin.Context) bool {
			// If middleware is not user-aware, exit
			if !userAware {
				return true
			}

			// Get user data from data context
			user, ok := data.(*authentication.User)
			if !ok {
				return false
			}

			// Get and convert parameter 'id' from request
			idParam := c.Param("id")
			var convErr error
			var productID int
			if productID, convErr = strconv.Atoi(idParam); convErr != nil {
				return false
			}

			// Create database controller
			dbController := controllers.DatabaseController{DB: db}
			// Call database controller function that returns owner state
			return dbController.UserIsProductOwner(user.ID, productID)
		},
		// Unauthorized implements the return function if user is not authorized
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
	})
}

// SetupRouter creates the gin engine and associated middleware
func SetupRouter(logger *zerolog.Logger, configuration *configuration.ExpiroConfiguration, db *gorm.DB, offacntrl controllers.OpenFoodFactsAPIController) *gin.Engine {
	// Generate new gin instance
	engine := gin.New()

	// Inject logging middleware
	engine.Use(ZerologMiddleware(logger), gin.Recovery())

	// Pass references to gin context
	// Logging
	engine.Use(func(c *gin.Context) {
		c.Set("logger", logger)
		c.Next()
	})

	// Database
	engine.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// OpenFoodFactsAPI Controller
	engine.Use(func(c *gin.Context) {
		c.Set("offacntrl", offacntrl)
		c.Next()
	})

	// Setup JWT authentication middleware
	jwtMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, db, false)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up authentication middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up authentication middleware")
	}
	// Initialize JWT authentication middleware
	jwtAuthMiddlewareInitErr := jwtMiddleware.MiddlewareInit()
	if jwtAuthMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing user-aware authentication middleware")
		panic("Error initializing authentication middleware")
	}

	// Setup JWT authentication and authorization middleware, aka user-aware
	jwtUserAwareMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, db, true)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up user-aware authentication middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up user-aware authentication middleware")
	}
	// Initialize JWT authentication and authorization middleware
	jwtAuthUserAwareMiddlewareInitErr := jwtUserAwareMiddleware.MiddlewareInit()
	if jwtAuthUserAwareMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing user-aware authentication middleware")
		panic("Error initializing user-aware authentication middleware")
	}

	// Map routes to handlers
	// Health routes
	engine.GET("/health", common.GetHealth)

	// Authentication routes
	engine.POST("/auth/login", jwtMiddleware.LoginHandler)

	// Signup routes
	engine.POST("/auth/signup", auth.Signup)
	engine.GET("/auth/refresh_token", jwtMiddleware.RefreshHandler)

	// Public product routes
	publicProductAPI := engine.Group("/api/v1/products")
	publicProductAPI.Use(jwtMiddleware.MiddlewareFunc())
	publicProductAPI.GET("", v1.GetProducts)
	publicProductAPI.GET("/expired", v1.GetExpired)
	publicProductAPI.POST("", v1.CreateProduct)

	// Protected product routes
	protectedProductAPI := engine.Group("/api/v1/products")
	protectedProductAPI.Use(jwtUserAwareMiddleware.MiddlewareFunc())
	protectedProductAPI.GET("/:id", v1.GetProduct)
	protectedProductAPI.PATCH("/:id", v1.UpdateProduct)
	protectedProductAPI.DELETE("/:id", v1.DeleteProduct)
	protectedProductAPI.POST("/:id/expire", v1.SetExpireAt)

	// Catch-All handler
	engine.NoRoute(jwtMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		logger.Error().Msgf("NoRoute claims: %#v\n", claims)
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	// Return engine to caller
	return engine
}
