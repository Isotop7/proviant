package v1

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	proviantErrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newOfflineSyncUser seeds an admin user who owns a household, the membership
// every sync-offline request is authorized by.
func newOfflineSyncUser(t *testing.T, db *gorm.DB) *authentication.User {
	t.Helper()
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	user.Role = authentication.RoleAdmin
	db.Save(user)
	return user
}

func TestSyncOfflineProductsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	user := newOfflineSyncUser(t, db)

	body := `{"items":[{"clientSyncId":"sync-1","productName":"Milk","amount":2,"expireAt":"2099-01-01"}]}`

	makeCtx := func() (*gin.Context, *httptest.ResponseRecorder, *AppContext) {
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Request = newRawJSONRequest(body)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
		ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
		appCtx := SetupTestAppContext(ctx, user.ID)
		return ctx, w, appCtx
	}

	ctx, w, appCtx := makeCtx()
	SyncOfflineProducts(ctx, appCtx)
	if w.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var first struct {
		Results []struct {
			Status    string `json:"status"`
			ProductID *uint  `json:"productId"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first: %v", err)
	}
	if len(first.Results) != 1 || first.Results[0].Status != "created" || first.Results[0].ProductID == nil {
		t.Fatalf("first result = %+v", first.Results)
	}
	if !waitBulkSideEffectsIdle(5 * time.Second) {
		t.Fatal("side-effect workers did not settle")
	}

	ctx2, w2, appCtx2 := makeCtx()
	SyncOfflineProducts(ctx2, appCtx2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body = %s", w2.Code, w2.Body.String())
	}
	var second struct {
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if len(second.Results) != 1 || second.Results[0].Status != "already-synced" {
		t.Fatalf("second result = %+v", second.Results)
	}

	var count int64
	db.Model(&dbModel.Product{}).Count(&count)
	if count != 1 {
		t.Fatalf("product count = %d, want 1", count)
	}
}

// The body cap is configuration: the outbox client splits its
// batches at the value injected into the page, so an
// operator-lowered cap has to be the one the endpoint enforces,
// not the built-in default.
func TestSyncOfflineProductsConfiguredBodyCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	user := newOfflineSyncUser(t, db)

	body := fmt.Sprintf(`{"items":[{"clientSyncId":"sync-fat","productName":"%s"}]}`, strings.Repeat("x", 2048))

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Request = newRawJSONRequest(body)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{
		OfflineSync: configuration.OfflineSyncConfiguration{MaxBatchBytes: 512},
	})
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
	appCtx := SetupTestAppContext(ctx, user.ID)

	SyncOfflineProducts(ctx, appCtx)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), proviantErrors.ErrFileTooLarge.Error()) {
		t.Errorf("body = %s, want the too-large error %q", w.Body.String(), proviantErrors.ErrFileTooLarge.Error())
	}
}

func TestSyncOfflineProductsItemOutcomes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type result struct {
		Index        int    `json:"index"`
		ClientSyncID string `json:"clientSyncId"`
		Status       string `json:"status"`
		Reason       string `json:"reason"`
		ProductID    *uint  `json:"productId"`
		Terminal     bool   `json:"terminal"`
	}
	type response struct {
		Results []result `json:"results"`
	}

	// post submits one batch and returns the per-item outcome.
	post := func(t *testing.T, db *gorm.DB, userID uint, body string) (int, response) {
		t.Helper()
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, userID)
		ctx.Request = newRawJSONRequest(body)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
		ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
		SyncOfflineProducts(ctx, SetupTestAppContext(ctx, userID))

		var decoded response
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
				t.Fatalf("decode response: %v", err)
			}
		}
		if !waitBulkSideEffectsIdle(5 * time.Second) {
			t.Fatal("side-effect workers did not settle")
		}
		return w.Code, decoded
	}

	t.Run("a blank client sync id fails without touching the database", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)

		code, resp := post(t, db, user.ID, `{"items":[{"clientSyncId":"   ","productName":"Milk"}]}`)

		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != "failed" {
			t.Fatalf("results = %+v", resp.Results)
		}
		if !resp.Results[0].Terminal {
			t.Error("blank sync id is not terminal — the same draft would fail the same way on every replay")
		}
		var count int64
		db.Model(&dbModel.Product{}).Count(&count)
		if count != 0 {
			t.Errorf("product count = %d, want 0", count)
		}
	})

	t.Run("an over-long client sync id fails without touching the database", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)

		oversized := strings.Repeat("s", util.OfflineSyncIDMaxLength+1)
		body := fmt.Sprintf(`{"items":[{"clientSyncId":%q,"productName":"Milk"}]}`, oversized)
		code, resp := post(t, db, user.ID, body)

		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != "failed" {
			t.Fatalf("results = %+v", resp.Results)
		}
		var count int64
		db.Model(&dbModel.Product{}).Count(&count)
		if count != 0 {
			t.Errorf("product count = %d, want 0", count)
		}
	})

	t.Run("a client sync id at the bound is accepted", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)

		atBound := strings.Repeat("s", util.OfflineSyncIDMaxLength)
		body := fmt.Sprintf(`{"items":[{"clientSyncId":%q,"productName":"Milk"}]}`, atBound)
		code, resp := post(t, db, user.ID, body)

		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != "created" {
			t.Fatalf("results = %+v, want created", resp.Results)
		}
	})

	t.Run("an invalid draft fails on its own without blocking the batch", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)

		body := `{"items":[` +
			`{"clientSyncId":"bad-date","productName":"Milk","expireAt":"not-a-date"},` +
			`{"clientSyncId":"good","productName":"Bread"}]}`
		code, resp := post(t, db, user.ID, body)

		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if len(resp.Results) != 2 {
			t.Fatalf("results = %+v", resp.Results)
		}
		if resp.Results[0].Status != "failed" || resp.Results[0].Reason == "" {
			t.Errorf("bad-date result = %+v, want failed with a reason", resp.Results[0])
		}
		if !resp.Results[0].Terminal {
			t.Errorf("bad-date result = %+v, want terminal — an unparseable date cannot be fixed by a replay", resp.Results[0])
		}
		if resp.Results[1].Status != "created" || resp.Results[1].ProductID == nil {
			t.Errorf("good result = %+v, want created", resp.Results[1])
		}
	})

	// A unique index on client_sync_id alone turned this into a constraint
	// violation reported as a permanent per-item failure.
	t.Run("the same sync id in another household is a separate product", func(t *testing.T) {
		db := testutil.SetupTestDB(t)

		first := newOfflineSyncUser(t, db)
		second := newOfflineSyncUser(t, db)

		body := `{"items":[{"clientSyncId":"shared-id","productName":"Milk"}]}`
		if code, resp := post(t, db, first.ID, body); code != http.StatusOK || resp.Results[0].Status != "created" {
			t.Fatalf("first household: status = %d, results = %+v", code, resp.Results)
		}
		code, resp := post(t, db, second.ID, body)
		if code != http.StatusOK {
			t.Fatalf("second household status = %d, want 200", code)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != "created" {
			t.Fatalf("second household results = %+v, want created", resp.Results)
		}

		var count int64
		db.Model(&dbModel.Product{}).Count(&count)
		if count != 2 {
			t.Errorf("product count = %d, want 2", count)
		}
	})

	// The lookup after a failed create exists for the retry that lost a race
	// to another worker's insert. A terminal failure is a property of the
	// draft — no worker inserted anything — so the lookup can only come back
	// empty. It is the query the endpoint sees most of, so it has to go.
	t.Run("a terminal failure does not spend the lost-race lookup", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := newOfflineSyncUser(t, db)

		var lookups int
		name := "test:count_sync_id_lookup"
		db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table != "products" {
				return
			}
			if strings.Contains(tx.Statement.SQL.String(), "client_sync_id") {
				lookups++
			}
		})
		t.Cleanup(func() { db.Callback().Query().Remove(name) })

		code, resp := post(t, db, user.ID, `{"items":[{"clientSyncId":"bad-location","productName":"Milk","storageLocationId":424242}]}`)

		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if len(resp.Results) != 1 || resp.Results[0].Status != "failed" || !resp.Results[0].Terminal {
			t.Fatalf("results = %+v, want one terminal failure", resp.Results)
		}
		if lookups != 1 {
			t.Errorf("client sync id lookups = %d, want 1 — only the pre-check, not a second one after the failure", lookups)
		}
	})
}

// A sync id held by another member's private product must not resolve as
// already-synced: that status makes the outbox drop the queued create, so the
// caller's product would be reported done and never created.
func TestSyncOfflineProductsIgnoresAnotherMembersPrivateSyncID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	owner := newOfflineSyncUser(t, db)
	member := testutil.CreateTestUser(db, owner.HouseholdID)

	syncID := "private-sync-id"
	private := &dbModel.Product{
		ProductName:  "Secret stash",
		Barcode:      "4006381333931",
		HouseholdID:  owner.HouseholdID,
		UserID:       owner.ID,
		IsPrivate:    true,
		ClientSyncID: &syncID,
	}
	if err := db.Create(private).Error; err != nil {
		t.Fatalf("seed private product: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, member.ID)
	ctx.Request = newRawJSONRequest(`{"items":[{"clientSyncId":"private-sync-id","productName":"Offline milk"}]}`)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
	SyncOfflineProducts(ctx, SetupTestAppContext(ctx, member.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Results []struct {
			Status    string `json:"status"`
			ProductID *uint  `json:"productId"`
			Reason    string `json:"reason"`
			Terminal  bool   `json:"terminal"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "failed" {
		t.Fatalf("results = %+v, want one failed item", resp.Results)
	}
	if !resp.Results[0].Terminal {
		t.Error("item is not terminal — the outbox would replay an id this member can never own, burning its whole retry budget on the same refusal")
	}
	if resp.Results[0].ProductID != nil {
		t.Errorf("failed item reported productId %d, want none — the row is not the caller's", *resp.Results[0].ProductID)
	}
	if resp.Results[0].Reason == "" {
		t.Error("failed item carries no reason")
	}

	var count int64
	db.Model(&dbModel.Product{}).Count(&count)
	if count != 1 {
		t.Errorf("product count = %d, want 1 — the sync must not have inserted a duplicate", count)
	}
}

// The other half of the terminal split: a failure the server did not choose must
// stay retryable. Parking a transient database error would strand a queued
// create that a second drain would have created.
func TestSyncOfflineProductsKeepsDatabaseErrorsRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	user := newOfflineSyncUser(t, db)

	name := "test:fail_product_create"
	db.Callback().Create().Before("gorm:before_create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "products" {
			_ = tx.AddError(stderrors.New("injected create failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Create().Remove(name) })

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Request = newRawJSONRequest(`{"items":[{"clientSyncId":"retry-me","productName":"Milk"}]}`)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
	SyncOfflineProducts(ctx, SetupTestAppContext(ctx, user.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Results []struct {
			Status   string `json:"status"`
			Terminal bool   `json:"terminal"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "failed" {
		t.Fatalf("results = %+v, want one failed item", resp.Results)
	}
	if resp.Results[0].Terminal {
		t.Error("a database error is marked terminal — the outbox would park a create the next drain would have made")
	}
}
