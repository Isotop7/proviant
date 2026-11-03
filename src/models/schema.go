package models

import (
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
)

// AllModels returns every model the application stores. The application
// migrates the list on startup, the tests migrate it for their in-memory
// database and the backup drill migrates it for its scratch database: one
// list, so a drill restores against the schema production runs on.
//
// A new model joins this list, not the migration call in src/proviant.go —
// omitting it from here means a runtime "no such table".
func AllModels() []any {
	return []any{
		&dbModel.Household{},
		&dbModel.StorageLocation{},
		&authentication.User{},
		&authentication.RevokedToken{},
		&authentication.PersonalAccessToken{},
		&authentication.CalendarToken{},
		&dbModel.Product{},
		&dbModel.ProductMutationClaim{},
		&dbModel.HouseholdApplication{},
		&dbModel.HouseholdInvitation{},
		&dbModel.OnboardingState{},
		&dbModel.OpenFoodFactsCache{},
		&dbModel.RecipeCache{},
		&dbModel.EmailVerification{},
		&dbModel.PasswordReset{},
		&dbModel.Webhook{},
		&dbModel.WebhookDeliveryLog{},
		&dbModel.WasteStreak{},
		&dbModel.ProductCategoryPrice{},
		&dbModel.HouseholdCategory{},
		&dbModel.SavingsRecord{},
		&dbModel.ModelCutover{},
		&dbModel.ExpiryScan{},
		&dbModel.AuditLog{},
		&dbModel.ActivityLog{},
		&dbModel.WebPushConfig{},
		&dbModel.MailDigestUnsubscribeToken{},
		&dbModel.ShoppingListItem{},
		&dbModel.HouseholdIntegration{},
		&dbModel.ShoppingListItemSyncRef{},
		&dbModel.Recipe{},
		&dbModel.RecipeIngredient{},
		&dbModel.MealPlanEntry{},
		&dbModel.TwoFactorRecoveryCode{},
	}
}
