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

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func mustGetRepos(ctx *gin.Context, logger *zerolog.Logger) (*database.RepositoryContainer, bool) {
	reposVal, exists := ctx.Get("repos")
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !exists || !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return nil, false
	}
	return repos, true
}

func mustGetDB(ctx *gin.Context, logger *zerolog.Logger) (*gorm.DB, bool) {
	db, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
	}
	return db, ok
}

func mustGetUserID(ctx *gin.Context, logger *zerolog.Logger) (uint, bool) {
	// PAT path: PAT middleware injects userID directly into context
	if id, exists := ctx.Get("userID"); exists {
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
	userID := uint(idClaim.(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return 0, false
	}
	return userID, true
}

func getNotificationController(ctx *gin.Context) (*controllers.NotificationController, bool) {
	notificationController, ok := ctx.MustGet("notificationController").(*controllers.NotificationController)
	return notificationController, ok
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
