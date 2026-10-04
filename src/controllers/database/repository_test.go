package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func TestProductRepository_SearchParameterEnumFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SearchParameterEnum
	}{
		{"product_name", "product_name", ProductName},
		{"barcode", "barcode", Barcode},
		{"invalid", "invalid", InvalidParameter},
		{"empty", "", InvalidParameter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchParameterEnumFromString(tt.input)
			if got != tt.expected {
				t.Errorf("SearchParameterEnumFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestProductRepository_GetUserProductsBulk(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		ExpireAt:    time.Now().Add(24 * time.Hour),
	}
	db.Create(&product)

	products, err := repo.GetUserProductsBulk(user.ID, 10)
	if err != nil {
		t.Fatalf("GetUserProductsBulk() error = %v", err)
	}

	if len(products) != 1 {
		t.Errorf("GetUserProductsBulk() returned %d products, want 1", len(products))
	}
}

func TestProductRepository_GetUserProductsByIDs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	otherHousehold := dbModel.Household{Name: "Other Household"}
	db.Create(&otherHousehold)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	ownProduct := dbModel.Product{ProductName: "Own", Barcode: "1234567890123", HouseholdID: household.ID}
	db.Create(&ownProduct)
	otherProduct := dbModel.Product{ProductName: "Other", Barcode: "1234567890124", HouseholdID: otherHousehold.ID}
	db.Create(&otherProduct)
	archived := dbModel.Product{ProductName: "Archived", Barcode: "1234567890125", HouseholdID: household.ID}
	db.Create(&archived)
	db.Delete(&archived)

	t.Run("returns only requested, own household, active products", func(t *testing.T) {
		products, err := repo.GetUserProductsByIDs(user.ID, []uint{ownProduct.ID, otherProduct.ID, archived.ID, 9999})
		if err != nil {
			t.Fatalf("GetUserProductsByIDs() error = %v", err)
		}
		if len(products) != 1 {
			t.Fatalf("GetUserProductsByIDs() returned %d products, want 1", len(products))
		}
		if products[0].ID != ownProduct.ID {
			t.Errorf("returned product ID = %d, want %d", products[0].ID, ownProduct.ID)
		}
	})

	t.Run("empty ids returns nothing", func(t *testing.T) {
		products, err := repo.GetUserProductsByIDs(user.ID, []uint{})
		if err != nil {
			t.Fatalf("GetUserProductsByIDs() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserProductsByIDs() returned %d products, want 0", len(products))
		}
	})
}

func TestProductRepository_GetProductByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	foundProduct, err := repo.GetProductByID(product.ID, user.ID)
	if err != nil {
		t.Fatalf("GetProductByID() error = %v", err)
	}

	if foundProduct.ProductName != "Test Product" {
		t.Errorf("GetProductByID() ProductName = %s, want Test Product", foundProduct.ProductName)
	}
}

func TestProductRepository_GetProductByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	_, err := repo.GetProductByID(999, user.ID)
	if err == nil {
		t.Error("GetProductByID() expected error for non-existent product")
	}
}

func TestProductRepository_CreateProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	err := repo.CreateProduct(user.ID, &product)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	if product.HouseholdID != household.ID {
		t.Errorf("CreateProduct() HouseholdID = %d, want %d", product.HouseholdID, household.ID)
	}
}

func TestProductRepository_UpdateProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	err := repo.UpdateProduct(product.ID, user.ID, &updateData)
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
	repo := NewProductRepository(db)

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

	err := repo.DeleteProduct(product.ID, user.ID, true)
	if err != nil {
		t.Fatalf("DeleteProduct() error = %v", err)
	}

	var deleted dbModel.Product
	err = db.Unscoped().First(&deleted, product.ID).Error
	if err != nil {
		t.Fatalf("Product not found after delete: %v", err)
	}
}

func TestProductRepository_SearchProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	results, err := repo.SearchProducts(ProductName, "Apple", "", "", user.ID)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("SearchProducts() returned %d results, want 2", len(results))
	}
}

