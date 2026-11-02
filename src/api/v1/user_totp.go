package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/audit"
	"github.com/Isotop7/proviant/controllers"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"
)

// GetTOTPStatus reports whether the authenticated user has a second factor and
// how many recovery codes are left.
// @Summary      Reports the two-factor state of the user
// @Description  Reports whether the user has enrolled a second factor and how many recovery codes are left
// @Tags         user
// @Produce      json
// @Success      200 {object} apiModel.TOTPStatusResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/user/totp [get]
// @Security     BearerAuth
func GetTOTPStatus(ctx *gin.Context, appCtx *AppContext) {
	enabled, err := twoFactorFeatureEnabled(ctx)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error reading the two-factor configuration: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}
	if !enabled {
		api.RespondError(ctx, http.StatusNotFound, controllers.ErrTwoFactorDisabled)
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusNotFound, apperrors.ErrInvalidUserData)
		return
	}

	remaining, err := appCtx.Repos.TOTPs.CountUnusedRecoveryCodes(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error counting the recovery codes: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusOK, apiModel.TOTPStatusResponse{
		Enabled:                user.TwoFactorEnabled,
		Pending:                user.TwoFactorPendingSecret != "",
		RecoveryCodesRemaining: remaining,
	})
}

// StartTOTPEnrolment generates a TOTP secret and returns the QR code the
// authenticator app scans. The secret itself never leaves the server.
// @Summary      Starts a two-factor enrolment
// @Description  Generates a TOTP secret and returns it as a QR code image. The secret is stored encrypted and is never part of the response
// @Tags         user
// @Produce      json
// @Success      200 {object} apiModel.TOTPEnrolResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/user/totp [post]
// @Security     BearerAuth
func StartTOTPEnrolment(ctx *gin.Context, appCtx *AppContext) {
	twoFactor, status, err := twoFactorServiceFromContext(ctx, appCtx)
	if err != nil {
		api.RespondError(ctx, status, err)
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusNotFound, apperrors.ErrInvalidUserData)
		return
	}

	qr, expiry, err := twoFactor.StartEnrolment(&user)
	if err != nil {
		respondTwoFactorError(ctx, appCtx.Logger, err)
		return
	}

	appCtx.Logger.Info().Msgf("Two-factor enrolment started for user ID %d", appCtx.UserID)

	ctx.JSON(http.StatusOK, apiModel.TOTPEnrolResponse{
		QR:        qr,
		ExpiresAt: expiry.Format(time.RFC3339),
	})
}

// ConfirmTOTPEnrolment activates the second factor once the user proved that
// the authenticator app holds the generated secret. The recovery codes are
// returned exactly once.
// @Summary      Confirms a two-factor enrolment
// @Description  Verifies the code of the pending enrolment and activates the second factor. The returned recovery codes are shown once and only their hashes are stored
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "Code"
// @Success      200 {object} apiModel.TOTPConfirmResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/user/totp/confirm [post]
// @Security     BearerAuth
func ConfirmTOTPEnrolment(ctx *gin.Context, appCtx *AppContext) {
	twoFactor, status, err := twoFactorServiceFromContext(ctx, appCtx)
	if err != nil {
		api.RespondError(ctx, status, err)
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserData)
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusNotFound, apperrors.ErrInvalidUserData)
		return
	}

	codes, err := twoFactor.ConfirmEnrolment(&user, req.Code)
	if err != nil {
		appCtx.Logger.Warn().Msgf("The two-factor enrolment of user ID %d was rejected: %s", appCtx.UserID, controllers.TwoFactorRejectionReason(err))
		go recordTwoFactorRejection(audit.FromGin(ctx), appCtx.UserID, controllers.TwoFactorRejectionReason(err))
		respondTwoFactorError(ctx, appCtx.Logger, err)
		return
	}

	appCtx.Logger.Info().Msgf("Two-factor authentication enabled for user ID %d", appCtx.UserID)
	go recordTwoFactorEvent(audit.FromGin(ctx), appCtx.UserID, dbModel.AuditActionTwoFactorEnabled)
	sendTwoFactorNotification(ctx, appCtx, &user)

	ctx.JSON(http.StatusOK, apiModel.TOTPConfirmResponse{
		TwoFactorEnabled: true,
		RecoveryCodes:    codes,
	})
}

// sendTwoFactorNotification tells the account owner that a second factor was
// enrolled. The first enrolment rests on the password alone, so without this
// notice an attacker with a leaked password could enrol their own device
// unnoticed.
func sendTwoFactorNotification(ctx *gin.Context, appCtx *AppContext, user *authentication.User) {
	notificationController, ok := ctx.Get(util.ContextKeyNotificationController)
	if !ok {
		return
	}
	nc, ok := notificationController.(*controllers.NotificationController)
	if !ok || nc == nil {
		return
	}

	lang := twoFactorNotificationLanguage(appCtx, user)
	go func() {
		if err := nc.SendTwoFactorEnabled(user.MailAddress, user.EffectiveName(), lang); err != nil {
			appCtx.Logger.Warn().Msgf("Failed to send the two-factor notification: %s", err.Error())
		}
	}()
}

