package database

import (
	"math"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

func TestWasteAnalyticsRepository_Aggregations(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWasteAnalyticsRepository(db)

	// Seed category price lookup
	if err := db.Create(&dbModel.ProductCategoryPrice{
		CategoryKey: "dairy",
		DisplayName: "Dairy",
		AvgPriceEUR: 2.0,
		CO2KgPerKg:  1.5,
		WeightGrams: 500,
	}).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	if err := db.Create(&dbModel.ProductCategoryPrice{
		CategoryKey: "bakery",
		DisplayName: "Bakery",
		AvgPriceEUR: 1.5,
		CO2KgPerKg:  1.0,
		WeightGrams: 300,
	}).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}

	// Two households: A has data, B must not be visible to A.
	adminA := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
	adminB := authentication.User{Username: "b", Password: "x", MailAddress: "b@x"}
	db.Create(&adminA)
	db.Create(&adminB)

	hhA := dbModel.Household{Name: "A", AdminID: adminA.ID}
	hhB := dbModel.Household{Name: "B", AdminID: adminB.ID}
	db.Create(&hhA)
	db.Create(&hhB)

	// The 31st is the day the month arithmetic breaks on, so the anchor is
	// pinned to one: the assertions have to hold on every day of the year.
	now := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)
	since := util.MonthShift(now, -1)

	// Household A: 1 consumed dairy, 2 wasted dairy, 1 wasted bakery
	dairyProd := dbModel.Product{
		ProductName: "Yogurt", Categories: "en:dairy", HouseholdID: hhA.ID, UserID: adminA.ID, ExpireAt: now,
		Quantity: 2, PriceOverride: new(1.99), CO2KgPerKg: new(1.2),
	}
	db.Create(&dairyProd)
	bakeryProd := dbModel.Product{
		ProductName: "Bread", Categories: "en:bakery", HouseholdID: hhA.ID, UserID: adminA.ID, ExpireAt: now,
		Quantity: 1, PriceOverride: new(0.99), CO2KgPerKg: new(0.8),
	}
	db.Create(&bakeryProd)

	insertSavings := func(productID, householdID uint, name, eventType string, eur, co2 float64, at time.Time) {
		rec := dbModel.SavingsRecord{
			HouseholdID: householdID, ProductID: productID, ProductName: name,
			EventType: eventType, PriceEUR: eur, CO2Kg: co2, Amount: 1,
		}
		rec.CreatedAt = at
		rec.UpdatedAt = at
		db.Create(&rec)
	}

	insertSavings(dairyProd.ID, hhA.ID, "Yogurt", "consumed", 1.99, 1.2, now.Add(-1*time.Hour))
	insertSavings(dairyProd.ID, hhA.ID, "Yogurt", "wasted", 1.99, 1.2, now.Add(-2*time.Hour))
	insertSavings(dairyProd.ID, hhA.ID, "Yogurt", "wasted", 1.99, 1.2, now.Add(-3*time.Hour))
	insertSavings(bakeryProd.ID, hhA.ID, "Bread", "wasted", 0.99, 0.8, now.Add(-4*time.Hour))

	// Household B: noise that must not appear in A's totals
	insertSavings(9999, hhB.ID, "Other", "wasted", 100.0, 50.0, now.Add(-1*time.Hour))

	t.Run("consumed vs wasted", func(t *testing.T) {
		row, err := repo.GetConsumedVsWasted(hhA.ID, since)
		if err != nil {
			t.Fatalf("GetConsumedVsWasted: %v", err)
		}
		if row.ConsumedCount != 1 {
			t.Errorf("consumed count = %d, want 1", row.ConsumedCount)
		}
		if row.WastedCount != 3 {
			t.Errorf("wasted count = %d, want 3", row.WastedCount)
		}
		if got, want := row.WastedEUR, 1.99+1.99+0.99; got != want {
			t.Errorf("wasted EUR = %v, want %v", got, want)
		}
		if got, want := row.WastedCO2Kg, 1.2+1.2+0.8; got != want {
			t.Errorf("wasted CO2 = %v, want %v", got, want)
		}
	})

	t.Run("monthly breakdown is padded and ordered", func(t *testing.T) {
		out, err := repo.GetMonthlyBreakdown(hhA.ID, 2, now)
		if err != nil {
			t.Fatalf("GetMonthlyBreakdown: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected 2 months, got %d", len(out))
		}
		assertConsecutiveMonths(t, []string{out[0].Month, out[1].Month})
		// The older month holds no event, the newer one all 4.
		if out[0].ConsumedCount+out[0].WastedCount != 0 {
			t.Errorf("older month events = %d, want 0", out[0].ConsumedCount+out[0].WastedCount)
		}
		last := out[len(out)-1]
		if last.ConsumedCount+last.WastedCount != 4 {
			t.Errorf("last month events = %d, want 4", last.ConsumedCount+last.WastedCount)
		}
	})

	t.Run("most wasted categories", func(t *testing.T) {
		cats, err := repo.GetMostWastedCategories(hhA.ID, since, 5, "count")
		if err != nil {
			t.Fatalf("GetMostWastedCategories: %v", err)
		}
		if len(cats) != 2 {
			t.Fatalf("expected 2 categories, got %d", len(cats))
		}
		if cats[0].CategoryKey != "dairy" {
			t.Errorf("top category = %q, want dairy", cats[0].CategoryKey)
		}
		if cats[0].Count != 2 {
			t.Errorf("dairy count = %d, want 2", cats[0].Count)
		}
		if cats[1].CategoryKey != "bakery" {
			t.Errorf("second category = %q, want bakery", cats[1].CategoryKey)
		}
	})

	t.Run("most wasted categories sorted by cost", func(t *testing.T) {
		// Dairy cost = 1.99*2 = 3.98 EUR, bakery cost = 0.99 EUR. Cost sort → dairy first.
		cats, err := repo.GetMostWastedCategories(hhA.ID, since, 5, "cost")
		if err != nil {
			t.Fatalf("GetMostWastedCategories(cost): %v", err)
		}
		if len(cats) != 2 {
			t.Fatalf("expected 2 categories, got %d", len(cats))
		}
		if cats[0].CategoryKey != "dairy" {
			t.Errorf("top by cost = %q, want dairy", cats[0].CategoryKey)
		}
		if cats[1].CategoryKey != "bakery" {
			t.Errorf("second by cost = %q, want bakery", cats[1].CategoryKey)
		}
	})

	t.Run("most wasted products grouped by category", func(t *testing.T) {
		prods, err := repo.GetMostWastedProducts(hhA.ID, since, 3)
		if err != nil {
			t.Fatalf("GetMostWastedProducts: %v", err)
		}
		dairy, ok := prods["dairy"]
		if !ok {
			t.Fatalf("dairy bucket missing; got keys %v", keysOfProducts(prods))
		}
		if len(dairy) != 1 {
			t.Fatalf("dairy products = %d, want 1 (Yogurt)", len(dairy))
		}
		if dairy[0].ProductName != "Yogurt" {
			t.Errorf("dairy product name = %q, want Yogurt", dairy[0].ProductName)
		}
		if dairy[0].Count != 2 {
			t.Errorf("Yogurt wasted count = %d, want 2", dairy[0].Count)
		}
		if got, want := dairy[0].CostEUR, 1.99*2; got != want {
			t.Errorf("Yogurt wasted EUR = %v, want %v", got, want)
		}
		bakery, ok := prods["bakery"]
		if !ok {
			t.Fatalf("bakery bucket missing")
		}
		if len(bakery) != 1 || bakery[0].ProductName != "Bread" {
			t.Errorf("bakery products = %+v, want single Bread", bakery)
		}
	})

	t.Run("trend months padded", func(t *testing.T) {
		out, err := repo.GetTrendMonths(hhA.ID, 6, now)
		if err != nil {
			t.Fatalf("GetTrendMonths: %v", err)
		}
		if len(out) != 6 {
			t.Fatalf("expected 6 months, got %d", len(out))
		}
		keys := make([]string, len(out))
		for i := range out {
			keys[i] = out[i].Month
		}
		assertConsecutiveMonths(t, keys)
		total := 0
		for _, m := range out {
			total += m.Count
		}
		if total != 3 {
			t.Errorf("trend total = %d, want 3", total)
		}
	})

	t.Run("most wasted products capped per category", func(t *testing.T) {
		// Add 2 more distinct product names in dairy, then cap at 2 → 2 returned.
		extraProd := dbModel.Product{
			ProductName: "Cheese", Categories: "en:dairy", HouseholdID: hhA.ID, UserID: adminA.ID, ExpireAt: now,
			Quantity: 1, PriceOverride: new(3.0), CO2KgPerKg: new(2.0),
		}
		db.Create(&extraProd)
		insertSavings(extraProd.ID, hhA.ID, "Cheese", "wasted", 3.0, 2.0, now.Add(-5*time.Hour))
		insertSavings(extraProd.ID, hhA.ID, "Cheese", "wasted", 3.0, 2.0, now.Add(-6*time.Hour))

		prods, err := repo.GetMostWastedProducts(hhA.ID, since, 2)
		if err != nil {
			t.Fatalf("GetMostWastedProducts: %v", err)
		}
		dairy := prods["dairy"]
		if len(dairy) != 2 {
			t.Fatalf("dairy products with cap=2 = %d, want 2", len(dairy))
		}
		// Sorted by count desc; both Cheese entries merge into a single row of 2.
		if dairy[0].Count != 2 {
			t.Errorf("top dairy product count = %d, want 2", dairy[0].Count)
		}
	})

	t.Run("household scoping", func(t *testing.T) {
		row, err := repo.GetConsumedVsWasted(hhB.ID, since)
		if err != nil {
			t.Fatalf("GetConsumedVsWasted B: %v", err)
		}
		if row.WastedCount != 1 {
			t.Errorf("B wasted count = %d, want 1", row.WastedCount)
		}
		// A's row should not include B's 100 EUR
		rowA, _ := repo.GetConsumedVsWasted(hhA.ID, since)
		if rowA.WastedEUR >= 100.0 {
			t.Errorf("A's wasted EUR includes B's data: %v", rowA.WastedEUR)
		}
	})
}

