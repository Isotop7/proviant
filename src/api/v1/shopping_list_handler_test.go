package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func seedSubThresholdProduct(t *testing.T, env *handlerTestEnv) *dbModel.Product {
	t.Helper()
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	product.MinStockAmount = 5
	product.Amount = 2
	product.Unit = "l"
	if err := env.DB.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	return product
}

func TestGetAutoShoppingList(t *testing.T) {
	t.Run("returns sub-threshold products", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedSubThresholdProduct(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/shopping-list/auto", nil)

		GetAutoShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var products []dbModel.Product
		if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(products) != 1 {
			t.Errorf("products = %d, want 1", len(products))
		}
	})

	t.Run("empty when nothing below threshold", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/shopping-list/auto", nil)

		GetAutoShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var products []dbModel.Product
		if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("products = %d, want 0", len(products))
		}
	})
}

func TestCreateShoppingListItem(t *testing.T) {
	t.Run("creates item with name", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{
			"name":     "Milk",
			"category": "Dairy",
			"quantity": 2,
			"unit":     "l",
			"notes":    "low fat",
		})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		if err := json.Unmarshal(env.W.Body.Bytes(), &item); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if item.Name != "Milk" || item.Quantity != 2 || item.HouseholdID != env.Household.ID || item.CreatedBy != env.User.ID {
			t.Errorf("item = %+v, want Milk/2 in household %d", item, env.Household.ID)
		}
	})

	t.Run("quantity defaults to 1", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "Eggs"})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		_ = json.Unmarshal(env.W.Body.Bytes(), &item)
		if item.Quantity != 1 {
			t.Errorf("quantity = %d, want 1", item.Quantity)
		}
	})

	t.Run("name is derived from product", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Categories = "Dairy"
		env.DB.Save(product)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"productId": product.ID})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var item dbModel.ShoppingListItem
		_ = json.Unmarshal(env.W.Body.Bytes(), &item)
		if item.Name != product.ProductName || item.Category != "Dairy" {
			t.Errorf("item = %+v, want name %q from product", item, product.ProductName)
		}
		if item.ProductID == nil || *item.ProductID != product.ID {
			t.Errorf("productId = %v, want %d", item.ProductID, product.ID)
		}
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"quantity": 2})

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("malformed body returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/x", bytesReaderString("{invalid"))
		env.Ctx.Request.Header.Set("Content-Type", "application/json")

		CreateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestListShoppingListItems(t *testing.T) {
	env := setupHandlerTest(t)
	env.DB.Create(&dbModel.ShoppingListItem{HouseholdID: env.Household.ID, Name: "Milk", Quantity: 1, CreatedBy: env.User.ID})
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/shopping-list", nil)

	ListShoppingListItems(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var items []dbModel.ShoppingListItem
	if err := json.Unmarshal(env.W.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Milk" {
		t.Errorf("items = %+v, want one Milk item", items)
	}
}

func TestUpdateShoppingListItem(t *testing.T) {
	t.Run("updates fields", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := dbModel.ShoppingListItem{HouseholdID: env.Household.ID, Name: "Milk", Quantity: 1, CreatedBy: env.User.ID}
		env.DB.Create(&item)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}
		checked := true
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "Oat Milk", "checked": checked, "quantity": 3})

		UpdateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var updated dbModel.ShoppingListItem
		if err := env.DB.First(&updated, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if updated.Name != "Oat Milk" || !updated.Checked || updated.Quantity != 3 {
			t.Errorf("item = %+v, want Oat Milk/checked/3", updated)
		}
	})

	t.Run("item not found returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "X"})

		UpdateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "X"})

		UpdateShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestDeleteShoppingListItem(t *testing.T) {
	t.Run("deletes item", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := dbModel.ShoppingListItem{HouseholdID: env.Household.ID, Name: "Milk", Quantity: 1, CreatedBy: env.User.ID}
		env.DB.Create(&item)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}

		DeleteShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&dbModel.ShoppingListItem{}).Where("id = ?", item.ID).Count(&count)
		if count != 0 {
			t.Errorf("item still present, count = %d", count)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "0"}}

		DeleteShoppingListItem(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestToggleShoppingListItem(t *testing.T) {
	env := setupHandlerTest(t)
	item := dbModel.ShoppingListItem{HouseholdID: env.Household.ID, Name: "Milk", Quantity: 1, CreatedBy: env.User.ID}
	env.DB.Create(&item)
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(item.ID)}}

	ToggleShoppingListItem(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var toggled dbModel.ShoppingListItem
	if err := json.Unmarshal(env.W.Body.Bytes(), &toggled); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !toggled.Checked {
		t.Errorf("checked = false, want true after toggle")
	}
}

func TestImportAutoListToShoppingList(t *testing.T) {
	t.Run("imports sub-threshold products", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := seedSubThresholdProduct(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shopping-list/import-auto", nil)

		ImportAutoListToShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var items []dbModel.ShoppingListItem
		if err := env.DB.Where("household_id = ?", env.Household.ID).Find(&items).Error; err != nil {
			t.Fatalf("load items: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("items = %d, want 1", len(items))
		}
		if items[0].ProductID == nil || *items[0].ProductID != product.ID {
			t.Errorf("item productId = %v, want %d", items[0].ProductID, product.ID)
		}
		if items[0].Quantity != 3 { // minStockAmount 5 - amount 2
			t.Errorf("item quantity = %d, want 3", items[0].Quantity)
		}
	})

	t.Run("second import skips existing", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedSubThresholdProduct(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shopping-list/import-auto", nil)

		ImportAutoListToShoppingList(env.Ctx, env.AppCtx)
		if env.W.Code != http.StatusOK {
			t.Fatalf("first call status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}

		// A recorder latches its first status code, so the second call needs
		// its own context + recorder to be able to fail the assertion.
		ctx2, w2 := env.freshCtx(env.User.ID)
		ctx2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shopping-list/import-auto", nil)
		appCtx2 := SetupTestAppContext(ctx2, env.User.ID)

		ImportAutoListToShoppingList(ctx2, appCtx2)

		if w2.Code != http.StatusOK {
			t.Errorf("second call status = %d, want 200; body = %s", w2.Code, w2.Body.String())
		}
		var items []dbModel.ShoppingListItem
		env.DB.Where("household_id = ?", env.Household.ID).Find(&items)
		if len(items) != 1 {
			t.Errorf("items = %d, want 1 (no duplicates)", len(items))
		}
	})

	t.Run("no sub-threshold products", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shopping-list/import-auto", nil)

		ImportAutoListToShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
	})
}
