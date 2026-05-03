package database

import (
	"strings"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
)

type SavingsRepositoryInterface interface {
	MatchCategory(categories string) (*dbModel.ProductCategoryPrice, error)
	RecordSavingsEvent(householdID uint, product *dbModel.Product, eventType string) error
	GetSavingsStats(householdID uint) (apiModel.SavingsStatsResponse, error)
}

var _ SavingsRepositoryInterface = (*SavingsRepository)(nil)

// SavingsRepository handles savings event recording and statistics queries.
type SavingsRepository struct {
	DB *gorm.DB
}

// NewSavingsRepository creates a new SavingsRepository.
func NewSavingsRepository(db *gorm.DB) *SavingsRepository {
	return &SavingsRepository{DB: db}
}

// MatchCategory resolves a Product.Categories string to a ProductCategoryPrice row.
// Returns nil if no match is found; caller should use zero-value pricing.
func (r *SavingsRepository) MatchCategory(categories string) (*dbModel.ProductCategoryPrice, error) {
	var allPrices []dbModel.ProductCategoryPrice
	if err := r.DB.Find(&allPrices).Error; err != nil {
		return nil, err
	}

	tokens := strings.Split(categories, ",")
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if idx := strings.Index(token, ":"); idx != -1 {
			token = strings.TrimSpace(token[idx+1:])
		}
		token = strings.ToLower(strings.ReplaceAll(token, "-", " "))

		for i := range allPrices {
			key := strings.ToLower(allPrices[i].CategoryKey)
			if strings.Contains(token, key) || strings.Contains(key, token) {
				return &allPrices[i], nil
			}
		}
	}
	return nil, nil
}

// RecordSavingsEvent writes a SavingsRecord for a consume or waste action.
// eventType must be "consumed" or "wasted".
// CO2 priority: per-product Agribalyse rate (product.CO2KgPerKg) > seeded category fallback.
// Price priority: product.PriceOverride > seeded category average.
func (r *SavingsRepository) RecordSavingsEvent(householdID uint, product *dbModel.Product, eventType string) error {
	catPrice, err := r.MatchCategory(product.Categories)
	if err != nil {
		return err
	}

	weightKg := 0.5
	if catPrice != nil {
		weightKg = catPrice.WeightGrams / 1000.0
	}

	var co2Kg float64
	if product.CO2KgPerKg != nil {
		co2Kg = *product.CO2KgPerKg * weightKg * float64(product.Amount)
	} else if catPrice != nil {
		co2Kg = catPrice.CO2KgPerKg * weightKg * float64(product.Amount)
	}

	var priceEUR float64
	if product.PriceOverride != nil {
		priceEUR = *product.PriceOverride * float64(product.Amount)
	} else if catPrice != nil {
		priceEUR = catPrice.AvgPriceEUR * float64(product.Amount)
	}

	record := dbModel.SavingsRecord{
		HouseholdID: householdID,
		ProductID:   product.ID,
		ProductName: product.ProductName,
		EventType:   eventType,
		PriceEUR:    priceEUR,
		CO2Kg:       co2Kg,
		Amount:      product.Amount,
	}
	return r.DB.Create(&record).Error
}

// GetSavingsStats returns savings and waste totals for the current calendar month and all time.
func (r *SavingsRepository) GetSavingsStats(householdID uint) (apiModel.SavingsStatsResponse, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var monthRecords []dbModel.SavingsRecord
	if err := r.DB.
		Where("household_id = ? AND created_at >= ? AND created_at <= ?", householdID, monthStart, monthEnd).
		Find(&monthRecords).Error; err != nil {
		return apiModel.SavingsStatsResponse{}, err
	}

	var allRecords []dbModel.SavingsRecord
	if err := r.DB.Where(util.QueryHouseholdId, householdID).Find(&allRecords).Error; err != nil {
		return apiModel.SavingsStatsResponse{}, err
	}

	var resp apiModel.SavingsStatsResponse
	for idx := range monthRecords {
		switch monthRecords[idx].EventType {
		case "consumed":
			resp.SavedEURThisMonth += monthRecords[idx].PriceEUR
			resp.SavedCO2KgThisMonth += monthRecords[idx].CO2Kg
		case "wasted":
			resp.WastedEURThisMonth += monthRecords[idx].PriceEUR
			resp.WastedCO2KgThisMonth += monthRecords[idx].CO2Kg
		}
	}
	for idx := range allRecords {
		switch allRecords[idx].EventType {
		case "consumed":
			resp.SavedEURLifetime += allRecords[idx].PriceEUR
			resp.SavedCO2KgLifetime += allRecords[idx].CO2Kg
		case "wasted":
			resp.WastedEURLifetime += allRecords[idx].PriceEUR
			resp.WastedCO2KgLifetime += allRecords[idx].CO2Kg
		}
	}

	resp.CO2Source = "Agribalyse LCA database via Open Food Facts ecoscore_data"
	return resp, nil
}
