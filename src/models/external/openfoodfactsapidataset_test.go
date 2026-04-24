package external

import (
	"encoding/json"
	"testing"
)

func TestOpenFoodFactsAPIDatasetStruct(t *testing.T) {
	t.Run("OpenFoodFactsAPIDataset has correct structure", func(t *testing.T) {
		dataset := OpenFoodFactsAPIDataset{
			Barcode: "1234567890123",
			Product: struct {
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
			}{
				ID:          "product123",
				ProductName: "Test Product",
				Categories:  "en:test",
				Countries:   "en:France",
				GenericName: "Generic test product",
				ImageURL:    "http://example.com/image.jpg",
			},
		}

		if dataset.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", dataset.Barcode)
		}
		if dataset.Product.ID != "product123" {
			t.Errorf("Product.ID = %v, want product123", dataset.Product.ID)
		}
		if dataset.Product.ProductName != "Test Product" {
			t.Errorf("Product.ProductName = %v, want Test Product", dataset.Product.ProductName)
		}
		if dataset.Product.Categories != "en:test" {
			t.Errorf("Product.Categories = %v, want en:test", dataset.Product.Categories)
		}
		if dataset.Product.Countries != "en:France" {
			t.Errorf("Product.Countries = %v, want en:France", dataset.Product.Countries)
		}
		if dataset.Product.GenericName != "Generic test product" {
			t.Errorf("Product.GenericName = %v, want Generic test product", dataset.Product.GenericName)
		}
		if dataset.Product.ImageURL != "http://example.com/image.jpg" {
			t.Errorf("Product.ImageURL = %v, want http://example.com/image.jpg", dataset.Product.ImageURL)
		}
	})
}

func TestOpenFoodFactsAPIDatasetJSONUnmarshal(t *testing.T) {
	jsonData := `{
		"code": "1234567890123",
		"product": {
			"_id": "product123",
			"product_name": "Test Product",
			"categories": "en:test",
			"countries": "en:France",
			"generic_name": "Generic test product",
			"image_url": "http://example.com/image.jpg"
		}
	}`

	var dataset OpenFoodFactsAPIDataset
	err := json.Unmarshal([]byte(jsonData), &dataset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dataset.Barcode != "1234567890123" {
		t.Errorf("Barcode = %v, want 1234567890123", dataset.Barcode)
	}
	if dataset.Product.ID != "product123" {
		t.Errorf("Product.ID = %v, want product123", dataset.Product.ID)
	}
	if dataset.Product.ProductName != "Test Product" {
		t.Errorf("Product.ProductName = %v, want Test Product", dataset.Product.ProductName)
	}
	if dataset.Product.Categories != "en:test" {
		t.Errorf("Product.Categories = %v, want en:test", dataset.Product.Categories)
	}
	if dataset.Product.Countries != "en:France" {
		t.Errorf("Product.Countries = %v, want en:France", dataset.Product.Countries)
	}
	if dataset.Product.GenericName != "Generic test product" {
		t.Errorf("Product.GenericName = %v, want Generic test product", dataset.Product.GenericName)
	}
	if dataset.Product.ImageURL != "http://example.com/image.jpg" {
		t.Errorf("Product.ImageURL = %v, want http://example.com/image.jpg", dataset.Product.ImageURL)
	}
}

func TestOpenFoodFactsAPIDatasetWithPartialData(t *testing.T) {
	t.Run("dataset can handle partial data", func(t *testing.T) {
		dataset := OpenFoodFactsAPIDataset{
			Barcode: "1234567890123",
			Product: struct {
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
			}{
				ProductName: "Only Name",
			},
		}

		if dataset.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", dataset.Barcode)
		}
		if dataset.Product.ProductName != "Only Name" {
			t.Errorf("Product.ProductName = %v, want Only Name", dataset.Product.ProductName)
		}
		if dataset.Product.Categories != "" {
			t.Errorf("Product.Categories should be empty, got %v", dataset.Product.Categories)
		}
		if dataset.Product.Countries != "" {
			t.Errorf("Product.Countries should be empty, got %v", dataset.Product.Countries)
		}
	})
}

func TestOpenFoodFactsAPIDatasetEmptyFields(t *testing.T) {
	t.Run("dataset can have empty fields", func(t *testing.T) {
		dataset := OpenFoodFactsAPIDataset{}

		if dataset.Barcode != "" {
			t.Errorf("Barcode should be empty, got %v", dataset.Barcode)
		}
		if dataset.Product.ID != "" {
			t.Errorf("Product.ID should be empty, got %v", dataset.Product.ID)
		}
		if dataset.Product.ProductName != "" {
			t.Errorf("Product.ProductName should be empty, got %v", dataset.Product.ProductName)
		}
	})
}

func TestOpenFoodFactsAPIDatasetConstant(t *testing.T) {
	t.Run("OpenFoodFactsAPIDatasetDefinition is set", func(t *testing.T) {
		if OpenFoodFactsAPIDatasetDefinition == "" {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition is empty")
		}
		if !contains(OpenFoodFactsAPIDatasetDefinition, "product_name") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'product_name'")
		}
		if !contains(OpenFoodFactsAPIDatasetDefinition, "categories") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'categories'")
		}
		if !contains(OpenFoodFactsAPIDatasetDefinition, "countries") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'countries'")
		}
		if !contains(OpenFoodFactsAPIDatasetDefinition, "generic_name") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'generic_name'")
		}
		if !contains(OpenFoodFactsAPIDatasetDefinition, "image_url") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'image_url'")
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
