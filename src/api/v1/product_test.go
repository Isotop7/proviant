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

	t.Run("cross-household product is 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrMismatcherUserID
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "2"}}

		appCtx := newTestAppContext(m, 1)
		GetProduct(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	// A DB failure must not hide behind 404: monitoring only sees the status.
	t.Run("server-side failure is 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInternalServer
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

		appCtx := newTestAppContext(m, 1)
		GetProduct(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
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

	// Validation failures on client-supplied OpenedAt/DaysAfterOpening are 400s;
	// the handler used to report every create failure as 500.
	t.Run("opened lifecycle validation failure is 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInvalidOpenedLifecycle
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		mockOpenFoodFactsAPIController := &MockOpenFoodFactsAPIController{
			MockGetDataset: func(barcode string) (dbModel.Product, error) {
				return dbModel.Product{}, errors.ErrProductSearchInvalidQuery
			},
		}
		ctx.Set("offacntrl", mockOpenFoodFactsAPIController)

		// Unknown barcode keeps the client-bound product (with its opened
		// fields) instead of replacing it with the OFF dataset row.
		productData := dbModel.Product{
			ProductName:      "Bad Shelf Life",
			Barcode:          "0000000000000",
			DaysAfterOpening: intPtr(9999),
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

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func intPtr(v int) *int { return &v }

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

	// A rejected OpenedAt/DaysAfterOpening is client input (400), and a
	// product outside the caller's household is not found (404) — neither is
	// a server fault, which is what the default branch used to report.
	t.Run("opened lifecycle validation failure is 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInvalidOpenedLifecycle
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

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("cross-household update is 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrMismatcherUserID
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

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusNotFound, w.Body.String())
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

	t.Run("sort outside the allowlist is rejected", func(t *testing.T) {
		// Payloads are chosen without '&' or ';', which Go's URL parser discards
		// before the query ever reaches the binder.
		for _, sortValue := range []string{"evil", "category", "storage_location", "created_at,sqlite_version()"} {
			m := repomocks.NewMockRepositoryContainer()
			ctx, w := repomocks.SetupGinContextWithMocks(m)
			testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
			ctx.Request = &http.Request{Header: make(http.Header)}
			ctx.Request.URL = &url.URL{RawQuery: "queryParam=product_name&queryValue=Apple&sort=" + sortValue}

			appCtx := &AppContext{
				Logger: &zerolog.Logger{},
				Repos:  m.ToRepositoryContainer(),
				UserID: 1,
			}
			SearchProducts(ctx, appCtx)

			if w.Code != http.StatusBadRequest {
				t.Errorf("sort=%q: Status = %v, want %v", sortValue, w.Code, http.StatusBadRequest)
			}
		}
	})

	t.Run("allowlisted sort is accepted", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Products = []dbModel.Product{{ProductName: "Apple Juice"}}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = &http.Request{Header: make(http.Header)}
		ctx.Request.URL = &url.URL{RawQuery: "queryParam=product_name&queryValue=Apple&sort=expire_at&order=desc"}

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

	// Client-side causes reaching the repo (sort allowlist drift, own account
	// row gone) are 400s; only real server faults may hide behind 500.
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"repo sort allowlist miss is 400", errors.ErrDatabaseInvalidSortParameter, http.StatusBadRequest},
		{"missing user row is 400", gorm.ErrRecordNotFound, http.StatusBadRequest},
		{"repository failure is 500", errors.ErrInternalServer, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := repomocks.NewMockRepositoryContainer()
			m.Products.Err = tc.err
			ctx, w := repomocks.SetupGinContextWithMocks(m)
			testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
			ctx.Request = &http.Request{Header: make(http.Header)}
			ctx.Request.URL = &url.URL{RawQuery: "queryParam=product_name&queryValue=Apple&sort=expire_at"}

			appCtx := &AppContext{
				Logger: &zerolog.Logger{},
				Repos:  m.ToRepositoryContainer(),
				UserID: 1,
			}
			SearchProducts(ctx, appCtx)

			if w.Code != tc.want {
				t.Errorf("Status = %v, want %v; body: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
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

	// An unusable account (no household, user row gone) is the caller's state
	// and stays 400; an unknown barcode never errors at all — it is an empty 200.
	t.Run("user without a household returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInvalidUserData
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

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("repository failure returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInternalServer
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

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
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

// TestProductActionAccessErrorsAre404 covers the shared not-found/cross-
// tenant mapping across the destructive product endpoints. Both conditions
// used to fall through to 500, reporting a caller's bad id as a server fault
// (and, for foreign ids, probing the existence of other households' rows
// through the status code difference).
func TestProductActionAccessErrorsAre404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		handler func(*gin.Context, *AppContext)
		err     error
	}{
		{"delete not found", DeleteProduct, gorm.ErrRecordNotFound},
		{"delete cross-tenant", DeleteProduct, errors.ErrMismatcherUserID},
		{"restore not found", RestoreProduct, gorm.ErrRecordNotFound},
		{"restore cross-tenant", RestoreProduct, errors.ErrMismatcherUserID},
		{"consume not found", ConsumeProduct, gorm.ErrRecordNotFound},
		{"consume cross-tenant", ConsumeProduct, errors.ErrMismatcherUserID},
		{"waste not found", WasteProduct, gorm.ErrRecordNotFound},
		{"waste cross-tenant", WasteProduct, errors.ErrMismatcherUserID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repomocks.NewMockRepositoryContainer()
			m.Products.Err = tc.err
			ctx, w := repomocks.SetupGinContextWithMocks(m)
			testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

			ctx.Request = &http.Request{Header: make(http.Header), URL: &url.URL{}}
			ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

			tc.handler(ctx, newTestAppContext(m, 1))

			if w.Code != http.StatusNotFound {
				t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusNotFound, w.Body.String())
			}
		})
	}
}

// TestUpdateProductAmountAccessErrorIs404 covers the amount endpoint's
// pre-fetch: a cross-tenant id must read as not-found, not as 500.
func TestUpdateProductAmountAccessErrorIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := repomocks.NewMockRepositoryContainer()
	m.Products.Err = errors.ErrMismatcherUserID
	ctx, w := repomocks.SetupGinContextWithMocks(m)
	testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
	ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/amount", apiModel.ProductAmountDTO{Delta: -1})
	ctx.Params = []gin.Param{{Key: "id", Value: "1"}}

	UpdateProductAmount(ctx, newTestAppContext(m, 1))

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}
