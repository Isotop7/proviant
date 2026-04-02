package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
)

// DatabaseControllerInterface defines the interface for database operations needed by other controllers
type DatabaseControllerInterface interface {
	GetProductsExpiredAndNotificationPending(sleepInterval time.Duration) ([]database.Product, error)
	GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error)
	GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error)
	SetProductNotifiedAt(productID uint) error
	CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error)
	GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error)
	GetInvitationByToken(token string) (database.HouseholdInvitation, error)
	AcceptInvitation(token, email string, userID uint) error
	CancelInvitation(invitationID, userID uint) error
	GetUserByID(userID uint) (authentication.User, error)
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error)
	MarkInvitationSent(invitationID uint) error
	MarkInvitationSendFailed(invitationID uint) error
}

// Ensure that DatabaseController implements the interface
var _ DatabaseControllerInterface = (*DatabaseController)(nil)
