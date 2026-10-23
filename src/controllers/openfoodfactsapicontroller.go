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

	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/models/external"
	"github.com/Isotop7/proviant/util"
	"github.com/rs/zerolog"
)

// DatasetGetter defines the contract for getting OpenFoodFacts dataset by barcode
type DatasetGetter interface {
	GetDataset(barcode string) (database.Product, error)
}

// OfflineProductLookup resolves a barcode from locally imported OpenFoodFacts
// bulk data without a live API call. A false result means the barcode is not in
// the local store, so the caller proceeds to the live API.
type OfflineProductLookup interface {
	LookupProduct(barcode string) (database.Product, bool)
}

// OpenFoodFactsAPIController is the object struct for interacting with the API of OpenFoodFacts
// It uses the given configuration for accessing the API
type OpenFoodFactsAPIController struct {
	Logger        *zerolog.Logger
	Configuration configuration.OpenFoodFactsConfiguration
	OfflineLookup OfflineProductLookup
}

// LookupOffline resolves a barcode from the imported bulk dump when it is
// configured, reporting whether a local entry was found.
func (offacntrl *OpenFoodFactsAPIController) LookupOffline(barcode string) (database.Product, bool) {
	if offacntrl.OfflineLookup == nil {
		return database.Product{}, false
	}
	return offacntrl.OfflineLookup.LookupProduct(barcode)
}

// GetDataset gets data from OpenFoodFacts by its API, consulting the imported
// bulk dump first so a locally known barcode costs no API call. The search
// parameter is the barcode of the product.
func (offacntrl *OpenFoodFactsAPIController) GetDataset(barcode string) (database.Product, error) {
	if product, found := offacntrl.LookupOffline(barcode); found {
		return product, nil
	}
	return offacntrl.GetLiveDataset(barcode)
}

// GetLiveDataset queries the live OpenFoodFacts API. The search parameter is
// the barcode of the product.
func (offacntrl *OpenFoodFactsAPIController) GetLiveDataset(barcode string) (database.Product, error) {
	// Create a context with a timeout
	queryTimeout := time.Second * time.Duration(offacntrl.Configuration.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	// Buffered, so a response arriving after the timeout below has already
	// returned still completes its send instead of parking the goroutine on a
	// channel nobody reads. Each offline sync batch calls this once per unnamed
	// draft, so an unbuffered channel leaked goroutines in bulk.
	queryChannel := make(chan bool, 1)
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

		// Check for HTTP errors before parsing
		if resp.StatusCode >= 400 {
			offacntrl.Logger.Warn().Msgf("OpenFoodFacts API error: status %d, body: %s", resp.StatusCode, string(body))
			queryChannel <- false
			return
		}

		// Parse the response and populate the dataset struct
		if err := json.Unmarshal(body, &dataset); err != nil {
			offacntrl.Logger.Error().Msgf("Error decoding response: %s", err)
			queryChannel <- false
			return
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
				v := float64(dataset.Product.EcoscoreData.Agribalyse.CO2Total)
				co2 = &v
			}
			return database.Product{
				Barcode:     dataset.Barcode,
				ProductName: dataset.Product.ProductName,
				Categories:  dataset.Product.Categories,
				Countries:   dataset.Product.Countries,
				ImageURL:    dataset.Product.ImageURL,
				PackSize:    util.PackSizeLabel(dataset.Product.Quantity),
				CO2KgPerKg:  co2,
				StorageHint: dataset.Product.ConservationConditions,
			}, nil
		}
	}

	return database.Product{}, nil
}

// DownloadImage fetches an image from imageURL and saves it to {cachePath}/{barcode}.jpg.
// Returns the local serve path /product-images/{barcode}.jpg on success.
func (offacntrl *OpenFoodFactsAPIController) DownloadImage(imageURL, barcode string) (string, error) {
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
