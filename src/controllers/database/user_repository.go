package database

import (
	"fmt"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// twoFactorColumns are the columns that hold the state of the second factor.
// They are owned by the enrolment and verification paths alone, so every
// generic full-record save has to omit them: a save that replays a stale
// snapshot would wipe a secret the user just generated and reset the replay
// guard of the last accepted code.
var twoFactorColumns = []string{
	"two_factor_secret",
	"two_factor_enabled",
	"two_factor_last_used_step",
	"two_factor_pending_secret",
	"two_factor_pending_expiry",
}

type UserRepositoryInterface interface {
	GetUserByUsername(username string) (authentication.User, error)
	GetUserByID(userID uint) (authentication.User, error)
	GetUserHouseholdByID(userID uint) (uint, error)
	GetUserDisplayPreferencesByID(userID uint) (string, string, error)
	GetUserHouseholdRole(userID uint) (string, error)
	UpdateUserHouseholdRole(userID, householdID uint, role string) error
	UserExistsByUsername(user *authentication.User) bool
	UserExistsByMailAddress(user *authentication.User) bool
	CreateUser(user *authentication.User) error
	UpdateUser(userID uint, user *authentication.User) error
	UpdateUserReceiptScanSettings(userID uint, prefs authentication.ReceiptScanPreferences, apiKey *string) error
	UpdateUserDashboardLayout(userID uint, layout string) error
	UpdateAdminUserFields(userID uint, username, mailAddress string) error
	UpdateDisplayName(userID uint, displayName string) error
	UpdateUserPassword(userID uint, login *authentication.Login) error
	VerifyPassword(userID uint, password string) (bool, error)
	EnableTwoFactor(userID uint, encryptedSecret string) (bool, error)
	DisableTwoFactor(userID uint) error
	SetTwoFactorPendingSecret(userID uint, encryptedSecret string, expiry time.Time) error
	// AdvanceTwoFactorStep stores the time step of the last accepted code.
	// The update only lands when the stored step is older, so a code that was
	// already accepted cannot be replayed.
	AdvanceTwoFactorStep(userID uint, step int64) (bool, error)
	// ConsumeRecoveryCodeAndDisable spends a recovery code and turns the
	// second factor off in one transaction.
	ConsumeRecoveryCodeAndDisable(userID uint, codeHash string, usedAt time.Time) (bool, error)
	IsAccountLocked(userID uint, maxLoginAttempts int, lockoutDurationMins int) (bool, time.Duration)
	RecordFailedLoginAttempt(userID uint, maxLoginAttempts int, lockoutDurationMins int) error
	ResetFailedLoginAttempts(userID uint) error
	CreateEmailVerification(userID uint, token string, expiresAt time.Time) error
	GetEmailVerificationByToken(token string) (database.EmailVerification, error)
	UpdateUserEmailVerified(userID uint, verifiedAt time.Time) error
	UpdateEmailVerification(userID uint, verifiedAt *time.Time) error
	UpdateEmailVerificationStatus(token, status string) error
	GetUserByMailAddress(mailAddress string) (authentication.User, error)
	CreatePasswordReset(userID uint, token string, expiresAt time.Time, ipAddress string) error
	GetPasswordResetByToken(token string) (database.PasswordReset, error)
	MarkPasswordResetUsed(resetID uint, usedAt time.Time) error
	// ConsumePasswordReset atomically marks the reset row as used only if it
	// is still pending and not expired. Returns (true, nil) on success,
	// (false, nil) if the token is missing/already-used/expired, and
	// (false, err) on DB error. Callers should treat false as "token is not
	// consumable" and not proceed with the password update.
	ConsumePasswordReset(tokenHash string, usedAt time.Time) (bool, error)
	// ApplyPasswordReset atomically (a) sets the user's password hash,
	// (b) consumes the reset token, and (c) invalidates all other pending
	// resets for the user. Returns the same semantics as ConsumePasswordReset
	// for the token-consumed flag, plus any DB error.
	ApplyPasswordReset(userID uint, tokenHash, hashedPassword string, usedAt time.Time) (bool, error)
	DeleteExpiredPasswordResets(before time.Time) error
	InvalidatePendingPasswordResetsForUser(userID uint) error
	SetUserPasswordHash(userID uint, hashedPassword string) error
	GetOnboardingState(userID uint) (database.OnboardingState, error)
	MarkNotificationsSetup(userID uint) error
	UpdateUsername(userID uint, username string) error
	MarkProfileStepDone(userID uint) error
	MarkHouseholdStepDone(userID uint) error
	MarkOnboardingComplete(userID uint) error
	EnsureOnboardingState(userID uint) error
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetUsersByHouseholdID(householdID uint) ([]authentication.User, error)
	DeleteUser(userID uint) error
}

var _ UserRepositoryInterface = (*UserRepository)(nil)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{DB: db}
}

