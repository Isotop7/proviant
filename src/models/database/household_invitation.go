package database

import (
	"time"

	"gorm.io/gorm"
)

const (
	InvitationStatusPending   = "pending"
	InvitationStatusAccepted  = "accepted"
	InvitationStatusExpired   = "expired"
	InvitationStatusCancelled = "cancelled"
)

// HouseholdInvitation represents an invitation sent by a household member to invite someone by email
type HouseholdInvitation struct {
	gorm.Model
	HouseholdID uint      `gorm:"index,not null" json:"householdId"`
	InviterID   uint      `gorm:"index,not null" json:"inviterId"`
	Email       string    `gorm:"not null" json:"email"`
	Token       string    `gorm:"uniqueIndex,not null" json:"-"`
	Status      string    `gorm:"not null;default:'pending'" json:"status"`
	ExpiresAt   time.Time `gorm:"not null" json:"expiresAt"`
}
