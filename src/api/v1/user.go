package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
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

	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
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

	var req struct {
		DisplayName string `json:"displayName"`
		MailAddress string `json:"mailAddress" binding:"required"`
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

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
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

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	userRepo := database.NewUserRepository(dbHandle)

	user, getErr := userRepo.GetUserByID(userID)
	if getErr != nil {
		logger.Error().Msgf("Error getting user: %s", getErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(getErr))
		return
	}

	ctx.JSON(http.StatusOK, user.NotificationPreferences)
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

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	// Get and parse body to notification preferences
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

	user.NotificationPreferences = preferences

	updateErr := userRepo.UpdateUser(user.ID, &user)
	if updateErr != nil {
		logger.Error().Msgf("Error updating notification preferences: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}

	// Return success
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Notification preferences updated successfully"})
}
