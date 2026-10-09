package router

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"

	"github.com/gin-gonic/gin"
)

func contains(s, substr string) bool { return strings.Contains(s, substr) }

func buildSecurityHeadersRouter(cfg *configuration.ProviantConfiguration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SecurityHeadersMiddleware(cfg))
	engine.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "ok")
	})
	return engine
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	t.Run("sets security headers and CSP nonce", func(t *testing.T) {
		engine := buildSecurityHeadersRouter(&configuration.ProviantConfiguration{})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// secure only emits HSTS over TLS; simulate an HTTPS request.
		req.TLS = &tls.ConnectionState{}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		headers := map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "strict-origin-when-cross-origin",
			"Content-Security-Policy": "default-src 'self'",
		}
		for header, want := range headers {
			got := w.Header().Get(header)
			if !contains(got, want) {
				t.Errorf("header %q = %q, want containing %q", header, got, want)
			}
		}
		if w.Header().Get("Strict-Transport-Security") == "" {
			t.Errorf("Strict-Transport-Security header not set")
		}
	})

	t.Run("custom CSP from configuration is used", func(t *testing.T) {
		cfg := &configuration.ProviantConfiguration{}
		custom := "default-src 'none'"
		cfg.Server.SecurityHeaders.ContentSecurityPolicy = custom
		engine := buildSecurityHeadersRouter(cfg)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if got := w.Header().Get("Content-Security-Policy"); got != custom {
			t.Errorf("Content-Security-Policy = %q, want %q", got, custom)
		}
	})
}
