package database

import "gorm.io/gorm"

// StorageLocation represents a named location within a household (e.g. Fridge, Freezer, Pantry)
type StorageLocation struct {
	gorm.Model
	HouseholdID uint      `gorm:"index;not null" json:"householdId"`
	Household   Household `json:"-"`
	Name        string    `gorm:"not null"       json:"name"`
	Icon        string    `gorm:"default:'📦'"  json:"icon"`
	SortOrder   int       `gorm:"default:0"      json:"sortOrder"`
}
