package database

import "gorm.io/gorm"

// ShoppingListItem represents a user-created shopping list entry.
type ShoppingListItem struct {
	gorm.Model
	HouseholdID uint   `gorm:"index, not null"`
	ProductID   *uint  `gorm:"index"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Quantity    int    `json:"quantity"`
	Unit        string `json:"unit"`
	Checked     bool   `gorm:"default:false" json:"checked"`
	Notes       string `json:"notes"`
	CreatedBy   uint   `gorm:"not null;default:0" json:"createdBy"`
}
