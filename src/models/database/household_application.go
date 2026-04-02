package database

import "gorm.io/gorm"

const (
	ApplicationStatusPending  = "pending"
	ApplicationStatusApproved = "approved"
	ApplicationStatusRejected = "rejected"
)

// HouseholdApplication represents a user's request to join a household
type HouseholdApplication struct {
	gorm.Model
	ApplicantID uint   `gorm:"index,not null" json:"applicantId"`
	HouseholdID uint   `gorm:"index,not null" json:"householdId"`
	Status      string `gorm:"not null;default:'pending'" json:"status"`
}
