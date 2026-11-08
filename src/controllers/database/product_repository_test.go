package database

import (
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

func TestProductRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetProductByID not found", func(t *testing.T) {
		productRepo := NewProductRepository(db)

		_, err := productRepo.GetProductByID(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetUserProductsBulk returns empty for no products", func(t *testing.T) {
		listRepo := NewProductListRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		products, err := listRepo.GetUserProductsBulk(testUser.ID, 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("len(products) = %v, want 0", len(products))
		}
	})

	t.Run("GetMatchableProductsByHousehold populates only the matched columns", func(t *testing.T) {
		householdRepo := NewProductHouseholdRepository(db)
		householdID := uint(1)
		product := testutil.CreateTestProduct(db, householdID)
		product.ProductName = "Chicken"
		product.Categories = "meat"
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := householdRepo.GetMatchableProductsByHousehold(householdID)
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
		if row.Quantity != 0 || row.Barcode != "" || !row.ExpireAt.IsZero() {
			t.Errorf("unselected columns are not zero: amount=%d barcode=%q expireAt=%v",
				row.Quantity, row.Barcode, row.ExpireAt)
		}
	})

	t.Run("GetMatchableProductsByHousehold excludes private and deleted products", func(t *testing.T) {
		householdRepo := NewProductHouseholdRepository(db)
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

		rows, err := householdRepo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].ProductName != "Visible" {
			t.Errorf("got %d rows (%+v), want only the visible product", len(rows), rows)
		}
	})
}
func TestCreateProductsBulk(t *testing.T) {
	t.Run("creates each distinct new location once and stamps the products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		productRepo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Reusing an existing location is the resolver's job, above this layer;
		// every row here names something the household does not have yet.
		existing := testutil.CreateTestStorageLocation(db, household.ID)

		rows := []ImportedProduct{
			{Product: database.Product{ProductName: "Milk", Barcode: "4006381333931"}, NewLocationName: "Cellar", NewLocationIcon: "📦"},
			{Product: database.Product{ProductName: "Cheese", Barcode: "4006381333948"}, NewLocationName: "cellar", NewLocationIcon: "📦"},
			{Product: database.Product{ProductName: "Butter", Barcode: "4006381333955"}, NewLocationName: "Pantry", NewLocationIcon: "📦"},
		}

		created, err := productRepo.CreateProductsBulk(user.ID, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(created) != 2 || created[0] != "Cellar" || created[1] != "Pantry" {
			t.Errorf("created = %+v, want [Cellar Pantry] in creation order", created)
		}

		var products []database.Product
		if err := db.Order("id").Find(&products).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 3 {
			t.Fatalf("got %d products, want 3", len(products))
		}
		for i := range products {
			if products[i].HouseholdID != household.ID || products[i].UserID != user.ID {
				t.Errorf("product %q stamped household %d user %d, want %d/%d",
					products[i].ProductName, products[i].HouseholdID, products[i].UserID, household.ID, user.ID)
			}
			if products[i].StorageLocationID == nil {
				t.Errorf("product %q has no storage location", products[i].ProductName)
			}
		}
		if *products[0].StorageLocationID != *products[1].StorageLocationID {
			t.Error("differently spelled names did not collapse onto one location")
		}
		if *products[0].StorageLocationID == *products[2].StorageLocationID {
			t.Error("two distinct names collapsed onto one location")
		}

		var locationCount int64
		if err := db.Model(&database.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The pre-existing Fridge plus exactly the two new names.
		if locationCount != 3 {
			t.Errorf("got %d locations, want 3 (Fridge, Cellar, Pantry)", locationCount)
		}

		// New locations sort after the ones the household already had, which
		// seeded SortOrder 0.
		var newLocation database.StorageLocation
		if err := db.Where(util.QueryHouseholdId, household.ID).Where("name = ?", "Cellar").First(&newLocation).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if newLocation.SortOrder <= existing.SortOrder {
			t.Errorf("new location SortOrder = %d, want after the existing %d", newLocation.SortOrder, existing.SortOrder)
		}
	})

	t.Run("rejects a CSV naming too many new locations", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		productRepo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		rows := make([]ImportedProduct, 0, util.CsvImportMaxNewLocations+1)
		for i := range util.CsvImportMaxNewLocations + 1 {
			rows = append(rows, ImportedProduct{
				Product:         database.Product{ProductName: "Item", Barcode: fmt.Sprintf("40063813339%02d", i)},
				NewLocationName: fmt.Sprintf("Location %d", i),
			})
		}

		created, err := productRepo.CreateProductsBulk(user.ID, rows)
		if !stderrors.Is(err, errors.ErrImportTooManyLocations) {
			t.Fatalf("err = %v, want ErrImportTooManyLocations", err)
		}
		if created != nil {
			t.Errorf("created = %+v, want none", created)
		}

		// The whole import rolls back: no product and no location may survive.
		var products, locations int64
		if err := db.Model(&database.Product{}).Count(&products).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.Model(&database.StorageLocation{}).Count(&locations).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if products != 0 || locations != 0 {
			t.Errorf("rolled back import left %d products and %d locations, want none", products, locations)
		}
	})
}
func TestProductRepository_GetProductByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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
		ProductName: "Test Product",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	foundProduct, err := productRepo.GetProductByID(product.ID, user.ID)
	if err != nil {
		t.Fatalf("GetProductByID() error = %v", err)
	}

	if foundProduct.ProductName != "Test Product" {
		t.Errorf("GetProductByID() ProductName = %s, want Test Product", foundProduct.ProductName)
	}
}
func TestProductRepository_GetProductByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	_, err := productRepo.GetProductByID(999, user.ID)
	if err == nil {
		t.Error("GetProductByID() expected error for non-existent product")
	}
}
func TestProductRepository_CreateProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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
		ProductName: "New Product",
		Barcode:     "1234567890123",
	}

	err := productRepo.CreateProduct(user.ID, &product)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	if product.HouseholdID != household.ID {
		t.Errorf("CreateProduct() HouseholdID = %d, want %d", product.HouseholdID, household.ID)
	}
}