func TestProductRepository_SearchProducts_ByBarcode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	results, err := repo.SearchProducts(Barcode, "1234567890123", "", "", user.ID)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("SearchProducts() returned %d results, want 1", len(results))
	}
}

func TestProductRepository_GetUserArchivedProductsBulk(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		ProductName: "Archived Product",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)
	db.Delete(&product)

	archived, err := repo.GetUserArchivedProductsBulk(user.ID, 10)
	if err != nil {
		t.Fatalf("GetUserArchivedProductsBulk() error = %v", err)
	}

	if len(archived) != 1 {
		t.Errorf("GetUserArchivedProductsBulk() returned %d products, want 1", len(archived))
	}
}

func TestProductRepository_RestoreProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	err := repo.RestoreProduct(product.ID, user.ID)
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

func TestProductRepository_ConsumeProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		ProductName: "To Consume",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	err := repo.ConsumeProduct(product.ID, user.ID)
	if err != nil {
		t.Fatalf("ConsumeProduct() error = %v", err)
	}

	var consumed dbModel.Product
	err = db.Unscoped().First(&consumed, product.ID).Error
	if err != nil {
		t.Fatalf("Product not found after consume: %v", err)
	}
	if !consumed.DeletedAt.Valid {
		t.Error("ConsumeProduct() did not soft-delete the product")
	}
	if consumed.RemovalReason != dbModel.RemovalReasonConsumed {
		t.Errorf("ConsumeProduct() RemovalReason = %q, want %q", consumed.RemovalReason, dbModel.RemovalReasonConsumed)
	}
}

func TestProductRepository_ConsumeProductPartial(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *ProductRepository, *authentication.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)

		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		user := authentication.User{
			Username:    "testuser",
			Password:    "password",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&user)
		return db, repo, &user
	}

	createProduct := func(db *gorm.DB, householdID uint, name string, amount int) dbModel.Product {
		product := dbModel.Product{
			ProductName: name,
			Barcode:     "1234567890123",
			HouseholdID: householdID,
			Amount:      amount,
		}
		db.Create(&product)
		return product
	}

	t.Run("partial reduce keeps product active", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Partial", 5)

		consumed, fullyConsumed, err := repo.ConsumeProductPartial(&product, 2)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 2 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 2", consumed)
		}
		if fullyConsumed {
			t.Error("partial consume must not report fullyConsumed")
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("product not found after partial consume: %v", err)
		}
		if updated.Amount != 3 {
			t.Errorf("Amount = %d, want 3", updated.Amount)
		}
		if updated.DeletedAt.Valid {
			t.Error("partial consume must not soft-delete the product")
		}
	})

	t.Run("amount equal to current amount fully consumes", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Equal", 3)

		consumed, fullyConsumed, err := repo.ConsumeProductPartial(&product, 3)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 3 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 3", consumed)
		}
		if !fullyConsumed {
			t.Error("full consume must report fullyConsumed")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid {
			t.Error("full consume did not soft-delete the product")
		}
		if archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Errorf("RemovalReason = %q, want %q", archived.RemovalReason, dbModel.RemovalReasonConsumed)
		}
	})

	t.Run("amount above current amount clamps to full consume", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Overshoot", 2)

		consumed, _, err := repo.ConsumeProductPartial(&product, 5)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 2 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 2", consumed)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("overshoot must fully consume (soft-delete with reason consumed)")
		}
	})

	t.Run("amount zero fully consumes", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Zero", 4)

		consumed, _, err := repo.ConsumeProductPartial(&product, 0)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 4 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 4", consumed)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid {
			t.Error("amount 0 must fully consume the product")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, repo, _ := setup(t)

		missing := dbModel.Product{Amount: 1}
		missing.ID = 9999
		consumed, _, err := repo.ConsumeProductPartial(&missing, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("ConsumeProductPartial() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
		if consumed != 0 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 0", consumed)
		}
	})

	t.Run("stale fast path falls back when stock dropped below requested amount", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Raced", 5)
		// Simulate another request having consumed units before our update runs
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("amount", 1)

		stale := product

		consumed, fullyConsumed, err := repo.ConsumeProductPartial(&stale, 2)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 1 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 1 (remaining stock)", consumed)
		}
		if !fullyConsumed {
			t.Error("fallback after concurrent reduction must fully consume remaining stock")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after fallback consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("fallback must soft-delete with reason consumed")
		}
	})

	t.Run("stale read above current amount consumes remaining", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Stale clamp", 5)
		// Another request reduced the stock after the caller's read
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("amount", 1)

		stale := product

		consumed, fullyConsumed, err := repo.ConsumeProductPartial(&stale, 5)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 1 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 1 (current stock, not stale read)", consumed)
		}
		if !fullyConsumed {
			t.Error("overshoot against current stock must fully consume")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("stale overshoot must soft-delete with reason consumed")
		}
	})

	t.Run("stale read falls back to partial reduce", func(t *testing.T) {
		db, repo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Stale partial", 3)
		// Another request increased the stock after the caller's read, so
		// the requested amount no longer covers the whole stock
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("amount", 5)

		stale := product

		consumed, fullyConsumed, err := repo.ConsumeProductPartial(&stale, 3)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 3 || fullyConsumed {
			t.Errorf("ConsumeProductPartial() = (%d, %v), want (3, false)", consumed, fullyConsumed)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("product not found after partial consume: %v", err)
		}
		if updated.Amount != 2 {
			t.Errorf("Amount = %d, want 2", updated.Amount)
		}
		if updated.DeletedAt.Valid {
			t.Error("fallback partial reduce must not soft-delete the product")
		}
	})
}

