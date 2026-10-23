// external provides model definitions from external parties
package external

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

var (
	// Query parameters for OpenFoodFacts API
	// This drastically minimizes the response from API calls and should match the properties from external.OpenFoodFactsAPIDataset
	OpenFoodFactsAPIDatasetDefinition = "product_name,categories,countries,generic_name,image_url,quantity,ecoscore_data,conservation_conditions"
)

// OpenFoodFactsAPIDataset represents the data model of a json respons from the OpenFoodFacts API
type OpenFoodFactsAPIDataset struct {
	gorm.Model
	Barcode string `json:"code"`
	Product struct {
		ID                     string       `json:"_id"`
		ProductName            string       `json:"product_name"`
		Categories             string       `json:"categories"`
		Countries              string       `json:"countries"`
		GenericName            string       `json:"generic_name"`
		ImageURL               string       `json:"image_url"`
		Quantity               string       `json:"quantity"`
		EcoscoreData           EcoscoreData `json:"ecoscore_data"`
		ConservationConditions string       `json:"conservation_conditions"`
	} `json:"product"`
}

type AgribalyseData struct {
	CO2Total FlexibleFloat64 `json:"co2_total"`
}

// FlexibleFloat64 accepts a JSON number or a numeric string. OpenFoodFacts
// stores the same Agribalyse value as either in different exports; a plain
// float64 rejects the string form and aborts a whole bulk import. An empty or
// null value is read as zero.
type FlexibleFloat64 float64

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexibleFloat64) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if trimmed == "" || trimmed == "null" {
		*f = 0
		return nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return err
	}
	*f = FlexibleFloat64(value)
	return nil
}

// FlexibleString accepts a JSON string or number. OpenFoodFacts exports the
// same barcode as either across dump versions; a plain string rejects the
// numeric form and drops the whole product. Objects, arrays and booleans are
// rejected so a malformed record is skipped rather than stored under a garbage
// barcode.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *FlexibleString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*s = ""
		return nil
	}
	switch trimmed[0] {
	case '"':
		var value string
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			return err
		}
		*s = FlexibleString(value)
		return nil
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return err
		}
		*s = FlexibleString(trimmed)
		return nil
	default:
		return fmt.Errorf("unsupported barcode JSON value: %s", trimmed)
	}
}

type EcoscoreData struct {
	Agribalyse AgribalyseData `json:"agribalyse"`
}

// OpenFoodFactsDumpProduct is one product object from the OpenFoodFacts JSONL
// bulk dump. The dump exposes products flat, unlike the API v2 response which
// nests them under "product", so this mirrors the fields Proviant uses directly
// on the object. It is what gets filtered and stored per barcode.
type OpenFoodFactsDumpProduct struct {
	Code                   FlexibleString `json:"code"`
	ProductName            string         `json:"product_name"`
	Quantity               string         `json:"quantity"`
	Categories             string         `json:"categories"`
	Countries              string         `json:"countries"`
	ImageURL               string         `json:"image_url"`
	ConservationConditions string         `json:"conservation_conditions"`
	EcoscoreData           EcoscoreData   `json:"ecoscore_data"`
}
