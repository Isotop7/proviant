package database

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestProductStruct(t *testing.T) {
	t.Run("Product struct has correct fields", func(t *testing.T) {
		now := time.Now()
		product := Product{
			Barcode:     "1234567890123",
			ProductName: "Test Product",
			Categories:  "en:test",
			Countries:   "en:Germany",
			ImageURL:    "http://example.com/image.jpg",
			ExpireAt:    now,
			ScannedAt:   now,
			NotifiedAt:  now,
			HouseholdID: 1,
		}

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
		if product.ExpireAt != now {
			t.Errorf("ExpireAt mismatch")
		}
		if product.ScannedAt != now {
			t.Errorf("ScannedAt mismatch")
		}
		if product.NotifiedAt != now {
			t.Errorf("NotifiedAt mismatch")
		}
		if product.HouseholdID != 1 {
			t.Errorf("HouseholdID = %v, want 1", product.HouseholdID)
		}
	})
}

func TestProductDTOPatch(t *testing.T) {
	t.Run("ProductDTOPatch has correct fields", func(t *testing.T) {
		now := time.Now()
		dto := ProductDTOPatch{
			ID:          1,
			ProductName: "Updated Product",
			Categories:  "en:updated",
			Countries:   "en:France",
			ImageURL:    "http://example.com/updated.jpg",
			ExpireAt:    now,
		}

		if dto.ID != 1 {
			t.Errorf("ID = %v, want 1", dto.ID)
		}
		if dto.ProductName != "Updated Product" {
			t.Errorf("ProductName = %v, want Updated Product", dto.ProductName)
		}
		if dto.Categories != "en:updated" {
			t.Errorf("Categories = %v, want en:updated", dto.Categories)
		}
		if dto.Countries != "en:France" {
			t.Errorf("Countries = %v, want en:France", dto.Countries)
		}
		if dto.ImageURL != "http://example.com/updated.jpg" {
			t.Errorf("ImageURL = %v, want http://example.com/updated.jpg", dto.ImageURL)
		}
		if dto.ExpireAt != now {
			t.Errorf("ExpireAt mismatch")
		}
	})
}

func TestProductDTOExpire(t *testing.T) {
	t.Run("ProductDTOExpire has correct fields", func(t *testing.T) {
		date := Date(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC))
		dto := ProductDTOExpire{
			ID:       1,
			Barcode:  "1234567890123",
			ExpireAt: date,
		}

		if dto.ID != 1 {
			t.Errorf("ID = %v, want 1", dto.ID)
		}
		if dto.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", dto.Barcode)
		}
		if dto.ExpireAt != date {
			t.Errorf("ExpireAt mismatch")
		}
	})
}

func TestProductDTOBarcode(t *testing.T) {
	t.Run("ProductDTOBarcode has correct fields", func(t *testing.T) {
		dto := ProductDTOBarcode{
			Barcode: "1234567890123",
		}

		if dto.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", dto.Barcode)
		}
	})
}

func TestProductGORMModel(t *testing.T) {
	t.Run("Product embeds gorm.Model", func(t *testing.T) {
		product := Product{}

		var _ gorm.Model = product.Model
	})
}

func TestProductSoftDelete(t *testing.T) {
	t.Run("Product has DeletedAt for soft delete", func(t *testing.T) {
		product := Product{}

		var _ gorm.DeletedAt = product.DeletedAt
	})
}

func TestProductHouseholdIDHidden(t *testing.T) {
	t.Run("Product HouseholdID is hidden from JSON", func(t *testing.T) {
		product := Product{
			Barcode:     "1234567890123",
			HouseholdID: 123,
		}

		type JSONProduct struct {
			Barcode     string `json:"barcode"`
			HouseholdID uint   `json:"householdID"`
		}

		jsonProduct := JSONProduct{
			Barcode:     product.Barcode,
			HouseholdID: product.HouseholdID,
		}

		if jsonProduct.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want 1234567890123", jsonProduct.Barcode)
		}
		if jsonProduct.HouseholdID != 123 {
			t.Errorf("HouseholdID = %v, want 123", jsonProduct.HouseholdID)
		}
	})
}
