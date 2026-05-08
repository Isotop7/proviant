package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
func UpdateUser(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var req struct {
		DisplayName string `json:"displayName"`
		MailAddress string `json:"mailAddress" binding:"required,email"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Warn().Msgf(errors.ErrInvalidUserIDWrapperWithMessage, userID, err)
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf(errors.ErrInvalidUserIDWrapper, userID)})
		return
	}

	user.DisplayName = req.DisplayName
	user.MailAddress = req.MailAddress

	updateErr := repos.Users.UpdateUser(user.ID, &user)
	if updateErr != nil {
		logger.Error().Msgf("Error saving user: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
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
func UpdateUserPassword(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	// Get and parse body to user
	var login authentication.Login
	if err := ctx.ShouldBindJSON(&login); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
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

	// Check for valid login credentials
	validationErr := login.IsValidWithValidator(passwordValidator)
	if validationErr != nil {
		logger.Error().Msg(validationErr.Error())
		go recordPasswordChangeFailed(ctx, userID, validationErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	updateErr := repos.Users.UpdateUserPassword(userID, &login)

	switch updateErr {
	// No error => password was updated
	case nil:
		go recordPasswordChangeSuccess(ctx, userID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password updated"})
		return
	// Requested user was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("User with ID '%d' was not found in database", userID)
		go recordPasswordChangeFailed(ctx, userID, "user_not_found")
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: fmt.Sprintf(errors.ErrInvalidUserIDWrapper, userID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error updating password: %s", updateErr)
		go recordPasswordChangeFailed(ctx, userID, updateErr.Error())
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
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
func GetUserNotificationPreferences(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	user, getErr := repos.Users.GetUserByID(userID)
	if getErr != nil {
		logger.Error().Msgf("Error getting user: %s", getErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(getErr))
		return
	}

	prefs := user.NotificationPreferences
	prefs.TelegramLinked = prefs.TelegramChatID != ""
	prefs.TelegramBotConfigured = prefs.TelegramBotToken != ""
	prefs.TelegramBotToken = "" // never expose raw token via API
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
func UpdateUserNotificationPreferences(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var preferences authentication.NotificationPreferences
	if err := ctx.ShouldBindJSON(&preferences); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	// Validate threshold
	if preferences.NotificationThresholdDays < 0 {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrNotificationInvalidThreshold))
		return
	}

	if err := validateNtfyPreferences(&preferences); err != nil {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	user, getErr := repos.Users.GetUserByID(userID)
	if getErr != nil {
		logger.Error().Msgf("Error getting user: %s", getErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(getErr))
		return
	}

	// Validate Telegram: can only enable if already linked
	if preferences.TelegramEnabled && user.NotificationPreferences.TelegramChatID == "" {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "link your Telegram account first before enabling Telegram notifications"})
		return
	}

	newToken, tokenChanged := resolveTelegramTokenUpdate(&user, &preferences)

	user.NotificationPreferences = preferences

	updateErr := repos.Users.UpdateUser(user.ID, &user)
	if updateErr != nil {
		logger.Error().Msgf("Error updating notification preferences: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}

	manageUserTelegramPoller(ctx, userID, tokenChanged, newToken)

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
func GenerateTelegramLinkToken(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		logger.Error().Msgf("Failed to generate telegram link token: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}
	token := hex.EncodeToString(tokenBytes)

	if err := repos.Notifications.SetTelegramLinkToken(userID, token); err != nil {
		logger.Error().Msgf("Failed to save telegram link token: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	botUsername := ""
	if notificationController, ok := getNotificationController(ctx); ok {
		botUsername = notificationController.GetUserTelegramBotUsername(userID)
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
