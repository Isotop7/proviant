package v1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"

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
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user [patch]
func UpdateUser(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
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
	if bindErr := ctx.ShouldBindJSON(&req); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(bindErr))
		return
	}

	userRepo := database.NewUserRepository(dbHandle)

	user, fetchErr := userRepo.GetUserByID(userID)
	if fetchErr != nil {
		logger.Error().Msgf("User with ID '%d' not found: %s", userID, fetchErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("User with id '%d' was not found", userID)})
		return
	}

	user.DisplayName = req.DisplayName
	user.MailAddress = req.MailAddress

	updateErr := userRepo.UpdateUser(user.ID, &user)
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
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/password [post]
func UpdateUserPassword(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	// Get and parse body to user
	var login authentication.Login
	if bindErr := ctx.ShouldBindJSON(&login); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(bindErr))
		return
	}

	proviantConfigInterface, pcOk := ctx.Get("proviantConfig")
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
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	userRepo := database.NewUserRepository(dbHandle)

	updateErr := userRepo.UpdateUserPassword(userID, &login)

	switch updateErr {
	// No error => password was updated
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password updated"})
		return
	// Requested user was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("User with ID '%d' was not found in database", userID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("User with id '%d' was not found", userID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error updating password: %s", updateErr)
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	userRepo := database.NewUserRepository(dbHandle)

	user, getErr := userRepo.GetUserByID(userID)
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var preferences authentication.NotificationPreferences
	if bindErr := ctx.ShouldBindJSON(&preferences); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(bindErr))
		return
	}

	// Validate threshold
	if preferences.NotificationThresholdDays < 0 {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrNotificationInvalidThreshold))
		return
	}

	// Validate ntfy configuration if enabled
	if preferences.NtfyEnabled {
		if preferences.NtfyTopic == "" {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "ntfy topic is required when ntfy is enabled"})
			return
		}
		if preferences.NtfyURL == "" {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "ntfy URL is required when ntfy is enabled"})
			return
		}
	}

	// Create database controller
	userRepo := database.NewUserRepository(dbHandle)

	user, getErr := userRepo.GetUserByID(userID)
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

	// Handle bot token lifecycle: start/stop poller when token changes.
	oldToken := user.NotificationPreferences.TelegramBotToken
	newToken := preferences.TelegramBotToken
	if newToken == "" {
		// Preserve existing token if not provided (allows partial updates)
		newToken = oldToken
	}

	tokenChanged := newToken != oldToken

	if tokenChanged && newToken == "" {
		// Token cleared — disable Telegram and wipe linked state.
		preferences.TelegramEnabled = false
		preferences.TelegramChatID = ""
		preferences.TelegramBotUsername = ""
	} else {
		// Preserve linking fields — managed by the dedicated link-token endpoint.
		preferences.TelegramChatID = user.NotificationPreferences.TelegramChatID
		preferences.TelegramLinkToken = user.NotificationPreferences.TelegramLinkToken
		if !tokenChanged {
			preferences.TelegramBotUsername = user.NotificationPreferences.TelegramBotUsername
		}
	}
	preferences.TelegramBotToken = newToken

	user.NotificationPreferences = preferences

	updateErr := userRepo.UpdateUser(user.ID, &user)
	if updateErr != nil {
		logger.Error().Msgf("Error updating notification preferences: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}

	// Manage per-user Telegram poller when token changed.
	if tokenChanged {
		if nc, ok := getNotificationController(ctx); ok {
			if newToken == "" {
				nc.StopUserTelegramPoller(userID)
			} else {
				nc.StartUserTelegramPoller(userID, newToken)
			}
		}
	}

	// Return success
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
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

	notificationRepo := database.NewNotificationRepository(dbHandle)
	if err := notificationRepo.SetTelegramLinkToken(userID, token); err != nil {
		logger.Error().Msgf("Failed to save telegram link token: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	botUsername := ""
	if nc, ok := getNotificationController(ctx); ok {
		botUsername = nc.GetUserTelegramBotUsername(userID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"token":       token,
		"botUsername": botUsername,
	})
}
