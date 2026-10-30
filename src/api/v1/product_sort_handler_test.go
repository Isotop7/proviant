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
)

// TestGetProductsSort covers the sort parameter of the product list: the
// effective expiry is the list's order, and a caller may name it or ask for
// another column.
func TestGetProductsSort(t *testing.T) {
	seed := func(t *testing.T, env *handlerTestEnv) (soon, later *dbModel.Product) {
		t.Helper()
		now := time.Now()
		soon = testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		soon.ProductName = "Soon"
		soon.ExpireAt = now.AddDate(0, 0, 2)
		env.DB.Save(soon)
		later = testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		later.ProductName = "Later"
		later.ExpireAt = now.AddDate(0, 0, 20)
		env.DB.Save(later)
		return soon, later
	}

	decode := func(t *testing.T, env *handlerTestEnv) apiModel.ProductListResponse {
		t.Helper()
		var response apiModel.ProductListResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return response
	}

	t.Run("the effective expiry orders the list by default", func(t *testing.T) {
		env := setupHandlerTest(t)
		soon, _ := seed(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

		GetProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		response := decode(t, env)
		if len(response.Products) != 2 {
			t.Fatalf("len(products) = %d, want 2", len(response.Products))
		}
		if response.Products[0].ID != soon.ID {
			t.Errorf("products[0] = %d, want the product that expires first (%d)", response.Products[0].ID, soon.ID)
		}
	})

	t.Run("the effective expiry can be named", func(t *testing.T) {
		env := setupHandlerTest(t)
		soon, _ := seed(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=effective_expire_at&order=asc", nil)

		GetProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		response := decode(t, env)
		if len(response.Products) != 2 || response.Products[0].ID != soon.ID {
			t.Errorf("products = %+v, want the soonest product first", response.Products)
		}
	})

	t.Run("another column is honoured", func(t *testing.T) {
		env := setupHandlerTest(t)
		_, later := seed(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=product_name&order=asc", nil)

		GetProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		response := decode(t, env)
		if len(response.Products) != 2 || response.Products[0].ID != later.ID {
			t.Errorf("products = %+v, want the product named 'Later' first", response.Products)
		}
	})

	t.Run("an unknown sort is refused", func(t *testing.T) {
		env := setupHandlerTest(t)
		seed(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=category", nil)

		GetProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
		}
	})

	t.Run("the limit cuts the head of the effective expiry order", func(t *testing.T) {
		env := setupHandlerTest(t)
		soon, _ := seed(t, env)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=effective_expire_at&limit=1", nil)

		GetProducts(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		response := decode(t, env)
		if len(response.Products) != 1 {
			t.Fatalf("len(products) = %d, want 1", len(response.Products))
		}
		if response.Products[0].ID != soon.ID {
			t.Errorf("products[0] = %d, want the soonest product (%d)", response.Products[0].ID, soon.ID)
		}
	})
}

// TestSearchProductsSortsByEffectiveExpiry pins that a search may be ordered by
// the effective expiry: the search field narrows the pantry, the sort key
// decides in which order the matches come back.
func TestSearchProductsSortsByEffectiveExpiry(t *testing.T) {
	env := setupHandlerTest(t)
	now := time.Now()

	later := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	later.ProductName = "Milk Later"
	later.ExpireAt = now.AddDate(0, 0, 20)
	env.DB.Save(later)

	soon := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	soon.ProductName = "Milk Soon"
	soon.ExpireAt = now.AddDate(0, 0, 2)
	env.DB.Save(soon)

	env.Ctx.Request = httptest.NewRequest(http.MethodGet,
		"/api/v1/products/search?queryParam=product_name&queryValue=Milk&sort=effective_expire_at&order=asc", nil)

	SearchProducts(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var response apiModel.ProductListResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Products) != 2 {
		t.Fatalf("len(products) = %d, want the two matches", len(response.Products))
	}
	if response.Products[0].ID != soon.ID {
		t.Errorf("products[0] = %d, want the match that expires first (%d)", response.Products[0].ID, soon.ID)
	}
}
