package v1

import (
	"net/http"
	"testing"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestApplyForHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful application", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		ApplyForHousehold(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("household not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrHouseholdNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		ApplyForHousehold(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestApproveHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("application not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrApplicationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		ApproveHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("not household admin", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrNotHouseholdAdmin
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		ApproveHouseholdApplication(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})
}

func TestRejectHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("application not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrApplicationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		RejectHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestCancelHouseholdApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("application not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrApplicationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		CancelHouseholdApplication(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})
}

func TestRemoveHouseholdMember(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("caller is not household admin returns forbidden", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrNotHouseholdAdmin
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "userId", Value: "999"}}

		RemoveHouseholdMember(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})

	t.Run("cannot remove admin returns forbidden", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = proviantErrors.ErrNotHouseholdAdmin
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "userId", Value: "2"}}

		RemoveHouseholdMember(ctx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})
}
