package router

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/util"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const MsgInvalidCredentials = "Invalid credentials"

var errEmailNotVerified = errors.New("email not verified")

func RequireHouseholdAdmin() gin.HandlerFunc {
	return RequireHouseholdRole(authentication.RoleAdmin)
}

func RequireHouseholdRole(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		dbHandle, _ := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)

		userIDVal, exists := ctx.Get(util.ContextKeyUserID)
		if !exists {
			api.RespondError(ctx, http.StatusUnauthorized, errors.New("user not authenticated"))
			ctx.Abort()
			return
		}
		userID := userIDVal.(uint)

		userRepo := database.NewUserRepository(dbHandle)
		user, err := userRepo.GetUserByID(userID)
		if err != nil {
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
			ctx.Abort()
			return
		}

		hasRole := slices.Contains(roles, user.Role)
		if !hasRole {
			api.RespondError(ctx, http.StatusForbidden, apperrors.ErrInsufficientRole)
			ctx.Abort()
			return
		}

		ctx.Set(util.ContextKeyHouseholdID, user.HouseholdID)
		ctx.Next()
	}
}

// parseRequestID validates if the string is a valid UUID, returns empty string if not
func parseRequestID(s string) string {
	if s == "" {
		return ""
	}
	if _, err := uuid.Parse(s); err == nil {
		return s
	}
	return ""
}

// RequestIDMiddleware mints a UUID per request, stashes it in gin.Context,
// sets the response header, and replaces the context logger with a child logger
func RequestIDMiddleware(baseLogger *zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Honor incoming X-Request-ID if present and valid; else mint
		reqID := parseRequestID(ctx.GetHeader(static.RequestIDHeader))
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx.Set(util.ContextKeyRequestID, reqID)
		ctx.Header(static.RequestIDHeader, reqID)

		// Replace context logger with child carrying request_id
		l := baseLogger.With().Str("request_id", reqID).Logger()
		ctx.Set(util.ContextKeyLogger, &l)

		ctx.Next()
	}
}

// UserContextLoggerMiddleware enriches the context logger with user_id after JWT/PAT auth
func UserContextLoggerMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims := jwt.ExtractClaims(ctx)
		if raw, ok := claims[static.TokenIdentityKey]; ok {
			if f, ok := raw.(float64); ok {
				uid := uint(f)
				ctx.Set(util.ContextKeyUserID, uid)

				// Enrich ctx logger with user_id
				if existing, ok := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger); ok {
					enriched := existing.With().Uint("user_id", uid).Logger()
					ctx.Set(util.ContextKeyLogger, &enriched)
				}
			}
		}
		ctx.Next()
	}
}

// ZerologMiddleware implements a gin.HandlerFunc and logs the output from gin
func ZerologMiddleware(logger *zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get start of request
		start := time.Now()

		// Process the request
		ctx.Next()

		// Read request_id and user_id from context
		reqID, _ := ctx.Get(util.ContextKeyRequestID)
		userID, _ := ctx.Get(util.ContextKeyUserID)

		// Log the request details
		evt := logger.Info().
			Str("remote", ctx.Request.RemoteAddr).
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status", ctx.Writer.Status()).
			Dur("duration", time.Since(start))
		if reqID != nil {
			evt = evt.Str("request_id", reqID.(string))
		}
		if userID != nil {
			evt = evt.Uint("user_id", userID.(uint))
		}
		evt.Msg("Request handled")
	}
}

func UnauthorizedAPIFunc(ctx *gin.Context, code int, message string) {
	if message == errEmailNotVerified.Error() {
		api.RespondError(ctx, http.StatusForbidden, errEmailNotVerified)
		return
	}

	failedUserID, failedUserIDExists := ctx.Get("failedUserID")
	if !failedUserIDExists {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	dbHandle, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	userRepo := database.NewUserRepository(dbHandle)

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	maxLoginAttempts, lockoutDurationMins := lockoutConfig(proviantConfig)

	locked, remaining := userRepo.IsAccountLocked(failedUserID.(uint), maxLoginAttempts, lockoutDurationMins)
	if !locked {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	retryAfter := int(remaining.Seconds())
	ctx.Header("Retry-After", strconv.Itoa(retryAfter))
	api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many failed login attempts, account is temporarily locked"))
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
	// Check if token is revoked
	if isTokenRevoked(ctx) {
		return false
	}

	// Get user data from data context
	user, ok := data.(*authentication.User)
	if !ok {
		return false
	}

	// Get and convert parameter 'id' from request
	idParam := ctx.Param("id")
	var err error
	var productID int
	if productID, err = strconv.Atoi(idParam); err != nil {
		return false
	}

	// Get database instance from context and fail if not found
	dbHandle, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		return false
	}
	productRepo := database.NewProductRepository(dbHandle)
	return productRepo.UserHasProductAccess(user.ID, productID)
}

func AuthorizatorShoppingListItem(data any, ctx *gin.Context) bool {
	if isTokenRevoked(ctx) {
		return false
	}

	user, ok := data.(*authentication.User)
	if !ok {
		return false
	}

	idParam := ctx.Param("id")
	itemID, err := strconv.Atoi(idParam)
	if err != nil || itemID < 0 {
		return false
	}

	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return false
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return false
	}

	householdID, err := repos.Users.GetUserHouseholdByID(user.ID)
	if err != nil {
		return false
	}

	_, err = repos.ShoppingListItems.GetByID(uint(itemID), householdID)
	return err == nil
}

