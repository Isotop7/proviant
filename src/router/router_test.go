package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
)

func TestSetupRouter_BuildsEngineWithRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.TokenPassword = "test-token-password"
	cfg.Server.Authentication.TokenLifetime = 1
	cfg.Server.CORS.AllowAllOrigins = true

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	cfg.TemplateCache = templateCache

	logger := zerolog.Nop()
	engine := SetupRouter(&logger, cfg, db, nil, nil, nil)
	if engine == nil {
		t.Fatal("SetupRouter() returned nil engine")
	}

	if len(engine.Routes()) == 0 {
		t.Error("engine has no registered routes")
	}

	// The health route must be reachable end to end.
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want %d; routes: %d", w.Code, http.StatusOK, len(engine.Routes()))
	}

	// Unknown routes fall through the middleware chain to gin's 404 handler.
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/definitely-not-a-route", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET unknown route = %d, want %d", w.Code, http.StatusNotFound)
	}
}
