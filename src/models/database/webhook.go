package database

import (
	"time"

	"gorm.io/gorm"
)

type Webhook struct {
	gorm.Model
	UserID uint   `gorm:"index;not null" json:"-"`
	URL    string `gorm:"not null" json:"url"`
	Secret string `gorm:"not null" json:"-"`
	Events string `gorm:"not null" json:"events"`
	// No gorm default tag: GORM drops zero-value bools on create when a default
	// is set, so Active=false would silently persist as true. Handlers set the
	// value explicitly.
	Active bool `json:"active"`
}

type WebhookDeliveryLog struct {
	gorm.Model
	WebhookID    uint      `gorm:"index;not null" json:"webhookId"`
	StatusCode   int       `json:"statusCode"`
	ResponseBody string    `json:"responseBody,omitempty"`
	Error        string    `json:"error,omitempty"`
	Attempt      int       `gorm:"not null" json:"attempt"`
	CreatedAt    time.Time `json:"createdAt"`
}
