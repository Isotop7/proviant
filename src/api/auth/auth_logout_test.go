package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func setupLogoutContext(t *testing.T, claims jwt.MapClaims) (*gin.Context, *httptest.ResponseRecorder, *gorm.DB) {
	t.Helper()

	db := testutil.SetupTestDB(t)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set(util.ContextKeyLogger, &mockLogger)
	ctx.Set(util.ContextKeyDBHandle, db)
	if claims != nil {
		ctx.Set("JWT_PAYLOAD", claims)
	}

	return ctx, w, db
}

func TestLogoutWithoutLogger(t *testing.T) {
	db := testutil.SetupTestDB(t)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set(util.ContextKeyDBHandle, db)

	Logout(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != api.ResponseErrLoggerContextNotFound.Message {
		t.Errorf("message = %v, want %v", response["message"], api.ResponseErrLoggerContextNotFound.Message)
	}
}

func TestLogoutWithoutDatabase(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set(util.ContextKeyLogger, &mockLogger)
	ctx.Set(util.ContextKeyDBHandle, "not-a-db-handle")

	Logout(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != api.ResponseErrDatabaseContextNotFound.Message {
		t.Errorf("message = %v, want %v", response["message"], api.ResponseErrDatabaseContextNotFound.Message)
	}
}

func TestLogoutInvalidClaims(t *testing.T) {
	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantMsg string
	}{
		{name: "no claims", claims: nil, wantMsg: "invalid token: no JTI"},
		{
			name:    "jti not string",
			claims:  jwt.MapClaims{"jti": 123, "exp": float64(time.Now().Unix())},
			wantMsg: "invalid token: JTI not string",
		},
		{name: "missing exp", claims: jwt.MapClaims{"jti": "abc"}, wantMsg: "invalid token: no expiry"},
		{
			name:    "exp not number",
			claims:  jwt.MapClaims{"jti": "abc", "exp": "soon"},
			wantMsg: "invalid token: expiry not number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, w, _ := setupLogoutContext(t, tt.claims)

			Logout(ctx)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Status = %v, want %v", w.Code, http.StatusUnauthorized)
			}
			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if response["message"] != tt.wantMsg {
				t.Errorf("message = %v, want %v", response["message"], tt.wantMsg)
			}
		})
	}
}

func TestLogoutSuccess(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	ctx, w, db := setupLogoutContext(t, jwt.MapClaims{
		"jti": "test-jti-123",
		"exp": float64(exp),
	})

	Logout(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != "Logged out successfully" {
		t.Errorf("message = %v, want %v", response["message"], "Logged out successfully")
	}

	var revoked authentication.RevokedToken
	if err := db.Where("jti = ?", "test-jti-123").First(&revoked).Error; err != nil {
		t.Fatalf("revoked token not stored: %v", err)
	}
	if revoked.ExpiresAt.Unix() != exp {
		t.Errorf("ExpiresAt = %v, want %v", revoked.ExpiresAt.Unix(), exp)
	}

	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "jwt=") {
		t.Errorf("Set-Cookie = %q, want jwt cookie", cookie)
	}
	if !strings.Contains(cookie, "Max-Age=0") {
		t.Errorf("Set-Cookie = %q, want expired cookie (Max-Age=0)", cookie)
	}
}

func TestLogoutDatabaseError(t *testing.T) {
	ctx, w, db := setupLogoutContext(t, jwt.MapClaims{
		"jti": "test-jti-err",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})
	if err := db.Exec("DROP TABLE revoked_tokens").Error; err != nil {
		t.Fatalf("drop table error: %v", err)
	}

	Logout(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != "failed to logout" {
		t.Errorf("message = %v, want %v", response["message"], "failed to logout")
	}
}

func TestSignupWithInviteTokenAccepted(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	// Config-driven password validator with the breached check off, so the
	// signup makes no live HTTPS call to api.pwnedpasswords.com.
	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.PasswordMinLength = 12
	cfg.Server.Authentication.PasswordRequireUppercase = true
	cfg.Server.Authentication.PasswordRequireDigit = true
	cfg.Server.Authentication.PasswordCheckBreached = false
	ctx.Set(util.ContextKeyProviantConfig, cfg)

	signup := authentication.Signup{
		Username:    "inviteduser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "invited@example.com",
		InviteToken: "test-invite-token",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	Signup(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != UserWasCreated {
		t.Errorf("message = %v, want %v", response["message"], UserWasCreated)
	}
}

func TestSignupWithInviteTokenAcceptanceError(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Invitations.Err = errors.New("accept failed")
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	// Config-driven password validator with the breached check off, so the
	// signup makes no live HTTPS call to api.pwnedpasswords.com.
	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.PasswordMinLength = 12
	cfg.Server.Authentication.PasswordRequireUppercase = true
	cfg.Server.Authentication.PasswordRequireDigit = true
	cfg.Server.Authentication.PasswordCheckBreached = false
	ctx.Set(util.ContextKeyProviantConfig, cfg)

	signup := authentication.Signup{
		Username:    "inviteduser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "invited@example.com",
		InviteToken: "test-invite-token",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	Signup(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != UserWasCreated {
		t.Errorf("message = %v, want %v", response["message"], UserWasCreated)
	}
}

func TestSignupSkipEmailVerification(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.SkipEmailVerification = true
	cfg.Server.Authentication.PasswordMinLength = 12
	cfg.Server.Authentication.PasswordRequireUppercase = true
	cfg.Server.Authentication.PasswordRequireDigit = true
	ctx.Set(util.ContextKeyProviantConfig, cfg)

	signup := authentication.Signup{
		Username:    "skipverifyuser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "skipverify@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	Signup(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response["message"] != UserWasCreated {
		t.Errorf("message = %v, want %v", response["message"], UserWasCreated)
	}
}
