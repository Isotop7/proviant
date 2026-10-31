package v1

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

func twoFactorHandlerConfig() *configuration.ProviantConfiguration {
	config := &configuration.ProviantConfiguration{}
	config.Server.TwoFactor = configuration.TwoFactorConfiguration{
		Enabled:           true,
		EncryptionKey:     base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Issuer:            "Proviant",
		RecoveryCodeCount: 10,
	}
	return config
}

// setupTwoFactorHandlerTest wires a gin context with two-factor authentication
// enabled and a user whose password is known.
func setupTwoFactorHandlerTest(t *testing.T) (*handlerTestEnv, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	user.Role = authentication.RoleAdmin
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	user.Password = string(hash)
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())

	appCtx := SetupTestAppContext(ctx, user.ID)
	return &handlerTestEnv{Ctx: ctx, W: w, AppCtx: appCtx, DB: db, User: user, Household: household}, "correct-password"
}

func TestGetTOTPStatusWithoutSecondFactor(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	GetTOTPStatus(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("GetTOTPStatus() = %d, want %d; body = %s", env.W.Code, http.StatusOK, env.W.Body.String())
	}

	var response struct {
		Enabled           bool `json:"enabled"`
		HouseholdRequired bool `json:"householdRequired"`
	}
	if err := json.Unmarshal(env.W.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response.Enabled {
		t.Error("enabled = true, want false")
	}
}

func TestGetTOTPStatusReportsThePendingState(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	GetTOTPStatus(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("GetTOTPStatus() = %d, want %d", env.W.Code, http.StatusOK)
	}

	var response struct {
		Enabled bool  `json:"enabled"`
		Pending bool  `json:"pending"`
		Codes   int64 `json:"recoveryCodesRemaining"`
	}
	if err := json.Unmarshal(env.W.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response.Enabled {
		t.Error("enabled = true, want false")
	}
	if response.Pending {
		t.Error("pending = true, want false")
	}
	if response.Codes != 0 {
		t.Errorf("recoveryCodesRemaining = %d, want 0", response.Codes)
	}
}

func TestGetTOTPStatusWithoutTwoFactorFeature(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	ctx, w := testutil.SetupGinContext(env.DB)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})

	GetTOTPStatus(ctx, SetupTestAppContext(ctx, env.User.ID))

	if w.Code != http.StatusNotFound {
		t.Errorf("GetTOTPStatus() = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// A missing configuration is a wiring mistake, not a disabled feature: it must
// not be reported to the user as "the feature is off".
func TestGetTOTPStatusWithoutConfiguration(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	ctx, w := testutil.SetupGinContext(env.DB)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))

	GetTOTPStatus(ctx, SetupTestAppContext(ctx, env.User.ID))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetTOTPStatus() = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTOTPEnrolmentFlow(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	StartTOTPEnrolment(env.Ctx, env.AppCtx)
	if env.W.Code != http.StatusOK {
		t.Fatalf("StartTOTPEnrolment() = %d, want %d; body = %s", env.W.Code, http.StatusOK, env.W.Body.String())
	}

	var enrol struct {
		QR string `json:"qr"`
	}
	if err := json.Unmarshal(env.W.Body.Bytes(), &enrol); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if enrol.QR == "" {
		t.Fatal("the enrolment carries no QR code")
	}

	var stored authentication.User
	if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.TwoFactorPendingSecret == "" {
		t.Fatal("the pending secret was not stored")
	}

	key, err := controllers.DecodeTwoFactorEncryptionKey(twoFactorHandlerConfig().Server.TwoFactor.EncryptionKey)
	if err != nil {
		t.Fatalf("DecodeTwoFactorEncryptionKey() error = %v", err)
	}
	secret, err := controllers.DecryptTwoFactorSecret(stored.TwoFactorPendingSecret, key)
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	confirmCtx, confirmWriter := testutil.SetupGinContext(env.DB)
	testutil.MockJWTClaims(confirmCtx, env.User.ID)
	confirmCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	confirmCtx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())
	testutil.CreateTestRequest(confirmCtx, map[string]string{"code": code})

	ConfirmTOTPEnrolment(confirmCtx, SetupTestAppContext(confirmCtx, env.User.ID))
	if confirmWriter.Code != http.StatusOK {
		t.Fatalf("ConfirmTOTPEnrolment() = %d, want %d; body = %s", confirmWriter.Code, http.StatusOK, confirmWriter.Body.String())
	}

	var confirm struct {
		TwoFactorEnabled bool     `json:"twoFactorEnabled"`
		RecoveryCodes    []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(confirmWriter.Body.Bytes(), &confirm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !confirm.TwoFactorEnabled {
		t.Error("twoFactorEnabled = false, want true")
	}
	if len(confirm.RecoveryCodes) != 10 {
		t.Errorf("len(recoveryCodes) = %d, want 10", len(confirm.RecoveryCodes))
	}

	var afterwards authentication.User
	if err := env.DB.First(&afterwards, env.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !afterwards.TwoFactorEnabled {
		t.Error("the second factor was not enabled")
	}
	if afterwards.TwoFactorPendingSecret != "" {
		t.Error("the pending secret was not cleared")
	}
}

// A rejected code answers with the reason and lands in the audit trail: it is
// the signal that somebody is trying to finish an enrolment they should not be
// able to finish.
func TestConfirmTOTPEnrolmentRejectsAWrongCode(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	StartTOTPEnrolment(env.Ctx, env.AppCtx)

	confirmCtx, confirmWriter := testutil.SetupGinContext(env.DB)
	testutil.MockJWTClaims(confirmCtx, env.User.ID)
	confirmCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	confirmCtx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())
	testutil.CreateTestRequest(confirmCtx, map[string]string{"code": "000000"})

	ConfirmTOTPEnrolment(confirmCtx, SetupTestAppContext(confirmCtx, env.User.ID))

	if confirmWriter.Code != http.StatusBadRequest {
		t.Errorf("ConfirmTOTPEnrolment() = %d, want %d", confirmWriter.Code, http.StatusBadRequest)
	}
	if !strings.Contains(confirmWriter.Body.String(), apperrors.ErrTwoFactorInvalidCode.Error()) {
		t.Errorf("the response says %q, want the invalid code error", confirmWriter.Body.String())
	}

	waitForAuditEntry(t, env.DB, env.User.ID, dbModel.AuditActionTwoFactorFailed)
}

// waitForAuditEntry waits for an audit entry, because the handlers write them
// in a goroutine.
func waitForAuditEntry(t *testing.T, db *gorm.DB, userID uint, action string) {
	t.Helper()

	for range 50 {
		var count int64
		if err := db.Model(&dbModel.AuditLog{}).
			Where("user_id = ? AND action = ?", userID, action).
			Count(&count).Error; err != nil {
			t.Fatalf("count the audit entries: %v", err)
		}
		if count > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Errorf("no audit entry with the action %q was written for user ID %d", action, userID)
}

func TestDisableTOTPRequiresThePassword(t *testing.T) {
	env, password := setupTwoFactorHandlerTest(t)

	if err := env.DB.Model(&authentication.User{}).
		Where("id = ?", env.User.ID).
		Updates(map[string]any{"two_factor_enabled": true, "two_factor_secret": "encrypted"}).Error; err != nil {
		t.Fatalf("enable two factor: %v", err)
	}

	wrongCtx, wrongWriter := testutil.SetupGinContext(env.DB)
	testutil.MockJWTClaims(wrongCtx, env.User.ID)
	wrongCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	wrongCtx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())
	testutil.CreateTestRequest(wrongCtx, map[string]string{"password": "wrong-password"})

	DisableTOTP(wrongCtx, SetupTestAppContext(wrongCtx, env.User.ID))
	if wrongWriter.Code != http.StatusUnauthorized {
		t.Errorf("DisableTOTP() with a wrong password = %d, want %d", wrongWriter.Code, http.StatusUnauthorized)
	}

	// The frontend branches on the code, not on the message: the message is
	// prose and may be reworded.
	var rejection struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(wrongWriter.Body.Bytes(), &rejection); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rejection.Reason != api.ReasonInvalidCredentials {
		t.Errorf("reason = %q, want %q", rejection.Reason, api.ReasonInvalidCredentials)
	}

	rightCtx, rightWriter := testutil.SetupGinContext(env.DB)
	testutil.MockJWTClaims(rightCtx, env.User.ID)
	rightCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	rightCtx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())
	testutil.CreateTestRequest(rightCtx, map[string]string{"password": password})

	DisableTOTP(rightCtx, SetupTestAppContext(rightCtx, env.User.ID))
	if rightWriter.Code != http.StatusOK {
		t.Fatalf("DisableTOTP() = %d, want %d; body = %s", rightWriter.Code, http.StatusOK, rightWriter.Body.String())
	}

	var stored authentication.User
	if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.TwoFactorEnabled {
		t.Error("the second factor was not disabled")
	}
	if stored.TwoFactorSecret != "" {
		t.Error("the secret was not cleared")
	}
}

// A server-side fault must not reach the client with its own message.
func TestStartTOTPEnrolmentWithABrokenEncryptionKey(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	brokenCtx, brokenWriter := testutil.SetupGinContext(env.DB)
	testutil.MockJWTClaims(brokenCtx, env.User.ID)
	brokenCtx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(env.DB, nil))
	brokenConfig := twoFactorHandlerConfig()
	brokenConfig.Server.TwoFactor.EncryptionKey = "not base64!"
	brokenCtx.Set(util.ContextKeyProviantConfig, brokenConfig)

	StartTOTPEnrolment(brokenCtx, SetupTestAppContext(brokenCtx, env.User.ID))

	if brokenWriter.Code != http.StatusInternalServerError {
		t.Errorf("StartTOTPEnrolment() = %d, want %d", brokenWriter.Code, http.StatusInternalServerError)
	}
	if strings.Contains(brokenWriter.Body.String(), "base64") {
		t.Errorf("the response leaks the server configuration: %q", brokenWriter.Body.String())
	}
}

// The response of the enrolment must never carry the TOTP secret.
func TestTOTPEnrolmentResponseCarriesNoSecret(t *testing.T) {
	env, _ := setupTwoFactorHandlerTest(t)

	StartTOTPEnrolment(env.Ctx, env.AppCtx)

	key, err := controllers.DecodeTwoFactorEncryptionKey(twoFactorHandlerConfig().Server.TwoFactor.EncryptionKey)
	if err != nil {
		t.Fatalf("DecodeTwoFactorEncryptionKey() error = %v", err)
	}
	var stored authentication.User
	if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	secret, err := controllers.DecryptTwoFactorSecret(stored.TwoFactorPendingSecret, key)
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}

	if body := env.W.Body.String(); strings.Contains(body, secret) {
		t.Error("the enrolment response carries the TOTP secret")
	}
}
