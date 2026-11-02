package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

func twoFactorAuthConfig() *configuration.ProviantConfiguration {
	config := &configuration.ProviantConfiguration{}
	config.Server.Authentication.MaxLoginAttempts = 3
	config.Server.Authentication.LockoutDurationMins = 10
	config.Server.TwoFactor = configuration.TwoFactorConfiguration{
		Enabled:           true,
		EncryptionKey:     base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Issuer:            "Proviant",
		RecoveryCodeCount: 10,
	}
	return config
}

// setupTwoFactorAuthTest wires a gin context with two-factor authentication
// enabled for a member of a household that demands it.
func setupTwoFactorAuthTest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *authentication.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	user.Password = string(hash)
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	if err := db.Model(&dbModel.Household{}).
		Where("id = ?", household.ID).
		Update("require_two_factor", true).Error; err != nil {
		t.Fatalf("update household: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, twoFactorAuthConfig())
	return ctx, w, user
}

// seedPendingSecret stores a pending enrolment and returns the plain secret.
func seedPendingSecret(t *testing.T, db *gorm.DB, user *authentication.User) string {
	t.Helper()
	return seedPendingSecretWithExpiry(t, db, user, time.Now().Add(10*time.Minute))
}

func seedPendingSecretWithExpiry(t *testing.T, db *gorm.DB, user *authentication.User, expiry time.Time) string {
	t.Helper()

	key, err := controllers.DecodeTwoFactorEncryptionKey(twoFactorAuthConfig().Server.TwoFactor.EncryptionKey)
	if err != nil {
		t.Fatalf("DecodeTwoFactorEncryptionKey() error = %v", err)
	}

	secret, uri, err := controllers.GenerateTwoFactorSecret("Proviant", user.Username)
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}
	encrypted, err := controllers.EncryptTwoFactorSecret(secret, key)
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}

	if err := database.NewUserRepository(db).SetTwoFactorPendingSecret(user.ID, encrypted, expiry); err != nil {
		t.Fatalf("SetTwoFactorPendingSecret() error = %v", err)
	}
	_ = uri
	return secret
}

// confirmTwoFactorSetup calls the handler on a fresh context. A reused gin
// context keeps the status of its first write, so every call needs its own
// recorder to be able to tell the answers apart.
func confirmTwoFactorSetup(t *testing.T, db *gorm.DB, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	ctx, w := testutil.SetupGinContext(db)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, twoFactorAuthConfig())
	testutil.CreateTestRequest(ctx, body)

	ConfirmTwoFactorSetup(ctx)
	return w
}

func TestConfirmTwoFactorSetupActivatesTheSecondFactor(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	secret := seedPendingSecret(t, ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), user)

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     code,
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("ConfirmTwoFactorSetup() = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response struct {
		TwoFactorEnabled bool     `json:"twoFactorEnabled"`
		RecoveryCodes    []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !response.TwoFactorEnabled {
		t.Error("twoFactorEnabled = false, want true")
	}
	if len(response.RecoveryCodes) != 10 {
		t.Errorf("len(recoveryCodes) = %d, want 10", len(response.RecoveryCodes))
	}

	var stored authentication.User
	if err := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB).First(&stored, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !stored.TwoFactorEnabled {
		t.Error("the second factor was not enabled")
	}
}

func TestConfirmTwoFactorSetupRejectsAWrongPassword(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	secret := seedPendingSecret(t, ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), user)

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "wrong-password",
		"code":     code,
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestConfirmTwoFactorSetupRejectsAWrongCode(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	seedPendingSecret(t, ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), user)

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     "000000",
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// A wrong code must answer exactly like a wrong password, so the endpoint
// cannot be used to confirm that a password guess was right.
func TestConfirmTwoFactorSetupDoesNotRevealAWrongPassword(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	seedPendingSecret(t, db, user)
	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "wrong-password",
		"code":     "000000",
	})
	ConfirmTwoFactorSetup(ctx)
	wrongPasswordStatus := w.Code
	wrongPasswordBody := w.Body.String()

	otherCtx, otherWriter := testutil.SetupGinContext(db)
	otherCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	otherCtx.Set(util.ContextKeyProviantConfig, twoFactorAuthConfig())
	testutil.CreateTestRequest(otherCtx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     "000000",
	})

	ConfirmTwoFactorSetup(otherCtx)

	if otherWriter.Code != wrongPasswordStatus {
		t.Errorf("a wrong code = %d and a wrong password = %d, want the same status", otherWriter.Code, wrongPasswordStatus)
	}
	if otherWriter.Body.String() != wrongPasswordBody {
		t.Errorf("a wrong code answered %q and a wrong password %q, want the same body", otherWriter.Body.String(), wrongPasswordBody)
	}
}

