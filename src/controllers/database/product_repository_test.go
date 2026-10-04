package database

import (
	"testing"

	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestProductRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetProductByID not found", func(t *testing.T) {
		repo := NewProductRepository(db)

		_, err := repo.GetProductByID(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetUserProductsBulk returns empty for no products", func(t *testing.T) {
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		products, err := repo.GetUserProductsBulk(testUser.ID, 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("len(products) = %v, want 0", len(products))
		}
	})

	t.Run("GetMatchableProductsByHousehold populates only the matched columns", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(1)
		product := testutil.CreateTestProduct(db, householdID)
		product.ProductName = "Chicken"
		product.Categories = "meat"
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		row := rows[0]
		if row.ID != product.ID {
			t.Errorf("row.ID = %d, want %d", row.ID, product.ID)
		}
		if row.ProductName != "Chicken" {
			t.Errorf("row.ProductName = %q, want %q", row.ProductName, "Chicken")
		}
		if row.Categories != "meat" {
			t.Errorf("row.Categories = %q, want %q", row.Categories, "meat")
		}
		// The projection deliberately leaves everything else zero so a caller
		// reading an unselected column gets an obviously wrong value instead of
		// a plausible one.
		if row.Amount != 0 || row.Barcode != "" || !row.ExpireAt.IsZero() {
			t.Errorf("unselected columns are not zero: amount=%d barcode=%q expireAt=%v",
				row.Amount, row.Barcode, row.ExpireAt)
		}
	})

	t.Run("GetMatchableProductsByHousehold excludes private and deleted products", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(2)
		visible := testutil.CreateTestProduct(db, householdID)
		visible.ProductName = "Visible"
		if err := db.Save(visible).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		private := testutil.CreateTestProduct(db, householdID)
		private.ProductName = "Private"
		private.IsPrivate = true
		if err := db.Save(private).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		removed := testutil.CreateTestProduct(db, householdID)
		removed.ProductName = "Removed"
		if err := db.Delete(removed).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].ProductName != "Visible" {
			t.Errorf("got %d rows (%+v), want only the visible product", len(rows), rows)
		}
	})
}
