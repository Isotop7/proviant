package metrics

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// documentedNames is the metric catalogue the README publishes. The names are
// a contract with every existing Grafana dashboard, so a rename or a dropped
// collector has to fail here first.
var documentedNames = []string{
	"proviant_backup_age_seconds",
	"proviant_db_connections",
	"proviant_db_connections_max",
	"proviant_db_wait_count_total",
	"proviant_db_wait_duration_seconds_total",
	"proviant_notification_attempts_total",
	"proviant_notification_failures_total",
	"proviant_notification_last_success_timestamp_seconds",
	"proviant_openfoodfacts_cache_lookups_total",
	"proviant_openfoodfacts_live_failures_total",
	"proviant_openfoodfacts_live_requests_total",
	"proviant_openfoodfacts_offline_lookups_total",
	"proviant_worker_attempts_total",
	"proviant_worker_failures_total",
	"proviant_worker_last_success_timestamp_seconds",
}

// resetCollectors clears the shared collector state so every test starts from
// zero. The collectors are package-level, so their values are shared by all
// tests in this package.
func resetCollectors(t *testing.T) {
	t.Helper()

	workerAttempts.Reset()
	workerFailures.Reset()
	workerLastSuccess.Reset()
	notificationAttempts.Reset()
	notificationFailures.Reset()
	notificationLastSuccess.Reset()
	cacheLookups.Reset()
	offlineLookups.Reset()
	backups.lastSuccess.Store(0)
}

// gatheredNames returns the metric names the registry currently exposes.
func gatheredNames(t *testing.T, registry *prometheus.Registry) map[string]bool {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	return names
}

// familyValue returns the summed value of every series of a metric family.
func familyValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		total := 0.0
		for _, series := range family.GetMetric() {
			if counter := series.GetCounter(); counter != nil {
				total += counter.GetValue()
				continue
			}
			total += series.GetGauge().GetValue()
		}
		return total
	}
	t.Fatalf("%s is not exposed", name)
	return 0
}

// labeledValue returns the value of the series of a metric family that carries
// the given label value.
func labeledValue(t *testing.T, registry *prometheus.Registry, name, label string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, series := range family.GetMetric() {
			for _, pair := range series.GetLabel() {
				if pair.GetValue() != label {
					continue
				}
				if counter := series.GetCounter(); counter != nil {
					return counter.GetValue()
				}
				return series.GetGauge().GetValue()
			}
		}
	}
	t.Fatalf("%s has no series labelled %q", name, label)
	return 0
}

// testPool returns a registry carrying the pool collector of an empty
// in-memory database, together with the pool it reports on.
func testPool(t *testing.T) (*prometheus.Registry, *sql.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	registry := prometheus.NewRegistry()
	registry.MustRegister(NewDatabaseCollector(pool))
	return registry, pool
}

// TestRegisterCollectorsExposesDocumentedNames pins the published catalogue:
// every documented metric has to be registered, and nothing else may carry the
// proviant_ prefix.
func TestRegisterCollectorsExposesDocumentedNames(t *testing.T) {
	resetCollectors(t)
	registry, _ := testPool(t)
	RegisterCollectors(registry)

	RecordWorkerAttempt("backup")
	RecordWorkerFailure("backup")
	RecordWorkerSuccess("backup")
	RecordDelivery("email", nil)
	RecordDelivery("email", errors.New("smtp refused"))
	RecordCacheLookup(true)
	RecordCacheLookup(false)
	RecordOfflineLookup(true)
	RecordLiveRequest()
	RecordLiveFailure()
	MarkBackupSuccess(time.Now())

	names := gatheredNames(t, registry)
	for _, name := range documentedNames {
		if !names[name] {
			t.Errorf("documented metric %q is not exposed by the registry", name)
		}
	}
	for name := range names {
		if !strings.HasPrefix(name, "proviant_") {
			t.Errorf("metric %q does not follow the proviant_ prefix convention", name)
		}
	}
}

// TestWorkerCountersRecordOutcomes proves a worker run shows up as an attempt
// and ends up either in the failure counter or in the last-success gauge.
func TestWorkerCountersRecordOutcomes(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	RecordWorkerAttempt("mail_digest")
	RecordWorkerFailure("mail_digest")
	RecordWorkerAttempt("mail_digest")
	RecordWorkerSuccess("mail_digest")

	if got := labeledValue(t, registry, "proviant_worker_attempts_total", "mail_digest"); got != 2 {
		t.Errorf("attempts = %v, want 2", got)
	}
	if got := labeledValue(t, registry, "proviant_worker_failures_total", "mail_digest"); got != 1 {
		t.Errorf("failures = %v, want 1", got)
	}
	if got := labeledValue(t, registry, "proviant_worker_last_success_timestamp_seconds", "mail_digest"); got <= 0 {
		t.Errorf("last success = %v, want a unix timestamp", got)
	}
}

