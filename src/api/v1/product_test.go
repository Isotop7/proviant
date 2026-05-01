package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
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

	t.Run("get products successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{
			{ProductName: "Product 1", Barcode: "1111111111111"},
			{ProductName: "Product 2", Barcode: "2222222222222"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}

		GetProducts(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("repo error returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}

		GetProducts(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestGetArchivedProducts tests the GetArchivedProducts endpoint
func TestGetArchivedProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("get archived products successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{
			{ProductName: "Archived Product 1"},
			{ProductName: "Archived Product 2"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}

		GetArchivedProducts(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})
}

// TestGetProduct tests the GetProduct endpoint
func TestGetProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("get product successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Product = dbModel.Product{ProductName: "Test Product", Barcode: "1234567890123"}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		GetProduct(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("get product not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		GetProduct(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestCreateProduct tests the CreateProduct endpoint
func TestCreateProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("create product successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
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

		productData := dbModel.Product{
			ProductName: "New Product",
			Barcode:     "1234567890123",
		}
		body, _ := json.Marshal(productData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		CreateProduct(ctx)

		if w.Code != http.StatusCreated {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusCreated)
		}

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

	t.Run("update product successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		updateData := dbModel.ProductDTOPatch{ProductName: "Updated Product"}
		body, _ := json.Marshal(updateData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		UpdateProduct(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("update product not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		updateData := dbModel.ProductDTOPatch{ProductName: "Updated Product"}
		body, _ := json.Marshal(updateData)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")
		ctx.Params = []gin.Param{{Key: "id", Value: "999"}}

		UpdateProduct(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestDeleteProduct tests the DeleteProduct endpoint
func TestDeleteProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("delete product successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
		ctx.Request.URL = &url.URL{RawQuery: "archiveOnly=true"}

		DeleteProduct(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = gorm.ErrInvalidData
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
		ctx.Request.URL = &url.URL{}

		DeleteProduct(ctx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestSearchProducts tests the SearchProducts endpoint
func TestSearchProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("search products by name successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{
			{ProductName: "Apple Juice"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Request.URL = &url.URL{RawQuery: "queryParam=product_name&queryValue=Apple"}

		SearchProducts(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("search products by barcode successfully", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{
			{ProductName: "Test Product", Barcode: "1234567890123"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Request.URL = &url.URL{RawQuery: "queryParam=barcode&queryValue=1234567890123"}

		SearchProducts(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("search products with invalid parameter", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Request.URL = &url.URL{RawQuery: "queryParam=test&queryValue=test"}

		SearchProducts(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}
