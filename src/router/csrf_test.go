package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func csrfTestConfig() *configuration.ProviantConfiguration {
	return &configuration.ProviantConfiguration{}
}

func TestIsStateMutatingMethod(t *testing.T) {
	mutating := []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete}
	safe := []string{http.MethodGet, http.MethodHead, http.MethodOptions}

	for _, method := range mutating {
		if !isStateMutatingMethod(method) {
			t.Errorf("isStateMutatingMethod(%q) = false, want true", method)
		}
	}
	for _, method := range safe {
		if isStateMutatingMethod(method) {
			t.Errorf("isStateMutatingMethod(%q) = true, want false", method)
		}
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() error = %v", err)
	}
	if len(token) != csrfTokenBytes*2 {
		t.Errorf("token length = %d, want %d", len(token), csrfTokenBytes*2)
	}
	other, _ := generateCSRFToken()
	if token == other {
		t.Errorf("two generated tokens are identical: %q", token)
	}
}

// buildCSRFRouter wires the middleware with a trivial handler behind it.
func buildCSRFRouter(cfg *configuration.ProviantConfiguration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CSRFMiddleware(cfg))
	engine.Any("/*path", func(ctx *gin.Context) {
		token, _ := ctx.Get(util.ContextKeyCSRFToken)
		ctx.String(http.StatusOK, "ok %v", token)
	})
	return engine
}

func TestCSRFMiddleware(t *testing.T) {
	t.Run("issues cookie and context token on first GET", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/web", nil))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		cookies := w.Result().Cookies()
		var found *http.Cookie
		for _, c := range cookies {
			if c.Name == "csrf_token" {
				found = c
			}
		}
		if found == nil {
			t.Fatalf("csrf_token cookie not set; cookies: %v", cookies)
		}
		if len(found.Value) != csrfTokenBytes*2 {
			t.Errorf("cookie token length = %d, want %d", len(found.Value), csrfTokenBytes*2)
		}
		if found.MaxAge != csrfDefaultMaxAge {
			t.Errorf("cookie MaxAge = %d, want %d", found.MaxAge, csrfDefaultMaxAge)
		}
	})

	t.Run("reuses existing cookie token", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		req := httptest.NewRequest(http.MethodGet, "/web", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "existing-token"})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Body.String(); got != "ok existing-token" {
			t.Errorf("body = %q, want %q", got, "ok existing-token")
		}
		for _, c := range w.Result().Cookies() {
			if c.Name == "csrf_token" {
				t.Errorf("csrf_token cookie re-issued: %q", c.Value)
			}
		}
	})

	t.Run("POST without token is rejected", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/products", nil))

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("POST with mismatched header and cookie is rejected", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
		req.Header.Set("X-CSRF-Token", "different-token")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("POST with matching header and cookie passes", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "valid-token"})
		req.Header.Set("X-CSRF-Token", "valid-token")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST without cookie but with header token is rejected", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", nil)
		req.Header.Set("X-CSRF-Token", "some-token")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("Authorization header exempts request from CSRF check", func(t *testing.T) {
		engine := buildCSRFRouter(csrfTestConfig())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", nil)
		req.Header.Set("Authorization", "Bearer some-token")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		for _, c := range w.Result().Cookies() {
			if c.Name == "csrf_token" {
				t.Errorf("csrf_token cookie issued for programmatic client: %q", c.Value)
			}
		}
	})

	t.Run("custom max age is honored", func(t *testing.T) {
		cfg := csrfTestConfig()
		cfg.Server.SecurityHeaders.CSRFTokenMaxAge = 3600
		engine := buildCSRFRouter(cfg)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/web", nil))

		var found *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "csrf_token" {
				found = c
			}
		}
		if found == nil {
			t.Fatalf("csrf_token cookie not set; cookies: %v", w.Result().Cookies())
		}
		if found.MaxAge != 3600 {
			t.Errorf("cookie MaxAge = %d, want 3600", found.MaxAge)
		}
	})
}
