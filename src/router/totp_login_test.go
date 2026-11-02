package router

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
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/templates"
	"github.com/Isotop7/proviant/testutil"
)

// testRepositoryContainer builds a repository container for the test database.
func testRepositoryContainer(db *gorm.DB) *database.RepositoryContainer {
	logger := zerolog.Nop()
	return database.NewRepositoryContainer(db, &logger)
}

func mustDecodeKey(t *testing.T, encoded string) []byte {
	t.Helper()
	key, err := controllers.DecodeTwoFactorEncryptionKey(encoded)
	if err != nil {
		t.Fatalf("DecodeTwoFactorEncryptionKey() error = %v", err)
	}
	return key
}

// seedRecoveryCode stores a single recovery code for the user and returns its
// plain value.
func seedRecoveryCode(t *testing.T, db *gorm.DB, userID uint) string {
	t.Helper()

	raw := "RECOVERYX"
	hash := dbModel.HashTwoFactorRecoveryCode(raw)
	if err := database.NewTOTPRepository(db).CreateRecoveryCodes(userID, []string{hash}); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}
	return raw
}

// twoFactorTestEngine builds a router with two-factor authentication enabled
// and a household that does not demand it yet.
func twoFactorTestEngine(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	return twoFactorTestEngineWithFeature(t, db, true)
}

// twoFactorTestEngineWithFeature builds a router whose deployment switch is
// either on or off.
func twoFactorTestEngineWithFeature(t *testing.T, db *gorm.DB, featureEnabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.TokenPassword = "test-token-password"
	cfg.Server.Authentication.TokenLifetime = 1
	cfg.Server.Authentication.SkipEmailVerification = true
	cfg.Server.CORS.AllowAllOrigins = true
	cfg.Server.RateLimit.LoginPerMinute = 1000
	cfg.Server.RateLimit.SignupPerMinute = 1000
	cfg.Server.RateLimit.ExportPerMinute = 1000
	cfg.Server.RateLimit.PasswordPerMinute = 1000
	cfg.Server.RateLimit.ScanPerMinute = 1000
	cfg.Server.RateLimit.RecipesPerMinute = 1000
	cfg.Server.TwoFactor = configuration.TwoFactorConfiguration{
		Enabled:           featureEnabled,
		EncryptionKey:     base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Issuer:            "Proviant",
		RecoveryCodeCount: 10,
	}

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	cfg.TemplateCache = templateCache

	logger := zerolog.Nop()
	return SetupRouter(&logger, cfg, db, nil, nil, nil)
}

// twoFactorTestUser creates a user with a known password.
func twoFactorTestUser(t *testing.T, db *gorm.DB, username string) *authentication.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}

	user := &authentication.User{
		Username:    username,
		Password:    string(hash),
		MailAddress: username + "@example.com",
		Role:        authentication.RoleAdmin,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	return user
}

// loginRequest posts to the login route and returns the recorder.
func loginRequest(engine *gin.Engine, username, password, code string) *httptest.ResponseRecorder {
	payload := map[string]string{"username": username, "password": password}
	if code != "" {
		payload["code"] = code
	}
	body, _ := json.Marshal(payload)

	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	// The browser carries the CSRF token in a cookie and repeats it in the
	// header; the middleware compares both.
	request.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf-token"})
	request.Header.Set("X-CSRF-Token", "test-csrf-token")

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

// enableTwoFactor enrols the user and returns the plain TOTP secret.
func enableTwoFactor(t *testing.T, db *gorm.DB, user *authentication.User) string {
	t.Helper()

	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	service := controllers.NewTwoFactorService(
		testRepositoryContainer(db),
		&configuration.TwoFactorConfiguration{Enabled: true, EncryptionKey: key, Issuer: "Proviant", RecoveryCodeCount: 10},
	)

	if _, _, err := service.StartEnrolment(user); err != nil {
		t.Fatalf("StartEnrolment() error = %v", err)
	}
	stored, err := testRepositoryContainer(db).Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	secret, err := controllers.DecryptTwoFactorSecret(stored.TwoFactorPendingSecret, mustDecodeKey(t, key))
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}
	if _, err := service.ConfirmEnrolment(&stored, code); err != nil {
		t.Fatalf("ConfirmEnrolment() error = %v", err)
	}
	return secret
}