func isTokenRevoked(ctx *gin.Context) bool {
	claims := jwt.ExtractClaims(ctx)
	jti, exists := claims[static.TokenJTIKey]
	if !exists {
		// If no JTI, allow (for backward compatibility)
		return false
	}

	jtiStr, ok := jti.(string)
	if !ok {
		// If JTI not string, allow
		return false
	}

	dbHandle, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		return false
	}

	var revokedToken authentication.RevokedToken
	err := dbHandle.Where("jti = ?", jtiStr).First(&revokedToken).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not revoked
			return false
		}
		// Error querying, allow to avoid blocking valid users
		return false
	}

	// Check if token is expired (cleanup might not have run yet)
	if revokedToken.ExpiresAt.Before(time.Now()) {
		// Expired, can clean up in background
		return false
	}

	// Token is revoked
	return true
}

func AuthorizatorNotUserAware(data any, ctx *gin.Context) bool {
	// Check if token is revoked
	if isTokenRevoked(ctx) {
		return false
	}

	// Always return true
	return true
}

func lockoutConfig(cfg *configuration.ProviantConfiguration) (maxAttempts, lockoutMins int) {
	maxAttempts = database.DefaultMaxLoginAttempts
	lockoutMins = database.DefaultLockoutDurationMins
	if cfg == nil {
		return
	}
	if cfg.Server.Authentication.MaxLoginAttempts > 0 {
		maxAttempts = cfg.Server.Authentication.MaxLoginAttempts
	}
	if cfg.Server.Authentication.LockoutDurationMins > 0 {
		lockoutMins = cfg.Server.Authentication.LockoutDurationMins
	}
	return
}

// JWTMiddleware implements a jwt.GinJWTMiddleware for authentication and authorization (optional)
func JWTMiddleware(
	proviantConfiguration *configuration.ProviantConfiguration,
	dbHandle *gorm.DB,
	authorizatorFunc func(data any, ctx *gin.Context) bool,
	unauthorizedFunc func(ctx *gin.Context, code int, message string)) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		// JWT configuration and timeouts
		Realm:          static.TokenRealm,
		Key:            []byte(proviantConfiguration.Server.Authentication.TokenPassword),
		Timeout:        (time.Duration(proviantConfiguration.Server.Authentication.TokenLifetime) * time.Hour),
		MaxRefresh:     (time.Duration(proviantConfiguration.Server.Authentication.TokenLifetime) * time.Hour),
		IdentityKey:    static.TokenIdentityKey,
		TokenLookup:    static.TokenLookup,
		TokenHeadName:  static.TokenHeadName,
		TimeFunc:       time.Now,
		SendCookie:     true,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		// Generate claims and return it to payload
		PayloadFunc: func(data any) jwt.MapClaims {
			if userData, ok := data.(authentication.User); ok {
				return jwt.MapClaims{
					static.TokenIdentityKey: userData.ID,
					static.TokenUsernameKey: userData.Username,
					static.TokenJTIKey:      uuid.New().String(),
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

			userRepo := database.NewUserRepository(dbHandle)

			maxLoginAttempts, lockoutDurationMins := lockoutConfig(proviantConfiguration)

			user, err := userRepo.GetUserByUsername(loginVals.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			if locked, _ := userRepo.IsAccountLocked(user.ID, maxLoginAttempts, lockoutDurationMins); locked {
				ctx.Set("failedUserID", user.ID)
				return nil, jwt.ErrFailedAuthentication
			}

			err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginVals.Password))
			if err != nil {
				_ = userRepo.RecordFailedLoginAttempt(user.ID, maxLoginAttempts, lockoutDurationMins)
				ctx.Set("failedUserID", user.ID)
				go recordLoginFailure(ctx, user.ID)
				return nil, jwt.ErrFailedAuthentication
			}

			// Check email verification
			if user.EmailVerifiedAt == nil && !proviantConfiguration.Server.Authentication.SkipEmailVerification {
				return nil, errEmailNotVerified
			}

			_ = userRepo.ResetFailedLoginAttempts(user.ID)
			go recordLoginSuccess(ctx, user.ID, loginVals.Username)
			return user, nil
		},
		// Authorizator checks if user is authorized to emit operation
		Authorizator: authorizatorFunc,
		// Unauthorized implements the return function if user is not authorized
		Unauthorized: unauthorizedFunc,
	})
}

func recordLoginSuccess(ctx *gin.Context, userID uint, username string) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	requestID, _ := ctx.Get(util.ContextKeyRequestID)
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &userID,
		Action:    dbModel.AuditActionLoginSuccess,
		IPAddress: ctx.Request.RemoteAddr,
		RequestID: requestID.(string),
		Details:   `{"username": "` + username + `"}`,
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}

func recordLoginFailure(ctx *gin.Context, userID uint) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	requestID, _ := ctx.Get(util.ContextKeyRequestID)
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &userID,
		Action:    dbModel.AuditActionLoginFailure,
		IPAddress: ctx.Request.RemoteAddr,
		RequestID: requestID.(string),
		Details:   `{"reason": "invalid_credentials"}`,
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}
