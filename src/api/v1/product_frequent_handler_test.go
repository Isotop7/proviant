package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	repomocks "github.com/Isotop7/proviant/testutil/mocks"
)

func TestGetFrequentlyBoughtProductsHandler(t *testing.T) {
	t.Run("returns the household's staples", func(t *testing.T) {
		env := setupHandlerTest(t)
		for range 2 {
			product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
			product.Barcode = "4006381333931"
			product.ProductName = "Milk"
			product.PackSize = "1 l"
			if err := env.DB.Save(product).Error; err != nil {
				t.Fatalf("save product: %v", err)
			}
			if err := env.DB.Delete(product).Error; err != nil {
				t.Fatalf("archive product: %v", err)
			}
		}
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/frequently-bought", nil)

		GetFrequentlyBoughtProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var products []apiModel.FrequentProduct
		if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(products) != 1 {
			t.Fatalf("products = %+v, want the one staple", products)
		}
		if products[0].Barcode != "4006381333931" {
			t.Errorf("barcode = %q, want 4006381333931", products[0].Barcode)
		}
		if products[0].ProductName != "Milk" {
			t.Errorf("name = %q, want Milk", products[0].ProductName)
		}
		if products[0].PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", products[0].PackSize)
		}
		if products[0].PurchaseCount != 2 {
			t.Errorf("purchase count = %d, want 2", products[0].PurchaseCount)
		}
		if products[0].LastBoughtAt.IsZero() {
			t.Error("lastBoughtAt is zero, want the date of the last purchase")
		}
	})

	t.Run("empty list when the household bought nothing twice", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Barcode = "4000000000099"
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		if err := env.DB.Delete(product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/frequently-bought", nil)

		GetFrequentlyBoughtProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var products []apiModel.FrequentProduct
		if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("products = %+v, want none", products)
		}
	})

	t.Run("reports a failing lookup as a server error", func(t *testing.T) {
		env := setupHandlerTest(t)
		mock := &repomocks.MockProductRepository{Err: errors.New("database is down")}
		env.AppCtx.Repos.Products = mock
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/frequently-bought", nil)

		GetFrequentlyBoughtProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body = %s", env.W.Code, env.W.Body.String())
		}
	})
}

// The strip is household-scoped: a staple of another household is never part
// of the answer.
func TestGetFrequentlyBoughtProductsHandlerHouseholdScope(t *testing.T) {
	env := setupHandlerTest(t)
	otherHousehold := testutil.CreateTestHousehold(env.DB, 0)
	otherUser := testutil.CreateTestUser(env.DB, otherHousehold.ID)
	for range 2 {
		product := testutil.CreateTestProduct(env.DB, otherHousehold.ID, otherUser.ID)
		product.Barcode = "4000000000077"
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		if err := env.DB.Delete(product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
	}
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/frequently-bought", nil)

	GetFrequentlyBoughtProducts(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", env.W.Code)
	}
	var products []apiModel.FrequentProduct
	if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(products) != 0 {
		t.Errorf("products = %+v, want none for another household's staple", products)
	}
}

// The archived row is what carries the household's own naming, so the answer
// must not lose it on the way out.
func TestGetFrequentlyBoughtProductsHandlerCarriesTheHouseholdsNaming(t *testing.T) {
	env := setupHandlerTest(t)
	categoryKey := "dairy products"
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	for range 2 {
		product := dbModel.Product{
			Barcode:           "4006381333931",
			ProductName:       "Milk",
			PackSize:          "1 l",
			CategoryKey:       &categoryKey,
			StorageLocationID: &location.ID,
			HouseholdID:       env.Household.ID,
			UserID:            env.User.ID,
			Quantity:          1,
			ExpireAt:          time.Now().Add(7 * 24 * time.Hour),
		}
		if err := env.DB.Create(&product).Error; err != nil {
			t.Fatalf("create product: %v", err)
		}
		if err := env.DB.Delete(&product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
	}
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/frequently-bought", nil)

	GetFrequentlyBoughtProducts(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", env.W.Code)
	}
	var products []apiModel.FrequentProduct
	if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("products = %+v, want the one staple", products)
	}
	if products[0].CategoryKey == nil || *products[0].CategoryKey != categoryKey {
		t.Errorf("category = %v, want %q", products[0].CategoryKey, categoryKey)
	}
	if products[0].StorageLocationID == nil || *products[0].StorageLocationID != location.ID {
		t.Errorf("storage location = %v, want %d", products[0].StorageLocationID, location.ID)
	}
}
