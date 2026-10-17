package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
)

// GetProductBySyncID scopes by household only, so the idempotent 200 in
// CreateProduct has to re-check the private-product rule itself: a known sync
// id must not turn into a read of another member's row.
func TestCreateProductSyncIDVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	seedWithSyncID := func(t *testing.T, env *handlerTestEnv, owner *authentication.User, name, syncID string, isPrivate bool) *dbModel.Product {
		t.Helper()
		product := &dbModel.Product{
			ProductName:  name,
			Barcode:      "4006381333931",
			HouseholdID:  env.Household.ID,
			UserID:       owner.ID,
			IsPrivate:    isPrivate,
			ClientSyncID: &syncID,
		}
		if err := env.DB.Create(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		return product
	}

	t.Run("an owner replays their own private sync id and gets the product", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		seedWithSyncID(t, env, member, "Secret stash", "priv-own", true)
		env.useMember(member)
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333932","clientSyncId":"priv-own"}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.ProductResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ProductName != "Secret stash" {
			t.Errorf("productName = %q, want the owner's product", resp.ProductName)
		}
	})

	// env.User is the household admin: same household, different owner.
	t.Run("another member's private product does not resolve by sync id", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		private := seedWithSyncID(t, env, member, "Secret stash", "priv-foreign", true)
		env.Ctx.Set("offacntrl", stubDatasetGetter{err: fmt.Errorf("no dataset entry")})
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333932","clientSyncId":"priv-foreign"}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 — the id is taken and the row is not the caller's; body = %s", env.W.Code, env.W.Body.String())
		}
		body := env.W.Body.String()
		if strings.Contains(body, "Secret stash") || strings.Contains(body, itoa(private.ID)) {
			t.Errorf("response leaks the private product: %s", body)
		}
	})

	t.Run("a shared product resolves for any member of the household", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		shared := seedWithSyncID(t, env, member, "Household milk", "shared-1", false)
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333932","clientSyncId":"shared-1"}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.ProductResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID != shared.ID || resp.ProductName != "Household milk" {
			t.Errorf("response = %+v, want product %d", resp, shared.ID)
		}
	})
}
