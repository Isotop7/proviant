package database

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// RepositoryContainer holds all repository interfaces.
// Injected into the Gin context under the key "repos" so handlers
// can depend on interfaces rather than constructing concrete types.
type RepositoryContainer struct {
	Products          ProductRepositoryInterface
	Users             UserRepositoryInterface
	Households        HouseholdRepositoryInterface
	Invitations       InvitationRepositoryInterface
	StorageLocations  StorageLocationRepositoryInterface
	Webhooks          WebhookRepositoryInterface
	PATs              PATRepositoryInterface
	Recipes           RecipeRepositoryInterface
	Savings           SavingsRepositoryInterface
	WasteAnalytics    *WasteAnalyticsRepository
	Notifications     NotificationRepositoryInterface
	Streaks           StreakRepositoryInterface
	ExpiryScan        ExpiryScanRepositoryInterface
	CalendarTokens    CalendarTokenRepositoryInterface
	AuditLogs         AuditLogRepositoryInterface
	ActivityLogs      ActivityLogRepositoryInterface
	ShoppingListItems ShoppingListItemRepository
}

// NewRepositoryContainer creates a RepositoryContainer backed by GORM implementations.
// logger may be nil (tests); the audit repository needs it to warn about
// entries that fail household attribution, because such an entry stays
// invisible to every household.
func NewRepositoryContainer(db *gorm.DB, logger *zerolog.Logger) *RepositoryContainer {
	return &RepositoryContainer{
		Products:          NewProductRepository(db),
		Users:             NewUserRepository(db),
		Households:        NewHouseholdRepository(db),
		Invitations:       NewInvitationRepository(db),
		StorageLocations:  NewStorageLocationRepository(db),
		Webhooks:          NewWebhookRepository(db),
		PATs:              NewPATRepository(db),
		Recipes:           NewRecipeRepository(db),
		Savings:           NewSavingsRepository(db),
		WasteAnalytics:    NewWasteAnalyticsRepository(db),
		Notifications:     NewNotificationRepository(db),
		Streaks:           NewStreakRepository(db),
		ExpiryScan:        NewExpiryScanRepository(db),
		CalendarTokens:    NewCalendarTokenRepository(db),
		AuditLogs:         NewAuditLogRepository(db, logger),
		ActivityLogs:      NewActivityLogRepository(db),
		ShoppingListItems: NewShoppingListItemRepository(db),
	}
}
