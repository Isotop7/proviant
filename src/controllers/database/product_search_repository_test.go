package database

import (
	"testing"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func TestProductRepository_SearchProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	searchRepo := NewProductSearchRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	products := []dbModel.Product{
		{ProductName: "Apple", Barcode: "111", HouseholdID: household.ID},
		{ProductName: "Banana", Barcode: "222", HouseholdID: household.ID},
		{ProductName: "Apple Juice", Barcode: "333", HouseholdID: household.ID},
	}
	for i := range products {
		db.Create(&products[i])
	}

	results, err := searchRepo.SearchProducts(ProductName, "Apple", "", "", user.ID, 0, 0)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("SearchProducts() returned %d results, want 2", len(results))
	}
}
func TestProductRepository_SearchProducts_ByBarcode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	searchRepo := NewProductSearchRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "Test",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	results, err := searchRepo.SearchProducts(Barcode, "1234567890123", "", "", user.ID, 0, 0)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("SearchProducts() returned %d results, want 1", len(results))
	}
}

func TestProductRepository_SearchErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	searchRepo := NewProductSearchRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)
	otherLocation := testutil.CreateTestStorageLocation(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ProductName = "Searchable"
	product.StorageLocationID = &location.ID
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("invalid search parameter", func(t *testing.T) {
		if _, err := searchRepo.SearchProducts(InvalidParameter, "x", "", "", user.ID, 0, 0); err != errors.ErrDatabaseInvalidSearchParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSearchParameter)
		}
	})

	t.Run("invalid sort parameter", func(t *testing.T) {
		if _, err := searchRepo.SearchProducts(ProductName, "x", "not-a-column", "", user.ID, 0, 0); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})

	t.Run("invalid sort direction", func(t *testing.T) {
		if _, err := searchRepo.SearchProducts(ProductName, "x", "product_name", "sideways", user.ID, 0, 0); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})

	t.Run("projections narrowed by storage location", func(t *testing.T) {
		products, err := searchRepo.SearchProductProjections(ProductName, "Searchable", "", "", user.ID, location.ID)
		if err != nil {
			t.Fatalf("SearchProductProjections() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != product.ID {
			t.Errorf("SearchProductProjections() = %+v, want product %d", products, product.ID)
		}

		products, err = searchRepo.SearchProductProjections(ProductName, "Searchable", "", "", user.ID, otherLocation.ID)
		if err != nil {
			t.Fatalf("SearchProductProjections() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("SearchProductProjections() with other location returned %d, want 0", len(products))
		}
	})

	t.Run("projections reject invalid sort", func(t *testing.T) {
		if _, err := searchRepo.SearchProductProjections(ProductName, "x", "barcode; DROP TABLE products", "", user.ID, 0); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProductProjections() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})
}
