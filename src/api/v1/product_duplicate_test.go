package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// duplicateExpiry is the expiry the duplicate cases share. It is a UTC
// midnight, the way the client sends the day it picked.
var duplicateExpiry = time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)

// offaMock answers every barcode lookup with the product's identity, the way
// a successful OpenFoodFacts request does.
func offaMock() *MockOpenFoodFactsAPIController {
	return &MockOpenFoodFactsAPIController{
		MockGetDataset: func(barcode string) (dbModel.Product, error) {
			return dbModel.Product{
				Barcode:     barcode,
				ProductName: "Yogurt",
				Categories:  "dairies",
			}, nil
		},
	}
}

// postCreate sends a create request for the given body and returns the
// recorder.
func postCreate(t *testing.T, env *handlerTestEnv, body dbModel.Product) *httptest.ResponseRecorder {
	t.Helper()
	raw, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("marshal body: %v", marshalErr)
	}
	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = newRawJSONRequest(string(raw))
	ctx.Set("offacntrl", offaMock())
	CreateProduct(ctx, env.AppCtx)
	return w
}

// countProducts reports how many active rows carry the given barcode.
func countProducts(t *testing.T, db *gorm.DB, barcode string) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&dbModel.Product{}).Where("barcode = ?", barcode).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	return count
}

