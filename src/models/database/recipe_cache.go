package database

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// RecipeCache stores cached recipe suggestions globally.
// The cache key is a hash of the provider and the sorted product IDs.
type RecipeCache struct {
	gorm.Model
	QueryHash    string    `gorm:"uniqueIndex;not null" json:"-"`
	Provider     string    `json:"provider"`
	ResponseJSON []byte    `json:"-"` // store raw JSON; not exposed via API
	ExpiresAt    time.Time `json:"expires_at"`
	HitCount     int       `gorm:"default:0" json:"-"`
}

// GenerateCacheKey creates a deterministic SHA256 hash from provider name and sorted product IDs.
func GenerateCacheKey(provider string, productIDs []uint) string {
	h := sha256.New()
	h.Write([]byte(provider))
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
