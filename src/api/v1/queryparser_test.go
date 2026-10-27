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
