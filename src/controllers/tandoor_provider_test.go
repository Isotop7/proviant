package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

func newTandoorTestServer(t *testing.T, recordAuth *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipe/", func(w http.ResponseWriter, r *http.Request) {
		if recordAuth != nil {
			*recordAuth = append(*recordAuth, r.Header.Get("Authorization"))
		}
		// Detail endpoint: /api/recipe/{id}/
		if path := stringsTrimPrefixSlash(r.URL.Path, "/api/recipe/"); path != "" {
			if path == "1" {
				fmt.Fprint(w, `{
					"id": 1,
					"slug": "chicken-dish",
					"name": "Chicken Dish",
					"image": "https://tandoor.example.com/media/recipes/1.jpg",
					"steps": [{"ingredients": [{"food": {"name": "Chicken"}}, {"food": {"name": "Chicken"}}, {"food": {"name": "Paprika"}}]}]
				}`)
				return
			}
			http.NotFound(w, r)
			return
		}
		// List endpoint: /api/recipe/
		if r.URL.Query().Get("search") != "chicken" {
			fmt.Fprint(w, `{"count":0,"results":[]}`)
			return
		}
		// List result omits steps (list serializer) → provider must fall back to details.
		fmt.Fprint(w, `{"count":1,"results":[{"id":1,"slug":"chicken-dish","name":"Chicken Dish","image":"/media/recipes/1.jpg"}]}`)
	})
	return httptest.NewServer(mux)
}

// stringsTrimPrefixSlash trims a prefix and a trailing slash; helper keeps the fixture readable.
func stringsTrimPrefixSlash(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		rest := s[len(prefix):]
		if len(rest) > 0 && rest[len(rest)-1] == '/' {
			rest = rest[:len(rest)-1]
		}
		return rest
	}
	return ""
}

func newTandoorTestProvider(t *testing.T) (*TandoorProvider, *httptest.Server, *[]string) {
	t.Helper()
	var authHeaders []string
	server := newTandoorTestServer(t, &authHeaders)
	t.Cleanup(server.Close)
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		Provider: "tandoor",
		URL:      server.URL,
		APIKey:   "secret-tandoor-token",
		Timeout:  5,
	}
	return &TandoorProvider{Logger: &logger, Config: config, HTTPClient: server.Client()}, server, &authHeaders
}

func TestTandoorProvider_Name(t *testing.T) {
	provider, _, _ := newTandoorTestProvider(t)
	if provider.Name() != "tandoor" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "tandoor")
	}
}