const (
	DefaultMaxLoginAttempts    = 10
	DefaultLockoutDurationMins = 15
)

func (r *UserRepository) GetUserByUsername(username string) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, "username = ?", username)
	return user, selectErr.Error
}

func (r *UserRepository) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user, selectErr.Error
}

func (r *UserRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user.HouseholdID, selectErr.Error
}

// GetUserDisplayPreferencesByID returns the user's language preference and the
// language preference of the user's household (empty string = automatic) in one
// query, so the per-request middleware and the offline sync need a single
// round trip.
func (r *UserRepository) GetUserDisplayPreferencesByID(userID uint) (string, string, error) {
	var result struct {
		UserLanguage      string
		HouseholdLanguage string
	}
	row := r.DB.Model(&authentication.User{}).
		Select("users.language AS user_language, households.language AS household_language").
		Joins("LEFT JOIN households ON households.id = users.household_id AND households.deleted_at IS NULL").
		Where("users.id = ?", userID).
		Scan(&result)
	return result.UserLanguage, result.HouseholdLanguage, row.Error
}

func (r *UserRepository) GetUserHouseholdRole(userID uint) (string, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user.Role, selectErr.Error
}

func (r *UserRepository) UpdateUserHouseholdRole(userID, householdID uint, role string) error {
	return r.DB.Model(&authentication.User{}).
		Where("id = ? AND household_id = ?", userID, householdID).
		Update("role", role).Error
}

func (r *UserRepository) UserExistsByUsername(user *authentication.User) bool {
	var dbUser authentication.User
	selectErr := r.DB.First(&dbUser, "username = ?", user.Username)
	return selectErr.Error != gorm.ErrRecordNotFound
}

func (r *UserRepository) UserExistsByMailAddress(user *authentication.User) bool {
	var dbUser authentication.User
	selectErr := r.DB.First(&dbUser, "mail_address = ?", user.MailAddress)
	return selectErr.Error != gorm.ErrRecordNotFound
}

