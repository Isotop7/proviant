package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestUpdateUser tests the UpdateUser endpoint
func TestUpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful user update", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{Username: "testuser", MailAddress: "test@example.com"}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := map[string]string{
			"displayName": "Updated Name",
			"mailAddress": "updated@example.com",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUser(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid user data", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := map[string]string{"mailAddress": "invalid-email"}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUser(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 999, testutil.TokenIdentityKey)

		reqBody := map[string]string{"mailAddress": "test@example.com"}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUser(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

// TestUpdateUserPassword tests the UpdateUserPassword endpoint
func TestUpdateUserPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful password update", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		loginData := authentication.Login{
			Username: "testuser",
			Password: "NewSecureTestPassword789!",
		}
		body, _ := json.Marshal(loginData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUserPassword(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid login data", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		loginData := authentication.Login{
			Username: "",
			Password: "short",
		}
		body, _ := json.Marshal(loginData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUserPassword(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 999, testutil.TokenIdentityKey)

		loginData := authentication.Login{
			Username: "testuser",
			Password: "ValidSecurePassword123!",
		}
		body, _ := json.Marshal(loginData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateUserPassword(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}
