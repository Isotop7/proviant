package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// TestBulkConsumeAndWasteReportFailures pins the per-id failure contract of
// the consume/waste bulk endpoints: they used to log failures and answer 200
// "N products marked as ..." regardless, so the client had no way to learn
// that nothing landed. Unknown ids must surface as 404 with failedIds, a
// partial run as 200 with the failed ids, never as a blanket success.
func TestBulkConsumeAndWasteReportFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type bulkCase struct {
		name       string
		handler    func(*gin.Context, *AppContext)
		route      string
		wantStatus int
		wantBody   string
	}

	newCtx := func(t *testing.T) (*gin.Context, *AppContext, *httptest.ResponseRecorder, uint) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID, user.ID)

		logger := zerolog.Nop()
		repos := database.NewRepositoryContainer(db, nil)
		ctx, w := repomocks.SetupGinContextWithDB(db)
		appCtx := &AppContext{
			Logger:   &logger,
			Repos:    repos,
			UserID:   user.ID,
			Products: services.NewProductService(repos, &logger),
		}
		return ctx, appCtx, w, product.ID
	}

	for _, tc := range []bulkCase{
		{
			name:       "consume: every id unknown is 404 naming the failures",
			handler:    BulkConsumeProducts,
			route:      "/api/v1/products/bulkConsume",
			wantStatus: http.StatusNotFound,
			wantBody:   `"failedIds":[999999]`,
		},
		{
			name:       "waste: every id unknown is 404 naming the failures",
			handler:    BulkWasteProducts,
			route:      "/api/v1/products/bulkWaste",
			wantStatus: http.StatusNotFound,
			wantBody:   `"failedIds":[999999]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, appCtx, w, _ := newCtx(t)
			ctx.Request = testutil.CreateJSONRequest(http.MethodPost, tc.route,
				apiModel.BulkProductsAPIModel{ProductIDs: []uint{999999}})

			tc.handler(ctx, appCtx)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Errorf("body must carry the failed ids %s, got: %s", tc.wantBody, w.Body.String())
			}
		})
	}

	t.Run("consume: partial failure is 200 with the failed ids", func(t *testing.T) {
		ctx, appCtx, w, productID := newCtx(t)
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkConsume",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{productID, 999999}})

		BulkConsumeProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"failedIds":[999999]`) {
			t.Errorf("partial success must carry failedIds, body: %s", w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "except '999999'") {
			t.Errorf("partial run must name the failed id, body: %s", w.Body.String())
		}
	})

	t.Run("waste: full success carries no failedIds", func(t *testing.T) {
		ctx, appCtx, w, productID := newCtx(t)
		ctx.Request = testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkWaste",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{productID}})

		BulkWasteProducts(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "failedIds") {
			t.Errorf("full success must not carry failedIds, body: %s", w.Body.String())
		}
	})
}
