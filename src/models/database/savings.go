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

// SavingsEventType names the kind of action a savings record was written for.
const (
	SavingsEventTypeConsumed = "consumed"
	SavingsEventTypeWasted   = "wasted"
)

// SavingsRecord is written once per consume or waste action. It captures the monetary
// value and CO2 equivalent at the time of the event so that later changes to
// ProductCategoryPrice do not retroactively alter history.
type SavingsRecord struct {
	gorm.Model
	HouseholdID uint    `gorm:"index;not null" json:"-"`
	ProductID   uint    `gorm:"index"          json:"productId"`
	ProductName string  `gorm:"not null"       json:"productName"`
	EventType   string  `gorm:"not null;index" json:"eventType"` // SavingsEventTypeConsumed or SavingsEventTypeWasted
	PriceEUR    float64 `gorm:"not null"       json:"priceEur"`
	CO2Kg       float64 `gorm:"not null"       json:"co2Kg"`
	// Amount is the number of packs the event covered. A record written before
	// the count model holds a net content instead, which is why such a record
	// is marked as legacy.
	Amount int `gorm:"not null;default:1" json:"amount"`
	// Legacy marks a record written before the count model, when the amount
	// was a net content and the money and CO2 were scaled by it. Such a
	// record is not comparable with the ones after the cutover.
	Legacy bool `gorm:"default:false" json:"legacy,omitempty"`
}
