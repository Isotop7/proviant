package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	repomocks "github.com/Isotop7/proviant/testutil/mocks"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func performProductAction(m *repomocks.MockRepositoryContainer, handler func(*gin.Context, *AppContext), request *http.Request) *httptest.ResponseRecorder {
	ctx, recorder := repomocks.SetupGinContextWithMocks(m)
	testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
	ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
	ctx.Request = request
	handler(ctx, newTestAppContext(m, 1))
	return recorder
}

// TestProductActions_OptionalBody verifies backward-compatible full removal and quantity/token forwarding.
func TestProductActions_OptionalBody(t *testing.T) {
	actions := []struct {
		name    string
		handler func(*gin.Context, *AppContext)
	}{
		{name: "consume", handler: ConsumeProduct},
		{name: "waste", handler: WasteProduct},
	}
	requests := []struct {
		name         string
		request      func() *http.Request
		wantQuantity int
		wantMutation string
	}{
		{
			name: "bodyless consumes whole stock",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/api/v1/products/1/consume", nil)
			},
			wantQuantity: 0,
		},
		{
			name: "empty object consumes whole stock",
			request: func() *http.Request {
				return testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/consume", apiModel.ProductActionDTO{})
			},
			wantQuantity: 0,
		},
		{
			name: "partial quantity and trimmed mutation id",
			request: func() *http.Request {
				quantity := 1
				mutationID := "  offline-action-1  "
				return testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/consume", apiModel.ProductActionDTO{Quantity: &quantity, ClientMutationID: &mutationID})
			},
			wantQuantity: 1,
			wantMutation: "offline-action-1",
		},
	}

	for _, action := range actions {
		for _, tt := range requests {
			t.Run(action.name+"/"+tt.name, func(t *testing.T) {
				m := repomocks.NewMockRepositoryContainer()
				m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}

				recorder := performProductAction(m, action.handler, tt.request())
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
				}

				var quantities []int
				var mutationIDs []*string
				if action.name == "consume" {
					quantities = m.Products.ConsumePartialQuantities
					mutationIDs = m.Products.ConsumePartialMutationIDs
				} else {
					quantities = m.Products.WastePartialQuantities
					mutationIDs = m.Products.WastePartialMutationIDs
				}
				if len(quantities) != 1 || quantities[0] != tt.wantQuantity {
					t.Errorf("quantities = %v, want [%d]", quantities, tt.wantQuantity)
				}
				if len(mutationIDs) != 1 {
					t.Fatalf("mutation id calls = %d, want 1", len(mutationIDs))
				}
				if tt.wantMutation == "" && mutationIDs[0] != nil {
					t.Errorf("mutation id = %q, want nil", *mutationIDs[0])
				}
				if tt.wantMutation != "" && (mutationIDs[0] == nil || *mutationIDs[0] != tt.wantMutation) {
					t.Errorf("mutation id = %v, want %q", mutationIDs[0], tt.wantMutation)
				}
			})
		}
	}
}

// TestProductActions_RejectsInvalidBody verifies malformed JSON and invalid quantities never reach the service.
func TestProductActions_RejectsInvalidBody(t *testing.T) {
	actions := []struct {
		name    string
		handler func(*gin.Context, *AppContext)
	}{
		{name: "consume", handler: ConsumeProduct},
		{name: "waste", handler: WasteProduct},
	}
	tests := []struct {
		name     string
		body     string
		wantText string
	}{
		{name: "zero quantity", body: `{"quantity":0}`, wantText: errors.ErrInvalidProductQuantity.Error()},
		{name: "negative quantity", body: `{"quantity":-2}`, wantText: errors.ErrInvalidProductQuantity.Error()},
		{name: "malformed JSON", body: `{"quantity":`},
		{name: "overlong mutation id", body: `{"clientMutationId":"` + strings.Repeat("x", util.OfflineSyncIDMaxLength+1) + `"}`, wantText: errors.ErrClientMutationIDTooLong.Error()},
	}

	for _, action := range actions {
		for _, tt := range tests {
			t.Run(action.name+"/"+tt.name, func(t *testing.T) {
				m := repomocks.NewMockRepositoryContainer()
				request := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/action", strings.NewReader(tt.body))
				request.Header.Set("Content-Type", "application/json")

				recorder := performProductAction(m, action.handler, request)
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				if tt.wantText != "" && !strings.Contains(recorder.Body.String(), tt.wantText) {
					t.Errorf("body %q does not contain %q", recorder.Body.String(), tt.wantText)
				}
				if len(m.Products.ConsumePartialQuantities) != 0 || len(m.Products.WastePartialQuantities) != 0 {
					t.Error("invalid body reached a repository removal")
				}
			})
		}
	}
}

