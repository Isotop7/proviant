package v1

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/audit"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

const (
	MsgErrFetchingHousehold  = "Error fetching household: %s"
	MsgErrFetchingTargetUser = "Error fetching target user: %s"
)

// adminPasswordResetEmailTimeout bounds the mail send of a password reset. A
// stalled mail server must not hold the admin's request open.
const adminPasswordResetEmailTimeout = 30 * time.Second

// GetHouseholdUsers returns all users in the household.
// @Summary      List household members
// @Description  Returns all users that belong to the household the caller is admin of
// @Tags         household
// @Produce      json
// @Success      200  {array}   apiModel.HouseholdUserResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/admin/users [get]
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

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdUserResponses(users))
}

// UpdateHouseholdUser updates a user's username or email.
// @Summary      Update household member
// @Description  Updates the username or email of a user in the household. Caller must be admin.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Param        user body      updateAdminUserRequest  true  "User data"
// @Success      200  {object}  apiModel.HouseholdUserResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/admin/users/{id} [patch]
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

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdUserResponse(&targetUser))
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
// @Router       /api/v1/admin/users/{id} [delete]
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

	go recordAccountDeleted(audit.FromGin(ctx), appCtx.UserID, targetUserID, householdID)
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
// @Router       /api/v1/admin/users/{id}/reset-password [post]
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

	if resetErr := sendPasswordResetToMember(ctx, appCtx, &targetUser, householdID); resetErr != nil {
		appCtx.Logger.Error().Msgf("Error sending reset email: %s", resetErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	// The mail is sent in the background, so the answer states what actually
	// happened: the token exists and the member is being sent a mail. It does
	// not claim that the mail arrived.
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password reset prepared, the member is being sent a mail"})
}

// sendPasswordResetToMember creates a password reset token for a member and
// mails the link. It is the part that removes somebody who knew the password
// from the account: without it the holder of the password signs in again and
// re-enrols.
//
// The token belongs in the password reset store, not in the email verification
// store — the verification link only marks the address as verified and never
// changes the password.
func sendPasswordResetToMember(ctx *gin.Context, appCtx *AppContext, targetUser *authentication.User, householdID uint) error {
	token, expiresAt, tokenErr := controllers.GeneratePasswordResetToken()
	if tokenErr != nil {
		return tokenErr
	}

	if err := appCtx.Repos.Users.InvalidatePendingPasswordResetsForUser(targetUser.ID); err != nil {
		appCtx.Logger.Warn().Msgf("Failed to invalidate pending password resets for user %d: %s", targetUser.ID, err.Error())
	}
	ipAddress := ""
	if ctx.Request != nil {
		ipAddress = ctx.ClientIP()
	}
	if err := appCtx.Repos.Users.CreatePasswordReset(targetUser.ID, token, expiresAt, ipAddress); err != nil {
		return err
	}

	notificationValue, _ := ctx.Get(util.ContextKeyNotificationController)
	notificationController, _ := notificationValue.(*controllers.NotificationController)
	if notificationController == nil {
		return nil
	}

	configValue, _ := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, _ := configValue.(*configuration.ProviantConfiguration)
	baseURL := ""
	if proviantConfig != nil {
		baseURL = proviantConfig.Server.BaseURL
	}
	householdLanguage := ""
	if household, householdErr := appCtx.Repos.Users.GetHouseholdByID(householdID); householdErr == nil {
		householdLanguage = household.Language
	}
	lang := i18n.ResolveLanguage(targetUser.Language, householdLanguage, "")

	// Fire-and-forget with a bounded timeout and panic recovery, like the
	// password reset the member requests themselves: a stalled or buggy mail
	// server must not hold the admin's request open.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				appCtx.Logger.Error().Msgf("Panic in the password reset email goroutine: %v\n%s", r, debug.Stack())
			}
		}()

		sendCtx, cancel := context.WithTimeout(context.Background(), adminPasswordResetEmailTimeout)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- notificationController.SendPasswordReset(targetUser.MailAddress, targetUser.Username, token, baseURL, expiresAt, lang)
		}()

		select {
		case sendErr := <-done:
			if sendErr != nil {
				appCtx.Logger.Error().Msgf("Failed to send the password reset email to %s: %s", targetUser.MailAddress, sendErr.Error())
			}
		case <-sendCtx.Done():
			appCtx.Logger.Error().Msgf("The password reset email to %s timed out after %s", targetUser.MailAddress, adminPasswordResetEmailTimeout)
		}
	}()

	return nil
}

