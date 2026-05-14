package v1

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type AppContext struct {
	Logger   *zerolog.Logger
	DB       *gorm.DB
	Repos    *database.RepositoryContainer
	UserID   uint
	Products *services.ProductService
}

func mustGetAppContext(ctx *gin.Context, logger *zerolog.Logger) *AppContext {
	db, ok := mustGetDB(ctx, logger)
	if !ok {
		return nil
	}
	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return nil
	}
	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return nil
	}
	return &AppContext{
		Logger:   logger,
		DB:       db,
		Repos:    repos,
		UserID:   userID,
		Products: services.NewProductService(repos, logger),
	}
}

type APIHandler func(*gin.Context, *AppContext)

func AppContextMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		loggerVal, loggerOk := ctx.Get(util.ContextKeyLogger)
		logger, _ := loggerVal.(*zerolog.Logger)
		if !loggerOk || logger == nil {
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		appCtx := mustGetAppContext(ctx, logger)
		if appCtx == nil {
			ctx.Abort()
			return
		}
		ctx.Set("appContext", appCtx)
		ctx.Next()
	}
}

func WrapHandler(fn APIHandler) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		appCtxVal, exists := ctx.Get("appContext")
		if !exists {
			loggerVal, _ := ctx.Get(util.ContextKeyLogger)
			logger, _ := loggerVal.(*zerolog.Logger)
			if logger != nil {
				logger.Error().Msg("appContext not found in context")
			}
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		appCtx, ok := appCtxVal.(*AppContext)
		if !ok {
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		fn(ctx, appCtx)
	}
}

func SetupTestAppContext(ctx *gin.Context, userID uint) *AppContext {
	loggerVal, _ := ctx.Get(util.ContextKeyLogger)
	logger, _ := loggerVal.(*zerolog.Logger)
	reposVal, _ := ctx.Get(util.ContextKeyRepos)
	repos, _ := reposVal.(*database.RepositoryContainer)
	dbVal, _ := ctx.Get(util.ContextKeyDBHandle)
	db, _ := dbVal.(*gorm.DB)

	appCtx := &AppContext{
		Logger: logger,
		DB:     db,
		Repos:  repos,
		UserID: userID,
	}
	ctx.Set("appContext", appCtx)
	return appCtx
}

func mustGetRepos(ctx *gin.Context, logger *zerolog.Logger) (*database.RepositoryContainer, bool) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !exists || !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrDatabaseContextNotFound)
		return nil, false
	}
	return repos, true
}

func mustGetDB(ctx *gin.Context, logger *zerolog.Logger) (*gorm.DB, bool) {
	db, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrDatabaseContextNotFound)
	}
	return db, ok
}

func mustGetUserID(ctx *gin.Context, logger *zerolog.Logger) (uint, bool) {
	// PAT path: PAT middleware injects userID directly into context
	if id, exists := ctx.Get(util.ContextKeyUserID); exists {
		if userID, ok := id.(uint); ok && userID > 0 {
			return userID, true
		}
	}

	// JWT path: extract from token claims
	claims := jwt.ExtractClaims(ctx)
	idClaim, ok := claims[static.TokenIdentityKey]
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUserIDFromToken)
		return 0, false
	}
	idFloat, ok := idClaim.(float64)
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUserIDFromToken)
		return 0, false
	}
	userID := uint(idFloat)
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUserIDFromToken)
		return 0, false
	}
	return userID, true
}

func getNotificationController(ctx *gin.Context) (*controllers.NotificationController, bool) {
	notificationController, ok := ctx.MustGet(util.ContextKeyNotificationController).(*controllers.NotificationController)
	return notificationController, ok
}

func parseLimitParam(ctx *gin.Context, logger *zerolog.Logger) (int, bool) {
	limitParam := ctx.Query("limit")
	if limitParam == "" {
		return 100, true
	}
	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		logger.Warn().Msgf("Invalid limit '%s' was specified", limitParam)
		api.RespondError(ctx, http.StatusBadRequest, fmt.Errorf("limit '%s' is invalid", limitParam))
		return 0, false
	}
	if limit < 1 {
		logger.Warn().Msgf("Limit must be at least 1, got %d", limit)
		api.RespondError(ctx, http.StatusBadRequest, errors.New("limit must be at least 1"))
		return 0, false
	}
	if limit > 1000 {
		return 1000, true
	}
	return limit, true
}

func bindJSON(ctx *gin.Context, logger *zerolog.Logger, v any) bool {
	if err := ctx.ShouldBindJSON(v); err != nil {
		logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return false
	}
	return true
}

func parseUintPathParam(ctx *gin.Context, logger *zerolog.Logger, paramName string) (uint, bool) {
	raw := ctx.Param(paramName)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		logger.Warn().Msgf(apperrors.FormatInvalidRequestId, raw)
		api.RespondError(ctx, http.StatusBadRequest, fmt.Errorf(apperrors.FormatInvalidRequestId, raw))
		return 0, false
	}
	return uint(id), true //nolint:gosec
}

// mustGetOwnedWebhookID parses the "id" path param, verifies ownership, and returns the webhook ID.
func mustGetOwnedWebhookID(ctx *gin.Context, repos *database.RepositoryContainer, logger *zerolog.Logger, userID uint) (uint, bool) {
	webhookID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
		return 0, false
	}
	if err := repos.Webhooks.CheckOwnership(webhookID, userID); err != nil {
		if err == apperrors.ErrWebhookNotFound || err == apperrors.ErrWebhookNotOwner {
			api.RespondError(ctx, http.StatusNotFound, err)
			return 0, false
		}
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return 0, false
	}
	return webhookID, true
}

// authorizeHouseholdAdmin verifies the caller is a household admin.
// Returns the household ID on success. Writes HTTP error and returns false on failure.
func authorizeHouseholdAdmin(ctx *gin.Context, repos *database.RepositoryContainer, logger *zerolog.Logger, adminID uint) (uint, bool) {
	admin, err := repos.Users.GetUserByID(adminID)
	if err != nil {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
		return 0, false
	}
	household, err := repos.Users.GetHouseholdByID(admin.HouseholdID)
	if err != nil {
		logger.Error().Msgf(MsgErrFetchingHousehold, err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return 0, false
	}
	if household.AdminID != adminID {
		api.RespondError(ctx, http.StatusForbidden, apperrors.ErrNotHouseholdAdmin)
		return 0, false
	}
	return household.ID, true
}

// fetchHouseholdMember retrieves a user and verifies they belong to the given household.
// Returns the user on success. Writes HTTP error and returns false on failure.
func fetchHouseholdMember(ctx *gin.Context, repos *database.RepositoryContainer, logger *zerolog.Logger, targetUserID uint, householdID uint) (authentication.User, bool) {
	targetUser, err := repos.Users.GetUserByID(targetUserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api.RespondError(ctx, http.StatusNotFound, fmt.Errorf(apperrors.ErrInvalidUserIDWrapper, targetUserID))
			return authentication.User{}, false
		}
		logger.Error().Msgf(MsgErrFetchingTargetUser, err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return authentication.User{}, false
	}
	if targetUser.HouseholdID != householdID {
		api.RespondError(ctx, http.StatusForbidden, apperrors.ErrUserNotInHousehold)
		return authentication.User{}, false
	}
	return targetUser, true
}
