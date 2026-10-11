package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/api"
	apperrors "codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
)

// TestAppContextMiddleware_ReturnsJSONBodyWithoutLogger covers the abort that
// used to write a bare 500: browser fetch callers parse every response as JSON.
func TestAppContextMiddleware_ReturnsJSONBodyWithoutLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)

	AppContextMiddleware()(ctx)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusInternalServerError, w.Body.String())
	}
	if !ctx.IsAborted() {
		t.Error("middleware did not abort: the handler chain would run and append a second body")
	}

	var resp api.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not parseable JSON: %v; body = %q", err, w.Body.String())
	}

	if resp.Message != apperrors.ErrLoggerContextNotFound.Error() {
		t.Errorf("message = %q, want %q", resp.Message, apperrors.ErrLoggerContextNotFound.Error())
	}
}
