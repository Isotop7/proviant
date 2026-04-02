package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/database"
)

// DatabaseControllerInterface defines the interface for database operations needed by other controllers
type DatabaseControllerInterface interface {
	GetProductsExpiredAndNotificationPending(sleepInterval time.Duration) ([]database.Product, error)
	GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error)
	GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error)
	SetProductNotifiedAt(productID uint) error
	CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error)
	GetInvitationsForHousehold(householdID uint) ([]database.HouseholdInvitation, error)
	GetInvitationByToken(token string) (database.HouseholdInvitation, error)
	AcceptInvitation(token, email string, userID uint) error
	CancelInvitation(invitationID, userID uint) error
}

// Ensure that DatabaseController implements the interface
var _ DatabaseControllerInterface = (*DatabaseController)(nil)
