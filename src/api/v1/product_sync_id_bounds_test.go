package v1

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// Both idempotency tokens land in an indexed column, and GORM maps an indexed Go
// string to varchar(191) on MySQL/MariaDB. An unbounded token therefore overflows
// at insert time and the caller sees a 500 for what is really bad input.
func TestSyncIDLengthBounds(t *testing.T) {
	oversized := strings.Repeat("m", util.OfflineSyncIDMaxLength+1)

	t.Run("CreateProduct rejects an over-long clientSyncId", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest(fmt.Sprintf(
			`{"barcode":"4006381333931","clientSyncId":%q}`, oversized,
		))

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
		}
	})

	t.Run("UpdateProductAmount rejects an over-long clientMutationId", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: fmt.Sprint(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(fmt.Sprintf(
			`{"delta":1,"clientMutationId":%q}`, oversized,
		))

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
		}
	})
}

// The lookup trims a clientSyncId, so the row has to be stored trimmed too: an
// untrimmed row never resolves, and every retry then collides on the unique
// index instead of reporting the product that already exists.
func TestCreateProductStoresClientSyncIDTrimmed(t *testing.T) {
	env := setupHandlerTest(t)
	env.Ctx.Set("offacntrl", stubDatasetGetter{err: fmt.Errorf("no dataset entry")})
	env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333931","clientSyncId":"  sync-padded  "}`)

	CreateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.Where("barcode = ?", "4006381333931").First(&stored).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.ClientSyncID == nil || *stored.ClientSyncID != "sync-padded" {
		t.Errorf("ClientSyncID = %v, want %q", stored.ClientSyncID, "sync-padded")
	}
}
