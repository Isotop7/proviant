package database

import (
	"testing"

	"github.com/Isotop7/proviant/metrics"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/prometheus/client_golang/prometheus"
)

// lookupCount returns the value of the cache lookup counter that carries the
// given result label.
func lookupCount(t *testing.T, registry *prometheus.Registry, result string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != "proviant_openfoodfacts_cache_lookups_total" {
			continue
		}
		for _, series := range family.GetMetric() {
			for _, pair := range series.GetLabel() {
				if pair.GetValue() == result {
					return series.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

// TestProductRepository_BulkCacheLookupIsCounted proves the bulk import path
// lands in the same cache hit rate as the single lookups: without it the hit
// rate would only describe the scan path and hide the import's cache misses.
func TestProductRepository_BulkCacheLookupIsCounted(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics.RegisterCollectors(registry)

	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	entry := dbModel.OpenFoodFactsCache{Barcode: "1234567890123", ProductName: "Cached Product"}
	if err := repo.CreateOpenFoodFactsCache(&entry); err != nil {
		t.Fatalf("CreateOpenFoodFactsCache() error = %v", err)
	}

	hitsBefore := lookupCount(t, registry, "hit")
	missesBefore := lookupCount(t, registry, "miss")

	if _, err := repo.GetOpenFoodFactsCachesByBarcodes([]string{"1234567890123", "9999999999999"}); err != nil {
		t.Fatalf("GetOpenFoodFactsCachesByBarcodes() error = %v", err)
	}

	if got := lookupCount(t, registry, "hit"); got != hitsBefore+1 {
		t.Errorf("cache hits = %v, want %v", got, hitsBefore+1)
	}
	if got := lookupCount(t, registry, "miss"); got != missesBefore+1 {
		t.Errorf("cache misses = %v, want %v", got, missesBefore+1)
	}
}

// TestProductRepository_BulkCacheLookupCountsDuplicatesOnce proves a barcode
// listed twice is one lookup: the hit rate counts barcodes, not rows.
func TestProductRepository_BulkCacheLookupCountsDuplicatesOnce(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics.RegisterCollectors(registry)

	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	entry := dbModel.OpenFoodFactsCache{Barcode: "1234567890123", ProductName: "Cached Product"}
	if err := repo.CreateOpenFoodFactsCache(&entry); err != nil {
		t.Fatalf("CreateOpenFoodFactsCache() error = %v", err)
	}

	hitsBefore := lookupCount(t, registry, "hit")
	missesBefore := lookupCount(t, registry, "miss")

	if _, err := repo.GetOpenFoodFactsCachesByBarcodes([]string{"1234567890123", "1234567890123"}); err != nil {
		t.Fatalf("GetOpenFoodFactsCachesByBarcodes() error = %v", err)
	}

	if got := lookupCount(t, registry, "hit"); got != hitsBefore+1 {
		t.Errorf("cache hits = %v, want %v", got, hitsBefore+1)
	}
	if got := lookupCount(t, registry, "miss"); got != missesBefore {
		t.Errorf("cache misses = %v, want %v", got, missesBefore)
	}
}
