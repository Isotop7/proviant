package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Isotop7/proviant/api"
	apperrors "github.com/Isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
)

// assertAPIResponseBody fails unless the recorder holds a 500 carrying a
// parseable api.APIResponse: browser fetch callers call res.json() unguarded, so
// an empty body surfaces as a SyntaxError rather than a message.
func assertAPIResponseBody(t *testing.T, w *httptest.ResponseRecorder, wantMessage string) {
	t.Helper()

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusInternalServerError, w.Body.String())
	}

	var resp api.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not parseable JSON: %v; body = %q", err, w.Body.String())
	}

	if resp.Message != wantMessage {
		t.Errorf("message = %q, want %q", resp.Message, wantMessage)
	}
}

// TestUnauthorizedFrontendFunc_ReturnsJSONBodyOnMissingTemplateCache pins the
// body of a defensive branch: MustGet panics on a missing key, so only a
// wrong-typed value reaches it.
func TestUnauthorizedFrontendFunc_ReturnsJSONBodyOnMissingTemplateCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/web/", nil)
	ctx.Set("templateCache", "not a template map")

	UnauthorizedFrontendFunc(ctx, http.StatusForbidden, "forbidden")

	assertAPIResponseBody(t, w, apperrors.ErrTemplateCacheNotFound.Error())
}
