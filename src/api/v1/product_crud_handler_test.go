package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func TestSetExpireAt(t *testing.T) {
	t.Run("updates expiry date", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"timestamp":"2030-05-17"}`)

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		// ProductDTOExpire.ExpireAt is a database.Date whose JSON round-trip
		// is asymmetric, so decode into a plain shape here.
		var resp struct {
			ID       uint   `json:"id"`
			Barcode  string `json:"barcode"`
			ExpireAt string `json:"expireAt"`
		}
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID != product.ID || resp.Barcode != product.Barcode {
			t.Errorf("response = %+v, want product %d/%s", resp, product.ID, product.Barcode)
		}
		if !strings.HasPrefix(resp.ExpireAt, "2030-05-17") {
			t.Errorf("expireAt = %q, want 2030-05-17", resp.ExpireAt)
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.ExpireAt.Format("2006-01-02") != "2030-05-17" {
			t.Errorf("expireAt = %v, want 2030-05-17", stored.ExpireAt)
		}
	})

	t.Run("product not found returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		env.Ctx.Request = newRawJSONRequest(`{"timestamp":"2030-05-17"}`)

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("product of another household returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		otherUser := testutil.CreateTestUser(env.DB, 0)
		otherHousehold := testutil.CreateTestHousehold(env.DB, otherUser.ID)
		otherUser.HouseholdID = otherHousehold.ID
		env.DB.Save(otherUser)
		foreign := testutil.CreateTestProduct(env.DB, otherHousehold.ID, otherUser.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(foreign.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"timestamp":"2030-05-17"}`)

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		env.Ctx.Request = newRawJSONRequest(`{"timestamp":"2030-05-17"}`)

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing timestamp returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/x", bytesReaderString("{}"))
		env.Ctx.Request.Header.Set("Content-Type", "application/json")

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("malformed date returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/x", bytesReaderString(`{"timestamp":"not-a-date"}`))
		env.Ctx.Request.Header.Set("Content-Type", "application/json")

		SetExpireAt(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateProductAmount(t *testing.T) {
	t.Run("applies delta", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Amount = 5
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductAmountDTO{Delta: -2})

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp api.APIResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Amount != 3 {
			t.Errorf("amount = %d, want 3", stored.Amount)
		}
	})

	t.Run("reaching zero deletes product", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Amount = 1
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductAmountDTO{Delta: -1})

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Errorf("product still present, count = %d", count)
		}
	})

	t.Run("product not found returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductAmountDTO{Delta: -1})

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductAmountDTO{Delta: -1})

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing delta returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/x", bytesReaderString("{}"))
		env.Ctx.Request.Header.Set("Content-Type", "application/json")

		UpdateProductAmount(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestGetProductsByIDs(t *testing.T) {
	env := setupHandlerTest(t)
	p1 := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	p2 := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	p2.ProductName = "Second Product"
	env.DB.Save(p2)
	// Third product that is deliberately NOT part of the ids filter.
	p3 := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?ids="+itoa(p1.ID)+","+itoa(p2.ID), nil)

	GetProducts(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var products []dbModel.Product
	if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("products = %d, want exactly the 2 requested (ids %d,%d, unrequested %d)", len(products), p1.ID, p2.ID, p3.ID)
	}
	want := map[uint]string{p1.ID: "Test Product", p2.ID: "Second Product"}
	for _, product := range products {
		name, ok := want[product.ID]
		if !ok {
			t.Errorf("product id %d not requested (must be %d or %d, not %d)", product.ID, p1.ID, p2.ID, p3.ID)
			continue
		}
		if product.ProductName != name {
			t.Errorf("product %d name = %q, want %q", product.ID, product.ProductName, name)
		}
	}
}

func TestGetProductsInvalidQuery(t *testing.T) {
	env := setupHandlerTest(t)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=abc", nil)

	GetProducts(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", env.W.Code)
	}
}
