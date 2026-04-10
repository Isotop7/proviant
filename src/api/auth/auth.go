// auth contains authentication method handlers
package auth

import (
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Signup creates a new user object in the database
// @Summary      	Creates a new user
// @Description  	Creates a new new user in the database
// @Tags         	user
// @Accept			json
// @Produce      	json
// @Param			signup	body	authentication.Signup	true	"Signup"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/auth/signup [post]
func Signup(ctx *gin.Context) {
	// Get logger instance from context
	loggerValue, loggerOk := ctx.Get("logger")
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}
	logger := loggerValue.(*zerolog.Logger)

	// Get database instance from context
	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	userRepo := database.NewUserRepository(dbHandle.(*gorm.DB))
	invitationRepo := database.NewInvitationRepository(dbHandle.(*gorm.DB))

	var signup authentication.Signup
	if err := ctx.ShouldBindJSON(&signup); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	var proviantConfig *configuration.ProviantConfiguration
	proviantConfigInterface, pcOk := ctx.Get("proviantConfig")
	if pcOk {
		var ok bool
		proviantConfig, ok = proviantConfigInterface.(*configuration.ProviantConfiguration)
		if !ok {
			proviantConfig = nil
		}
	}

	var passwordValidator *authentication.PasswordValidator
	if proviantConfig != nil {
		passwordValidator = authentication.PasswordValidatorFromConfig(authentication.PasswordConfig{
			MinLength:        proviantConfig.Server.Authentication.PasswordMinLength,
			RequireUppercase: proviantConfig.Server.Authentication.PasswordRequireUppercase,
			RequireDigit:     proviantConfig.Server.Authentication.PasswordRequireDigit,
			RequireSpecial:   proviantConfig.Server.Authentication.PasswordRequireSpecial,
			CheckBreached:    proviantConfig.Server.Authentication.PasswordCheckBreached,
		})
	}
	if passwordValidator == nil {
		passwordValidator = authentication.DefaultPasswordValidator()
	}

	validationErr := signup.IsValidWithValidator(passwordValidator)
	if validationErr != nil {
		logger.Error().Msgf("User data was invalid: '%s'", validationErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	// Create new user object
	user := authentication.User{
		ID:          userRepo.GetNextUserID(),
		Username:    signup.Username,
		Password:    signup.Password,
		MailAddress: signup.MailAddress,
	}

	// Check if user with username already exists
	if userRepo.UserExistsByUsername(&user) {
		logger.Error().Msgf("User '%s' already exists", user.Username)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithUsernameExists)
		return
	}

	// Check if user with mail address already exists
	if userRepo.UserExistsByMailAddress(&user) {
		logger.Error().Msgf("User with mail address '%s' already exists", user.MailAddress)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithMailAddressExists)
		return
	}

	// Create user object in database
	createError := userRepo.CreateUser(&user)
	if createError != nil {
		logger.Error().Msgf("User '%s' with ID '%d' could not be created. Error: %s", user.Username, user.ID, createError.Error())
		ctx.JSON(http.StatusBadRequest, api.ResponseErrInvalidUserData)
		return
	}

	logger.Info().Msgf("New User '%s' with ID '%d' created", user.Username, user.ID)

	if signup.InviteToken != "" {
		acceptErr := invitationRepo.AcceptInvitation(signup.InviteToken, user.MailAddress, user.ID)
		if acceptErr != nil {
			logger.Warn().Msgf("Failed to auto-accept invitation after signup: %s", acceptErr.Error())
		} else {
			logger.Info().Msgf("Successfully auto-accepted invitation for user '%s'", user.Username)
		}
	}

	notificationControllerInterface, ncOk := ctx.Get("notificationController")
	if !ncOk {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}
	notificationController, ok := notificationControllerInterface.(*controllers.NotificationController)
	if !ok {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}
	if proviantConfig == nil {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}

	token, expiresAt, tokenErr := controllers.GenerateEmailVerificationToken()
	if tokenErr != nil {
		logger.Error().Msgf("Failed to generate email verification token: %s", tokenErr.Error())
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}
	if err := userRepo.CreateEmailVerification(user.ID, token, expiresAt); err != nil {
		logger.Error().Msgf("Failed to create email verification record: %s", err.Error())
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}

	go func() {
		if sendErr := notificationController.SendEmailVerification(user.MailAddress, user.Username, token, proviantConfig.Server.BaseURL, expiresAt); sendErr != nil {
			logger.Error().Msgf("Failed to send email verification: %s", sendErr.Error())
		} else {
			logger.Info().Msgf("Email verification sent to %s", user.MailAddress)
		}
	}()

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
}

// Logout revokes the current JWT token
// @Summary      	Logout user by revoking token
// @Description  	Revokes the current JWT token by adding its JTI to the blocklist
// @Tags         	auth
// @Accept			json
// @Produce      	json
// @Security		BearerAuth
// @Success      	200  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/auth/logout [post]
func Logout(ctx *gin.Context) {
	// Get logger instance from context
	loggerValue, loggerOk := ctx.Get("logger")
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}
	logger := loggerValue.(*zerolog.Logger)

	// Get database instance from context
	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract claims from current token
	claims := jwt.ExtractClaims(ctx)
	jti, exists := claims["jti"]
	if !exists {
		logger.Error().Msg("No JTI found in token claims")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Invalid token: no JTI"})
		return
	}

	jtiStr, ok := jti.(string)
	if !ok {
		logger.Error().Msg("JTI claim is not a string")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Invalid token: JTI not string"})
		return
	}

	exp, exists := claims["exp"]
	if !exists {
		logger.Error().Msg("No exp found in token claims")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Invalid token: no expiry"})
		return
	}

	expFloat, ok := exp.(float64)
	if !ok {
		logger.Error().Msg("exp claim is not a number")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Invalid token: expiry not number"})
		return
	}

	expiresAt := time.Unix(int64(expFloat), 0)

	// Create revoked token entry
	revokedToken := authentication.RevokedToken{
		JTI:       jtiStr,
		ExpiresAt: expiresAt,
	}

	// Insert into database
	db := dbHandle.(*gorm.DB)
	if err := db.Create(&revokedToken).Error; err != nil {
		logger.Error().Msgf("Failed to revoke token: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to logout"})
		return
	}

	logger.Info().Msgf("Token revoked: JTI %s", jtiStr)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Logged out successfully"})
}
