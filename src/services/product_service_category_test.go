package services

import (
	"testing"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// A bulk action reads the canonical categories once, no matter how many
// products it handles.
func TestBulkConsumeProductsReadsTheCanonicalCategoriesOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	counter := testutil.NewQueryCounter()
	counting := db.Session(&gorm.Session{Logger: counter})

	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(counting, &logger)
	service := NewProductService(repos, &logger)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	for _, row := range []dbModel.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}

	productIDs := make([]uint, 0, 10)
	for i := 0; i < 10; i++ {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := db.Model(product).Update("categories", "dairy products, milk").Error; err != nil {
			t.Fatalf("set product categories: %v", err)
		}
		productIDs = append(productIDs, product.ID)
	}

	if _, err := service.BulkConsumeProducts(productIDs, user.ID); err != nil {
		t.Fatalf("BulkConsumeProducts() error = %v", err)
	}

	if got := counter.CountOf("product_category_prices"); got != 1 {
		t.Errorf("canonical category reads = %d, want 1 for %d products", got, len(productIDs))
	}
}