func TestLoginWithoutSecondFactorNeedsNoCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")

	recorder := loginRequest(engine, user.Username, "correct-password", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestLoginWithSecondFactorRequiresACode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	enableTwoFactor(t, db, user)

	recorder := loginRequest(engine, user.Username, "correct-password", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["reason"] != api.ReasonTwoFactorRequired {
		t.Errorf("reason = %q, want %q", response["reason"], api.ReasonTwoFactorRequired)
	}
}

func TestLoginWithValidSecondFactorCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	secret := enableTwoFactor(t, db, user)

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	recorder := loginRequest(engine, user.Username, "correct-password", code)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestLoginWithWrongSecondFactorCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	enableTwoFactor(t, db, user)

	recorder := loginRequest(engine, user.Username, "correct-password", "000000")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["reason"] == api.ReasonTwoFactorRequired {
		t.Error("a wrong code was reported as a missing code")
	}
}

func TestLoginWithReplayedSecondFactorCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	secret := enableTwoFactor(t, db, user)

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	if recorder := loginRequest(engine, user.Username, "correct-password", code); recorder.Code != http.StatusOK {
		t.Fatalf("the first sign-in = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	recorder := loginRequest(engine, user.Username, "correct-password", code)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("the replayed sign-in = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

func TestLoginWithRecoveryCodeDisablesTheSecondFactor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	enableTwoFactor(t, db, user)

	repos := testRepositoryContainer(db)
	recoveryCode := seedRecoveryCode(t, db, user.ID)

	recorder := loginRequest(engine, user.Username, "correct-password", recoveryCode)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	stored, err := repos.Users.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if stored.TwoFactorEnabled {
		t.Error("the second factor was not disabled by the recovery code")
	}

	// The recovery turned the second factor off, so the sign-in needs no code
	// any more and the remaining recovery codes are gone.
	withoutCode := loginRequest(engine, user.Username, "correct-password", "")
	if withoutCode.Code != http.StatusOK {
		t.Errorf("the sign-in without a code = %d, want %d; body = %s", withoutCode.Code, http.StatusOK, withoutCode.Body.String())
	}

	remaining, err := repos.TOTPs.CountUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes() error = %v", err)
	}
	if remaining != 0 {
		t.Errorf("remaining recovery codes = %d, want 0", remaining)
	}
}

func TestLoginOfMemberOfHouseholdThatRequiresTwoFactor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")

	if err := db.Model(&dbModel.Household{}).
		Where("id = ?", user.HouseholdID).
		Update("require_two_factor", true).Error; err != nil {
		t.Fatalf("update household: %v", err)
	}

	recorder := loginRequest(engine, user.Username, "correct-password", "")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("POST /auth/login = %d, want %d; body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["reason"] != api.ReasonTwoFactorSetupRequired {
		t.Errorf("reason = %q, want %q", response["reason"], api.ReasonTwoFactorSetupRequired)
	}
	if !strings.HasPrefix(response["qr"], "data:image/png;base64,") {
		t.Error("the response carries no QR code")
	}
}

func TestLoginResponseCarriesNoSecret(t *testing.T) {
	db := testutil.SetupTestDB(t)
	engine := twoFactorTestEngine(t, db)
	user := twoFactorTestUser(t, db, "alice")
	secret := enableTwoFactor(t, db, user)

	recorder := loginRequest(engine, user.Username, "correct-password", "")
	if strings.Contains(recorder.Body.String(), secret) {
		t.Error("the sign-in response carries the TOTP secret")
	}
}

// Switching the deployment switch off must never quietly drop the second
// factor of an account that already enrolled one.
func TestLoginKeepsTheSecondFactorWhenTheDeploymentSwitchIsOff(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := twoFactorTestUser(t, db, "alice")
	secret := enableTwoFactor(t, db, user)

	offEngine := twoFactorTestEngineWithFeature(t, db, false)

	recorder := loginRequest(offEngine, user.Username, "correct-password", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("the sign-in without a code = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	recorder = loginRequest(offEngine, user.Username, "correct-password", code)
	if recorder.Code != http.StatusOK {
		t.Fatalf("the sign-in with a code = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