// A wrong password on this endpoint counts against the account lockout, so
// the endpoint is not a way around it.
func TestConfirmTwoFactorSetupCountsAWrongPasswordAgainstTheLockout(t *testing.T) {
	ctx, _, user := setupTwoFactorAuthTest(t)
	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	seedPendingSecret(t, db, user)

	for range 3 {
		recorder := confirmTwoFactorSetup(t, db, map[string]string{
			"username": user.Username,
			"password": "wrong-password",
			"code":     "000000",
		})
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("ConfirmTwoFactorSetup() = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	}

	var stored authentication.User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.FailedLoginAttempts == 0 {
		t.Error("the wrong password was not counted against the account lockout")
	}
	if !stored.LockedUntil.Valid {
		t.Error("the account was not locked after the wrong passwords")
	}
}

func TestConfirmTwoFactorSetupNeedsTheHouseholdPolicy(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	secret := seedPendingSecret(t, ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), user)

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if err := db.Model(&dbModel.Household{}).
		Where("id = ?", user.HouseholdID).
		Update("require_two_factor", false).Error; err != nil {
		t.Fatalf("update household: %v", err)
	}

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     code,
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), apperrors.ErrTwoFactorNotRequired.Error()) {
		t.Errorf("the response says %q, want the household policy error", w.Body.String())
	}
}

// The household policy is answered before the password, so the answer never
// depends on whether the password was right.
func TestConfirmTwoFactorSetupDoesNotRevealThePasswordThroughThePolicy(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	seedPendingSecret(t, db, user)

	if err := db.Model(&dbModel.Household{}).
		Where("id = ?", user.HouseholdID).
		Update("require_two_factor", false).Error; err != nil {
		t.Fatalf("update household: %v", err)
	}

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "wrong-password",
		"code":     "000000",
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ConfirmTwoFactorSetup() with a wrong password = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// A locked account answers like the sign-in route does, whatever the state of
// its enrolment.
func TestConfirmTwoFactorSetupReportsALockedAccount(t *testing.T) {
	ctx, _, user := setupTwoFactorAuthTest(t)
	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	seedPendingSecret(t, db, user)

	for range 3 {
		testutil.CreateTestRequest(ctx, map[string]string{
			"username": user.Username,
			"password": "wrong-password",
			"code":     "000000",
		})
		ConfirmTwoFactorSetup(ctx)
	}

	recorder := confirmTwoFactorSetup(t, db, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     "000000",
	})

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Error("the locked answer carries no Retry-After header")
	}
}

func TestConfirmTwoFactorSetupRejectsAnUnknownUser(t *testing.T) {
	ctx, w, _ := setupTwoFactorAuthTest(t)

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": "nobody",
		"password": "correct-password",
		"code":     "123456",
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if strings.Contains(w.Body.String(), "nobody") {
		t.Error("the response echoes the unknown username")
	}
}

// The endpoint must not confirm that a username exists: an account that never
// started an enrolment answers exactly like an unknown one.
func TestConfirmTwoFactorSetupDoesNotEnumerateUsers(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)

	// The account exists but never started an enrolment.
	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     "123456",
	})
	ConfirmTwoFactorSetup(ctx)
	knownUserStatus := w.Code
	knownUserBody := w.Body.String()

	otherCtx, otherWriter, _ := setupTwoFactorAuthTest(t)
	testutil.CreateTestRequest(otherCtx, map[string]string{
		"username": "nobody",
		"password": "correct-password",
		"code":     "123456",
	})
	ConfirmTwoFactorSetup(otherCtx)

	if otherWriter.Code != knownUserStatus {
		t.Errorf("a known account = %d and an unknown one = %d, want the same status", knownUserStatus, otherWriter.Code)
	}
	if otherWriter.Body.String() != knownUserBody {
		t.Errorf("a known account answered %q and an unknown one %q, want the same body", knownUserBody, otherWriter.Body.String())
	}
}

// An enrolment that timed out answers with the setup error, so the user knows
// to sign in again for a fresh QR code.
func TestConfirmTwoFactorSetupReportsAnExpiredEnrolment(t *testing.T) {
	ctx, w, user := setupTwoFactorAuthTest(t)
	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)

	seedPendingSecretWithExpiry(t, db, user, time.Now().Add(-time.Minute))

	testutil.CreateTestRequest(ctx, map[string]string{
		"username": user.Username,
		"password": "correct-password",
		"code":     "123456",
	})

	ConfirmTwoFactorSetup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ConfirmTwoFactorSetup() = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), apperrors.ErrTwoFactorSetupExpired.Error()) {
		t.Errorf("the response says %q, want the expired setup error", w.Body.String())
	}
}
