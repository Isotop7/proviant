package migrations

import (
	"fmt"

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

func RunBreakingDatabaseMigrations(logger *zerolog.Logger, db *gorm.DB) error {
	// Migrations version 0.2.0
	logger.Info().Msg("Running database migrations for version 0.2.0")
	if err := assignHouseholdsToUsers(db); err != nil {
		return err
	}

	return nil
}
