package v1

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	repomocks "github.com/Isotop7/proviant/testutil/mocks"
)

// TestProductActions_WasteCause verifies that a waste carries its reason, that a
// waste without one stays unattributed and that an unknown reason is refused
// before it reaches the service.
func TestProductActions_WasteCause(t *testing.T) {
	t.Run("waste records the cause", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
		cause := dbModel.WasteCauseSpoiled
		request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/waste", apiModel.ProductActionDTO{WasteCause: &cause})

		recorder := performProductAction(m, WasteProduct, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if len(m.Products.WastePartialCauses) != 1 || m.Products.WastePartialCauses[0] != dbModel.WasteCauseSpoiled {
			t.Errorf("waste causes = %v, want [%s]", m.Products.WastePartialCauses, dbModel.WasteCauseSpoiled)
		}
	})

	t.Run("waste without a cause stays unattributed", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
		request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/waste", apiModel.ProductActionDTO{})

		recorder := performProductAction(m, WasteProduct, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if len(m.Products.WastePartialCauses) != 1 || m.Products.WastePartialCauses[0] != "" {
			t.Errorf("waste causes = %v, want one empty cause", m.Products.WastePartialCauses)
		}
	})

	t.Run("unknown cause is refused", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
		cause := "melted"
		request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/waste", apiModel.ProductActionDTO{WasteCause: &cause})

		recorder := performProductAction(m, WasteProduct, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), errors.ErrInvalidWasteCause.Error()) {
			t.Errorf("body %q does not contain %q", recorder.Body.String(), errors.ErrInvalidWasteCause.Error())
		}
		if len(m.Products.WastePartialQuantities) != 0 {
			t.Error("a refused cause reached a repository removal")
		}
	})

	t.Run("consume ignores the cause", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
		cause := dbModel.WasteCauseSpoiled
		request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/consume", apiModel.ProductActionDTO{WasteCause: &cause})

		recorder := performProductAction(m, ConsumeProduct, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if len(m.Products.ConsumePartialQuantities) != 1 {
			t.Errorf("consume quantities = %v, want one removal", m.Products.ConsumePartialQuantities)
		}
		if len(m.Products.WastePartialCauses) != 0 {
			t.Errorf("a consume recorded a waste cause: %v", m.Products.WastePartialCauses)
		}
	})
}

// TestBulkWaste_Cause verifies that a bulk waste writes its reason to every
// product of the request and that an unknown reason is refused.
func TestBulkWaste_Cause(t *testing.T) {
	t.Run("unknown cause is refused", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		cause := "melted"
		request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/bulkWaste",
			apiModel.BulkProductsAPIModel{ProductIDs: []uint{1}, WasteCause: &cause})

		recorder := performProductAction(m, BulkWasteProducts, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), errors.ErrInvalidWasteCause.Error()) {
			t.Errorf("body %q does not contain %q", recorder.Body.String(), errors.ErrInvalidWasteCause.Error())
		}
	})
}
