package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/Isotop7/proviant/models/configuration"
	"gitlab.com/Isotop7/proviant/models/database"
	"gitlab.com/Isotop7/proviant/models/external"
)

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
			return database.Product{
				Barcode:     dataset.Barcode,
				ProductName: dataset.Product.ProductName,
				Categories:  dataset.Product.Categories,
				Countries:   dataset.Product.Countries,
				ImageURL:    dataset.Product.ImageURL,
			}, nil
		}
	}

	return database.Product{}, nil
}
