package database

import (
	"time"

	"gorm.io/gorm"
)

// MealPlanEntry schedules one recipe on one day of the household meal plan.
// Date is stored date-only (midnight); Servings states how many people the
// meal is planned for. It does not scale what leaves the stock: a planned
// meal takes one pack of every product the recipe links.
type MealPlanEntry struct {
	gorm.Model
	HouseholdID uint      `gorm:"index, not null"`
	Date        time.Time `gorm:"index" json:"date"`
	RecipeID    uint      `gorm:"index, not null"`
	Servings    int       `gorm:"not null;default:1" json:"servings"`
	Notes       string    `json:"notes"`
	CreatedBy   uint      `gorm:"not null;default:0" json:"createdBy"`
}