func (r *UserRepository) CreateUser(user *authentication.User) error {
	tx := r.DB.Begin()

	household := database.Household{
		Name: fmt.Sprintf("%s's Household", user.Username),
	}
	if err := tx.Create(&household).Error; err != nil {
		tx.Rollback()
		return err
	}

	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	user.HouseholdID = household.ID
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&household).Update("admin_id", user.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	onboardingState := database.OnboardingState{
		UserID: user.ID,
	}
	if err := tx.Create(&onboardingState).Error; err != nil {
		tx.Rollback()
		return err
	}

	defaultLocations := []database.StorageLocation{
		{HouseholdID: household.ID, Name: "Fridge", Icon: "🧊", SortOrder: 0},
		{HouseholdID: household.ID, Name: "Freezer", Icon: "❄️", SortOrder: 1},
		{HouseholdID: household.ID, Name: "Pantry", Icon: "🗄️", SortOrder: 2},
	}
	for i := range defaultLocations {
		if err := tx.Create(&defaultLocations[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (r *UserRepository) UpdateUser(userID uint, user *authentication.User) error {
	if userID == 0 {
		return errors.ErrInvalidUserID
	}

	var dbUser authentication.User
	getError := r.DB.First(&dbUser, userID)

	if getError.Error != nil {
		return getError.Error
	}
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	dbUser.DisplayName = user.DisplayName
	dbUser.MailAddress = user.MailAddress
	dbUser.Language = user.Language
	dbUser.NotificationPreferences = user.NotificationPreferences

	// Receipt-scan columns (including the API key) are owned exclusively by
	// UpdateUserReceiptScanSettings. Omitting them from this full-record save
	// keeps a generic update from replaying a stale snapshot and reverting a
	// concurrently saved preference or resurrecting a cleared API key.
	//
	// The two-factor columns are owned by the enrolment endpoints. The same
	// reasoning applies: a save that replays a stale snapshot would wipe a
	// secret the user just generated and reset the replay guard.
	omitColumns := append([]string{
		"receipt_scan_override_enabled",
		"receipt_scan_endpoint",
		"receipt_scan_model",
		"receipt_scan_timeout",
		"receipt_scan_api_key",
		"dashboard_layout",
	}, twoFactorColumns...)

	saveResult := r.DB.Omit(omitColumns...).Save(&dbUser)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

// UpdateUserReceiptScanSettings persists the receipt scan preference columns
// and, when apiKey is non-nil, the API key column in a single transaction.
// The key stays out of the preference write set — a keep-the-key save
// (apiKey == nil) must not replay a stale value and resurrect a key that was
// cleared concurrently — and the transaction guarantees the new endpoint can
// never go live while the old key is still stored (or vice versa).
func (r *UserRepository) UpdateUserReceiptScanSettings(userID uint, prefs authentication.ReceiptScanPreferences, apiKey *string) error {
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Model(&authentication.User{}).
			Where("id = ?", userID).
			Updates(map[string]any{
				"receipt_scan_override_enabled": prefs.OverrideEnabled,
				"receipt_scan_endpoint":         prefs.Endpoint,
				"receipt_scan_model":            prefs.Model,
				"receipt_scan_timeout":          prefs.Timeout,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}

		if apiKey == nil {
			return nil
		}

		keyResult := tx.Model(&authentication.User{}).
			Where("id = ?", userID).
			Update("receipt_scan_api_key", *apiKey)
		return keyResult.Error
	})
}

// UpdateUserDashboardLayout stores the arrangement of the dashboard tiles. The
// column is written on its own so a concurrent save of another preference can
// never be replayed from a stale snapshot.
func (r *UserRepository) UpdateUserDashboardLayout(userID uint, layout string) error {
	if userID <= 0 {
		return errors.ErrInvalidUserID
	}

	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]any{"dashboard_layout": layout}).Error
}

// UpdateAdminUserFields allows admins to change login-credential fields (username, email).
func (r *UserRepository) UpdateAdminUserFields(userID uint, username, mailAddress string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]any{
			"username":     username,
			"mail_address": mailAddress,
		}).Error
}

func (r *UserRepository) UpdateDisplayName(userID uint, displayName string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Update("display_name", displayName).Error
}

func (r *UserRepository) UpdateUserPassword(userID uint, login *authentication.Login) error {
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	var dbUser authentication.User
	getError := r.DB.First(&dbUser, userID)

	if getError.Error != nil {
		return getError.Error
	}
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	if dbUser.Username != login.Username {
		return errors.ErrMismatchedUsername
	}

	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(login.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	dbUser.Password = string(hashedPassword)

	// The two-factor columns are owned by the enrolment paths. Writing them
	// back from this snapshot would wipe a secret the user generated while the
	// password was being changed.
	saveResult := r.DB.Omit(twoFactorColumns...).Save(&dbUser)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

func (r *UserRepository) IsAccountLocked(userID uint, maxLoginAttempts int, lockoutDurationMins int) (bool, time.Duration) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return false, 0
	}
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		remaining := time.Until(user.LockedUntil.Time)
		return true, remaining
	}
	return false, 0
}

