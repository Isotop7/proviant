package migrations

import (
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestRunBreakingDatabaseMigrations tests the complete migration process
func TestRunBreakingDatabaseMigrations(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schemas
	if err := db.AutoMigrate(&authentication.User{}, &database.Household{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create test users without households
	user1 := authentication.User{Username: "user1", Password: "password1"}
	user2 := authentication.User{Username: "user2", Password: "password2"}
	db.Create(&user1)
	db.Create(&user2)

	// Verify users don't have households initially
	var dbUser1, dbUser2 authentication.User
	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	assert.Equal(t, uint(0), dbUser1.HouseholdID)
	assert.Equal(t, uint(0), dbUser2.HouseholdID)

	// Create mock logger
	logger := zerolog.Nop()

	// Run migrations
	err = RunBreakingDatabaseMigrations(&logger, db)
	assert.NoError(t, err)

	// Verify users now have households
	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	assert.NotEqual(t, uint(0), dbUser1.HouseholdID)
	assert.NotEqual(t, uint(0), dbUser2.HouseholdID)

	// Verify households were created
	var household1, household2 database.Household
	db.First(&household1, dbUser1.HouseholdID)
	db.First(&household2, dbUser2.HouseholdID)
	assert.Equal(t, user1.ID, household1.AdminID)
	assert.Equal(t, user2.ID, household2.AdminID)
	assert.Contains(t, household1.Name, user1.Username)
	assert.Contains(t, household2.Name, user2.Username)
}

// TestRunBreakingDatabaseMigrations_EmptyDatabase tests with empty database
func TestRunBreakingDatabaseMigrations_EmptyDatabase(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schemas
	if err := db.AutoMigrate(&authentication.User{}, &database.Household{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create mock logger
	logger := zerolog.Nop()

	// Run migrations on empty database
	err = RunBreakingDatabaseMigrations(&logger, db)
	assert.NoError(t, err)

	// Verify no households were created
	var count int64
	db.Model(&database.Household{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

// TestRunBreakingDatabaseMigrations_UsersWithHouseholds tests that users with existing households are not affected
func TestRunBreakingDatabaseMigrations_UsersWithHouseholds(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schemas
	if err := db.AutoMigrate(&authentication.User{}, &database.Household{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create a household
	household := database.Household{Name: "Existing Household"}
	db.Create(&household)

	// Create a user with an existing household
	user := authentication.User{
		Username:     "user1",
		Password:     "password1",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	// Store original household ID
	originalHouseholdID := user.HouseholdID

	// Create mock logger
	logger := zerolog.Nop()

	// Run migrations
	err = RunBreakingDatabaseMigrations(&logger, db)
	assert.NoError(t, err)

	// Verify user still has the same household
	var dbUser authentication.User
	db.First(&dbUser, user.ID)
	assert.Equal(t, originalHouseholdID, dbUser.HouseholdID)
}
