package database

import (
	"strings"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// The name column is a bounded varchar, so a name that exceeds it is refused
// by the database instead of stored. Every write path therefore stores the
// same capped label — including the bulk and offline paths, which fill the
// name from a client draft.
func TestCreateProductCapsTheNameToTheColumn(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	long := strings.Repeat("Milch ", 100)
	product := dbModel.Product{
		Barcode:     "4006381333931",
		ProductName: long,
		HouseholdID: household.ID,
		UserID:      user.ID,
		Quantity:    1,
	}
	if err := repo.CreateProduct(user.ID, &product); err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	var stored dbModel.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if len(stored.ProductName) > util.MaxProductNameLength {
		t.Errorf("name is %d bytes, want at most %d", len(stored.ProductName), util.MaxProductNameLength)
	}
	if stored.ProductName == "" {
		t.Error("the name was dropped instead of capped")
	}
}

// A name of multi-byte characters is capped in bytes, not in characters: the
// column counts bytes, so a character bound would still overflow it.
func TestCreateProductCapsTheNameInBytes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	product := dbModel.Product{
		Barcode:     "4006381333931",
		ProductName: strings.Repeat("ä", 200),
		HouseholdID: household.ID,
		UserID:      user.ID,
		Quantity:    1,
	}
	if err := repo.CreateProduct(user.ID, &product); err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	var stored dbModel.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if len(stored.ProductName) > util.MaxProductNameLength {
		t.Errorf("name is %d bytes, want at most %d", len(stored.ProductName), util.MaxProductNameLength)
	}
	if !strings.HasSuffix(stored.ProductName, "ä") {
		t.Errorf("the name was cut in the middle of a rune: %q", stored.ProductName)
	}
}
