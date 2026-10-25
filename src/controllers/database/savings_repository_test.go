package database

import (
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func seedCanonicalPrices(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []dbModel.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
}

// A product tagged with a generic and a specific category keeps the price it
// had before the resolution became explicit.
func TestRecordSavingsEventPricesTheTaggedCategoryTheSameWay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	seedCanonicalPrices(t, db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.Categories = "dairy products, milk"
	product.Quantity = 1

	repo := NewSavingsRepository(db)
	if err := repo.RecordSavingsEvent(household.ID, product, dbModel.SavingsEventTypeConsumed, nil); err != nil {
		t.Fatalf("RecordSavingsEvent() error = %v", err)
	}

	var record dbModel.SavingsRecord
	if err := db.Where("household_id = ?", household.ID).First(&record).Error; err != nil {
		t.Fatalf("read savings record: %v", err)
	}
	if record.PriceEUR != 1.80 {
		t.Errorf("price = %v, want 1.80 (the dairy products category)", record.PriceEUR)
	}
	if record.CO2Kg != 1.60 {
		t.Errorf("CO₂ = %v, want 1.60 (the dairy products category)", record.CO2Kg)
	}
}

// A product whose categories name no canonical category is priced with the
// zero-value fallback.
func TestRecordSavingsEventLeavesAnUnmatchedProductUnpriced(t *testing.T) {
	db := testutil.SetupTestDB(t)
	seedCanonicalPrices(t, db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.Categories = "en:snacks"
	product.Quantity = 1

	repo := NewSavingsRepository(db)
	if err := repo.RecordSavingsEvent(household.ID, product, dbModel.SavingsEventTypeConsumed, nil); err != nil {
		t.Fatalf("RecordSavingsEvent() error = %v", err)
	}

	var record dbModel.SavingsRecord
	if err := db.Where("household_id = ?", household.ID).First(&record).Error; err != nil {
		t.Fatalf("read savings record: %v", err)
	}
	if record.PriceEUR != 0 {
		t.Errorf("price = %v, want 0 (no canonical category)", record.PriceEUR)
	}
	if record.CO2Kg != 0 {
		t.Errorf("CO₂ = %v, want 0 (no canonical category)", record.CO2Kg)
	}
}

// The canonical rows are read once, not once per product.
func TestRecordSavingsEventReadsTheCanonicalRowsOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	seedCanonicalPrices(t, db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	counter := testutil.NewQueryCounter()
	counting := db.Session(&gorm.Session{Logger: counter})
	repo := NewSavingsRepository(counting)

	resolver, err := repo.LoadCategoryResolver()
	if err != nil {
		t.Fatalf("LoadCategoryResolver() error = %v", err)
	}

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.Categories = "dairy products, milk"
	product.Quantity = 1

	for i := 0; i < 20; i++ {
		if err := repo.RecordSavingsEvent(household.ID, product, dbModel.SavingsEventTypeConsumed, resolver); err != nil {
			t.Fatalf("RecordSavingsEvent() error = %v", err)
		}
	}

	if got := counter.CountOf("product_category_prices"); got != 1 {
		t.Errorf("canonical category reads = %d, want 1 for 20 products", got)
	}
}
