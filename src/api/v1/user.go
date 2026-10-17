package v1

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/audit"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

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
		DisplayName string  `json:"displayName" binding:"max=100"`
		MailAddress string  `json:"mailAddress" binding:"required,email"`
		Language    *string `json:"language"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}
	if req.Language != nil && *req.Language != "" && !i18n.Supported(*req.Language) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUnsupportedLanguage)
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
	if req.Language != nil {
		user.Language = *req.Language
	}

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
		go recordPasswordChangeFailed(audit.FromGin(ctx), appCtx.UserID, validationErr.Error())
		api.RespondError(ctx, http.StatusBadRequest, validationErr)
		return
	}

	updateErr := appCtx.Repos.Users.UpdateUserPassword(appCtx.UserID, &login)

	switch updateErr {
	case nil:
		go recordPasswordChangeSuccess(audit.FromGin(ctx), appCtx.UserID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password updated"})
		return
	case gorm.ErrRecordNotFound:
		appCtx.Logger.Error().Msgf("User with ID '%d' was not found in database", appCtx.UserID)
		go recordPasswordChangeFailed(audit.FromGin(ctx), appCtx.UserID, "user_not_found")
		api.RespondError(ctx, http.StatusNotFound, fmt.Errorf(apperrors.ErrInvalidUserIDWrapper, appCtx.UserID))
		return
	default:
		appCtx.Logger.Error().Msgf("Error updating password: %s", updateErr)
		go recordPasswordChangeFailed(audit.FromGin(ctx), appCtx.UserID, updateErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}
}

// GetUserNotificationPreferences gets a user's notification preferences
// @Summary			Gets a user's notification preferences
// @Description		Retrieves notification preferences for the current user; stored secrets are reported as configured flags only
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Success       	200  {object}  apiModel.NotificationPreferencesResponse
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

	ctx.JSON(http.StatusOK, apiModel.NewNotificationPreferencesResponse(&user.NotificationPreferences))
}

// UpdateUserNotificationPreferences updates a user's notification preferences
// @Summary			Updates a user's notification preferences
// @Description		Updates notification preferences for the current user; empty or omitted secret fields keep their stored value and cannot be cleared through this endpoint, while every other field replaces its stored value
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Param			preferences body apiModel.UpdateNotificationPreferencesRequest true "Notification preferences"
// @Success       	200  {object}  api.APIResponse
// @Failure       	400  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/notification-preferences [post]
func UpdateUserNotificationPreferences(ctx *gin.Context, appCtx *AppContext) {
	var request apiModel.UpdateNotificationPreferencesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	preferences := request.ToNotificationPreferences()
	if preferences.NotificationThresholdDays < 0 {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNotificationInvalidThreshold)
		return
	}

	if err := validateNtfyPreferences(&preferences); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	if err := validateMailDigestFrequency(preferences.MailDigestFrequency); err != nil {
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
	resolveNtfyTokenUpdate(&user, &preferences)
	// Written only by the webpush endpoints, never by this request body.
	preferences.WebPushSubscriptionJSON = user.NotificationPreferences.WebPushSubscriptionJSON

	oldMailDigestFrequency := user.NotificationPreferences.MailDigestFrequency
	user.NotificationPreferences = preferences

	updateErr := appCtx.Repos.Users.UpdateUser(user.ID, &user)
	if updateErr != nil {
		appCtx.Logger.Error().Msgf("Error updating notification preferences: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}

	manageUserTelegramPoller(ctx, appCtx.UserID, tokenChanged, newToken)

	if oldMailDigestFrequency == authentication.MailDigestFrequencyDisabled && preferences.MailDigestFrequency != authentication.MailDigestFrequencyDisabled {
		if _, genErr := appCtx.Repos.Notifications.GenerateMailDigestUnsubscribeToken(user.ID); genErr != nil {
			appCtx.Logger.Warn().Msgf("Failed to generate unsubscribe token for user %d: %s", user.ID, genErr)
		}
	}

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

func validateMailDigestFrequency(freq string) error {
	if freq == "" {
		return nil
	}
	switch freq {
	case authentication.MailDigestFrequencyDisabled, authentication.MailDigestFrequencyDaily, authentication.MailDigestFrequencyWeekly:
		return nil
	default:
		return fmt.Errorf("invalid mail digest frequency: must be 'disabled', 'daily', or 'weekly'")
	}
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
	// Preserve linking fields — managed by the dedicated link-token endpoint.
	prefs.TelegramChatID = user.NotificationPreferences.TelegramChatID
	prefs.TelegramLinkToken = user.NotificationPreferences.TelegramLinkToken
	if !tokenChanged {
		prefs.TelegramBotUsername = user.NotificationPreferences.TelegramBotUsername
	}
	prefs.TelegramBotToken = newToken
	return
}

// resolveNtfyTokenUpdate keeps the stored token when the request carries none,
// so a save cannot wipe a credential the owner did not retype.
func resolveNtfyTokenUpdate(user *authentication.User, prefs *authentication.NotificationPreferences) {
	if prefs.NtfyToken == "" {
		prefs.NtfyToken = user.NotificationPreferences.NtfyToken
	}
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

func recordPasswordChangeSuccess(values audit.Values, userID uint) {
	values.Log(userID, dbModel.AuditActionPasswordChange, struct{}{})
}

func recordPasswordChangeFailed(values audit.Values, userID uint, reason string) {
	values.Log(userID, dbModel.AuditActionPasswordChangeFailed, map[string]string{"reason": reason})
}
