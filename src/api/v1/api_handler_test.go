package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	proviantErrors "codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func TestGetSavingsStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user without household returns empty stats", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 0
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		GetSavingsStats(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household returns savings", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 42
		m.Savings.SavingsStats = apiModel.SavingsStatsResponse{CO2Source: "test"}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		GetSavingsStats(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestListStorageLocations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user with no locations returns empty list", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		ListStorageLocations(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with locations returns list", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.StorageLocations.Locations = []dbModel.StorageLocation{{Name: "Freezer"}}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

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

	t.Run("successful storage location creation", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.StorageLocations.Location = dbModel.StorageLocation{Name: "Freezer"}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := storageLocationRequest{Name: "Freezer", Icon: "🧊", SortOrder: 2}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		CreateStorageLocation(ctx)

		if w.Code != http.StatusCreated {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid request without name", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := storageLocationRequest{Name: "", Icon: "🧊", SortOrder: 2}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		CreateStorageLocation(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestGetInvitations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user without household returns not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 0}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		GetInvitations(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("user with household returns invitations", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 42}
		m.Invitations.Invitations = []dbModel.HouseholdInvitation{
			{Email: "test@example.com", Token: "test-token-123"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		GetInvitations(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestGetNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user without household returns empty notifications", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 0}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		GetNotifications(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("user with household returns notifications", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 42}
		m.Invitations.Invitations = []dbModel.HouseholdInvitation{
			{Email: "newuser@example.com", Token: "test-token-456"},
		}
		m.Households.Household = dbModel.Household{AdminID: 99}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

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

func TestCreateHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful household creation", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := createHouseholdRequest{Name: "My New Household"}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		CreateHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("empty household name returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := createHouseholdRequest{Name: ""}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		CreateHousehold(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestLeaveHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user leaves successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		LeaveHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

func TestUpdateHouseholdName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("household not found returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 42}
		m.Households.Err = proviantErrors.ErrHouseholdNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := updateHouseholdNameRequest{Name: "New Name"}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateHouseholdName(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("empty name returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := updateHouseholdNameRequest{Name: ""}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateHouseholdName(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestListWebhooks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user with no webhooks returns empty list", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

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
