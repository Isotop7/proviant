package database

import "gorm.io/gorm"

// ProductCategoryPrice maps canonical food category keys to average EUR prices and
// CO2e coefficients (kg CO2e per kg food) sourced from Agribalyse LCA database via
// Open Food Facts. Used as fallback when per-product Agribalyse data is unavailable.
type ProductCategoryPrice struct {
	gorm.Model
	CategoryKey string  `gorm:"uniqueIndex;not null" json:"categoryKey"`
	DisplayName string  `gorm:"not null"             json:"displayName"`
	AvgPriceEUR float64 `gorm:"not null"             json:"avgPriceEur"`
	CO2KgPerKg  float64 `gorm:"not null"             json:"co2KgPerKg"`
	WeightGrams float64 `gorm:"not null;default:500" json:"weightGrams"`
}

// SavingsRecord is written once per consume or waste action. It captures the monetary
// value and CO2 equivalent at the time of the event so that later changes to
// ProductCategoryPrice do not retroactively alter history.
type SavingsRecord struct {
	gorm.Model
	HouseholdID uint    `gorm:"index;not null" json:"-"`
	ProductID   uint    `gorm:"index"          json:"productId"`
	ProductName string  `gorm:"not null"       json:"productName"`
	EventType   string  `gorm:"not null;index" json:"eventType"` // "consumed" or "wasted"
	PriceEUR    float64 `gorm:"not null"       json:"priceEur"`
	CO2Kg       float64 `gorm:"not null"       json:"co2Kg"`
	Amount      int     `gorm:"not null;default:1" json:"amount"`
}
