package database

import "gorm.io/gorm"

// ShoppingListItem represents a user-created shopping list entry.
type ShoppingListItem struct {
	gorm.Model
	HouseholdID uint   `gorm:"index, not null"`
	ProductID   *uint  `gorm:"index"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	// CategoryKey is the canonical category the item belongs to. It is what
	// the household's shelf-life defaults are looked up by; Category keeps
	// the free text.
	CategoryKey *string `gorm:"index" json:"categoryKey,omitempty"`
	Quantity    int     `json:"quantity"`
	Checked     bool    `gorm:"default:false" json:"checked"`
	Notes       string  `json:"notes"`
	CreatedBy   uint    `gorm:"not null;default:0" json:"createdBy"`

	// PackSize is the size of one pack of the linked product. It is display
	// metadata copied from the product, not a column of its own.
	PackSize string `gorm:"-" json:"packSize,omitempty"`
}
