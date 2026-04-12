package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func GetHouseholdUsers(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	userRepo := database.NewUserRepository(dbHandle)
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching user: %s", err)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserID.Error()})
		return
	}

	household, hhErr := userRepo.GetHouseholdByID(user.HouseholdID)
	if hhErr != nil {
		logger.Error().Msgf("Error fetching household: %s", hhErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(hhErr))
		return
	}
	if household.AdminID != userID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
		return
	}

	users, err := userRepo.GetUsersByHouseholdID(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Error fetching household users: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	ctx.JSON(http.StatusOK, users)
}

func UpdateHouseholdUser(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	adminID := uint(claims[static.TokenIdentityKey].(float64))
	if adminID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	idParam := ctx.Param("id")
	targetUserID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid user id"})
		return
	}

	var req updateAdminUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	userRepo := database.NewUserRepository(dbHandle)
	admin, adminErr := userRepo.GetUserByID(adminID)
	if adminErr != nil {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserID.Error()})
		return
	}

	household, hhErr := userRepo.GetHouseholdByID(admin.HouseholdID)
	if hhErr != nil {
		logger.Error().Msgf("Error fetching household: %s", hhErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(hhErr))
		return
	}
	if household.AdminID != adminID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
		return
	}

	targetUser, targetErr := userRepo.GetUserByID(uint(targetUserID))
	if targetErr != nil {
		if targetErr == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf("User with id '%d' not found", targetUserID)})
			return
		}
		logger.Error().Msgf("Error fetching target user: %s", targetErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(targetErr))
		return
	}
	if targetUser.HouseholdID != admin.HouseholdID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrUserNotInHousehold))
		return
	}

	if req.Username != "" {
		targetUser.Username = req.Username
	}
	if req.MailAddress != "" {
		targetUser.MailAddress = req.MailAddress
	}

	if err := targetUser.IsValid(true); err != nil {
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	updateErr := userRepo.UpdateUser(targetUser.ID, &targetUser)
	if updateErr != nil {
		logger.Error().Msgf("Error updating user: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}

	ctx.JSON(http.StatusOK, targetUser)
}

func DeleteHouseholdUser(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	adminID := uint(claims[static.TokenIdentityKey].(float64))
	if adminID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	idParam := ctx.Param("id")
	targetUserID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid user id"})
		return
	}

	userRepo := database.NewUserRepository(dbHandle)
	admin, adminErr := userRepo.GetUserByID(adminID)
	if adminErr != nil {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserID.Error()})
		return
	}

	household, hhErr := userRepo.GetHouseholdByID(admin.HouseholdID)
	if hhErr != nil {
		logger.Error().Msgf("Error fetching household: %s", hhErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(hhErr))
		return
	}
	if household.AdminID != adminID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
		return
	}

	if uint(targetUserID) == adminID {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrCannotRemoveAdmin))
		return
	}

	targetUser, targetErr := userRepo.GetUserByID(uint(targetUserID))
	if targetErr != nil {
		if targetErr == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf("User with id '%d' not found", targetUserID)})
			return
		}
		logger.Error().Msgf("Error fetching target user: %s", targetErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(targetErr))
		return
	}
	if targetUser.HouseholdID != admin.HouseholdID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrUserNotInHousehold))
		return
	}

	deleteErr := userRepo.DeleteUser(uint(targetUserID))
	if deleteErr != nil {
		logger.Error().Msgf("Error deleting user: %s", deleteErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(deleteErr))
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "User deleted"})
}

func AdminResetUserPassword(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	adminID := uint(claims[static.TokenIdentityKey].(float64))
	if adminID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	idParam := ctx.Param("id")
	targetUserID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid user id"})
		return
	}

	userRepo := database.NewUserRepository(dbHandle)
	admin, adminErr := userRepo.GetUserByID(adminID)
	if adminErr != nil {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserID.Error()})
		return
	}

	household, hhErr := userRepo.GetHouseholdByID(admin.HouseholdID)
	if hhErr != nil {
		logger.Error().Msgf("Error fetching household: %s", hhErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(hhErr))
		return
	}
	if household.AdminID != adminID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
		return
	}

	targetUser, targetErr := userRepo.GetUserByID(uint(targetUserID))
	if targetErr != nil {
		if targetErr == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf("User with id '%d' not found", targetUserID)})
			return
		}
		logger.Error().Msgf("Error fetching target user: %s", targetErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(targetErr))
		return
	}
	if targetUser.HouseholdID != admin.HouseholdID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrUserNotInHousehold))
		return
	}

	token, tokErr := uuid.NewUUID()
	if tokErr != nil {
		logger.Error().Msgf("Error generating reset token: %s", tokErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "error generating reset token"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if createErr := userRepo.CreateEmailVerification(targetUser.ID, token.String(), expiresAt); createErr != nil {
		logger.Error().Msgf("Error creating reset token: %s", createErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(createErr))
		return
	}

	proviantConfig, _ := ctx.MustGet("proviantConfig").(*configuration.ProviantConfiguration)
	notificationController, _ := ctx.MustGet("notificationController").(*controllers.NotificationController)
	if notificationController != nil {
		baseURL := ""
		if proviantConfig != nil {
			baseURL = proviantConfig.Server.BaseURL
		}
		if sendErr := notificationController.SendEmailVerification(targetUser.MailAddress, targetUser.Username, token.String(), baseURL, expiresAt); sendErr != nil {
			logger.Error().Msgf("Error sending reset email: %s", sendErr)
			ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "error sending reset email"})
			return
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password reset email sent"})
}

type updateAdminUserRequest struct {
	Username    string `json:"username"`
	MailAddress string `json:"mailAddress"`
}
