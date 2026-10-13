package database

import (
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestRecipeRepository_HasPopulatedCache(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	seed := func(t *testing.T, hash, response string, expiresAt time.Time) dbModel.RecipeCache {
		t.Helper()
		entry := dbModel.RecipeCache{
			QueryHash:    hash,
			Provider:     "themealdb",
			ResponseJSON: []byte(response),
			ExpiresAt:    expiresAt,
		}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		return entry
	}

	t.Run("no entry is unpopulated", func(t *testing.T) {
		populated, err := repo.HasPopulatedCache("missing-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for missing entry")
		}
	})

	t.Run("entry with real suggestions is populated", func(t *testing.T) {
		seed(t, "populated-hash", `[{"ingredients":["milk"],"totalIngredients":1,"matchedProductIds":[1]}]`, time.Now().Add(time.Hour))
		populated, err := repo.HasPopulatedCache("populated-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if !populated {
			t.Error("HasPopulatedCache() = false, want true")
		}
	})

	t.Run("expired entry is unpopulated", func(t *testing.T) {
		seed(t, "expired-hash", `[{"ingredients":["milk"],"totalIngredients":1,"matchedProductIds":[1]}]`, time.Now().Add(-time.Hour))
		populated, err := repo.HasPopulatedCache("expired-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for expired entry")
		}
	})

	t.Run("stale payload without matched ids is unpopulated", func(t *testing.T) {
		seed(t, "stale-hash", `[{"ingredients":["milk"],"totalIngredients":1}]`, time.Now().Add(time.Hour))
		populated, err := repo.HasPopulatedCache("stale-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for stale payload")
		}
	})

	t.Run("empty ingredients against non-zero total is unpopulated", func(t *testing.T) {
		seed(t, "empty-ingredients-hash", `[{"ingredients":[],"totalIngredients":2,"matchedProductIds":[1]}]`, time.Now().Add(time.Hour))
		populated, err := repo.HasPopulatedCache("empty-ingredients-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for empty ingredients")
		}
	})

	t.Run("unparseable payload is unpopulated", func(t *testing.T) {
		seed(t, "broken-hash", `not json`, time.Now().Add(time.Hour))
		populated, err := repo.HasPopulatedCache("broken-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for unparseable payload")
		}
	})

	t.Run("empty suggestion list is unpopulated", func(t *testing.T) {
		seed(t, "empty-list-hash", `[]`, time.Now().Add(time.Hour))
		populated, err := repo.HasPopulatedCache("empty-list-hash")
		if err != nil {
			t.Fatalf("HasPopulatedCache() error = %v", err)
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false for empty list")
		}
	})
}

func TestRecipeRepository_CreateCacheUpsert(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	entry := dbModel.RecipeCache{
		QueryHash:    "upsert-hash",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[{"title":"Old"}]`),
		ExpiresAt:    time.Now().Add(time.Hour),
		HitCount:     7,
	}
	if err := repo.CreateCache(&entry); err != nil {
		t.Fatalf("CreateCache() error = %v", err)
	}

	// A second CreateCache with the same hash overwrites the entry and
	// resets the hit count.
	replacement := dbModel.RecipeCache{
		QueryHash:    "upsert-hash",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[{"title":"New"}]`),
		ExpiresAt:    time.Now().Add(2 * time.Hour),
	}
	if err := repo.CreateCache(&replacement); err != nil {
		t.Fatalf("CreateCache() upsert error = %v", err)
	}

	var count int64
	db.Model(&dbModel.RecipeCache{}).Where("query_hash = ?", "upsert-hash").Count(&count)
	if count != 1 {
		t.Fatalf("upsert left %d rows, want 1", count)
	}

	found, err := repo.GetCacheByQueryHash("upsert-hash")
	if err != nil {
		t.Fatalf("GetCacheByQueryHash() error = %v", err)
	}
	if string(found.ResponseJSON) != `[{"title":"New"}]` {
		t.Errorf("ResponseJSON = %s, want overwritten payload", found.ResponseJSON)
	}
	if found.HitCount != 0 {
		t.Errorf("HitCount = %d, want 0 after upsert reset", found.HitCount)
	}
}

func TestRecipeRepository_CleanupExpiredCaches(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	expired := dbModel.RecipeCache{
		QueryHash:    "cleanup-expired",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[]`),
		ExpiresAt:    time.Now().Add(-time.Hour),
	}
	valid := dbModel.RecipeCache{
		QueryHash:    "cleanup-valid",
		Provider:     "themealdb",
		ResponseJSON: []byte(`[]`),
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := db.Create(&expired).Error; err != nil {
		t.Fatalf("seed expired: %v", err)
	}
	if err := db.Create(&valid).Error; err != nil {
		t.Fatalf("seed valid: %v", err)
	}

	if err := repo.CleanupExpiredCaches(); err != nil {
		t.Fatalf("CleanupExpiredCaches() error = %v", err)
	}

	// Hard delete: the row must be gone even with Unscoped off.
	var count int64
	db.Unscoped().Model(&dbModel.RecipeCache{}).Where("query_hash = ?", "cleanup-expired").Count(&count)
	if count != 0 {
		t.Error("expired cache entry survived CleanupExpiredCaches")
	}
	db.Unscoped().Model(&dbModel.RecipeCache{}).Where("query_hash = ?", "cleanup-valid").Count(&count)
	if count != 1 {
		t.Error("valid cache entry was deleted by CleanupExpiredCaches")
	}

	// Cleanup on an empty table is a no-op.
	if err := repo.CleanupExpiredCaches(); err != nil {
		t.Errorf("CleanupExpiredCaches() on empty table error = %v", err)
	}
}
