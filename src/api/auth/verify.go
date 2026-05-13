package auth

import (
	"errors"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// VerifyEmail verifies a user's email address using a token
// @Summary      Verify email
// @Description  Verifies a user's email address using a token from the verification email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token  query  string  true  "Verification token"
// @Success      200    {object}  api.APIResponse
// @Failure      400    {object}  api.APIResponse
// @Failure      500    {object}  api.APIResponse
// @Router       /auth/verify-email [post]
func VerifyEmail(ctx *gin.Context) {
	loggerValue, loggerOk := ctx.Get(util.ContextKeyLogger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}
	logger := loggerValue.(*zerolog.Logger)

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	token := ctx.Query("token")
	if token == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("token is required"))
		return
	}

	verification, err := repos.Users.GetEmailVerificationByToken(token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api.RespondError(ctx, http.StatusNotFound, errors.New("invalid verification token"))
			return
		}
		logger.Error().Msgf("Error looking up email verification token: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	if verification.Status == dbModel.EmailVerificationStatusVerified {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("email address already verified"))
		return
	}

	if verification.Status == dbModel.EmailVerificationStatusExpired || time.Now().After(verification.ExpiresAt) {
		_ = repos.Users.UpdateEmailVerificationStatus(token, dbModel.EmailVerificationStatusExpired)
		api.RespondError(ctx, http.StatusBadRequest, errors.New("verification token has expired"))
		return
	}

	now := time.Now()
	if err := repos.Users.UpdateUserEmailVerified(verification.UserID, now); err != nil {
		logger.Error().Msgf("Error updating user email verified status: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	if err := repos.Users.UpdateEmailVerificationStatus(token, dbModel.EmailVerificationStatusVerified); err != nil {
		logger.Error().Msgf("Error updating email verification status: %s", err.Error())
	}

	logger.Info().Msgf("Email verified for user ID %d", verification.UserID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Email address verified successfully"})
}
