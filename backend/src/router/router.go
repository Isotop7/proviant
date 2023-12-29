package router

import (
	"expiro/controllers"
	"expiro/handlers/auth"
	"expiro/handlers/common"
	v1 "expiro/handlers/v1"
	"expiro/models/authentication"
	"expiro/models/configuration"
	"expiro/models/configuration/static"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
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

func JWTMiddleware(configuration *configuration.ExpiroConfiguration, db *gorm.DB, userAware bool) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:       static.TokenRealm,
		Key:         []byte(configuration.Server.Authentication.TokenPassword),
		Timeout:     (time.Duration(configuration.Server.Authentication.TokenLifetime) * time.Hour),
		MaxRefresh:  (time.Duration(configuration.Server.Authentication.TokenLifetime) * time.Hour),
		IdentityKey: static.TokenIdentityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(authentication.User); ok {
				return jwt.MapClaims{
					static.TokenIdentityKey: v.ID,
					static.TokenUsernameKey: v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &authentication.User{
				ID:       uint(claims[static.TokenIdentityKey].(float64)),
				Username: claims[static.TokenUsernameKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var loginVals authentication.Login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			dbController := controllers.DatabaseController{DB: db}
			user, err := dbController.FindUserByUsername(loginVals.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			authErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginVals.Password))
			if authErr != nil {
				return nil, jwt.ErrFailedAuthentication
			} else {
				return user, nil
			}
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
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

			// Get database handle from context
			db, ok := c.MustGet("db").(*gorm.DB)
			if !ok {
				return false
			}
			dbController := controllers.DatabaseController{DB: db}

			// Call database controller function that returns owner state
			return dbController.UserIsProductOwner(user.ID, productID)
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},

		TokenLookup:   static.TokenLookup,
		TokenHeadName: static.TokenHeadName,
		TimeFunc:      time.Now,
	})
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

	// JWT Authentication
	jwtMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, db, false)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up authentication middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up authentication middleware")
	}
	jwtAuthMiddlewareInitErr := jwtMiddleware.MiddlewareInit()
	if jwtAuthMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing user-aware authentication middleware")
		panic("Error initializing authentication middleware")
	}

	// JWT Authentication and Authorization, aka user-aware
	jwtUserAwareMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, db, true)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up user-aware authentication middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up user-aware authentication middleware")
	}
	jwtAuthUserAwareMiddlewareInitErr := jwtUserAwareMiddleware.MiddlewareInit()
	if jwtAuthUserAwareMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing user-aware authentication middleware")
		panic("Error initializing user-aware authentication middleware")
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

	// Public product routes
	publicProductAPI := r.Group("/api/v1/products")
	publicProductAPI.Use(jwtMiddleware.MiddlewareFunc())
	publicProductAPI.GET("", v1.GetProducts)
	publicProductAPI.GET("/expired", v1.GetExpired)
	publicProductAPI.POST("", v1.CreateProduct)

	// Protected product routes
	protectedProductAPI := r.Group("/api/v1/products")
	protectedProductAPI.Use(jwtUserAwareMiddleware.MiddlewareFunc())
	protectedProductAPI.GET("/:id", v1.GetProduct)
	protectedProductAPI.PATCH("/:id", v1.UpdateProduct)
	protectedProductAPI.DELETE("/:id", v1.DeleteProduct)
	protectedProductAPI.POST("/:id/expire", v1.SetExpireAt)

	// Catch-All handler
	r.NoRoute(jwtMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		logger.Error().Msgf("NoRoute claims: %#v\n", claims)
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	return r
}
