package database

import (
	"time"

	"gorm.io/gorm"
)

type ActivityLog struct {
	gorm.Model
	HouseholdID uint   `gorm:"index;not null" json:"householdId"`
	UserID      *uint  `gorm:"index" json:"userId"`
	UserName    string `gorm:"not null" json:"userName"`
	Action      string `gorm:"index;not null" json:"action"`
	ProductID   uint   `gorm:"index" json:"productId"`
	ProductName string `gorm:"not null" json:"productName"`
	Quantity    int    `json:"quantity"`
	// WasteCause is why the action wasted the product. It is empty for every
	// action but a waste that carried a reason.
	WasteCause string    `gorm:"default:''" json:"wasteCause"`
	Timestamp  time.Time `gorm:"index;not null" json:"timestamp"`
}

const (
	ActivityActionAdd     = "add"
	ActivityActionConsume = "consume"
	ActivityActionWaste   = "waste"
	ActivityActionRestore = "restore"
	ActivityActionCook    = "cook"
	// ActivityActionQuantityChange adjusts a product's pack count. The
	// stored token is the legacy "amount_change": the activity feed and the
	// rows already in the database key on it.
	ActivityActionQuantityChange = "amount_change"
	ActivityActionStreakReset    = "streak_reset"
	ActivityActionImport         = "import"
)
