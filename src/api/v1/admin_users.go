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

// GetHouseholdUsers returns all users in the household.
// @Summary      List household members
// @Description  Returns all users that belong to the household the caller is admin of
// @Tags         household
// @Produce      json
// @Success      200  {array}   authentication.User
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/users [get]
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

// UpdateHouseholdUser updates a user's username or email.
// @Summary      Update household member
// @Description  Updates the username or email of a user in the household. Caller must be admin.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Param        user body      updateAdminUserRequest  true  "User data"
// @Success      200  {object}  authentication.User
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/users/{id} [patch]
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

	newUsername := targetUser.Username
	if req.Username != "" {
		newUsername = req.Username
	}
	newMailAddress := targetUser.MailAddress
	if req.MailAddress != "" {
		newMailAddress = req.MailAddress
	}

	updateErr := userRepo.UpdateAdminUserFields(targetUser.ID, newUsername, newMailAddress)
	if updateErr != nil {
		logger.Error().Msgf("Error updating user: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}
	targetUser.Username = newUsername
	targetUser.MailAddress = newMailAddress

	ctx.JSON(http.StatusOK, targetUser)
}

// DeleteHouseholdUser deletes a user from the household.
// @Summary      Delete household member
// @Description  Deletes a user from the household. Caller must be admin.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/users/{id} [delete]
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

// AdminResetUserPassword triggers a password reset email for a household member.
// @Summary      Reset user password
// @Description  Sends a password reset email to the specified user. Caller must be admin.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/users/{id}/reset-password [post]
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
