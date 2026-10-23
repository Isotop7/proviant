package auth

import (
	"net/http"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/i18n"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func mustGetLogger(ctx *gin.Context) (*zerolog.Logger, bool) {
	loggerVal, ok := ctx.Get(util.ContextKeyLogger)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return nil, false
	}
	logger, ok := loggerVal.(*zerolog.Logger)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return nil, false
	}
	return logger, true
}

func mustGetRepos(ctx *gin.Context, logger *zerolog.Logger) (*database.RepositoryContainer, bool) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !exists || !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return nil, false
	}
	return repos, true
}

func passwordValidatorFromContext(ctx *gin.Context) *authentication.PasswordValidator {
	proviantConfigVal, ok := ctx.Get(util.ContextKeyProviantConfig)
	if !ok {
		return authentication.DefaultPasswordValidator()
	}
	proviantConfig, ok := proviantConfigVal.(*configuration.ProviantConfiguration)
	if !ok {
		return authentication.DefaultPasswordValidator()
	}
	return authentication.PasswordValidatorFromConfig(authentication.PasswordConfig{
		MinLength:        proviantConfig.Server.Authentication.PasswordMinLength,
		RequireUppercase: proviantConfig.Server.Authentication.PasswordRequireUppercase,
		RequireDigit:     proviantConfig.Server.Authentication.PasswordRequireDigit,
		RequireSpecial:   proviantConfig.Server.Authentication.PasswordRequireSpecial,
		CheckBreached:    proviantConfig.Server.Authentication.PasswordCheckBreached,
	})
}

func getNotificationController(ctx *gin.Context) (*controllers.NotificationController, bool) {
	ncVal, ok := ctx.Get(util.ContextKeyNotificationController)
	if !ok {
		return nil, false
	}
	nc, ok := ncVal.(*controllers.NotificationController)
	return nc, ok
}

// secureCookie reports whether cookies of this deployment may carry the
// Secure attribute. The server never terminates TLS itself, so the scheme of
// the configured base URL is the only statement about the transport to the
// browser.
func secureCookie(ctx *gin.Context) bool {
	proviantConfigVal, ok := ctx.Get(util.ContextKeyProviantConfig)
	if !ok {
		return false
	}
	proviantConfig, ok := proviantConfigVal.(*configuration.ProviantConfiguration)
	if !ok {
		return false
	}
	return util.IsHTTPSBaseURL(proviantConfig.Server.BaseURL)
}

func handleInviteAcceptance(repos *database.RepositoryContainer, signup *authentication.Signup, user *authentication.User, logger *zerolog.Logger) {
	if signup.InviteToken == "" {
		return
	}
	acceptErr := repos.Invitations.AcceptInvitation(signup.InviteToken, user.MailAddress, user.ID)
	if acceptErr != nil {
		logger.Warn().Msgf("Failed to auto-accept invitation after signup: %s", acceptErr.Error())
	} else {
		logger.Info().Msgf("Successfully auto-accepted invitation for user '%s'", user.Username)
	}
}

func trySendEmailVerification(ctx *gin.Context, repos *database.RepositoryContainer, user *authentication.User, logger *zerolog.Logger) {
	nc, ncOk := getNotificationController(ctx)
	if !ncOk {
		return
	}
	proviantConfigVal, pcOk := ctx.Get(util.ContextKeyProviantConfig)
	if !pcOk {
		return
	}
	proviantConfig, ok := proviantConfigVal.(*configuration.ProviantConfiguration)
	if !ok {
		return
	}
	if proviantConfig.Server.Authentication.SkipEmailVerification {
		return
	}
	token, expiresAt, tokenErr := controllers.GenerateEmailVerificationToken()
	if tokenErr != nil {
		logger.Error().Msgf("Failed to generate email verification token: %s", tokenErr.Error())
		return
	}
	if err := repos.Users.CreateEmailVerification(user.ID, token, expiresAt); err != nil {
		logger.Error().Msgf("Failed to create email verification record: %s", err.Error())
		return
	}
	householdLanguage := ""
	if household, householdErr := repos.Users.GetHouseholdByID(user.HouseholdID); householdErr == nil {
		householdLanguage = household.Language
	}
	lang := i18n.ResolveLanguage(user.Language, householdLanguage, "")
	go func() {
		if sendErr := nc.SendEmailVerification(user.MailAddress, user.EffectiveName(), token, proviantConfig.Server.BaseURL, expiresAt, lang); sendErr != nil {
			logger.Error().Msgf("Failed to send email verification: %s", sendErr.Error())
		} else {
			logger.Info().Msgf("Email verification sent to %s", user.MailAddress)
		}
	}()
}
