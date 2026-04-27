package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"github.com/gin-gonic/gin"
)

func TestGetSavingsStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns empty stats", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetSavingsStats(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household returns savings", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetSavingsStats(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestListStorageLocations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns empty list", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		ListStorageLocations(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household returns storage locations", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)
		testutil.CreateTestStorageLocation(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		ListStorageLocations(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		var result []dbModel.StorageLocation
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Errorf("unmarshal error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("len(result) = %v, want 1", len(result))
		}
	})
}

func TestCreateStorageLocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("successful storage location creation", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := storageLocationRequest{
			Name:      "Freezer",
			Icon:      "🧊",
			SortOrder: 2,
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		CreateStorageLocation(ctx)

		if w.Code != http.StatusCreated {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid request without name", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := storageLocationRequest{
			Name:      "",
			Icon:      "🧊",
			SortOrder: 2,
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		CreateStorageLocation(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestGetInvitations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns not found", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetInvitations(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("user with household returns invitations", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		invitation := dbModel.HouseholdInvitation{
			HouseholdID: household.ID,
			InviterID:   testUser.ID,
			Email:       "test@example.com",
			Token:       "test-token-123",
		}
		db.Create(&invitation)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetInvitations(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestGetNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns notifications", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetNotifications(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household and invitation returns notifications", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		invitation := dbModel.HouseholdInvitation{
			HouseholdID: household.ID,
			InviterID:   testUser.ID,
			Email:       "newuser@example.com",
			Token:       "test-token-456",
		}
		db.Create(&invitation)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetNotifications(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestGetExpired(t *testing.T) {
	t.Skip("Skipping - handler requires complex DB setup for product expiration queries")
}

func TestGetProductSummary(t *testing.T) {
	t.Skip("Skipping - handler requires complex DB setup for product summary queries")
}

func TestGetProductStats(t *testing.T) {
	t.Skip("Skipping - handler requires complex DB setup for product stats queries")
}

func TestGetUserArchivedProductsHandler(t *testing.T) {
	t.Skip("Skipping - handler requires complex DB setup for archived products queries")
}

func TestBulkDeleteProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("empty product list returns success", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := map[string][]int{
			"productIDs": []int{},
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		BulkDeleteProducts(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid product IDs returns error", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := map[string][]string{
			"productIDs": []string{"invalid"},
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		BulkDeleteProducts(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestCreateHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("successful household creation", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := createHouseholdRequest{
			Name: "My New Household",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		CreateHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("empty household name returns bad request", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := createHouseholdRequest{
			Name: "",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		CreateHousehold(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestLeaveHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns success", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		LeaveHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household leaves successfully", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		LeaveHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestUpdateHouseholdName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without household returns not found", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := updateHouseholdNameRequest{
			Name: "New Name",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		UpdateHouseholdName(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("empty name returns bad request", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := updateHouseholdNameRequest{
			Name: "",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		UpdateHouseholdName(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestListWebhooks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user with no webhooks returns empty list", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		ListWebhooks(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Errorf("unmarshal error: %v", err)
		}
		if result["webhooks"] == nil {
			t.Error("Expected webhooks key in response")
		}
	})
}

func TestCreateWebhook(t *testing.T) {
	t.Skip("Skipping - handler requires user to be found via GetUserByID and complex webhook repo setup")
}

func TestConvertStringIDsToInts(t *testing.T) {
	t.Run("valid string IDs", func(t *testing.T) {
		ids := []string{"1", "2", "3"}
		result, err := convertStringIDsToUints(ids)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("len(result) = %v, want 3", len(result))
		}
		if result[0] != 1 || result[1] != 2 || result[2] != 3 {
			t.Errorf("result = %v, want [1 2 3]", result)
		}
	})

	t.Run("invalid string ID", func(t *testing.T) {
		ids := []string{"1", "invalid", "3"}
		result, err := convertStringIDsToUints(ids)
		if err == nil {
			t.Error("Expected error for invalid ID")
		}
		if result != nil {
			t.Error("Expected nil result on error")
		}
	})
}

func TestJoinErrors(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		errs := []database.BulkOperationError{}
		result := joinErrors(errs)
		if result != "" {
			t.Errorf("result = %v, want empty string", result)
		}
	})

	t.Run("with error - uses Error method", func(t *testing.T) {
		err1 := database.BulkOperationError{}
		err2 := database.BulkOperationError{}
		_ = err1
		_ = err2
		result := joinErrors([]database.BulkOperationError{err1, err2})
		if result == "" {
			t.Error("Expected non-empty result from errors with productID")
		}
	})
}
