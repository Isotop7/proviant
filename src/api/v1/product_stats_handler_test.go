package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestGetExpiredHandler(t *testing.T) {
	t.Run("returns expired products", func(t *testing.T) {
		env := setupHandlerTest(t)
		expired := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		expired.ExpireAt = time.Now().Add(-48 * time.Hour)
		env.DB.Save(expired)
		future := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		future.ExpireAt = time.Now().Add(48 * time.Hour)
		env.DB.Save(future)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/expired", nil)

		GetExpired(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var products []dbModel.Product
		if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(products) != 1 || products[0].ID != expired.ID {
			t.Errorf("products = %+v, want only past-expired product %d (future product %d excluded)",
				products, expired.ID, future.ID)
		}
	})

	t.Run("empty when nothing matches", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/expired", nil)

		GetExpired(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
	})
}

func TestGetProductSummaryHandler(t *testing.T) {
	env := setupHandlerTest(t)
	soon := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	soon.ExpireAt = time.Now().Add(3 * 24 * time.Hour)
	env.DB.Save(soon)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/summary", nil)

	GetProductSummary(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var resp apiModel.ProductSummaryResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalActive != 1 {
		t.Errorf("totalActive = %d, want 1", resp.TotalActive)
	}
	if resp.ExpiringSoonCount != 1 {
		t.Errorf("expiringSoonCount = %d, want 1", resp.ExpiringSoonCount)
	}
}

func TestGetProductStatsHandler(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	product.Categories = "Dairy"
	product.ExpireAt = time.Now().Add(3 * 24 * time.Hour)
	env.DB.Save(product)
	archived := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.DB.Delete(archived)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/stats", nil)

	GetProductStats(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var resp apiModel.ProductStatsResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalActive != 1 {
		t.Errorf("totalActive = %d, want 1", resp.TotalActive)
	}
	if resp.TotalArchived != 1 {
		t.Errorf("totalArchived = %d, want 1", resp.TotalArchived)
	}
	if resp.UniqueArchived != 1 {
		t.Errorf("uniqueArchived = %d, want 1", resp.UniqueArchived)
	}
	if resp.LastInsertedProduct == "" {
		t.Errorf("lastInsertedProduct empty, want a product name")
	}
	if resp.ExpiringSoonDays != 7 {
		t.Errorf("expiringSoonDays = %d, want 7", resp.ExpiringSoonDays)
	}
	if len(resp.ExpiringSoon) != 1 || resp.ExpiringSoon[0].ProductName != product.ProductName {
		t.Errorf("expiringSoon = %+v, want %q", resp.ExpiringSoon, product.ProductName)
	}
	if resp.Categories["Dairy"] != 1 {
		t.Errorf("categories = %v, want Dairy:1", resp.Categories)
	}
	if len(resp.ExpiryTrend) == 0 {
		t.Errorf("expiryTrend empty, want 12 months")
	}
}

func TestGetProductStatsUsesNotificationThreshold(t *testing.T) {
	env := setupHandlerTest(t)
	env.User.NotificationPreferences.NotificationThresholdDays = 14
	env.DB.Save(env.User)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/stats", nil)

	GetProductStats(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var resp apiModel.ProductStatsResponse
	_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
	if resp.ExpiringSoonDays != 14 {
		t.Errorf("expiringSoonDays = %d, want 14", resp.ExpiringSoonDays)
	}
}

func TestGetLastInsertedProductName(t *testing.T) {
	t.Run("empty without household", func(t *testing.T) {
		env := setupHandlerTest(t)
		lonely := testutil.CreateTestUser(env.DB, 0)
		if got := getLastInsertedProductName(env.AppCtx.Repos, lonely.ID); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns latest product name", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if got := getLastInsertedProductName(env.AppCtx.Repos, env.User.ID); got != product.ProductName {
			t.Errorf("got %q, want %q", got, product.ProductName)
		}
	})
}
