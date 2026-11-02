package controllers

import (
	"errors"
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/pquerna/otp/totp"

	"gorm.io/gorm"
)

func twoFactorTestConfig() *configuration.TwoFactorConfiguration {
	return &configuration.TwoFactorConfiguration{
		Enabled:           true,
		EncryptionKey:     "cHJvdmlhbnQtZGVmYXVsdC10b3RwLWtleS0wMDAwMDA=",
		Issuer:            "Proviant",
		RecoveryCodeCount: 10,
	}
}

func twoFactorTestKey() []byte {
	key, err := DecodeTwoFactorEncryptionKey(twoFactorTestConfig().EncryptionKey)
	if err != nil {
		panic(err)
	}
	return key
}

func twoFactorTestService(t *testing.T, db *gorm.DB) *TwoFactorService {
	t.Helper()
	return NewTwoFactorService(database.NewRepositoryContainer(db, nil), twoFactorTestConfig())
}

func twoFactorTestUser(t *testing.T, db *gorm.DB) authentication.User {
	t.Helper()
	user := testutil.CreateTestUser(db, 0)
	return *user
}

// enrolTwoFactorUser runs a full enrolment and returns the user together with
// the generated recovery codes.
func enrolTwoFactorUser(t *testing.T, service *TwoFactorService, user authentication.User) (authentication.User, []string) {
	t.Helper()

	qr, _, err := service.StartEnrolment(&user)
	if err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	if qr == "" {
		t.Fatal("StartEnrolment() returned an empty QR code")
	}

	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if stored.TwoFactorPendingSecret == "" {
		t.Fatal("the pending secret was not stored")
	}

	secret, err := DecryptTwoFactorSecret(stored.TwoFactorPendingSecret, twoFactorTestKey())
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	codes, err := service.ConfirmEnrolment(&stored, code)
	if err != nil {
		t.Fatalf("ConfirmEnrolment() error = %v", err)
	}

	enrolled, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	return enrolled, codes
}

func TestTwoFactorServiceStartEnrolmentStoresPendingSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	qr, expiry, err := service.StartEnrolment(&user)
	if err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	if qr == "" {
		t.Error("StartEnrolment() returned an empty QR code")
	}
	if !expiry.After(time.Now()) {
		t.Errorf("expiry = %v, want a point in time in the future", expiry)
	}

	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if stored.TwoFactorPendingSecret == "" {
		t.Error("the pending secret was not stored")
	}
	if stored.TwoFactorEnabled {
		t.Error("the second factor was enabled before the enrolment was confirmed")
	}
}

// The rejection classification is one list: an error the user can act on and
// the class the log carries must never disagree.
func TestTwoFactorRejectionClassification(t *testing.T) {
	actionable := []error{
		apperrors.ErrTwoFactorInvalidCode,
		apperrors.ErrTwoFactorPendingExpired,
		apperrors.ErrTwoFactorAlreadyEnabled,
		apperrors.ErrTwoFactorNotEnabled,
	}

	for _, err := range actionable {
		if !IsTwoFactorRejection(err) {
			t.Errorf("IsTwoFactorRejection(%v) = false, want true", err)
		}
		if TwoFactorRejectionReason(err) == "internal_error" {
			t.Errorf("TwoFactorRejectionReason(%v) = internal_error, want a class of its own", err)
		}
	}

	if IsTwoFactorRejection(errors.New("a driver error")) {
		t.Error("IsTwoFactorRejection(driver error) = true, want false")
	}
	if TwoFactorRejectionReason(errors.New("a driver error")) != "internal_error" {
		t.Error("TwoFactorRejectionReason(driver error) is not internal_error")
	}
}

func TestTwoFactorServiceStartEnrolmentKeepsAValidPendingSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	firstQR, _, err := service.StartEnrolment(&user)
	if err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}

	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	pendingSecret := stored.TwoFactorPendingSecret

	// A second enrolment must hand out the same QR code: the user may
	// already have scanned the first one.
	secondQR, _, err := service.StartEnrolment(&stored)
	if err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	if secondQR != firstQR {
		t.Error("the second enrolment replaced the QR code of the pending one")
	}

	afterwards, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if afterwards.TwoFactorPendingSecret != pendingSecret {
		t.Error("the pending secret was replaced")
	}
}

