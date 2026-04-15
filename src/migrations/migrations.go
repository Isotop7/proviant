package migrations

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
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
	logger.Info().Msg("Running database migrations for backfilling product amounts")
	return db.Exec("UPDATE products SET amount = 1 WHERE amount IS NULL OR amount = 0").Error
}

func RunBreakingDatabaseMigrations(logger *zerolog.Logger, db *gorm.DB) error {
	// Migrations version 0.2.0
	logger.Info().Msg("Running database migrations for version 0.2.0")
	if err := assignHouseholdsToUsers(db); err != nil {
		return err
	}

	// Migrations for notification preferences
	logger.Info().Msg("Running database migrations for notification preferences")
	if err := AddNotificationPreferencesMigration(db); err != nil {
		return err
	}

	// Backfill email verification for existing users
	logger.Info().Msg("Running database migrations for email verification backfill")
	if err := BackfillEmailVerification(logger, db); err != nil {
		return err
	}

	return nil
}

// BackfillEmailVerification sets EmailVerifiedAt for all existing users that don't have it set.
// This is a one-time migration to ensure existing users aren't locked out after email verification is introduced.
func BackfillEmailVerification(logger *zerolog.Logger, db *gorm.DB) error {
	logger.Info().Msg("Running database migrations for backfilling email verification")

	result := db.Exec("UPDATE users SET email_verified_at = ? WHERE email_verified_at IS NULL", time.Now())
	if result.Error != nil {
		return result.Error
	}

	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled email verification")
	return nil
}
