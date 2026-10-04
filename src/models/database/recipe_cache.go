package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// RecipeCache stores cached recipe suggestions globally.
// The cache key is a hash of the provider, its base URL and the sorted product IDs.
type RecipeCache struct {
	gorm.Model
	QueryHash    string    `gorm:"uniqueIndex;not null" json:"-"`
	Provider     string    `json:"provider"`
	ResponseJSON []byte    `json:"-"` // store raw JSON; not exposed via API
	ExpiresAt    time.Time `json:"expires_at"`
	HitCount     int       `gorm:"default:0" json:"-"`
}

// GenerateCacheKey creates a deterministic SHA256 hash from provider name,
// provider base URL, the household product set fingerprint and the sorted
// expiring product IDs. The URL is part of the key so a repointed provider
// instance never serves entries from the previous one. The fingerprint is part
// of it because a cached suggestion carries the IDs of the products that
// matched it, and the client turns those IDs into an irreversible consume: an
// entry computed against a product set that has since changed must not be
// served, even though the expiring set feeding the search is unchanged.
func GenerateCacheKey(provider, baseURL, productSetFingerprint string, productIDs []uint) string {
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte{':'})
	h.Write([]byte(baseURL))
	h.Write([]byte{':'})
	h.Write([]byte(productSetFingerprint))
	for _, id := range productIDs {
		h.Write([]byte{':'})
		h.Write([]byte(strconv.FormatUint(uint64(id), 10)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// IsExpired returns true if the cache entry has passed its expiry time.
func (r *RecipeCache) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// cachedSuggestionFields is the subset of a cached suggestion the staleness
// rules read. It deliberately mirrors the JSON field names of
// controllers.RecipeSuggestion instead of importing it: the database package
// cannot import controllers, and a payload shape change would silently turn
// into "no suggestions" here rather than a compile error.
type cachedSuggestionFields struct {
	Ingredients       []json.RawMessage `json:"ingredients"`
	TotalIngredients  int               `json:"totalIngredients"`
	MatchedProductIDs []uint            `json:"matchedProductIds"`
}

// HasStaleSuggestions reports whether the stored payload must not be served.
// It is the single validity predicate shared by the cache read path and the
// guarded write path: two hand-maintained copies disagree whenever one learns
// a staleness rule the other does not, and the disagreement is silent — the
// read refreshes an entry the write then refuses to repair, so every later
// request repeats the provider fan-out until the entry's TTL elapses.
//
// The rules, and why each exists:
//   - empty ingredients against a non-zero total: the pre-change payload shape
//     did not store them at all.
//   - nil MatchedProductIDs: likewise absent before the cook action existed, so
//     such an entry carries no product IDs and cannot authorize a consume.
func (r *RecipeCache) HasStaleSuggestions() (bool, error) {
	var suggestions []cachedSuggestionFields
	if err := json.Unmarshal(r.ResponseJSON, &suggestions); err != nil {
		return false, err
	}
	for i := range suggestions {
		if len(suggestions[i].Ingredients) == 0 && suggestions[i].TotalIngredients > 0 {
			return true, nil
		}
		if suggestions[i].MatchedProductIDs == nil {
			return true, nil
		}
	}
	return false, nil
}
