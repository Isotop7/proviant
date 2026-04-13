package authentication

import (
	"time"

	"gorm.io/gorm"
)

type PersonalAccessToken struct {
	gorm.Model
	UserID     uint       `gorm:"index;not null"`
	Name       string     `gorm:"not null"`
	TokenHash  string     `gorm:"uniqueIndex;not null"`
	LastUsedAt *time.Time `gorm:"index"`
	ExpiresAt  *time.Time `gorm:"index"`
	Scopes     string     `gorm:"default:''"`
}
