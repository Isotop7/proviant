package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestContext creates a test context with database and logger
func setupTestContext(db *gorm.DB) (*gin.Context, *httptest.ResponseRecorder) {
	// Create test context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Setup mock logger
	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)

	// Setup test database
	ctx.Set("dbHandle", db)

	return ctx, w
}

// mockJWTContext sets up JWT claims in context for testing
func mockJWTContext(ctx *gin.Context, userID uint) {
	// Mock JWT claims using the same structure as the JWT middleware
	ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
		static.TokenIdentityKey: float64(userID),
	})
}

// TestUpdateUser tests the UpdateUser endpoint
func TestUpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("successful user update", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			ID:          1,
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, testUser.ID)

		// Request body
		userData := authentication.User{
			ID:          1,
			Username:    "updateduser",
			MailAddress: "updated@example.com",
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
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		// Verify user was updated in database
		var updatedUser authentication.User
		db.First(&updatedUser, testUser.ID)
		if updatedUser.Username != "updateduser" {
			t.Errorf("Username = %v, want updateduser", updatedUser.Username)
		}
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
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, testUser.ID)

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
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, 999) // Non-existent user ID

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

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("successful password update", func(t *testing.T) {
		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "oldpassword123",
			MailAddress: "test@example.com",
		}
		db.Create(&testUser)

		// Setup test context
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, testUser.ID)

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
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, testUser.ID)

		// Request body with invalid data
		loginData := authentication.Login{
			Username: "", // Empty username
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
		ctx, w := setupTestContext(db)
		mockJWTContext(ctx, 999) // Non-existent user ID

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
