package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// TestRestoreFullJSONRoundTrip restores an exported document and verifies the
// replacement keeps active and archived rows apart and remaps storage
// locations by name.
func TestRestoreFullJSONRoundTrip(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	product.ProductName = "Fresh Milk"
	product.StorageLocationID = &location.ID
	if err := env.DB.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	archived := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	archived.ProductName = "Old Milk"
	if err := env.DB.Save(archived).Error; err != nil {
		t.Fatalf("save archived product: %v", err)
	}
	if err := env.DB.Delete(archived).Error; err != nil {
		t.Fatalf("archive product: %v", err)
	}

	export := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Household: &apiModel.FullExportHousehold{
			Name:      "Restored Household",
			CreatedAt: "2026-01-01",
		},
		StorageLocations: []apiModel.StorageLocationResponse{
			{Name: "Cellar", Icon: "🏺", SortOrder: 0},
			{Name: "Pantry", Icon: "🥫", SortOrder: 2},
		},
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{
				{ProductName: "Restored Active", Barcode: "1111111111111", Amount: 2, MinStockAmount: 5, StorageLocation: &apiModel.StorageLocationResponse{Name: "Cellar"}},
			},
			Archived: []apiModel.ExportProductResponse{
				{ProductName: "Restored Archived", Barcode: "2222222222222", ArchivedAt: timePointer(t, "2026-02-03T04:05:06Z")},
			},
		},
	}
	payload, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal backup: %v", err)
	}

	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var result apiModel.RestoreResult
	if err := json.Unmarshal(env.W.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.ProductsImported != 1 || result.ProductsArchived != 1 {
		t.Errorf("result = %+v, want 1 active / 1 archived", result)
	}

	var restored dbModel.Product
	if err := env.DB.Where("product_name = ?", "Restored Active").First(&restored).Error; err != nil {
		t.Fatalf("restored active product: %v", err)
	}
	if restored.Quantity != 2 {
		t.Errorf("amount = %d, want 2", restored.Quantity)
	}
	if restored.StorageLocationID == nil {
		t.Fatal("storage location not remapped")
	}
	var restoredLocation dbModel.StorageLocation
	if err := env.DB.First(&restoredLocation, *restored.StorageLocationID).Error; err != nil {
		t.Fatalf("restored location: %v", err)
	}
	if restoredLocation.Name != "Cellar" {
		t.Errorf("location = %q, want Cellar", restoredLocation.Name)
	}

	var restoredArchived dbModel.Product
	if err := env.DB.Unscoped().Where("product_name = ?", "Restored Archived").First(&restoredArchived).Error; err != nil {
		t.Fatalf("restored archived product: %v", err)
	}
	if !restoredArchived.DeletedAt.Valid {
		t.Error("archived product not soft-deleted")
	}

	var household dbModel.Household
	if err := env.DB.First(&household, env.Household.ID).Error; err != nil {
		t.Fatalf("household: %v", err)
	}
	if household.Name != "Restored Household" {
		t.Errorf("household name = %q, want Restored Household", household.Name)
	}
}

// TestRestoreFullJSONClearsProductReferences keeps restored product ids from
// leaving dangling links behind in shopping lists, recipes and scan history.
func TestRestoreFullJSONClearsProductReferences(t *testing.T) {
	env := setupHandlerTest(t)
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	shoppingItem := dbModel.ShoppingListItem{
		HouseholdID: env.Household.ID,
		ProductID:   &product.ID,
		Name:        "Milk",
	}
	if err := env.DB.Create(&shoppingItem).Error; err != nil {
		t.Fatalf("create shopping item: %v", err)
	}
	recipe := dbModel.Recipe{
		HouseholdID:  env.Household.ID,
		Name:         "Pancakes",
		Instructions: "Mix",
		CreatedBy:    env.User.ID,
	}
	if err := env.DB.Create(&recipe).Error; err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	ingredient := dbModel.RecipeIngredient{
		RecipeID:  recipe.ID,
		ProductID: &product.ID,
		Name:      "Milk",
	}
	if err := env.DB.Create(&ingredient).Error; err != nil {
		t.Fatalf("create recipe ingredient: %v", err)
	}
	scan := dbModel.ExpiryScan{
		ProductID: &product.ID,
		UserID:    env.User.ID,
	}
	if err := env.DB.Create(&scan).Error; err != nil {
		t.Fatalf("create expiry scan: %v", err)
	}

	payload := fmt.Appendf(nil, `{"version": %d, "products": {"active": [], "archived": []}}`, apiModel.BackupSchemaVersion)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}

	var shoppingItems []dbModel.ShoppingListItem
	if err := env.DB.Find(&shoppingItems).Error; err != nil {
		t.Fatalf("load shopping items: %v", err)
	}
	if len(shoppingItems) != 1 || shoppingItems[0].ProductID != nil {
		t.Errorf("shopping item product ref = %+v, want nil", shoppingItems)
	}

	var ingredients []dbModel.RecipeIngredient
	if err := env.DB.Find(&ingredients).Error; err != nil {
		t.Fatalf("load recipe ingredients: %v", err)
	}
	if len(ingredients) != 1 || ingredients[0].ProductID != nil {
		t.Errorf("recipe ingredient product ref = %+v, want nil", ingredients)
	}

	var scans []dbModel.ExpiryScan
	if err := env.DB.Find(&scans).Error; err != nil {
		t.Fatalf("load expiry scans: %v", err)
	}
	if len(scans) != 1 || scans[0].ProductID != nil {
		t.Errorf("expiry scan product ref = %+v, want nil", scans)
	}
}

