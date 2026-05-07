package controllers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
)

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	DoFunc func(*http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func newTestOFFController(serverURL string, timeout int) OpenFoodFactsAPIController {
	logger := zerolog.New(io.Discard)
	return OpenFoodFactsAPIController{
		Logger: &logger,
		Configuration: configuration.OpenFoodFactsConfiguration{
			URL:     serverURL,
			Timeout: timeout,
		},
	}
}

func assertOFFProductFields(t *testing.T, p *dbModel.Product, barcode, name, categories, countries, imageURL string) {
	t.Helper()
	if p.Barcode != barcode {
		t.Errorf("Barcode = %v, want %v", p.Barcode, barcode)
	}
	if p.ProductName != name {
		t.Errorf("ProductName = %v, want %v", p.ProductName, name)
	}
	if p.Categories != categories {
		t.Errorf("Categories = %v, want %v", p.Categories, categories)
	}
	if p.Countries != countries {
		t.Errorf("Countries = %v, want %v", p.Countries, countries)
	}
	if p.ImageURL != imageURL {
		t.Errorf("ImageURL = %v, want %v", p.ImageURL, imageURL)
	}
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

func TestGetDataset_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "1234567890123") {
			t.Errorf("Request path = %v, want to contain 1234567890123", r.URL.Path)
		}
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
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	controller := newTestOFFController(server.URL, 10)
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}
	assertOFFProductFields(t, &product, "1234567890123", "Test Product", "en:test", "en:Germany", "http://example.com/image.jpg")
}

func TestGetDataset_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code": "1234567890123"}`))
	}))
	defer server.Close()

	controller := newTestOFFController(server.URL, 1)
	_, err := controller.GetDataset("1234567890123")
	if err == nil {
		t.Errorf("GetDataset() error = nil, want timeout error")
		return
	}
	if err.Error() != "timeout occured" {
		t.Errorf("GetDataset() error = %v, want timeout occured", err.Error())
	}
}

func TestGetDataset_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	controller := newTestOFFController(server.URL, 10)
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}
	if product.Barcode != "" {
		t.Errorf("Barcode = %v, want empty string", product.Barcode)
	}
}

func TestGetDataset_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	controller := newTestOFFController(server.URL, 10)
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}
	if product.Barcode != "" {
		t.Errorf("Barcode = %v, want empty string", product.Barcode)
	}
}

// TestOpenFoodFactsAPIController_Configuration tests different configuration scenarios
func TestOpenFoodFactsAPIController_Configuration(t *testing.T) {
	t.Run("can create controller with minimum timeout", func(t *testing.T) {
		controller := newTestOFFController("https://world.openfoodfacts.org", 1)
		if controller.Configuration.Timeout != 1 {
			t.Errorf("Timeout = %v, want 1", controller.Configuration.Timeout)
		}
	})

	t.Run("can create controller with large timeout", func(t *testing.T) {
		controller := newTestOFFController("https://world.openfoodfacts.org", 60)
		if controller.Configuration.Timeout != 60 {
			t.Errorf("Timeout = %v, want 60", controller.Configuration.Timeout)
		}
	})
}

// TestOpenFoodFactsAPIController_EmptyResponse tests handling of empty responses
func TestOpenFoodFactsAPIController_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	controller := newTestOFFController(server.URL, 10)
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}
	if product.Barcode != "" {
		t.Errorf("Barcode = %v, want empty string", product.Barcode)
	}
}

// TestOpenFoodFactsAPIController_MissingFields tests handling of missing fields in response
func TestOpenFoodFactsAPIController_MissingFields(t *testing.T) {
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

	controller := newTestOFFController(server.URL, 10)
	product, err := controller.GetDataset("1234567890123")
	if err != nil {
		t.Errorf("GetDataset() error = %v, want nil", err)
	}
	if product.Barcode != "1234567890123" {
		t.Errorf("Barcode = %v, want 1234567890123", product.Barcode)
	}
	if product.ProductName != "" {
		t.Errorf("ProductName = %v, want empty string", product.ProductName)
	}
}
