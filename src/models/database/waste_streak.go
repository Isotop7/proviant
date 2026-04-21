package database

import (
	"time"

	"gorm.io/gorm"
)

type WasteStreak struct {
	gorm.Model
	HouseholdID     uint `gorm:"uniqueIndex;not null"`
	CurrentStreak   int  `gorm:"default:0"`
	LongestStreak   int  `gorm:"default:0"`
	LastCheckedDate time.Time
	LastWastedDate  *time.Time
}
