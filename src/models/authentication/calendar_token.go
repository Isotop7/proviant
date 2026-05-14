package authentication

import (
	"time"

	"gorm.io/gorm"
)

type CalendarToken struct {
	gorm.Model
	UserID    uint      `gorm:"index, not null"`
	Token     string    `gorm:"uniqueIndex, not null"`
	ExpiresAt time.Time `gorm:"index, not null"`
}
