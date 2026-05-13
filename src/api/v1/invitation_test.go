package v1

import (
	"net/http"
	"testing"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestCancelInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful cancellation", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		CancelInvitation(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("invitation not found returns not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Invitations.Err = proviantErrors.ErrInvitationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		CancelInvitation(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("invalid invitation ID returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)
		ctx.Params = []gin.Param{{Key: "id", Value: "invalid"}}

		CancelInvitation(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("non-admin cannot cancel invitation", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Invitations.Err = proviantErrors.ErrInvitationNotAuthorized
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 2, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 2)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		CancelInvitation(ctx, appCtx)

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusForbidden)
		}
	})
}
