package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbcontroller "github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/templates"
	"github.com/Isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
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
	engine := SetupRouter(&logger, cfg, db, dbcontroller.NewRepositoryContainer(db, &logger), nil, nil, nil)
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

	// Readiness pings the database, so it must report ready against the test DB.
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /health/ready = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Metrics are opt-in: the route must not exist while the toggle is off.
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /metrics with metricsEnabled=false = %d, want %d", w.Code, http.StatusNotFound)
	}

	// Unknown routes fall through the middleware chain to gin's 404 handler.
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/definitely-not-a-route", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET unknown route = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestSetupRouter_MetricsEnabled covers the opt-in branch: with the toggle on,
// /metrics is served in Prometheus text format.
func TestSetupRouter_MetricsEnabled(t *testing.T) {
	db := testutil.SetupTestDB(t)

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.TokenPassword = "test-token-password"
	cfg.Server.Authentication.TokenLifetime = 1
	cfg.Server.CORS.AllowAllOrigins = true
	cfg.Server.MetricsEnabled = true

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	cfg.TemplateCache = templateCache

	logger := zerolog.Nop()
	engine := SetupRouter(&logger, cfg, db, dbcontroller.NewRepositoryContainer(db, &logger), nil, nil, nil)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want %d", w.Code, http.StatusOK)
	}

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want %d", w.Code, http.StatusOK)
	}
	// The request above went through the middleware, so its own counter must be
	// present — that is what proves the middleware is wired, not just the route.
	if !strings.Contains(w.Body.String(), `proviant_http_requests_total{method="GET",route="/health",status="200"}`) {
		t.Errorf("metrics body missing the /health observation:\n%s", w.Body.String())
	}
	// The domain collectors have to be registered next to the HTTP ones: the
	// worker, provider and Open Food Facts series are what tells an operator
	// whether the background jobs still work.
	for _, name := range []string{"proviant_openfoodfacts_live_requests_total", "proviant_db_connections"} {
		if !strings.Contains(w.Body.String(), name) {
			t.Errorf("metrics body missing the domain metric %q:\n%s", name, w.Body.String())
		}
	}
}

// TestSetupRouter_MetricsCountsRecoveredPanic pins the middleware order in the
// real engine: MetricsMiddleware must be registered before gin.Recovery.
// Swapping the two still serves every route and every test above, but a
// recovered panic then unwinds past the code after ctx.Next() and the 500 never
// reaches the counter — an error-rate alert silently loses its worst case.
func TestSetupRouter_MetricsCountsRecoveredPanic(t *testing.T) {
	db := testutil.SetupTestDB(t)

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.TokenPassword = "test-token-password"
	cfg.Server.Authentication.TokenLifetime = 1
	cfg.Server.CORS.AllowAllOrigins = true
	cfg.Server.MetricsEnabled = true

	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	cfg.TemplateCache = templateCache

	logger := zerolog.Nop()
	engine := SetupRouter(&logger, cfg, db, dbcontroller.NewRepositoryContainer(db, &logger), nil, nil, nil)
	engine.GET("/test-panic", func(ctx *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test-panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET /test-panic = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `proviant_http_requests_total{method="GET",route="/test-panic",status="500"}`) {
		t.Errorf("recovered panic missing from the counter; middleware order is wrong:\n%s", w.Body.String())
	}
}
