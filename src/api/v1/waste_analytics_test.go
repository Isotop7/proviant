package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestGetWasteAnalytics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user without household returns zeroed response", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 0
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CO2Source == "" {
			t.Error("CO2Source must be set even with no household")
		}
		if resp.Monthly == nil || resp.Trend == nil {
			t.Error("Monthly and Trend must be non-nil empty slices")
		}
	})

	t.Run("invalid period returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 42
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?period=garbage", nil)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("real DB: aggregates consumed vs wasted", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		prod := dbModel.Product{
			ProductName: "Yogurt", Categories: "en:dairy", HouseholdID: hh.ID, UserID: admin.ID,
			ExpireAt: time.Now(), Amount: 1,
		}
		db.Create(&prod)
		db.Create(&dbModel.SavingsRecord{
			HouseholdID: hh.ID, ProductID: prod.ID, ProductName: "Yogurt",
			EventType: "consumed", PriceEUR: 1.5, CO2Kg: 0.5, Amount: 1,
		})
		db.Create(&dbModel.SavingsRecord{
			HouseholdID: hh.ID, ProductID: prod.ID, ProductName: "Yogurt",
			EventType: "wasted", PriceEUR: 2.0, CO2Kg: 0.8, Amount: 1,
		})

		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?period=6months", nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ConsumedCount != 1 || resp.WastedCount != 1 {
			t.Errorf("counts = (%d, %d), want (1, 1)", resp.ConsumedCount, resp.WastedCount)
		}
		if resp.WastedEUR != 2.0 {
			t.Errorf("wasted EUR = %v, want 2.0", resp.WastedEUR)
		}
		if len(resp.Monthly) == 0 {
			t.Error("expected non-empty monthly breakdown")
		}
	})

	t.Run("period query param is echoed", func(t *testing.T) {
		// Real DB path; period is parsed and reflected in the response.
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		ctx, w := repomocks.SetupGinContextWithDB(db)
		q := url.Values{}
		q.Set("period", "month")
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?"+q.Encode(), nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Period != "month" {
			t.Errorf("Period = %q, want month", resp.Period)
		}
		if len(resp.Monthly) != 1 {
			t.Errorf("Monthly length = %d, want 1", len(resp.Monthly))
		}
	})

	t.Run("sort=cost returns categories sorted by EUR desc", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		// Dairy: 1 wasted 5 EUR; Bakery: 2 wasted at 0.5 EUR each (total 1 EUR).
		dairy := dbModel.Product{ProductName: "Cheese", Categories: "en:dairy", HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
		db.Create(&dairy)
		bakery := dbModel.Product{ProductName: "Bread", Categories: "en:bakery", HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
		db.Create(&bakery)
		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "dairy", DisplayName: "Dairy", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})
		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "bakery", DisplayName: "Bakery", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})
		db.Create(&dbModel.SavingsRecord{HouseholdID: hh.ID, ProductID: dairy.ID, ProductName: "Cheese", EventType: "wasted", PriceEUR: 5, CO2Kg: 1, Amount: 1})
		db.Create(&dbModel.SavingsRecord{HouseholdID: hh.ID, ProductID: bakery.ID, ProductName: "Bread", EventType: "wasted", PriceEUR: 0.5, CO2Kg: 0.2, Amount: 1})
		db.Create(&dbModel.SavingsRecord{HouseholdID: hh.ID, ProductID: bakery.ID, ProductName: "Bread", EventType: "wasted", PriceEUR: 0.5, CO2Kg: 0.2, Amount: 1})

		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?sort=cost", nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Sort != "cost" {
			t.Errorf("Sort = %q, want cost", resp.Sort)
		}
		if len(resp.MostWastedCategories) < 2 {
			t.Fatalf("expected >=2 categories, got %d", len(resp.MostWastedCategories))
		}
		if resp.MostWastedCategories[0].CategoryKey != "dairy" {
			t.Errorf("top by cost = %q, want dairy", resp.MostWastedCategories[0].CategoryKey)
		}
	})

	t.Run("sort=garbage returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 42
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?sort=garbage", nil)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("period=12months returns 12 monthly rows", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?period=12months", nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Period != "12months" {
			t.Errorf("Period = %q, want 12months", resp.Period)
		}
		if len(resp.Monthly) != 12 {
			t.Errorf("Monthly length = %d, want 12", len(resp.Monthly))
		}
	})

	t.Run("MostWastedCategories[0].Products is populated and capped", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		dairy := dbModel.Product{ProductName: "Yogurt", Categories: "en:dairy", HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
		db.Create(&dairy)
		cheese := dbModel.Product{ProductName: "Cheese", Categories: "en:dairy", HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
		db.Create(&cheese)
		milk := dbModel.Product{ProductName: "Milk", Categories: "en:dairy", HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
		db.Create(&milk)
		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "dairy", DisplayName: "Dairy", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})

		insertWasted := func(name string, pid uint, eur float64) {
			rec := dbModel.SavingsRecord{HouseholdID: hh.ID, ProductID: pid, ProductName: name, EventType: "wasted", PriceEUR: eur, CO2Kg: 0.1, Amount: 1}
			db.Create(&rec)
		}
		// 3 wasted yogurts, 2 wasted cheese, 1 wasted milk → top 3 by cap=3, all included.
		insertWasted("Yogurt", dairy.ID, 1.0)
		insertWasted("Yogurt", dairy.ID, 1.0)
		insertWasted("Yogurt", dairy.ID, 1.0)
		insertWasted("Cheese", cheese.ID, 2.0)
		insertWasted("Cheese", cheese.ID, 2.0)
		insertWasted("Milk", milk.ID, 0.5)

		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste", nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.MostWastedCategories) != 1 {
			t.Fatalf("expected 1 category, got %d", len(resp.MostWastedCategories))
		}
		prods := resp.MostWastedCategories[0].Products
		if len(prods) != 3 {
			t.Fatalf("Products length = %d, want 3 (cap=3)", len(prods))
		}
		if prods[0].ProductName != "Yogurt" || prods[0].Count != 3 {
			t.Errorf("top product = %+v, want Yogurt count 3", prods[0])
		}
		if prods[1].ProductName != "Cheese" || prods[1].Count != 2 {
			t.Errorf("2nd product = %+v, want Cheese count 2", prods[1])
		}
		if prods[2].ProductName != "Milk" || prods[2].Count != 1 {
			t.Errorf("3rd product = %+v, want Milk count 1", prods[2])
		}
	})

	t.Run("limit=100 returns 400 (over max 50)", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 42
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?limit=100", nil)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, 1)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("limit query param caps categories returned", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "dairy", DisplayName: "Dairy", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})
		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "bakery", DisplayName: "Bakery", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})
		db.Create(&dbModel.ProductCategoryPrice{CategoryKey: "snacks", DisplayName: "Snacks", AvgPriceEUR: 1, CO2KgPerKg: 1, WeightGrams: 100})

		mkProd := func(name, cat string) dbModel.Product {
			p := dbModel.Product{ProductName: name, Categories: "en:" + cat, HouseholdID: hh.ID, UserID: admin.ID, ExpireAt: time.Now(), Amount: 1}
			db.Create(&p)
			return p
		}
		dp := mkProd("Cheese", "dairy")
		bp := mkProd("Bread", "bakery")
		sp := mkProd("Chips", "snacks")
		for _, ps := range []dbModel.Product{dp, bp, sp} {
			db.Create(&dbModel.SavingsRecord{HouseholdID: hh.ID, ProductID: ps.ID, ProductName: ps.ProductName, EventType: "wasted", PriceEUR: 1, CO2Kg: 0.1, Amount: 1})
		}

		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats/waste?limit=2", nil)
		testutil.MockJWTClaimsWithKey(ctx, admin.ID, testutil.TokenIdentityKey)
		appCtx := SetupTestAppContext(ctx, admin.ID)

		GetWasteAnalytics(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var resp api.WasteAnalyticsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.MostWastedCategories) != 2 {
			t.Errorf("MostWastedCategories length = %d, want 2 (limit=2)", len(resp.MostWastedCategories))
		}
	})
}
