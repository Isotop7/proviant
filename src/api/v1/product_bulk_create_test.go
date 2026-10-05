package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type bulkCreateFixture struct {
	db      *gorm.DB
	userID  uint
	houseID uint
}

// newBulkCreateFixture creates a real database with one household so the
// storage-location membership and insert paths run against the same queries
// production uses.
func newBulkCreateFixture(t *testing.T) *bulkCreateFixture {
	t.Helper()
	db := testutil.SetupTestDB(t)
	user := authentication.User{Username: "batchscanner", Password: "x", MailAddress: "b@x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	household := dbModel.Household{Name: "Batch Household", AdminID: user.ID}
	if err := db.Create(&household).Error; err != nil {
		t.Fatalf("create household: %v", err)
	}
	user.HouseholdID = household.ID
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	return &bulkCreateFixture{db: db, userID: user.ID, houseID: household.ID}
}

// bulkCreateRequest posts a JSON body to BulkCreateProducts. items may be nil
// to exercise the invalid-body path.
func (f *bulkCreateFixture) bulkCreateRequest(
	t *testing.T,
	items []api.BulkCreateProductItem,
	rawBody string,
	getter controllers.DatasetGetter,
) (*httptest.ResponseRecorder, api.BulkCreateResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var body []byte
	if rawBody != "" {
		body = []byte(rawBody)
	} else {
		encoded, encErr := json.Marshal(api.BulkCreateProductsAPIModel{Items: items})
		if encErr != nil {
			t.Fatalf("marshal request: %v", encErr)
		}
		body = encoded
	}

	ctx, w := repomocks.SetupGinContextWithDB(f.db)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/bulk", bytes.NewReader(body))
	ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{
		OpenFoodFacts: configuration.OpenFoodFactsConfiguration{CacheEnabled: true},
	})
	if getter != nil {
		ctx.Set("offacntrl", getter)
	}
	testutil.MockJWTClaimsWithKey(ctx, f.userID, testutil.TokenIdentityKey)
	appCtx := SetupTestAppContext(ctx, f.userID)

	BulkCreateProducts(ctx, appCtx)

	var resp api.BulkCreateResponse
	if w.Body.Len() > 0 {
		if decodeErr := json.Unmarshal(w.Body.Bytes(), &resp); decodeErr != nil {
			t.Fatalf("decode response: %v (body=%s)", decodeErr, w.Body.String())
		}
	}
	return w, resp
}

func (f *bulkCreateFixture) activeProducts(t *testing.T) []dbModel.Product {
	t.Helper()
	rows, err := database.NewProductRepository(f.db).GetUserActiveProductsFiltered(f.userID, nil, nil)
	if err != nil {
		t.Fatalf("read back products: %v", err)
	}
	return rows
}

func bulkItem(barcode string, amount int, expireAt string, locationID *uint) api.BulkCreateProductItem {
	expiry := time.Time{}
	if expireAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, expireAt)
		if parseErr != nil {
			panic(parseErr)
		}
		expiry = parsed
	}
	return api.BulkCreateProductItem{
		Barcode:           barcode,
		ProductName:       "",
		ExpireAt:          expiry,
		Amount:            amount,
		StorageLocationID: locationID,
	}
}

const bulkValidExpiry = "2026-12-31T00:00:00Z"