// TestRestoreFullJSONRejectsInvalidOpenedLifecycle applies the same product
// bounds as create/update instead of trusting the backup file.
func TestRestoreFullJSONRejectsInvalidOpenedLifecycle(t *testing.T) {
	env := setupHandlerTest(t)
	payload := fmt.Appendf(nil, `{
		"version": %d,
		"products": {
			"active": [{"productName": "Bad", "barcode": "3333333333333", "daysAfterOpening": 400}],
			"archived": []
		}
	}`, apiModel.BackupSchemaVersion)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var count int64
	if err := env.DB.Model(&dbModel.Product{}).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 0 {
		t.Errorf("products = %d, want 0 after a rejected restore", count)
	}
}

// TestRestoreFullJSONRequiresAdminRole rechecks the destructive operation's
// authorization inside the handler, not only in the route middleware.
func TestRestoreFullJSONRequiresAdminRole(t *testing.T) {
	env := setupHandlerTest(t)
	member := env.addMember(t, "member")
	env.useMember(member)
	payload := fmt.Appendf(nil, `{"version": %d, "products": {"active": [], "archived": []}}`, apiModel.BackupSchemaVersion)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body = %s", env.W.Code, env.W.Body.String())
	}
}

// TestRestoreFullJSONRejectsInvalidBarcode holds an uploaded document to the
// same barcode bounds as the CSV import before it can reach Open Food Facts.
func TestRestoreFullJSONRejectsInvalidBarcode(t *testing.T) {
	env := setupHandlerTest(t)
	payload := fmt.Appendf(nil, `{
		"version": %d,
		"products": {
			"active": [{"productName": "Bad", "barcode": "../etc/passwd"}],
			"archived": []
		}
	}`, apiModel.BackupSchemaVersion)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var count int64
	if err := env.DB.Model(&dbModel.Product{}).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 0 {
		t.Errorf("products = %d, want 0 after a rejected restore", count)
	}
}

// TestRestoreFullJSONRejectsMissingVersion keeps an unversioned document out:
// without the schema version there is no way to know which fields it omits.
func TestRestoreFullJSONRejectsMissingVersion(t *testing.T) {
	env := setupHandlerTest(t)
	payload := []byte(`{"products": {"active": [], "archived": []}}`)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
}

// TestRestoreFullJSONRejectsUnsupportedVersion keeps a document from a future
// schema out of the database rather than partially interpreting it.
func TestRestoreFullJSONRejectsUnsupportedVersion(t *testing.T) {
	env := setupHandlerTest(t)
	payload := fmt.Appendf(nil, `{"version": %d, "products": {"active": [], "archived": []}}`, apiModel.BackupSchemaVersion+1)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(payload))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var count int64
	if err := env.DB.Model(&dbModel.Product{}).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 0 {
		t.Errorf("products = %d, want 0 after a rejected restore", count)
	}
}

// TestRestoreFullJSONRejectsMalformedDocument keeps a truncated upload from
// being interpreted as an empty restore that wipes the household.
func TestRestoreFullJSONRejectsMalformedDocument(t *testing.T) {
	env := setupHandlerTest(t)
	testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/restore/full.json", bytes.NewReader(fmt.Appendf(nil, `{"version": %d,`, apiModel.BackupSchemaVersion)))
	env.Ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

	RestoreFullJSON(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
	}
	var count int64
	if err := env.DB.Model(&dbModel.Product{}).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 1 {
		t.Errorf("products = %d, want the existing row untouched", count)
	}
}

func timePointer(t *testing.T, value string) *time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return &parsed
}
