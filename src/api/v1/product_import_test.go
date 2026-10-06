package v1

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

// stubDatasetGetter stands in for the Open Food Facts controller so the
// barcode-only path is exercised without a network call.
type stubDatasetGetter struct {
	name  string
	err   error
	calls *int
}

func (s stubDatasetGetter) GetDataset(barcode string) (dbModel.Product, error) {
	if s.calls != nil {
		*s.calls++
	}
	if s.err != nil {
		return dbModel.Product{}, s.err
	}
	if s.name == "" {
		return dbModel.Product{}, fmt.Errorf("no entry for %s", barcode)
	}
	return dbModel.Product{ProductName: s.name}, nil
}

type importFixture struct {
	db      *gorm.DB
	userID  uint
	houseID uint
}

// newImportFixture creates a real database with one household so the location
// and duplicate paths run against the same queries production uses.
func newImportFixture(t *testing.T) *importFixture {
	t.Helper()
	db := testutil.SetupTestDB(t)
	user := authentication.User{Username: "importer", Password: "x", MailAddress: "i@x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	household := dbModel.Household{Name: "Import Household", AdminID: user.ID}
	if err := db.Create(&household).Error; err != nil {
		t.Fatalf("create household: %v", err)
	}
	user.HouseholdID = household.ID
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	return &importFixture{db: db, userID: user.ID, houseID: household.ID}
}

// importRequest posts the CSV body to ImportProducts through a real multipart
// request so ctx.FormFile exercises the same path as the browser.
func (f *importFixture) importRequest(t *testing.T, body string) (*httptest.ResponseRecorder, api.ImportProductsResponse) {
	t.Helper()
	return f.importRequestWithOFF(t, body, nil)
}

// importRequestNoCache runs the import with openfoodfacts.cacheEnabled off, the
// shipped default being true.
func (f *importFixture) importRequestNoCache(t *testing.T, body string, getter controllers.DatasetGetter) (*httptest.ResponseRecorder, api.ImportProductsResponse) {
	t.Helper()
	return f.importRequestWithConfig(t, body, getter, false)
}

func (f *importFixture) importRequestWithOFF(t *testing.T, body string, getter controllers.DatasetGetter) (*httptest.ResponseRecorder, api.ImportProductsResponse) {
	t.Helper()
	return f.importRequestWithConfig(t, body, getter, true)
}

func (f *importFixture) importRequestWithConfig(t *testing.T, body string, getter controllers.DatasetGetter, cacheEnabled bool) (*httptest.ResponseRecorder, api.ImportProductsResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile(importFormField, "import.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte(body)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	ctx, w := repomocks.SetupGinContextWithDB(f.db)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/import", &buffer)
	ctx.Request.Header.Set(util.RequestHeaderContentType, writer.FormDataContentType())
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{
		Server:        configuration.ServerConfiguration{MaxUploadSizeMB: 10},
		OpenFoodFacts: configuration.OpenFoodFactsConfiguration{CacheEnabled: cacheEnabled},
	})
	if getter != nil {
		ctx.Set("offacntrl", getter)
	}
	testutil.MockJWTClaimsWithKey(ctx, f.userID, testutil.TokenIdentityKey)
	appCtx := SetupTestAppContext(ctx, f.userID)

	ImportProducts(ctx, appCtx)

	var resp api.ImportProductsResponse
	if w.Body.Len() > 0 {
		if decodeErr := json.Unmarshal(w.Body.Bytes(), &resp); decodeErr != nil {
			t.Fatalf("decode response: %v (body=%s)", decodeErr, w.Body.String())
		}
	}
	return w, resp
}

const importHeader = "name,barcode,quantity,unit,category,storage_location,expiry_date,added_at\n"

func (f *importFixture) activeProducts(t *testing.T) []dbModel.Product {
	t.Helper()
	rows, err := database.NewProductRepository(f.db).GetUserActiveProductsFiltered(f.userID, nil, nil)
	if err != nil {
		t.Fatalf("read back products: %v", err)
	}
	return rows
}

