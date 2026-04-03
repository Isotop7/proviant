package database

import (
	"gorm.io/gorm"
)

type Household struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	AdminID     uint `gorm:"not null"`
}

// HouseholdWithMemberCount pairs a household with its current member count
type HouseholdWithMemberCount struct {
	Household
	MemberCount int
}
