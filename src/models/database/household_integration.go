package database

import (
	"time"

	"gorm.io/gorm"
)

// HouseholdIntegration stores household-scoped credentials and sync state for an external shopping list provider.
type HouseholdIntegration struct {
	gorm.Model
	HouseholdID  uint       `gorm:"uniqueIndex:idx_household_provider;index;not null" json:"householdId"`
	Provider     string     `gorm:"uniqueIndex:idx_household_provider;index;not null" json:"provider"`
	Enabled      bool       `json:"enabled"`
	AccessToken  string     `gorm:"type:text" json:"-"`
	RefreshToken string     `gorm:"type:text" json:"-"`
	AccountEmail string     `json:"accountEmail,omitempty"`
	ListID       string     `json:"listId,omitempty"`
	ListName     string     `json:"listName,omitempty"`
	LastSyncAt   *time.Time `json:"lastSyncAt,omitempty"`
	LastError    string     `json:"lastError,omitempty"`
}

// ShoppingListItemSyncRef maps a local shopping list item to its external provider item identifier.
type ShoppingListItemSyncRef struct {
	gorm.Model
	HouseholdID        uint      `gorm:"index;not null" json:"householdId"`
	ShoppingListItemID uint      `gorm:"index" json:"shoppingListItemId"`
	Provider           string    `gorm:"index;not null" json:"provider"`
	ExternalID         string    `gorm:"index;not null" json:"externalId"`
	ListID             string    `json:"listId,omitempty"`
	SyncedAt           time.Time `json:"syncedAt"`
	Status             string    `gorm:"not null;default:'synced'" json:"status"`
}
