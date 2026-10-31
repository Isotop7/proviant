package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/testutil"
)

// TestParseProductSortQueryDefaultsToTheEffectiveExpiry pins the default of
// the product list: the effective expiry, the order the endpoint has always
// returned. Every path has to agree on it — a request that never reads the
// query must not fall back to the repository's own default, which orders by
// name.
func TestParseProductSortQueryDefaultsToTheEffectiveExpiry(t *testing.T) {
	t.Run("a request without a sort", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

		q, ok := ParseProductSortQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSortQuery() = false, want true")
		}
		if q.Sort != database.SortEffectiveExpiry {
			t.Errorf("sort = %q, want %q", q.Sort, database.SortEffectiveExpiry)
		}
		if q.Order != "asc" {
			t.Errorf("order = %q, want %q", q.Order, "asc")
		}
	})

	t.Run("a request without a URL", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		ctx.Request.URL = nil

		q, ok := ParseProductSortQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSortQuery() = false, want true")
		}
		if q.Sort != database.SortEffectiveExpiry {
			t.Errorf("sort = %q, want %q", q.Sort, database.SortEffectiveExpiry)
		}
		if q.Order != "asc" {
			t.Errorf("order = %q, want %q", q.Order, "asc")
		}
	})

	t.Run("a named sort survives", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=product_name&order=desc", nil)

		q, ok := ParseProductSortQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSortQuery() = false, want true")
		}
		if q.Sort != "product_name" || q.Order != "desc" {
			t.Errorf("sort/order = %q/%q, want product_name/desc", q.Sort, q.Order)
		}
	})

	t.Run("an unknown sort is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=category", nil)

		if _, ok := ParseProductSortQuery(ctx); ok {
			t.Error("ParseProductSortQuery() = true, want false for a sort outside the allowlist")
		}
	})
}

// TestParseProductListQueryWindow pins the window a product list is read in:
// the page size, the offset and the default of each.
func TestParseProductListQueryWindow(t *testing.T) {
	t.Run("a request without a window", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

		q, ok := ParseProductListQuery(ctx)

		if !ok {
			t.Fatal("ParseProductListQuery() = false, want true")
		}
		if q.Limit != 100 {
			t.Errorf("limit = %d, want 100", q.Limit)
		}
		if q.Offset != 0 {
			t.Errorf("offset = %d, want 0", q.Offset)
		}
	})

	t.Run("a named window survives", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=25&offset=50", nil)

		q, ok := ParseProductListQuery(ctx)

		if !ok {
			t.Fatal("ParseProductListQuery() = false, want true")
		}
		if q.Limit != 25 || q.Offset != 50 {
			t.Errorf("limit/offset = %d/%d, want 25/50", q.Limit, q.Offset)
		}
	})

	t.Run("a negative offset is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?offset=-1", nil)

		if _, ok := ParseProductListQuery(ctx); ok {
			t.Error("ParseProductListQuery() = true, want false for a negative offset")
		}
	})

	t.Run("an offset beyond the pantry is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?offset=1000001", nil)

		if _, ok := ParseProductListQuery(ctx); ok {
			t.Error("ParseProductListQuery() = true, want false for an offset beyond the bound")
		}
	})
}

// TestParseProductSearchQueryWindow pins the window of a search: without a
// limit it answers with every match, and the offset skips matches.
func TestParseProductSearchQueryWindow(t *testing.T) {
	t.Run("a search without a limit", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk", nil)

		q, ok := ParseProductSearchQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSearchQuery() = false, want true")
		}
		if q.Limit != 0 {
			t.Errorf("limit = %d, want 0 — a search without a limit returns every match", q.Limit)
		}
		if q.Offset != 0 {
			t.Errorf("offset = %d, want 0", q.Offset)
		}
	})

	t.Run("a named window survives", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk&limit=10&offset=20", nil)

		q, ok := ParseProductSearchQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSearchQuery() = false, want true")
		}
		if q.Limit != 10 || q.Offset != 20 {
			t.Errorf("limit/offset = %d/%d, want 10/20", q.Limit, q.Offset)
		}
	})

	t.Run("a limit beyond the bound is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk&limit=1001", nil)

		if _, ok := ParseProductSearchQuery(ctx); ok {
			t.Error("ParseProductSearchQuery() = true, want false for a limit beyond the bound")
		}
	})

	t.Run("a limit of zero means every match", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk&limit=0", nil)

		q, ok := ParseProductSearchQuery(ctx)

		if !ok {
			t.Fatal("ParseProductSearchQuery() = false, want true")
		}
		if q.Limit != 0 {
			t.Errorf("limit = %d, want 0 — a search without a limit returns every match", q.Limit)
		}
	})

	t.Run("a negative limit is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk&limit=-1", nil)

		if _, ok := ParseProductSearchQuery(ctx); ok {
			t.Error("ParseProductSearchQuery() = true, want false for a negative limit")
		}
	})

	t.Run("an offset beyond the pantry is refused", func(t *testing.T) {
		ctx, _ := testutil.SetupGinContext(nil)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search?queryParam=product_name&queryValue=Milk&offset=1000001", nil)

		if _, ok := ParseProductSearchQuery(ctx); ok {
			t.Error("ParseProductSearchQuery() = true, want false for an offset beyond the bound")
		}
	})
}
