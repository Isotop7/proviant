package v1

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func TestExportProductsCSV(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	product.StorageLocationID = &location.ID
	product.Amount = 3
	product.Unit = "pcs"
	product.Categories = "Dairy"
	env.DB.Save(product)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/export/products.csv", nil)

	ExportProductsCSV(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	if ct := env.W.Header().Get(util.RequestHeaderContentType); ct != mimeTypeCSV {
		t.Errorf("content-type = %q, want %q", ct, mimeTypeCSV)
	}
	records, err := csv.NewReader(strings.NewReader(env.W.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + 1 row", len(records))
	}
	if records[0][0] != "name" || records[1][0] != product.ProductName {
		t.Errorf("records = %v, want header and product row", records)
	}
	if records[1][5] != location.Name {
		t.Errorf("storage location = %q, want %q", records[1][5], location.Name)
	}
}

func TestExportProductsCSVWithDateRange(t *testing.T) {
	env := setupHandlerTest(t)
	testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/export/products.csv?from=2020-01-01&to=2030-01-01", nil)

	ExportProductsCSV(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	if !strings.Contains(env.W.Body.String(), "Test Product") {
		t.Errorf("body missing product row: %s", env.W.Body.String())
	}
}

func TestExportProductsJSON(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/export/products.json", nil)

	ExportProductsJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var products []dbModel.Product
	if err := json.Unmarshal(env.W.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(products) != 1 || products[0].ID != product.ID {
		t.Errorf("products = %+v, want one product %d", products, product.ID)
	}
}

func TestExportArchiveCSV(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	if err := env.DB.Delete(product).Error; err != nil {
		t.Fatalf("archive product: %v", err)
	}
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/export/archive.csv", nil)

	ExportArchiveCSV(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(env.W.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + 1 row", len(records))
	}
	if records[1][0] != product.ProductName {
		t.Errorf("row = %v, want product name %q", records[1], product.ProductName)
	}
	if records[1][8] == "" {
		t.Errorf("archived_at column empty: %v", records[1])
	}
}

func TestExportFullJSON(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	product.ProductName = "Fresh Milk"
	product.Categories = "Dairy"
	env.DB.Save(product)
	// The archived product is created later but soft-deleted, so the active
	// product must still win "last inserted"; give it a distinct name so the
	// assertion can actually tell the two apart.
	archived := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	archived.ProductName = "Old Milk"
	env.DB.Save(archived)
	env.DB.Delete(archived)
	env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/export/full.json", nil)

	ExportFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var resp FullExportResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Household == nil || resp.Household.Name != env.Household.Name {
		t.Errorf("household = %+v, want %q", resp.Household, env.Household.Name)
	}
	if len(resp.Members) != 1 || resp.Members[0].Username != env.User.Username {
		t.Errorf("members = %+v, want one member %q", resp.Members, env.User.Username)
	}
	if len(resp.Products.Active) != 1 || len(resp.Products.Archived) != 1 {
		t.Errorf("products = %d active / %d archived, want 1/1", len(resp.Products.Active), len(resp.Products.Archived))
	}
	if resp.Stats.TotalActive != 1 || resp.Stats.TotalArchived != 1 {
		t.Errorf("stats = %+v, want totalActive 1 / totalArchived 1", resp.Stats)
	}
	if resp.Stats.LastInsertedProduct != "Fresh Milk" {
		t.Errorf("lastInsertedProduct = %q, want %q", resp.Stats.LastInsertedProduct, "Fresh Milk")
	}
	if resp.ExportedAt == "" {
		t.Errorf("exportedAt empty")
	}
}

func TestParseDateRange(t *testing.T) {
	newCtx := func(target string) *gin.Context {
		ctx, _ := testutil.SetupGinContext(testutil.SetupTestDB(t))
		ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
		return ctx
	}

	t.Run("empty range", func(t *testing.T) {
		from, to := parseDateRange(newCtx("/x"))
		if from != nil || to != nil {
			t.Errorf("from/to = %v/%v, want nil/nil", from, to)
		}
	})

	t.Run("valid range extends to end of day", func(t *testing.T) {
		from, to := parseDateRange(newCtx("/x?from=2026-01-10&to=2026-01-20"))
		if from == nil || from.Format("2006-01-02") != "2026-01-10" {
			t.Errorf("from = %v, want 2026-01-10", from)
		}
		if to == nil || to.Format("2006-01-02") != "2026-01-20" || to.Hour() != 23 {
			t.Errorf("to = %v, want end of 2026-01-20", to)
		}
	})

	t.Run("invalid dates are ignored", func(t *testing.T) {
		from, to := parseDateRange(newCtx("/x?from=garbage&to=also-garbage"))
		if from != nil || to != nil {
			t.Errorf("from/to = %v/%v, want nil/nil", from, to)
		}
	})
}

func TestProductToExportRow(t *testing.T) {
	expireAt := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
	product := dbModel.Product{
		ProductName: "Milk",
		Barcode:     "1234567890123",
		Amount:      2,
		Unit:        "l",
		Categories:  "Dairy",
		ExpireAt:    expireAt,
	}
	row := productToExportRow(&product)
	want := []string{"Milk", "1234567890123", "2", "l", "Dairy", "", "2030-01-02", row[7]}
	if len(row) != 8 {
		t.Fatalf("row = %v, want 8 columns", row)
	}
	for i := range want {
		if row[i] != want[i] {
			t.Errorf("row[%d] = %q, want %q", i, row[i], want[i])
		}
	}

	withLocation := product
	location := dbModel.StorageLocation{Name: "Fridge"}
	withLocation.StorageLocation = &location
	row = productToExportRow(&withLocation)
	if row[5] != "Fridge" {
		t.Errorf("storage location = %q, want Fridge", row[5])
	}

	zeroProduct := dbModel.Product{ProductName: "X"}
	row = productToExportRow(&zeroProduct)
	if row[6] != "" || row[7] != "" {
		t.Errorf("zero dates should render empty: %v", row)
	}
}
