package migrations

import (
	"testing"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
)

// The backfill fills the canonical category of a product and leaves the free
// text the recipe suggestions read alone.
func TestBackfillProductCategoryKeys(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()

	for _, row := range []database.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	milk := testutil.CreateTestProduct(db, household.ID, user.ID)
	snacks := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(milk).Update("categories", "en:dairy products,en:milk").Error; err != nil {
		t.Fatalf("set categories: %v", err)
	}
	if err := db.Model(snacks).Update("categories", "en:snacks").Error; err != nil {
		t.Fatalf("set categories: %v", err)
	}

	if err := BackfillProductCategoryKeys(&logger, db); err != nil {
		t.Fatalf("BackfillProductCategoryKeys() error = %v", err)
	}

	var stored database.Product
	if err := db.First(&stored, milk.ID).Error; err != nil {
		t.Fatalf("read product: %v", err)
	}
	if stored.CategoryKey == nil {
		t.Fatal("the category of the milk product is empty")
	}
	if *stored.CategoryKey != "dairy products" {
		t.Errorf("category key = %q, want dairy products", *stored.CategoryKey)
	}
	if stored.Categories != "en:dairy products,en:milk" {
		t.Errorf("categories = %q, want the free text to be untouched", stored.Categories)
	}

	var uncategorised database.Product
	if err := db.First(&uncategorised, snacks.ID).Error; err != nil {
		t.Fatalf("read product: %v", err)
	}
	if uncategorised.CategoryKey != nil {
		t.Errorf("category key = %q, want none for a product that names no canonical category", *uncategorised.CategoryKey)
	}
}

// The backfill runs on every startup, so a second run must change nothing.
func TestBackfillProductCategoryKeysIsIdempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()

	for _, row := range []database.ProductCategoryPrice{
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(product).Update("categories", "milk").Error; err != nil {
		t.Fatalf("set categories: %v", err)
	}

	if err := BackfillProductCategoryKeys(&logger, db); err != nil {
		t.Fatalf("BackfillProductCategoryKeys() error = %v", err)
	}
	if err := BackfillProductCategoryKeys(&logger, db); err != nil {
		t.Fatalf("second BackfillProductCategoryKeys() error = %v", err)
	}

	var count int64
	if err := db.Model(&database.Product{}).Where("category_key IS NOT NULL").Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 1 {
		t.Errorf("%d products carry a category, want 1", count)
	}
}