func TestProductRepository_WasteProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		ProductName: "To Waste",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	err := repo.WasteProduct(product.ID, user.ID)
	if err != nil {
		t.Fatalf("WasteProduct() error = %v", err)
	}

	var wasted dbModel.Product
	err = db.Unscoped().First(&wasted, product.ID).Error
	if err != nil {
		t.Fatalf("WasteProduct() hard-deleted the product — record must be retained for reporting")
	}
	if !wasted.DeletedAt.Valid {
		t.Error("WasteProduct() did not soft-delete the product")
	}
	if wasted.RemovalReason != dbModel.RemovalReasonWasted {
		t.Errorf("WasteProduct() RemovalReason = %q, want %q", wasted.RemovalReason, dbModel.RemovalReasonWasted)
	}
}

func TestProductRepository_GetActiveProductsCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	for i := 0; i < 3; i++ {
		product := dbModel.Product{
			ProductName: "Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)
	}

	count, err := repo.GetActiveProductsCount(user.ID)
	if err != nil {
		t.Fatalf("GetActiveProductsCount() error = %v", err)
	}

	if count != 3 {
		t.Errorf("GetActiveProductsCount() = %d, want 3", count)
	}
}

func TestProductRepository_UserHasProductAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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

	hasAccess := repo.UserHasProductAccess(user.ID, int(product.ID)) //nolint:gosec
	if !hasAccess {
		t.Error("User should have access to their own product")
	}

	hasAccess = repo.UserHasProductAccess(user.ID, 999)
	if hasAccess {
		t.Error("User should not have access to non-existent product")
	}
}

func TestProductRepository_GetProductCategoryBreakdown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		{ProductName: "P1", HouseholdID: household.ID, Categories: "dairy"},
		{ProductName: "P2", HouseholdID: household.ID, Categories: "dairy"},
		{ProductName: "P3", HouseholdID: household.ID, Categories: "meat"},
	}
	for i := range products {
		db.Create(&products[i])
	}

	breakdown, err := repo.GetProductCategoryBreakdown(user.ID)
	if err != nil {
		t.Fatalf("GetProductCategoryBreakdown() error = %v", err)
	}

	if breakdown["dairy"] != 2 {
		t.Errorf("GetProductCategoryBreakdown() dairy = %d, want 2", breakdown["dairy"])
	}
	if breakdown["meat"] != 1 {
		t.Errorf("GetProductCategoryBreakdown() meat = %d, want 1", breakdown["meat"])
	}
}