func (r *UserRepository) RecordFailedLoginAttempt(userID uint, maxLoginAttempts int, lockoutDurationMins int) error {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return err
	}
	user.FailedLoginAttempts++
	if maxLoginAttempts > 0 && user.FailedLoginAttempts >= uint(maxLoginAttempts) {
		lockoutUntil := time.Now().Add(time.Duration(lockoutDurationMins) * time.Minute)
		return r.DB.Model(&user).Updates(map[string]any{
			"failed_login_attempts": user.FailedLoginAttempts,
			"locked_until":          lockoutUntil,
		}).Error
	}
	return r.DB.Model(&user).Update("failed_login_attempts", user.FailedLoginAttempts).Error
}

func (r *UserRepository) ResetFailedLoginAttempts(userID uint) error {
	return r.DB.Model(&authentication.User{}).Where(util.QueryId, userID).Updates(map[string]any{
		"failed_login_attempts": 0,
		"locked_until":          nil,
	}).Error
}

func (r *UserRepository) CreateEmailVerification(userID uint, token string, expiresAt time.Time) error {
	verification := database.EmailVerification{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		Status:    database.EmailVerificationStatusPending,
	}
	return r.DB.Create(&verification).Error
}

func (r *UserRepository) GetEmailVerificationByToken(token string) (database.EmailVerification, error) {
	var verification database.EmailVerification
	result := r.DB.Where("token = ?", token).First(&verification)
	return verification, result.Error
}

func (r *UserRepository) UpdateUserEmailVerified(userID uint, verifiedAt time.Time) error {
	return r.DB.Model(&authentication.User{}).Where(util.QueryId, userID).Update("email_verified_at", verifiedAt).Error
}

func (r *UserRepository) UpdateEmailVerification(userID uint, verifiedAt *time.Time) error {
	return r.DB.Model(&authentication.User{}).Where(util.QueryId, userID).Update("email_verified_at", verifiedAt).Error
}

func (r *UserRepository) UpdateEmailVerificationStatus(token, status string) error {
	return r.DB.Model(&database.EmailVerification{}).Where("token = ?", token).Update("status", status).Error
}

func (r *UserRepository) GetOnboardingState(userID uint) (database.OnboardingState, error) {
	var onboardingState database.OnboardingState
	err := r.DB.Where(util.QueryUserId, userID).First(&onboardingState).Error
	return onboardingState, err
}

func (r *UserRepository) MarkNotificationsSetup(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where(util.QueryUserId, userID).
		Update("notifications_setup", true).Error
}

func (r *UserRepository) UpdateUsername(userID uint, username string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Update("username", username).Error
}

func (r *UserRepository) MarkProfileStepDone(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where(util.QueryUserId, userID).
		Update("profile_step_done", true).Error
}

func (r *UserRepository) MarkHouseholdStepDone(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where(util.QueryUserId, userID).
		Update("household_step_done", true).Error
}

func (r *UserRepository) MarkOnboardingComplete(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where(util.QueryUserId, userID).
		Update("onboarding_completed", true).Error
}

func (r *UserRepository) EnsureOnboardingState(userID uint) error {
	var existing database.OnboardingState
	findErr := r.DB.Where(util.QueryUserId, userID).First(&existing).Error
	if findErr == nil {
		return nil
	}
	if findErr != gorm.ErrRecordNotFound {
		return findErr
	}
	state := database.OnboardingState{UserID: userID}
	return r.DB.Create(&state).Error
}

func (r *UserRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	selectErr := r.DB.First(&household, householdID)
	return household, selectErr.Error
}

func (r *UserRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where(util.QueryHouseholdId, householdID).Find(&users).Error
	return users, err
}

func (r *UserRepository) DeleteUser(userID uint) error {
	return r.DB.Delete(&authentication.User{}, userID).Error
}

func (r *UserRepository) GetUserByMailAddress(mailAddress string) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, "mail_address = ?", mailAddress)
	return user, selectErr.Error
}