// TestLastSuccessIsAbsentUntilTheWorkerSucceeds pins the "never ran" signal: a
// worker that has not succeeded yet has no series at all, because a zero would
// read as the epoch and fire a staleness alert on a fresh install.
func TestLastSuccessIsAbsentUntilTheWorkerSucceeds(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	RecordWorkerAttempt("backup")
	RecordWorkerFailure("backup")

	if names := gatheredNames(t, registry); names["proviant_worker_last_success_timestamp_seconds"] {
		t.Error("last-success gauge exists for a worker that never succeeded")
	}
}

// TestRecordDeliverySeparatesAttemptsFromFailures pins the provider contract:
// every send is an attempt, a failed send also counts a failure, and a
// successful one stamps the provider's last-success time.
func TestRecordDeliverySeparatesAttemptsFromFailures(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	RecordDelivery("email", errors.New("smtp refused"))
	RecordDelivery("email", nil)

	if got := labeledValue(t, registry, "proviant_notification_attempts_total", "email"); got != 2 {
		t.Errorf("attempts = %v, want 2", got)
	}
	if got := labeledValue(t, registry, "proviant_notification_failures_total", "email"); got != 1 {
		t.Errorf("failures = %v, want 1", got)
	}
	if got := labeledValue(t, registry, "proviant_notification_last_success_timestamp_seconds", "email"); got <= 0 {
		t.Errorf("last success = %v, want a unix timestamp", got)
	}
}

// TestLookupCountersSeparateHitsFromMisses pins the cache ratio the operator
// reads: a hit and a miss are counted apart, so the hit rate is a ratio of two
// counters instead of a gauge that never decays.
func TestLookupCountersSeparateHitsFromMisses(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	RecordCacheLookup(true)
	RecordCacheLookup(true)
	RecordCacheLookup(false)
	RecordOfflineLookup(true)

	if got := labeledValue(t, registry, "proviant_openfoodfacts_cache_lookups_total", "hit"); got != 2 {
		t.Errorf("cache hits = %v, want 2", got)
	}
	if got := labeledValue(t, registry, "proviant_openfoodfacts_cache_lookups_total", "miss"); got != 1 {
		t.Errorf("cache misses = %v, want 1", got)
	}
	if got := labeledValue(t, registry, "proviant_openfoodfacts_offline_lookups_total", "hit"); got != 1 {
		t.Errorf("offline hits = %v, want 1", got)
	}
}

// TestRecordCacheLookupBatchCountsEveryBarcode proves the bulk import path
// lands in the same hit rate as the single lookups.
func TestRecordCacheLookupBatchCountsEveryBarcode(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	RecordCacheLookupBatch(3, 2)

	if got := labeledValue(t, registry, "proviant_openfoodfacts_cache_lookups_total", "hit"); got != 3 {
		t.Errorf("cache hits = %v, want 3", got)
	}
	if got := labeledValue(t, registry, "proviant_openfoodfacts_cache_lookups_total", "miss"); got != 2 {
		t.Errorf("cache misses = %v, want 2", got)
	}
}

// TestBackupAgeCollectorStaysAbsentUntilFirstBackup proves the backup freshness
// gauge only appears once a backup was actually written.
func TestBackupAgeCollectorStaysAbsentUntilFirstBackup(t *testing.T) {
	resetCollectors(t)
	registry := prometheus.NewRegistry()
	RegisterCollectors(registry)

	if names := gatheredNames(t, registry); names["proviant_backup_age_seconds"] {
		t.Error("backup age is exposed before the first backup was written")
	}

	MarkBackupSuccess(time.Now())

	age := familyValue(t, registry, "proviant_backup_age_seconds")
	if age < 0 || age > 60 {
		t.Errorf("backup age = %v, want a fresh backup", age)
	}
}

// TestDatabaseCollectorReportsPoolState proves the pool collector reads the
// live connection statistics instead of reporting zeroes.
func TestDatabaseCollectorReportsPoolState(t *testing.T) {
	registry, pool := testPool(t)
	stats := pool.Stats()

	if got := labeledValue(t, registry, "proviant_db_connections", "open"); got != float64(stats.OpenConnections) {
		t.Errorf("open connections = %v, want %v", got, stats.OpenConnections)
	}
	if got := labeledValue(t, registry, "proviant_db_connections", "in_use"); got != float64(stats.InUse) {
		t.Errorf("in-use connections = %v, want %v", got, stats.InUse)
	}
	if got := labeledValue(t, registry, "proviant_db_connections", "idle"); got != float64(stats.Idle) {
		t.Errorf("idle connections = %v, want %v", got, stats.Idle)
	}
	if got := familyValue(t, registry, "proviant_db_connections_max"); got != float64(stats.MaxOpenConnections) {
		t.Errorf("max connections = %v, want %v", got, stats.MaxOpenConnections)
	}
}
