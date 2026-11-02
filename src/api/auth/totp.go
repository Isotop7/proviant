// auth contains authentication method handlers
package auth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/audit"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"
)

// ConfirmTwoFactorSetup activates the second factor for a member of a
// household that requires one. The member has no session yet, so the password
// confirms the request, and the code proves that the authenticator app holds
// the secret the sign-in handed out.
//
// The recovery codes are returned exactly once and only their hashes are
// stored.
// @Summary      Confirms a two-factor setup
// @Description  Activates the TOTP second factor for an account whose household requires one. The account is confirmed with its password and the code of the pending enrolment
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "Username, password and code"
// @Success      200 {object} apiModel.TOTPConfirmResponse
// @Failure      400 {object} api.APIResponse
// @Failure      401 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /auth/totp/setup/confirm [post]
func ConfirmTwoFactorSetup(ctx *gin.Context) {
	logger, ok := mustGetLogger(ctx)
	if !ok {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	proviantConfigVal, configOk := ctx.Get(util.ContextKeyProviantConfig)
	if !configOk {
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}
	proviantConfig, configOk := proviantConfigVal.(*configuration.ProviantConfiguration)
	if !configOk {
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	twoFactor := controllers.NewTwoFactorService(repos, &proviantConfig.Server.TwoFactor)
	if !twoFactor.Enabled() {
		api.RespondError(ctx, http.StatusNotFound, controllers.ErrTwoFactorDisabled)
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserData)
		return
	}

	user, err := repos.Users.GetUserByUsername(req.Username)
	if err != nil {
		logger.Debug().Msg("The two-factor setup was requested for an unknown user")
		api.RespondError(ctx, http.StatusUnauthorized, apperrors.ErrInvalidCredentials)
		return
	}

	maxLoginAttempts, lockoutDurationMins := proviantConfig.LockoutConfig()

	// The lockout is answered before anything else, exactly like on the
	// sign-in route, so both routes agree on the state of an account.
	if locked, remaining := repos.Users.IsAccountLocked(user.ID, maxLoginAttempts, lockoutDurationMins); locked {
		ctx.Header("Retry-After", strconv.Itoa(int(remaining.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many failed login attempts, account is temporarily locked"))
		return
	}

	// The state of the pending enrolment is knowable without the password, so
	// answering on it reveals nothing about the password. Without this the
	// user whose QR code expired would only learn "invalid credentials".
	//
	// An account that never started an enrolment answers exactly like an
	// unknown user, so the endpoint cannot confirm that a username exists.
	if user.TwoFactorPendingSecret == "" {
		api.RespondError(ctx, http.StatusUnauthorized, apperrors.ErrInvalidCredentials)
		return
	}
	if user.TwoFactorPendingExpiry == nil || !time.Now().Before(*user.TwoFactorPendingExpiry) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrTwoFactorSetupExpired)
		return
	}

	// The household policy is checked before the password, so the answer never
	// depends on whether the password was right. It comes after the pending
	// enrolment, so the accounts it can name are the ones that hold a pending
	// enrolment and not every account of the instance.
	householdRequired, err := twoFactor.HouseholdRequiresTwoFactor(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Error reading the household two-factor policy: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}
	if !householdRequired {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrTwoFactorNotRequired)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		// A wrong password counts against the account lockout, exactly like a
		// wrong password on the sign-in route. Otherwise this endpoint would
		// be a way around the lockout.
		_ = repos.Users.RecordFailedLoginAttempt(user.ID, maxLoginAttempts, lockoutDurationMins)
		logger.Warn().Msgf("Wrong password during the two-factor setup of user ID %d", user.ID)
		api.RespondError(ctx, http.StatusUnauthorized, apperrors.ErrInvalidCredentials)
		return
	}

	codes, err := twoFactor.ConfirmEnrolment(&user, req.Code)
	if err != nil {
		// A rejected code answers exactly like a rejected password, so the
		// endpoint never confirms that a password guess was right. The reason
		// stays in the log and in the audit trail.
		logger.Warn().Msgf("The two-factor setup of user ID %d was rejected: %s", user.ID, controllers.TwoFactorRejectionReason(err))
		go recordTwoFactorRejection(audit.FromGin(ctx), user.ID, controllers.TwoFactorRejectionReason(err))
		api.RespondError(ctx, http.StatusUnauthorized, apperrors.ErrInvalidCredentials)
		return
	}

	logger.Info().Msgf("Two-factor authentication enabled for user ID %d", user.ID)
	go recordTwoFactorEvent(audit.FromGin(ctx), user.ID, dbModel.AuditActionTwoFactorEnabled)
	sendTwoFactorNotification(ctx, repos, &user, logger)

	ctx.JSON(http.StatusOK, apiModel.TOTPConfirmResponse{
		TwoFactorEnabled: true,
		RecoveryCodes:    codes,
	})
}

// sendTwoFactorNotification tells the account owner that a second factor was
// enrolled. The first enrolment rests on the password alone, so without this
// notice an attacker with a leaked password could enrol their own device
// unnoticed.
func sendTwoFactorNotification(ctx *gin.Context, repos *database.RepositoryContainer, user *authentication.User, logger *zerolog.Logger) {
	nc, ok := getNotificationController(ctx)
	if !ok || nc == nil {
		return
	}

	userLanguage, householdLanguage, err := repos.Users.GetUserDisplayPreferencesByID(user.ID)
	if err != nil {
		userLanguage, householdLanguage = user.Language, ""
	}
	lang := i18n.ResolveLanguage(userLanguage, householdLanguage, "")

	go func() {
		if err := nc.SendTwoFactorEnabled(user.MailAddress, user.EffectiveName(), lang); err != nil {
			logger.Warn().Msgf("Failed to send the two-factor notification: %s", err.Error())
		}
	}()
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