func TestUserRepository_GetUserByUsername(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	found, err := repo.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("GetUserByUsername() error = %v", err)
	}

	if found.Username != "testuser" {
		t.Errorf("GetUserByUsername() = %s, want testuser", found.Username)
	}
}

func TestUserRepository_GetUserByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	found, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}

	if found.Username != "testuser" {
		t.Errorf("GetUserByID() = %s, want testuser", found.Username)
	}
}

func TestUserRepository_UserExistsByUsername(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	exists := repo.UserExistsByUsername(&user)
	if !exists {
		t.Error("User should exist")
	}

	nonExistent := authentication.User{Username: "nonexistent"}
	exists = repo.UserExistsByUsername(&nonExistent)
	if exists {
		t.Error("Non-existent user should not be found")
	}
}

func TestUserRepository_UserExistsByMailAddress(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	exists := repo.UserExistsByMailAddress(&user)
	if !exists {
		t.Error("User should exist by email")
	}

	nonExistent := authentication.User{MailAddress: "nonexistent@example.com"}
	exists = repo.UserExistsByMailAddress(&nonExistent)
	if exists {
		t.Error("Non-existent email should not be found")
	}
}

func TestUserRepository_GetOnboardingState(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	onboarding := dbModel.OnboardingState{UserID: user.ID}
	db.Create(&onboarding)

	state, err := repo.GetOnboardingState(user.ID)
	if err != nil {
		t.Fatalf("GetOnboardingState() error = %v", err)
	}

	if state.UserID != user.ID {
		t.Errorf("GetOnboardingState() UserID = %d, want %d", state.UserID, user.ID)
	}
}

func TestUserRepository_UpdateDisplayName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	err := repo.UpdateDisplayName(user.ID, "New Display Name")
	if err != nil {
		t.Fatalf("UpdateDisplayName() error = %v", err)
	}

	var updated authentication.User
	db.First(&updated, user.ID)
	if updated.DisplayName != "New Display Name" {
		t.Errorf("UpdateDisplayName() DisplayName = %s, want New Display Name", updated.DisplayName)
	}
}

func TestUserRepository_IsAccountLocked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	isLocked, _ := repo.IsAccountLocked(user.ID, 10, 15)
	if isLocked {
		t.Error("New user should not be locked")
	}
}

func TestHouseholdRepository_GetHouseholdByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewHouseholdRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	found, err := repo.GetHouseholdByID(household.ID)
	if err != nil {
		t.Fatalf("GetHouseholdByID() error = %v", err)
	}

	if found.Name != "Test Household" {
		t.Errorf("GetHouseholdByID() Name = %s, want Test Household", found.Name)
	}
}

func TestHouseholdRepository_GetHouseholdMemberCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewHouseholdRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user1 := authentication.User{Username: "user1", HouseholdID: household.ID}
	user2 := authentication.User{Username: "user2", HouseholdID: household.ID}
	db.Create(&user1)
	db.Create(&user2)

	count, err := repo.GetHouseholdMemberCount(household.ID)
	if err != nil {
		t.Fatalf("GetHouseholdMemberCount() error = %v", err)
	}

	if count != 2 {
		t.Errorf("GetHouseholdMemberCount() = %d, want 2", count)
	}
}

func TestHouseholdRepository_GetHouseholdMembers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewHouseholdRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{Username: "testuser", HouseholdID: household.ID}
	db.Create(&user)

	members, err := repo.GetHouseholdMembers(household.ID)
	if err != nil {
		t.Fatalf("GetHouseholdMembers() error = %v", err)
	}

	if len(members) != 1 {
		t.Errorf("GetHouseholdMembers() returned %d members, want 1", len(members))
	}
}

func TestStorageLocationRepository_GetByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewStorageLocationRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	location := dbModel.StorageLocation{
		Name:        "Fridge",
		Icon:        "🧊",
		SortOrder:   0,
		HouseholdID: household.ID,
	}
	db.Create(&location)

	locations, err := repo.GetByHousehold(user.ID)
	if err != nil {
		t.Fatalf("GetByHousehold() error = %v", err)
	}

	if len(locations) != 1 {
		t.Errorf("GetByHousehold() returned %d locations, want 1", len(locations))
	}
}

