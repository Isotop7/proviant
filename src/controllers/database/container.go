package database

import "gorm.io/gorm"

// RepositoryContainer holds all repository interfaces.
// Injected into the Gin context under the key "repos" so handlers
// can depend on interfaces rather than constructing concrete types.
type RepositoryContainer struct {
	Products         ProductRepositoryInterface
	Users            UserRepositoryInterface
	Households       HouseholdRepositoryInterface
	Invitations      InvitationRepositoryInterface
	StorageLocations StorageLocationRepositoryInterface
	Webhooks         WebhookRepositoryInterface
	PATs             PATRepositoryInterface
	Recipes          RecipeRepositoryInterface
	Savings          SavingsRepositoryInterface
	Notifications    NotificationRepositoryInterface
	Streaks          StreakRepositoryInterface
	ExpiryScan       ExpiryScanRepositoryInterface
	CalendarTokens   CalendarTokenRepositoryInterface
	AuditLogs        AuditLogRepositoryInterface
}

// NewRepositoryContainer creates a RepositoryContainer backed by GORM implementations.
func NewRepositoryContainer(db *gorm.DB) *RepositoryContainer {
	return &RepositoryContainer{
		Products:         NewProductRepository(db),
		Users:            NewUserRepository(db),
		Households:       NewHouseholdRepository(db),
		Invitations:      NewInvitationRepository(db),
		StorageLocations: NewStorageLocationRepository(db),
		Webhooks:         NewWebhookRepository(db),
		PATs:             NewPATRepository(db),
		Recipes:          NewRecipeRepository(db),
		Savings:          NewSavingsRepository(db),
		Notifications:    NewNotificationRepository(db),
		Streaks:          NewStreakRepository(db),
		ExpiryScan:       NewExpiryScanRepository(db),
		CalendarTokens:   NewCalendarTokenRepository(db),
		AuditLogs:        NewAuditLogRepository(db),
	}
}
