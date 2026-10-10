package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/rs/zerolog"
)

// newTestRegistry returns a fresh registry with the same runtime collectors the
// production registry carries, so tests are isolated from each other and can
// safely run in parallel.
func newTestRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

// metricsMiddlewareFor is the test-local variant of MetricsMiddleware: it
// records into the supplied registry instead of the production registry so
// tests stay isolated.
func metricsMiddlewareFor(registry prometheus.Registerer) gin.HandlerFunc {
	requestsTotal, requestDuration := newRequestMetrics(registry)
	return newMetricsMiddleware(requestsTotal, requestDuration)
}

// scrape returns the current exposition body from the given registry, so
// assertions read what a real scraper would read rather than reaching into the
// collector variables.
func scrape(t *testing.T, registry *prometheus.Registry) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	logger := zerolog.Nop()
	metricsHandlerFor(&logger, registry)(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q, want the Prometheus text format", contentType)
	}
	return recorder.Body.String()
}

// requireSeries asserts a counter sample with the given labels was emitted.
func requireSeries(t *testing.T, body, series string) {
	t.Helper()

	if !strings.Contains(body, series) {
		t.Errorf("metric series missing: %s", series)
	}
}

// TestMetricsMiddlewareCountsRequest checks a handled request lands in the
// counter and the histogram under its route template.
func TestMetricsMiddlewareCountsRequest(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/count-me", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/count-me", http.NoBody))

	body := scrape(t, registry)
	requireSeries(t, body, `proviant_http_requests_total{method="GET",route="/count-me",status="200"}`)
	requireSeries(t, body, `proviant_http_request_duration_seconds_count{method="GET",route="/count-me"}`)
}

// TestMetricsMiddlewareLabelsRouteTemplate proves the label is the matched
// route template rather than the raw path — labelling by path would give every
// product id its own time series and grow without bound.
func TestMetricsMiddlewareLabelsRouteTemplate(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/api/v1/products/:id", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/products/42", http.NoBody))

	body := scrape(t, registry)
	requireSeries(t, body, `proviant_http_requests_total{method="GET",route="/api/v1/products/:id",status="200"}`)
	if strings.Contains(body, `route="/api/v1/products/42"`) {
		t.Errorf("raw request path used as a route label:\n%s", body)
	}
}

// TestMetricsMiddlewareCollapsesUnmatchedRoutes guards the cardinality
// safeguard: requests that match no route share one label instead of one series
// per path. The label names the missing route, not the status, so a 404 alert
// stays meaningful once other statuses exist.
func TestMetricsMiddlewareCollapsesUnmatchedRoutes(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/known-route", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	for _, path := range []string{"/unknown-one", "/unknown-two"} {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, http.NoBody))
	}

	body := scrape(t, registry)
	requireSeries(t, body, `proviant_http_requests_total{method="GET",route="unmatched",status="404"}`)
	for _, path := range []string{"unknown-one", "unknown-two"} {
		if strings.Contains(body, path) {
			t.Errorf("unmatched path %q became its own label:\n%s", path, body)
		}
	}
}

// TestMetricsMiddlewareRecordsErrorStatus proves the status label reflects the
// written status, not a hardcoded 200 — an error-rate alert depends on it.
func TestMetricsMiddlewareRecordsErrorStatus(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/boom", func(ctx *gin.Context) { ctx.Status(http.StatusInternalServerError) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))

	requireSeries(t, scrape(t, registry), `proviant_http_requests_total{method="GET",route="/boom",status="500"}`)
}

// TestMetricsMiddlewareSkipsScrapeEndpoint keeps the scrape itself out of the
// request metrics, so the request rate does not follow the scrape interval.
func TestMetricsMiddlewareSkipsScrapeEndpoint(t *testing.T) {
	registry := newTestRegistry(t)
	logger := zerolog.Nop()

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/metrics", metricsHandlerFor(&logger, registry))
	engine.GET("/counted", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want %d", recorder.Code, http.StatusOK)
	}

	// The counter is incremented after ctx.Next(), so the body rendered above
	// was gathered before this scrape's own series could exist: asserting on it
	// passes whether or not the guard works. Scrape a second time, once a
	// recorded request proves the middleware is emitting series at all.
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/counted", http.NoBody))
	body := scrape(t, registry)

	if strings.Contains(body, `route="/metrics"`) {
		t.Errorf("scrape endpoint was counted as application traffic:\n%s", body)
	}
	requireSeries(t, body, `proviant_http_requests_total{method="GET",route="/counted",status="200"}`)
}

