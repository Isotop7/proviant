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
}
