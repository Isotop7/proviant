package database

import (
	"time"

	"gorm.io/gorm"
)

// Product is the database model of a product
type Product struct {
	gorm.Model
	Barcode     string         `json:"barcode"`
	ProductName string         `json:"productName"`
	Categories  string         `json:"categories"`
	Countries   string         `json:"countries"`
	ImageURL    string         `json:"imageUrl"`
	ExpireAt    time.Time      `json:"expireAt"`
	ScannedAt   time.Time      `json:"scannedAt"`
	NotifiedAt  time.Time      `json:"notifiedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	HouseholdID uint           `gorm:"index, not null" json:"-"`
	Household   Household      `json:"-"`
}

// ProductDTOExpire is a simplified DTO for product expiration
type ProductDTOExpire struct {
	ID       uint   `json:"id"`
	Barcode  string `json:"barcode"`
	ExpireAt Date   `json:"expireAt"`
}

// ProductDTOBarcode is a simplified DTO only containing a barcode
type ProductDTOBarcode struct {
	Barcode string `json:"barcode"`
}

// ProductDTOPatch is a simplified DTO only containing the patchable elements
type ProductDTOPatch struct {
	ID          uint      `json:"ID"`
	ProductName string    `json:"productName"`
	Categories  string    `json:"categories"`
	Countries   string    `json:"countries"`
	ImageURL    string    `json:"imageUrl"`
	ExpireAt    time.Time `json:"expireAt"`
}
