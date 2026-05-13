package testutil

import (
	"path/filepath"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var testModels = []any{
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
	&dbModel.ProductCategoryPrice{},
}

func SetupTestDB(t *testing.T) *gorm.DB {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.db")

	db, err := gorm.Open(sqlite.Open(tempFile), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err = db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func MigrateAllModels(db *gorm.DB) error {
	return db.AutoMigrate(testModels...)
}
