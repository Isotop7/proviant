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
	Active bool   `gorm:"default:true" json:"active"`
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
