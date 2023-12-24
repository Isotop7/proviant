package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"expiro/backend/models/configuration"
	"expiro/backend/models/database"
	"expiro/backend/models/external"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type OpenFoodFactsAPIController struct {
	Configuration configuration.OpenFoodFactsConfiguration
}

func (cntrl OpenFoodFactsAPIController) GetDataset(db *gorm.DB, barcode string) (database.Product, error) {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Channel to receive the response or timeout signal
	ch := make(chan bool)
	// Dataset to store query response
	var dataset external.OpenFoodFactsAPIDataset
	filteredDataset := "product_name,categories,countries,generic_name,image_url"

	// Query API for dataset
	go func() {
		resp, err := http.Get(fmt.Sprintf("%s/%s?fields=%s", cntrl.Configuration.URL, barcode, filteredDataset))
		// If upstream error is received, we also throw it
		if err != nil {
			fmt.Println("Error:", err)
			ch <- false
			return
		}
		defer resp.Body.Close()

		// Parse the response and populate the dataset struct
		if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
			fmt.Println("Error decoding response:", err)
		}

		ch <- true
	}()

	// Wait for either a response or a timeout
	select {
	case <-ctx.Done():
		// Timeout occured
		return database.Product{}, errors.New("timeout occured")
	case success := <-ch:
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
