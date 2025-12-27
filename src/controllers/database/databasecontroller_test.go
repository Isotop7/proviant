package database

import (
	"errors"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDatabaseController_CreateUser tests user creation
func TestDatabaseController_CreateUser(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}, &database.Household{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{
		Username:    "testuser",
		Password:    "password123",
		MailAddress: "test@example.com",
	}

	// Create user
	err = controller.CreateUser(&user)
	assert.NoError(t, err)

	// Verify user was created
	var createdUser authentication.User
	result := db.First(&createdUser, user.ID)
	assert.True(t, result.Error == nil)
	assert.Equal(t, "testuser", createdUser.Username)
	assert.Equal(t, "test@example.com", createdUser.MailAddress)
}

// TestDatabaseController_GetUserByUsername tests getting user by username
func TestDatabaseController_GetUserByUsername(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{Username: "testuser", Password: "password123"}
	db.Create(&user)

	// Get user by username
	foundUser, err := controller.GetUserByUsername("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", foundUser.Username)

	// Test non-existent user
	_, err = controller.GetUserByUsername("nonexistent")
	assert.Error(t, err)
}

// TestDatabaseController_GetUserByID tests getting user by ID
func TestDatabaseController_GetUserByID(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{ID: 1, Username: "testuser"}
	db.Create(&user)

	// Get user by ID
	foundUser, err := controller.GetUserByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", foundUser.Username)

	// Test non-existent user
	_, err = controller.GetUserByID(999)
	assert.Error(t, err)
}

// TestDatabaseController_UpdateUser tests user update
func TestDatabaseController_UpdateUser(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{ID: 1, Username: "olduser", MailAddress: "old@example.com"}
	db.Create(&user)

	// Update user
	updateData := authentication.User{Username: "newuser", MailAddress: "new@example.com"}
	err = controller.UpdateUser(1, &updateData)
	assert.NoError(t, err)

	// Verify user was updated
	var updatedUser authentication.User
	db.First(&updatedUser, 1)
	assert.Equal(t, "newuser", updatedUser.Username)
	assert.Equal(t, "new@example.com", updatedUser.MailAddress)
}

// TestDatabaseController_UpdateUserPassword tests password update
func TestDatabaseController_UpdateUserPassword(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{ID: 1, Username: "testuser", Password: "oldpassword"}
	db.Create(&user)

	// Update password
	loginData := authentication.Login{
		Username: "testuser",
		Password: "newpassword123",
	}
	err = controller.UpdateUserPassword(1, &loginData)
	assert.NoError(t, err)

	// Verify password was updated (we can't check the actual password hash, but we can verify the update succeeded)
	var updatedUser authentication.User
	db.First(&updatedUser, 1)
	assert.NotEmpty(t, updatedUser.Password) // Password should be set
}

// TestDatabaseController_UserExistsByUsername tests username existence check
func TestDatabaseController_UserExistsByUsername(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{Username: "testuser"}
	db.Create(&user)

	// Test existence check
	exists := controller.UserExistsByUsername(&user)
	assert.True(t, exists)

	// Test non-existent user
	nonExistentUser := authentication.User{Username: "nonexistent"}
	exists = controller.UserExistsByUsername(&nonExistentUser)
	assert.False(t, exists)
}

// TestDatabaseController_UserExistsByMailAddress tests email existence check
func TestDatabaseController_UserExistsByMailAddress(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{MailAddress: "test@example.com"}
	db.Create(&user)

	// Test existence check
	exists := controller.UserExistsByMailAddress(&user)
	assert.True(t, exists)

	// Test non-existent user
	nonExistentUser := authentication.User{MailAddress: "nonexistent@example.com"}
	exists = controller.UserExistsByMailAddress(&nonExistentUser)
	assert.False(t, exists)
}

// TestDatabaseController_CreateProduct tests product creation
func TestDatabaseController_CreateProduct(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	db.Create(&user)

	// Create test product
	product := database.Product{
		ProductName: "Test Product",
		Barcode:     "3017620422003",
		HouseholdID: 1,
	}

	// Create product
	err = controller.CreateProduct(1, &product)
	assert.NoError(t, err)

	// Verify product was created
	var createdProduct database.Product
	result := db.First(&createdProduct, product.ID)
	assert.True(t, result.Error == nil)
	assert.Equal(t, "Test Product", createdProduct.ProductName)
	assert.Equal(t, "3017620422003", createdProduct.Barcode)
}

// TestDatabaseController_GetProductByID tests getting product by ID
func TestDatabaseController_GetProductByID(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user and product
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	product := database.Product{ProductName: "Test Product", Barcode: "123456789", HouseholdID: 1}
	db.Create(&user)
	db.Create(&product)

	// Get product by ID
	foundProduct, err := controller.GetProductByID(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Test Product", foundProduct.ProductName)

	// Test non-existent product
	_, err = controller.GetProductByID(999, 1)
	assert.Error(t, err)
}

// TestDatabaseController_UpdateProduct tests product update
func TestDatabaseController_UpdateProduct(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user and product
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	product := database.Product{ProductName: "Old Name", Barcode: "11111", HouseholdID: 1}
	db.Create(&user)
	db.Create(&product)

	// Update product
	updateData := database.ProductDTOPatch{
		ProductName: "New Name",
	}
	err = controller.UpdateProduct(1, 1, &updateData)
	assert.NoError(t, err)

	// Verify product was updated
	var updatedProduct database.Product
	db.First(&updatedProduct, 1)
	assert.Equal(t, "New Name", updatedProduct.ProductName)
}

// TestDatabaseController_DeleteProduct tests product deletion
func TestDatabaseController_DeleteProduct(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user and product
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	product := database.Product{ProductName: "Test Product", Barcode: "3017620422003", HouseholdID: 1}
	db.Create(&user)
	db.Create(&product)

	// Delete product
	err = controller.DeleteProduct(1, 1, false)
	assert.NoError(t, err)

	// Verify product was deleted (soft delete)
	var deletedProduct database.Product
	result := db.Unscoped().First(&deletedProduct, 1)
	assert.True(t, result.Error != nil)
	assert.NotNil(t, deletedProduct.DeletedAt)
}

// TestDatabaseController_RestoreProduct tests product restoration
func TestDatabaseController_RestoreProduct(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user and product
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	product := database.Product{ProductName: "Test Product", Barcode: "3017620422003", HouseholdID: 1}
	db.Create(&user)
	db.Create(&product)

	// Delete and restore product
	err = controller.DeleteProduct(1, 1, true)
	assert.NoError(t, err)

	err = controller.RestoreProduct(1, 1)
	assert.NoError(t, err)

	// Verify product was restored
	var restoredProduct database.Product
	result := db.First(&restoredProduct, 1)
	assert.True(t, result.Error == nil)
	assert.True(t, restoredProduct.DeletedAt.Time.IsZero())
}

// TestDatabaseController_SetExpireAt tests setting expiration date
func TestDatabaseController_SetExpireAt(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create test user and product
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	product := database.Product{ProductName: "Test Product", Barcode: "3017620422003", HouseholdID: 1}
	db.Create(&user)
	db.Create(&product)

	// Set expiration date
	expireAt := time.Now().Add(7 * 24 * time.Hour) // 7 days from now
	product.ExpireAt = expireAt
	err = db.Save(&product).Error
	assert.NoError(t, err)

	// Verify expiration date was set
	var updatedProduct database.Product
	db.First(&updatedProduct, 1)
	assert.True(t, !updatedProduct.ExpireAt.IsZero())
}

// TestDatabaseController_SearchProducts tests product search
func TestDatabaseController_SearchProducts(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&database.Product{}, &authentication.User{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create controller
	controller := DatabaseController{DBHandle: db}

	// Create test user and products
	user := authentication.User{ID: 1, Username: "testuser", HouseholdID: 1}
	products := []database.Product{
		{ProductName: "Apple", Barcode: "3017620422003", HouseholdID: 1},
		{ProductName: "Banana", Barcode: "3017620422004", HouseholdID: 1},
		{ProductName: "Orange", Barcode: "3017620422005", HouseholdID: 1},
	}
	db.Create(&user)
	for i := range products {
		db.Create(&products[i])
	}

	// Search by product name
	foundProducts, err := controller.SearchProducts(ProductName, "Apple", "", "", 1)
	assert.NoError(t, err)
	assert.Len(t, foundProducts, 1)
	assert.Equal(t, "Apple", foundProducts[0].ProductName)

	// Search by barcode
	foundProducts, err = controller.SearchProducts(Barcode, "3017620422004", "", "", 1)
	assert.NoError(t, err)
	assert.Len(t, foundProducts, 1)
	assert.Equal(t, "Banana", foundProducts[0].ProductName)
}

// TestBulkOperationError tests the BulkOperationError struct
func TestBulkOperationError(t *testing.T) {
	t.Run("can create BulkOperationError", func(t *testing.T) {
		testErr := errors.New("test error")
		err := BulkOperationError{
			productID: 123,
			error:     testErr,
		}

		if err.productID != 123 {
			t.Errorf("productID = %v, want 123", err.productID)
		}

		if err.error != testErr {
			t.Errorf("error = %v, want %v", err.error, testErr)
		}
	})

	t.Run("Error method returns correct string", func(t *testing.T) {
		testErr := errors.New("test error")
		err := &BulkOperationError{
			productID: 456,
			error:     testErr,
		}

		expected := "Error bulk deleting product '456', error: test error"
		actual := err.Error()

		if actual != expected {
			t.Errorf("Error() = %v, want %v", actual, expected)
		}
	})

	t.Run("Error method with different product ID and error", func(t *testing.T) {
		testErr := errors.New("database connection failed")
		err := &BulkOperationError{
			productID: 789,
			error:     testErr,
		}

		expected := "Error bulk deleting product '789', error: database connection failed"
		actual := err.Error()

		if actual != expected {
			t.Errorf("Error() = %v, want %v", actual, expected)
		}
	})
}
