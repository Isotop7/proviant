package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

// UpdateCacheHit increments the hit count for a cache entry.
func (r *RecipeRepository) UpdateCacheHit(hash string) error {
	return r.DB.Model(&database.RecipeCache{}).Where("query_hash = ?", hash).UpdateColumn("hit_count", gorm.Expr("hit_count + ?", 1)).Error
}

// CleanupExpiredCaches deletes all cache entries where expires_at < now.
func (r *RecipeRepository) CleanupExpiredCaches() error {
	return r.DB.Where("expires_at < ?", time.Now()).Delete(&database.RecipeCache{}).Error
}
