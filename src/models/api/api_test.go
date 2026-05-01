package api

import (
	"testing"
)

func TestBulkProductsAPIModelStruct(t *testing.T) {
	t.Run("can create BulkProductsAPIModel", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []uint{123, 456, 789},
		}

		if len(model.ProductIDs) != 3 {
			t.Errorf("ProductIDs length = %v, want 3", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != 123 {
			t.Errorf("ProductIDs[0] = %v, want 123", model.ProductIDs[0])
		}
		if model.ProductIDs[1] != 456 {
			t.Errorf("ProductIDs[1] = %v, want 456", model.ProductIDs[1])
		}
		if model.ProductIDs[2] != 789 {
			t.Errorf("ProductIDs[2] = %v, want 789", model.ProductIDs[2])
		}
	})
}

func TestBulkProductsAPIModelWithEmptySlice(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with empty slice", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []uint{},
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
			ProductIDs: []uint{1234567890},
		}

		if len(model.ProductIDs) != 1 {
			t.Errorf("ProductIDs length = %v, want 1", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != 1234567890 {
			t.Errorf("ProductIDs[0] = %v, want 1234567890", model.ProductIDs[0])
		}
	})
}

func TestBulkProductsAPIModelWithLargeIDs(t *testing.T) {
	t.Run("can create BulkProductsAPIModel with large IDs", func(t *testing.T) {
		model := BulkProductsAPIModel{
			ProductIDs: []uint{1, 999999},
		}

		if len(model.ProductIDs) != 2 {
			t.Errorf("ProductIDs length = %v, want 2", len(model.ProductIDs))
		}

		if model.ProductIDs[0] != 1 {
			t.Errorf("ProductIDs[0] = %v, want 1", model.ProductIDs[0])
		}
		if model.ProductIDs[1] != 999999 {
			t.Errorf("ProductIDs[1] = %v, want 999999", model.ProductIDs[1])
		}
	})
}
