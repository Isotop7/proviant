package database

import (
	"time"

	"gorm.io/gorm"
)

type MailDigestUnsubscribeToken struct {
	gorm.Model
	MailDigestToken string    `gorm:"uniqueIndex, not null"`
	UserID          uint      `gorm:"index, not null"`
	CreatedAt       time.Time `json:"createdAt"`
}