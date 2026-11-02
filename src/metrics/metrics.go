// metrics holds the application's Prometheus collectors.
//
// The registry lives in src/router/metrics.go; this package holds the
// collector definitions so the background workers in src/controllers can
// record their outcomes without importing the router, which imports them back.
package metrics

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Label sets. Every label value comes from one of these lists, so the number
// of series per metric is fixed:
//
//	worker:   notification_dispatch, invitation_retry, mail_digest,
//	          monthly_waste_report, streak_updates, telegram_poller,
//	          backup, offdump_import
//	provider: email, ntfy, telegram, webpush
//	result:   hit, miss
//	state:    open, in_use, idle
//
// A label carrying a product name, a household id, an e-mail address or a
// Telegram chat id would mint one series per user and grow the registry
// without bound. The worker and provider values are constants in
// src/util/util.go, so a typo is a compile error instead of a new series.
const (
	lookupResultHit  = "hit"
	lookupResultMiss = "miss"

	poolStateOpen  = "open"
	poolStateInUse = "in_use"
	poolStateIdle  = "idle"
)

var (
	workerAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_worker_attempts_total",
		Help: "Runs of a background worker that were started, by worker.",
	}, []string{"worker"})

	workerFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_worker_failures_total",
		Help: "Runs of a background worker that ended in an error, by worker.",
	}, []string{"worker"})

	workerLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "proviant_worker_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful run of a background worker, by worker. The series is absent until the worker succeeds once.",
	}, []string{"worker"})

	notificationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_notification_attempts_total",
		Help: "Notification sends that were attempted, by provider.",
	}, []string{"provider"})

	notificationFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_notification_failures_total",
		Help: "Notification sends that failed, by provider.",
	}, []string{"provider"})

	notificationLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "proviant_notification_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful notification send, by provider. The series is absent until a send succeeds once.",
	}, []string{"provider"})

	cacheLookups = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_openfoodfacts_cache_lookups_total",
		Help: "Open Food Facts database cache lookups, by result.",
	}, []string{"result"})

	offlineLookups = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proviant_openfoodfacts_offline_lookups_total",
		Help: "Open Food Facts offline dump lookups, by result.",
	}, []string{"result"})

	liveRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proviant_openfoodfacts_live_requests_total",
		Help: "Live Open Food Facts API requests that were issued.",
	})

	liveFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proviant_openfoodfacts_live_failures_total",
		Help: "Live Open Food Facts API requests that failed.",
	})
)

// RecordWorkerAttempt counts one started run of a background worker.
func RecordWorkerAttempt(worker string) {
	workerAttempts.WithLabelValues(worker).Inc()
}

// RecordWorkerSuccess counts a successful run of a background worker and
// stamps the time of that success.
func RecordWorkerSuccess(worker string) {
	workerLastSuccess.WithLabelValues(worker).Set(float64(time.Now().Unix()))
}

// RecordWorkerFailure counts a background worker run that ended in an error.
func RecordWorkerFailure(worker string) {
	workerFailures.WithLabelValues(worker).Inc()
}

// RecordDelivery counts one notification send attempt and its outcome: a
// failed send increments the failure counter, a successful one stamps the
// provider's last-success time.
func RecordDelivery(provider string, err error) {
	notificationAttempts.WithLabelValues(provider).Inc()
	if err != nil {
		notificationFailures.WithLabelValues(provider).Inc()
		return
	}
	notificationLastSuccess.WithLabelValues(provider).Set(float64(time.Now().Unix()))
}

// RecordCacheLookup counts one Open Food Facts database cache lookup.
func RecordCacheLookup(hit bool) {
	if hit {
		RecordCacheLookupBatch(1, 0)
		return
	}
	RecordCacheLookupBatch(0, 1)
}

// RecordCacheLookupBatch counts a batch of Open Food Facts database cache
// lookups, so the bulk import path lands in the same hit rate as the single
// lookups.
func RecordCacheLookupBatch(hits, misses int) {
	if hits > 0 {
		cacheLookups.WithLabelValues(lookupResultHit).Add(float64(hits))
	}
	if misses > 0 {
		cacheLookups.WithLabelValues(lookupResultMiss).Add(float64(misses))
	}
}

// RecordOfflineLookup counts one Open Food Facts offline dump lookup.
func RecordOfflineLookup(hit bool) {
	offlineLookups.WithLabelValues(lookupResult(hit)).Inc()
}

// RecordLiveRequest counts one live Open Food Facts API request.
func RecordLiveRequest() {
	liveRequests.Inc()
}

