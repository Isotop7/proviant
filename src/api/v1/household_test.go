package v1

import (
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/testutil"
	"github.com/gin-gonic/gin"
)

func TestApplyForHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("successful application", func(t *testing.T) {
		testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		ApplyForHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("household not found", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		ApplyForHousehold(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestApproveHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("application not found", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		ApproveHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("not household admin", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		ApproveHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v or %v", w.Code, http.StatusNotFound, http.StatusForbidden)
		}
	})
}

func TestRejectHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("application not found", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		RejectHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestCancelHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("application not found", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		CancelHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestRemoveHouseholdMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("member not in household returns forbidden", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "userId", Value: "999"}}

		RemoveHouseholdMember(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})

	t.Run("cannot remove admin returns forbidden", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "userId", Value: "1"}}

		RemoveHouseholdMember(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})
}