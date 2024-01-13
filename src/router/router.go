// router contains the gin router definitions and maps requests to handlers
package router

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab.com/Isotop7/expiro/api/auth"
	"gitlab.com/Isotop7/expiro/api/common"
	v1 "gitlab.com/Isotop7/expiro/api/v1"
	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/configuration"
	"gitlab.com/Isotop7/expiro/models/configuration/static"
	"gitlab.com/Isotop7/expiro/templates"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ZerologMiddleware implements a gin.HandlerFunc and logs the output from gin
func ZerologMiddleware(logger *zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get start of request
		start := time.Now()

		// Process the request
		ctx.Next()

		// Log the request details
		logger.Info().
			Str("remote", ctx.Request.RemoteAddr).
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status", ctx.Writer.Status()).
			Dur("duration", time.Since(start)).
			Msg("Request handled")
	}
}

// JWTMiddleware implements a jwt.GinJWTMiddleware for authentication and authorization (optional)
func JWTMiddleware(configuration *configuration.ExpiroConfiguration, dbHandle *gorm.DB, userAware bool) (*jwt.GinJWTMiddleware, error) {
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
		IdentityHandler: func(ctx *gin.Context) any {
			claims := jwt.ExtractClaims(ctx)
			return &authentication.User{
				ID:       uint(claims[static.TokenIdentityKey].(float64)),
				Username: claims[static.TokenUsernameKey].(string),
			}
		},
		// Authenticate user from context
		Authenticator: func(ctx *gin.Context) (any, error) {
			// Get and parse login credentials
			var loginVals authentication.Login
			if err := ctx.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			// Create database controller
			dbController := controllers.DatabaseController{DBHandle: dbHandle}
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
		Authorizator: func(data any, ctx *gin.Context) bool {
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
			idParam := ctx.Param("id")
			var convErr error
			var productID int
			if productID, convErr = strconv.Atoi(idParam); convErr != nil {
				return false
			}

			// Create database controller
			dbController := controllers.DatabaseController{DBHandle: dbHandle}
			// Call database controller function that returns owner state
			return dbController.UserIsProductOwner(user.ID, productID)
		},
		// Unauthorized implements the return function if user is not authorized
		Unauthorized: func(ctx *gin.Context, code int, message string) {
			ctx.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
	})
}

// SetupRouter creates the gin engine and associated middleware
func SetupRouter(logger *zerolog.Logger, configuration *configuration.ExpiroConfiguration, dbHandle *gorm.DB, offacntrl controllers.OpenFoodFactsAPIController) *gin.Engine {
	// Generate new gin instance
	engine := gin.New()

	// Inject logging middleware
	engine.Use(ZerologMiddleware(logger), gin.Recovery())

	// Setup cors
	corsConfig := cors.DefaultConfig()
	// Check if any origin is allowed or set list
	if configuration.Server.CORS.AllowAllOrigins {
		corsConfig.AllowAllOrigins = true
		logger.Info().Msg("Allowed all CORS origins")
	} else {
		corsConfig.AllowAllOrigins = false
		corsConfig.AllowOrigins = configuration.Server.CORS.AllowedOrigins
		logger.Info().Msgf("Allowed CORS origins: %s", strings.Join(configuration.Server.CORS.AllowedOrigins, "; "))
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

	// Setup JWT authentication middleware
	jwtMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, dbHandle, false)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("Error setting up authentication middleware: %s", jwtAuthSetupErr.Error())
		panic("Error setting up authentication middleware")
	}
	// Initialize JWT authentication middleware
	jwtAuthMiddlewareInitErr := jwtMiddleware.MiddlewareInit()
	if jwtAuthMiddlewareInitErr != nil {
		logger.Error().Msg("Error initializing authentication middleware")
		panic("Error initializing authentication middleware")
	}

	// Setup JWT authentication and authorization middleware, aka user-aware
	jwtUserAwareMiddleware, jwtAuthSetupErr := JWTMiddleware(configuration, dbHandle, true)
	if jwtAuthSetupErr != nil {
		logger.Error().Msgf("%s: %s", errors.ErrUserAwareAuthMiddlewareInit.Error(), jwtAuthSetupErr.Error())
		panic(errors.ErrUserAwareAuthMiddlewareInit.Error())
	}
	// Initialize JWT authentication and authorization middleware
	jwtAuthUserAwareMiddlewareInitErr := jwtUserAwareMiddleware.MiddlewareInit()
	if jwtAuthUserAwareMiddlewareInitErr != nil {
		logger.Error().Msg(errors.ErrUserAwareAuthMiddlewareInit.Error())
		panic(errors.ErrUserAwareAuthMiddlewareInit.Error())
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
	publicProductAPI.POST("/scan", v1.ScanProduct)

	// Protected product routes
	protectedProductAPI := engine.Group("/api/v1/products")
	protectedProductAPI.Use(jwtUserAwareMiddleware.MiddlewareFunc())
	protectedProductAPI.GET("/:id", v1.GetProduct)
	protectedProductAPI.PATCH("/:id", v1.UpdateProduct)
	protectedProductAPI.DELETE("/:id", v1.DeleteProduct)
	protectedProductAPI.POST("/:id/expire", v1.SetExpireAt)

	// Web frontend routes
	engine.Static("/static", "./static")
	webFrontend := engine.Group("/web")
	webFrontend.GET("/", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "home.tmpl")
	})
	webFrontend.GET("/auth", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "auth.tmpl")
	})
	webFrontend.GET("/auth/login", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "authLogin.tmpl")
	})
	webFrontend.GET("/auth/register", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "authRegister.tmpl")
	})

	// Protected web frontend routes
	protectedWebFrontend := engine.Group("/web")
	protectedWebFrontend.Use(jwtMiddleware.MiddlewareFunc())
	protectedWebFrontend.GET("/user", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "user.tmpl")
	})
	protectedWebFrontend.GET("/user/settings", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "userSettings.tmpl")
	})
	protectedWebFrontend.GET("/products", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "products.tmpl")
	})
	protectedWebFrontend.GET("/products/create", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "productsCreate.tmpl")
	})
	protectedWebFrontend.GET("/products/scan", func(ctx *gin.Context) {
		templates.Render(ctx, configuration.TemplateCache, http.StatusOK, "productsScan.tmpl")
	})

	// Catch-All handler
	engine.NoRoute(jwtMiddleware.MiddlewareFunc(), func(ctx *gin.Context) {
		claims := jwt.ExtractClaims(ctx)
		logger.Error().Msgf("NoRoute claims: %#v\n", claims)
		ctx.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	// Return engine to caller
	return engine
}
