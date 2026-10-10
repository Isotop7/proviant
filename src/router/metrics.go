package router

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

var metricsRegistry = prometheus.NewRegistry()

func init() {
	metricsRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

type promLogger struct {
	logger *zerolog.Logger
}

func (p promLogger) Errorln(args ...any) {
	p.logger.Error().Msg(strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

func (p promLogger) Println(args ...any) {
	p.logger.Warn().Msg(strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

// MetricsHandler returns a Gin handler that exposes the Prometheus metrics
// scrape endpoint from the application registry.
// @Summary	Prometheus metrics
// @Description	Exposes the Prometheus scrape endpoint in the text exposition format. Served without authentication, so network access must be restricted to trusted scrapers
// @Tags		common
// @Produce	text/plain
// @Success	200	{string}	string
// @Router		/metrics [get]
func MetricsHandler(logger *zerolog.Logger) gin.HandlerFunc {
	return metricsHandlerFor(logger, metricsRegistry)
}

func metricsHandlerFor(logger *zerolog.Logger, registry *prometheus.Registry) gin.HandlerFunc {
	return gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		MaxRequestsInFlight: 2,
		Timeout:             5 * time.Second,
		ErrorHandling:       promhttp.ContinueOnError,
		Registry:            registry,
		ErrorLog:            promLogger{logger: logger},
	}))
}

// newRequestMetrics creates the request counter and duration histogram for a
// given registry. It is separate from the middleware constructor so the same
// shape is used for the production middleware and per-test registries.
func newRequestMetrics(registry prometheus.Registerer) (*prometheus.CounterVec, *prometheus.HistogramVec) {
	requestsTotal := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "proviant_http_requests_total",
			Help: "Total number of HTTP requests handled, by method, route and status.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration := promauto.With(registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proviant_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds, by method and route.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	return requestsTotal, requestDuration
}

// MetricsMiddleware records request counts and durations in the application
// registry. The counters are created once so repeated router setups do not
// panic with duplicate-collector errors.
var (
	metricsMiddlewareOnce sync.Once
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
)

func MetricsMiddleware() gin.HandlerFunc {
	metricsMiddlewareOnce.Do(func() {
		requestsTotal, requestDuration = newRequestMetrics(metricsRegistry)
	})

	return newMetricsMiddleware(requestsTotal, requestDuration)
}

func newMetricsMiddleware(requestsTotal *prometheus.CounterVec, requestDuration *prometheus.HistogramVec) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()

		// Counting scrapes would make the request rate follow the scrape interval.
		if ctx.FullPath() == util.RouteMetrics {
			return
		}

		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}

		requestsTotal.WithLabelValues(httpMethodLabel(ctx.Request.Method), route, strconv.Itoa(ctx.Writer.Status())).Inc()
		requestDuration.WithLabelValues(httpMethodLabel(ctx.Request.Method), route).Observe(time.Since(start).Seconds())
	}
}

// httpMethodLabel collapses the request method to a fixed set. net/http accepts
// any method token over the wire and the metrics endpoint is unauthenticated, so
// labelling by the raw method lets one request mint a counter series plus a
// histogram child per bucket — unbounded memory growth on a public endpoint.
// Anything outside the known set is reported as "other".
func httpMethodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "other"
	}
}
