package database

import "gorm.io/gorm"

const (
	// MaxCategoryShelfLifeDays is the upper bound on
	// HouseholdCategory.DefaultShelfLifeDays, the shelf life a household may
	// give a category from purchase.
	MaxCategoryShelfLifeDays = 3650

	// MinCategoryShelfLifeDays is the lower bound on the shelf life a
	// household may give a category. A category with no shelf life carries no
	// value at all, so a day count below one is a mistake rather than a
	// shorter shelf life.
	MinCategoryShelfLifeDays = 1
)

// HouseholdCategory is one household's override of a canonical category: the
// name it shows, the place it has in the household's list, and the shelf life
// the household's products of that category default to.
//
// The category itself is a row of the global ProductCategoryPrice table. A
// household overrides a category, it never replaces or removes it: the
// canonical rows carry the prices and the CO2 factors the savings and the
// waste analytics depend on.
type HouseholdCategory struct {
	gorm.Model
	HouseholdID uint   `gorm:"uniqueIndex:idx_household_category_key,priority:1;not null" json:"householdId"`
	CategoryKey string `gorm:"uniqueIndex:idx_household_category_key,priority:2;not null" json:"categoryKey"`
	// DisplayName is the name the household shows for the category. An empty
	// name falls back to the canonical one.
	DisplayName string `gorm:"not null" json:"displayName"`
	// SortOrder is the place of the category in the household's list.
	SortOrder int `gorm:"default:0" json:"sortOrder"`
	// Hidden keeps the category out of the household's entry form.
	Hidden bool `gorm:"default:false" json:"hidden"`
	// DefaultShelfLifeDays is the shelf life from purchase, in days.
	DefaultShelfLifeDays *int `gorm:"default:null" json:"defaultShelfLifeDays,omitempty"`
	// DefaultDaysAfterOpening is the shelf life after opening, in days. It is
	// bounded by MaxDaysAfterOpening, the same bound every product write
	// enforces.
	DefaultDaysAfterOpening *int `gorm:"default:null" json:"defaultDaysAfterOpening,omitempty"`
}

// HouseholdCategoryView is one category of a household's list: the canonical
// category with the household's overrides applied.
type HouseholdCategoryView struct {
	CategoryKey             string
	DisplayName             string
	SortOrder               int
	Hidden                  bool
	DefaultShelfLifeDays    *int
	DefaultDaysAfterOpening *int
}
