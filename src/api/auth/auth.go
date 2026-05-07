// auth contains authentication method handlers
package auth

import (
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const UserWasCreated = "User was created"

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
	logger, ok := mustGetLogger(ctx)
	if !ok {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	var signup authentication.Signup
	if err := ctx.ShouldBindJSON(&signup); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	passwordValidator := passwordValidatorFromContext(ctx)
	if validationErr := signup.IsValidWithValidator(passwordValidator); validationErr != nil {
		logger.Error().Msgf("User data was invalid: '%s'", validationErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	user := authentication.User{
		Username:    signup.Username,
		Password:    signup.Password,
		MailAddress: signup.MailAddress,
	}

	if repos.Users.UserExistsByUsername(&user) {
		logger.Error().Msgf("User '%s' already exists", user.Username)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithUsernameExists)
		return
	}

	if repos.Users.UserExistsByMailAddress(&user) {
		logger.Error().Msgf("User with mail address '%s' already exists", user.MailAddress)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithMailAddressExists)
		return
	}

	if err := repos.Users.CreateUser(&user); err != nil {
		logger.Error().Msgf("User '%s' with ID '%d' could not be created. Error: %s", user.Username, user.ID, err.Error())
		ctx.JSON(http.StatusBadRequest, api.ResponseErrInvalidUserData)
		return
	}

	logger.Info().Msgf("New User '%s' with ID '%d' created", user.Username, user.ID)

	handleInviteAcceptance(repos, &signup, &user, logger)
	trySendEmailVerification(ctx, repos, &user, logger)

	ctx.JSON(http.StatusOK, api.APIResponse{Message: UserWasCreated})
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
	loggerValue, loggerOk := ctx.Get(util.ContextKeyLogger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}
	logger := loggerValue.(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	jti, exists := claims["jti"]
	if !exists {
		logger.Error().Msg("No JTI found in token claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Invalid token: no JTI"})
		return
	}

	jtiStr, ok := jti.(string)
	if !ok {
		logger.Error().Msg("JTI claim is not a string")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Invalid token: JTI not string"})
		return
	}

	exp, exists := claims["exp"]
	if !exists {
		logger.Error().Msg("No exp found in token claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Invalid token: no expiry"})
		return
	}

	expFloat, ok := exp.(float64)
	if !ok {
		logger.Error().Msg("exp claim is not a number")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Invalid token: expiry not number"})
		return
	}

	expiresAt := time.Unix(int64(expFloat), 0)

	revokedToken := authentication.RevokedToken{
		JTI:       jtiStr,
		ExpiresAt: expiresAt,
	}

	if err := dbHandle.Create(&revokedToken).Error; err != nil {
		logger.Error().Msgf("Failed to revoke token: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to logout"})
		return
	}

	logger.Info().Msgf("Token revoked: JTI %s", jtiStr)

	// Expire the HttpOnly JWT cookie so the browser discards it immediately.
	ctx.SetSameSite(http.SameSiteStrictMode)
	ctx.SetCookie("jwt", "", -1, "/", "", false, true)

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Logged out successfully"})
}
