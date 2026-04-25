package onboarding

import (
	"testing"

	"codeberg.org/isotop7/proviant/testutil"
	"github.com/gin-gonic/gin"
)

func TestGetOnboardingState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("user without onboarding state returns completed", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		GetOnboardingState(ctx)
	})
}