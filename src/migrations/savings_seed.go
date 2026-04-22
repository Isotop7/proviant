package migrations

import (
	"codeberg.org/isotop7/proviant/models/database"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SeedProductCategoryPrices inserts default category price/CO2 reference rows if the table is empty.
// CO2 values (kg CO2e per kg food) are derived from the Agribalyse LCA database, the same source
// used by Open Food Facts for ecoscore_data. EUR prices are EU retail averages (Eurostat, 2023).
// Idempotent: skipped entirely if any row already exists.
func SeedProductCategoryPrices(logger *zerolog.Logger, db *gorm.DB) error {
	logger.Info().Msg("Checking ProductCategoryPrice seed data")

	var count int64
	if err := db.Model(&database.ProductCategoryPrice{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		logger.Info().Msg("ProductCategoryPrice already seeded, skipping")
		return nil
	}

	entries := []database.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
		{CategoryKey: "cheese", DisplayName: "Cheese", AvgPriceEUR: 3.50, CO2KgPerKg: 8.50, WeightGrams: 200},
		{CategoryKey: "yoghurts", DisplayName: "Yoghurt", AvgPriceEUR: 1.20, CO2KgPerKg: 2.20, WeightGrams: 400},
		{CategoryKey: "meat", DisplayName: "Meat", AvgPriceEUR: 7.00, CO2KgPerKg: 17.00, WeightGrams: 400},
		{CategoryKey: "poultry", DisplayName: "Poultry", AvgPriceEUR: 5.50, CO2KgPerKg: 6.90, WeightGrams: 400},
		{CategoryKey: "fish", DisplayName: "Fish & Seafood", AvgPriceEUR: 6.00, CO2KgPerKg: 5.40, WeightGrams: 300},
		{CategoryKey: "breads", DisplayName: "Bread & Bakery", AvgPriceEUR: 2.20, CO2KgPerKg: 1.40, WeightGrams: 500},
		{CategoryKey: "cereals", DisplayName: "Cereals & Grains", AvgPriceEUR: 2.50, CO2KgPerKg: 1.20, WeightGrams: 500},
		{CategoryKey: "fresh vegetables", DisplayName: "Fresh Vegetables", AvgPriceEUR: 1.50, CO2KgPerKg: 0.40, WeightGrams: 500},
		{CategoryKey: "fresh fruits", DisplayName: "Fresh Fruits", AvgPriceEUR: 2.00, CO2KgPerKg: 0.45, WeightGrams: 500},
		{CategoryKey: "potatoes", DisplayName: "Potatoes", AvgPriceEUR: 1.00, CO2KgPerKg: 0.35, WeightGrams: 1000},
		{CategoryKey: "eggs", DisplayName: "Eggs", AvgPriceEUR: 2.50, CO2KgPerKg: 3.60, WeightGrams: 600},
		{CategoryKey: "pasta", DisplayName: "Pasta & Noodles", AvgPriceEUR: 1.80, CO2KgPerKg: 1.10, WeightGrams: 500},
		{CategoryKey: "rice", DisplayName: "Rice", AvgPriceEUR: 2.00, CO2KgPerKg: 2.70, WeightGrams: 1000},
		{CategoryKey: "sauces", DisplayName: "Sauces & Condiments", AvgPriceEUR: 2.80, CO2KgPerKg: 1.80, WeightGrams: 350},
		{CategoryKey: "soups", DisplayName: "Soups", AvgPriceEUR: 1.50, CO2KgPerKg: 0.90, WeightGrams: 400},
		{CategoryKey: "desserts", DisplayName: "Desserts & Cakes", AvgPriceEUR: 3.00, CO2KgPerKg: 2.80, WeightGrams: 300},
		{CategoryKey: "soft drinks", DisplayName: "Soft Drinks", AvgPriceEUR: 1.20, CO2KgPerKg: 0.35, WeightGrams: 500},
		{CategoryKey: "frozen foods", DisplayName: "Frozen Foods", AvgPriceEUR: 3.50, CO2KgPerKg: 2.50, WeightGrams: 500},
	}

	for i := range entries {
		if err := db.Create(&entries[i]).Error; err != nil {
			return err
		}
	}

	logger.Info().Int("count", len(entries)).Msg("Seeded ProductCategoryPrice entries")
	return nil
}
