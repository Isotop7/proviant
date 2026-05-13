package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpdateUser updates a user's display name and email address.
// The login username is never modified by this endpoint.
// @Summary			Updates a user object
// @Description		Updates display name and email of the authenticated user
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Success       	200  {object}  api.APIResponse
// @Failure       	400  {object}  api.APIResponse
// @Failure       	404  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user [patch]
func UpdateUser(ctx *gin.Context, appCtx *AppContext) {
	var req struct {
		DisplayName string `json:"displayName"`
		MailAddress string `json:"mailAddress" binding:"required,email"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusNotFound, fmt.Errorf(apperrors.ErrInvalidUserIDWrapper, appCtx.UserID))
		return
	}

	user.DisplayName = req.DisplayName
	user.MailAddress = req.MailAddress

	updateErr := appCtx.Repos.Users.UpdateUser(user.ID, &user)
	if updateErr != nil {
		appCtx.Logger.Error().Msgf("Error saving user: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "User updated"})
}

// UpdateUserPassword updates a user password
// @Summary			Updates a user password
// @Description		Updates password of a user
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Param         	login   body    authentication.Login  true  "Login"
// @Success       	200  {object}  api.APIResponse
// @Failure       	400  {object}  api.APIResponse
// @Failure       	404  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/password [post]
func UpdateUserPassword(ctx *gin.Context, appCtx *AppContext) {
	var login authentication.Login
	if err := ctx.ShouldBindJSON(&login); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	proviantConfigInterface, pcOk := ctx.Get(util.ContextKeyProviantConfig)
	var passwordValidator *authentication.PasswordValidator
	if pcOk {
		proviantConfig, ok := proviantConfigInterface.(*configuration.ProviantConfiguration)
		if ok {
			passwordValidator = authentication.PasswordValidatorFromConfig(authentication.PasswordConfig{
				MinLength:        proviantConfig.Server.Authentication.PasswordMinLength,
				RequireUppercase: proviantConfig.Server.Authentication.PasswordRequireUppercase,
				RequireDigit:     proviantConfig.Server.Authentication.PasswordRequireDigit,
				RequireSpecial:   proviantConfig.Server.Authentication.PasswordRequireSpecial,
				CheckBreached:    proviantConfig.Server.Authentication.PasswordCheckBreached,
			})
		}
	}
	if passwordValidator == nil {
		passwordValidator = authentication.DefaultPasswordValidator()
	}

	validationErr := login.IsValidWithValidator(passwordValidator)
	if validationErr != nil {
		appCtx.Logger.Error().Msg(validationErr.Error())
		go recordPasswordChangeFailed(ctx, appCtx.UserID, validationErr.Error())
		api.RespondError(ctx, http.StatusBadRequest, validationErr)
		return
	}

	updateErr := appCtx.Repos.Users.UpdateUserPassword(appCtx.UserID, &login)

	switch updateErr {
	case nil:
		go recordPasswordChangeSuccess(ctx, appCtx.UserID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password updated"})
		return
	case gorm.ErrRecordNotFound:
		appCtx.Logger.Error().Msgf("User with ID '%d' was not found in database", appCtx.UserID)
		go recordPasswordChangeFailed(ctx, appCtx.UserID, "user_not_found")
		api.RespondError(ctx, http.StatusNotFound, fmt.Errorf(apperrors.ErrInvalidUserIDWrapper, appCtx.UserID))
		return
	default:
		appCtx.Logger.Error().Msgf("Error updating password: %s", updateErr)
		go recordPasswordChangeFailed(ctx, appCtx.UserID, updateErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}
}

// GetUserNotificationPreferences gets a user's notification preferences
// @Summary			Gets a user's notification preferences
// @Description		Retrieves notification preferences for the current user
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Success       	200  {object}  authentication.NotificationPreferences
// @Failure       	400  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/notification-preferences [get]
func GetUserNotificationPreferences(ctx *gin.Context, appCtx *AppContext) {
	user, getErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", getErr)
		api.RespondError(ctx, http.StatusInternalServerError, getErr)
		return
	}

	prefs := user.NotificationPreferences
	prefs.TelegramLinked = prefs.TelegramChatID != ""
	prefs.TelegramBotConfigured = prefs.TelegramBotToken != ""
	prefs.TelegramBotToken = ""
	ctx.JSON(http.StatusOK, prefs)
}

// UpdateUserNotificationPreferences updates a user's notification preferences
// @Summary			Updates a user's notification preferences
// @Description		Updates notification preferences for the current user
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Success       	200  {object}  api.APIResponse
// @Failure       	400  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/notification-preferences [post]
func UpdateUserNotificationPreferences(ctx *gin.Context, appCtx *AppContext) {
	var preferences authentication.NotificationPreferences
	if err := ctx.ShouldBindJSON(&preferences); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	if preferences.NotificationThresholdDays < 0 {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNotificationInvalidThreshold)
		return
	}

	if err := validateNtfyPreferences(&preferences); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, getErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", getErr)
		api.RespondError(ctx, http.StatusInternalServerError, getErr)
		return
	}

	if preferences.TelegramEnabled && user.NotificationPreferences.TelegramChatID == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("link your Telegram account first before enabling Telegram notifications"))
		return
	}

	newToken, tokenChanged := resolveTelegramTokenUpdate(&user, &preferences)

	user.NotificationPreferences = preferences

	updateErr := appCtx.Repos.Users.UpdateUser(user.ID, &user)
	if updateErr != nil {
		appCtx.Logger.Error().Msgf("Error updating notification preferences: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}

	manageUserTelegramPoller(ctx, appCtx.UserID, tokenChanged, newToken)

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Notification preferences updated successfully"})
}

// GenerateTelegramLinkToken generates a one-time token for linking a Telegram chat to the user account.
// @Summary			Generate Telegram link token
// @Description		Generates a short-lived token the user sends to the Proviant Telegram bot to link their account
// @Tags          	user
// @Produce       	json
// @Success       	200  {object}  map[string]string
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/telegram-link-token [post]
func GenerateTelegramLinkToken(ctx *gin.Context, appCtx *AppContext) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		appCtx.Logger.Error().Msgf("Failed to generate telegram link token: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	if err := appCtx.Repos.Notifications.SetTelegramLinkToken(appCtx.UserID, token); err != nil {
		appCtx.Logger.Error().Msgf("Failed to save telegram link token: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	botUsername := ""
	if notificationController, ok := getNotificationController(ctx); ok {
		botUsername = notificationController.GetUserTelegramBotUsername(appCtx.UserID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"token":       token,
		"botUsername": botUsername,
	})
}

func validateNtfyPreferences(prefs *authentication.NotificationPreferences) error {
	if !prefs.NtfyEnabled {
		return nil
	}
	if prefs.NtfyTopic == "" {
		return fmt.Errorf("ntfy topic is required when ntfy is enabled")
	}
	if prefs.NtfyURL == "" {
		return fmt.Errorf("ntfy URL is required when ntfy is enabled")
	}
	return nil
}

// resolveTelegramTokenUpdate reconciles the incoming bot token with the stored one,
// mutates prefs in place, and returns the resolved token and whether it changed.
func resolveTelegramTokenUpdate(user *authentication.User, prefs *authentication.NotificationPreferences) (newToken string, tokenChanged bool) {
	oldToken := user.NotificationPreferences.TelegramBotToken
	newToken = prefs.TelegramBotToken
	if newToken == "" {
		newToken = oldToken
	}
	tokenChanged = newToken != oldToken
	if tokenChanged && newToken == "" {
		// Token cleared — disable Telegram and wipe linked state.
		prefs.TelegramEnabled = false
		prefs.TelegramChatID = ""
		prefs.TelegramBotUsername = ""
	} else {
		// Preserve linking fields — managed by the dedicated link-token endpoint.
		prefs.TelegramChatID = user.NotificationPreferences.TelegramChatID
		prefs.TelegramLinkToken = user.NotificationPreferences.TelegramLinkToken
		if !tokenChanged {
			prefs.TelegramBotUsername = user.NotificationPreferences.TelegramBotUsername
		}
	}
	prefs.TelegramBotToken = newToken
	return
}

func manageUserTelegramPoller(ctx *gin.Context, userID uint, tokenChanged bool, newToken string) {
	if !tokenChanged {
		return
	}
	notificationController, ok := getNotificationController(ctx)
	if !ok {
		return
	}
	if newToken == "" {
		notificationController.StopUserTelegramPoller(userID)
	} else {
		notificationController.StartUserTelegramPoller(userID, newToken)
	}
}

func recordPasswordChangeSuccess(ctx *gin.Context, userID uint) {
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
		UserID:    &userID,
		Action:    dbModel.AuditActionPasswordChange,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}

func recordPasswordChangeFailed(ctx *gin.Context, userID uint, reason string) {
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
		UserID:    &userID,
		Action:    dbModel.AuditActionPasswordChangeFailed,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
		Details:   `{"reason": "` + reason + `"}`,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}