// TestProductRepository_CreateProductValidatesOpenedLifecycle pins the create-
// path half of the opened-shelf-life invariant: POST binds the raw model, so
// the repository is the only gate. A row with DaysAfterOpening > 365 would be
// pruned out of effectiveExpiryCandidateScope's window and silently vanish
// from expiring-soon lists and notifications.
func TestProductRepository_CreateProductValidatesOpenedLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)
	user := authentication.User{
		Username:    "lifecycleuser",
		Password:    "password",
		MailAddress: "lifecycle@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	t.Run("daysAfterOpening above the bound is rejected", func(t *testing.T) {
		tooManyDays := 366
		product := dbModel.Product{
			ProductName:      "Bad Shelf Life",
			Barcode:          "1111111111111",
			DaysAfterOpening: &tooManyDays,
		}
		if err := productRepo.CreateProduct(user.ID, &product); err != errors.ErrInvalidOpenedLifecycle {
			t.Errorf("CreateProduct() error = %v, want %v", err, errors.ErrInvalidOpenedLifecycle)
		}
	})

	t.Run("future openedAt is rejected", func(t *testing.T) {
		future := time.Now().Add(72 * time.Hour)
		product := dbModel.Product{
			ProductName: "Future Open",
			Barcode:     "2222222222222",
			OpenedAt:    &future,
		}
		if err := productRepo.CreateProduct(user.ID, &product); err != errors.ErrInvalidOpenedLifecycle {
			t.Errorf("CreateProduct() error = %v, want %v", err, errors.ErrInvalidOpenedLifecycle)
		}
	})

	t.Run("valid opened lifecycle is accepted", func(t *testing.T) {
		opened := time.Now().Add(-24 * time.Hour)
		validDays := 14
		product := dbModel.Product{
			ProductName:      "Good Shelf Life",
			Barcode:          "3333333333333",
			OpenedAt:         &opened,
			DaysAfterOpening: &validDays,
		}
		if err := productRepo.CreateProduct(user.ID, &product); err != nil {
			t.Fatalf("CreateProduct() error = %v, want nil", err)
		}
	})
}
func TestProductRepository_UpdateProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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
		ProductName: "Original Name",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	updateData := dbModel.ProductDTOPatch{
		ProductName: "Updated Name",
	}

	err := productRepo.UpdateProduct(product.ID, user.ID, &updateData)
	if err != nil {
		t.Fatalf("UpdateProduct() error = %v", err)
	}

	var updated dbModel.Product
	db.First(&updated, product.ID)
	if updated.ProductName != "Updated Name" {
		t.Errorf("UpdateProduct() ProductName = %s, want Updated Name", updated.ProductName)
	}
}
func TestProductRepository_DeleteProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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
		ProductName: "To Delete",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	err := productRepo.DeleteProduct(product.ID, user.ID, true)
	if err != nil {
		t.Fatalf("DeleteProduct() error = %v", err)
	}

	var deleted dbModel.Product
	err = db.Unscoped().First(&deleted, product.ID).Error
	if err != nil {
		t.Fatalf("Product not found after delete: %v", err)
	}
}
func TestProductRepository_RestoreProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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
		ProductName: "To Restore",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)
	db.Delete(&product)

	err := productRepo.RestoreProduct(product.ID, user.ID)
	if err != nil {
		t.Fatalf("RestoreProduct() error = %v", err)
	}

	var restored dbModel.Product
	err = db.First(&restored, product.ID).Error
	if err != nil {
		t.Fatalf("Product not found after restore: %v", err)
	}
	if restored.RemovalReason != "" {
		t.Errorf("RestoreProduct() RemovalReason = %q, want empty string", restored.RemovalReason)
	}
}
func TestProductRepository_UserHasProductAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

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

	hasAccess := productRepo.UserHasProductAccess(user.ID, int(product.ID)) //nolint:gosec
	if !hasAccess {
		t.Error("User should have access to their own product")
	}

	hasAccess = productRepo.UserHasProductAccess(user.ID, 999)
	if hasAccess {
		t.Error("User should not have access to non-existent product")
	}
}
func TestProductRepository_BulkRestoreProductsSkipsUnattributableIDs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

	householdA := dbModel.Household{Name: "Household A"}
	householdB := dbModel.Household{Name: "Household B"}
	db.Create(&householdA)
	db.Create(&householdB)
	userA := authentication.User{
		Username:    "bulkrestorea",
		Password:    "password",
		MailAddress: "a@example.com",
		HouseholdID: householdA.ID,
	}
	db.Create(&userA)

	// An archived product of ANOTHER household: must be rejected, not restored.
	foreign := dbModel.Product{
		ProductName: "Foreign Archived",
		Barcode:     "4444444444444",
		HouseholdID: householdB.ID,
	}
	db.Create(&foreign)
	db.Delete(&foreign)

	countRows := func() int64 {
		t.Helper()
		var n int64
		if err := db.Unscoped().Model(&dbModel.Product{}).Count(&n).Error; err != nil {
			t.Fatalf("count failed: %v", err)
		}
		return n
	}

	before := countRows()

	unknownID := uint(999999)
	errs := productRepo.BulkRestoreProducts([]uint{unknownID, foreign.ID}, userA.ID)

	if len(errs) != 2 {
		t.Errorf("got %d errors, want 2 (both ids must fail)", len(errs))
	}
	if len(errs) == 2 && errs[0].ProductID() != unknownID {
		t.Errorf("errs[0].ProductID() = %d, want %d", errs[0].ProductID(), unknownID)
	}
	if got := countRows(); got != before {
		t.Errorf("product rows = %d, want %d (failed restores must not insert rows)", got, before)
	}

	// The foreign product must still be archived.
	var reloaded dbModel.Product
	if err := db.Unscoped().First(&reloaded, foreign.ID).Error; err != nil {
		t.Fatalf("failed to reload foreign product: %v", err)
	}
	if !reloaded.DeletedAt.Valid {
		t.Errorf("foreign product was restored across household boundary")
	}
}
func TestProductRepository_CreateProductsBulkEdges(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("empty rows return nil", func(t *testing.T) {
		created, err := productRepo.CreateProductsBulk(user.ID, nil)
		if err != nil || created != nil {
			t.Errorf("CreateProductsBulk(nil) = %v, %v; want nil, nil", created, err)
		}
	})

	// The rejection is whole: no products and no locations were written.
	t.Run("too many distinct new locations is rejected whole", func(t *testing.T) {
		rows := make([]ImportedProduct, 0, util.CsvImportMaxNewLocations+1)
		for i := range util.CsvImportMaxNewLocations + 1 {
			rows = append(rows, ImportedProduct{
				Product:         dbModel.Product{ProductName: fmt.Sprintf("P%d", i), Barcode: fmt.Sprintf("77%011d", i)},
				NewLocationName: fmt.Sprintf("Location %d", i),
			})
		}
		_, err := productRepo.CreateProductsBulk(user.ID, rows)
		if err != errors.ErrImportTooManyLocations {
			t.Errorf("CreateProductsBulk() error = %v, want %v", err, errors.ErrImportTooManyLocations)
		}

		var productCount, locationCount int64
		db.Model(&dbModel.Product{}).Where(util.QueryHouseholdId, household.ID).Count(&productCount)
		db.Model(&dbModel.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount)
		if productCount != 0 || locationCount != 0 {
			t.Errorf("rejected import wrote %d products and %d locations", productCount, locationCount)
		}
	})

	t.Run("shared location names create one location", func(t *testing.T) {
		rows := []ImportedProduct{
			{Product: dbModel.Product{ProductName: "A", Barcode: "8000000000001"}, NewLocationName: "Pantry"},
			{Product: dbModel.Product{ProductName: "B", Barcode: "8000000000002"}, NewLocationName: "pantry "},
		}
		created, err := productRepo.CreateProductsBulk(user.ID, rows)
		if err != nil {
			t.Fatalf("CreateProductsBulk() error = %v", err)
		}
		if len(created) != 1 || created[0] != "Pantry" {
			t.Errorf("created = %v, want [Pantry]", created)
		}
		var locationCount int64
		db.Model(&dbModel.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount)
		if locationCount != 1 {
			t.Errorf("locations = %d, want 1 (case/space-insensitive dedupe)", locationCount)
		}
	})
}

func TestProductRepository_GetProductIdentity(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ProductName = "Identity Product"
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateProduct.IsPrivate = true
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	t.Run("success returns identity fields", func(t *testing.T) {
		identity, err := productRepo.GetProductIdentity(product.ID, user.ID)
		if err != nil {
			t.Fatalf("GetProductIdentity() error = %v", err)
		}
		if identity.ProductName != "Identity Product" || identity.Barcode != product.Barcode {
			t.Errorf("GetProductIdentity() = %+v, want name and barcode", identity)
		}
	})

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, err := productRepo.GetProductIdentity(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetProductIdentity(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("cross-household access is rejected", func(t *testing.T) {
		if _, err := productRepo.GetProductIdentity(foreign.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductIdentity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		if _, err := productRepo.GetProductIdentity(privateProduct.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductIdentity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("owner can read own private product", func(t *testing.T) {
		if _, err := productRepo.GetProductIdentity(privateProduct.ID, owner.ID); err != nil {
			t.Errorf("GetProductIdentity() owner error = %v, want nil", err)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, err := productRepo.GetProductIdentity(9999, user.ID); err == nil {
			t.Error("GetProductIdentity(9999) error = nil, want error")
		}
	})
}
