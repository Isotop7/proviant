package migrations

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// AssignHouseholdsToUsers ensures every user belongs to a household
// This fixes breaking changes of version 0.2.0
func assignHouseholdsToUsers(db *gorm.DB) error {
	// Fetch all users who are not assigned to a household
	var usersWithoutHouseholds []*authentication.User
	result := db.Where("household_id IS NULL or household_id = 0").Find(&usersWithoutHouseholds)
	if result.Error != nil {
		return fmt.Errorf("error fetching users without households: %v", result.Error)
	}

	// Assign each user a new household
	for _, user := range usersWithoutHouseholds {
		newHousehold := database.Household{
			Name:    fmt.Sprintf("%s's Household", user.Username),
			AdminID: user.ID, // Automatically set the user as the admin
		}

		// Create the household
		if err := db.Create(&newHousehold).Error; err != nil {
			return fmt.Errorf("error creating household for user '%s': %v", user.Username, err)
		}

		// Update the user's HouseholdID
		user.HouseholdID = newHousehold.ID
		if err := db.Save(&user).Error; err != nil {
			return fmt.Errorf("error updating user '%s' with household ID: %v", user.Username, err)
		}
	}

	return nil
}

// AddNotificationPreferencesMigration adds notification preference columns to users table
func AddNotificationPreferencesMigration(db *gorm.DB) error {
	// Try to add the columns - if they already exist, this will fail but that's okay
	err := db.Exec(`
		ALTER TABLE users
		ADD COLUMN email_enabled BOOLEAN DEFAULT TRUE,
		ADD COLUMN ntfy_enabled BOOLEAN DEFAULT FALSE,
		ADD COLUMN ntfy_url TEXT,
		ADD COLUMN ntfy_topic TEXT,
		ADD COLUMN ntfy_token TEXT
	`).Error

	// If the error is about duplicate columns, we can ignore it
	if err != nil && (strings.Contains(err.Error(), "duplicate column") ||
		strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "already has column")) {
		return nil
	}

	return err
}

// SetDefaultProductAmounts sets amount=1 for all existing products that have amount=0 or NULL.
// When the amount column is first added via AutoMigrate, existing rows receive NULL (not 0),
// so both cases must be handled.
func SetDefaultProductAmounts(logger *zerolog.Logger, db *gorm.DB) error {
	return db.Exec("UPDATE products SET amount = 1 WHERE amount IS NULL OR amount = 0").Error
}

// DropLegacyStorageLocationColumn removes the old free-text storage_location column from products.
// GORM AutoMigrate never drops columns, so this must be done explicitly.
// SQLite does not support IF EXISTS on DROP COLUMN, so we attempt the drop and swallow
// any error that indicates the column is already absent.
func DropLegacyStorageLocationColumn(logger *zerolog.Logger, db *gorm.DB) error {
	err := db.Exec(`ALTER TABLE products DROP COLUMN storage_location`).Error
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no such column") ||
			strings.Contains(errStr, "no such table") ||
			strings.Contains(errStr, "Unknown column") ||
			strings.Contains(errStr, "Can't DROP") ||
			strings.Contains(errStr, "syntax error") {
			logger.Debug().Msg("storage_location column not present or not droppable, skipping")
			return nil
		}
		return err
	}
	return nil
}

// SeedDefaultStorageLocations creates Fridge, Freezer and Pantry for every
// existing household that has no storage locations yet.
func SeedDefaultStorageLocations(logger *zerolog.Logger, db *gorm.DB) error {
	type defaultLocation struct {
		Name      string
		Icon      string
		SortOrder int
	}
	defaults := []defaultLocation{
		{"Fridge", "🧊", 0},
		{"Freezer", "❄️", 1},
		{"Pantry", "🗄️", 2},
	}

	var householdIDs []uint
	if err := db.Model(&database.Household{}).Pluck("id", &householdIDs).Error; err != nil {
		return fmt.Errorf("fetching household IDs: %w", err)
	}

	for _, hhID := range householdIDs {
		var count int64
		db.Model(&database.StorageLocation{}).Where(util.QueryHouseholdId, hhID).Count(&count)
		if count > 0 {
			continue
		}
		for _, d := range defaults {
			loc := database.StorageLocation{
				HouseholdID: hhID,
				Name:        d.Name,
				Icon:        d.Icon,
				SortOrder:   d.SortOrder,
			}
			if err := db.Create(&loc).Error; err != nil {
				return fmt.Errorf("seeding location '%s' for household %d: %w", d.Name, hhID, err)
			}
		}
	}
	return nil
}

// AddPerformanceIndexes creates missing indexes for product and user tables
// to improve query performance for common access patterns.
func AddPerformanceIndexes(logger *zerolog.Logger, db *gorm.DB) error {
	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_products_household_deleted ON products(household_id, deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_products_expire_at ON products(expire_at)",
		"CREATE INDEX IF NOT EXISTS idx_products_barcode_household ON products(barcode, household_id)",
		"CREATE INDEX IF NOT EXISTS idx_users_mail_address ON users(mail_address)",
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)",
	}
	for _, sql := range statements {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("creating index: %w (sql: %s)", err, sql)
		}
	}
	return nil
}

func RunBreakingDatabaseMigrations(logger *zerolog.Logger, db *gorm.DB) error {
	logger.Info().Msg("Running database migrations")

	logger.Debug().Msg("Running database migrations to assign households to users")
	if err := assignHouseholdsToUsers(db); err != nil {
		return err
	}

	// Migrations for notification preferences
	logger.Debug().Msg("Running database migrations for notification preferences")
	if err := AddNotificationPreferencesMigration(db); err != nil {
		return err
	}

	// Backfill email verification for existing users
	logger.Debug().Msg("Running database migrations for email verification backfill")
	if err := BackfillEmailVerification(logger, db); err != nil {
		return err
	}

	// Drop legacy free-text storage_location column from products
	logger.Debug().Msg("Drop legacy storage location columns")
	if err := DropLegacyStorageLocationColumn(logger, db); err != nil {
		return err
	}

	// Seed default storage locations for existing households
	logger.Debug().Msg("Backfill database with default storage locations")
	if err := SeedDefaultStorageLocations(logger, db); err != nil {
		return err
	}

	// Add performance indexes
	logger.Debug().Msg("Add performance indexes")
	if err := AddPerformanceIndexes(logger, db); err != nil {
		return err
	}

	// Backfill removal_reason for existing consumed products
	logger.Debug().Msg("Backfill removal_reason for consumed products")
	if err := BackfillRemovalReason(logger, db); err != nil {
		return err
	}

	return nil
}

// BackfillRemovalReason sets removal_reason = "consumed" for all existing soft-deleted products
// that have no removal reason set. These predate the RemovalReason field and were all consumed
// (wasted products were hard-deleted at the time and therefore absent from the table).
func BackfillRemovalReason(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Unscoped().Exec(
		"UPDATE products SET removal_reason = ? WHERE deleted_at IS NOT NULL AND (removal_reason IS NULL OR removal_reason = '')",
		database.RemovalReasonConsumed,
	)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled removal_reason for consumed products")
	return nil
}

// BackfillEmailVerification sets EmailVerifiedAt for all existing users that don't have it set.
// This is a one-time migration to ensure existing users aren't locked out after email verification is introduced.
func BackfillEmailVerification(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec("UPDATE users SET email_verified_at = ? WHERE email_verified_at IS NULL", time.Now())
	if result.Error != nil {
		return result.Error
	}

	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled email verification")
	return nil
}
