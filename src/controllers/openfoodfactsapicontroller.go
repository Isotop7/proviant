package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/models/external"
	"github.com/rs/zerolog"
)

// OpenFoodFactsAPIControllerInterface defines the contract for interacting with OpenFoodFacts
type OpenFoodFactsAPIControllerInterface interface {
	GetDataset(barcode string) (database.Product, error)
}

// OpenFoodFactsAPIController is the object struct for interacting with the API of OpenFoodFacts
// It uses the given configuration for accessing the API
type OpenFoodFactsAPIController struct {
	Logger        *zerolog.Logger
	Configuration configuration.OpenFoodFactsConfiguration
}

// GetDataset gets data from OpenFoodFacts by its API. The search parameter is the barcode of the product
func (offacntrl OpenFoodFactsAPIController) GetDataset(barcode string) (database.Product, error) {
	// Create a context with a timeout
	queryTimeout := time.Second * time.Duration(offacntrl.Configuration.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	// Channel to receive the response or timeout signal
	queryChannel := make(chan bool)
	// Dataset to store query response
	var dataset external.OpenFoodFactsAPIDataset
	filteredDataset := external.OpenFoodFactsAPIDatasetDefinition

	// Query API for dataset
	go func() {
		queryURL := fmt.Sprintf("%s/%s?fields=%s", offacntrl.Configuration.URL, barcode, filteredDataset)
		resp, err := http.Get(queryURL)
		// If upstream error is received, we also throw it
		if err != nil {
			offacntrl.Logger.Error().Msgf("Error decoding response: %s", err)
			queryChannel <- false
			return
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			offacntrl.Logger.Error().Msgf("Error reading response body: %s", err)
			queryChannel <- false
			return
		}

		// Anonymous function to close response body
		defer func() {
			if err := resp.Body.Close(); err != nil {
				offacntrl.Logger.Error().Msgf("Error closing response body: %s", err)
			}
		}()

		// Parse the response and populate the dataset struct
		if err := json.Unmarshal(body, &dataset); err != nil {
			offacntrl.Logger.Error().Msgf("Error decoding response: %s", err)
		}
		queryChannel <- true
	}()

	// Wait for either a response or a timeout
	select {
	case <-ctx.Done():
		// Timeout occured
		return database.Product{}, errors.New("timeout occured")
	case success := <-queryChannel:
		if success {
			// Request was successful, returning subset of populated dataset
			var co2 *float64
			if dataset.Product.EcoscoreData.Agribalyse.CO2Total > 0 {
				v := dataset.Product.EcoscoreData.Agribalyse.CO2Total
				co2 = &v
			}
			return database.Product{
				Barcode:     dataset.Barcode,
				ProductName: dataset.Product.ProductName,
				Categories:  dataset.Product.Categories,
				Countries:   dataset.Product.Countries,
				ImageURL:    dataset.Product.ImageURL,
				CO2KgPerKg:  co2,
			}, nil
		}
	}

	return database.Product{}, nil
}

// DownloadImage fetches an image from imageURL and saves it to {cachePath}/{barcode}.jpg.
// Returns the local serve path /product-images/{barcode}.jpg on success.
func (offacntrl OpenFoodFactsAPIController) DownloadImage(imageURL, barcode string) (string, error) {
	if err := os.MkdirAll(offacntrl.Configuration.ImageCachePath, 0755); err != nil {
		return "", fmt.Errorf("creating image cache dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(offacntrl.Configuration.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating image request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching image: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			offacntrl.Logger.Error().Msgf("Error closing image response body: %s", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching image", resp.StatusCode)
	}

	// Sanitize barcode to prevent path traversal — allow only alphanumeric and hyphen/underscore.
	// Barcodes are typically numeric EAN-13; this prevents "..", "/" etc.
	safeBarcode := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(barcode, "")
	if safeBarcode != barcode {
		return "", fmt.Errorf("invalid barcode characters")
	}

	cleanCachePath := filepath.Clean(offacntrl.Configuration.ImageCachePath)
	destPath := filepath.Join(cleanCachePath, safeBarcode+".jpg")
	// Ensure the final path is still within the cache directory (defense in depth)
	if !strings.HasPrefix(filepath.Clean(destPath), cleanCachePath) {
		return "", fmt.Errorf("invalid image path")
	}
	f, err := os.Create(destPath) // #nosec G304 — barcode validated via regex and path prefix check
	if err != nil {
		return "", fmt.Errorf("creating image file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			offacntrl.Logger.Error().Msgf("Error closing image file: %s", err)
		}
	}()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing image file: %w", err)
	}

	return "/product-images/" + barcode + ".jpg", nil
}
