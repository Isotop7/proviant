package v1

import (
	"net/http"
	"testing"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"github.com/gin-gonic/gin"
)

func TestCancelInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("successful cancellation", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		invitation := dbModel.HouseholdInvitation{
			HouseholdID: household.ID,
			InviterID:   testUser.ID,
			Email:       "invitee@example.com",
			Token:       "cancel-token",
		}
		db.Create(&invitation)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		CancelInvitation(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invitation not found returns not found", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		CancelInvitation(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("invalid invitation ID returns bad request", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "invalid"}}

		CancelInvitation(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("non-admin cannot cancel invitation", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		invitation := dbModel.HouseholdInvitation{
			HouseholdID: household.ID,
			InviterID:   testUser.ID,
			Email:       "invitee@example.com",
			Token:       "cancel-token-2",
		}
		db.Create(&invitation)

		otherUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, otherUser.ID, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		CancelInvitation(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})
}