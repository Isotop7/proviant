package database

import "time"

type WebPushConfig struct {
	ID         uint   `gorm:"primaryKey"`
	PublicKey  string `gorm:"not null"`
	PrivateKey string `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
