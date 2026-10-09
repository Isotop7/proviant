package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// TestBulkRestoreProductsReportsFailures pins the partial-failure contract:
// the endpoint used to answer 200 "were restored" for every requested id even
// when the repository restored none of them, so the client could not tell a
// failure from a success. Unknown ids must surface as 404 (all failed) or in
// the 200 message (partial), never as a blanket success.
func TestBulkRestoreProductsReportsFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	archived := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(archived).Error; err != nil {
		t.Fatalf("failed to archive product: %v", err)
	}

	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)
	newCtx := func() (*gin.Context, *AppContext, *httptest.ResponseRecorder) {
		ctx, w := repomocks.SetupGinContextWithDB(db)
		appCtx := &AppContext{
			Logger:   &logger,
			Repos:    repos,
			UserID:   user.ID,
			Products: services.NewProductService(repos, &logger),
		}
		return ctx, appCtx, w
	}

	t.Run("all ids failing is 404", func(t *testing.T) {
		ctx, appCtx, w := newCtx()
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkRestore",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{999999}})

		BulkRestoreProducts(ctx, appCtx)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "failedIds") || !strings.Contains(w.Body.String(), "999999") {
			t.Errorf("404 must carry the failed ids, body: %s", w.Body.String())
		}
	})

	t.Run("partial failure is 200 naming the failed ids", func(t *testing.T) {
		ctx, appCtx, w := newCtx()
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkRestore",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{archived.ID, 999999}})

		BulkRestoreProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "except") || !strings.Contains(w.Body.String(), "999999") {
			t.Errorf("partial success must name the failed ids, body: %s", w.Body.String())
		}
		// The message is prose; the client branches on failedIds.
		if !strings.Contains(w.Body.String(), `"failedIds":[999999]`) {
			t.Errorf("partial success must carry failedIds, body: %s", w.Body.String())
		}
	})

	t.Run("full success is 200 without a failure note", func(t *testing.T) {
		second := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := db.Delete(second).Error; err != nil {
			t.Fatalf("failed to archive product: %v", err)
		}
		ctx, appCtx, w := newCtx()
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkRestore",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{second.ID}})

		BulkRestoreProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "except") {
			t.Errorf("full success must not report failures, body: %s", w.Body.String())
		}
		if strings.Contains(w.Body.String(), "failedIds") {
			t.Errorf("full success must not carry failedIds, body: %s", w.Body.String())
		}
	})

	// A server-side failure still answers 500, but the ids that did not land
	// ride along: without them the client cannot tell what to retry.
	t.Run("server-side failure is 500 carrying failedIds", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Err = errors.ErrInternalServer
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkRestore",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{123}})

		logger := zerolog.Nop()
		repos := m.ToRepositoryContainer()
		BulkRestoreProducts(ctx, &AppContext{
			Logger:   &logger,
			Repos:    repos,
			UserID:   1,
			Products: services.NewProductService(repos, &logger),
		})

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"failedIds":[123]`) {
			t.Errorf("500 must carry the failed ids, body: %s", w.Body.String())
		}
	})
}

// TestRestoreProductRejectsZeroID keeps id 0 at 400: the repositories answer it
// with gorm.ErrNotImplemented, which the handler would otherwise report as 500.
func TestRestoreProductRejectsZeroID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)

	ctx, w := repomocks.SetupGinContextWithDB(db)
	ctx.Params = []gin.Param{{Key: "id", Value: "0"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/0/restore", nil)

	RestoreProduct(ctx, &AppContext{
		Logger:   &logger,
		Repos:    repos,
		UserID:   user.ID,
		Products: services.NewProductService(repos, &logger),
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}
