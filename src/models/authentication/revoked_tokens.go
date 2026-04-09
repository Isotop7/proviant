package authentication

import (
	"time"

	"gorm.io/gorm"
)

// RevokedToken represents a revoked JWT token identified by its JTI
type RevokedToken struct {
	gorm.Model
	JTI       string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
}
