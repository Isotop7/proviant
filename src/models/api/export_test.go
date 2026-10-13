package api

import (
	"encoding/json"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// TestExportProductResponse_DropsGORMInternals covers what the raw
// database.Product shipped into the export file that the DTO must not.
func TestExportProductResponse_DropsGORMInternals(t *testing.T) {
	product := dbModel.Product{ProductName: "Oat Milk", Barcode: "4006381333931"}
	product.ID = 7
	product.CreatedAt = time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	product.StorageLocation = &dbModel.StorageLocation{Name: "Fridge", Icon: "🧊", SortOrder: 2}
	product.StorageLocation.ID = 3
	product.StorageLocation.HouseholdID = 42

	data, err := json.Marshal(NewExportProductResponse(&product))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"UpdatedAt", "DeletedAt", "CreatedAt", "StorageHint", "HouseholdID", "UserID"} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if decoded["ID"] != float64(7) {
		t.Errorf("ID = %v, want 7", decoded["ID"])
	}
	if decoded["productName"] != "Oat Milk" {
		t.Errorf("productName = %v, want Oat Milk", decoded["productName"])
	}

	// The purchase date is data a backup exists to carry, so it survives under a
	// lowerCamel key rather than the gorm.Model spelling.
	createdAt, ok := decoded["createdAt"].(string)
	if !ok || createdAt != "2026-03-04T09:00:00Z" {
		t.Errorf("createdAt = %v, want 2026-03-04T09:00:00Z", decoded["createdAt"])
	}
}

// TestExportProductResponse_StorageLocationOmitsHouseholdID covers the nested
// object: raw, it carried its own GORM internals plus the household id.
func TestExportProductResponse_StorageLocationOmitsHouseholdID(t *testing.T) {
	product := dbModel.Product{ProductName: "Oat Milk"}
	product.StorageLocation = &dbModel.StorageLocation{Name: "Fridge", Icon: "🧊", SortOrder: 2}
	product.StorageLocation.ID = 3
	product.StorageLocation.HouseholdID = 42

	resp := NewExportProductResponse(&product)
	if resp.StorageLocation == nil {
		t.Fatal("storageLocation = nil, want the preloaded location")
	}
	if resp.StorageLocation.Name != "Fridge" || resp.StorageLocation.SortOrder != 2 {
		t.Errorf("storageLocation = %+v, want Fridge/sortOrder 2", *resp.StorageLocation)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		StorageLocation map[string]any `json:"storageLocation"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"householdId", "Household", "CreatedAt", "UpdatedAt", "DeletedAt"} {
		if _, exists := decoded.StorageLocation[key]; exists {
			t.Errorf("storageLocation.%s present, want it omitted", key)
		}
	}
	if decoded.StorageLocation["ID"] != float64(3) {
		t.Errorf("storageLocation.ID = %v, want 3", decoded.StorageLocation["ID"])
	}
}

// TestExportProductResponse_ArchivedAt covers the archive timestamp, which the
// CSV export already exposes as archived_at: it must survive, and an active
// product must not carry the field at all.
func TestExportProductResponse_ArchivedAt(t *testing.T) {
	archivedAt := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	product := dbModel.Product{ProductName: "Old Milk"}
	product.ID = 9
	product.DeletedAt = gorm.DeletedAt{Time: archivedAt, Valid: true}

	resp := NewExportProductResponse(&product)
	if resp.ArchivedAt == nil {
		t.Fatal("archivedAt = nil, want the soft-delete timestamp")
	}
	if !resp.ArchivedAt.Equal(archivedAt) {
		t.Errorf("archivedAt = %v, want %v", resp.ArchivedAt, archivedAt)
	}

	active := NewExportProductResponse(&dbModel.Product{ProductName: "Fresh Milk"})
	data, err := json.Marshal(active)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := decoded["archivedAt"]; exists {
		t.Error("archivedAt present for an active product, want it omitted")
	}
}