// TestMetricsMiddlewareCollapsesUnknownMethods guards the second cardinality
// safeguard. The route label is safe because it falls back to "unmatched", but
// net/http accepts any method token on the wire: without this, one request per
// junk method mints a counter series plus a histogram child per bucket, and the
// metrics endpoint is unauthenticated.
func TestMetricsMiddlewareCollapsesUnknownMethods(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry))
	engine.GET("/known-route", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	for _, method := range []string{"EVIL0", "EVIL1", "ZZTOP"} {
		req, err := http.NewRequest(method, "/known-route", http.NoBody)
		if err != nil {
			t.Fatalf("http.NewRequest(%q) error = %v", method, err)
		}
		engine.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := scrape(t, registry)
	// The junk methods match no route, so they land on the same "unmatched"
	// series the path safeguard already collapses them into.
	requireSeries(t, body, `proviant_http_requests_total{method="other",route="unmatched",status="404"}`)
	for _, method := range []string{"EVIL0", "EVIL1", "ZZTOP"} {
		if strings.Contains(body, `method="`+method+`"`) {
			t.Errorf("method %q became its own label:\n%s", method, body)
		}
	}
}

// TestMetricsMiddlewareCountsRecoveredPanic pins the middleware order: it must
// wrap gin.Recovery, otherwise the recovered panic skips the code after
// ctx.Next() and the 500 never reaches the counter.
func TestMetricsMiddlewareCountsRecoveredPanic(t *testing.T) {
	registry := newTestRegistry(t)

	engine := gin.New()
	engine.Use(metricsMiddlewareFor(registry), gin.Recovery())
	engine.GET("/panic", func(ctx *gin.Context) { panic("boom") })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", http.NoBody))

	requireSeries(t, scrape(t, registry), `proviant_http_requests_total{method="GET",route="/panic",status="500"}`)
}

// TestMetricsHandlerExposesRuntimeCollectors verifies the Go/process
// collectors are registered, so a slow endpoint can be told apart from a
// starved process.
func TestMetricsHandlerExposesRuntimeCollectors(t *testing.T) {
	body := scrape(t, newTestRegistry(t))

	for _, name := range []string{"go_goroutines", "process_cpu_seconds_total"} {
		if !strings.Contains(body, name) {
			t.Errorf("runtime metric %q missing from exposition", name)
		}
	}
}

// failingCollector stands in for a collector that cannot do its job — the
// process collector reading /proc on a locked-down host. Panicking is how a
// collector breaks the gather, and what prometheus turns into a gather error.
// Describe must send a real Desc: a collector without one is stored as
// "unchecked" and Unregister cannot find it again, which would leave this
// collector in the package-global registry for every later test.
type failingCollector struct{}

func (failingCollector) Describe(descriptions chan<- *prometheus.Desc) {
	descriptions <- prometheus.NewDesc("proviant_test_failing", "always fails", nil, nil)
}

func (failingCollector) Collect(chan<- prometheus.Metric) {
	panic("collector cannot read its source")
}

var testFailingCollector = failingCollector{}

// TestMetricsHandlerServesPartialScrapeOnCollectorError pins
// ErrorHandling: ContinueOnError. With the HTTPErrorOnError default, one
// broken collector answers 500 with an empty body, so a transient /proc
// failure erases every series — an alert on a flat counter fires, and a real
// outage hides behind a scrape error. Registry must be set as well, or the
// degradation is invisible: the operator gets a quietly missing series with
// neither a log line nor a counter to alert on.
func TestMetricsHandlerServesPartialScrapeOnCollectorError(t *testing.T) {
	registry := newTestRegistry(t)
	registry.MustRegister(testFailingCollector)
	t.Cleanup(func() {
		if !registry.Unregister(testFailingCollector) {
			t.Error("Unregister() = false, collector left in the shared registry")
		}
	})

	// The error counter is incremented after the gather that renders it, so the
	// scrape that hits the failure still reports 0 — and promhttp pre-creates
	// both children at 0 anyway, so its presence alone proves nothing. Only the
	// following scrape shows the failure was counted.
	scrape(t, registry)
	body := scrape(t, registry)

	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("working collectors dropped from the scrape:\n%s", body)
	}
	if !strings.Contains(body, `promhttp_metric_handler_errors_total{cause="gathering"} 1`) {
		t.Errorf("gathering failure not counted, degradation is invisible:\n%s", body)
	}
}