func TestTwoFactorServiceStartEnrolmentReplacesAnExpiredPendingSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	if _, _, err := service.StartEnrolment(&user); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}

	expired := time.Now().Add(-time.Minute)
	stored.TwoFactorPendingExpiry = &expired
	if err := service.repos.Users.SetTwoFactorPendingSecret(stored.ID, stored.TwoFactorPendingSecret, expired); err != nil {
		t.Fatalf("SetTwoFactorPendingSecret() error = %v", err)
	}

	if _, _, err := service.StartEnrolment(&stored); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}

	afterwards, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !afterwards.TwoFactorPendingExpiry.After(time.Now()) {
		t.Error("the expired pending secret was not replaced")
	}
}

func TestTwoFactorServiceConfirmEnrolmentEnablesAndIssuesRecoveryCodes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, codes := enrolTwoFactorUser(t, service, user)

	if !enrolled.TwoFactorEnabled {
		t.Error("the second factor was not enabled")
	}
	if enrolled.TwoFactorSecret == "" {
		t.Error("the secret was not stored")
	}
	if enrolled.TwoFactorPendingSecret != "" {
		t.Error("the pending secret was not cleared")
	}
	if len(codes) != 10 {
		t.Fatalf("len(codes) = %d, want 10", len(codes))
	}

	storedCodes, err := service.repos.TOTPs.GetRecoveryCodesByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetRecoveryCodesByUserID() error = %v", err)
	}
	if len(storedCodes) != 10 {
		t.Fatalf("len(storedCodes) = %d, want 10", len(storedCodes))
	}
	for _, stored := range storedCodes {
		for _, raw := range codes {
			if stored.CodeHash == raw {
				t.Errorf("the recovery code %q was stored in the clear", raw)
			}
		}
	}
}

func TestTwoFactorServiceConfirmEnrolmentRejectsWrongCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	if _, _, err := service.StartEnrolment(&user); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}

	if _, err := service.ConfirmEnrolment(&stored, "000000"); err != apperrors.ErrTwoFactorInvalidCode {
		t.Errorf("error = %v, want %v", err, apperrors.ErrTwoFactorInvalidCode)
	}
}

func TestTwoFactorServiceConfirmEnrolmentRejectsExpiredPendingSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	if _, _, err := service.StartEnrolment(&user); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	expired := time.Now().Add(-time.Minute)
	stored.TwoFactorPendingExpiry = &expired

	if _, err := service.ConfirmEnrolment(&stored, "123456"); err != apperrors.ErrTwoFactorPendingExpired {
		t.Errorf("error = %v, want %v", err, apperrors.ErrTwoFactorPendingExpired)
	}
}

func TestTwoFactorServiceConfirmEnrolmentOnlyActivatesOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	if _, _, err := service.StartEnrolment(&user); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	stored, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	secret, err := DecryptTwoFactorSecret(stored.TwoFactorPendingSecret, twoFactorTestKey())
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	first, err := service.ConfirmEnrolment(&stored, code)
	if err != nil {
		t.Fatalf("ConfirmEnrolment() error = %v", err)
	}
	if len(first) == 0 {
		t.Fatal("the first confirmation issued no recovery codes")
	}

	// A second confirmation of the same enrolment must not activate again:
	// the codes of the first one have to stay valid.
	storedAgain, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	storedAgain.TwoFactorEnabled = false
	storedAgain.TwoFactorPendingSecret = stored.TwoFactorPendingSecret
	storedAgain.TwoFactorPendingExpiry = stored.TwoFactorPendingExpiry

	if _, err := service.ConfirmEnrolment(&storedAgain, code); err != apperrors.ErrTwoFactorAlreadyEnabled {
		t.Errorf("the second confirmation returned %v, want %v", err, apperrors.ErrTwoFactorAlreadyEnabled)
	}

	remaining, err := service.repos.TOTPs.CountUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes() error = %v", err)
	}
	if remaining != int64(len(first)) {
		t.Errorf("remaining recovery codes = %d, want %d", remaining, len(first))
	}
}

func TestTwoFactorServiceVerifyCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, _ := enrolTwoFactorUser(t, service, user)
	secret, err := DecryptTwoFactorSecret(enrolled.TwoFactorSecret, twoFactorTestKey())
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	valid, err := service.VerifyCode(&enrolled, code)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if !valid {
		t.Error("VerifyCode() rejected a fresh code")
	}
}

func TestTwoFactorServiceVerifyCodeRejectsReplay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, _ := enrolTwoFactorUser(t, service, user)
	secret, err := DecryptTwoFactorSecret(enrolled.TwoFactorSecret, twoFactorTestKey())
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	valid, err := service.VerifyCode(&enrolled, code)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if !valid {
		t.Fatal("VerifyCode() rejected a fresh code")
	}

	replayed, err := service.VerifyCode(&enrolled, code)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if replayed {
		t.Error("VerifyCode() accepted a replayed code")
	}
}

func TestTwoFactorServiceConsumeRecoveryCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, codes := enrolTwoFactorUser(t, service, user)

	consumed, err := service.ConsumeRecoveryCode(&enrolled, codes[0])
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if !consumed {
		t.Fatal("ConsumeRecoveryCode() did not consume the code")
	}

	afterwards, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if afterwards.TwoFactorEnabled {
		t.Error("the second factor was not disabled by the recovery code")
	}
	if afterwards.TwoFactorSecret != "" {
		t.Error("the secret was not cleared by the recovery code")
	}
}

func TestTwoFactorServiceConsumeRecoveryCodeWorksOnlyOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, codes := enrolTwoFactorUser(t, service, user)

	consumed, err := service.ConsumeRecoveryCode(&enrolled, codes[0])
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if !consumed {
		t.Fatal("ConsumeRecoveryCode() did not consume the code")
	}

	consumed, err = service.ConsumeRecoveryCode(&enrolled, codes[0])
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if consumed {
		t.Error("ConsumeRecoveryCode() consumed the same code twice")
	}
}

func TestTwoFactorServiceDisable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	enrolled, _ := enrolTwoFactorUser(t, service, user)

	if err := service.Disable(&enrolled); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	afterwards, err := service.repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if afterwards.TwoFactorEnabled {
		t.Error("the second factor is still enabled")
	}

	remaining, err := service.repos.TOTPs.CountUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes() error = %v", err)
	}
	if remaining != 0 {
		t.Errorf("remaining recovery codes = %d, want 0", remaining)
	}
}

func TestTwoFactorServiceDisableWithoutSecondFactor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)
	user := twoFactorTestUser(t, db)

	if err := service.Disable(&user); err != apperrors.ErrTwoFactorNotEnabled {
		t.Errorf("error = %v, want %v", err, apperrors.ErrTwoFactorNotEnabled)
	}
}

func TestTwoFactorServiceHouseholdRequiresTwoFactor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	service := twoFactorTestService(t, db)

	user := twoFactorTestUser(t, db)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	required, err := service.HouseholdRequiresTwoFactor(household.ID)
	if err != nil {
		t.Fatalf("HouseholdRequiresTwoFactor() error = %v", err)
	}
	if required {
		t.Error("the household requires a second factor although the policy is off")
	}

	if err := db.Model(&dbModel.Household{}).Where("id = ?", household.ID).Update("require_two_factor", true).Error; err != nil {
		t.Fatalf("update household: %v", err)
	}

	required, err = service.HouseholdRequiresTwoFactor(household.ID)
	if err != nil {
		t.Fatalf("HouseholdRequiresTwoFactor() error = %v", err)
	}
	if !required {
		t.Error("the household does not require a second factor although the policy is on")
	}
}
