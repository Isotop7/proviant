package database

import (
	"gorm.io/gorm"
)

type Household struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	AdminID     uint   `gorm:"not null"`
	Language    string `gorm:"default:''"`

	// RequireTwoFactor makes a second factor mandatory for every member of
	// this household. A member without an enrolled authenticator is sent into
	// the setup flow instead of being let in.
	RequireTwoFactor bool `gorm:"default:false"`

	MonthlyWasteGoalType    string   `gorm:"default:''"`   // "", "count", or "percent"
	MonthlyWasteGoalCount   *int     `gorm:"default:null"` // nil = disabled
	MonthlyWasteGoalPercent *float64 `gorm:"default:null"` // nil = disabled
}

// HouseholdWithMemberCount pairs a household with its current member count
type HouseholdWithMemberCount struct {
	Household
	MemberCount int
}
