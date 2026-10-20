package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Isotop7/proviant/controllers/database"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// newBulkTestSetup creates a user + household + storage location and wires a
// gin context for the bulk create handler. Returns the location ID too.
func newBulkTestSetup(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *AppContext, uint) {
	db := testutil.SetupTestDB(t)
	// The handler spawns a side-effect worker goroutine per batch; wait for it
	// to finish before the temp DB dir is removed underneath it. Registered
	// after SetupTestDB, so it runs before t.TempDir cleanup.
	t.Cleanup(func() {
		if !waitBulkSideEffectsIdle(5 * time.Second) {
			t.Errorf("bulk side-effect worker did not finish within timeout")
		}
	})
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	db.Save(user)
	location := testutil.CreateTestStorageLocation(db, household.ID)

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
	appCtx := SetupTestAppContext(ctx, user.ID)
	return ctx, w, appCtx, location.ID
}

func TestBulkCreateProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("creates all valid items", func(t *testing.T) {
		ctx, w, appCtx, locationID := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "Milk", Quantity: 2, PackSize: "1 l", ExpireAt: "2030-01-02", Price: pricePtr(2.49)},
				{ProductName: "Bread", Quantity: 1, StorageLocationID: &locationID},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.BulkCreateResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Results) != 2 {
			t.Fatalf("results = %d, want 2", len(resp.Results))
		}
		for _, r := range resp.Results {
			if r.Status != apiModel.BulkItemStatusCreated {
				t.Errorf("item %d status = %q (%s), want created", r.Index, r.Status, r.Reason)
			}
		}
		var count int64
		appCtx.DB.Model(&dbModel.Product{}).Count(&count)
		if count != 2 {
			t.Errorf("product count = %d, want 2", count)
		}
	})

	// The receipt prints the price of the whole line; the product carries the
	// price of one pack.
	t.Run("the line price becomes the price of one pack", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "Milk", Quantity: 2, Price: pricePtr(2.49)},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var stored dbModel.Product
		if err := appCtx.DB.First(&stored, "product_name = ?", "Milk").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if stored.PriceOverride == nil || *stored.PriceOverride < 1.244 || *stored.PriceOverride > 1.246 {
			t.Errorf("price = %v, want 1.245 (the line price of 2.49 for two packs)", stored.PriceOverride)
		}
	})

	t.Run("the deprecated per-pack override stays the price of one pack", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "Milk", Quantity: 3, PriceOverride: pricePtr(1.50)},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var stored dbModel.Product
		if err := appCtx.DB.First(&stored, "product_name = ?", "Milk").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if stored.PriceOverride == nil || *stored.PriceOverride != 1.50 {
			t.Errorf("price = %v, want 1.50 (the override was already a per-pack price)", stored.PriceOverride)
		}
	})

	t.Run("a draft written before packs are counted becomes one pack of that size", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "Milk", Amount: 500, Unit: "g"},
				{ProductName: "Eggs", Amount: 6, Unit: "pcs"},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var milk dbModel.Product
		if err := appCtx.DB.First(&milk, "product_name = ?", "Milk").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if milk.Quantity != 1 || milk.PackSize != "500 g" {
			t.Errorf("milk = %d packs of %q, want 1 pack of 500 g", milk.Quantity, milk.PackSize)
		}
		var eggs dbModel.Product
		if err := appCtx.DB.First(&eggs, "product_name = ?", "Eggs").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if eggs.Quantity != 6 || eggs.PackSize != "" {
			t.Errorf("eggs = %d packs of %q, want 6 packs", eggs.Quantity, eggs.PackSize)
		}
	})

	t.Run("partial success: invalid item does not block valid ones", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "", Quantity: 1},
				{ProductName: "Milk", Quantity: 1, ExpireAt: "not-a-date"},
				{ProductName: "Bread", Quantity: 1},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.BulkCreateResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp.Results) != 3 {
			t.Fatalf("results = %d, want 3", len(resp.Results))
		}
		if resp.Results[0].Status != apiModel.BulkItemStatusFailed || resp.Results[0].Reason == "" {
			t.Errorf("item 0 = %+v, want failed with reason", resp.Results[0])
		}
		if resp.Results[1].Status != apiModel.BulkItemStatusFailed {
			t.Errorf("item 1 = %+v, want failed (invalid expiry)", resp.Results[1])
		}
		if resp.Results[2].Status != apiModel.BulkItemStatusCreated || resp.Results[2].ProductID == nil {
			t.Errorf("item 2 = %+v, want created", resp.Results[2])
		}
	})

	t.Run("unknown storage location fails that item only", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		unknown := uint(9999)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{{ProductName: "Milk", StorageLocationID: &unknown}},
		})

		BulkCreateProducts(ctx, appCtx)

		var resp apiModel.BulkCreateResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if w.Code != http.StatusOK || resp.Results[0].Status != apiModel.BulkItemStatusFailed {
			t.Fatalf("status = %d, results = %+v; want 200 with failed item", w.Code, resp.Results)
		}
	})

	t.Run("out of range prices fail that item only", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		negative := -1.5
		tooHigh := util.ReceiptItemMaxPrice + 1
		valid := 4.99
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{ProductName: "Milk", Price: &negative},
				{ProductName: "Caviar", Price: &tooHigh},
				{ProductName: "Bread", Price: &valid},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.BulkCreateResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Results[0].Status != apiModel.BulkItemStatusFailed || resp.Results[0].Reason == "" {
			t.Errorf("item 0 = %+v, want failed (negative price)", resp.Results[0])
		}
		if resp.Results[1].Status != apiModel.BulkItemStatusFailed || resp.Results[1].Reason == "" {
			t.Errorf("item 1 = %+v, want failed (price above cap)", resp.Results[1])
		}
		if resp.Results[2].Status != apiModel.BulkItemStatusCreated {
			t.Errorf("item 2 = %+v, want created", resp.Results[2])
		}
		var stored dbModel.Product
		if err := appCtx.DB.First(&stored, "product_name = ?", "Bread").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if stored.PriceOverride == nil || *stored.PriceOverride != valid {
			t.Errorf("priceOverride = %v, want %v", stored.PriceOverride, valid)
		}
		var failedCount int64
		appCtx.DB.Model(&dbModel.Product{}).Where("product_name IN ?", []string{"Milk", "Caviar"}).Count(&failedCount)
		if failedCount != 0 {
			t.Errorf("rejected items were stored: %d rows", failedCount)
		}
	})

	t.Run("empty items list is rejected", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{Items: []apiModel.BulkProductDraft{}})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("too many items are rejected", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		items := make([]apiModel.BulkProductDraft, util.ReceiptBulkMaxItems+1)
		for i := range items {
			items[i] = apiModel.BulkProductDraft{ProductName: "X"}
		}
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{Items: items})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		testutil.CreateTestRequest(ctx, "not-an-object")

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("oversized barcode, pack size and categories are clamped", func(t *testing.T) {
		ctx, w, appCtx, _ := newBulkTestSetup(t)
		longPackSize := strings.Repeat("u", util.MaxPackSizeLength+10)
		longCategories := strings.Repeat("c", util.ReceiptItemMaxCategoriesLength+10)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{{
				ProductName: "Milk",
				Barcode:     strings.Repeat("9", 40),
				PackSize:    longPackSize,
				Categories:  longCategories,
			}},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var stored dbModel.Product
		if err := appCtx.DB.First(&stored, "product_name = ?", "Milk").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if len(stored.Barcode) != util.CsvImportMaxBarcodeLength {
			t.Errorf("barcode length = %d, want %d", len(stored.Barcode), util.CsvImportMaxBarcodeLength)
		}
		if stored.PackSize != longPackSize[:util.MaxPackSizeLength] {
			t.Errorf("pack size = %q, want clamped to %d bytes", stored.PackSize, util.MaxPackSizeLength)
		}
		if stored.Categories != longCategories[:util.ReceiptItemMaxCategoriesLength] {
			t.Errorf("categories = %q, want clamped to %d bytes", stored.Categories, util.ReceiptItemMaxCategoriesLength)
		}
	})

	t.Run("multibyte strings are clamped without splitting a rune", func(t *testing.T) {
		ctx, _, appCtx, _ := newBulkTestSetup(t)
		// "ä" is 2 UTF-8 bytes; a byte-boundary cut must not split it.
		multibyte := strings.Repeat("ä", 15)
		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{{ProductName: "Milk", PackSize: multibyte}},
		})

		BulkCreateProducts(ctx, appCtx)

		var stored dbModel.Product
		if err := appCtx.DB.First(&stored, "product_name = ?", "Milk").Error; err != nil {
			t.Fatalf("stored product: %v", err)
		}
		if !utf8.ValidString(stored.PackSize) {
			t.Errorf("pack size = %q is not valid UTF-8", stored.PackSize)
		}
		if len(stored.PackSize) > util.MaxPackSizeLength {
			t.Errorf("pack size length = %d, want <= %d", len(stored.PackSize), util.MaxPackSizeLength)
		}
	})
}

func pricePtr(v float64) *float64 { return &v }