func keysOfProducts(m map[string][]apiModel.WasteProductStat) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestWasteAnalyticsRepository_CauseBreakdown verifies that the waste is
// grouped by the reason it was wasted and that the category the household most
// over-buys is named with its count and its share of that category's waste.
func TestWasteAnalyticsRepository_CauseBreakdown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWasteAnalyticsRepository(db)

	if err := db.Create(&dbModel.ProductCategoryPrice{
		CategoryKey: "dairy",
		DisplayName: "Dairy",
		AvgPriceEUR: 2.0,
		CO2KgPerKg:  1.5,
		WeightGrams: 500,
	}).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}

	admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
	db.Create(&admin)
	household := dbModel.Household{Name: "A", AdminID: admin.ID}
	db.Create(&household)

	now := time.Now()
	since := util.MonthShift(now, -1)

	dairy := dbModel.Product{
		ProductName: "Yogurt", Categories: "en:dairy", HouseholdID: household.ID, UserID: admin.ID,
		ExpireAt: now, Quantity: 1,
	}
	db.Create(&dairy)
	bakery := dbModel.Product{
		ProductName: "Bread", Categories: "en:bakery", HouseholdID: household.ID, UserID: admin.ID,
		ExpireAt: now, Quantity: 1,
	}
	db.Create(&bakery)

	insertWaste := func(productID uint, name, cause string, eur float64, at time.Time) {
		record := dbModel.SavingsRecord{
			HouseholdID: household.ID, ProductID: productID, ProductName: name,
			EventType: dbModel.SavingsEventTypeWasted, WasteCause: cause,
			PriceEUR: eur, CO2Kg: 1, Amount: 1,
		}
		record.CreatedAt = at
		record.UpdatedAt = at
		db.Create(&record)
	}

	insertWaste(dairy.ID, "Yogurt", dbModel.WasteCauseOverbought, 2.0, now.Add(-1*time.Hour))
	insertWaste(dairy.ID, "Yogurt", dbModel.WasteCauseOverbought, 2.0, now.Add(-2*time.Hour))
	insertWaste(dairy.ID, "Yogurt", dbModel.WasteCauseSpoiled, 2.0, now.Add(-3*time.Hour))
	insertWaste(bakery.ID, "Bread", "", 1.0, now.Add(-4*time.Hour))

	breakdown, err := repo.GetWasteCauseBreakdown(household.ID, since)
	if err != nil {
		t.Fatalf("GetWasteCauseBreakdown: %v", err)
	}

	if breakdown.WastedEvents != 4 {
		t.Errorf("wasted events = %d, want 4", breakdown.WastedEvents)
	}
	if len(breakdown.Causes) != 3 {
		t.Fatalf("causes = %d, want 3", len(breakdown.Causes))
	}
	if breakdown.Causes[0].Cause != dbModel.WasteCauseOverbought || breakdown.Causes[0].Count != 2 {
		t.Errorf("top cause = %+v, want overbought with 2 events", breakdown.Causes[0])
	}
	if got, want := breakdown.Causes[0].Share, 50.0; got != want {
		t.Errorf("top cause share = %v, want %v", got, want)
	}
	if got, want := breakdown.Causes[0].CostEUR, 4.0; got != want {
		t.Errorf("top cause cost = %v, want %v", got, want)
	}
	var unattributed int
	for _, cause := range breakdown.Causes {
		if cause.Cause == "" {
			unattributed = cause.Count
		}
	}
	if unattributed != 1 {
		t.Errorf("unattributed waste = %d, want 1", unattributed)
	}

	if breakdown.Overbuy.CategoryKey != "dairy" {
		t.Errorf("overbought category = %q, want dairy", breakdown.Overbuy.CategoryKey)
	}
	if breakdown.Overbuy.DisplayName != "Dairy" {
		t.Errorf("overbought display name = %q, want Dairy", breakdown.Overbuy.DisplayName)
	}
	if breakdown.Overbuy.Count != 2 {
		t.Errorf("overbought count = %d, want 2", breakdown.Overbuy.Count)
	}
	if breakdown.Overbuy.CategoryWaste != 3 {
		t.Errorf("category waste = %d, want 3", breakdown.Overbuy.CategoryWaste)
	}
	if got, want := breakdown.Overbuy.Share, 200.0/3.0; math.Abs(got-want) > 0.0001 {
		t.Errorf("overbought share = %v, want %v", got, want)
	}
}

// A household without over-bought waste has no over-buy answer.
func TestWasteAnalyticsRepository_CauseBreakdownWithoutOverbuying(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWasteAnalyticsRepository(db)

	admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
	db.Create(&admin)
	household := dbModel.Household{Name: "A", AdminID: admin.ID}
	db.Create(&household)

	now := time.Now()
	record := dbModel.SavingsRecord{
		HouseholdID: household.ID, ProductID: 1, ProductName: "Yogurt",
		EventType: dbModel.SavingsEventTypeWasted, WasteCause: dbModel.WasteCauseSpoiled,
		PriceEUR: 1, CO2Kg: 1, Amount: 1,
	}
	record.CreatedAt = now
	db.Create(&record)

	breakdown, err := repo.GetWasteCauseBreakdown(household.ID, util.MonthShift(now, -1))
	if err != nil {
		t.Fatalf("GetWasteCauseBreakdown: %v", err)
	}
	if breakdown.Overbuy.Count != 0 || breakdown.Overbuy.CategoryKey != "" {
		t.Errorf("overbuy = %+v, want an empty answer", breakdown.Overbuy)
	}
}
