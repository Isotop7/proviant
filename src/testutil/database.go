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
	&dbModel.PasswordReset{},
	&dbModel.ProductCategoryPrice{},
	&dbModel.ActivityLog{},
	&dbModel.AuditLog{},
}

func SetupTestDB(t *testing.T) *gorm.DB {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.db")

	// Services fire activity log / savings event writes from background
	// goroutines while test code writes on the main goroutine. Without a
	// busy timeout those concurrent writes intermittently fail with
	// "database is locked"; _txlock=immediate additionally avoids the
	// deferred-transaction upgrade deadlock that busy_timeout cannot
	// retry.
	db, err := gorm.Open(sqlite.Open(tempFile+"?_busy_timeout=5000&_txlock=immediate&_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Services fire activity log / savings event writes from background
	// goroutines that can outlive the test. Close the pool before
	// t.TempDir() cleanup removes the directory (cleanups run LIFO, and
	// TempDir registered first): Close waits for in-flight queries, and
	// later goroutine writes fail with ErrPoolClosed instead of recreating
	// WAL files inside a directory that is being deleted — the source of
	// flaky "TempDir RemoveAll cleanup: directory not empty" failures.
	t.Cleanup(func() {
		if sqlDB, closeErr := db.DB(); closeErr == nil {
			_ = sqlDB.Close()
		}
	})

	if err = db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func MigrateAllModels(db *gorm.DB) error {
	return db.AutoMigrate(testModels...)
}