// CreatePasswordReset persists a new password-reset row. The raw token is
// SHA-256 hashed before storage; only the hash is written to the database.
func (r *UserRepository) CreatePasswordReset(userID uint, token string, expiresAt time.Time, ipAddress string) error {
	reset := database.PasswordReset{
		UserID:    userID,
		TokenHash: database.HashPasswordResetToken(token),
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
	}
	return r.DB.Create(&reset).Error
}

// GetPasswordResetByToken looks up a reset by the raw token submitted by the
// client (form POST or API body). The raw token is hashed before the lookup.
func (r *UserRepository) GetPasswordResetByToken(token string) (database.PasswordReset, error) {
	var reset database.PasswordReset
	result := r.DB.Where("token_hash = ?", database.HashPasswordResetToken(token)).First(&reset)
	return reset, result.Error
}

func (r *UserRepository) MarkPasswordResetUsed(resetID uint, usedAt time.Time) error {
	return r.DB.Model(&database.PasswordReset{}).
		Where(util.QueryId, resetID).
		Updates(map[string]any{
			"used_at": usedAt,
		}).Error
}

// ConsumePasswordReset atomically marks a reset row as used only if it is
// still pending and not expired. The conditional WHERE makes this safe under
// concurrent use: the second concurrent caller sees zero rows affected.
func (r *UserRepository) ConsumePasswordReset(tokenHash string, usedAt time.Time) (bool, error) {
	result := r.DB.Model(&database.PasswordReset{}).
		Where("token_hash = ?", tokenHash).
		Where("used_at IS NULL").
		Where("expires_at > ?", usedAt).
		Update("used_at", usedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ApplyPasswordReset runs the password update, token consumption, and
// invalidation of other pending resets in a single transaction. It returns
// (consumed, err) where consumed is true only if the token was the one
// that actually got consumed — i.e. was pending, unexpired, and the update
// succeeded. On consumed=false the password has NOT been changed and the
// caller should respond with the appropriate token-invalid error.
func (r *UserRepository) ApplyPasswordReset(userID uint, tokenHash, hashedPassword string, usedAt time.Time) (bool, error) {
	var consumed bool
	txErr := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&authentication.User{}).
			Where(util.QueryId, userID).
			Update("password", hashedPassword).Error; err != nil {
			return err
		}
		result := tx.Model(&database.PasswordReset{}).
			Where("token_hash = ?", tokenHash).
			Where("user_id = ?", userID).
			Where("used_at IS NULL").
			Where("expires_at > ?", usedAt).
			Update("used_at", usedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// Token was already used, expired, or unknown for this user.
			// Aborting the transaction rolls back the password update.
			consumed = false
			return nil
		}
		consumed = true
		// Invalidate every other still-pending reset for this user, so a
		// token issued before this successful reset cannot be replayed.
		if err := tx.Model(&database.PasswordReset{}).
			Where(util.QueryUserId, userID).
			Where("used_at IS NULL").
			Update("used_at", usedAt).Error; err != nil {
			return err
		}
		// Reset failed-login counter on successful password change.
		if err := tx.Model(&authentication.User{}).
			Where(util.QueryId, userID).
			Update("failed_login_attempts", 0).Error; err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		return false, txErr
	}
	return consumed, nil
}

func (r *UserRepository) DeleteExpiredPasswordResets(before time.Time) error {
	return r.DB.Where("expires_at < ?", before).Delete(&database.PasswordReset{}).Error
}

func (r *UserRepository) InvalidatePendingPasswordResetsForUser(userID uint) error {
	return r.DB.Model(&database.PasswordReset{}).
		Where(util.QueryUserId, userID).
		Where("used_at IS NULL").
		Update("used_at", time.Now()).Error
}

func (r *UserRepository) SetUserPasswordHash(userID uint, hashedPassword string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Update("password", hashedPassword).Error
}

// VerifyPassword checks a password against the stored bcrypt hash.
func (r *UserRepository) VerifyPassword(userID uint, password string) (bool, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return false, err
	}
	if user.Password == "" {
		return false, nil
	}
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil, nil
}

