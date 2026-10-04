package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

// authRecorder collects Authorization headers seen by the test server. The
// provider fans keyword searches out across a worker pool, so several handlers
// append concurrently and the slice needs its own lock — without it the race
// detector fails the suite.
type authRecorder struct {
	mu      sync.Mutex
	headers []string
}

func (a *authRecorder) record(header string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.headers = append(a.headers, header)
}

func (a *authRecorder) records() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.headers)
}

func newMealieTestServer(t *testing.T, recordAuth *authRecorder) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipes", func(w http.ResponseWriter, r *http.Request) {
		if recordAuth != nil {
			recordAuth.record(r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("search") != "chicken" {
			fmt.Fprint(w, `{"page":1,"perPage":20,"total":0,"totalPages":1,"items":[]}`)
			return
		}
		fmt.Fprint(w, `{"page":1,"perPage":20,"total":1,"totalPages":1,"items":[{"id":"uuid-1","slug":"chicken-dish","name":"Chicken Dish"}]}`)
	})
	// Mealie resolves a UUID on this route before falling back to a slug, and
	// the provider addresses the detail endpoint by ID, so the fixture keys on
	// the ID rather than the slug.
	mux.HandleFunc("/api/recipes/uuid-1", func(w http.ResponseWriter, r *http.Request) {
		if recordAuth != nil {
			recordAuth.record(r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{
			"id": "uuid-1",
			"slug": "chicken-dish",
			"name": "Chicken Dish",
			"recipeIngredient": [
				{"originalText": "500 g chicken breast", "note": "", "display": "500 g chicken breast"},
				{"originalText": "", "note": "a pinch of salt", "display": ""},
				{"originalText": "", "note": "", "display": ""},
				{"originalText": "2 cloves garlic", "note": "crushed", "display": "2 cloves garlic, crushed"}
			]
		}`)
	})
	return httptest.NewServer(mux)
}

func newMealieTestProvider(t *testing.T) (*MealieProvider, *httptest.Server, *authRecorder) {
	t.Helper()
	authHeaders := &authRecorder{}
	server := newMealieTestServer(t, authHeaders)
	t.Cleanup(server.Close)
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		Provider: "mealie",
		URL:      server.URL,
		APIKey:   "secret-mealie-token",
		Timeout:  5,
	}
	return &MealieProvider{Logger: &logger, Config: config, HTTPClient: server.Client()}, server, authHeaders
}

func TestMealieProvider_Name(t *testing.T) {
	provider, _, _ := newMealieTestProvider(t)
	if provider.Name() != "mealie" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "mealie")
	}
}

func TestMealieProvider_FetchRecipes(t *testing.T) {
	provider, _, _ := newMealieTestProvider(t)

	t.Run("search result is resolved with details", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"chicken"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
		}
		recipe := recipes[0]
		if recipe.ID != "uuid-1" || recipe.Title != "Chicken Dish" {
			t.Errorf("recipe = %+v, want id uuid-1 Chicken Dish", recipe)
		}
		wantURL := provider.Config.URL + "/recipe/chicken-dish"
		if recipe.SourceURL != wantURL {
			t.Errorf("SourceURL = %q, want %q", recipe.SourceURL, wantURL)
		}
		if recipe.ImageURL != "" {
			t.Errorf("ImageURL = %q, want empty (media endpoint not fetched)", recipe.ImageURL)
		}
		wantIngredients := []string{"500 g chicken breast", "a pinch of salt", "2 cloves garlic"}
		if len(recipe.Ingredients) != len(wantIngredients) {
			t.Fatalf("ingredients = %v, want %v", recipe.Ingredients, wantIngredients)
		}
		for i := range wantIngredients {
			if recipe.Ingredients[i] != wantIngredients[i] {
				t.Errorf("ingredients[%d] = %q, want %q", i, recipe.Ingredients[i], wantIngredients[i])
			}
		}
	})

	t.Run("keyword with no results yields empty list", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"nonexistent"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 0 {
			t.Errorf("FetchRecipes() returned %d recipes, want 0", len(recipes))
		}
	})

	t.Run("duplicate recipes are deduplicated by ID", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"chicken", "chicken"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Errorf("FetchRecipes() returned %d recipes, want 1 after dedup", len(recipes))
		}
	})
}

func TestMealieProvider_SendsBearerAuth(t *testing.T) {
	provider, _, authHeaders := newMealieTestProvider(t)

	if _, _, err := provider.FetchRecipes([]string{"chicken"}); err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(authHeaders.records()) == 0 {
		t.Fatal("no requests recorded")
	}
	for _, header := range authHeaders.records() {
		if header != "Bearer secret-mealie-token" {
			t.Errorf("Authorization header = %q, want %q", header, "Bearer secret-mealie-token")
		}
	}
}

func TestMealieProvider_FetchRecipes_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &MealieProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "mealie", URL: server.URL, APIKey: "k"},
		HTTPClient: server.Client(),
	}

	// A total provider outage must surface as an error, not as empty suggestions.
	recipes, _, err := provider.FetchRecipes([]string{"chicken"})
	if err == nil {
		t.Fatalf("FetchRecipes() expected error for total provider outage, got nil")
	}
	if len(recipes) != 0 {
		t.Errorf("FetchRecipes() returned %d recipes, want 0 on error", len(recipes))
	}
}

// A Mealie version that omits the slug used to drop the search item entirely,
// because the dedupe key was the slug. The item never reached the detail phase,
// so the run reported itself complete with zero recipes and the controller
// cached that empty answer — a silent "no suggestions" with no provider error.
// The key must be the immutable ID, with the slug only as a fallback.
func TestMealieProvider_SearchItemWithoutSlugIsNotDropped(t *testing.T) {
	logger := zerolog.Nop()
	detailRequests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipes", func(w http.ResponseWriter, r *http.Request) {
		// ID present, slug absent — the case that used to be filtered out.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"uuid-9","slug":"","name":"Slugless Dish"}]}`))
	})
	mux.HandleFunc("/api/recipes/", func(w http.ResponseWriter, r *http.Request) {
		detailRequests++
		// The exact path is asserted, not just the prefix: a subtree handler
		// also matches the collection endpoint "/api/recipes/", which is what
		// an empty slug produces, so a prefix match would let that regression
		// pass while still returning one recipe.
		if want := "/api/recipes/uuid-9"; r.URL.Path != want {
			t.Errorf("detail request path = %q, want %q (the ID must address the detail endpoint, not the slug or the collection)", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"uuid-9","slug":"","name":"Slugless Dish","recipeIngredient":[{"originalText":"chicken"}]}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	provider := &MealieProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "mealie", URL: server.URL, APIKey: "k", Timeout: 5},
		HTTPClient: server.Client(),
	}

	recipes, _, err := provider.FetchRecipes([]string{"chicken"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("FetchRecipes() returned %d recipes, want 1 — a slugless item was dropped by the dedupe key", len(recipes))
	}
	if recipes[0].ID != "uuid-9" {
		t.Errorf("recipe ID = %q, want %q (the ID must survive when the slug is empty)", recipes[0].ID, "uuid-9")
	}
	if detailRequests != 1 {
		t.Errorf("detail requests = %d, want 1", detailRequests)
	}
}

// Two entries that differ only by slug must not collapse into one when their
// IDs differ, and two entries sharing an ID must deduplicate across keywords.
func TestMealieProvider_DedupeKeysOnIDNotSlug(t *testing.T) {
	logger := zerolog.Nop()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipes", func(w http.ResponseWriter, r *http.Request) {
		keyword := r.URL.Query().Get("search")
		w.Header().Set("Content-Type", "application/json")
		if keyword == "beef" {
			_, _ = w.Write([]byte(`{"items":[{"id":"uuid-1","slug":"stale-slug","name":"Beef Dish"}]}`))
			return
		}
		// Same ID, different slug: one recipe seen under two keywords, and the
		// slug is not stable enough to be a reliable identity.
		_, _ = w.Write([]byte(`{"items":[{"id":"uuid-1","slug":"fresh-slug","name":"Beef Dish"}]}`))
	})
	mux.HandleFunc("/api/recipes/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"uuid-1","slug":"x","name":"Beef Dish","recipeIngredient":[{"originalText":"beef"}]}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	provider := &MealieProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "mealie", URL: server.URL, APIKey: "k", Timeout: 5},
		HTTPClient: server.Client(),
	}

	recipes, _, err := provider.FetchRecipes([]string{"beef", "steak"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(recipes) != 1 {
		t.Errorf("FetchRecipes() returned %d recipes, want 1 — the shared ID must deduplicate across keywords", len(recipes))
	}
}