// RecordLiveFailure counts one live Open Food Facts API request that failed.
func RecordLiveFailure() {
	liveFailures.Inc()
}

func lookupResult(hit bool) string {
	if hit {
		return lookupResultHit
	}
	return lookupResultMiss
}

// backups tracks the newest backup the household backup job has written.
var backups = &backupAgeCollector{}

var backupAgeDesc = prometheus.NewDesc(
	"proviant_backup_age_seconds",
	"Age of the newest successful household backup in seconds. The series is absent until the first backup is written.",
	nil, nil,
)

// MarkBackupSuccess records the time of the newest written backup.
func MarkBackupSuccess(at time.Time) {
	backups.lastSuccess.Store(at.Unix())
}

// backupAgeCollector exports how old the newest backup is. The timestamp is
// pushed by the backup controller, the age is computed at scrape time so the
// gauge never goes stale between two scrapes.
type backupAgeCollector struct {
	lastSuccess atomic.Int64
}

func (c *backupAgeCollector) Describe(descriptions chan<- *prometheus.Desc) {
	descriptions <- backupAgeDesc
}

func (c *backupAgeCollector) Collect(metrics chan<- prometheus.Metric) {
	lastSuccess := c.lastSuccess.Load()
	if lastSuccess == 0 {
		return
	}
	metrics <- prometheus.MustNewConstMetric(
		backupAgeDesc,
		prometheus.GaugeValue,
		float64(time.Now().Unix()-lastSuccess),
	)
}

var (
	dbConnectionsDesc = prometheus.NewDesc(
		"proviant_db_connections",
		"Connections in the database pool, by state.",
		[]string{"state"}, nil,
	)

	dbConnectionsMaxDesc = prometheus.NewDesc(
		"proviant_db_connections_max",
		"Maximum number of open connections the database pool allows. Zero means the pool is unlimited.",
		nil, nil,
	)

	dbWaitCountDesc = prometheus.NewDesc(
		"proviant_db_wait_count_total",
		"Total number of connection requests that had to wait for a free connection.",
		nil, nil,
	)

	dbWaitDurationDesc = prometheus.NewDesc(
		"proviant_db_wait_duration_seconds_total",
		"Total time connection requests spent waiting for a free connection.",
		nil, nil,
	)
)

// NewDatabaseCollector returns a collector for the connection pool of db. The
// pool statistics are read at scrape time, so the gauges never go stale.
func NewDatabaseCollector(db *sql.DB) prometheus.Collector {
	return &databaseCollector{db: db}
}

type databaseCollector struct {
	db *sql.DB
}

func (c *databaseCollector) Describe(descriptions chan<- *prometheus.Desc) {
	descriptions <- dbConnectionsDesc
	descriptions <- dbConnectionsMaxDesc
	descriptions <- dbWaitCountDesc
	descriptions <- dbWaitDurationDesc
}

func (c *databaseCollector) Collect(metrics chan<- prometheus.Metric) {
	stats := c.db.Stats()
	metrics <- prometheus.MustNewConstMetric(dbConnectionsDesc, prometheus.GaugeValue, float64(stats.OpenConnections), poolStateOpen)
	metrics <- prometheus.MustNewConstMetric(dbConnectionsDesc, prometheus.GaugeValue, float64(stats.InUse), poolStateInUse)
	metrics <- prometheus.MustNewConstMetric(dbConnectionsDesc, prometheus.GaugeValue, float64(stats.Idle), poolStateIdle)
	metrics <- prometheus.MustNewConstMetric(dbConnectionsMaxDesc, prometheus.GaugeValue, float64(stats.MaxOpenConnections))
	metrics <- prometheus.MustNewConstMetric(dbWaitCountDesc, prometheus.CounterValue, float64(stats.WaitCount))
	metrics <- prometheus.MustNewConstMetric(dbWaitDurationDesc, prometheus.CounterValue, stats.WaitDuration.Seconds())
}

// RegisterCollectors registers the application collectors into reg. The
// registry is a dedicated prometheus.NewRegistry, so only the collectors
// registered here are exposed on the scrape endpoint.
func RegisterCollectors(reg prometheus.Registerer) {
	reg.MustRegister(
		workerAttempts,
		workerFailures,
		workerLastSuccess,
		notificationAttempts,
		notificationFailures,
		notificationLastSuccess,
		cacheLookups,
		offlineLookups,
		liveRequests,
		liveFailures,
		backups,
	)
}
