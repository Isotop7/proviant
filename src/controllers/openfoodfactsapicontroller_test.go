package controllers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"

	"github.com/rs/zerolog"
)

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	DoFunc func(*http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// TestOpenFoodFactsAPIController_Struct tests the OpenFoodFactsAPIController struct
func TestOpenFoodFactsAPIController_Struct(t *testing.T) {
	t.Run("can create OpenFoodFactsAPIController", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 10,
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		if controller.Configuration.URL != "https://world.openfoodfacts.org" {
			t.Errorf("URL = %v, want https://world.openfoodfacts.org", controller.Configuration.URL)
		}

		if controller.Configuration.Timeout != 10 {
			t.Errorf("Timeout = %v, want 10", controller.Configuration.Timeout)
		}
	})
}

// TestOpenFoodFactsAPIController_GetDataset tests the GetDataset method
func TestOpenFoodFactsAPIController_GetDataset(t *testing.T) {
	t.Run("can get dataset successfully", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 10,
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request path contains the expected barcode
			if !strings.Contains(r.URL.Path, "1234567890123") {
				t.Errorf("Request path = %v, want to contain 1234567890123", r.URL.Path)
			}

			// Send a mock response
			mockResponse := `{
				"code": "1234567890123",
				"product": {
					"_id": "1234567890123",
					"product_name": "Test Product",
					"categories": "en:test",
					"countries": "en:Germany",
					"image_url": "http://example.com/image.jpg"
				}
			}`
			w.WriteHeader(http.StatusOK)
			_,_ = w.Write([]byte(mockResponse))
		}))
		defer server.Close()

		// Update the controller URL to use the test server
		controller.Configuration.URL = server.URL

		// Call GetDataset
		product, err := controller.GetDataset("1234567890123")
		if err != nil {
			t.Errorf("GetDataset() error = %v, want nil", err)
		}

		// Verify the product data
		if product.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", product.Barcode)
		}

		if product.ProductName != "Test Product" {
			t.Errorf("ProductName = %v, want Test Product", product.ProductName)
		}

		if product.Categories != "en:test" {
			t.Errorf("Categories = %v, want en:test", product.Categories)
		}

		if product.Countries != "en:Germany" {
			t.Errorf("Countries = %v, want en:Germany", product.Countries)
		}

		if product.ImageURL != "http://example.com/image.jpg" {
			t.Errorf("ImageURL = %v, want http://example.com/image.jpg", product.ImageURL)
		}
	})

	t.Run("handles timeout correctly", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 1, // 1 second timeout
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		// Create a test server that delays responses
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Delay longer than timeout
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code": "1234567890123"}`))
		}))
		defer server.Close()

		// Update the controller URL to use the test server
		controller.Configuration.URL = server.URL

		// Call GetDataset - should timeout
		_, err := controller.GetDataset("1234567890123")
		if err == nil {
			t.Errorf("GetDataset() error = nil, want timeout error")
		}

		if err.Error() != "timeout occured" {
			t.Errorf("GetDataset() error = %v, want timeout occured", err.Error())
		}
	})

	t.Run("handles HTTP errors", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 10,
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		// Create a test server that returns an error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		// Update the controller URL to use the test server
		controller.Configuration.URL = server.URL

		// Call GetDataset - should handle HTTP error
		product, err := controller.GetDataset("1234567890123")
		if err != nil {
			t.Errorf("GetDataset() error = %v, want nil", err)
		}

		// Should return empty product on HTTP error
		if product.Barcode != "" {
			t.Errorf("Barcode = %v, want empty string", product.Barcode)
		}
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 10,
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		// Create a test server that returns invalid JSON
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		// Update the controller URL to use the test server
		controller.Configuration.URL = server.URL

		// Call GetDataset - should handle invalid JSON
		product, err := controller.GetDataset("1234567890123")
		if err != nil {
			t.Errorf("GetDataset() error = %v, want nil", err)
		}

		// Should return empty product on JSON error
		if product.Barcode != "" {
			t.Errorf("Barcode = %v, want empty string", product.Barcode)
		}
	})
}

// TestOpenFoodFactsAPIController_Configuration tests different configuration scenarios
func TestOpenFoodFactsAPIController_Configuration(t *testing.T) {
	t.Run("can create controller with minimum timeout", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 1, // Minimum timeout
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		if controller.Configuration.Timeout != 1 {
			t.Errorf("Timeout = %v, want 1", controller.Configuration.Timeout)
		}
	})

	t.Run("can create controller with large timeout", func(t *testing.T) {
		logger := zerolog.New(io.Discard)
		config := configuration.OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 60, // Large timeout
		}

		controller := OpenFoodFactsAPIController{
			Logger:        &logger,
			Configuration: config,
		}

		if controller.Configuration.Timeout != 60 {
			t.Errorf("Timeout = %v, want 60", controller.Configuration.Timeout)
		}
	})
}

// TestOpenFoodFactsAPIController_EmptyResponse tests handling of empty responses
func TestOpenFoodFactsAPIController_EmptyResponse(t *testing.T) {
	logger := zerolog.New(io.Discard)
	config := configuration.OpenFoodFactsConfiguration{
		URL:     "https://world.openfoodfacts.org",
		Timeout: 10,
	}

	controller := OpenFoodFactsAPIController{
		Logger:        &logger,
		Configuration: config,
	}

	// Create a test server that returns empty response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// Update the controller URL to use the test server
	controller.Configuration.URL = server.URL

	// Call GetDataset - should handle empty response
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}

	// Should return empty product on empty response
	if product.Barcode != "" {
		t.Errorf("Barcode = %v, want empty string", product.Barcode)
	}
}

// TestOpenFoodFactsAPIController_MissingFields tests handling of missing fields in response
func TestOpenFoodFactsAPIController_MissingFields(t *testing.T) {
	logger := zerolog.New(io.Discard)
	config := configuration.OpenFoodFactsConfiguration{
		URL:     "https://world.openfoodfacts.org",
		Timeout: 10,
	}

	controller := OpenFoodFactsAPIController{
		Logger:        &logger,
		Configuration: config,
	}

	// Create a test server that returns response with missing fields
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": "1234567890123",
			"product": {
				"_id": "1234567890123"
			}
		}`))
	}))
	defer server.Close()

	// Update the controller URL to use the test server
	controller.Configuration.URL = server.URL

	// Call GetDataset - should handle missing fields
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}

	// Should return product with barcode from response, but empty fields for missing data
	if product.Barcode != "1234567890123" {
		t.Errorf("Barcode = %v, want 1234567890123", product.Barcode)
	}

	if product.ProductName != "" {
		t.Errorf("ProductName = %v, want empty string", product.ProductName)
	}
}
