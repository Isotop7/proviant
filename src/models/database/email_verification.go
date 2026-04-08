package database

import (
	"time"

	"gorm.io/gorm"
)

const (
	EmailVerificationStatusPending  = "pending"
	EmailVerificationStatusVerified = "verified"
	EmailVerificationStatusExpired  = "expired"
)

type EmailVerification struct {
	gorm.Model
	UserID    uint      `gorm:"uniqueIndex,not null"`
	Token     string    `gorm:"uniqueIndex,not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Status    string    `gorm:"not null;default:'pending'"`
}
