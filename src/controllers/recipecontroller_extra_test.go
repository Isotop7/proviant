package controllers

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func TestRecipeSuggestionToAPIResponse(t *testing.T) {
	suggestion := RecipeSuggestion{
		ID:                "r1",
		Title:             "Chicken Rice",
		ImageURL:          "https://img.example.com/r1.jpg",
		SourceURL:         "https://recipes.example.com/r1",
		Ingredients:       []api.IngredientMatch{{Name: "chicken", Matched: true}},
		MatchedProducts:   []string{"chicken"},
		MatchedProductIDs: []uint{7},
		MissingCount:      1,
		TotalIngredients:  2,
		MatchPercent:      50,
		ExpiryPoints:      3, // internal, must not leak into the response
	}
	resp := suggestion.ToAPIResponse()
	if resp.ID != "r1" || resp.Title != "Chicken Rice" || resp.ImageURL != suggestion.ImageURL || resp.SourceURL != suggestion.SourceURL {
		t.Errorf("resp = %+v, want the mapped fields", resp)
	}
	if len(resp.Ingredients) != 1 || resp.Ingredients[0].Name != "chicken" {
		t.Errorf("Ingredients = %+v", resp.Ingredients)
	}
	if len(resp.MatchedProducts) != 1 || resp.MatchedProductIDs[0] != 7 {
		t.Errorf("MatchedProducts = %+v, MatchedProductIDs = %+v", resp.MatchedProducts, resp.MatchedProductIDs)
	}
	if resp.MissingCount != 1 || resp.TotalIngredients != 2 || resp.MatchPercent != 50 {
		t.Errorf("resp = %+v, want counts 1/2 and 50%%", resp)
	}
}

func TestSanitizeRecipeURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https kept", raw: "https://recipes.example.com/r/1", want: "https://recipes.example.com/r/1"},
		{name: "http kept", raw: "http://recipes.example.com/r/1", want: "http://recipes.example.com/r/1"},
		{name: "surrounding whitespace trimmed", raw: "  https://recipes.example.com/r/1  ", want: "https://recipes.example.com/r/1"},
		{name: "empty", raw: "", want: ""},
		{name: "whitespace only", raw: "   ", want: ""},
		{name: "ftp scheme rejected", raw: "ftp://recipes.example.com/r/1", want: ""},
		{name: "javascript scheme rejected", raw: "javascript:alert(1)", want: ""},
		{name: "relative path rejected", raw: "/local/path", want: ""},
		{name: "parse error rejected", raw: "://bad url", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRecipeURL(tt.raw); got != tt.want {
				t.Errorf("sanitizeRecipeURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNewRecipeController(t *testing.T) {
	logger := zerolog.Nop()
	db := testutil.SetupTestDB(t)
	baseConfig := configuration.RecipeAPIConfiguration{
		Provider: util.RecipeProviderThemealDB,
		URL:      "https://www.themealdb.com/api/json/v1/1",
		Timeout:  10,
	}

	t.Run("disabled short-circuits provider setup", func(t *testing.T) {
		ctrl := NewRecipeController(baseConfig, &logger, db, true)
		if !ctrl.disabled {
			t.Error("disabled = false, want true")
		}
		if ctrl.Provider != nil {
			t.Error("Provider set, want nil when disabled")
		}
		if ctrl.RecipeRepo == nil {
			t.Error("RecipeRepo not initialized")
		}
	})

	t.Run("known provider is wired", func(t *testing.T) {
		ctrl := NewRecipeController(baseConfig, &logger, db, false)
		if ctrl.Provider == nil {
			t.Fatal("Provider = nil, want the themealdb provider")
		}
		if got := ctrl.Provider.Name(); got != util.RecipeProviderThemealDB {
			t.Errorf("Provider.Name() = %q, want %q", got, util.RecipeProviderThemealDB)
		}
	})

	t.Run("unknown provider degrades to nil", func(t *testing.T) {
		config := baseConfig
		config.Provider = "bogus"
		ctrl := NewRecipeController(config, &logger, db, false)
		if ctrl.Provider != nil {
			t.Error("Provider set, want nil for an unknown provider")
		}
	})
}

func validCacheEntry(t *testing.T, ttl time.Duration) dbModel.RecipeCache {
	t.Helper()
	payload, err := json.Marshal([]RecipeSuggestion{{
		ID:                "cached",
		Title:             "Cached Recipe",
		Ingredients:       []api.IngredientMatch{{Name: "chicken", Matched: true}},
		MatchedProducts:   []string{"chicken"},
		MatchedProductIDs: []uint{1},
		TotalIngredients:  1,
		MatchPercent:      100,
	}})
	if err != nil {
		t.Fatalf("marshal cache payload: %v", err)
	}
	return dbModel.RecipeCache{QueryHash: "hash", ResponseJSON: payload, ExpiresAt: time.Now().Add(ttl)}
}

func TestTryGetCache(t *testing.T) {
	newCtrl := func(cacheEnabled bool, repo *repomocks.MockRecipeRepository) *RecipeController {
		logger := zerolog.Nop()
		return &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{CacheEnabled: cacheEnabled},
			RecipeRepo: repo,
		}
	}

	t.Run("cache disabled misses", func(t *testing.T) {
		ctrl := newCtrl(false, &repomocks.MockRecipeRepository{})
		if _, hit := ctrl.tryGetCache("hash"); hit {
			t.Error("tryGetCache() hit, want miss when caching is disabled")
		}
	})

	t.Run("repository error misses", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{Err: errors.New("db down")}
		ctrl := newCtrl(true, repo)
		if _, hit := ctrl.tryGetCache("hash"); hit {
			t.Error("tryGetCache() hit, want miss on repository error")
		}
	})

	t.Run("expired entry misses", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{Cache: validCacheEntry(t, -time.Hour)}
		ctrl := newCtrl(true, repo)
		if _, hit := ctrl.tryGetCache("hash"); hit {
			t.Error("tryGetCache() hit, want miss for an expired entry")
		}
	})

	t.Run("unparseable payload misses", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{Cache: dbModel.RecipeCache{ResponseJSON: []byte("{broken"), ExpiresAt: time.Now().Add(time.Hour)}}
		ctrl := newCtrl(true, repo)
		if _, hit := ctrl.tryGetCache("hash"); hit {
			t.Error("tryGetCache() hit, want miss for an unparseable payload")
		}
	})

	t.Run("stale entry misses", func(t *testing.T) {
		// Pre-change payload shape: ingredients absent but total > 0.
		repo := &repomocks.MockRecipeRepository{Cache: dbModel.RecipeCache{
			ResponseJSON: []byte(`[{"totalIngredients":2}]`),
			ExpiresAt:    time.Now().Add(time.Hour),
		}}
		ctrl := newCtrl(true, repo)
		if _, hit := ctrl.tryGetCache("hash"); hit {
			t.Error("tryGetCache() hit, want miss for a stale entry")
		}
	})

	t.Run("valid entry hits", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{Cache: validCacheEntry(t, time.Hour)}
		ctrl := newCtrl(true, repo)
		suggestions, hit := ctrl.tryGetCache("hash")
		if !hit {
			t.Fatal("tryGetCache() miss, want hit for a valid entry")
		}
		if len(suggestions) != 1 || suggestions[0].ID != "cached" {
			t.Errorf("suggestions = %+v, want the cached recipe", suggestions)
		}
	})
}

