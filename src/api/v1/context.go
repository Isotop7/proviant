package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func mustGetRepos(ctx *gin.Context, logger *zerolog.Logger) (*database.RepositoryContainer, bool) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !exists || !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return nil, false
	}
	return repos, true
}

func mustGetDB(ctx *gin.Context, logger *zerolog.Logger) (*gorm.DB, bool) {
	db, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
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
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return 0, false
	}
	idFloat, ok := idClaim.(float64)
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return 0, false
	}
	userID := uint(idFloat)
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
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
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Limit '%s' is invalid", limitParam)})
		return 0, false
	}
	if limit < 1 {
		logger.Warn().Msgf("Limit must be at least 1, got %d", limit)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Limit must be at least 1"})
		return 0, false
	}
	if limit > 1000 {
		return 1000, true
	}
	return limit, true
}

func bindJSON(ctx *gin.Context, logger *zerolog.Logger, v any) bool {
	if err := ctx.ShouldBindJSON(v); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return false
	}
	return true
}

func parseUintPathParam(ctx *gin.Context, logger *zerolog.Logger, paramName string) (uint, bool) {
	raw := ctx.Param(paramName)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, raw)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatInvalidRequestId, raw)})
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
		if err == errors.ErrWebhookNotFound || err == errors.ErrWebhookNotOwner {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return 0, false
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return 0, false
	}
	return webhookID, true
}

// authorizeHouseholdAdmin verifies the caller is a household admin.
// Returns the household ID on success. Writes HTTP error and returns false on failure.
func authorizeHouseholdAdmin(ctx *gin.Context, repos *database.RepositoryContainer, logger *zerolog.Logger, adminID uint) (uint, bool) {
	admin, err := repos.Users.GetUserByID(adminID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrInvalidUserID))
		return 0, false
	}
	household, err := repos.Users.GetHouseholdByID(admin.HouseholdID)
	if err != nil {
		logger.Error().Msgf(MsgErrFetchingHousehold, err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return 0, false
	}
	if household.AdminID != adminID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
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
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf(errors.ErrInvalidUserIDWrapper, targetUserID)})
			return authentication.User{}, false
		}
		logger.Error().Msgf(MsgErrFetchingTargetUser, err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return authentication.User{}, false
	}
	if targetUser.HouseholdID != householdID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrUserNotInHousehold))
		return authentication.User{}, false
	}
	return targetUser, true
}
