package controllers

import (
	"errors"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"
)

// TwoFactorPendingLifetime is how long a generated enrolment secret stays
// valid before it has to be regenerated.
const TwoFactorPendingLifetime = 10 * time.Minute

// TwoFactorService drives the TOTP second factor: enrolment, verification and
// recovery. It owns no state of its own, so the same instance serves the
// sign-in flow and the settings page.
type TwoFactorService struct {
	repos  *database.RepositoryContainer
	config *configuration.TwoFactorConfiguration
}

func NewTwoFactorService(
	repos *database.RepositoryContainer,
	config *configuration.TwoFactorConfiguration,
) *TwoFactorService {
	return &TwoFactorService{repos: repos, config: config}
}

// Enabled reports whether the deployment offers a second factor at all.
func (s *TwoFactorService) Enabled() bool {
	return s != nil && s.config != nil && s.config.Enabled
}

// Issuer is the account name shown in the authenticator app.
func (s *TwoFactorService) Issuer() string {
	if s == nil || s.config == nil || s.config.Issuer == "" {
		return util.DefaultTwoFactorIssuer
	}
	return s.config.Issuer
}

// RecoveryCodeCount is the number of recovery codes one enrolment issues.
func (s *TwoFactorService) RecoveryCodeCount() int {
	if s == nil || s.config == nil || s.config.RecoveryCodeCount < 1 {
		return util.DefaultTwoFactorRecoveryCodeCount
	}
	return s.config.RecoveryCodeCount
}

// EncryptionKey decodes the configured key that encrypts the TOTP secret.
func (s *TwoFactorService) EncryptionKey() ([]byte, error) {
	if s == nil || s.config == nil {
		return nil, apperrors.ErrServerEmptyTwoFactorKey
	}
	return DecodeTwoFactorEncryptionKey(s.config.EncryptionKey)
}

// HouseholdRequiresTwoFactor reports whether the household demands a second
// factor from its members.
func (s *TwoFactorService) HouseholdRequiresTwoFactor(householdID uint) (bool, error) {
	if householdID == 0 {
		return false, nil
	}
	household, err := s.repos.Households.GetHouseholdByID(householdID)
	if err != nil {
		return false, err
	}
	return household.RequireTwoFactor, nil
}

// StartEnrolment generates a TOTP secret for the account and returns the QR
// code the authenticator app scans. The secret is stored encrypted and only
// reaches the browser as pixels of that QR code.
func (s *TwoFactorService) StartEnrolment(user *authentication.User) (qr string, expiry time.Time, err error) {
	if user.TwoFactorEnabled {
		return "", time.Time{}, apperrors.ErrTwoFactorAlreadyEnabled
	}

	key, err := s.EncryptionKey()
	if err != nil {
		return "", time.Time{}, err
	}

	if reusedQR, reusedExpiry, ok := s.reusePendingEnrolment(user, key); ok {
		return reusedQR, reusedExpiry, nil
	}

	secret, uri, err := GenerateTwoFactorSecret(s.Issuer(), user.Username)
	if err != nil {
		return "", time.Time{}, err
	}

	encrypted, err := EncryptTwoFactorSecret(secret, key)
	if err != nil {
		return "", time.Time{}, err
	}

	qr, err = BuildTwoFactorQRDataURI(uri)
	if err != nil {
		return "", time.Time{}, err
	}

	expiry = time.Now().Add(TwoFactorPendingLifetime)
	if err := s.repos.Users.SetTwoFactorPendingSecret(user.ID, encrypted, expiry); err != nil {
		return "", time.Time{}, err
	}

	return qr, expiry, nil
}

// reusePendingEnrolment hands out the QR code of a pending enrolment that has
// not expired yet. The secret keeps its value: the user may already have
// scanned the QR code, and a fresh secret would invalidate it.
func (s *TwoFactorService) reusePendingEnrolment(user *authentication.User, key []byte) (string, time.Time, bool) {
	if user.TwoFactorPendingSecret == "" || user.TwoFactorPendingExpiry == nil {
		return "", time.Time{}, false
	}
	if !time.Now().Before(*user.TwoFactorPendingExpiry) {
		return "", time.Time{}, false
	}

	secret, err := DecryptTwoFactorSecret(user.TwoFactorPendingSecret, key)
	if err != nil {
		return "", time.Time{}, false
	}

	uri, err := TwoFactorURIFromSecret(secret, s.Issuer(), user.Username)
	if err != nil {
		return "", time.Time{}, false
	}

	qr, err := BuildTwoFactorQRDataURI(uri)
	if err != nil {
		return "", time.Time{}, false
	}

	return qr, *user.TwoFactorPendingExpiry, true
}