func TestStoreCacheWithTTL(t *testing.T) {
	suggestions := []RecipeSuggestion{{ID: "r1", Title: "T"}}

	t.Run("cache disabled writes nothing", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{CacheEnabled: false},
			RecipeRepo: repo,
		}
		ctrl.storeCacheWithTTL("hash", suggestions, time.Hour)
		if len(repo.CacheWrites) != 0 {
			t.Errorf("cache writes = %d, want 0 when caching is disabled", len(repo.CacheWrites))
		}
	})

	t.Run("repository error is logged", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{Err: errors.New("db down")}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{CacheEnabled: true, Provider: util.RecipeProviderThemealDB},
			RecipeRepo: repo,
		}
		ctrl.storeCacheWithTTL("hash", suggestions, time.Hour)
	})

	t.Run("entry stored with TTL", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{CacheEnabled: true, Provider: util.RecipeProviderThemealDB},
			RecipeRepo: repo,
		}
		before := time.Now()
		ctrl.storeCacheWithTTL("hash", suggestions, 2*time.Hour)
		if len(repo.CacheWrites) != 1 {
			t.Fatalf("cache writes = %d, want 1", len(repo.CacheWrites))
		}
		entry := repo.CacheWrites[0]
		if entry.QueryHash != "hash" || entry.Provider != util.RecipeProviderThemealDB {
			t.Errorf("entry = %+v, want hash + provider set", entry)
		}
		if entry.ExpiresAt.Before(before.Add(2*time.Hour)) || entry.ExpiresAt.After(time.Now().Add(2*time.Hour)) {
			t.Errorf("ExpiresAt = %v, want ~2h in the future", entry.ExpiresAt)
		}
	})
}

func TestStoreUnlessDowngrade(t *testing.T) {
	newCtrl := func(repo *repomocks.MockRecipeRepository) *RecipeController {
		logger := zerolog.Nop()
		return &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{CacheEnabled: true, Provider: util.RecipeProviderThemealDB},
			RecipeRepo: repo,
		}
	}

	t.Run("populated entry is kept", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{PopulatedCache: true}
		ctrl := newCtrl(repo)
		ctrl.storeUnlessDowngrade("hash", []RecipeSuggestion{}, time.Hour)
		if len(repo.CacheWrites) != 0 {
			t.Errorf("cache writes = %d, want 0 (existing entry kept)", len(repo.CacheWrites))
		}
	})

	t.Run("empty cache is written", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{PopulatedCache: false}
		ctrl := newCtrl(repo)
		ctrl.storeUnlessDowngrade("hash", []RecipeSuggestion{}, time.Hour)
		if len(repo.CacheWrites) != 1 {
			t.Errorf("cache writes = %d, want 1", len(repo.CacheWrites))
		}
	})

	t.Run("check error still writes", func(t *testing.T) {
		repo := &repomocks.MockRecipeRepository{Err: errors.New("db down")}
		ctrl := newCtrl(repo)
		ctrl.storeUnlessDowngrade("hash", []RecipeSuggestion{}, time.Hour)
		if len(repo.CacheWrites) != 1 {
			t.Errorf("cache writes = %d, want 1 (inconclusive check must not block the write)", len(repo.CacheWrites))
		}
	})
}

func TestFetchFromAPI(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := &RecipeController{Logger: &logger}
	if _, _, err := ctrl.fetchFromAPI([]string{"chicken"}); !errors.Is(err, proviantErrors.ErrRecipeInvalidProvider) {
		t.Errorf("fetchFromAPI() = %v, want ErrRecipeInvalidProvider", err)
	}
}

// failingRecipeProvider always errors, standing in for an unreachable API.
type failingRecipeProvider struct{}

func (p *failingRecipeProvider) Name() string { return util.RecipeProviderThemealDB }
func (p *failingRecipeProvider) FetchRecipes([]string) ([]Recipe, bool, error) {
	return nil, false, errors.New("connection refused")
}

// countingRecipeProvider records whether it was called.
type countingRecipeProvider struct {
	called   bool
	recipes  []Recipe
	complete bool
}

func (p *countingRecipeProvider) Name() string { return util.RecipeProviderThemealDB }
func (p *countingRecipeProvider) FetchRecipes([]string) ([]Recipe, bool, error) {
	p.called = true
	return p.recipes, p.complete, nil
}

