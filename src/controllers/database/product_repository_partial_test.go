package database

import (
	"errors"
	"testing"
	"time"

	apperrors "github.com/Isotop7/proviant/errors"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func setupPartialRemoval(t *testing.T, quantity int) (*gorm.DB, *ProductRepository, *dbModel.Product) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 1)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)
	product := &dbModel.Product{
		ProductName:       "Yogurt",
		Barcode:           "1234567890123",
		Categories:        "Dairy",
		ExpireAt:          time.Date(2027, time.January, 2, 0, 0, 0, 0, time.UTC),
		HouseholdID:       household.ID,
		UserID:            user.ID,
		Quantity:          quantity,
		StorageLocationID: &location.ID,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return db, repo, product
}

// TestProductRepository_PartialRemovalPreservesStock verifies that one removed pack leaves all other product data untouched.
func TestProductRepository_PartialRemovalPreservesStock(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*ProductRepository, *dbModel.Product, int, *string, string) (int, bool, bool, error)
	}{
		{name: "consume", remove: func(r *ProductRepository, p *dbModel.Product, q int, m *string, _ string) (int, bool, bool, error) {
			return r.ConsumeProductPartial(p, q, m)
		}},
		{name: "waste", remove: (*ProductRepository).WasteProductPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, repo, product := setupPartialRemoval(t, 3)

			removed, fullyRemoved, applied, err := tt.remove(repo, product, 1, nil, "")
			if err != nil {
				t.Fatalf("partial removal error = %v", err)
			}
			if removed != 1 || fullyRemoved || !applied {
				t.Fatalf("partial removal = (%d, %v, %v), want (1, false, true)", removed, fullyRemoved, applied)
			}

			var updated dbModel.Product
			if err := db.First(&updated, product.ID).Error; err != nil {
				t.Fatalf("reload active product: %v", err)
			}
			if updated.Quantity != 2 {
				t.Errorf("Quantity = %d, want 2", updated.Quantity)
			}
			if updated.DeletedAt.Valid || updated.RemovalReason != "" {
				t.Errorf("partial removal archived or claimed active row: DeletedAt=%v RemovalReason=%q", updated.DeletedAt.Valid, updated.RemovalReason)
			}
			if !updated.ExpireAt.Equal(product.ExpireAt) || updated.Categories != product.Categories {
				t.Errorf("expiry or categories changed: ExpireAt=%v Categories=%q", updated.ExpireAt, updated.Categories)
			}
			if updated.StorageLocationID == nil || *updated.StorageLocationID != *product.StorageLocationID {
				t.Errorf("StorageLocationID = %v, want %d", updated.StorageLocationID, *product.StorageLocationID)
			}
		})
	}
}

// TestProductRepository_RemovalOverCountArchivesActualStock verifies that an excess request archives only the stock that exists.
func TestProductRepository_RemovalOverCountArchivesActualStock(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		remove func(*ProductRepository, *dbModel.Product, int, *string, string) (int, bool, bool, error)
	}{
		{name: "consume", reason: dbModel.RemovalReasonConsumed, remove: func(r *ProductRepository, p *dbModel.Product, q int, m *string, _ string) (int, bool, bool, error) {
			return r.ConsumeProductPartial(p, q, m)
		}},
		{name: "waste", reason: dbModel.RemovalReasonWasted, remove: (*ProductRepository).WasteProductPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, repo, product := setupPartialRemoval(t, 3)

			removed, fullyRemoved, applied, err := tt.remove(repo, product, 5, nil, "")
			if err != nil {
				t.Fatalf("over-count removal error = %v", err)
			}
			if removed != 3 || !fullyRemoved || !applied {
				t.Fatalf("over-count removal = (%d, %v, %v), want (3, true, true)", removed, fullyRemoved, applied)
			}

			var archived dbModel.Product
			if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
				t.Fatalf("reload archived product: %v", err)
			}
			if !archived.DeletedAt.Valid || archived.RemovalReason != tt.reason {
				t.Errorf("archive state = (DeletedAt %v, reason %q), want (true, %q)", archived.DeletedAt.Valid, archived.RemovalReason, tt.reason)
			}
		})
	}
}

