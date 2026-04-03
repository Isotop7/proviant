package database

import "gorm.io/gorm"

// OpenFoodFactsCache stores cached responses from the OpenFoodFacts API keyed by barcode
type OpenFoodFactsCache struct {
	gorm.Model
	Barcode     string `gorm:"uniqueIndex;not null" json:"barcode"`
	ProductName string `json:"productName"`
	Categories  string `json:"categories"`
	Countries   string `json:"countries"`
	ImageURL    string `json:"imageUrl"`
}