// TestProductActions_RejectsImpossibleInput verifies that a pack count above the maximum and an oversized body never reach the service.
func TestProductActions_RejectsImpossibleInput(t *testing.T) {
	actions := []struct {
		name    string
		handler func(*gin.Context, *AppContext)
	}{
		{name: "consume", handler: ConsumeProduct},
		{name: "waste", handler: WasteProduct},
	}
	tests := []struct {
		name     string
		body     string
		wantText string
	}{
		{name: "quantity above the maximum", body: `{"quantity":1000}`, wantText: errors.ErrProductQuantityTooLarge.Error()},
		{name: "oversized body", body: `{"quantity":1,"clientMutationId":"` + strings.Repeat("x", maxProductActionBodyBytes) + `"}`, wantText: api.InvalidInputError().Message},
	}

	for _, action := range actions {
		for _, tt := range tests {
			t.Run(action.name+"/"+tt.name, func(t *testing.T) {
				m := repomocks.NewMockRepositoryContainer()
				request := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/action", strings.NewReader(tt.body))
				request.Header.Set("Content-Type", "application/json")

				recorder := performProductAction(m, action.handler, request)
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				if !strings.Contains(recorder.Body.String(), tt.wantText) {
					t.Errorf("body %q does not contain %q", recorder.Body.String(), tt.wantText)
				}
				if len(m.Products.ConsumePartialQuantities) != 0 || len(m.Products.WastePartialQuantities) != 0 {
					t.Error("refused input reached a repository removal")
				}
			})
		}
	}
}

// TestProductActions_ReportsRemovedPacks verifies the response carries the number of packs that left the stock.
func TestProductActions_ReportsRemovedPacks(t *testing.T) {
	m := repomocks.NewMockRepositoryContainer()
	m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
	quantity := 2
	request := testutil.CreateJSONRequest(http.MethodPost, "/api/v1/products/1/consume", apiModel.ProductActionDTO{Quantity: &quantity})

	recorder := performProductAction(m, ConsumeProduct, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Removed int `json:"removed"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Removed != 2 {
		t.Errorf("removed = %d, want 2", body.Removed)
	}
}

// TestProductActions_ConcurrentModification verifies both actions expose guarded-write conflicts as 409.
func TestProductActions_ConcurrentModification(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*gin.Context, *AppContext)
		waste   bool
	}{
		{name: "consume", handler: ConsumeProduct},
		{name: "waste", handler: WasteProduct, waste: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := repomocks.NewMockRepositoryContainer()
			m.Products.Product = dbModel.Product{ProductName: "Yogurt", Quantity: 3}
			if tt.waste {
				m.Products.WastePartialErr = errors.ErrProductConcurrentModification
			} else {
				m.Products.ConsumePartialErr = errors.ErrProductConcurrentModification
			}

			recorder := performProductAction(m, tt.handler, httptest.NewRequest(http.MethodPost, "/api/v1/products/1/action", nil))
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusConflict, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), MsgProductConcurrentModification) {
				t.Errorf("body must carry retry message %q, got: %s", MsgProductConcurrentModification, recorder.Body.String())
			}
		})
	}
}
