package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

// TestUpdateUser tests the UpdateUser endpoint
func TestUpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)

	t.Run("successful user update", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		// Request body
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
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUser(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		// Verify user was updated in database
		var updatedUser authentication.User
		db.First(&updatedUser, testUser.ID)
		if updatedUser.MailAddress != "updated@example.com" {
			t.Errorf("MailAddress = %v, want updated@example.com", updatedUser.MailAddress)
		}
	})

	t.Run("invalid user data", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		// Request body with invalid data
		userData := authentication.User{
			Username:    "", // Empty username
			MailAddress: "invalid-email",
		}
		body, _ := json.Marshal(userData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUser(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, 999, testutil.TokenIdentityKey) // Non-existent user ID

		// Request body
		userData := authentication.User{
			Username:    "testuser",
			MailAddress: "test@example.com",
		}
		body, _ := json.Marshal(userData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUser(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestUpdateUserPassword tests the UpdateUserPassword endpoint
func TestUpdateUserPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)

	t.Run("successful password update", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "oldpassword123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		// Request body
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
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUserPassword(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid login data", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		// Request body with invalid data
		loginData := authentication.Login{
			Username: "",      // Empty username
			Password: "short", // Too short password
		}
		body, _ := json.Marshal(loginData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUserPassword(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		// Setup test context
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, 999, testutil.TokenIdentityKey) // Non-existent user ID

		// Request body
		loginData := authentication.Login{
			Username: "testuser",
			Password: "newpassword123",
		}
		body, _ := json.Marshal(loginData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		UpdateUserPassword(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}
