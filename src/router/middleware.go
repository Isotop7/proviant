package router

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/templates"
	jwt "github.com/appleboy/gin-jwt/v2"
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

func UnauthorizedAPIFunc(ctx *gin.Context, code int, message string) {
	failedUserID, failedUserIDExists := ctx.Get("failedUserID")
	if !failedUserIDExists {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}

	proviantConfig, _ := ctx.MustGet("proviantConfig").(*configuration.ProviantConfiguration)
	maxLoginAttempts := database.DefaultMaxLoginAttempts
	lockoutDurationMins := database.DefaultLockoutDurationMins
	if proviantConfig != nil {
		if proviantConfig.Server.Authentication.MaxLoginAttempts > 0 {
			maxLoginAttempts = proviantConfig.Server.Authentication.MaxLoginAttempts
		}
		if proviantConfig.Server.Authentication.LockoutDurationMins > 0 {
			lockoutDurationMins = proviantConfig.Server.Authentication.LockoutDurationMins
		}
	}

	locked, remaining := dbController.IsAccountLocked(failedUserID.(uint), maxLoginAttempts, lockoutDurationMins)
	if !locked {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	retryAfter := int(remaining.Seconds())
	ctx.Header("Retry-After", strconv.Itoa(retryAfter))
	ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code":    "ACCOUNT_LOCKED",
		"message": "Too many failed login attempts. Account is temporarily locked.",
	})
}

func UnauthorizedFrontendFunc(ctx *gin.Context, code int, message string) {
	// Redirect on unauthorized error
	if code == http.StatusUnauthorized {
		ctx.Redirect(http.StatusTemporaryRedirect, "/web/auth")
		return
	}

	// Get template cache instance from context and fail if not found
	templateCache, ok := ctx.MustGet("templateCache").(map[string]*template.Template)
	if !ok {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	templates.RenderError(ctx, templateCache, code, message)
}

func AuthorizatorUserAware(data any, ctx *gin.Context) bool {
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

	// Get database instance from context and fail if not found
	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		return false
	}
	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Call database controller function that returns owner state
	return dbController.UserHasProductAccess(user.ID, productID)
}

func AuthorizatorNotUserAware(data any, ctx *gin.Context) bool {
	// Always return true
	return true
}

// JWTMiddleware implements a jwt.GinJWTMiddleware for authentication and authorization (optional)
func JWTMiddleware(
	proviantConfiguration *configuration.ProviantConfiguration,
	dbHandle *gorm.DB,
	authorizatorFunc func(data any, ctx *gin.Context) bool,
	unauthorizedFunc func(ctx *gin.Context, code int, message string)) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		// JWT configuration and timeouts
		Realm:         static.TokenRealm,
		Key:           []byte(proviantConfiguration.Server.Authentication.TokenPassword),
		Timeout:       (time.Duration(proviantConfiguration.Server.Authentication.TokenLifetime) * time.Hour),
		MaxRefresh:    (time.Duration(proviantConfiguration.Server.Authentication.TokenLifetime) * time.Hour),
		IdentityKey:   static.TokenIdentityKey,
		TokenLookup:   static.TokenLookup,
		TokenHeadName: static.TokenHeadName,
		TimeFunc:      time.Now,
		SendCookie:    true,
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
			dbController := database.DatabaseController{DBHandle: dbHandle}

			// Get config values with defaults
			maxLoginAttempts := database.DefaultMaxLoginAttempts
			lockoutDurationMins := database.DefaultLockoutDurationMins
			if proviantConfiguration != nil {
				if proviantConfiguration.Server.Authentication.MaxLoginAttempts > 0 {
					maxLoginAttempts = proviantConfiguration.Server.Authentication.MaxLoginAttempts
				}
				if proviantConfiguration.Server.Authentication.LockoutDurationMins > 0 {
					lockoutDurationMins = proviantConfiguration.Server.Authentication.LockoutDurationMins
				}
			}

			// Get user object by username
			user, err := dbController.GetUserByUsername(loginVals.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			// Check if account is already locked
			if locked, _ := dbController.IsAccountLocked(user.ID, maxLoginAttempts, lockoutDurationMins); locked {
				ctx.Set("failedUserID", user.ID)
				return nil, jwt.ErrFailedAuthentication
			}

			// Compare supplied password with database hash
			// Return result of comparison
			authErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginVals.Password))
			if authErr != nil {
				_ = dbController.RecordFailedLoginAttempt(user.ID, maxLoginAttempts, lockoutDurationMins)
				ctx.Set("failedUserID", user.ID)
				return nil, jwt.ErrFailedAuthentication
			}

			// Successful login - reset failed attempts
			_ = dbController.ResetFailedLoginAttempts(user.ID)
			return user, nil
		},
		// Authorizator checks if user is authorized to emit operation
		Authorizator: authorizatorFunc,
		// Unauthorized implements the return function if user is not authorized
		Unauthorized: unauthorizedFunc,
	})
}
