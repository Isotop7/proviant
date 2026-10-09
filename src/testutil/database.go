package testutil

import (
	"path/filepath"
	"testing"
	"time"

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
	&authentication.RevokedToken{},
	&authentication.PersonalAccessToken{},
	&authentication.CalendarToken{},
	&dbModel.WebPushConfig{},
	&dbModel.MailDigestUnsubscribeToken{},
	&dbModel.ShoppingListItem{},
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
	// TempDir registered first): later goroutine writes fail with
	// ErrPoolClosed instead of recreating WAL files inside a directory
	// that is being deleted.
	//
	// Close does not wait for a query already executing on a busy
	// connection: that connection can recreate test.db-wal/-shm by path
	// after Close returns and while TempDir's RemoveAll runs, which is the
	// source of flaky "TempDir RemoveAll cleanup: directory not empty"
	// failures. Drain in-use connections (the busy timeout is 5000ms) and
	// leave a small grace for the driver to finish closing the file.
	//
	// honey: worst-case 5.1s per test when a goroutine pins a connection;
	// revisit (e.g. track background writes with a WaitGroup) if the suite
	// ever grows noticeably slower.
	t.Cleanup(func() {
		sqlDB, closeErr := db.DB()
		if closeErr != nil {
			return
		}
		_ = sqlDB.Close()
		deadline := time.Now().Add(5100 * time.Millisecond)
		drained := false
		for sqlDB.Stats().InUse > 0 {
			drained = true
			if time.Now().After(deadline) {
				t.Logf("SetupTestDB cleanup: %d connection(s) still in use after 5.1s; TempDir RemoveAll may race with a late WAL write", sqlDB.Stats().InUse)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		if drained {
			// Small grace for the driver to finish closing the file.
			time.Sleep(5 * time.Millisecond)
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
