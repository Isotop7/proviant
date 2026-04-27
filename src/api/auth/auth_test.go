package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func TestSignupWithoutLogger(t *testing.T) {
	db := testutil.SetupTestDB(t)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("dbHandle", db)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != api.ResponseErrLoggerContextNotFound.Message {
		t.Errorf("message = %v, want %v", response["message"], api.ResponseErrLoggerContextNotFound.Message)
	}
}

func TestSignupWithoutDatabase(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != api.ResponseErrDatabaseContextNotFound.Message {
		t.Errorf("message = %v, want %v", response["message"], api.ResponseErrDatabaseContextNotFound.Message)
	}
}

func TestSignupGenericCreateError(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "testuserGenericCreateError",
		Password:    "ThisIsAVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryLongString",
		MailAddress: "testuserGenericCreateError@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != api.ResponseErrInvalidUserData.Message {
		t.Errorf("message = %v, want %v", response["message"], api.ResponseErrInvalidUserData.Message)
	}
}

func TestSignupSuccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != "User was created" {
		t.Errorf("message = %v, want User was created", response["message"])
	}
}

func TestSignupInvalidJSON(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBufferString("{invalid json}"))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == "" || len(response["message"]) < 15 {
		t.Errorf("message = %v, want to contain 'invalid character'", response["message"])
	}
}

func TestSignupMissingFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username: "testuser",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == "" || len(response["message"]) < 8 {
		t.Errorf("message = %v, want to contain 'required'", response["message"])
	}
}

func TestSignupInvalidEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "ThisIsAVeryStrongTestPass999!",
		MailAddress: "invalid-email",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == "" || len(response["message"]) < 1 {
		t.Errorf("message = %v, want to contain '@'", response["message"])
	}
}

func TestSignupShortPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "short",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == "" || len(response["message"]) < 35 {
		t.Errorf("message = %v, want to contain 'password must be at least 12 characters long'", response["message"])
	}
}

func TestSignupDuplicateUsername(t *testing.T) {
	db := testutil.SetupTestDB(t)

	firstUser := authentication.User{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
	}
	if err := db.Create(&firstUser).Error; err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "testuser",
		Password:    "AnotherVeryStrongTestPass456!",
		MailAddress: "another@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != "user with this username already exists" {
		t.Errorf("message = %v, want 'user with this username already exists'", response["message"])
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)

	firstUser := authentication.User{
		Username:    "testuser",
		Password:    "ThisIsAVeryStrongTestPass123!",
		MailAddress: "test@example.com",
	}
	if err := db.Create(&firstUser).Error; err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)

	signup := authentication.Signup{
		Username:    "anotheruser",
		Password:    "AnotherVeryStrongTestPass456!",
		MailAddress: "test@example.com",
	}

	jsonValue, _ := json.Marshal(signup)
	ctx.Request, _ = http.NewRequest("POST", "/auth/signup", bytes.NewBuffer(jsonValue))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Signup(ctx)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] != "user with this mail address already exists" {
		t.Errorf("message = %v, want 'user with this mail address already exists'", response["message"])
	}
}
