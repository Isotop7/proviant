package database

import (
	"testing"

	"github.com/Isotop7/proviant/testutil"
)

// The pack size column is added without a default, so every product that
// predates it carries NULL. The backfill has to see those rows, or it never
// fills the size of anything that existed before the column was added.
func TestListProductsWithoutPackSizeFindsNullAndEmptyRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	withoutSize := testutil.CreateTestProduct(db, household.ID, user.ID)
	withoutSize.Barcode = "4006381333931"
	withoutSize.PackSize = ""
	if err := db.Save(withoutSize).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	nullSize := testutil.CreateTestProduct(db, household.ID, user.ID)
	nullSize.Barcode = "4006381333932"
	if err := db.Save(nullSize).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	// A product that predates the column carries NULL, not an empty string.
	if err := db.Exec("UPDATE products SET pack_size = NULL WHERE id = ?", nullSize.ID).Error; err != nil {
		t.Fatalf("null out the pack size: %v", err)
	}
	withSize := testutil.CreateTestProduct(db, household.ID, user.ID)
	withSize.Barcode = "4006381333933"
	withSize.PackSize = "1 l"
	if err := db.Save(withSize).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}

	repo := NewProductRepository(db)
	products, err := repo.ListProductsWithoutPackSize(0, 50)
	if err != nil {
		t.Fatalf("ListProductsWithoutPackSize() error = %v", err)
	}
	found := make(map[uint]bool, len(products))
	for _, product := range products {
		found[product.ID] = true
	}
	if !found[withoutSize.ID] {
		t.Errorf("the product without a pack size is missing from the backfill")
	}
	if !found[nullSize.ID] {
		t.Errorf("the product with a NULL pack size is missing from the backfill")
	}
	if found[withSize.ID] {
		t.Errorf("the product with a pack size is in the backfill")
	}
}

// The backfill walks the table with a cursor, so a barcode the data source
// cannot resolve does not hold the queue in place.
func TestListProductsWithoutPackSizeRespectsTheCursor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	first := testutil.CreateTestProduct(db, household.ID, user.ID)
	first.Barcode = "4006381333931"
	second := testutil.CreateTestProduct(db, household.ID, user.ID)
	second.Barcode = "4006381333932"
	if err := db.Save(first).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := db.Save(second).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}

	repo := NewProductRepository(db)
	products, err := repo.ListProductsWithoutPackSize(first.ID, 50)
	if err != nil {
		t.Fatalf("ListProductsWithoutPackSize() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != second.ID {
		t.Errorf("products after the cursor = %+v, want only the second one", products)
	}
}
