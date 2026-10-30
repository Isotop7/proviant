package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Isotop7/proviant/api"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func TestUpdateProductQuantity(t *testing.T) {
	t.Run("applies delta", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 5
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: -2})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

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
		if stored.Quantity != 3 {
			t.Errorf("amount = %d, want 3", stored.Quantity)
		}
	})

	t.Run("reaching zero deletes product", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 1
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: -1})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Errorf("product still present, count = %d", count)
		}
	})

	t.Run("a delta beyond the pack count range is refused", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 5
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: util.MaxProductQuantity + 1})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Quantity != 5 {
			t.Errorf("count = %d, want 5 (the refused delta changed nothing)", stored.Quantity)
		}
	})

	t.Run("the count never leaves the pack count range", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = util.MaxProductQuantity
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: 10})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Quantity != util.MaxProductQuantity {
			t.Errorf("count = %d, want %d (the count never passes the maximum)", stored.Quantity, util.MaxProductQuantity)
		}
	})

	t.Run("product not found returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: -1})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: -1})

		UpdateProductQuantity(env.Ctx, env.AppCtx)

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

		UpdateProductQuantity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("an empty clientMutationId never dedupes", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 5
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		empty := ""

		for call := 1; call <= 2; call++ {
			testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: 1, ClientMutationID: &empty})
			UpdateProductQuantity(env.Ctx, env.AppCtx)
			if env.W.Code != http.StatusOK {
				t.Fatalf("call %d status = %d, want 200; body = %s", call, env.W.Code, env.W.Body.String())
			}
		}

		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Quantity != 7 {
			t.Errorf("amount = %d, want 7 — an empty token stored as-is would have made the second call a replay", stored.Quantity)
		}
	})

	t.Run("a padded clientMutationId is stored trimmed", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 5
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		padded := "  mutation-1  "
		bare := "mutation-1"

		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: 1, ClientMutationID: &padded})
		UpdateProductQuantity(env.Ctx, env.AppCtx)
		if env.W.Code != http.StatusOK {
			t.Fatalf("padded call status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}

		testutil.CreateTestRequest(env.Ctx, apiModel.ProductQuantityDTO{Delta: 1, ClientMutationID: &bare})
		UpdateProductQuantity(env.Ctx, env.AppCtx)
		if env.W.Code != http.StatusOK {
			t.Fatalf("bare call status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}

		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Quantity != 6 {
			t.Errorf("amount = %d, want 6 — the padded and bare tokens are one replay token once trimmed", stored.Quantity)
		}
		var claim dbModel.ProductMutationClaim
		if err := env.DB.First(&claim, "product_id = ? AND mutation_id = ?", product.ID, bare).Error; err != nil {
			t.Errorf("claim for %q not found: %v", bare, err)
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
	var response apiModel.ProductListResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Products) != 2 {
		t.Fatalf("products = %d, want exactly the 2 requested (ids %d,%d, unrequested %d)", len(response.Products), p1.ID, p2.ID, p3.ID)
	}
	want := map[uint]string{p1.ID: "Test Product", p2.ID: "Second Product"}
	for _, product := range response.Products {
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

// A replace states the whole product: a field the request does not carry is
// cleared instead of keeping its stored value.
func TestUpdateProductReplaceClearsTheAbsentFields(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Updates(map[string]any{
		"product_name":     "Milk",
		"quantity":         3,
		"min_stock_amount": 2,
		"pack_size":        "1 l",
	}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"productName":"Oat Milk","quantity":4}`)

	UpdateProductReplace(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.ProductName != "Oat Milk" {
		t.Errorf("product name = %q, want Oat Milk", stored.ProductName)
	}
	if stored.Quantity != 4 {
		t.Errorf("count = %d, want 4", stored.Quantity)
	}
	if stored.MinStockAmount != 0 {
		t.Errorf("min stock = %d, want 0 (a field the request did not carry is cleared)", stored.MinStockAmount)
	}
	if stored.PackSize != "" {
		t.Errorf("pack size = %q, want empty (a field the request did not carry is cleared)", stored.PackSize)
	}
}

// A replace states the whole product, so a request without a pack count is
// refused: it would leave the product with no packs.
func TestUpdateProductReplaceRefusesAMissingCount(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Update("quantity", 3).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"productName":"Oat Milk"}`)

	UpdateProductReplace(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != 3 {
		t.Errorf("count = %d, want 3 (the refused replace changed nothing)", stored.Quantity)
	}
}

// A patch carries only the fields the client changed: a field the request does
// not carry keeps its stored value.
func TestUpdateProductPatchesOnlyTheSentFields(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Updates(map[string]any{
		"product_name":     "Milk",
		"quantity":         3,
		"min_stock_amount": 2,
	}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"productName":"Oat Milk"}`)

	UpdateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.ProductName != "Oat Milk" {
		t.Errorf("product name = %q, want Oat Milk", stored.ProductName)
	}
	if stored.Quantity != 3 {
		t.Errorf("count = %d, want 3 (a field the request did not carry stays)", stored.Quantity)
	}
	if stored.MinStockAmount != 2 {
		t.Errorf("min stock = %d, want 2 (a field the request did not carry stays)", stored.MinStockAmount)
	}
}

// A category that is not one of the canonical categories is user input, not a
// server fault: the patch is refused with a 400.
func TestUpdateProductRefusesAnUnknownCategory(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"categoryKey":"unicorns"}`)

	UpdateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
}

// A patch that assigns a category and states no date leaves the product
// without one, so the household's default for that category fills it.
func TestUpdateProductFillsTheHouseholdShelfLife(t *testing.T) {
	env := setupHandlerTest(t)
	seedCategoryPrices(t, env)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Updates(map[string]any{
		"expire_at":    nil,
		"categories":   "",
		"category_key": nil,
	}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}

	body := `{"defaultShelfLifeDays":5}`
	env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/categories/milk", strings.NewReader(body))
	env.Ctx.Request.Header.Set("Content-Type", "application/json")
	env.Ctx.Params = gin.Params{{Key: "categoryKey", Value: "milk"}}
	UpdateHouseholdCategory(env.Ctx, env.AppCtx)

	env.Ctx.Request = newRawJSONRequest(`{"categoryKey":"milk"}`)
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	UpdateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.ExpireAt.IsZero() {
		t.Fatal("the product has no expiry date")
	}
	if !stored.ExpireAt.After(time.Now().AddDate(0, 0, 4)) {
		t.Errorf("expire at = %v, want about five days from now", stored.ExpireAt)
	}
}

// A product always holds at least one pack, so a patch that sets the count to
// zero is refused instead of emptying the product.
func TestUpdateProductRejectsAZeroCount(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Update("quantity", 3).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"quantity":0}`)

	UpdateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != 3 {
		t.Errorf("count = %d, want 3 (the refused patch changed nothing)", stored.Quantity)
	}
}

// A create stores a count inside the range a product may hold: the count is
// optional there, so a count outside the range is stored as the nearest count
// that is.
func TestCreateProductClampsTheCount(t *testing.T) {
	env := setupHandlerTest(t)
	env.Ctx.Set("offacntrl", &MockOpenFoodFactsAPIController{
		MockGetDataset: func(barcode string) (dbModel.Product, error) {
			return dbModel.Product{}, gorm.ErrRecordNotFound
		},
	})
	env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333931","productName":"Milk","quantity":100000,"packSize":"  1   l  "}`)

	CreateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != util.MaxProductQuantity {
		t.Errorf("count = %d, want %d", stored.Quantity, util.MaxProductQuantity)
	}
	if stored.PackSize != "1 l" {
		t.Errorf("pack size = %q, want 1 l (the whitespace is collapsed)", stored.PackSize)
	}
}

// names a larger count is refused instead of stored.
func TestUpdateProductRejectsAnOversizedCount(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Model(product).Update("quantity", 3).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
	env.Ctx.Request = newRawJSONRequest(`{"quantity":100000}`)

	UpdateProduct(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored dbModel.Product
	if err := env.DB.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != 3 {
		t.Errorf("count = %d, want 3 (the refused patch changed nothing)", stored.Quantity)
	}
}

func TestUpdateProductPatchesThePackSize(t *testing.T) {
	t.Run("sets the pack size and answers with it", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if err := env.DB.Model(product).Update("pack_size", "1 l").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"packSize":"500 g"}`)

		UpdateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "500 g" {
			t.Errorf("pack size = %q, want 500 g", stored.PackSize)
		}
		var resp dbModel.ProductDTOPatch
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.PackSize != "500 g" {
			t.Errorf("answered pack size = %q, want 500 g", resp.PackSize)
		}
	})

	t.Run("keeps the stored pack size when the patch leaves it out", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if err := env.DB.Model(product).Update("pack_size", "1 l").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"productName":"Oat Milk"}`)

		UpdateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l (a field the request did not carry stays)", stored.PackSize)
		}
	})

	t.Run("clears the pack size on an empty string", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if err := env.DB.Model(product).Update("pack_size", "1 l").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"packSize":""}`)

		UpdateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "" {
			t.Errorf("pack size = %q, want an empty string", stored.PackSize)
		}
	})

	t.Run("keeps the pack size cleared when the barcode cache knows one", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if err := env.DB.Model(product).Update("barcode", "4006381333931").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		if err := env.DB.Model(product).Update("pack_size", "1 l").Error; err != nil {
			t.Fatalf("seed pack size: %v", err)
		}
		if err := env.DB.Create(&dbModel.OpenFoodFactsCache{
			Barcode: "4006381333931", ProductName: "Milk", PackSize: "1 l",
		}).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"packSize":""}`)

		UpdateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "" {
			t.Errorf("pack size = %q, want an empty string (the cache must not put it back)", stored.PackSize)
		}
	})

	t.Run("answers with the pack size it copied from the barcode cache", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		if err := env.DB.Model(product).Update("barcode", "4006381333931").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		if err := env.DB.Create(&dbModel.OpenFoodFactsCache{
			Barcode: "4006381333931", ProductName: "Milk", PackSize: "1 l",
		}).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(product.ID)}}
		env.Ctx.Request = newRawJSONRequest(`{"productName":"Oat Milk"}`)

		UpdateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", stored.PackSize)
		}
		var resp dbModel.ProductDTOPatch
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.PackSize != "1 l" {
			t.Errorf("answered pack size = %q, want 1 l", resp.PackSize)
		}
	})
}
