package v1

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newConsumptionAppCtx(db *gorm.DB, userID uint) *AppContext {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db)
	return &AppContext{
		Logger:      &logger,
		DB:          db,
		Repos:       repos,
		UserID:      userID,
		Products:    services.NewProductService(repos, &logger),
		Consumption: services.NewConsumptionService(repos, &logger),
	}
}

func TestGetConsumptionRate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("rate available for product with sufficient history", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Milk", Barcode: "1111111111111",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 1, Unit: "L",
			ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)

		now := time.Now()
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "1111111111111", "Milk", "L", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "1111111111111", "Milk", "L", 1, now.Add(-7*24*time.Hour))

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetConsumptionRate(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp api.ConsumptionRateResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.HasEstimate {
			t.Errorf("HasEstimate = false, want true")
		}
		if resp.SampleCount != 2 {
			t.Errorf("SampleCount = %d, want 2", resp.SampleCount)
		}
		if resp.Unit != "L" {
			t.Errorf("Unit = %q, want L", resp.Unit)
		}
		if resp.Display == "" {
			t.Errorf("Display = empty, want non-empty")
		}
	})

	t.Run("insufficient history returns HasEstimate=false", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Yogurt", Barcode: "2222222222222",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 1, Unit: "pcs",
			ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "2222222222222", "Yogurt", "pcs", 1, time.Now().Add(-2*24*time.Hour))

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetConsumptionRate(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.ConsumptionRateResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.HasEstimate {
			t.Errorf("HasEstimate = true, want false (1 sample)")
		}
		if resp.SampleCount != 1 {
			t.Errorf("SampleCount = %d, want 1", resp.SampleCount)
		}
		if resp.Display != "" {
			t.Errorf("Display = %q, want empty when no estimate", resp.Display)
		}
	})

	t.Run("not found product returns 404", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "9999"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetConsumptionRate(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "not-a-number"}}
		appCtx := newConsumptionAppCtx(db, 1)

		GetConsumptionRate(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestGetRestockSuggestion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns rate-based suggestion with perWeekDisplay", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Milk", Barcode: "3333333333333",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 0, Unit: "L",
			ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)

		now := time.Now()
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "3333333333333", "Milk", "L", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "3333333333333", "Milk", "L", 1, now.Add(-7*24*time.Hour))

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetRestockSuggestion(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp api.RestockSuggestionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if !resp.HasSuggestion {
			t.Errorf("HasSuggestion = false, want true")
		}
		if !resp.HasEstimate {
			t.Errorf("HasEstimate = false, want true")
		}
		if resp.Source != "consumption_rate" {
			t.Errorf("Source = %q, want consumption_rate", resp.Source)
		}
		if resp.SuggestedQty < 1 {
			t.Errorf("SuggestedQty = %d, want >= 1", resp.SuggestedQty)
		}
		if resp.ProductName == "" {
			t.Errorf("ProductName = empty, want non-empty")
		}
		if resp.PerWeekDisplay == "" {
			t.Errorf("PerWeekDisplay = empty, want non-empty (server pre-formats the rate)")
		}
	})

	t.Run("falls back to min stock when no estimate and amount < minStock", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Flour", Barcode: "4444444444444",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 1, Unit: "kg",
			MinStockAmount: 3, ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetRestockSuggestion(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.RestockSuggestionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Source != "min_stock" {
			t.Errorf("Source = %q, want min_stock", resp.Source)
		}
		if resp.SuggestedQty != 2 {
			t.Errorf("SuggestedQty = %d, want 2 (minStock - amount)", resp.SuggestedQty)
		}
		if resp.PerWeekDisplay != "" {
			t.Errorf("PerWeekDisplay = %q, want empty for min_stock branch", resp.PerWeekDisplay)
		}
	})

	t.Run("min stock already met returns HasSuggestion=false (no misleading modal)", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Sugar", Barcode: "5555555555555",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 5, Unit: "kg",
			MinStockAmount: 5, ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetRestockSuggestion(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.RestockSuggestionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.HasSuggestion {
			t.Errorf("HasSuggestion = true, want false (minimum already met, no restock needed)")
		}
		if resp.Source != "none" {
			t.Errorf("Source = %q, want none", resp.Source)
		}
		if resp.SuggestedQty != 0 {
			t.Errorf("SuggestedQty = %d, want 0", resp.SuggestedQty)
		}
	})

	t.Run("returns none when nothing to suggest", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		product := dbModel.Product{
			ProductName: "Salt", Barcode: "6666666666666",
			HouseholdID: hh.ID, UserID: admin.ID, Amount: 5, Unit: "g",
			ExpireAt: time.Now().Add(7 * 24 * time.Hour),
		}
		db.Create(&product)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetRestockSuggestion(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp api.RestockSuggestionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.HasSuggestion {
			t.Errorf("HasSuggestion = true, want false")
		}
		if resp.Source != "none" {
			t.Errorf("Source = %q, want none", resp.Source)
		}
	})

	t.Run("not found product returns 404", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
		db.Create(&admin)
		hh := dbModel.Household{Name: "H", AdminID: admin.ID}
		db.Create(&hh)
		admin.HouseholdID = hh.ID
		db.Save(&admin)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Params = gin.Params{{Key: "id", Value: "9999"}}
		appCtx := newConsumptionAppCtx(db, admin.ID)

		GetRestockSuggestion(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})
}
