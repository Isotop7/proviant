package external

import "gorm.io/gorm"

type OpenFoodFactsAPIDataset struct {
	gorm.Model
	Barcode string `json:"code"`
	Product struct {
		ID          string `json:"_id"`
		ProductName string `json:"product_name"`
		Categories  string `json:"categories"`
		Countries   string `json:"countries"`
		GenericName string `json:"generic_name"`
		ImageURL    string `json:"image_url"`
	} `json:"product"`
}
