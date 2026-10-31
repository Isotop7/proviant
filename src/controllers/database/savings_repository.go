package database

import (
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

type SavingsRepositoryInterface interface {
	LoadCategoryResolver() (*CategoryResolver, error)
	RecordSavingsEvent(householdID uint, product *dbModel.Product, eventType string, cause string, resolver *CategoryResolver) error
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

// LoadCategoryResolver reads the canonical category rows and prepares them for
// resolution. A caller that prices many products loads the resolver once and
// hands it to every RecordSavingsEvent, so the canonical rows are read once
// per request instead of once per product.
func (r *SavingsRepository) LoadCategoryResolver() (*CategoryResolver, error) {
	return LoadCategoryResolver(r.DB)
}

// RecordSavingsEvent writes a SavingsRecord for a consume or waste action.
// eventType must be dbModel.SavingsEventTypeConsumed or dbModel.SavingsEventTypeWasted.
// The cause is why the event wasted the product; it does not change the event
// type and stays empty for a consumed event and for an unattributed waste.
// CO2 priority: per-product Agribalyse rate (product.CO2KgPerKg) > seeded category fallback.
// Price priority: product.PriceOverride > seeded category average. The override is
// the price of one pack, so it is multiplied by the number of packs.
// A nil resolver makes the repository load one; a caller that records many
// events passes the resolver it already loaded.
func (r *SavingsRepository) RecordSavingsEvent(householdID uint, product *dbModel.Product, eventType string, cause string, resolver *CategoryResolver) error {
	if resolver == nil {
		loaded, err := r.LoadCategoryResolver()
		if err != nil {
			return err
		}
		resolver = loaded
	}

	var catPrice *dbModel.ProductCategoryPrice
	if resolved, ok := resolver.Resolve(product.Categories); ok {
		catPrice = resolved
	}

	weightKg := 0.5
	if catPrice != nil {
		weightKg = catPrice.WeightGrams / 1000.0
	}

	var co2Kg float64
	if product.CO2KgPerKg != nil {
		co2Kg = *product.CO2KgPerKg * weightKg * float64(product.Quantity)
	} else if catPrice != nil {
		co2Kg = catPrice.CO2KgPerKg * weightKg * float64(product.Quantity)
	}

	var priceEUR float64
	if product.PriceOverride != nil {
		priceEUR = *product.PriceOverride * float64(product.Quantity)
	} else if catPrice != nil {
		priceEUR = catPrice.AvgPriceEUR * float64(product.Quantity)
	}

	record := dbModel.SavingsRecord{
		HouseholdID: householdID,
		ProductID:   product.ID,
		ProductName: product.ProductName,
		EventType:   eventType,
		WasteCause:  cause,
		PriceEUR:    priceEUR,
		CO2Kg:       co2Kg,
		Amount:      product.Quantity,
	}
	return r.DB.Create(&record).Error
}

// GetSavingsStats returns savings and waste totals for the current calendar
// month and all time. The records written before the count model are left out:
// their money and CO2 were scaled by the net content of the product, so they
// are not comparable with the ones after the cutover. Their number is
// reported so the gap can be explained.
func (r *SavingsRepository) GetSavingsStats(householdID uint) (apiModel.SavingsStatsResponse, error) {
	now := time.Now()
	monthStart := util.MonthStart(now)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var monthRecords []dbModel.SavingsRecord
	if err := r.DB.
		Where("household_id = ? AND created_at >= ? AND created_at <= ?", householdID, monthStart, monthEnd).
		Where("legacy = ?", false).
		Find(&monthRecords).Error; err != nil {
		return apiModel.SavingsStatsResponse{}, err
	}

	var allRecords []dbModel.SavingsRecord
	if err := r.DB.Where(util.QueryHouseholdId, householdID).Where("legacy = ?", false).Find(&allRecords).Error; err != nil {
		return apiModel.SavingsStatsResponse{}, err
	}

	var resp apiModel.SavingsStatsResponse
	for idx := range monthRecords {
		switch monthRecords[idx].EventType {
		case dbModel.SavingsEventTypeConsumed:
			resp.SavedEURThisMonth += monthRecords[idx].PriceEUR
			resp.SavedCO2KgThisMonth += monthRecords[idx].CO2Kg
		case dbModel.SavingsEventTypeWasted:
			resp.WastedEURThisMonth += monthRecords[idx].PriceEUR
			resp.WastedCO2KgThisMonth += monthRecords[idx].CO2Kg
		}
	}
	for idx := range allRecords {
		switch allRecords[idx].EventType {
		case dbModel.SavingsEventTypeConsumed:
			resp.SavedEURLifetime += allRecords[idx].PriceEUR
			resp.SavedCO2KgLifetime += allRecords[idx].CO2Kg
		case dbModel.SavingsEventTypeWasted:
			resp.WastedEURLifetime += allRecords[idx].PriceEUR
			resp.WastedCO2KgLifetime += allRecords[idx].CO2Kg
		}
	}

	var legacyCount int64
	if err := r.DB.Model(&dbModel.SavingsRecord{}).
		Where(util.QueryHouseholdId, householdID).
		Where("legacy IS NULL OR legacy = ?", true).
		Count(&legacyCount).Error; err != nil {
		return apiModel.SavingsStatsResponse{}, err
	}
	resp.LegacyEvents = int(legacyCount)

	resp.CO2Source = "Agribalyse LCA database via Open Food Facts ecoscore_data"
	return resp, nil
}