// The create path refuses a duplicate and hands the matched row back, so the
// client can ask whether the pack belongs to that row or is a product of its
// own.
func TestCreateProductRefusesADuplicate(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	existing := dbModel.Product{
		Barcode:           "4006381333931",
		ProductName:       "Yogurt",
		HouseholdID:       env.Household.ID,
		UserID:            env.User.ID,
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		Quantity:          2,
	}
	if err := env.DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing product: %v", err)
	}

	w := postCreate(t, env, dbModel.Product{
		Barcode:           "4006381333931",
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		Quantity:          1,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", w.Code, w.Body.String())
	}

	var body struct {
		Message   string `json:"message"`
		Duplicate *struct {
			ID          uint   `json:"id"`
			ProductName string `json:"productName"`
			Quantity    int    `json:"quantity"`
		} `json:"duplicate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != errors.ErrDuplicateProduct.Error() {
		t.Errorf("message = %q, want %q", body.Message, errors.ErrDuplicateProduct.Error())
	}
	if body.Duplicate == nil {
		t.Fatal("duplicate is missing from the refusal body")
	}
	if body.Duplicate.ID != existing.ID {
		t.Errorf("duplicate.id = %d, want %d", body.Duplicate.ID, existing.ID)
	}
	if body.Duplicate.Quantity != 2 {
		t.Errorf("duplicate.quantity = %d, want 2", body.Duplicate.Quantity)
	}
	if count := countProducts(t, env.DB, "4006381333931"); count != 1 {
		t.Errorf("rows with the barcode = %d, want 1", count)
	}
}

// Two products of one brand with two best-before dates are two products, and
// the same product in two places is two products. Neither is a duplicate.
func TestCreateProductAcceptsADifferentProduct(t *testing.T) {
	t.Run("a differing expiry is not a duplicate", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       env.Household.ID,
			UserID:            env.User.ID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := env.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		w := postCreate(t, env, dbModel.Product{
			Barcode:           "4006381333931",
			ExpireAt:          duplicateExpiry.AddDate(0, 0, 1),
			StorageLocationID: &location.ID,
		})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		if count := countProducts(t, env.DB, "4006381333931"); count != 2 {
			t.Errorf("rows with the barcode = %d, want 2", count)
		}
	})

	t.Run("a differing location is not a duplicate", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		otherLocation := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       env.Household.ID,
			UserID:            env.User.ID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := env.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		w := postCreate(t, env, dbModel.Product{
			Barcode:           "4006381333931",
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &otherLocation.ID,
		})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		if count := countProducts(t, env.DB, "4006381333931"); count != 2 {
			t.Errorf("rows with the barcode = %d, want 2", count)
		}
	})

	t.Run("an archived row is not a duplicate", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       env.Household.ID,
			UserID:            env.User.ID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := env.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}
		if err := env.DB.Delete(&existing).Error; err != nil {
			t.Fatalf("archive existing product: %v", err)
		}

		w := postCreate(t, env, dbModel.Product{
			Barcode:           "4006381333931",
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("another member's private row is not a duplicate", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       env.Household.ID,
			UserID:            member.ID,
			IsPrivate:         true,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := env.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		w := postCreate(t, env, dbModel.Product{
			Barcode:           "4006381333931",
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}
		if body := w.Body.String(); strings.Contains(body, "duplicate") {
			t.Errorf("body = %s, want no mention of another member's private row", body)
		}
	})
}

// The idempotency check stands in front of the duplicate check: a replayed
// clientSyncId answers with the row it already wrote.
func TestCreateProductSyncIDReplayBeatsTheDuplicateCheck(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	syncID := "replay-42"
	existing := dbModel.Product{
		Barcode:           "4006381333931",
		HouseholdID:       env.Household.ID,
		UserID:            env.User.ID,
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		ClientSyncID:      &syncID,
	}
	if err := env.DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing product: %v", err)
	}

	w := postCreate(t, env, dbModel.Product{
		Barcode:           "4006381333931",
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		ClientSyncID:      &syncID,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if count := countProducts(t, env.DB, "4006381333931"); count != 1 {
		t.Errorf("rows with the barcode = %d, want 1", count)
	}
}

// The client that has seen the refusal asks for a row of its own.
func TestCreateProductAllowDuplicateWritesTheRow(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	existing := dbModel.Product{
		Barcode:           "4006381333931",
		HouseholdID:       env.Household.ID,
		UserID:            env.User.ID,
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
	}
	if err := env.DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing product: %v", err)
	}

	w := postCreate(t, env, dbModel.Product{
		Barcode:           "4006381333931",
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		AllowDuplicate:    true,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
	if count := countProducts(t, env.DB, "4006381333931"); count != 2 {
		t.Errorf("rows with the barcode = %d, want 2", count)
	}
}

// A padded barcode is the same barcode: the row is stored trimmed, and a
// padded draft is refused against the trimmed row it duplicates.
func TestCreateProductNormalizesTheBarcode(t *testing.T) {
	t.Run("the row is stored with a trimmed barcode", func(t *testing.T) {
		env := setupHandlerTest(t)

		w := postCreate(t, env, dbModel.Product{
			Barcode:  " 4006381333931 ",
			ExpireAt: duplicateExpiry,
		})

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}

		var stored dbModel.Product
		if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.Barcode != "4006381333931" {
			t.Errorf("barcode = %q, want %q", stored.Barcode, "4006381333931")
		}
		if count := countProducts(t, env.DB, " 4006381333931 "); count != 0 {
			t.Errorf("rows with the padded barcode = %d, want 0", count)
		}
	})

	t.Run("a padded draft is refused against the trimmed row", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       env.Household.ID,
			UserID:            env.User.ID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := env.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		w := postCreate(t, env, dbModel.Product{
			Barcode:           " 4006381333931 ",
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		})

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body = %s", w.Code, w.Body.String())
		}
		if count := countProducts(t, env.DB, "4006381333931"); count != 1 {
			t.Errorf("rows with the barcode = %d, want 1", count)
		}
	})

	t.Run("a blank barcode is missing, not a barcode", func(t *testing.T) {
		env := setupHandlerTest(t)

		w := postCreate(t, env, dbModel.Product{
			Barcode:  "   ",
			ExpireAt: duplicateExpiry,
		})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
		if count := countProducts(t, env.DB, ""); count != 0 {
			t.Errorf("rows with an empty barcode = %d, want 0", count)
		}
	})
}

// Adding a pack to a matched row is a delta on that row. The count stays
// inside the range a pack count may hold.
func TestAddAPackKeepsTheCountInRange(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	existing := dbModel.Product{
		Barcode:           "4006381333931",
		HouseholdID:       env.Household.ID,
		UserID:            env.User.ID,
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		Quantity:          util.MaxProductQuantity - 2,
	}
	if err := env.DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing product: %v", err)
	}

	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Params = []gin.Param{{Key: "id", Value: itoa(existing.ID)}}
	ctx.Request = newRawJSONRequest(`{"delta":5}`)
	UpdateProductQuantity(ctx, env.AppCtx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var stored dbModel.Product
	if err := env.DB.First(&stored, existing.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != util.MaxProductQuantity {
		t.Errorf("quantity = %d, want %d", stored.Quantity, util.MaxProductQuantity)
	}
}

// The create path keeps the stock the client answered: where the packs go,
// who may see them and when they were opened. The OpenFoodFacts lookup knows
// the product, not the stock.
func TestCreateProductKeepsTheClientStockFields(t *testing.T) {
	env := setupHandlerTest(t)
	location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
	openedAt := duplicateExpiry.AddDate(0, 0, -3)
	shelfLife := 7

	w := postCreate(t, env, dbModel.Product{
		Barcode:           "4006381333931",
		ExpireAt:          duplicateExpiry,
		StorageLocationID: &location.ID,
		IsPrivate:         true,
		OpenedAt:          &openedAt,
		DaysAfterOpening:  &shelfLife,
		Quantity:          3,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}

	var stored dbModel.Product
	if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.StorageLocationID == nil || *stored.StorageLocationID != location.ID {
		t.Errorf("storageLocationId = %v, want %d", stored.StorageLocationID, location.ID)
	}
	if !stored.IsPrivate {
		t.Error("isPrivate = false, want true")
	}
	if stored.OpenedAt == nil || !stored.OpenedAt.Equal(openedAt) {
		t.Errorf("openedAt = %v, want %v", stored.OpenedAt, openedAt)
	}
	if stored.DaysAfterOpening == nil || *stored.DaysAfterOpening != shelfLife {
		t.Errorf("daysAfterOpening = %v, want %d", stored.DaysAfterOpening, shelfLife)
	}
	if stored.Quantity != 3 {
		t.Errorf("quantity = %d, want 3", stored.Quantity)
	}
}

// The bulk create and the offline sync are insert-only: a batch may hold
// several packs of one product, and a background sync cannot ask a question.
func TestBulkAndOfflineSyncWriteADuplicateAsItsOwnRow(t *testing.T) {
	t.Run("the bulk create writes the row", func(t *testing.T) {
		ctx, w, appCtx, locationID := newBulkTestSetup(t)
		householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
		if err != nil {
			t.Fatalf("household of the user: %v", err)
		}
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            appCtx.UserID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &locationID,
		}
		if err := appCtx.DB.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		testutil.CreateTestRequest(ctx, apiModel.BulkCreateRequest{
			Items: []apiModel.BulkProductDraft{
				{
					ProductName:       "Yogurt",
					Barcode:           "4006381333931",
					Quantity:          1,
					ExpireAt:          duplicateExpiry.Format(util.DefaultDateFormatParseStr),
					StorageLocationID: &locationID,
				},
			},
		})

		BulkCreateProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if count := countProducts(t, appCtx.DB, "4006381333931"); count != 2 {
			t.Errorf("rows with the barcode = %d, want 2", count)
		}
	})

	t.Run("the offline sync writes the row", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)
		location := testutil.CreateTestStorageLocation(db, user.HouseholdID)
		existing := dbModel.Product{
			Barcode:           "4006381333931",
			HouseholdID:       user.HouseholdID,
			UserID:            user.ID,
			ExpireAt:          duplicateExpiry,
			StorageLocationID: &location.ID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create existing product: %v", err)
		}

		body := `{"items":[{"clientSyncId":"sync-1","barcode":"4006381333931","productName":"Yogurt","quantity":1,"expireAt":"2026-10-29","storageLocationId":` +
			itoa(location.ID) + `}]}`
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Request = newRawJSONRequest(body)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
		ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
		appCtx := SetupTestAppContext(ctx, user.ID)

		SyncOfflineProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var count int64
		db.Model(&dbModel.Product{}).Where("barcode = ?", "4006381333931").Count(&count)
		if count != 2 {
			t.Errorf("rows with the barcode = %d, want 2", count)
		}
	})
}

// A lookup that failed without an error leaves the client's product alone,
// so the row keeps its barcode and its name.
func TestCreateProductKeepsTheClientProductWhenTheLookupFails(t *testing.T) {
	env := setupHandlerTest(t)
	raw, marshalErr := json.Marshal(dbModel.Product{
		Barcode:     "4006381333931",
		ProductName: "Client Name",
		ExpireAt:    duplicateExpiry,
	})
	if marshalErr != nil {
		t.Fatalf("marshal body: %v", marshalErr)
	}
	ctx, w := env.freshCtx(env.AppCtx.UserID)
	ctx.Request = newRawJSONRequest(string(raw))
	ctx.Set("offacntrl", &MockOpenFoodFactsAPIController{
		MockGetDataset: func(barcode string) (dbModel.Product, error) {
			return dbModel.Product{}, nil
		},
	})
	CreateProduct(ctx, env.AppCtx)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}

	var stored dbModel.Product
	if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.ProductName != "Client Name" {
		t.Errorf("productName = %q, want %q", stored.ProductName, "Client Name")
	}
}
