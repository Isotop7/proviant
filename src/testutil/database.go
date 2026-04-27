package testutil

import (
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	err = db.AutoMigrate(
		&dbModel.Household{},
		&authentication.User{},
		&dbModel.Product{},
		&dbModel.StorageLocation{},
		&dbModel.Webhook{},
		&dbModel.WebhookDeliveryLog{},
		&dbModel.RecipeCache{},
		&dbModel.SavingsRecord{},
		&dbModel.WasteStreak{},
		&dbModel.OnboardingState{},
		&dbModel.HouseholdInvitation{},
		&dbModel.OpenFoodFactsCache{},
		&dbModel.ExpiryScan{},
		&dbModel.HouseholdApplication{},
		&dbModel.EmailVerification{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func MigrateAllModels(db *gorm.DB) error {
	return db.AutoMigrate(
		&dbModel.Household{},
		&authentication.User{},
		&dbModel.Product{},
		&dbModel.StorageLocation{},
		&dbModel.Webhook{},
		&dbModel.WebhookDeliveryLog{},
		&dbModel.RecipeCache{},
		&dbModel.SavingsRecord{},
		&dbModel.WasteStreak{},
		&dbModel.OnboardingState{},
		&dbModel.HouseholdInvitation{},
		&dbModel.OpenFoodFactsCache{},
		&dbModel.ExpiryScan{},
		&dbModel.HouseholdApplication{},
		&dbModel.EmailVerification{},
	)
}
