package migrations

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunBreakingDatabaseMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	user1 := authentication.User{Username: "user1", Password: "password1"}
	user2 := authentication.User{Username: "user2", Password: "password2"}
	db.Create(&user1)
	db.Create(&user2)

	var dbUser1, dbUser2 authentication.User
	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	if dbUser1.HouseholdID != 0 {
		t.Errorf("dbUser1.HouseholdID = %v, want 0", dbUser1.HouseholdID)
	}
	if dbUser2.HouseholdID != 0 {
		t.Errorf("dbUser2.HouseholdID = %v, want 0", dbUser2.HouseholdID)
	}

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	if dbUser1.HouseholdID == 0 {
		t.Errorf("dbUser1.HouseholdID = %v, want non-zero", dbUser1.HouseholdID)
	}
	if dbUser2.HouseholdID == 0 {
		t.Errorf("dbUser2.HouseholdID = %v, want non-zero", dbUser2.HouseholdID)
	}

	var household1, household2 database.Household
	db.First(&household1, dbUser1.HouseholdID)
	db.First(&household2, dbUser2.HouseholdID)
	if household1.AdminID != user1.ID {
		t.Errorf("household1.AdminID = %v, want %v", household1.AdminID, user1.ID)
	}
	if household2.AdminID != user2.ID {
		t.Errorf("household2.AdminID = %v, want %v", household2.AdminID, user2.ID)
	}
	if len(household1.Name) < len(user1.Username) {
		t.Errorf("household1.Name = %v, want to contain %v", household1.Name, user1.Username)
	}
	if len(household2.Name) < len(user2.Username) {
		t.Errorf("household2.Name = %v, want to contain %v", household2.Name, user2.Username)
	}
}

func TestRunBreakingDatabaseMigrations_EmptyDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	var count int64
	db.Model(&database.Household{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %v, want 0", count)
	}
}

func TestRunBreakingDatabaseMigrations_UsersWithHouseholds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	household := database.Household{Name: "Existing Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "user1",
		Password:    "password1",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	originalHouseholdID := user.HouseholdID

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	var dbUser authentication.User
	db.First(&dbUser, user.ID)
	if dbUser.HouseholdID != originalHouseholdID {
		t.Errorf("dbUser.HouseholdID = %v, want %v", dbUser.HouseholdID, originalHouseholdID)
	}
}

// TestBackfillAuditLogHouseholdID covers the one-time attribution of audit
// entries written before household_id existed. Entries that cannot be
// attributed (unknown username, no user at all) must stay NULL.
func TestBackfillAuditLogHouseholdID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	household := database.Household{Name: "Household"}
	db.Create(&household)
	user := authentication.User{
		Username:    "user1",
		Password:    "password1",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	seed := func(userID *uint) database.AuditLog {
		row := database.AuditLog{
			Timestamp: time.Now(),
			UserID:    userID,
			Action:    database.AuditActionLoginSuccess,
			IPAddress: "10.0.0.1",
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("failed to seed audit log: %v", err)
		}
		return row
	}

	knownUser := seed(&user.ID)
	unknownUsername := uint(0)
	noUsernameMatch := seed(&unknownUsername)
	noUser := seed(nil)

	logger := zerolog.Nop()
	if err := BackfillAuditLogHouseholdID(&logger, db); err != nil {
		t.Fatalf("BackfillAuditLogHouseholdID() error = %v", err)
	}

	reload := func(row database.AuditLog) database.AuditLog {
		var reloaded database.AuditLog
		if err := db.First(&reloaded, row.ID).Error; err != nil {
			t.Fatalf("failed to reload audit log: %v", err)
		}
		return reloaded
	}

	if got := reload(knownUser); got.HouseholdID == nil || *got.HouseholdID != household.ID {
		t.Errorf("known user entry HouseholdID = %v, want %d", got.HouseholdID, household.ID)
	}
	if got := reload(noUsernameMatch); got.HouseholdID != nil {
		t.Errorf("unknown username entry HouseholdID = %v, want nil", *got.HouseholdID)
	}
	if got := reload(noUser); got.HouseholdID != nil {
		t.Errorf("userless entry HouseholdID = %v, want nil", *got.HouseholdID)
	}

	// Re-running must be a no-op, not a re-attribution.
	if err := BackfillAuditLogHouseholdID(&logger, db); err != nil {
		t.Fatalf("second BackfillAuditLogHouseholdID() error = %v", err)
	}
	if got := reload(knownUser); *got.HouseholdID != household.ID {
		t.Errorf("after re-run HouseholdID = %v, want %d", *got.HouseholdID, household.ID)
	}
}