// ConfirmEnrolment activates the second factor once the user proved that the
// authenticator app holds the generated secret. It returns the recovery codes,
// which are shown exactly once.
func (s *TwoFactorService) ConfirmEnrolment(user *authentication.User, code string) ([]string, error) {
	if user.TwoFactorEnabled {
		return nil, apperrors.ErrTwoFactorAlreadyEnabled
	}
	if user.TwoFactorPendingSecret == "" {
		return nil, apperrors.ErrTwoFactorPendingExpired
	}
	if user.TwoFactorPendingExpiry == nil || !time.Now().Before(*user.TwoFactorPendingExpiry) {
		return nil, apperrors.ErrTwoFactorPendingExpired
	}

	key, err := s.EncryptionKey()
	if err != nil {
		return nil, err
	}

	secret, err := DecryptTwoFactorSecret(user.TwoFactorPendingSecret, key)
	if err != nil {
		return nil, err
	}

	if _, valid := VerifyTwoFactorCode(secret, code, time.Now()); !valid {
		return nil, apperrors.ErrTwoFactorInvalidCode
	}

	encrypted, err := EncryptTwoFactorSecret(secret, key)
	if err != nil {
		return nil, err
	}

	codes, err := GenerateRecoveryCodes(s.RecoveryCodeCount())
	if err != nil {
		return nil, err
	}

	hashes := make([]string, 0, len(codes))
	for _, recoveryCode := range codes {
		hashes = append(hashes, HashRecoveryCode(recoveryCode))
	}

	activated, err := s.repos.Users.EnableTwoFactor(user.ID, encrypted)
	if err != nil {
		return nil, err
	}
	if !activated {
		// Another confirmation activated the second factor first. Its codes
		// stay valid; the ones this call would have issued never existed.
		return nil, apperrors.ErrTwoFactorAlreadyEnabled
	}

	if err := s.repos.TOTPs.ReplaceRecoveryCodes(user.ID, hashes); err != nil {
		// The second factor is on but its recovery codes are missing. Take it
		// back off, so the account is not left without a way back in.
		_ = s.repos.Users.DisableTwoFactor(user.ID)
		return nil, err
	}

	return codes, nil
}

// Disable removes the second factor and every recovery code that belonged to
// it.
func (s *TwoFactorService) Disable(user *authentication.User) error {
	if !user.TwoFactorEnabled {
		return apperrors.ErrTwoFactorNotEnabled
	}
	return s.repos.Users.DisableTwoFactor(user.ID)
}

// VerifyCode checks a TOTP code against the stored secret. A code that was
// already accepted is rejected, so it cannot be replayed.
func (s *TwoFactorService) VerifyCode(user *authentication.User, code string) (bool, error) {
	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		return false, apperrors.ErrTwoFactorNotEnabled
	}

	key, err := s.EncryptionKey()
	if err != nil {
		return false, err
	}

	secret, err := DecryptTwoFactorSecret(user.TwoFactorSecret, key)
	if err != nil {
		return false, err
	}

	step, valid := VerifyTwoFactorCode(secret, code, time.Now())
	if !valid {
		return false, nil
	}

	advanced, err := s.repos.Users.AdvanceTwoFactorStep(user.ID, step)
	if err != nil {
		return false, err
	}
	return advanced, nil
}

// ConsumeRecoveryCode spends a recovery code. A successful recovery disables
// the second factor, so the account is protected by the password alone until
// the user enrols again. Spending the code and disabling the factor happen in
// one transaction: a recovery that consumed the code but failed to disable the
// factor would cost the user a code for nothing.
func (s *TwoFactorService) ConsumeRecoveryCode(user *authentication.User, code string) (bool, error) {
	return s.repos.Users.ConsumeRecoveryCodeAndDisable(user.ID, HashRecoveryCode(code), time.Now())
}

// twoFactorRejections maps the errors a user can act on to the class the
// logs and the audit trail carry. Every other failure is a server-side fault:
// it gets no class of its own and never reaches the client with its own
// message. One map decides both questions, so the two can never drift apart.
var twoFactorRejections = map[error]string{
	apperrors.ErrTwoFactorInvalidCode:    "invalid_code",
	apperrors.ErrTwoFactorPendingExpired: "setup_expired",
	apperrors.ErrTwoFactorAlreadyEnabled: "already_enabled",
	apperrors.ErrTwoFactorNotEnabled:     "not_enabled",
}

// IsTwoFactorRejection reports whether the error is one the user can act on.
func IsTwoFactorRejection(err error) bool {
	for candidate := range twoFactorRejections {
		if errors.Is(err, candidate) {
			return true
		}
	}
	return false
}

// TwoFactorRejectionReason names the reason a second factor was rejected
// without echoing a driver error. It is the class the audit log and the
// server log carry, never the raw error text.
func TwoFactorRejectionReason(err error) string {
	for candidate, reason := range twoFactorRejections {
		if errors.Is(err, candidate) {
			return reason
		}
	}
	return "internal_error"
}

// ErrTwoFactorDisabled is returned when a caller tries to act on a second
// factor that the deployment does not offer.
var ErrTwoFactorDisabled = errors.New("two-factor authentication is disabled on this server")
