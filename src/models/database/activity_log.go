package database

import (
	"time"

	"gorm.io/gorm"
)

type ActivityLog struct {
	gorm.Model
	HouseholdID  uint      `gorm:"index;not null" json:"householdId"`
	UserID       *uint     `gorm:"index" json:"userId"`
	UserName     string    `gorm:"not null" json:"userName"`
	Action       string    `gorm:"index;not null" json:"action"`
	ProductID    uint      `gorm:"index" json:"productId"`
	ProductName  string    `gorm:"not null" json:"productName"`
	Quantity     int       `json:"quantity"`
	Timestamp    time.Time `gorm:"index;not null" json:"timestamp"`
}

const (
	ActivityActionAdd          = "add"
	ActivityActionConsume      = "consume"
	ActivityActionWaste        = "waste"
	ActivityActionRestore      = "restore"
	ActivityActionAmountChange = "amount_change"
)