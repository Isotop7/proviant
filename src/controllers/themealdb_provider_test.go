package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

// newTheMealDBTestServer serves TheMealDB-shaped fixtures:
//   - /filter.php?c=chicken  → one meal summary (idMeal 1)
//   - /search.php?s=garlic   → one meal summary (idMeal 2)
//   - /lookup.php?i=<id>     → full meal with ingredients
func newTheMealDBTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/filter.php", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("c") != "chicken" {
			fmt.Fprint(w, `{"meals":[]}`)
			return
		}
		fmt.Fprint(w, `{"meals":[{"idMeal":"1","strMeal":"Chicken A","strMealThumb":"http://img/1","strSource":"http://src/1"}]}`)
	})
	mux.HandleFunc("/search.php", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("s") != "garlic" {
			fmt.Fprint(w, `{"meals":[]}`)
			return
		}
		fmt.Fprint(w, `{"meals":[{"idMeal":"2","strMeal":"Garlic B","strMealThumb":"http://img/2","strSource":"http://src/2"}]}`)
	})
	mux.HandleFunc("/lookup.php", func(w http.ResponseWriter, r *http.Request) {
		meal := map[string]string{
			"1": `{"meals":[{"idMeal":"1","strMeal":"Chicken A","strMealThumb":"http://img/1","strSource":"http://src/1","strIngredient1":"chicken","strIngredient2":null,"strIngredient3":""}]}`,
			"2": `{"meals":[{"idMeal":"2","strMeal":"Garlic B","strMealThumb":"http://img/2","strSource":"http://src/2","strIngredient1":"garlic"}]}`,
		}
		fmt.Fprint(w, meal[r.URL.Query().Get("i")])
	})
	return httptest.NewServer(mux)
}

func newTheMealDBTestProvider(t *testing.T) (*TheMealDBProvider, *httptest.Server) {
	t.Helper()
	server := newTheMealDBTestServer(t)
	t.Cleanup(server.Close)
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		Provider: "themealdb",
		URL:      server.URL,
		Timeout:  5,
	}
	return &TheMealDBProvider{Logger: &logger, Config: config, HTTPClient: server.Client()}, server
}

func TestTheMealDBProvider_FetchRecipes_CategoryAndText(t *testing.T) {
	provider, _ := newTheMealDBTestProvider(t)

	t.Run("known category uses filter endpoint", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"chicken"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
		}
		if recipes[0].ID != "1" || recipes[0].Title != "Chicken A" {
			t.Errorf("recipe = %+v, want id 1 Chicken A", recipes[0])
		}
		if len(recipes[0].Ingredients) != 1 || recipes[0].Ingredients[0] != "chicken" {
			t.Errorf("ingredients = %v, want [chicken]", recipes[0].Ingredients)
		}
	})

	t.Run("unknown keyword uses text search", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"garlic"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
		}
		if recipes[0].ID != "2" {
			t.Errorf("recipe ID = %q, want 2", recipes[0].ID)
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

	t.Run("keyword with no results yields empty list", func(t *testing.T) {
		recipes, _, err := provider.FetchRecipes([]string{"unknown-ingredient"})
		if err != nil {
			t.Fatalf("FetchRecipes() error = %v", err)
		}
		if len(recipes) != 0 {
			t.Errorf("FetchRecipes() returned %d recipes, want 0", len(recipes))
		}
	})
}

func TestTheMealDBProvider_FetchRecipes_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &TheMealDBProvider{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "themealdb", URL: server.URL},
		HTTPClient: server.Client(),
	}

	// A total provider outage must surface as an error, not as empty suggestions.
	recipes, _, err := provider.FetchRecipes([]string{"garlic"})
	if err == nil {
		t.Fatalf("FetchRecipes() expected error for total provider outage, got nil")
	}
	if len(recipes) != 0 {
		t.Errorf("FetchRecipes() returned %d recipes, want 0 on error", len(recipes))
	}
}
