package database

import "testing"

// The cache key must separate entries that share a provider, URL and expiring
// product set but were computed against different product sets. Without the
// fingerprint an entry written before a rename or a delete is served
// afterwards, and its MatchedProductIDs then authorize consuming a product the
// match no longer justifies.
func TestGenerateCacheKey_ProductSetFingerprintSeparatesEntries(t *testing.T) {
	ids := []uint{7, 3, 11}

	base := GenerateCacheKey("themealdb", "https://example.test/api", "3:2026-10-03", ids)

	renamed := GenerateCacheKey("themealdb", "https://example.test/api", "3:2026-10-04", ids)
	if renamed == base {
		t.Error("key unchanged after the product set changed, so a stale entry could be served")
	}

	grown := GenerateCacheKey("themealdb", "https://example.test/api", "4:2026-10-03", ids)
	if grown == base {
		t.Error("key unchanged after a product was added")
	}

	otherURL := GenerateCacheKey("themealdb", "https://other.test/api", "3:2026-10-03", ids)
	if otherURL == base {
		t.Error("key unchanged after the provider URL was repointed")
	}
}
