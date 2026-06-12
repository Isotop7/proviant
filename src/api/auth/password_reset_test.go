package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
)

func TestForgotPassword_MissingEmail(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestForgotPassword_EmptyEmail(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"mailAddress": "   "})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestForgotPassword_NoUser_StillReturnsGeneric(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.Err = gorm.ErrRecordNotFound
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"mailAddress": "nobody@example.com"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v (no user-enumeration)", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["message"] != forgotPasswordResponseGeneric {
		t.Errorf("message = %q, want %q", response["message"], forgotPasswordResponseGeneric)
	}
}

func TestForgotPassword_UserExists_NoNotificationController_StillReturnsGeneric(t *testing.T) {
	// When no notification controller is on the context, the endpoint must still
	// respond identically so the existence of the user is not leaked.
	m := repomocks.NewMockRepositoryContainer()
	// No user Err set — GetUserByMailAddress will return m.User (zero value) without an error
	// which is treated as "found".
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"mailAddress": "test@example.com"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v (no user-enumeration)", w.Code, http.StatusOK)
	}
}

func TestForgotPassword_DBError_StillReturnsGeneric(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.Err = gorm.ErrInvalidDB
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"mailAddress": "test@example.com"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v (no user-enumeration)", w.Code, http.StatusOK)
	}
}

func TestForgotPassword_UnverifiedEmail_StillReturnsGeneric(t *testing.T) {
	// A user exists but has not verified their email. The endpoint must
	// skip the reset path AND return the same generic 200 response to
	// avoid leaking that the email is registered but unverified.
	m := repomocks.NewMockRepositoryContainer()
	m.Users.User = authentication.User{
		MailAddress:     "test@example.com",
		EmailVerifiedAt: nil,
	}
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"mailAddress": "test@example.com"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ForgotPassword(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v (no user-enumeration)", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["message"] != forgotPasswordResponseGeneric {
		t.Errorf("message = %q, want %q", response["message"], forgotPasswordResponseGeneric)
	}
}

func TestResetPassword_MissingToken(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"password": "NewValidPassword123!"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestResetPassword_WeakPassword(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.PasswordReset = dbModel.PasswordReset{
		Model:     gorm.Model{ID: 1},
		UserID:    42,
		TokenHash: "valid-hash",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"token": "abc", "password": "short"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestResetPassword_UnknownToken(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.Err = gorm.ErrRecordNotFound
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"token": "no-such-token", "password": "NewValidPassword123!"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestResetPassword_AlreadyUsed(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	usedAt := time.Now()
	m.Users.PasswordReset = dbModel.PasswordReset{
		UserID:    7,
		TokenHash: "used-hash",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    &usedAt,
	}
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"token": "used-token", "password": "NewValidPassword123!"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response api.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response.Message != "password reset link has already been used" {
		t.Errorf("message = %q, want already-used message", response.Message)
	}
}

func TestResetPassword_Expired(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.PasswordReset = dbModel.PasswordReset{
		UserID:    7,
		TokenHash: "expired-hash",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"token": "expired-token", "password": "NewValidPassword123!"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestResetPassword_Success(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Users.PasswordReset = dbModel.PasswordReset{
		Model:     gorm.Model{ID: 1},
		UserID:    42,
		TokenHash: "valid-hash",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx, w := repomocks.SetupGinContextWithMocks(m)

	body, _ := json.Marshal(map[string]string{"token": "valid-token", "password": "NewValidPassword123!"})
	ctx.Request, _ = http.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	ResetPassword(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
}
