package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// productPage reads one page of the product list. Every page needs its own
// context and recorder: a recorder keeps the status code of the first answer.
func productPage(t *testing.T, env *handlerTestEnv, query string) apiModel.ProductListResponse {
	t.Helper()
	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products"+query, nil)

	GetProducts(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var response apiModel.ProductListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response
}

// searchPage reads one page of the product search.
func searchPage(t *testing.T, env *handlerTestEnv, query string) apiModel.ProductListResponse {
	t.Helper()
	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/search"+query, nil)

	SearchProducts(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var response apiModel.ProductListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response
}

// archivedPage reads one page of the archived products.
func archivedPage(t *testing.T, env *handlerTestEnv, query string) apiModel.ProductListResponse {
	t.Helper()
	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/archived"+query, nil)

	GetArchivedProducts(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var response apiModel.ProductListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response
}

// seedProducts creates products that expire one day after the previous one, so
// the order of the list is the order they were created in.
func seedProducts(t *testing.T, env *handlerTestEnv, count int) []*dbModel.Product {
	t.Helper()
	products := make([]*dbModel.Product, 0, count)
	for i := range count {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Product %d", i)
		product.ExpireAt = time.Now().AddDate(0, 0, i+1)
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		products = append(products, product)
	}
	return products
}

// TestGetProductsAnswersWithTheEnvelope pins the wire shape of the list: the
// products of the page, the total and the window. A client reads these names,
// so they are part of the contract.
func TestGetProductsAnswersWithTheEnvelope(t *testing.T) {
	env := setupHandlerTest(t)
	seedProducts(t, env, 2)

	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=1", nil)

	GetProducts(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"products", "total", "limit", "offset"} {
		if _, ok := body[field]; !ok {
			t.Errorf("the body has no %q field: %s", field, w.Body.String())
		}
	}
	if body["total"] != float64(2) {
		t.Errorf("total = %v, want 2 — the total counts the whole set, not the page", body["total"])
	}
	if body["limit"] != float64(1) {
		t.Errorf("limit = %v, want 1", body["limit"])
	}
}

// TestGetProductsEmptyPageIsAnEmptyArray pins that a page without rows answers
// with an empty array: a client walks the page by iterating it, and a null
// would break the iteration.
func TestGetProductsEmptyPageIsAnEmptyArray(t *testing.T) {
	env := setupHandlerTest(t)

	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

	GetProducts(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"products":[]`) {
		t.Errorf("body = %s, want an empty products array", w.Body.String())
	}
}

// TestGetProductsWalksTheWholeSet pins the reason the endpoint takes an
// offset: a household with more products than one page holds has to be
// walkable. Every product comes back exactly once — no row skipped, no row
// repeated.
func TestGetProductsWalksTheWholeSet(t *testing.T) {
	env := setupHandlerTest(t)
	seedProducts(t, env, 5)

	seen := map[uint]bool{}
	total := 0
	for offset := 0; offset < 10; offset += 2 {
		response := productPage(t, env, fmt.Sprintf("?limit=2&offset=%d", offset))
		total = response.Total
		for _, product := range response.Products {
			if seen[product.ID] {
				t.Fatalf("product %d came back on two pages", product.ID)
			}
			seen[product.ID] = true
		}
		if len(response.Products) < 2 {
			break
		}
	}

	if total != 5 {
		t.Errorf("total = %d, want the 5 products of the household", total)
	}
	if len(seen) != 5 {
		t.Errorf("the walk collected %d products, want all 5", len(seen))
	}
}

// TestGetProductsKeepsTheOrderStableAcrossPages pins the order the offset
// walk depends on: products that share the expiry keep the order of their
// ids, so a product cannot land on two pages or on none.
func TestGetProductsKeepsTheOrderStableAcrossPages(t *testing.T) {
	env := setupHandlerTest(t)
	expiry := time.Now().AddDate(0, 0, 3)
	tied := make([]*dbModel.Product, 0, 4)
	for i := range 4 {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Tied %d", i)
		product.ExpireAt = expiry
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		tied = append(tied, product)
	}

	first := productPage(t, env, "?limit=2&offset=0")
	second := productPage(t, env, "?limit=2&offset=2")

	if len(first.Products) != 2 || len(second.Products) != 2 {
		t.Fatalf("pages hold %d and %d products, want 2 and 2", len(first.Products), len(second.Products))
	}
	want := []uint{tied[0].ID, tied[1].ID, tied[2].ID, tied[3].ID}
	got := []uint{first.Products[0].ID, first.Products[1].ID, second.Products[0].ID, second.Products[1].ID}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("product %d of the walk = %d, want %d — the tied products keep the order of their ids", i, got[i], want[i])
		}
	}
}

// TestGetProductsCountsOnlyWhatTheCallerMaySee pins the scope of the count: it
// describes the same set the list returns. A product of another household and
// a private product of another member are in neither the page nor the count.
func TestGetProductsCountsOnlyWhatTheCallerMaySee(t *testing.T) {
	env := setupHandlerTest(t)
	seedProducts(t, env, 2)

	member := env.addMember(t, "member")
	private := testutil.CreateTestProduct(env.DB, env.Household.ID, member.ID)
	private.ProductName = "Member's private product"
	private.IsPrivate = true
	if err := env.DB.Save(private).Error; err != nil {
		t.Fatalf("seed private product: %v", err)
	}

	otherHousehold := testutil.CreateTestHousehold(env.DB, 0)
	otherUser := testutil.CreateTestUser(env.DB, otherHousehold.ID)
	foreign := testutil.CreateTestProduct(env.DB, otherHousehold.ID, otherUser.ID)
	foreign.ProductName = "Foreign product"

	response := productPage(t, env, "?limit=100")

	if response.Total != 2 {
		t.Errorf("total = %d, want the 2 products the caller may see", response.Total)
	}
	if len(response.Products) != 2 {
		t.Fatalf("the page holds %d products, want 2", len(response.Products))
	}
	for _, product := range response.Products {
		if product.ID == private.ID || product.ID == foreign.ID {
			t.Errorf("product %d is in the page, want only the caller's own products", product.ID)
		}
	}
}

// TestGetProductsIdsWinOverTheOffset pins the precedence the Swagger
// annotation documents: the named products come back, unpaged.
func TestGetProductsIdsWinOverTheOffset(t *testing.T) {
	env := setupHandlerTest(t)
	products := seedProducts(t, env, 3)

	response := productPage(t, env, fmt.Sprintf("?ids=%d,%d&limit=1&offset=1", products[0].ID, products[2].ID))

	if len(response.Products) != 2 {
		t.Fatalf("the page holds %d products, want the 2 named ones", len(response.Products))
	}
	if response.Total != 2 {
		t.Errorf("total = %d, want the 2 named products", response.Total)
	}
	if response.Offset != 0 {
		t.Errorf("offset = %d, want 0 — the ids win over the offset", response.Offset)
	}
}

// TestSearchProductsPages pins the paging of the search: it takes a limit and
// an offset and reports how many products matched.
func TestSearchProductsPages(t *testing.T) {
	env := setupHandlerTest(t)
	for i := range 3 {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Milk %d", i)
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
	}
	unrelated := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	unrelated.ProductName = "Bread"
	if err := env.DB.Save(unrelated).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}

	first := searchPage(t, env, "?queryParam=product_name&queryValue=Milk&limit=2&offset=0")
	second := searchPage(t, env, "?queryParam=product_name&queryValue=Milk&limit=2&offset=2")

	if first.Total != 3 || second.Total != 3 {
		t.Errorf("totals = %d and %d, want the 3 matches", first.Total, second.Total)
	}
	if len(first.Products) != 2 || len(second.Products) != 1 {
		t.Fatalf("pages hold %d and %d matches, want 2 and 1", len(first.Products), len(second.Products))
	}
	for _, product := range second.Products {
		if product.ProductName == unrelated.ProductName {
			t.Errorf("the page holds %q, want only the matches", product.ProductName)
		}
	}
}

// TestSearchProductsPagesByEffectiveExpiry pins the paging of a search ordered
// by the effective expiry: the matches are resolved before the page is cut out,
// because the effective expiry has no sortable column.
func TestSearchProductsPagesByEffectiveExpiry(t *testing.T) {
	env := setupHandlerTest(t)
	now := time.Now()
	for i := range 3 {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Milk %d", i)
		product.ExpireAt = now.AddDate(0, 0, i+1)
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
	}

	first := searchPage(t, env, "?queryParam=product_name&queryValue=Milk&sort=effective_expire_at&limit=2&offset=0")
	second := searchPage(t, env, "?queryParam=product_name&queryValue=Milk&sort=effective_expire_at&limit=2&offset=2")

	if first.Total != 3 || second.Total != 3 {
		t.Errorf("totals = %d and %d, want the 3 matches", first.Total, second.Total)
	}
	if len(first.Products) != 2 || len(second.Products) != 1 {
		t.Fatalf("pages hold %d and %d matches, want 2 and 1", len(first.Products), len(second.Products))
	}
	if !first.Products[0].ExpireAt.Before(first.Products[1].ExpireAt) {
		t.Errorf("the first page is not ordered by the expiry: %v then %v", first.Products[0].ExpireAt, first.Products[1].ExpireAt)
	}
	if !first.Products[1].ExpireAt.Before(second.Products[0].ExpireAt) {
		t.Errorf("the second page continues the order: %v then %v", first.Products[1].ExpireAt, second.Products[0].ExpireAt)
	}
}

// TestSearchProductsWithoutALimitReturnsEveryMatch pins the default of the
// search: without a limit it answers with every match, as it did before it
// could be paged.
func TestSearchProductsWithoutALimitReturnsEveryMatch(t *testing.T) {
	env := setupHandlerTest(t)
	for i := range 3 {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Milk %d", i)
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
	}

	response := searchPage(t, env, "?queryParam=product_name&queryValue=Milk")

	if len(response.Products) != 3 {
		t.Errorf("the page holds %d matches, want all 3", len(response.Products))
	}
	if response.Total != 3 {
		t.Errorf("total = %d, want the 3 matches", response.Total)
	}
}

// TestGetArchivedProductsPages pins the paging of the archived list: the
// archived rows are ordered by their id, so a walk over them reads every one
// exactly once.
func TestGetArchivedProductsPages(t *testing.T) {
	env := setupHandlerTest(t)
	archived := make([]*dbModel.Product, 0, 3)
	for i := range 3 {
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = fmt.Sprintf("Archived %d", i)
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		if err := env.DB.Delete(product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
		archived = append(archived, product)
	}

	first := archivedPage(t, env, "?limit=2&offset=0")
	second := archivedPage(t, env, "?limit=2&offset=2")

	if first.Total != 3 || second.Total != 3 {
		t.Errorf("totals = %d and %d, want the 3 archived products", first.Total, second.Total)
	}
	if len(first.Products) != 2 || len(second.Products) != 1 {
		t.Fatalf("pages hold %d and %d products, want 2 and 1", len(first.Products), len(second.Products))
	}
	want := []uint{archived[0].ID, archived[1].ID, archived[2].ID}
	got := []uint{first.Products[0].ID, first.Products[1].ID, second.Products[0].ID}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("archived product %d = %d, want %d — the archived rows are ordered by their id", i, got[i], want[i])
		}
	}
}
