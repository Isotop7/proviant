package api

import (
	"testing"
)

func TestBulkProductsAPIModelStruct(t *testing.T) {
	t.Run("can create BulkProductsAPIModel", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []string{"123", "456", "789"},
		}

		if len(model.ProductIDs) != 3 {
			t.Errorf("ProductIDs length = %v, want 3", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != "123" {
			t.Errorf("ProductIDs[0] = %v, want 123", model.ProductIDs[0])
		}
		if model.ProductIDs[1] != "456" {
			t.Errorf("ProductIDs[1] = %v, want 456", model.ProductIDs[1])
		}
		if model.ProductIDs[2] != "789" {
			t.Errorf("ProductIDs[2] = %v, want 789", model.ProductIDs[2])
		}
	})
}

func TestBulkProductsAPIModelWithEmptySlice(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with empty slice", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []string{},
		}

		if len(model.ProductIDs) != 0 {
			t.Errorf("ProductIDs length = %v, want 0", len(model.ProductIDs))
		}
	})
}

func TestBulkProductsAPIModelWithNilSlice(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with nil slice", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: nil,
		}

		if model.ProductIDs != nil {
			t.Errorf("ProductIDs = %v, want nil", model.ProductIDs)
		}
	})
}

func TestBulkProductsAPIModelWithSingleID(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with single ID", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []string{"1234567890123"},
		}

		if len(model.ProductIDs) != 1 {
			t.Errorf("ProductIDs length = %v, want 1", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != "1234567890123" {
			t.Errorf("ProductIDs[0] = %v, want 1234567890123", model.ProductIDs[0])
		}
	})
}

func TestBulkProductsAPIModelWithSpecialCharacters(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with special characters", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []string{"123-ABC", "456_DEF", "789!@#"},
		}

		if len(model.ProductIDs) != 3 {
			t.Errorf("ProductIDs length = %v, want 3", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != "123-ABC" {
			t.Errorf("ProductIDs[0] = %v, want 123-ABC", model.ProductIDs[0])
		}
		if model.ProductIDs[1] != "456_DEF" {
			t.Errorf("ProductIDs[1] = %v, want 456_DEF", model.ProductIDs[1])
		}
		if model.ProductIDs[2] != "789!@#" {
			t.Errorf("ProductIDs[2] = %v, want 789!@#", model.ProductIDs[2])
		}
	})
}