// twoFactorNotificationLanguage resolves the language of the enrolment mail:
// the preference of the user, then the one of the household.
func twoFactorNotificationLanguage(appCtx *AppContext, user *authentication.User) string {
	userLanguage, householdLanguage, err := appCtx.Repos.Users.GetUserDisplayPreferencesByID(user.ID)
	if err != nil {
		appCtx.Logger.Warn().Msgf("Error reading the language preferences of user ID %d: %s", user.ID, err.Error())
		return i18n.ResolveLanguage(user.Language, "", "")
	}
	return i18n.ResolveLanguage(userLanguage, householdLanguage, "")
}

// DisableTOTP removes the second factor from the account. The current
// password confirms the request.
// @Summary      Disables the two-factor authentication
// @Description  Removes the TOTP secret and the recovery codes after the current password was confirmed
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "Password"
// @Success      200 {object} api.APIResponse
// @Failure      400 {object} api.APIResponse
// @Failure      401 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/user/totp/disable [post]
// @Security     BearerAuth
func DisableTOTP(ctx *gin.Context, appCtx *AppContext) {
	twoFactor, status, err := twoFactorServiceFromContext(ctx, appCtx)
	if err != nil {
		api.RespondError(ctx, status, err)
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserData)
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusNotFound, apperrors.ErrInvalidUserData)
		return
	}

	passwordValid, err := appCtx.Repos.Users.VerifyPassword(appCtx.UserID, req.Password)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error verifying the password: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}
	if !passwordValid {
		appCtx.Logger.Warn().Msgf("Wrong password while disabling the two-factor authentication of user ID %d", appCtx.UserID)
		api.RespondErrorWithReason(ctx, http.StatusUnauthorized, apperrors.ErrInvalidCredentials, api.ReasonInvalidCredentials)
		return
	}

	if err := twoFactor.Disable(&user); err != nil {
		respondTwoFactorError(ctx, appCtx.Logger, err)
		return
	}

	appCtx.Logger.Info().Msgf("Two-factor authentication disabled for user ID %d", appCtx.UserID)
	go recordTwoFactorEvent(audit.FromGin(ctx), appCtx.UserID, dbModel.AuditActionTwoFactorDisabled)

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Two-factor authentication disabled"})
}

// recordTwoFactorEvent writes a security relevant change of the second factor
// to the audit log.
func recordTwoFactorEvent(values audit.Values, userID uint, action string) {
	values.Log(userID, action, map[string]string{})
}

// recordTwoFactorRejection writes the audit entry of a rejected second factor.
func recordTwoFactorRejection(values audit.Values, userID uint, reason string) {
	values.Log(userID, dbModel.AuditActionTwoFactorFailed, map[string]string{"reason": reason})
}

// respondTwoFactorError answers a failed two-factor request. The named errors
// the user can act on keep their status and their message; everything else is
// a server-side fault and gets a generic body, so a configuration problem or a
// driver error never reaches the client. One log line per failure, at the
// level the failure deserves.
func respondTwoFactorError(ctx *gin.Context, logger *zerolog.Logger, err error) {
	if controllers.IsTwoFactorRejection(err) {
		logger.Warn().Msgf("The two-factor request was rejected: %s", controllers.TwoFactorRejectionReason(err))
		api.RespondErrorWithReason(ctx, http.StatusBadRequest, err, twoFactorErrorReason(err))
		return
	}

	logger.Error().Msgf("The two-factor request failed: %s", err.Error())
	api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
}

// twoFactorErrorCode names the cause for the client. It is empty for a failure
// the user cannot act on.
func twoFactorErrorReason(err error) string {
	switch {
	case errors.Is(err, apperrors.ErrTwoFactorNotEnabled):
		return api.ReasonTwoFactorNotEnabled
	case errors.Is(err, apperrors.ErrInvalidCredentials):
		return api.ReasonInvalidCredentials
	default:
		return ""
	}
}

// twoFactorConfigFromContext reads the server configuration from the request.
// A missing configuration is a wiring mistake, not a disabled feature.
func twoFactorConfigFromContext(ctx *gin.Context) (*configuration.ProviantConfiguration, error) {
	config, ok := ctx.Get(util.ContextKeyProviantConfig)
	if !ok {
		return nil, apperrors.ErrInternalServer
	}
	proviantConfig, ok := config.(*configuration.ProviantConfiguration)
	if !ok {
		return nil, apperrors.ErrInternalServer
	}
	return proviantConfig, nil
}

// twoFactorFeatureEnabled reports whether the deployment offers a second
// factor. The household policy endpoint only needs the switch, not the whole
// service.
func twoFactorFeatureEnabled(ctx *gin.Context) (bool, error) {
	proviantConfig, err := twoFactorConfigFromContext(ctx)
	if err != nil {
		return false, err
	}
	return proviantConfig.Server.TwoFactor.Enabled, nil
}

// twoFactorServiceFromContext builds the two-factor service for the request.
// It returns the status the handler has to answer with when no service is
// available: 404 when the deployment offers no second factor, 500 when the
// configuration is missing from the request — that one is a wiring mistake
// and must not be reported to the user as "the feature is off".
func twoFactorServiceFromContext(ctx *gin.Context, appCtx *AppContext) (*controllers.TwoFactorService, int, error) {
	proviantConfig, err := twoFactorConfigFromContext(ctx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	service := controllers.NewTwoFactorService(appCtx.Repos, &proviantConfig.Server.TwoFactor)
	if !service.Enabled() {
		return nil, http.StatusNotFound, controllers.ErrTwoFactorDisabled
	}
	return service, http.StatusOK, nil
}
