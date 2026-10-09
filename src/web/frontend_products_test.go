package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func candidateRows(n int) []dbModel.Product {
	products := make([]dbModel.Product, n)
	for i := range products {
		products[i].ID = uint(i + 1)
	}
	return products
}

func TestSliceProductPage(t *testing.T) {
	t.Run("empty list yields one empty page", func(t *testing.T) {
		ids, page, totalPages := sliceProductPage(nil, "")
		if len(ids) != 0 {
			t.Errorf("len(ids) = %d, want 0", len(ids))
		}
		if page != 1 || totalPages != 1 {
			t.Errorf("page, totalPages = %d, %d; want 1, 1", page, totalPages)
		}
	})

	t.Run("unusable page numbers fall back to the first page", func(t *testing.T) {
		for _, raw := range []string{"", "0", "-3", "not-a-number", "1.5"} {
			_, page, _ := sliceProductPage(candidateRows(productsPageSize+10), raw)
			if page != 1 {
				t.Errorf("page for %q = %d, want 1", raw, page)
			}
		}
	})

	t.Run("list smaller than one page has a single page", func(t *testing.T) {
		ids, page, totalPages := sliceProductPage(candidateRows(productsPageSize), "7")
		if page != 1 || totalPages != 1 {
			t.Errorf("page, totalPages = %d, %d; want 1, 1", page, totalPages)
		}
		if len(ids) != productsPageSize {
			t.Errorf("len(ids) = %d, want %d", len(ids), productsPageSize)
		}
	})

	t.Run("page beyond the end clamps to the last page", func(t *testing.T) {
		total := productsPageSize*2 + 7
		ids, page, totalPages := sliceProductPage(candidateRows(total), "99")
		if totalPages != 3 {
			t.Errorf("totalPages = %d, want 3", totalPages)
		}
		if page != 3 {
			t.Errorf("page = %d, want 3", page)
		}
		if len(ids) != 7 {
			t.Errorf("len(ids) = %d, want 7", len(ids))
		}
		if ids[0] != uint(productsPageSize*2+1) {
			t.Errorf("first id = %d, want %d", ids[0], productsPageSize*2+1)
		}
	})

	t.Run("a middle page returns exactly its own rows in order", func(t *testing.T) {
		candidates := candidateRows(productsPageSize*2 + 7)
		ids, page, totalPages := sliceProductPage(candidates, "2")
		if page != 2 || totalPages != 3 {
			t.Errorf("page, totalPages = %d, %d; want 2, 3", page, totalPages)
		}
		if len(ids) != productsPageSize {
			t.Fatalf("len(ids) = %d, want %d", len(ids), productsPageSize)
		}
		for i, id := range ids {
			if want := candidates[productsPageSize+i].ID; id != want {
				t.Errorf("ids[%d] = %d, want %d", i, id, want)
			}
		}
	})
}

func TestPageURL(t *testing.T) {
	params := map[string][]string{
		"status": {"expired"},
		"sort":   {"expire_at"},
	}

	got := pageURL(params, 3)
	for _, want := range []string{"status=expired", "sort=expire_at", "page=3"} {
		if !strings.Contains(got, want) {
			t.Errorf("pageURL() = %q, missing %q", got, want)
		}
	}
	if _, ok := params["page"]; ok {
		t.Errorf("pageURL() mutated the input params")
	}
}

func TestProductsHandlerPagesTheList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	total := productsPageSize + 10
	for i := 0; i < total; i++ {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.ProductName = productNameFor(i)
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("failed to save product: %v", err)
		}
	}

	cache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	frontend := &Frontend{TemplateCache: cache}

	render := func(target string) *httptest.ResponseRecorder {
		t.Helper()
		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
		testutil.MockJWTClaimsWithKey(ctx, user.ID, static.TokenIdentityKey)
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
		frontend.Products(ctx)
		return w
	}

	t.Run("second page renders its own rows and the pager", func(t *testing.T) {
		w := render("/web/products?page=2")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !strings.Contains(body, "Page 2 of") {
			t.Errorf("body missing the pager status, want \"Page 2 of\"")
		}
		if !strings.Contains(body, productNameFor(50)) {
			t.Errorf("page 2 missing its first row (%s)", productNameFor(50))
		}
		if strings.Contains(body, productNameFor(0)) {
			t.Errorf("page 2 rendered a first-page row")
		}
	})

	t.Run("an out-of-range page falls back to the last page", func(t *testing.T) {
		w := render("/web/products?page=999")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Page 2 of") {
			t.Errorf("body missing \"Page 2 of\" for a clamped page")
		}
	})

	t.Run("first page links forward only", func(t *testing.T) {
		w := render("/web/products")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !strings.Contains(body, "Page 1 of 2") {
			t.Errorf("body missing \"Page 1 of 2\"")
		}
		if strings.Contains(body, `rel="prev"`) {
			t.Errorf("first page rendered a Previous link")
		}
		if !strings.Contains(body, `rel="next"`) {
			t.Errorf("first page missing its Next link")
		}
	})

	t.Run("a sort outside the allowlist is rejected", func(t *testing.T) {
		w := render("/web/products?queryParam=product_name&queryValue=Product&sort=evil")
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

// TestProductsHandlerOmitsPagerOnASinglePage keeps the pager out of the way
// when one page holds the whole list: no navigation chrome for a list that has
// nowhere else to go.
func TestProductsHandlerOmitsPagerOnASinglePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	for i := 0; i < productsPageSize-10; i++ {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.ProductName = productNameFor(i)
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("failed to save product: %v", err)
		}
	}

	cache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}

	ctx, w := repomocks.SetupGinContextWithDB(db)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/web/products", nil)
	testutil.MockJWTClaimsWithKey(ctx, user.ID, static.TokenIdentityKey)
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})

	(&Frontend{TemplateCache: cache}).Products(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "products-pagination") {
		t.Errorf("single-page list rendered a pager")
	}
}

func productNameFor(i int) string {
	return fmt.Sprintf("Product %03d", i)
}

func TestHydrateProductsPage(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repos := database.NewRepositoryContainer(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	names := []string{"Alpha", "Beta", "Gamma"}
	created := make([]dbModel.Product, 0, len(names))
	for _, name := range names {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.ProductName = name
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("failed to save product: %v", err)
		}
		created = append(created, *product)
	}

	t.Run("returns full rows in the requested order", func(t *testing.T) {
		ids := []uint{created[2].ID, created[0].ID, created[1].ID}
		rows, err := hydrateProductsPage(repos, user.ID, ids, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != len(ids) {
			t.Fatalf("len(rows) = %d, want %d", len(rows), len(ids))
		}
		for i, id := range ids {
			if rows[i].ID != id {
				t.Errorf("rows[%d].ID = %d, want %d (hydration reordered the page)", i, rows[i].ID, id)
			}
		}
		if rows[0].ProductName != "Gamma" {
			t.Errorf("rows[0].ProductName = %q, want %q (hydration must return full rows)",
				rows[0].ProductName, "Gamma")
		}
	})

	t.Run("an empty page is not a query", func(t *testing.T) {
		rows, err := hydrateProductsPage(repos, user.ID, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("len(rows) = %d, want 0", len(rows))
		}
	})
}