// TestProductRepository_RemovalMutationReplay verifies that a mutation id applies stock removal at most once.
func TestProductRepository_RemovalMutationReplay(t *testing.T) {
	t.Run("partial consume", func(t *testing.T) {
		db, repo, product := setupPartialRemoval(t, 3)
		mutationID := "offline-consume-1"

		_, _, applied, err := repo.ConsumeProductPartial(product, 1, &mutationID)
		if err != nil || !applied {
			t.Fatalf("first removal = (applied %v, error %v), want (true, nil)", applied, err)
		}
		removed, fullyRemoved, applied, err := repo.ConsumeProductPartial(product, 1, &mutationID)
		if err != nil || applied || removed != 0 || fullyRemoved {
			t.Fatalf("replayed removal = (%d, %v, %v, %v), want (0, false, false, nil)", removed, fullyRemoved, applied, err)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 2 {
			t.Errorf("Quantity after replay = %d, want 2", updated.Quantity)
		}
		var claims int64
		if err := db.Model(&dbModel.ProductMutationClaim{}).Where("product_id = ? AND mutation_id = ?", product.ID, mutationID).Count(&claims).Error; err != nil {
			t.Fatalf("count mutation claims: %v", err)
		}
		if claims != 1 {
			t.Errorf("mutation claims = %d, want 1", claims)
		}
	})

	t.Run("full waste", func(t *testing.T) {
		db, repo, product := setupPartialRemoval(t, 2)
		mutationID := "offline-waste-1"

		if _, _, applied, err := repo.WasteProductPartial(product, 0, &mutationID, ""); err != nil || !applied {
			t.Fatalf("first waste = (applied %v, error %v), want (true, nil)", applied, err)
		}
		if _, _, applied, err := repo.WasteProductPartial(product, 0, &mutationID, ""); err != nil || applied {
			t.Fatalf("replayed waste = (applied %v, error %v), want (false, nil)", applied, err)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("reload archived product: %v", err)
		}
		if archived.RemovalReason != dbModel.RemovalReasonWasted {
			t.Errorf("RemovalReason = %q, want %q", archived.RemovalReason, dbModel.RemovalReasonWasted)
		}
	})
}

// TestProductRepository_WastePartialConcurrentModification verifies that waste uses the same claimed-row guard as consume.
func TestProductRepository_WastePartialConcurrentModification(t *testing.T) {
	db, repo, product := setupPartialRemoval(t, 3)
	if err := db.Model(product).Update("removal_reason", dbModel.RemovalReasonConsumed).Error; err != nil {
		t.Fatalf("claim product: %v", err)
	}

	_, _, _, err := repo.WasteProductPartial(product, 1, nil, "")
	if !errors.Is(err, apperrors.ErrProductConcurrentModification) {
		t.Fatalf("WasteProductPartial() error = %v, want %v", err, apperrors.ErrProductConcurrentModification)
	}
	var unchanged dbModel.Product
	if err := db.Unscoped().First(&unchanged, product.ID).Error; err != nil {
		t.Fatalf("reload product: %v", err)
	}
	if unchanged.Quantity != 3 || unchanged.RemovalReason != dbModel.RemovalReasonConsumed || unchanged.DeletedAt.Valid {
		t.Errorf("failed removal changed row: Quantity=%d RemovalReason=%q DeletedAt=%v", unchanged.Quantity, unchanged.RemovalReason, unchanged.DeletedAt.Valid)
	}
}

// TestProductRepository_WasteKeepsItsCause verifies that the archived row
// carries the reason it was wasted beside the removal reason, and that a
// consume leaves the reason empty.
func TestProductRepository_WasteKeepsItsCause(t *testing.T) {
	t.Run("waste archives the row with the cause", func(t *testing.T) {
		db, repo, product := setupPartialRemoval(t, 2)

		if _, _, _, err := repo.WasteProductPartial(product, 0, nil, dbModel.WasteCauseSpoiled); err != nil {
			t.Fatalf("waste error = %v", err)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("reload archived product: %v", err)
		}
		if archived.RemovalReason != dbModel.RemovalReasonWasted {
			t.Errorf("RemovalReason = %q, want %q", archived.RemovalReason, dbModel.RemovalReasonWasted)
		}
		if archived.WasteCause != dbModel.WasteCauseSpoiled {
			t.Errorf("WasteCause = %q, want %q", archived.WasteCause, dbModel.WasteCauseSpoiled)
		}
	})

	t.Run("consume leaves the cause empty", func(t *testing.T) {
		db, repo, product := setupPartialRemoval(t, 2)

		if _, _, _, err := repo.ConsumeProductPartial(product, 0, nil); err != nil {
			t.Fatalf("consume error = %v", err)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("reload archived product: %v", err)
		}
		if archived.WasteCause != "" {
			t.Errorf("WasteCause = %q, want empty", archived.WasteCause)
		}
	})

	t.Run("a restored product loses the cause", func(t *testing.T) {
		db, repo, product := setupPartialRemoval(t, 2)

		if err := repo.WasteProduct(product.ID, product.UserID, dbModel.WasteCauseForgotten); err != nil {
			t.Fatalf("waste error = %v", err)
		}
		if err := repo.RestoreProduct(product.ID, product.UserID); err != nil {
			t.Fatalf("restore error = %v", err)
		}

		var restored dbModel.Product
		if err := db.First(&restored, product.ID).Error; err != nil {
			t.Fatalf("reload restored product: %v", err)
		}
		if restored.WasteCause != "" {
			t.Errorf("WasteCause = %q, want empty", restored.WasteCause)
		}
	})
}
