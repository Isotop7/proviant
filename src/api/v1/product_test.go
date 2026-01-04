package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockOpenFoodFactsAPIController is a test double for OpenFoodFactsAPIControllerInterface
type MockOpenFoodFactsAPIController struct {
    MockGetDataset func(barcode string) (dbModel.Product, error)
}

// GetDataset implements OpenFoodFactsAPIControllerInterface and simply returns a generic product
func (m *MockOpenFoodFactsAPIController) GetDataset(barcode string) (dbModel.Product, error) {
    return m.MockGetDataset(barcode)
}

// TestGetProducts tests the GetProducts endpoint
func TestGetProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("get products successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and products
		testUser := authentication.User{
			ID:          1,
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		products := []dbModel.Product{
			{ProductName: "Product 1", Barcode: "1111111111111", HouseholdID: household.ID},
			{ProductName: "Product 2", Barcode: "2222222222222", HouseholdID: household.ID},
			{ProductName: "Product 3", Barcode: "3333333333333", HouseholdID: household.ID},
		}
		for i := range products {
			db.Create(&products[i])
		}

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}

		// Execute
		GetProducts(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("unauthorized access", func(t *testing.T) {
		// Setup test context without JWT
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		// Payload set to unknown user
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(99),
		})

		// Create test request
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}

		// Execute
		GetProducts(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestGetArchivedProducts tests the GetArchivedProducts endpoint
func TestGetArchivedProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("get archived products successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and archived products
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Create archived products
		archivedProducts := []dbModel.Product{
			{ProductName: "Archived Product 1", Barcode: "1111111111111", HouseholdID: household.ID, DeletedAt: gorm.DeletedAt{Time: time.Now()}},
			{ProductName: "Archived Product 2", Barcode: "2222222222222", HouseholdID: household.ID, DeletedAt: gorm.DeletedAt{Time: time.Now()}},
		}
		for i := range archivedProducts {
			db.Create(&archivedProducts[i])
		}

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}

		// Execute
		GetArchivedProducts(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

// TestGetProduct tests the GetProduct endpoint
func TestGetProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("get product successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and product
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		product := dbModel.Product{
			ProductName: "New Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

		// Execute
		GetProduct(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("get product not found", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request with non-existent product ID
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		// Execute
		GetProduct(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestCreateProduct tests the CreateProduct endpoint
func TestCreateProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("create product successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})
		mockOpenFoodFactsAPIController := &MockOpenFoodFactsAPIController{
			MockGetDataset: func(barcode string) (dbModel.Product, error) {
				return dbModel.Product{
					ProductName: "New Product",
					Barcode:     barcode,
					Categories:  "Test Category",
					Countries:   "de",
					ImageURL:    "https://example.com/image.jpg",
				}, nil
			},
		}
		ctx.Set("offacntrl", mockOpenFoodFactsAPIController)

		// Request body
		productData := dbModel.Product{
			ProductName: "New Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		body, _ := json.Marshal(productData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		// Execute
		CreateProduct(ctx)

		// Verify response
		if w.Code != http.StatusCreated {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusCreated)
		}

		// Verify response body
		var responseProduct dbModel.Product
		if err := json.Unmarshal(w.Body.Bytes(), &responseProduct); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if responseProduct.ProductName != "New Product" {
			t.Errorf("ProductName = %v, want %v", responseProduct.ProductName, "New Product")
		}
		if responseProduct.Barcode != "1234567890123" {
			t.Errorf("Barcode = %v, want %v", responseProduct.Barcode, "1234567890123")
		}
	})
}

// TestUpdateProduct tests the UpdateProduct endpoint
func TestUpdateProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("update product successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and product
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		product := dbModel.Product{
			ProductName: "Original Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Request body
		updateData := dbModel.ProductDTOPatch{
			ID:          product.ID,
			ProductName: "Updated Product",
		}
		body, _ := json.Marshal(updateData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

		// Execute
		UpdateProduct(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("update product not found", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Request body
		updateData := dbModel.ProductDTOPatch{
			ID:          999, // Non-existent product
			ProductName: "Updated Product",
		}
		body, _ := json.Marshal(updateData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		// Execute
		UpdateProduct(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestDeleteProduct tests the DeleteProduct endpoint
func TestDeleteProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("delete product successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and product
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		product := dbModel.Product{
			ProductName: "Product to Delete",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}
		ctx.Request.URL = &url.URL{
			Path:     fmt.Sprintf("/api/v1/products/%d", product.ID),
			RawQuery: "archiveOnly=true",
		}

		// Execute
		DeleteProduct(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		// Verify soft delete
		var deletedProduct dbModel.Product
		if err := db.Unscoped().First(&deletedProduct, product.ID).Error; err != nil {
			t.Fatalf("Failed to find deleted product: %v", err)
		}
		if deletedProduct.DeletedAt.Time.IsZero() {
			t.Error("Product was not soft-deleted")
		}
	})

	t.Run("delete product not found", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request with non-existent product ID
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}
		ctx.Request.URL = &url.URL{Path: "/products/999"}

		// Execute
		DeleteProduct(ctx)

		// Verify response
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestSearchProducts tests the SearchProducts endpoint
func TestSearchProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate schema
	if err := db.AutoMigrate(&dbModel.Household{}, &authentication.User{}, &dbModel.Product{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	t.Run("search products by name successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and products
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		products := []dbModel.Product{
			{ProductName: "Apple Juice", Barcode: "1111111111111", HouseholdID: household.ID},
			{ProductName: "Banana Bread", Barcode: "2222222222222", HouseholdID: household.ID},
			{ProductName: "Orange Soda", Barcode: "3333333333333", HouseholdID: household.ID},
		}
		for i := range products {
			db.Create(&products[i])
		}

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request with search parameters
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Request.URL = &url.URL{Path: "/?queryParam=productName&queryValue=Apple"}

		// Execute
		SearchProducts(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("search products by barcode successfully", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user and products
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		product := dbModel.Product{
			ProductName: "Test Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request with search parameters
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Request.URL = &url.URL{Path: "/?queryParam=barcode&queryValue=1234567890123"}

		// Execute
		SearchProducts(ctx)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("search products with invalid parameter", func(t *testing.T) {
		// Create test household
		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		// Create test user
		testUser := authentication.User{
			Username:    "testuser",
			Password:    "password123",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&testUser)

		// Setup test context
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		mockLogger := zerolog.Nop()
		ctx.Set("logger", &mockLogger)
		ctx.Set("dbHandle", db)
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
			static.TokenIdentityKey: float64(testUser.ID),
		})

		// Create test request with invalid search parameter
		ctx.Request = &http.Request{
			Header: make(http.Header),
		}
		ctx.Request.URL = &url.URL{
			RawQuery: "queryParam=test&queryValue=test",
		}

		// Execute
		SearchProducts(ctx)

		// Verify response
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}