func TestGetSuggestionsExtraPaths(t *testing.T) {
	newSource := func(all []dbModel.Product, allErr error, fingerprint string, fpErr error) ProductSource {
		return ProductSource{
			All:         func() ([]dbModel.Product, error) { return all, allErr },
			Fingerprint: func() (string, error) { return fingerprint, fpErr },
		}
	}
	expiring := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Chicken", Categories: "meat", ExpireAt: time.Now().AddDate(0, 0, 2)},
	}

	t.Run("product source error propagates", func(t *testing.T) {
		logger := zerolog.Nop()
		ctrl := &RecipeController{
			Logger:   &logger,
			Config:   configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB},
			Provider: &countingRecipeProvider{},
		}
		source := newSource(nil, errors.New("db down"), "fp", nil)
		if _, err := ctrl.GetSuggestions(expiring, source, 5, false); err == nil {
			t.Error("GetSuggestions() = nil, want the product source error")
		}
	})

	t.Run("provider error maps to ErrRecipeAPIUnavailable", func(t *testing.T) {
		logger := zerolog.Nop()
		ctrl := &RecipeController{
			Logger:   &logger,
			Config:   configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB},
			Provider: &failingRecipeProvider{},
		}
		source := newSource([]dbModel.Product{{Model: gorm.Model{ID: 1}, ProductName: "Chicken"}}, nil, "fp", nil)
		_, err := ctrl.GetSuggestions(expiring, source, 5, false)
		if !errors.Is(err, proviantErrors.ErrRecipeAPIUnavailable) {
			t.Errorf("GetSuggestions() = %v, want ErrRecipeAPIUnavailable", err)
		}
	})

	t.Run("fingerprint error fails the cache lookup open", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{}
		provider := &countingRecipeProvider{recipes: matchingRecipe, complete: true}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB, CacheEnabled: true, CacheTTL: 24},
			RecipeRepo: repo,
			Provider:   provider,
		}
		// An unverifiable fingerprint must not read the cache, but the fresh
		// path still serves suggestions; and the unreachable key is never written.
		source := newSource([]dbModel.Product{{Model: gorm.Model{ID: 1}, ProductName: "Chicken", Categories: "meat"}}, nil, "", errors.New("fp down"))
		suggestions, err := ctrl.GetSuggestions(expiring, source, 5, false)
		if err != nil {
			t.Fatalf("GetSuggestions() = %v", err)
		}
		if !provider.called {
			t.Error("provider was not called")
		}
		if len(suggestions) == 0 {
			t.Error("suggestions = 0, want matches from the fresh path")
		}
		if len(repo.CacheWrites) != 0 {
			t.Errorf("cache writes = %d, want 0 (unverifiable key must never be written)", len(repo.CacheWrites))
		}
	})

	t.Run("cache hit serves without calling the provider", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{Cache: validCacheEntry(t, time.Hour)}
		provider := &countingRecipeProvider{recipes: matchingRecipe, complete: true}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB, CacheEnabled: true, CacheTTL: 24},
			RecipeRepo: repo,
			Provider:   provider,
		}
		source := newSource(nil, errors.New("must not be called"), "fp", nil)
		suggestions, err := ctrl.GetSuggestions(expiring, source, 5, false)
		if err != nil {
			t.Fatalf("GetSuggestions() = %v", err)
		}
		if provider.called {
			t.Error("provider was called despite a cache hit")
		}
		if len(suggestions) != 1 || suggestions[0].ID != "cached" {
			t.Errorf("suggestions = %+v, want the cached entry", suggestions)
		}
	})

	t.Run("refresh bypasses the cache", func(t *testing.T) {
		logger := zerolog.Nop()
		repo := &repomocks.MockRecipeRepository{Cache: validCacheEntry(t, time.Hour)}
		provider := &countingRecipeProvider{recipes: matchingRecipe, complete: true}
		ctrl := &RecipeController{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB, CacheEnabled: true, CacheTTL: 24},
			RecipeRepo: repo,
			Provider:   provider,
		}
		source := newSource([]dbModel.Product{{Model: gorm.Model{ID: 1}, ProductName: "Chicken", Categories: "meat"}}, nil, "fp", nil)
		suggestions, err := ctrl.GetSuggestions(expiring, source, 5, true)
		if err != nil {
			t.Fatalf("GetSuggestions() = %v", err)
		}
		if !provider.called {
			t.Error("provider was not called despite refresh=true")
		}
		if len(suggestions) == 0 || suggestions[0].ID == "cached" {
			t.Errorf("suggestions = %+v, want fresh provider results", suggestions)
		}
	})
}
