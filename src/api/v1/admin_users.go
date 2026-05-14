package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
func GetHouseholdUsers(ctx *gin.Context, appCtx *AppContext) {
	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error fetching user: %s", err)
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidUserID)
		return
	}

	users, err := appCtx.Repos.Users.GetUsersByHouseholdID(user.HouseholdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error fetching household users: %s", err)
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
func UpdateHouseholdUser(ctx *gin.Context, appCtx *AppContext) {
	targetUserID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req updateAdminUserRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	targetUser, ok := fetchHouseholdMember(ctx, appCtx.Repos, appCtx.Logger, targetUserID, householdID)
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

	updateErr := appCtx.Repos.Users.UpdateAdminUserFields(targetUser.ID, newUsername, newMailAddress)
	if updateErr != nil {
		appCtx.Logger.Error().Msgf("Error updating user: %s", updateErr)
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
func DeleteHouseholdUser(ctx *gin.Context, appCtx *AppContext) {
	targetUserID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	if targetUserID == appCtx.UserID {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrCannotRemoveAdmin)
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	_, ok = fetchHouseholdMember(ctx, appCtx.Repos, appCtx.Logger, targetUserID, householdID)
	if !ok {
		return
	}

	deleteErr := appCtx.Repos.Users.DeleteUser(targetUserID)
	if deleteErr != nil {
		appCtx.Logger.Error().Msgf("Error deleting user: %s", deleteErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	go recordAccountDeleted(ctx, appCtx.UserID, targetUserID, householdID)
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
func AdminResetUserPassword(ctx *gin.Context, appCtx *AppContext) {
	targetUserID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, ok := authorizeHouseholdAdmin(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	targetUser, ok := fetchHouseholdMember(ctx, appCtx.Repos, appCtx.Logger, targetUserID, householdID)
	if !ok {
		return
	}

	token, tokErr := uuid.NewUUID()
	if tokErr != nil {
		appCtx.Logger.Error().Msgf("Error generating reset token: %s", tokErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if createErr := appCtx.Repos.Users.CreateEmailVerification(targetUser.ID, token.String(), expiresAt); createErr != nil {
		appCtx.Logger.Error().Msgf("Error creating reset token: %s", createErr)
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
			appCtx.Logger.Error().Msgf("Error sending reset email: %s", sendErr)
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

func recordAccountDeleted(ctx *gin.Context, adminID uint, deletedUserID uint, householdID uint) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	ipAddress := ""
	if ctx.Request != nil {
		ipAddress = ctx.Request.RemoteAddr
	}
	var requestIDStr string
	if requestID, ok := ctx.Get(util.ContextKeyRequestID); ok {
		requestIDStr, _ = requestID.(string)
	}
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &adminID,
		Action:    dbModel.AuditActionAccountDeleted,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
		Details:   `{"deleted_user_id": ` + strconv.FormatUint(uint64(deletedUserID), 10) + `, "household_id": ` + strconv.FormatUint(uint64(householdID), 10) + `}`,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}
