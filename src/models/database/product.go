package database

import (
	"time"

	"gorm.io/gorm"
)

const (
	RemovalReasonConsumed = "consumed"
	RemovalReasonWasted   = "wasted"
)

// Product is the database model of a product
type Product struct {
	gorm.Model
	Barcode           string           `gorm:"index:idx_products_barcode_household,priority:1" json:"barcode"`
	ProductName       string           `json:"productName"`
	Categories        string           `json:"categories"`
	Countries         string           `json:"countries"`
	ImageURL          string           `json:"imageUrl"`
	ExpireAt          time.Time        `gorm:"index" json:"expireAt"`
	ScannedAt         time.Time        `json:"scannedAt"`
	NotifiedAt        time.Time        `json:"notifiedAt"`
	DeletedAt         gorm.DeletedAt   `gorm:"index:idx_products_household_deleted,priority:2"`
	HouseholdID       uint             `gorm:"index;index:idx_products_household_deleted,priority:1;index:idx_products_barcode_household,priority:2;not null" json:"-"`
	Household         Household        `json:"-"`
	Amount            int              `json:"amount"`
	Unit              string           `json:"unit"`
	StorageLocationID *uint            `gorm:"index"                        json:"storageLocationId"`
	StorageLocation   *StorageLocation `gorm:"foreignKey:StorageLocationID" json:"storageLocation,omitempty"`
	PriceOverride     *float64         `gorm:"default:null"                 json:"priceOverride,omitempty"`
	CO2KgPerKg        *float64         `gorm:"default:null"                 json:"co2KgPerKg,omitempty"`
	RemovalReason     string           `gorm:"default:''"                   json:"removalReason"`
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
	ID                uint      `json:"ID"`
	ProductName       string    `json:"productName"`
	Categories        string    `json:"categories"`
	Countries         string    `json:"countries"`
	ImageURL          string    `json:"imageUrl"`
	ExpireAt          time.Time `json:"expireAt"`
	Amount            int       `json:"amount"`
	Unit              string    `json:"unit"`
	StorageLocationID *uint     `json:"storageLocationId"`
}