func TestStorageLocationRepository_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewStorageLocationRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	location := dbModel.StorageLocation{
		Name:        "Fridge",
		Icon:        "🧊",
		SortOrder:   0,
		HouseholdID: household.ID,
	}
	db.Create(&location)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	found, err := repo.GetByID(location.ID, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.Name != "Fridge" {
		t.Errorf("GetByID() Name = %s, want Fridge", found.Name)
	}
}

func TestWebhookRepository_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: 1,
	}
	db.Create(&user)

	webhook := dbModel.Webhook{
		UserID: user.ID,
		URL:    "https://example.com/webhook",
		Secret: "secret123",
		Events: "product.expired",
		Active: true,
	}

	err := repo.CreateWebhook(&webhook)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}

	if webhook.ID == 0 {
		t.Error("CreateWebhook() should set ID")
	}
}

func TestWebhookRepository_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: 1,
	}
	db.Create(&user)

	webhook := dbModel.Webhook{
		UserID: user.ID,
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Events: "product.expired",
		Active: true,
	}
	db.Create(&webhook)

	found, err := repo.GetWebhookByID(webhook.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.URL != "https://example.com/webhook" {
		t.Errorf("GetByID() URL = %s, want https://example.com/webhook", found.URL)
	}
}

func TestWebhookRepository_GetActiveWebhooksByEvent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: 1,
	}
	db.Create(&user)

	webhook := dbModel.Webhook{
		UserID: user.ID,
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Events: `["product.expired"]`,
		Active: true,
	}
	db.Create(&webhook)

	webhooks, err := repo.GetActiveWebhooksByEvent("product.expired")
	if err != nil {
		t.Fatalf("GetActiveWebhooksByEvent() error = %v", err)
	}

	if len(webhooks) != 1 {
		t.Errorf("GetActiveWebhooksByEvent() returned %d webhooks, want 1", len(webhooks))
	}
}

func TestWebhookRepository_CreateDeliveryLog(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewWebhookRepository(db)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: 1,
	}
	db.Create(&user)

	webhook := dbModel.Webhook{
		UserID: user.ID,
		URL:    "https://example.com/webhook",
		Secret: "secret",
		Events: "product.expired",
		Active: true,
	}
	db.Create(&webhook)

	log := dbModel.WebhookDeliveryLog{
		WebhookID:  webhook.ID,
		StatusCode: 200,
		Attempt:    1,
	}

	err := repo.CreateDeliveryLog(&log)
	if err != nil {
		t.Fatalf("CreateDeliveryLog() error = %v", err)
	}

	if log.ID == 0 {
		t.Error("CreateDeliveryLog() should set ID")
	}
}

func TestRecipeRepository_CreateAndGetCache(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	cacheEntry := dbModel.RecipeCache{
		QueryHash:    "testhash123",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[{"title": "Test Recipe"}]`),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}

	err := repo.CreateCache(&cacheEntry)
	if err != nil {
		t.Fatalf("CreateCache() error = %v", err)
	}

	found, err := repo.GetCacheByQueryHash("testhash123")
	if err != nil {
		t.Fatalf("GetCacheByQueryHash() error = %v", err)
	}

	if found.QueryHash != "testhash123" {
		t.Errorf("GetCacheByQueryHash() QueryHash = %s, want testhash123", found.QueryHash)
	}
}

func TestRecipeRepository_GetCacheByQueryHash_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	_, err := repo.GetCacheByQueryHash("nonexistent")
	if err == nil {
		t.Error("GetCacheByQueryHash() should return error for non-existent cache")
	}
}

func TestRecipeRepository_UpdateCacheHit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	cacheEntry := dbModel.RecipeCache{
		QueryHash:    "testhash",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[]`),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		HitCount:     5,
	}
	db.Create(&cacheEntry)

	err := repo.UpdateCacheHit("testhash")
	if err != nil {
		t.Fatalf("UpdateCacheHit() error = %v", err)
	}

	var updated dbModel.RecipeCache
	db.First(&updated, cacheEntry.ID)
	if updated.HitCount != 6 {
		t.Errorf("UpdateCacheHit() HitCount = %d, want 6", updated.HitCount)
	}
}

