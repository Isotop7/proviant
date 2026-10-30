package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

// seedCanonicalCategory puts a canonical category into the price table, so
// the resolver can name it.
func seedCanonicalCategory(t *testing.T, env *handlerTestEnv, key string) {
	t.Helper()
	row := dbModel.ProductCategoryPrice{CategoryKey: key, DisplayName: key, AvgPriceEUR: 1.50, CO2KgPerKg: 1.00, WeightGrams: 500}
	if err := env.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed canonical category: %v", err)
	}
}

// seedShelfLife gives a category of the household a default shelf life.
func seedShelfLife(t *testing.T, env *handlerTestEnv, key string, days int) {
	t.Helper()
	row := dbModel.HouseholdCategory{
		HouseholdID:          env.Household.ID,
		CategoryKey:          key,
		DisplayName:          key,
		DefaultShelfLifeDays: &days,
	}
	if err := env.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed shelf life: %v", err)
	}
}

// purchaseItem creates a free-text item of the household's list.
func purchaseItem(t *testing.T, env *handlerTestEnv, name, category string, quantity int) dbModel.ShoppingListItem {
	t.Helper()
	item := dbModel.ShoppingListItem{
		HouseholdID: env.Household.ID,
		Name:        name,
		Category:    category,
		Quantity:    quantity,
		CreatedBy:   env.User.ID,
	}
	if err := env.DB.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	return item
}

// buyItem runs the purchase handler for an item.
func buyItem(t *testing.T, env *handlerTestEnv, itemID uint, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	ctx, w := env.freshCtx(env.User.ID)
	ctx.Params = []gin.Param{{Key: "id", Value: itoa(itemID)}}
	testutil.CreateTestRequest(ctx, body)
	appCtx := SetupTestAppContext(ctx, env.User.ID)

	PurchaseShoppingListItem(ctx, appCtx)

	return w
}

