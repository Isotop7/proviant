package database

import (
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
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

	now := time.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)

	// Household A: 1 consumed dairy, 2 wasted dairy, 1 wasted bakery
	dairyProd := dbModel.Product{
		ProductName: "Yogurt", Categories: "en:dairy", HouseholdID: hhA.ID, UserID: adminA.ID, ExpireAt: now,
		Amount: 2, PriceOverride: float64Ptr(1.99), CO2KgPerKg: float64Ptr(1.2),
	}
	db.Create(&dairyProd)
	bakeryProd := dbModel.Product{
		ProductName: "Bread", Categories: "en:bakery", HouseholdID: hhA.ID, UserID: adminA.ID, ExpireAt: now,
		Amount: 1, PriceOverride: float64Ptr(0.99), CO2KgPerKg: float64Ptr(0.8),
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
		out, err := repo.GetMonthlyBreakdown(hhA.ID, since, 2)
		if err != nil {
			t.Fatalf("GetMonthlyBreakdown: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected 2 months, got %d", len(out))
		}
		// Newest month should have all 4 events
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
		out, err := repo.GetTrendMonths(hhA.ID, 6)
		if err != nil {
			t.Fatalf("GetTrendMonths: %v", err)
		}
		if len(out) != 6 {
			t.Fatalf("expected 6 months, got %d", len(out))
		}
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
			Amount: 1, PriceOverride: float64Ptr(3.0), CO2KgPerKg: float64Ptr(2.0),
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

func float64Ptr(v float64) *float64 { return &v }

func keysOfProducts(m map[string][]apiModel.WasteProductStat) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
