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
				ID                     string       `json:"_id"`
				ProductName            string       `json:"product_name"`
				Categories             string       `json:"categories"`
				Countries              string       `json:"countries"`
				GenericName            string       `json:"generic_name"`
				ImageURL               string       `json:"image_url"`
				EcoscoreData           EcoscoreData `json:"ecoscore_data"`
				ConservationConditions string       `json:"conservation_conditions"`
			}{
				ID:                     "product123",
				ProductName:            "Test Product",
				Categories:             "en:test",
				Countries:              "en:France",
				GenericName:            "Generic test product",
				ImageURL:               "http://example.com/image.jpg",
				ConservationConditions: "Store in a cool, dry place",
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
		if dataset.Product.ConservationConditions != "Store in a cool, dry place" {
			t.Errorf("Product.ConservationConditions = %v, want 'Store in a cool, dry place'", dataset.Product.ConservationConditions)
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
			"image_url": "http://example.com/image.jpg",
			"conservation_conditions": "Store in a cool, dry place"
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
	if dataset.Product.ConservationConditions != "Store in a cool, dry place" {
		t.Errorf("Product.ConservationConditions = %v, want 'Store in a cool, dry place'", dataset.Product.ConservationConditions)
	}
}

func TestOpenFoodFactsAPIDatasetWithPartialData(t *testing.T) {
	t.Run("dataset can handle partial data", func(t *testing.T) {
		dataset := OpenFoodFactsAPIDataset{
			Barcode: "1234567890123",
			Product: struct {
				ID                     string       `json:"_id"`
				ProductName            string       `json:"product_name"`
				Categories             string       `json:"categories"`
				Countries              string       `json:"countries"`
				GenericName            string       `json:"generic_name"`
				ImageURL               string       `json:"image_url"`
				EcoscoreData           EcoscoreData `json:"ecoscore_data"`
				ConservationConditions string       `json:"conservation_conditions"`
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
		if !contains(OpenFoodFactsAPIDatasetDefinition, "conservation_conditions") {
			t.Errorf("OpenFoodFactsAPIDatasetDefinition does not contain 'conservation_conditions'")
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

// TestFlexibleFloat64Unmarshal verifies the Agribalyse CO2 field accepts the
// number and numeric-string forms OpenFoodFacts exports in turn.
func TestFlexibleFloat64Unmarshal(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "number", input: "1.5", want: 1.5},
		{name: "string", input: `"1.5"`, want: 1.5},
		{name: "integer", input: "2", want: 2},
		{name: "empty string", input: `""`, want: 0},
		{name: "null", input: "null", want: 0},
		{name: "whitespace", input: "  3.25  ", want: 3.25},
		{name: "not a number", input: `"unknown"`, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var value FlexibleFloat64
			err := json.Unmarshal([]byte(testCase.input), &value)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = nil, want error", testCase.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s): %v", testCase.input, err)
			}
			if float64(value) != testCase.want {
				t.Errorf("value = %v, want %v", float64(value), testCase.want)
			}
		})
	}
}

// TestAgribalyseDataStringCO2 verifies the dump-shaped nesting unmarshals when
// co2_total is a string, which previously aborted a whole import.
func TestAgribalyseDataStringCO2(t *testing.T) {
	payload := `{"ecoscore_data":{"agribalyse":{"co2_total":"7.564940134"}}}`
	var product OpenFoodFactsDumpProduct
	if err := json.Unmarshal([]byte(payload), &product); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := float64(product.EcoscoreData.Agribalyse.CO2Total); got != 7.564940134 {
		t.Errorf("CO2Total = %v, want 7.564940134", got)
	}
}

// TestFlexibleStringUnmarshal verifies the barcode field accepts the string and
// number forms OpenFoodFacts exports, and rejects types that would store a
// garbage barcode.
func TestFlexibleStringUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "string", input: `"1234567890123"`, want: "1234567890123"},
		{name: "number", input: "1234567890123", want: "1234567890123"},
		{name: "empty string", input: `""`, want: ""},
		{name: "null", input: "null", want: ""},
		{name: "object", input: `{"unexpected":"object"}`, wantErr: true},
		{name: "array", input: `[1,2]`, wantErr: true},
		{name: "boolean", input: "true", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var value FlexibleString
			err := json.Unmarshal([]byte(testCase.input), &value)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = nil, want error", testCase.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s): %v", testCase.input, err)
			}
			if string(value) != testCase.want {
				t.Errorf("value = %q, want %q", string(value), testCase.want)
			}
		})
	}
}