func TestPurchaseShoppingListItem(t *testing.T) {
	t.Run("creates the product and links the item", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 2)

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !answer.Created {
			t.Error("created = false, want true for a free-text item")
		}
		if answer.Product == nil || answer.Product.ProductName != "Milk" {
			t.Fatalf("product = %+v, want a product named Milk", answer.Product)
		}
		if answer.Product.Quantity != 2 {
			t.Errorf("product quantity = %d, want 2", answer.Product.Quantity)
		}
		if !answer.Product.ScannedAt.IsZero() {
			t.Errorf("scannedAt = %v, want the zero time for an unscanned purchase", answer.Product.ScannedAt)
		}

		var stored dbModel.ShoppingListItem
		if err := env.DB.First(&stored, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if !stored.Checked {
			t.Error("item is not checked after the purchase")
		}
		if stored.ProductID == nil || *stored.ProductID != answer.Product.ID {
			t.Errorf("productId = %v, want %d", stored.ProductID, answer.Product.ID)
		}
	})

	t.Run("the expiry comes from the category default", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedShelfLife(t, env, "dairy products", 7)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		want := time.Now().AddDate(0, 0, 7)
		if answer.Product == nil || answer.Product.ExpireAt.IsZero() {
			t.Fatalf("product = %+v, want an expiry date", answer.Product)
		}
		if diff := answer.Product.ExpireAt.Sub(want); diff > time.Minute || diff < -time.Minute {
			t.Errorf("expireAt = %v, want about %v", answer.Product.ExpireAt, want)
		}
	})

	t.Run("a category without a default leaves the expiry empty", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !answer.Product.ExpireAt.IsZero() {
			t.Errorf("expireAt = %v, want the zero time without a category default", answer.Product.ExpireAt)
		}
	})

	t.Run("a confirmed date wins over the default", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedShelfLife(t, env, "dairy products", 7)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"expireAt": "2030-01-02"})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got := answer.Product.ExpireAt.Format("2006-01-02"); got != "2030-01-02" {
			t.Errorf("expireAt = %s, want 2030-01-02", got)
		}
	})

	t.Run("a malformed date is refused", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"expireAt": "not a date"})

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("the bought count is the product's count", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 2)

		w := buyItem(t, env, item.ID, map[string]any{"quantity": 5})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Product.Quantity != 5 {
			t.Errorf("product quantity = %d, want 5 (the bought count, not the asked one)", answer.Product.Quantity)
		}
	})

	t.Run("the bought count is clamped", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"quantity": 5000})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Product.Quantity != 999 {
			t.Errorf("product quantity = %d, want 999", answer.Product.Quantity)
		}
	})

	t.Run("a scanned purchase carries the scan stamp", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"barcode": "4006381333931", "scanned": true})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Product.ScannedAt.IsZero() {
			t.Error("scannedAt is zero, want the stamp of a scanned purchase")
		}
	})

	t.Run("an empty category key clears the category of the product", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedCanonicalCategory(t, env, "dairy products")
		seedShelfLife(t, env, "dairy products", 7)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"categoryKey": ""})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Product.Categories != "" {
			t.Errorf("categories = %q, want none", answer.Product.Categories)
		}
		if answer.Product.CategoryKey != nil {
			t.Errorf("categoryKey = %v, want none", *answer.Product.CategoryKey)
		}
		if !answer.Product.ExpireAt.IsZero() {
			t.Errorf("expireAt = %v, want the zero time without a category", answer.Product.ExpireAt)
		}
	})

	t.Run("an item that already points at a product gets the packs added", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 1
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		item := dbModel.ShoppingListItem{
			HouseholdID: env.Household.ID,
			ProductID:   &product.ID,
			Name:        product.ProductName,
			Category:    product.Categories,
			Quantity:    3,
			CreatedBy:   env.User.ID,
		}
		if err := env.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Created {
			t.Error("created = true, want false when the packs went into the linked product")
		}
		if answer.Product == nil || answer.Product.ID != product.ID {
			t.Fatalf("product = %+v, want the linked product %d", answer.Product, product.ID)
		}
		if answer.Product.Quantity != 4 {
			t.Errorf("product quantity = %d, want 4 (one plus three bought)", answer.Product.Quantity)
		}

		var products int64
		env.DB.Model(&dbModel.Product{}).Count(&products)
		if products != 1 {
			t.Errorf("products = %d, want 1 (no second row)", products)
		}

		var stored dbModel.ShoppingListItem
		if err := env.DB.First(&stored, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if !stored.Checked {
			t.Error("item is not checked after the purchase")
		}
		if stored.ProductID == nil || *stored.ProductID != product.ID {
			t.Errorf("productId = %v, want %d", stored.ProductID, product.ID)
		}
	})

	t.Run("an item whose product is gone buys a new one", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		item := dbModel.ShoppingListItem{
			HouseholdID: env.Household.ID,
			ProductID:   &product.ID,
			Name:        "Milk",
			Quantity:    2,
			CreatedBy:   env.User.ID,
		}
		if err := env.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}
		if err := env.DB.Delete(&product).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !answer.Created {
			t.Error("created = false, want true when the product of the item is gone")
		}
		if answer.Product == nil || answer.Product.ProductName != "Milk" {
			t.Fatalf("product = %+v, want a product named Milk", answer.Product)
		}
	})

	t.Run("a storage location of another household is refused", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := testutil.CreateTestHousehold(env.DB, 0)
		location := testutil.CreateTestStorageLocation(env.DB, other.ID)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"storageLocationId": location.ID})

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", w.Code, w.Body.String())
		}
		var products int64
		env.DB.Model(&dbModel.Product{}).Count(&products)
		if products != 0 {
			t.Errorf("products = %d, want 0 (the purchase was refused)", products)
		}
	})

	t.Run("a storage location that does not exist is refused", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"storageLocationId": 9999})

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("a storage location of the household is kept", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"storageLocationId": location.ID})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Product.StorageLocationID == nil || *answer.Product.StorageLocationID != location.ID {
			t.Errorf("storageLocationId = %v, want %d", answer.Product.StorageLocationID, location.ID)
		}
	})

	t.Run("the packs go into the product the request names", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 1
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		item := purchaseItem(t, env, "Milk", "dairy products", 2)

		w := buyItem(t, env, item.ID, map[string]any{"addToProductId": product.ID})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		var answer apiModel.PurchaseShoppingListItemResponse
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if answer.Created {
			t.Error("created = true, want false when the packs went into the named product")
		}
		if answer.Product == nil || answer.Product.ID != product.ID {
			t.Fatalf("product = %+v, want the named product %d", answer.Product, product.ID)
		}
		if answer.Product.Quantity != 3 {
			t.Errorf("product quantity = %d, want 3 (one plus two bought)", answer.Product.Quantity)
		}

		var products int64
		env.DB.Model(&dbModel.Product{}).Count(&products)
		if products != 1 {
			t.Errorf("products = %d, want 1 (no second row)", products)
		}

		var stored dbModel.ShoppingListItem
		if err := env.DB.First(&stored, item.ID).Error; err != nil {
			t.Fatalf("load item: %v", err)
		}
		if !stored.Checked {
			t.Error("item is not checked after the purchase")
		}
		if stored.ProductID == nil || *stored.ProductID != product.ID {
			t.Errorf("productId = %v, want %d", stored.ProductID, product.ID)
		}
	})

	t.Run("a product of another household cannot be bought into", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := testutil.CreateTestHousehold(env.DB, 0)
		product := testutil.CreateTestProduct(env.DB, other.ID, env.User.ID)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{"addToProductId": product.ID})

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("an item of another household is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := testutil.CreateTestHousehold(env.DB, 0)
		item := dbModel.ShoppingListItem{HouseholdID: other.ID, Name: "Milk", Quantity: 1, CreatedBy: env.User.ID}
		if err := env.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("the activity log fires once", func(t *testing.T) {
		env := setupHandlerTest(t)
		item := purchaseItem(t, env, "Milk", "dairy products", 1)

		w := buyItem(t, env, item.ID, map[string]any{})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		// The log is written by a worker, so it arrives a moment later.
		var logs []dbModel.ActivityLog
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			logs = nil
			env.DB.Where("action = ?", dbModel.ActivityActionAdd).Find(&logs)
			if len(logs) > 0 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if len(logs) != 1 {
			t.Fatalf("activity logs = %d, want exactly 1", len(logs))
		}
		if logs[0].ProductName != "Milk" {
			t.Errorf("logged product = %q, want Milk", logs[0].ProductName)
		}
	})
}