func TestImportProducts(t *testing.T) {
	t.Run("happy path creates every valid row", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader +
			"Milk,4006381333931,2,l,Dairy,Fridge,2026-12-31,\n" +
			"Bread,4006381333948,1,pcs,Bakery,,2026-11-01,\n" +
			"Coffee,4006381333955,3,pcs,Beverages,Freezer,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 3 || resp.Failed != 0 || resp.TotalRows != 3 {
			t.Errorf("counts = imported %d / failed %d / total %d, want 3/0/3", resp.Imported, resp.Failed, resp.TotalRows)
		}
		products := f.activeProducts(t)
		if len(products) != 3 {
			t.Fatalf("stored products = %d, want 3", len(products))
		}
		if products[0].Amount != 2 || products[0].Unit != "l" || products[0].Categories != "Dairy" {
			t.Errorf("first product = %+v, want amount 2 / unit l / category Dairy", products[0])
		}
		if products[0].ExpireAt.IsZero() {
			t.Error("expiry_date was not stored")
		}
		if products[1].StorageLocationID != nil {
			t.Error("empty storage_location must leave the product unassigned")
		}
	})

	t.Run("re-importing an export rejects every row as a duplicate", func(t *testing.T) {
		f := newImportFixture(t)
		original := importHeader +
			"Milk,4006381333931,2,l,Dairy,Fridge,2026-12-31,\n" +
			"Bread,4006381333948,1,pcs,Bakery,Pantry,2026-11-01,\n"
		if w, resp := f.importRequest(t, original); w.Code != http.StatusOK || resp.Imported != 2 {
			t.Fatalf("setup import: status %d imported %d", w.Code, resp.Imported)
		}

		w, resp := f.importRequest(t, original)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 0 || resp.Failed != 2 {
			t.Errorf("counts = imported %d / failed %d, want 0/2", resp.Imported, resp.Failed)
		}
		for _, rowErr := range resp.Errors {
			if rowErr.Reason != "barcode already exists in household" {
				t.Errorf("row %d reason = %q, want the household-duplicate reason", rowErr.Row, rowErr.Reason)
			}
		}
		if products := f.activeProducts(t); len(products) != 2 {
			t.Errorf("stored products = %d, want the original 2 untouched", len(products))
		}
	})

	t.Run("archived barcodes are not duplicates", func(t *testing.T) {
		f := newImportFixture(t)
		archived := dbModel.Product{
			ProductName: "Milk", Barcode: "4006381333931",
			HouseholdID: f.houseID, UserID: f.userID, Amount: 1,
		}
		if err := f.db.Create(&archived).Error; err != nil {
			t.Fatalf("seed archived: %v", err)
		}
		if err := f.db.Delete(&archived).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}

		w, resp := f.importRequest(t, importHeader+"Milk,4006381333931,1,l,Dairy,Fridge,,\n")

		if w.Code != http.StatusOK || resp.Imported != 1 {
			t.Errorf("status %d imported %d, want 200/1 for a restorable archived barcode", w.Code, resp.Imported)
		}
	})

	t.Run("missing barcode column is rejected", func(t *testing.T) {
		f := newImportFixture(t)
		w, _ := f.importRequest(t, "name,quantity\nMilk,2\n")

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
		}
		if products := f.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})

	t.Run("bad quantity fails only its own row", func(t *testing.T) {
		f := newImportFixture(t)
		// Zero is accepted: the CSV export writes Amount verbatim and the model
		// has no minimum, so an exported 0 has to re-import rather than vanish.
		body := importHeader +
			"Zero,4006381333931,0,pcs,,,,\n" +
			"Negative,4006381333948,-1,pcs,,,,\n" +
			"Text,4006381333955,abc,pcs,,,,\n" +
			"Good,4006381333962,4,pcs,,,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 2 || resp.Failed != 2 {
			t.Errorf("counts = imported %d / failed %d, want 2/2", resp.Imported, resp.Failed)
		}
		for _, rowErr := range resp.Errors {
			if rowErr.Reason == "" {
				t.Errorf("row %d was rejected without a reason", rowErr.Row)
			}
		}
		products := f.activeProducts(t)
		if len(products) != 2 || products[0].ProductName != "Zero" || products[0].Amount != 0 ||
			products[1].ProductName != "Good" || products[1].Amount != 4 {
			t.Errorf("stored products = %+v, want Zero(0) and Good(4)", products)
		}
	})

	t.Run("bad expiry date fails only its own row", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader +
			"Bad,4006381333931,1,pcs,,,31-12-2026,\n" +
			"Good,4006381333948,1,pcs,,,2026-12-31,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 1 || resp.Failed != 1 {
			t.Errorf("counts = imported %d / failed %d, want 1/1", resp.Imported, resp.Failed)
		}
		if !strings.Contains(resp.Errors[0].Reason, "expiry_date") {
			t.Errorf("reason = %q, want it to name expiry_date", resp.Errors[0].Reason)
		}
	})

	t.Run("duplicate barcode within one file fails the second row", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader +
			"First,4006381333931,1,pcs,,,,\n" +
			"Second,4006381333931,1,pcs,,,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 1 || resp.Failed != 1 {
			t.Errorf("counts = imported %d / failed %d, want 1/1", resp.Imported, resp.Failed)
		}
		if resp.Errors[0].Row != 3 {
			t.Errorf("failing row = %d, want 3 (header is line 1)", resp.Errors[0].Row)
		}
	})

	t.Run("semicolon-delimited file parses", func(t *testing.T) {
		f := newImportFixture(t)
		body := "name;barcode;quantity;unit;category;storage_location;expiry_date;added_at\n" +
			"Milch;4006381333931;2;l;Milchprodukte;Kühlschrank;2026-12-31;\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK || resp.Imported != 1 {
			t.Fatalf("status %d imported %d, want 200/1; body=%s", w.Code, resp.Imported, w.Body.String())
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].ProductName != "Milch" {
			t.Errorf("stored products = %+v, want the semicolon row", products)
		}
		if len(resp.CreatedLocations) != 1 || resp.CreatedLocations[0] != "Kühlschrank" {
			t.Errorf("createdLocations = %v, want [Kühlschrank]", resp.CreatedLocations)
		}
	})

	t.Run("unknown location is created once for repeated spellings", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader +
			"Milk,4006381333931,1,l,Dairy,Cellar,,\n" +
			"Bread,4006381333948,1,pcs,Bakery,cellar,,\n" +
			"Coffee,4006381333955,1,pcs,Beverages,CELLAR,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK || resp.Imported != 3 {
			t.Fatalf("status %d imported %d, want 200/3; body=%s", w.Code, resp.Imported, w.Body.String())
		}
		if len(resp.CreatedLocations) != 1 || resp.CreatedLocations[0] != "Cellar" {
			t.Errorf("createdLocations = %v, want exactly [Cellar]", resp.CreatedLocations)
		}
		var locations []dbModel.StorageLocation
		if err := f.db.Where(util.QueryHouseholdId, f.houseID).Find(&locations).Error; err != nil {
			t.Fatalf("read locations: %v", err)
		}
		if len(locations) != 1 {
			t.Fatalf("stored locations = %d, want 1", len(locations))
		}
		for i, product := range f.activeProducts(t) {
			if product.StorageLocationID == nil || *product.StorageLocationID != locations[0].ID {
				t.Errorf("product %d storage location = %v, want %d", i, product.StorageLocationID, locations[0].ID)
			}
		}
	})

	t.Run("existing location is reused rather than duplicated", func(t *testing.T) {
		f := newImportFixture(t)
		existing := dbModel.StorageLocation{Name: "Pantry", Icon: "🗄️", HouseholdID: f.houseID}
		if err := f.db.Create(&existing).Error; err != nil {
			t.Fatalf("seed location: %v", err)
		}

		w, resp := f.importRequest(t, importHeader+"Rice,4006381333931,1,pcs,Grains,pantry,,\n")

		if w.Code != http.StatusOK || resp.Imported != 1 {
			t.Fatalf("status %d imported %d, want 200/1; body=%s", w.Code, resp.Imported, w.Body.String())
		}
		if len(resp.CreatedLocations) != 0 {
			t.Errorf("createdLocations = %v, want none", resp.CreatedLocations)
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].StorageLocationID == nil || *products[0].StorageLocationID != existing.ID {
			t.Errorf("product = %+v, want it assigned to the existing location %d", products[0], existing.ID)
		}
	})

	t.Run("barcode-only row is filled from Open Food Facts", func(t *testing.T) {
		f := newImportFixture(t)

		w, _ := f.importRequestWithOFF(t, importHeader+",4006381333931,1,pcs,,Fridge,,\n",
			stubDatasetGetter{name: "Vollmilch"})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].ProductName != "Vollmilch" {
			t.Errorf("stored products = %+v, want the Open Food Facts name", products)
		}
	})

	t.Run("barcode-only row without an Open Food Facts entry fails", func(t *testing.T) {
		f := newImportFixture(t)

		w, resp := f.importRequestWithOFF(t, importHeader+",4006381333931,1,pcs,,Fridge,,\n",
			stubDatasetGetter{err: fmt.Errorf("no entry")})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
		}
		if resp.Errors[0].Reason != fmtImportOffMiss {
			t.Errorf("reason = %q, want %q", resp.Errors[0].Reason, fmtImportOffMiss)
		}
	})

	t.Run("cached Open Food Facts name is used without a live lookup", func(t *testing.T) {
		f := newImportFixture(t)
		entry := dbModel.OpenFoodFactsCache{Barcode: "4006381333931", ProductName: "Cached Milk"}
		if err := f.db.Create(&entry).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		calls := 0
		w, _ := f.importRequestWithOFF(t, importHeader+",4006381333931,1,pcs,,Fridge,,\n",
			stubDatasetGetter{name: "Live Milk", calls: &calls})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if calls != 0 {
			t.Errorf("live lookups = %d, want 0 for a cached barcode", calls)
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].ProductName != "Cached Milk" {
			t.Errorf("stored products = %+v, want the cached name", products)
		}
	})

	t.Run("the cache is ignored when cacheEnabled is off", func(t *testing.T) {
		f := newImportFixture(t)
		entry := dbModel.OpenFoodFactsCache{Barcode: "4006381333931", ProductName: "Cached Milk"}
		if err := f.db.Create(&entry).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		calls := 0
		w, _ := f.importRequestNoCache(t, importHeader+",4006381333931,1,pcs,,Fridge,,\n",
			stubDatasetGetter{name: "Live Milk", calls: &calls})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		// Every other reader and writer of open_food_facts_caches is gated on
		// this flag, so a disabled cache must not name rows off stale rows.
		if calls != 1 {
			t.Errorf("live lookups = %d, want 1 while the cache is disabled", calls)
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].ProductName != "Live Milk" {
			t.Errorf("stored products = %+v, want the live name", products)
		}
	})

	t.Run("live Open Food Facts lookups are capped per import", func(t *testing.T) {
		f := newImportFixture(t)

		var builder strings.Builder
		builder.WriteString(importHeader)
		overshoot := util.CsvImportOpenFoodFactsLookups + 5
		for i := 0; i < overshoot; i++ {
			fmt.Fprintf(&builder, ",4006381333%03d,1,pcs,,Fridge,,\n", i)
		}

		calls := 0
		w, resp := f.importRequestWithOFF(t, builder.String(), stubDatasetGetter{name: "Milk", calls: &calls})

		if calls != util.CsvImportOpenFoodFactsLookups {
			t.Errorf("live lookups = %d, want the %d lookup budget", calls, util.CsvImportOpenFoodFactsLookups)
		}
		if resp.Imported != util.CsvImportOpenFoodFactsLookups {
			t.Errorf("imported = %d, want %d", resp.Imported, util.CsvImportOpenFoodFactsLookups)
		}
		if resp.Failed != overshoot-util.CsvImportOpenFoodFactsLookups {
			t.Errorf("failed = %d, want %d", resp.Failed, overshoot-util.CsvImportOpenFoodFactsLookups)
		}
		for _, rowErr := range resp.Errors {
			if rowErr.Reason != fmtImportOffBudget {
				t.Errorf("row %d reason = %q, want the budget reason", rowErr.Row, rowErr.Reason)
			}
		}
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 for the partially imported file", w.Code)
		}
	})

	t.Run("column aliases from the issue are accepted", func(t *testing.T) {
		f := newImportFixture(t)
		body := "product_name,barcode,amount,categories\n" +
			"Hafermilch,4006381333931,2,en:beverages\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK || resp.Imported != 1 {
			t.Fatalf("status %d imported %d, want 200/1; body=%s", w.Code, resp.Imported, w.Body.String())
		}
		products := f.activeProducts(t)
		if len(products) != 1 {
			t.Fatalf("stored products = %d, want 1", len(products))
		}
		if products[0].Amount != 2 || products[0].Categories != "en:beverages" {
			t.Errorf("product = %+v, want amount 2 from the amount alias", products[0])
		}
	})

	t.Run("blank rows are skipped and the row cap rejects the whole file", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader + ",,,,,\n" + "Milk,4006381333931,1,l,,,,\n"
		if w, resp := f.importRequest(t, body); w.Code != http.StatusOK || resp.TotalRows != 1 {
			t.Errorf("blank row: status %d totalRows %d, want 200/1", w.Code, resp.TotalRows)
		}

		oversized := newImportFixture(t)
		var builder strings.Builder
		builder.WriteString(importHeader)
		for i := 0; i <= util.CsvImportMaxRows; i++ {
			fmt.Fprintf(&builder, "Product %d,40063813339%02d,1,pcs,,,,\n", i, i%100)
		}
		w, _ := oversized.importRequest(t, builder.String())

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for %d rows; body=%s", w.Code, util.CsvImportMaxRows, w.Body.String())
		}
		if products := oversized.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})

	t.Run("empty file is rejected", func(t *testing.T) {
		f := newImportFixture(t)
		if w, _ := f.importRequest(t, ""); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
		if w, _ := f.importRequest(t, importHeader); w.Code != http.StatusBadRequest {
			t.Errorf("header-only status = %d, want 400", w.Code)
		}
	})

	t.Run("missing file part is rejected", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		ctx, w := repomocks.SetupGinContextWithDB(testutil.SetupTestDB(t))
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/import", nil)
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{
			Server: configuration.ServerConfiguration{MaxUploadSizeMB: 10},
		})

		ImportProducts(ctx, SetupTestAppContext(ctx, 1))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("one aggregate activity entry is written", func(t *testing.T) {
		f := newImportFixture(t)
		if w, resp := f.importRequest(t, importHeader+
			"Milk,4006381333931,1,l,Dairy,Fridge,,\n"+
			"Bread,4006381333948,1,pcs,Bakery,Fridge,,\n"); w.Code != http.StatusOK || resp.Imported != 2 {
			t.Fatalf("setup import: status %d imported %d", w.Code, resp.Imported)
		}

		// The activity write happens in a goroutine; the test DB is torn down
		// when the subtest ends, so the wait has to be bounded.
		var entries []dbModel.ActivityLog
		deadline := time.Now().Add(3 * time.Second)
		for {
			entries = nil
			if err := f.db.Find(&entries).Error; err != nil {
				t.Fatalf("read activity log: %v", err)
			}
			if len(entries) > 0 || time.Now().After(deadline) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}

		if len(entries) != 1 {
			t.Fatalf("activity entries = %d, want exactly 1 aggregate entry", len(entries))
		}
		if entries[0].Action != dbModel.ActivityActionImport {
			t.Errorf("action = %q, want %q", entries[0].Action, dbModel.ActivityActionImport)
		}
		if entries[0].Quantity != 2 {
			t.Errorf("quantity = %d, want 2", entries[0].Quantity)
		}
	})
	t.Run("a rejected row does not consume its barcode", func(t *testing.T) {
		f := newImportFixture(t)
		// The first row is rejected on a negative quantity, so the second row
		// carrying the same barcode is the household's only Milk and must still
		// import.
		body := importHeader +
			"Milk,4006381333931,-1,pcs,,Fridge,,\n" +
			"Milk,4006381333931,2,pcs,,Fridge,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if resp.Imported != 1 || resp.Failed != 1 {
			t.Errorf("counts = imported %d / failed %d, want 1/1", resp.Imported, resp.Failed)
		}
		products := f.activeProducts(t)
		if len(products) != 1 || products[0].Amount != 2 {
			t.Errorf("stored products = %+v, want only the valid Milk", products)
		}
	})

	t.Run("a repeated unknown barcode is looked up once", func(t *testing.T) {
		f := newImportFixture(t)
		body := importHeader +
			",4006381333931,1,pcs,,Fridge,,\n" +
			",4006381333931,1,pcs,,Fridge,,\n" +
			",4006381333931,1,pcs,,Fridge,,\n"

		calls := 0
		w, resp := f.importRequestWithOFF(t, body, stubDatasetGetter{err: fmt.Errorf("no entry"), calls: &calls})

		if calls != 1 {
			t.Errorf("live lookups = %d, want 1 for three rows sharing one barcode", calls)
		}
		if w.Code != http.StatusBadRequest || resp.Imported != 0 || resp.Failed != 3 {
			t.Errorf("status %d imported %d failed %d, want 400/0/3", w.Code, resp.Imported, resp.Failed)
		}
		for _, rowErr := range resp.Errors {
			if rowErr.Reason != fmtImportOffMiss {
				t.Errorf("row %d reason = %q, want the cache-miss reason", rowErr.Row, rowErr.Reason)
			}
		}
	})

	t.Run("row numbers follow physical lines past blank ones", func(t *testing.T) {
		f := newImportFixture(t)
		// Physical lines: 1 header, 2 empty, 3 all-empty cells, 4 First,
		// 5 Second. The csv reader drops line 2 and line 3 is an all-blank
		// record, so counting records would report the failure on line 4.
		body := importHeader + "\n" + ",,,,,\n" + "First,4006381333931,1,pcs,,,,\n" + "Second,4006381333931,1,pcs,,,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if len(resp.Errors) != 1 || resp.Errors[0].Row != 5 {
			t.Fatalf("errors = %+v, want one rejection reported on physical line 5", resp.Errors)
		}
	})

	t.Run("row numbers stay correct past a buffer boundary", func(t *testing.T) {
		f := newImportFixture(t)

		// Bigger than bufio's 4 KiB buffer, so the source is read in several
		// chunks. A counter sitting under the source therefore reports the
		// newline count of whichever chunk landed last, not the row's line.
		var builder strings.Builder
		builder.WriteString(importHeader)
		const validRows = 200
		for i := range validRows {
			fmt.Fprintf(&builder, "Filler%03d,%013d,1,pcs,,,,\n", i, 5000000000000+i)
		}
		// One rejected row at the very end, on physical line validRows+2.
		builder.WriteString("Duplicate,5000000000000,1,pcs,,,,\n")

		w, resp := f.importRequest(t, builder.String())

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if len(resp.Errors) != 1 {
			t.Fatalf("errors = %+v, want exactly one rejection", resp.Errors)
		}
		if resp.Errors[0].Row != validRows+2 {
			t.Errorf("rejection reported on row %d, want %d", resp.Errors[0].Row, validRows+2)
		}
	})

	t.Run("barcode with URL characters is rejected", func(t *testing.T) {
		f := newImportFixture(t)

		w, resp := f.importRequest(t, importHeader+"Evil,../../api/v2,1,pcs,,,,\n")

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
		}
		if len(resp.Errors) != 1 || !strings.Contains(resp.Errors[0].Reason, "letters, digits") {
			t.Errorf("errors = %+v, want the barcode charset reason", resp.Errors)
		}
		if products := f.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})

	t.Run("a failed write leaves no orphaned storage locations", func(t *testing.T) {
		f := newImportFixture(t)
		// A trigger makes the product insert fail deterministically, after the
		// locations for the accepted rows would already have been created.
		if execErr := f.db.Exec(`CREATE TRIGGER reject_products BEFORE INSERT ON products
			WHEN NEW.product_name = 'boom'
			BEGIN SELECT RAISE(ABORT, 'rejected'); END`).Error; execErr != nil {
			t.Fatalf("install trigger: %v", execErr)
		}
		body := importHeader +
			"Milk,4006381333931,1,l,Dairy,Cellar,,\n" +
			"boom,4006381333948,1,pcs,,Cellar,,\n"

		w, resp := f.importRequest(t, body)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
		}
		if resp.TotalRows != resp.Imported+resp.Failed {
			t.Errorf("counts = total %d / imported %d / failed %d, want total == imported + failed",
				resp.TotalRows, resp.Imported, resp.Failed)
		}
		if len(resp.CreatedLocations) != 0 {
			t.Errorf("createdLocations = %v, want none after a failed write", resp.CreatedLocations)
		}
		var locations []dbModel.StorageLocation
		if err := f.db.Where(util.QueryHouseholdId, f.houseID).Find(&locations).Error; err != nil {
			t.Fatalf("read locations: %v", err)
		}
		if len(locations) != 0 {
			t.Errorf("stored locations = %+v, want the transaction to have rolled them back", locations)
		}
		if products := f.activeProducts(t); len(products) != 0 {
			t.Errorf("stored products = %d, want 0", len(products))
		}
	})
}

func TestImportTemplateCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newImportFixture(t)
	ctx, w := repomocks.SetupGinContextWithDB(f.db)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/import/template.csv", nil)

	ImportTemplateCSV(ctx, SetupTestAppContext(ctx, f.userID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get(util.RequestHeaderContentDisposition); !strings.Contains(got, util.CsvImportTemplateFilename) {
		t.Errorf("Content-Disposition = %q, want it to name %q", got, util.CsvImportTemplateFilename)
	}
	records, err := csv.NewReader(w.Body).ReadAll()
	if err != nil {
		t.Fatalf("template is not valid CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want a header and one example row", len(records))
	}
	want := []string{"name", "barcode", "quantity", "unit", "category", "storage_location", "expiry_date", "added_at"}
	if strings.Join(records[0], ",") != strings.Join(want, ",") {
		t.Errorf("header = %v, want %v", records[0], want)
	}
}