// EnableTwoFactor stores the encrypted TOTP secret and activates the second
// factor for the account.
//
// The update only lands while the account has no second factor yet. Two
// concurrent confirmations of the same enrolment would otherwise both activate
// and both hand out recovery codes, and the second one would delete the codes
// the first one already showed to the user.
func (r *UserRepository) EnableTwoFactor(userID uint, encryptedSecret string) (bool, error) {
	result := r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Where("two_factor_enabled = ?", false).
		Updates(map[string]any{
			"two_factor_secret":         encryptedSecret,
			"two_factor_enabled":        true,
			"two_factor_last_used_step": 0,
			"two_factor_pending_secret": "",
			"two_factor_pending_expiry": nil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// DisableTwoFactor removes the TOTP secret, deactivates the second factor and
// deletes its recovery codes in one transaction. The codes of a disabled
// second factor must not survive it: they would keep a way back in alive for
// an account that no longer has a second factor.
func (r *UserRepository) DisableTwoFactor(userID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&authentication.User{}).
			Where(util.QueryId, userID).
			Updates(map[string]any{
				"two_factor_secret":         "",
				"two_factor_enabled":        false,
				"two_factor_last_used_step": 0,
				"two_factor_pending_secret": "",
				"two_factor_pending_expiry": nil,
			}).Error; err != nil {
			return err
		}
		// A recovery code is a single-use credential, not a record: it is
		// removed for good, so no hash of an issued code outlives it.
		return tx.Unscoped().
			Where(util.QueryUserId, userID).
			Delete(&database.TwoFactorRecoveryCode{}).Error
	})
}

// SetTwoFactorPendingSecret stores a secret for an enrolment that has not
// been confirmed yet.
func (r *UserRepository) SetTwoFactorPendingSecret(userID uint, encryptedSecret string, expiry time.Time) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]any{
			"two_factor_pending_secret": encryptedSecret,
			"two_factor_pending_expiry": expiry,
		}).Error
}

// ConsumeRecoveryCodeAndDisable spends a recovery code and turns the second
// factor off in one transaction. A recovery that consumed the code but failed
// to disable the factor would cost the user a code for nothing.
func (r *UserRepository) ConsumeRecoveryCodeAndDisable(userID uint, codeHash string, usedAt time.Time) (bool, error) {
	var consumed bool

	txErr := r.DB.Transaction(func(tx *gorm.DB) error {
		// The update is the guard, not a leftover write: only one
		// transaction can match a code that is still unused, so a recovery
		// code cannot be spent twice by two concurrent sign-ins. The codes are
		// deleted further down, which is why the marker looks redundant.
		result := tx.Model(&database.TwoFactorRecoveryCode{}).
			Where(util.QueryUserId, userID).
			Where("code_hash = ?", codeHash).
			Where("used_at IS NULL").
			Update("used_at", usedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		consumed = true

		if err := tx.Model(&authentication.User{}).
			Where(util.QueryId, userID).
			Updates(map[string]any{
				"two_factor_secret":         "",
				"two_factor_enabled":        false,
				"two_factor_last_used_step": 0,
				"two_factor_pending_secret": "",
				"two_factor_pending_expiry": nil,
			}).Error; err != nil {
			return err
		}

		return tx.Unscoped().
			Where(util.QueryUserId, userID).
			Delete(&database.TwoFactorRecoveryCode{}).Error
	})
	if txErr != nil {
		return false, txErr
	}
	return consumed, nil
}

// AdvanceTwoFactorStep stores the time step of the last accepted code. The
// update only lands when the stored step is older, so a replayed code is
// rejected even when two requests race.
func (r *UserRepository) AdvanceTwoFactorStep(userID uint, step int64) (bool, error) {
	result := r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Where("two_factor_last_used_step < ?", step).
		Update("two_factor_last_used_step", step)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

var _ = (*UserRepository)(nil)
