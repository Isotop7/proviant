package database

import (
	"gorm.io/gorm"
)

type Household struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	AdminID     uint `gorm:"not null"`

	MonthlyWasteGoalType    string   `gorm:"default:''"`   // "", "count", or "percent"
	MonthlyWasteGoalCount   *int     `gorm:"default:null"` // nil = disabled
	MonthlyWasteGoalPercent *float64 `gorm:"default:null"` // nil = disabled
}

// HouseholdWithMemberCount pairs a household with its current member count
type HouseholdWithMemberCount struct {
	Household
	MemberCount int
}
