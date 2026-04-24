// external provides model definitions from external parties
package external

import "gorm.io/gorm"

var (
	// Query parameters for OpenFoodFacts API
	// This drastically minimizes the response from API calls and should match the properties from external.OpenFoodFactsAPIDataset
	OpenFoodFactsAPIDatasetDefinition = "product_name,categories,countries,generic_name,image_url,ecoscore_data"
)

// OpenFoodFactsAPIDataset represents the data model of a json respons from the OpenFoodFacts API
type OpenFoodFactsAPIDataset struct {
	gorm.Model
	Barcode string `json:"code"`
	Product struct {
		ID           string `json:"_id"`
		ProductName  string `json:"product_name"`
		Categories   string `json:"categories"`
		Countries    string `json:"countries"`
		GenericName  string `json:"generic_name"`
		ImageURL     string `json:"image_url"`
		EcoscoreData struct {
			Agribalyse struct {
				CO2Total float64 `json:"co2_total"`
			} `json:"agribalyse"`
		} `json:"ecoscore_data"`
	} `json:"product"`
}