// AdminResetUserTwoFactor clears the second factor of a household member.
//
// It is the way back in for a member whose authenticator was lost, and the way
// out for a member whose enrolment was taken over by somebody who knew the
// password: the forced setup hands the QR to whoever presents the password,
// so the account can end up protected by a device its owner never held.
//
// Clearing the factor alone does not evict that somebody — they still know the
// password and simply enrol again. The action therefore also sends a password
// reset mail, which is the part that removes them.
//
// @Summary      Clear the two-factor authentication of a household member
// @Description  Removes the second factor and the recovery codes of a member of the household and sends them a password reset mail. Caller must be admin.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/admin/users/{id}/reset-two-factor [post]
func AdminResetUserTwoFactor(ctx *gin.Context, appCtx *AppContext) {
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

	if !targetUser.TwoFactorEnabled && targetUser.TwoFactorPendingSecret == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrTwoFactorNotEnabled)
		return
	}

	// The password reset comes first: if it fails, the member still has their
	// second factor and the admin can retry. Clearing first would leave the
	// member with neither a second factor nor a way to set a new password, and
	// the holder of the old password would still be in.
	if resetErr := sendPasswordResetToMember(ctx, appCtx, &targetUser, householdID); resetErr != nil {
		appCtx.Logger.Error().Msgf("Error creating the password reset for user %d: %s", targetUser.ID, resetErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	// The repository clears the factor, the recovery codes and a pending
	// enrolment in one transaction. It does not go through the two-factor
	// service: an admin has to be able to clear a member's factor even while
	// the deployment has the feature switched off — that is often the reason
	// they are clearing it.
	if disableErr := appCtx.Repos.Users.DisableTwoFactor(targetUser.ID); disableErr != nil {
		appCtx.Logger.Error().Msgf("Error clearing the two-factor authentication of user %d: %s", targetUser.ID, disableErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	go recordTwoFactorReset(audit.FromGin(ctx), appCtx.UserID, targetUserID, householdID)
	sendTwoFactorRemovedNotification(ctx, appCtx, &targetUser, householdID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Two-factor authentication cleared and a password reset prepared, the member is being sent a mail"})
}

// sendTwoFactorRemovedNotification tells the member that their second factor
// was cleared. The reset happens behind their back, so they have to learn
// about it — the same reason the enrolment sends a mail.
func sendTwoFactorRemovedNotification(ctx *gin.Context, appCtx *AppContext, targetUser *authentication.User, householdID uint) {
	notificationController, ok := ctx.Get(util.ContextKeyNotificationController)
	if !ok {
		return
	}
	nc, ok := notificationController.(*controllers.NotificationController)
	if !ok || nc == nil {
		return
	}

	householdLanguage := ""
	if household, householdErr := appCtx.Repos.Users.GetHouseholdByID(householdID); householdErr == nil {
		householdLanguage = household.Language
	}
	lang := i18n.ResolveLanguage(targetUser.Language, householdLanguage, "")

	go func() {
		if sendErr := nc.SendTwoFactorRemoved(targetUser.MailAddress, targetUser.EffectiveName(), lang); sendErr != nil {
			appCtx.Logger.Warn().Msgf("Failed to send the two-factor notification: %s", sendErr.Error())
		}
	}()
}

type updateAdminUserRequest struct {
	Username    string `json:"username"`
	MailAddress string `json:"mailAddress"`
}

func recordAccountDeleted(values audit.Values, adminID uint, deletedUserID uint, householdID uint) {
	values.Log(adminID, dbModel.AuditActionAccountDeleted, map[string]uint{"deleted_user_id": deletedUserID, "household_id": householdID})
}

// recordTwoFactorReset writes the audit entry of a second factor that an admin
// cleared for a member.
func recordTwoFactorReset(values audit.Values, adminID uint, targetUserID uint, householdID uint) {
	values.Log(adminID, dbModel.AuditActionTwoFactorReset, map[string]uint{"target_user_id": targetUserID, "household_id": householdID})
}