func TestBulkCreateProducts(t *testing.T) {
	t.Run("all items are created and stored with their values", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		location := dbModel.StorageLocation{Name: "Fridge", Icon: "🧊", HouseholdID: f.houseID}
		if err := f.db.Create(&location).Error; err != nil {
			t.Fatalf("seed location: %v", err)
		}

		items := []api.BulkCreateProductItem{
			bulkItem("4006381333931", 2, bulkValidExpiry, &location.ID),
			bulkItem("4006381333948", 1, bulkValidExpiry, nil),
		}
		items[0].ProductName = "Milk"
		items[1].ProductName = "Bread"

		w, resp := f.bulkCreateRequest(t, items, "", nil)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Created != 2 || resp.Failed != 0 {
			t.Errorf("counts = created %d / failed %d, want 2/0", resp.Created, resp.Failed)
		}
		if resp.Errors == nil {
			t.Error("errors must serialise as [] rather than null")
		}
		products := f.activeProducts(t)
		if len(products) != 2 {
			t.Fatalf("stored products = %d, want 2", len(products))
		}
		byBarcode := map[string]dbModel.Product{}
		for _, product := range products {
			byBarcode[product.Barcode] = product
		}
		milk, milkOk := byBarcode["4006381333931"]
		bread, breadOk := byBarcode["4006381333948"]
		if !milkOk || !breadOk {
			t.Fatalf("stored barcodes = %v, want both scanned barcodes", byBarcode)
		}
		if milk.ProductName != "Milk" || milk.Amount != 2 {
			t.Errorf("milk = %+v, want Milk with amount 2", milk)
		}
		if milk.StorageLocationID == nil || *milk.StorageLocationID != location.ID {
			t.Errorf("milk storage location = %v, want %d", milk.StorageLocationID, location.ID)
		}
		if milk.HouseholdID != f.houseID {
			t.Errorf("milk household = %d, want %d", milk.HouseholdID, f.houseID)
		}
		if milk.ExpireAt.IsZero() {
			t.Error("expiry was not stored")
		}
		if bread.ProductName != "Bread" || bread.Amount != 1 {
			t.Errorf("bread = %+v, want Bread with amount 1", bread)
		}
		if bread.StorageLocationID != nil {
			t.Errorf("bread storage location = %v, want none", bread.StorageLocationID)
		}
	})

	t.Run("partial failure rejects only its own items", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		items := []api.BulkCreateProductItem{
			bulkItem("../../api/v2", 1, bulkValidExpiry, nil),  // bad charset
			bulkItem("4006381333948", 0, bulkValidExpiry, nil), // amount 0
			bulkItem("4006381333955", 1, "", nil),              // no expiry
			bulkItem("4006381333962", 3, bulkValidExpiry, nil),
		}
		items[3].ProductName = "Good"

		w, resp := f.bulkCreateRequest(t, items, "", nil)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Created != 1 || resp.Failed != 3 {
			t.Errorf("counts = created %d / failed %d, want 1/3", resp.Created, resp.Failed)
		}
		if resp.Created+resp.Failed != len(items) {
			t.Errorf("created + failed = %d, want %d", resp.Created+resp.Failed, len(items))
		}
		if len(resp.Errors) != 3 {
			t.Fatalf("errors = %+v, want 3 rejections", resp.Errors)
		}
		for _, itemErr := range resp.Errors {
			if itemErr.Reason == "" {
				t.Errorf("item %d was rejected without a reason", itemErr.Index)
			}
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].ProductName != "Good" || products[0].Amount != 3 {
			t.Errorf("stored products = %+v, want only Good(3)", products)
		}
	})

	t.Run("invalid JSON body is rejected", func(t *testing.T) {
		f := newBulkCreateFixture(t)

		w, _ := f.bulkCreateRequest(t, nil, "not json", nil)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		if products := f.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})

	t.Run("empty items are rejected", func(t *testing.T) {
		f := newBulkCreateFixture(t)

		w, _ := f.bulkCreateRequest(t, []api.BulkCreateProductItem{}, "", nil)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("more than the item cap is rejected", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		items := make([]api.BulkCreateProductItem, util.BulkCreateMaxItems+1)
		for i := range items {
			items[i] = bulkItem(fmt.Sprintf("4006381333%03d", i%1000), 1, bulkValidExpiry, nil)
			items[i].ProductName = "Filler"
		}

		w, _ := f.bulkCreateRequest(t, items, "", nil)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for %d items", w.Code, len(items))
		}
		if products := f.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})

	t.Run("a storage location from another household is rejected", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		foreignUser := authentication.User{Username: "outsider", Password: "x", MailAddress: "o@x"}
		if err := f.db.Create(&foreignUser).Error; err != nil {
			t.Fatalf("create foreign user: %v", err)
		}
		foreignHousehold := dbModel.Household{Name: "Foreign Household", AdminID: foreignUser.ID}
		if err := f.db.Create(&foreignHousehold).Error; err != nil {
			t.Fatalf("create foreign household: %v", err)
		}
		foreignLocation := dbModel.StorageLocation{Name: "Their Fridge", Icon: "🧊", HouseholdID: foreignHousehold.ID}
		if err := f.db.Create(&foreignLocation).Error; err != nil {
			t.Fatalf("seed foreign location: %v", err)
		}

		items := []api.BulkCreateProductItem{
			bulkItem("4006381333931", 1, bulkValidExpiry, &foreignLocation.ID),
			bulkItem("4006381333948", 1, bulkValidExpiry, nil),
		}
		items[0].ProductName = "Milk"
		items[1].ProductName = "Bread"

		w, resp := f.bulkCreateRequest(t, items, "", nil)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Created != 1 || resp.Failed != 1 {
			t.Errorf("counts = created %d / failed %d, want 1/1", resp.Created, resp.Failed)
		}
		if len(resp.Errors) != 1 || resp.Errors[0].Index != 0 {
			t.Fatalf("errors = %+v, want one rejection on index 0", resp.Errors)
		}
	})

	t.Run("Open Food Facts miss creates the item with an empty name", func(t *testing.T) {
		f := newBulkCreateFixture(t)

		items := []api.BulkCreateProductItem{bulkItem("4006381333931", 1, bulkValidExpiry, nil)}

		w, resp := f.bulkCreateRequest(t, items, "", stubDatasetGetter{err: fmt.Errorf("no entry")})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Created != 1 || resp.Failed != 0 {
			t.Errorf("counts = created %d / failed %d, want 1/0", resp.Created, resp.Failed)
		}
		products := f.activeProducts(t)
		if len(products) != 1 {
			t.Fatalf("stored products = %d, want 1", len(products))
		}
		if products[0].ProductName != "" || products[0].Barcode != "4006381333931" {
			t.Errorf("stored product = %+v, want an unnamed product with the scanned barcode", products[0])
		}
	})

	t.Run("a missing name is resolved live from Open Food Facts", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		entry := dbModel.OpenFoodFactsCache{Barcode: "4006381333931", ProductName: "Cached Milk"}
		if err := f.db.Create(&entry).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		items := []api.BulkCreateProductItem{
			bulkItem("4006381333931", 1, bulkValidExpiry, nil),
			bulkItem("4006381333948", 1, bulkValidExpiry, nil),
		}

		calls := 0
		w, resp := f.bulkCreateRequest(t, items, "", stubDatasetGetter{name: "Live Bread", calls: &calls})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Created != 2 {
			t.Errorf("created = %d, want 2", resp.Created)
		}
		if calls != 1 {
			t.Errorf("live lookups = %d, want 1 (the other barcode came from the cache)", calls)
		}
		names := map[string]bool{}
		for _, product := range f.activeProducts(t) {
			names[product.ProductName] = true
		}
		if !names["Cached Milk"] || !names["Live Bread"] {
			t.Errorf("stored names = %v, want Cached Milk and Live Bread", names)
		}
	})

	t.Run("a client-provided name wins and skips the resolver", func(t *testing.T) {
		f := newBulkCreateFixture(t)

		items := []api.BulkCreateProductItem{bulkItem("4006381333931", 1, bulkValidExpiry, nil)}
		items[0].ProductName = "Hand-labelled Jar"

		calls := 0
		w, _ := f.bulkCreateRequest(t, items, "", stubDatasetGetter{name: "OFF Name", calls: &calls})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if calls != 0 {
			t.Errorf("live lookups = %d, want 0 when the client names the product", calls)
		}
		if products := f.activeProducts(t); len(products) != 1 || products[0].ProductName != "Hand-labelled Jar" {
			t.Errorf("stored products = %+v, want the client-provided name", products)
		}
	})

	t.Run("duplicates of an existing household barcode are allowed", func(t *testing.T) {
		f := newBulkCreateFixture(t)
		existing := dbModel.Product{
			ProductName: "Milk", Barcode: "4006381333931",
			HouseholdID: f.houseID, UserID: f.userID, Amount: 1,
		}
		if err := f.db.Create(&existing).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}

		items := []api.BulkCreateProductItem{bulkItem("4006381333931", 2, bulkValidExpiry, nil)}
		items[0].ProductName = "Milk"

		w, resp := f.bulkCreateRequest(t, items, "", nil)

		if w.Code != http.StatusOK || resp.Created != 1 {
			t.Fatalf("status %d created %d, want 200/1; body=%s", w.Code, resp.Created, w.Body.String())
		}
		if products := f.activeProducts(t); len(products) != 2 {
			t.Errorf("stored products = %d, want 2 (the original plus the scanned one)", len(products))
		}
	})
}