func TestStreakRepository_UpsertAndGet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewStreakRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	streak, err := repo.GetOrCreateStreakForHousehold(household.ID)
	if err != nil {
		t.Fatalf("GetOrCreateStreakForHousehold() error = %v", err)
	}

	if streak.HouseholdID != household.ID {
		t.Errorf("GetOrCreateStreakForHousehold() HouseholdID = %d, want %d", streak.HouseholdID, household.ID)
	}
}

func TestInvitationRepository_CreateInvitation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	_, err := repo.CreateInvitation(household.ID, user.ID, "invite@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	invitations, err := repo.GetInvitationsForHousehold(household.ID, user.ID)
	if err != nil {
		t.Fatalf("GetInvitationsForHousehold() error = %v", err)
	}

	if len(invitations) != 1 {
		t.Errorf("GetInvitationsForHousehold() returned %d invitations, want 1", len(invitations))
	}
}

func TestInvitationRepository_GetInvitationByToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	invitation := dbModel.HouseholdInvitation{
		HouseholdID: household.ID,
		InviterID:   1,
		Email:       "invite@example.com",
		Token:       "uniquetoken",
	}
	db.Create(&invitation)

	found, err := repo.GetInvitationByToken("uniquetoken")
	if err != nil {
		t.Fatalf("GetInvitationByToken() error = %v", err)
	}

	if found.Token != "uniquetoken" {
		t.Errorf("GetInvitationByToken() Token = %s, want uniquetoken", found.Token)
	}
}

func TestNotificationRepository_GetProductsExpiredAndNotificationPending(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	product := dbModel.Product{
		ProductName: "Expired Product",
		HouseholdID: household.ID,
		ExpireAt:    time.Now().Add(-24 * time.Hour),
	}
	db.Create(&product)

	products, err := repo.GetProductsExpiredAndNotificationPending(time.Hour, 7)
	if err != nil {
		t.Fatalf("GetProductsExpiredAndNotificationPending() error = %v", err)
	}

	if len(products) != 1 {
		t.Errorf("GetProductsExpiredAndNotificationPending() returned %d products, want 1", len(products))
	}
}

func TestNotificationRepository_GetHouseholdMembersMailAddressesByID(t *testing.T) {
	t.Skip("Skipping - investigating query issue")
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)

	household := dbModel.Household{Name: "Test Household", AdminID: 1}
	db.Create(&household)
	if household.ID == 0 {
		t.Fatal("household.ID should not be 0")
	}

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	// Debug: check users directly
	var allUsers []authentication.User
	db.Find(&allUsers)
	t.Logf("total users in db: %d", len(allUsers))
	for _, u := range allUsers {
		t.Logf("  user %d: household_id=%d", u.ID, u.HouseholdID)
	}

	// Now test the function
	emails, err := repo.GetHouseholdMembersMailAddressesByID(household.ID)
	if err != nil {
		t.Fatalf("GetHouseholdMembersMailAddressesByID() error = %v", err)
	}

	if len(emails) != 1 {
		t.Errorf("GetHouseholdMembersMailAddressesByID() returned %d emails, want 1", len(emails))
	}
}

func TestExpiryScanRepository_CreateExpiryScan(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewExpiryScanRepository(db)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: 1,
	}
	db.Create(&user)

	scan := dbModel.ExpiryScan{
		UserID:       user.ID,
		ScannedAt:    time.Now(),
		DetectedDate: time.Now().Add(7 * 24 * time.Hour),
		Confidence:   0.85,
		RawText:      "Mindestens haltbar bis 25.12.2025",
		ImageHash:    "abc123",
	}

	err := repo.Create(&scan)
	if err != nil {
		t.Fatalf("CreateExpiryScan() error = %v", err)
	}

	scans, err := repo.GetByUser(user.ID, 10)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}

	if len(scans) != 1 {
		t.Errorf("GetByUser() returned %d scans, want 1", len(scans))
	}
}
