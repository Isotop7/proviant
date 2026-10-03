package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// MockOpenFoodFactsAPIController is a test double for DatasetGetter
type MockOpenFoodFactsAPIController struct {
	MockGetDataset func(barcode string) (dbModel.Product, error)
}

// GetDataset implements DatasetGetter and simply returns a generic product
func (m *MockOpenFoodFactsAPIController) GetDataset(barcode string) (dbModel.Product, error) {
	return m.MockGetDataset(barcode)
}

func newTestAppContext(m *repomocks.MockRepositoryContainer, userID uint) *AppContext {
	logger := zerolog.Nop()
	return &AppContext{
		Logger:      &logger,
		Repos:       m.ToRepositoryContainer(),
		UserID:      userID,
		Products:    services.NewProductService(m.ToRepositoryContainer(), &logger),
		Consumption: services.NewConsumptionService(m.ToRepositoryContainer(), &logger),
	}
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

		appCtx := newTestAppContext(m, 1)
		GetProducts(ctx, appCtx)

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

		appCtx := newTestAppContext(m, 1)
		GetProducts(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetArchivedProducts(ctx, appCtx)

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

		appCtx := newTestAppContext(m, 1)
		GetProduct(ctx, appCtx)

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

		appCtx := newTestAppContext(m, 1)
		GetProduct(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		CreateProduct(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		UpdateProduct(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		UpdateProduct(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
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

		appCtx := newTestAppContext(m, 1)
		DeleteProduct(ctx, appCtx)

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

		appCtx := newTestAppContext(m, 1)
		DeleteProduct(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		SearchProducts(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		SearchProducts(ctx, appCtx)

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

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		SearchProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestGetProductsByBarcode tests the GetProductsByBarcode endpoint
func TestGetProductsByBarcode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid EAN-13 barcode returns 200", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{
			{ProductName: "Nutella", Barcode: "4001724814405"},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "4001724814405"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("barcode too short returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "123456789012"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("barcode too long returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "12345678901234"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("non-numeric barcode returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "1234abcdefghi"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("valid length with invalid checksum returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "4001724814400"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("non-integer barcode returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "barcode", Value: "abc"}}

		appCtx := &AppContext{
			Logger: &zerolog.Logger{},
			Repos:  m.ToRepositoryContainer(),
			UserID: 1,
		}
		GetProductsByBarcode(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

// TestCookProducts tests the CookProducts endpoint
func TestCookProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("cook request returns 200 with response model", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		cookRequest := apiModel.CookProductsAPIModel{
			Items: []apiModel.CookItemAPIModel{
				{ProductID: 1, Amount: 2},
				{ProductID: 2, Amount: 0},
			},
		}
		body, _ := json.Marshal(cookRequest)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}

		var response apiModel.CookResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if response.Consumed != 2 {
			t.Errorf("Consumed = %d, want 2 (mock reports full consume for every item)", response.Consumed)
		}
		if response.Partial != 0 {
			t.Errorf("Partial = %d, want 0", response.Partial)
		}
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		body := []byte("{invalid")
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("empty items returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		cookRequest := apiModel.CookProductsAPIModel{Items: []apiModel.CookItemAPIModel{}}
		body, _ := json.Marshal(cookRequest)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("more than 100 items returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		cookRequest := apiModel.CookProductsAPIModel{}
		for i := 1; i <= 101; i++ {
			cookRequest.Items = append(cookRequest.Items, apiModel.CookItemAPIModel{ProductID: uint(i), Amount: 1})
		}
		body, _ := json.Marshal(cookRequest)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("all items failed returns 400 with errors", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.ConsumePartialErr = errors.ErrProductConcurrentModification
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		cookRequest := apiModel.CookProductsAPIModel{
			Items: []apiModel.CookItemAPIModel{
				{ProductID: 1, Amount: 1},
				{ProductID: 2, Amount: 1},
			},
		}
		body, _ := json.Marshal(cookRequest)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}

		var response apiModel.CookResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if len(response.Errors) != 2 {
			t.Errorf("Errors = %v, want 2 entries", response.Errors)
		}
		if response.Consumed != 0 || response.Partial != 0 {
			t.Errorf("Consumed/Partial = %d/%d, want 0/0", response.Consumed, response.Partial)
		}
	})

	t.Run("server-side failure returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrDatabaseOperationFailed
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		cookRequest := apiModel.CookProductsAPIModel{
			Items: []apiModel.CookItemAPIModel{
				{ProductID: 1, Amount: 1},
			},
		}
		body, _ := json.Marshal(cookRequest)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		appCtx := newTestAppContext(m, 1)
		CookProducts(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestProductListQueryIDsBinding guards the comma-separated ids query format
// used by the cook workflow (fetchFreshProducts). gin only splits
// comma-joined values when collection_format:"csv" is set; without it,
// ?ids=1,2,3 fails uint parsing and the endpoint answers 400.
func TestProductListQueryIDsBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("comma-separated ids bind to the slice", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?ids=1,2,3", nil)

		var q ProductListQuery
		if err := ctx.ShouldBindQuery(&q); err != nil {
			t.Fatalf("ShouldBindQuery() error = %v", err)
		}
		if len(q.IDs) != 3 {
			t.Fatalf("IDs = %v, want 3 elements", q.IDs)
		}
		if q.IDs[0] != 1 || q.IDs[1] != 2 || q.IDs[2] != 3 {
			t.Errorf("IDs = %v, want [1 2 3]", q.IDs)
		}
	})

	t.Run("repeated ids params still bind", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?ids=7&ids=8", nil)

		var q ProductListQuery
		if err := ctx.ShouldBindQuery(&q); err != nil {
			t.Fatalf("ShouldBindQuery() error = %v", err)
		}
		if len(q.IDs) != 2 || q.IDs[0] != 7 || q.IDs[1] != 8 {
			t.Errorf("IDs = %v, want [7 8]", q.IDs)
		}
	})

	t.Run("empty ids leaves slice unset", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

		var q ProductListQuery
		if err := ctx.ShouldBindQuery(&q); err != nil {
			t.Fatalf("ShouldBindQuery() error = %v", err)
		}
		if len(q.IDs) != 0 {
			t.Errorf("IDs = %v, want empty", q.IDs)
		}
	})
}
