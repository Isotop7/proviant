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
