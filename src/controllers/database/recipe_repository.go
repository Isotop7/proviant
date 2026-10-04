package database

import (
	"encoding/json"
	"errors"
	"time"

	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RecipeRepositoryInterface interface {
	GetCacheByQueryHash(hash string) (database.RecipeCache, error)
	CreateCache(cache *database.RecipeCache) error
	HasPopulatedCache(hash string) (bool, error)
	UpdateCacheHit(hash string) error
	CleanupExpiredCaches() error
}

var _ RecipeRepositoryInterface = (*RecipeRepository)(nil)

type RecipeRepository struct {
	DB *gorm.DB
}

func NewRecipeRepository(db *gorm.DB) *RecipeRepository {
	return &RecipeRepository{DB: db}
}

// GetCacheByQueryHash retrieves a cache entry by its query hash if not expired.
func (r *RecipeRepository) GetCacheByQueryHash(hash string) (database.RecipeCache, error) {
	var cache database.RecipeCache
	err := r.DB.Where("query_hash = ? AND expires_at > ?", hash, time.Now()).First(&cache).Error
	return cache, err
}

// CreateCache inserts or replaces a cache entry atomically using ON CONFLICT (upsert).
// If an entry with the same query_hash exists, it is overwritten (resetting hit_count and updated_at).
func (r *RecipeRepository) CreateCache(cache *database.RecipeCache) error {
	// gorm.io/clause OnConflict generates INSERT ... ON CONFLICT DO UPDATE
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "query_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{"response_json", "expires_at", "provider", "hit_count", "updated_at"}),
	}).Create(cache).Error
}

// HasPopulatedCache reports whether hash already has an unexpired entry holding
// real suggestions. CreateCache is an unconditional upsert, so a caller about to
// store an empty or partial result calls this first: without it, one degraded
// run would overwrite a good entry and the household would see the degraded
// answer for the remainder of that entry's TTL.
//
// Staleness is judged by RecipeCache.HasStaleSuggestions, the same predicate
// the read path uses. Anything the read path refuses to serve must also read as
// unpopulated here, or a degraded run could not repair the entry it just called
// stale and every later request would repeat the provider fan-out until the TTL
// elapsed.
func (r *RecipeRepository) HasPopulatedCache(hash string) (bool, error) {
	cache, err := r.GetCacheByQueryHash(hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	// A payload that no longer unmarshals is treated as absent rather than as
	// a reason to store over it: the caller's own write is the repair.
	stale, staleErr := cache.HasStaleSuggestions()
	if staleErr != nil {
		return false, nil
	}
	if stale {
		return false, nil
	}
	var suggestions []json.RawMessage
	if unmarshalErr := json.Unmarshal(cache.ResponseJSON, &suggestions); unmarshalErr != nil {
		return false, nil
	}
	return len(suggestions) > 0, nil
}

// UpdateCacheHit increments the hit count for a cache entry.
func (r *RecipeRepository) UpdateCacheHit(hash string) error {
	return r.DB.Model(&database.RecipeCache{}).Where("query_hash = ?", hash).UpdateColumn("hit_count", gorm.Expr("hit_count + ?", 1)).Error
}

// CleanupExpiredCaches deletes all cache entries where expires_at < now.
// Unscoped hard delete: soft-deleted tombstones would keep occupying the
// query_hash unique index and redirect later CreateCache upserts into an
// invisible row.
func (r *RecipeRepository) CleanupExpiredCaches() error {
	return r.DB.Unscoped().Where("expires_at < ?", time.Now()).Delete(&database.RecipeCache{}).Error
}
