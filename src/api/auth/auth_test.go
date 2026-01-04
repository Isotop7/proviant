package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestContext creates a test context with mock dependencies
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

// TestSignupWithoutLogger tests the signup without a logger in the context
func TestSignupWithoutLogger(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create test context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Setup test database
	ctx.Set("dbHandle", db)

	// Create test request body
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, api.ResponseErrLoggerContextNotFound.Message, response["message"])
}

// TestSignupWithoutDatabase tests the signup without a logger in the context
func TestSignupWithoutDatabase(t *testing.T) {
	// Create test context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Setup mock logger
	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)

	// Create test request body
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, api.ResponseErrDatabaseContextNotFound.Message, response["message"])
}

// TestSignupGenericCreateError tests signup create error
func TestSignupGenericCreateError(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with short password
	signup := authentication.Signup{
		Username:    "testuserGenericCreateError",
		Password:    "ThisIsAVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryLongString",
		MailAddress: "testuserGenericCreateError@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, api.ResponseErrInvalidUserData.Message, response["message"])
}

// TestSignupSuccess tests the successful signup of a new user
func TestSignupSuccess(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request body
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "User was created", response["message"])
}

// TestSignupInvalidJSON tests signup with invalid JSON
func TestSignupInvalidJSON(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with invalid JSON
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBufferString("{invalid json}"))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "invalid character")
}

// TestSignupMissingFields tests signup with missing required fields
func TestSignupMissingFields(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with missing fields
	signup := authentication.Signup{
		Username: "testuser",
		// Missing password and mail address
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "required")
}

// TestSignupInvalidEmail tests signup with invalid email format
func TestSignupInvalidEmail(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with invalid email
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "invalid-email",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "@")
}

// TestSignupShortPassword tests signup with password that is too short
func TestSignupShortPassword(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with short password
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "short",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["message"], "password must at least be 8 characters long")
}

// TestSignupDuplicateUsername tests signup with duplicate username
func TestSignupDuplicateUsername(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create first user directly in database
	firstUser := authentication.User{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}
	if err := db.Create(&firstUser).Error; err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with duplicate username
	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "anotherpassword123",
		MailAddress: "another@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "user with this username already exists", response["message"])
}

// TestSignupDuplicateEmail tests signup with duplicate email
func TestSignupDuplicateEmail(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create first user directly in database
	firstUser := authentication.User{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}
	if err := db.Create(&firstUser).Error; err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	// Setup test context
	ctx, w := setupTestContext(db)

	// Create test request with duplicate email
	signup := authentication.Signup{
		Username:    "anotheruser",
		Password:    "anotherpassword123",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	// Call the handler directly
	Signup(ctx)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "user with this mail address already exists", response["message"])
}
