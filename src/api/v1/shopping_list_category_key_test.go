package v1

import (
	"encoding/json"
	"net/http"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

// The canonical category of a shopping list item is what its wording names.
// The category of the product it came from is the fallback for a wording that
// names none. One order for every path, so the same wording always names the
// same category.
func TestShoppingListItemCategoryKey(t *testing.T) {
	t.Run("the wording of the item names the category", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedCanonicalCategory(t, env, "milk")
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Categories = "Dairy"
		product.CategoryKey = new("milk")
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"productId": product.ID})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		if err := json.Unmarshal(env.W.Body.Bytes(), &item); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if item.CategoryKey == nil || *item.CategoryKey != "dairy products" {
			t.Errorf("categoryKey = %v, want dairy products (the wording of the item)", item.CategoryKey)
		}
	})

	t.Run("a wording that names none keeps the category of the product", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "breads")
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Categories = "Bakery"
		product.CategoryKey = new("breads")
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"productId": product.ID})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		if err := json.Unmarshal(env.W.Body.Bytes(), &item); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if item.CategoryKey == nil || *item.CategoryKey != "breads" {
			t.Errorf("categoryKey = %v, want breads (the category of the product)", item.CategoryKey)
		}
	})

	t.Run("a free-text item names its own category", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "Milk", "category": "Dairy"})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		if err := json.Unmarshal(env.W.Body.Bytes(), &item); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if item.CategoryKey == nil || *item.CategoryKey != "dairy products" {
			t.Errorf("categoryKey = %v, want dairy products", item.CategoryKey)
		}
	})

	t.Run("a renamed category re-resolves the key", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedCanonicalCategory(t, env, "breads")
		item := dbModel.ShoppingListItem{
			HouseholdID: env.Household.ID,
			Name:        "Roll",
			Category:    "Dairy",
			CategoryKey: new("dairy products"),
			Quantity:    1,
			CreatedBy:   env.User.ID,
		}
		env.DB.Create(&item)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"category": "Breads"})

		UpdateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var updated dbModel.ShoppingListItem
		if err := env.DB.First(&updated, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if updated.CategoryKey == nil || *updated.CategoryKey != "breads" {
			t.Errorf("categoryKey = %v, want breads", updated.CategoryKey)
		}
	})

	t.Run("a renamed category that names none keeps the category of the product", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedCanonicalCategory(t, env, "breads")
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Categories = "Dairy"
		product.CategoryKey = new("breads")
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		item := dbModel.ShoppingListItem{
			HouseholdID: env.Household.ID,
			ProductID:   &product.ID,
			Name:        "Roll",
			Category:    "Dairy",
			CategoryKey: new("dairy products"),
			Quantity:    1,
			CreatedBy:   env.User.ID,
		}
		env.DB.Create(&item)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"category": "Bakery"})

		UpdateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var updated dbModel.ShoppingListItem
		if err := env.DB.First(&updated, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if updated.CategoryKey == nil || *updated.CategoryKey != "breads" {
			t.Errorf("categoryKey = %v, want breads (the category of the product)", updated.CategoryKey)
		}
	})
}
