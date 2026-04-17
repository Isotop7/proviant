package database

import "gorm.io/gorm"

// OnboardingState tracks which onboarding steps a user has completed
type OnboardingState struct {
	gorm.Model
	UserID              uint `gorm:"uniqueIndex,not null"`
	ProfileStepDone     bool `gorm:"default:false"`
	NotificationsSetup  bool `gorm:"default:false"`
	HouseholdStepDone   bool `gorm:"default:false"`
	OnboardingCompleted bool `gorm:"default:false"`
}
