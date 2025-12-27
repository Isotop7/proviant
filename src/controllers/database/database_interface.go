package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models/database"
)

// DatabaseControllerInterface defines the interface for database operations needed by other controllers
type DatabaseControllerInterface interface {
	GetProductsExpiredAndNotificationPending(sleepInterval time.Duration) ([]database.Product, error)
	GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error)
	SetProductNotifiedAt(productID uint) error
}

// Ensure that DatabaseController implements the interface
var _ DatabaseControllerInterface = (*DatabaseController)(nil)
