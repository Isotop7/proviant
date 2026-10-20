package database

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProductStringFields(t *testing.T) {
	product := Product{
		Barcode:     "1234567890123",
		ProductName: "Test Product",
		Categories:  "en:test",
		Countries:   "en:Germany",
		ImageURL:    "http://example.com/image.jpg",
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
}

func TestProductTimeFields(t *testing.T) {
	now := time.Now()
	product := Product{
		ExpireAt:   now,
		ScannedAt:  now,
		NotifiedAt: now,
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
}

func TestProductHouseholdID(t *testing.T) {
	product := Product{
		HouseholdID: 1,
	}
	if product.HouseholdID != 1 {
		t.Errorf("HouseholdID = %v, want 1", product.HouseholdID)
	}
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

// The patch remembers which fields a request carried, so an omitted field can
// be told apart from one sent as null. A key that only differs in case still
// fills the field — the decoder matches case-insensitively — so it has to
// count as carried too.
func TestProductDTOPatchReportsTheCarriedFields(t *testing.T) {
	t.Run("a field the body does not carry is absent", func(t *testing.T) {
		var patch ProductDTOPatch
		if err := json.Unmarshal([]byte(`{"productName":"Oat Milk"}`), &patch); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if patch.Has("productName") != true {
			t.Error("Has(productName) = false, want true")
		}
		if patch.Has("quantity") != false {
			t.Error("Has(quantity) = true, want false")
		}
		if patch.Has("packSize") != false {
			t.Error("Has(packSize) = true, want false")
		}
	})

	t.Run("a field sent as null is carried", func(t *testing.T) {
		var patch ProductDTOPatch
		if err := json.Unmarshal([]byte(`{"packSize":null}`), &patch); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !patch.Has("packSize") {
			t.Error("Has(packSize) = false, want true")
		}
	})

	t.Run("a key that only differs in case is carried", func(t *testing.T) {
		var patch ProductDTOPatch
		if err := json.Unmarshal([]byte(`{"Quantity":7,"PACKSIZE":"1 l"}`), &patch); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if patch.Quantity != 7 {
			t.Errorf("Quantity = %d, want 7 (the decoder fills the field)", patch.Quantity)
		}
		if patch.PackSize != "1 l" {
			t.Errorf("PackSize = %q, want 1 l", patch.PackSize)
		}
		if !patch.Has("quantity") {
			t.Error("Has(quantity) = false, want true")
		}
		if !patch.Has("packSize") {
			t.Error("Has(packSize) = false, want true")
		}
	})

	t.Run("a patch that was not decoded reports every field as carried", func(t *testing.T) {
		patch := ProductDTOPatch{Quantity: 3}
		if !patch.Has("quantity") {
			t.Error("Has(quantity) = false, want true")
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

		var _ = product.Model
	})
}

func TestProductSoftDelete(t *testing.T) {
	t.Run("Product has DeletedAt for soft delete", func(t *testing.T) {
		product := Product{}

		var _ = product.DeletedAt
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
