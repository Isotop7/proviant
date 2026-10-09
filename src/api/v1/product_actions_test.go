package v1

import (
	"net/http"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestWasteProduct(t *testing.T) {
	t.Run("concurrent modification returns 409 with retry message", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrProductConcurrentModification
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		appCtx := newTestAppContext(m, 1)
		WasteProduct(ctx, appCtx)

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusConflict, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), MsgProductConcurrentModification) {
			t.Errorf("body must carry retry message %q, got: %s", MsgProductConcurrentModification, w.Body.String())
		}
	})
}
