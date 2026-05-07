package v1

import (
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const (
	MsgErrFetchingHousehold  = "Error fetching household: %s"
	MsgErrFetchingTargetUser = "Error fetching target user: %s"
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching user: %s", err)
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrInvalidUserID))
		return
	}

	household, hhErr := repos.Users.GetHouseholdByID(user.HouseholdID)
	if hhErr != nil {
		logger.Error().Msgf(MsgErrFetchingHousehold, hhErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
	if household.AdminID != userID {
		ctx.JSON(http.StatusForbidden, api.Error(errors.ErrNotHouseholdAdmin))
		return
	}

	users, err := repos.Users.GetUsersByHouseholdID(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Error fetching household users: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	adminID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	targetUserID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
		return
	}

	var req updateAdminUserRequest
	if !bindJSON(ctx, logger, &req) {
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, repos, logger, adminID)
	if !ok {
		return
	}

	targetUser, ok := fetchHouseholdMember(ctx, repos, logger, targetUserID, householdID)
	if !ok {
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

	updateErr := repos.Users.UpdateAdminUserFields(targetUser.ID, newUsername, newMailAddress)
	if updateErr != nil {
		logger.Error().Msgf("Error updating user: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	adminID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	targetUserID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
		return
	}

	if targetUserID == adminID {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrCannotRemoveAdmin))
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, repos, logger, adminID)
	if !ok {
		return
	}

	_, ok = fetchHouseholdMember(ctx, repos, logger, targetUserID, householdID)
	if !ok {
		return
	}

	deleteErr := repos.Users.DeleteUser(targetUserID)
	if deleteErr != nil {
		logger.Error().Msgf("Error deleting user: %s", deleteErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	adminID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	targetUserID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, repos, logger, adminID)
	if !ok {
		return
	}

	targetUser, ok := fetchHouseholdMember(ctx, repos, logger, targetUserID, householdID)
	if !ok {
		return
	}

	token, tokErr := uuid.NewUUID()
	if tokErr != nil {
		logger.Error().Msgf("Error generating reset token: %s", tokErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if createErr := repos.Users.CreateEmailVerification(targetUser.ID, token.String(), expiresAt); createErr != nil {
		logger.Error().Msgf("Error creating reset token: %s", createErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	notificationController, _ := ctx.MustGet(util.ContextKeyNotificationController).(*controllers.NotificationController)
	if notificationController != nil {
		baseURL := ""
		if proviantConfig != nil {
			baseURL = proviantConfig.Server.BaseURL
		}
		if sendErr := notificationController.SendEmailVerification(targetUser.MailAddress, targetUser.Username, token.String(), baseURL, expiresAt); sendErr != nil {
			logger.Error().Msgf("Error sending reset email: %s", sendErr)
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password reset email sent"})
}

type updateAdminUserRequest struct {
	Username    string `json:"username"`
	MailAddress string `json:"mailAddress"`
}