func TestTandoorProvider_FetchRecipes(t *testing.T) {
	provider, _, _ := newTandoorTestProvider(t)

	t.Run("list result without steps falls back to detail endpoint", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"chicken"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
		}
		recipe := recipes[0]
		if recipe.ID != "1" || recipe.Title != "Chicken Dish" {
			t.Errorf("recipe = %+v, want id 1 Chicken Dish", recipe)
		}
		wantURL := provider.Config.URL + "/recipe/chicken-dish/"
		if recipe.SourceURL != wantURL {
			t.Errorf("SourceURL = %q, want %q", recipe.SourceURL, wantURL)
		}
		// Relative image path is dropped, absolute is kept.
		if recipe.ImageURL != "https://tandoor.example.com/media/recipes/1.jpg" {
			t.Errorf("ImageURL = %q, want absolute URL from detail response", recipe.ImageURL)
		}
		wantIngredients := []string{"Chicken", "Paprika"}
		if len(recipe.Ingredients) != len(wantIngredients) {
			t.Fatalf("ingredients = %v, want %v (deduplicated)", recipe.Ingredients, wantIngredients)
		}
		for i := range wantIngredients {
			if recipe.Ingredients[i] != wantIngredients[i] {
				t.Errorf("ingredients[%d] = %q, want %q", i, recipe.Ingredients[i], wantIngredients[i])
			}
		}
	})

	t.Run("list result with steps needs no detail call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/recipe/" {
				t.Errorf("unexpected detail call to %s", r.URL.Path)
			}
			fmt.Fprint(w, `{"count":1,"results":[{"id":2,"slug":"salad","name":"Salad","steps":[{"ingredients":[{"food":{"name":"Lettuce"}}]}]}]}`)
		}))
		defer server.Close()

		logger := zerolog.Nop()
		provider := &TandoorProvider{
			Logger:     &logger,
			Config:     configuration.RecipeAPIConfiguration{Provider: "tandoor", URL: server.URL, APIKey: "k"},
			HTTPClient: server.Client(),
		}

		recipes, _, err := provider.FetchRecipes([]string{"lettuce"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
		}
		if len(recipes[0].Ingredients) != 1 || recipes[0].Ingredients[0] != "Lettuce" {
			t.Errorf("ingredients = %v, want [Lettuce]", recipes[0].Ingredients)
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
}

func TestTandoorProvider_SendsTokenAuth(t *testing.T) {
	provider, _, authHeaders := newTandoorTestProvider(t)

	if _, _, err := provider.FetchRecipes([]string{"chicken"}); err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(*authHeaders) == 0 {
		t.Fatal("no requests recorded")
	}
	for _, header := range *authHeaders {
		if header != "Token secret-tandoor-token" {
			t.Errorf("Authorization header = %q, want %q", header, "Token secret-tandoor-token")
		}
	}
}

// TestTandoorProvider_DetailFailureKeepsListResolvedRecipes is the regression for
// the detail-phase availability verdict: when Tandoor's list payload already
// resolved some recipes and every detail lookup then fails, the provider used to
// count only the detail phase, declare a total outage and throw the good
// recipes away — a 500 instead of the suggestions it already held.
func TestTandoorProvider_DetailFailureKeepsListResolvedRecipes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/recipe/" {
			// The detail endpoint for the step-less recipe fails.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"count":2,"results":[
			{"id":1,"slug":"salad","name":"Salad","steps":[{"ingredients":[{"food":{"name":"Lettuce"}}]}]},
			{"id":2,"slug":"stew","name":"Stew"}
		]}`)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &TandoorProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "tandoor", URL: server.URL, APIKey: "k"},
		HTTPClient: server.Client(),
	}

	recipes, complete, err := provider.FetchRecipes([]string{"salad"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v, want nil (list-resolved recipes are usable)", err)
	}
	if complete {
		t.Error("complete = true, want false (the detail lookup failed)")
	}
	if len(recipes) != 1 {
		t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
	}
	if recipes[0].ID != "1" || recipes[0].Title != "Salad" {
		t.Errorf("recipe = %+v, want the list-resolved id 1 Salad", recipes[0])
	}
	if len(recipes[0].Ingredients) != 1 || recipes[0].Ingredients[0] != "Lettuce" {
		t.Errorf("ingredients = %v, want [Lettuce]", recipes[0].Ingredients)
	}
}

// Some Tandoor versions omit the id on the detail payload. Without a fallback
// every detail-resolved recipe would be keyed "0" and deduplicateRecipes would
// collapse them into one, so the list entry's id must be used instead.
func TestTandoorProvider_DetailWithoutIDKeepsListID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/recipe/" {
			fmt.Fprint(w, `{"count":2,"results":[
				{"id":1,"slug":"salad","name":"Salad"},
				{"id":2,"slug":"stew","name":"Stew"}
			]}`)
			return
		}
		// Detail payloads deliberately carry no "id" field.
		switch r.URL.Path {
		case "/api/recipe/1/":
			fmt.Fprint(w, `{"slug":"salad","name":"Salad","steps":[{"ingredients":[{"food":{"name":"Lettuce"}}]}]}`)
		case "/api/recipe/2/":
			fmt.Fprint(w, `{"slug":"stew","name":"Stew","steps":[{"ingredients":[{"food":{"name":"Beef"}}]}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &TandoorProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "tandoor", URL: server.URL, APIKey: "k"},
		HTTPClient: server.Client(),
	}

	recipes, complete, err := provider.FetchRecipes([]string{"salad", "stew"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v, want nil", err)
	}
	if !complete {
		t.Error("complete = false, want true (both detail lookups succeeded)")
	}
	if len(recipes) != 2 {
		t.Fatalf("FetchRecipes() returned %d recipes, want 2 — deduplication collapsed the id-less detail payloads into one", len(recipes))
	}
	got := map[string]string{}
	for _, recipe := range recipes {
		got[recipe.Title] = recipe.ID
	}
	if got["Salad"] != "1" || got["Stew"] != "2" {
		t.Errorf("detail ids = %v, want Salad->1 and Stew->2 from the list entries", got)
	}
}

func TestTandoorProvider_FetchRecipes_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &TandoorProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "tandoor", URL: server.URL, APIKey: "k"},
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
