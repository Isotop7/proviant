package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
)

func seedCategoryPrices(t *testing.T, env *handlerTestEnv) {
	t.Helper()
	for _, row := range []dbModel.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
	} {
		if err := env.DB.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
}

func TestListHouseholdCategoriesReturnsTheCanonicalList(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/categories", nil)
	ListHouseholdCategories(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", env.W.Code)
	}
	var categories []map[string]any
	if err := json.Unmarshal(env.W.Body.Bytes(), &categories); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("returned %d categories, want 2", len(categories))
	}
	if categories[0]["categoryKey"] != "dairy products" {
		t.Errorf("first category = %v, want dairy products", categories[0]["categoryKey"])
	}
}

func TestUpdateHouseholdCategoryStoresTheOverrides(t *testing.T) {
	categoryKey := "milk"
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"displayName":"Our milk","sortOrder":3,"hidden":true,"defaultShelfLifeDays":5,"defaultDaysAfterOpening":2}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: categoryKey}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", env.W.Code, env.W.Body.String())
	}

	var stored dbModel.HouseholdCategory
	if err := env.DB.Where("household_id = ? AND category_key = ?", env.Household.ID, "milk").First(&stored).Error; err != nil {
		t.Fatalf("read the stored override: %v", err)
	}
	if stored.DisplayName != "Our milk" {
		t.Errorf("display name = %q, want Our milk", stored.DisplayName)
	}
	if !stored.Hidden {
		t.Error("the category is not hidden")
	}
	if stored.DefaultShelfLifeDays == nil || *stored.DefaultShelfLifeDays != 5 {
		t.Errorf("shelf life = %v, want 5", stored.DefaultShelfLifeDays)
	}
}

func TestUpdateHouseholdCategoryRefusesAnUnknownCategory(t *testing.T) {
	categoryKey := "unicorns"
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"displayName":"Unicorns"}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/unicorns", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: categoryKey}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", env.W.Code)
	}
}

func TestUpdateHouseholdCategoryRefusesAnOutOfRangeShelfLife(t *testing.T) {
	categoryKey := "milk"
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"defaultShelfLifeDays":99999}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: categoryKey}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", env.W.Code)
	}
}

// A patch that names one field leaves the others alone: a shelf life must not
// wipe the household's display name or unhide the category.
func TestUpdateHouseholdCategoryKeepsTheUnmentionedValues(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	first := `{"displayName":"Our milk","hidden":true,"defaultShelfLifeDays":5}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(first))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: "milk"}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)
	if env.W.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200: %s", env.W.Code, env.W.Body.String())
	}

	second := `{"defaultShelfLifeDays":9}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(second))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: "milk"}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)
	if env.W.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200: %s", env.W.Code, env.W.Body.String())
	}

	var stored dbModel.HouseholdCategory
	if err := env.DB.Where("household_id = ? AND category_key = ?", env.Household.ID, "milk").First(&stored).Error; err != nil {
		t.Fatalf("read the stored override: %v", err)
	}
	if stored.DisplayName != "Our milk" {
		t.Errorf("display name = %q, want Our milk", stored.DisplayName)
	}
	if !stored.Hidden {
		t.Error("the category is no longer hidden")
	}
	if stored.DefaultShelfLifeDays == nil || *stored.DefaultShelfLifeDays != 9 {
		t.Errorf("shelf life = %v, want 9", stored.DefaultShelfLifeDays)
	}
}

// A cleared shelf life leaves the products of the category without a date: a
// shelf life of zero days would expire them on the day they were bought.
func TestUpdateHouseholdCategoryClearedShelfLifeDoesNotExpireTheProducts(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"defaultShelfLifeDays":0}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: "milk"}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", env.W.Code, env.W.Body.String())
	}

	product := dbModel.Product{Barcode: "1234567890123", ProductName: "Milk", Categories: "milk", Quantity: 1}
	if !applyHouseholdCategory(env.Ctx, env.AppCtx, &product) {
		t.Fatal("applyHouseholdCategory refused the product")
	}
	if !product.ExpireAt.IsZero() {
		t.Errorf("expire at = %v, want no date at all", product.ExpireAt)
	}
}

// A category the household gave a shelf life fills the date of a product that
// names no date.
func TestApplyHouseholdCategoryFillsTheShelfLife(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"defaultShelfLifeDays":5}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: "milk"}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	product := dbModel.Product{Barcode: "1234567890123", ProductName: "Milk", Categories: "milk", Quantity: 1}
	if !applyHouseholdCategory(env.Ctx, env.AppCtx, &product) {
		t.Fatal("applyHouseholdCategory refused the product")
	}
	if product.ExpireAt.IsZero() {
		t.Fatal("the product has no expiry date")
	}
	if !product.ExpireAt.After(time.Now().AddDate(0, 0, 4)) {
		t.Errorf("expire at = %v, want about five days from now", product.ExpireAt)
	}
}

func TestReorderHouseholdCategoriesWritesTheOrder(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)

	body := `{"keys":["milk","dairy products"]}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/household/categories/order", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	ReorderHouseholdCategories(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", env.W.Code, env.W.Body.String())
	}

	var stored dbModel.HouseholdCategory
	if err := env.DB.Where("household_id = ? AND category_key = ?", env.Household.ID, "milk").First(&stored).Error; err != nil {
		t.Fatalf("read the stored override: %v", err)
	}
	if stored.SortOrder != 0 {
		t.Errorf("sort order = %d, want 0", stored.SortOrder)
	}
}
