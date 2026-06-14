package auth

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	forgotPasswordResponseGeneric = "If an account with that email address exists, a password reset link has been sent." //nolint:gosec // G101: user-facing message, not a credential
	passwordResetResponseOK       = "Password has been reset successfully."

	// passwordResetEmailTimeout caps the per-send SMTP attempt. A stalled
	// SMTP server must not leak goroutines or hold buffers indefinitely.
	passwordResetEmailTimeout = 30 * time.Second
)

type forgotPasswordRequest struct {
	MailAddress string `json:"mailAddress" binding:"required"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ForgotPassword starts a self-service password reset by emailing a reset link.
// Always returns the same response to avoid leaking which addresses are
// registered, and silently skips users with unverified email addresses.
func ForgotPassword(ctx *gin.Context) {
	logger, ok := mustGetLogger(ctx)
	if !ok {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	var req forgotPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("mailAddress is required"))
		return
	}

	mailAddress := strings.ToLower(strings.TrimSpace(req.MailAddress))
	if mailAddress == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("mailAddress is required"))
		return
	}

	respondGenericOK := func() {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: forgotPasswordResponseGeneric})
	}

	user, userErr := repos.Users.GetUserByMailAddress(mailAddress)
	if userErr != nil {
		if userErr != gorm.ErrRecordNotFound {
			logger.Error().Msgf("Error looking up user by mail address: %s", userErr.Error())
		}
		respondGenericOK()
		return
	}

	// Refuse to issue a reset for accounts whose email has never been
	// verified. This prevents an attacker who registered with a victim's
	// address (and thus receives signup mail at that address) from taking
	// over the account via password reset. The response is identical to
	// the not-found path so registration state is not leaked.
	if user.EmailVerifiedAt == nil {
		logger.Debug().Msgf("Skipping password reset for unverified email %s", mailAddress)
		respondGenericOK()
		return
	}

	nc, ncOk := getNotificationController(ctx)
	if !ncOk {
		respondGenericOK()
		return
	}
	proviantConfigVal, pcOk := ctx.Get(util.ContextKeyProviantConfig)
	if !pcOk {
		respondGenericOK()
		return
	}
	proviantConfig, ok := proviantConfigVal.(*configuration.ProviantConfiguration)
	if !ok {
		respondGenericOK()
		return
	}

	token, expiresAt, tokenErr := controllers.GeneratePasswordResetToken()
	if tokenErr != nil {
		logger.Error().Msgf("Failed to generate password reset token: %s", tokenErr.Error())
		respondGenericOK()
		return
	}

	if err := repos.Users.InvalidatePendingPasswordResetsForUser(user.ID); err != nil {
		logger.Warn().Msgf("Failed to invalidate pending password resets for user %d: %s", user.ID, err.Error())
	}
	if err := repos.Users.CreatePasswordReset(user.ID, token, expiresAt, ctx.ClientIP()); err != nil {
		logger.Error().Msgf("Failed to create password reset record: %s", err.Error())
		respondGenericOK()
		return
	}

	// Fire-and-forget the SMTP send with a bounded timeout and panic
	// recovery so a stalled or buggy mail server cannot leak goroutines.
	go func(email, name, rawToken, baseURL string, exp time.Time) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Msgf("Panic in password reset email goroutine: %v\n%s", r, debug.Stack())
			}
		}()
		sendCtx, cancel := context.WithTimeout(context.Background(), passwordResetEmailTimeout)
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- nc.SendPasswordReset(email, name, rawToken, baseURL, exp)
		}()
		select {
		case sendErr := <-done:
			if sendErr != nil {
				logger.Error().Msgf("Failed to send password reset email: %s", sendErr.Error())
			} else {
				logger.Info().Msgf("Password reset email sent to %s", email)
			}
		case <-sendCtx.Done():
			logger.Error().Msgf("Password reset email send to %s timed out after %s", email, passwordResetEmailTimeout)
		}
	}(user.MailAddress, user.EffectiveName(), token, proviantConfig.Server.BaseURL, expiresAt)

	respondGenericOK()
}

// ResetPassword consumes a password reset token and updates the user's
// password. The token is validated and consumed atomically: bcrypt and
// password policy checks only run after the token has been confirmed valid
// and not already consumed, so unauthenticated callers cannot force the
// server to do expensive CPU work.
func ResetPassword(ctx *gin.Context) {
	logger, ok := mustGetLogger(ctx)
	if !ok {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	var req resetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("token and password are required"))
		return
	}

	// Look up the token first. Only after the token is confirmed valid do
	// we run the (potentially expensive) password policy and bcrypt work.
	reset, lookupErr := repos.Users.GetPasswordResetByToken(req.Token)
	if lookupErr != nil {
		if lookupErr == gorm.ErrRecordNotFound {
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrPasswordResetTokenInvalid)
			return
		}
		logger.Error().Msgf("Error looking up password reset token: %s", lookupErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	if reset.UsedAt != nil {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrPasswordResetTokenUsed)
		return
	}

	now := time.Now()
	if !now.Before(reset.ExpiresAt) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrPasswordResetTokenExpired)
		return
	}

	passwordValidator := passwordValidatorFromContext(ctx)
	login := authentication.Login{
		Username: "_reset_", // unused for validation; only password is checked
		Password: req.Password,
	}
	if validationErr := login.IsValidWithValidator(passwordValidator); validationErr != nil {
		api.RespondError(ctx, http.StatusBadRequest, validationErr)
		return
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if hashErr != nil {
		logger.Error().Msgf("Failed to hash new password: %s", hashErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	consumed, consumeErr := repos.Users.ApplyPasswordReset(reset.UserID, reset.TokenHash, string(hashedPassword), now)
	if consumeErr != nil {
		logger.Error().Msgf("Failed to apply password reset for user %d: %s", reset.UserID, consumeErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}
	if !consumed {
		// Token was valid at lookup time but was consumed by a concurrent
		// request before our update landed, or it expired between the
		// check and the transaction. Treat as "already used" — the user
		// has not had their password changed.
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrPasswordResetTokenUsed)
		return
	}

	logger.Info().Msgf("Password reset successfully for user ID %d", reset.UserID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: passwordResetResponseOK})
}
