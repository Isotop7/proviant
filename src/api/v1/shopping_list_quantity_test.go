package v1

import (
	"net/http"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

// A row always stands for at least one pack, so a count below one is clamped
// instead of stored.
func TestUpdateShoppingListItemClampsTheCount(t *testing.T) {
	env := setupHandlerTest(t)
	item := dbModel.ShoppingListItem{HouseholdID: env.Household.ID, Name: "Milk", Quantity: 3, CreatedBy: env.User.ID}
	if err := env.DB.Create(&item).Error; err != nil {
		t.Fatalf("seed item: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}
	testutil.CreateTestRequest(env.Ctx, map[string]any{"quantity": -5})

	UpdateShoppingListItem(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var updated dbModel.ShoppingListItem
	if err := env.DB.First(&updated, item.ID).Error; err != nil {
		t.Fatalf("load item: %v", err)
	}
	if updated.Quantity != 1 {
		t.Errorf("count = %d, want 1 (a row always stands for at least one pack)", updated.Quantity)
	}
}
