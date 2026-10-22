package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

// theMealDBFixture serves TheMealDB-shaped fixtures and records the paths it
// was asked for, so a test can assert which endpoint a keyword reached:
//   - /filter.php?c=chicken    → one meal summary (idMeal 1)
//   - /filter.php?c=breakfast  → one meal summary (idMeal 3)
//   - /filter.php?i=oats       → one meal summary (idMeal 4)
//   - /search.php?s=garlic     → one meal summary (idMeal 2)
//   - /lookup.php?i=<id>       → full meal with ingredients
type theMealDBFixture struct {
	server  *httptest.Server
	paths   []string
	pathMux sync.Mutex
}

func newTheMealDBFixture(t *testing.T) *theMealDBFixture {
	t.Helper()
	fixture := &theMealDBFixture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/filter.php", func(w http.ResponseWriter, r *http.Request) {
		fixture.record(r.URL.RequestURI())
		switch {
		case r.URL.Query().Get("c") == "chicken":
			fmt.Fprint(w, `{"meals":[{"idMeal":"1","strMeal":"Chicken A","strMealThumb":"http://img/1","strSource":"http://src/1"}]}`)
		case r.URL.Query().Get("c") == "breakfast":
			fmt.Fprint(w, `{"meals":[{"idMeal":"3","strMeal":"Breakfast A","strMealThumb":"http://img/3","strSource":"http://src/3"}]}`)
		case r.URL.Query().Get("i") == "oats":
			fmt.Fprint(w, `{"meals":[{"idMeal":"4","strMeal":"Oats A","strMealThumb":"http://img/4","strSource":"http://src/4"}]}`)
		default:
			fmt.Fprint(w, `{"meals":[]}`)
		}
	})
	mux.HandleFunc("/search.php", func(w http.ResponseWriter, r *http.Request) {
		fixture.record(r.URL.RequestURI())
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
			"3": `{"meals":[{"idMeal":"3","strMeal":"Breakfast A","strMealThumb":"http://img/3","strSource":"http://src/3","strIngredient1":"oats"}]}`,
			"4": `{"meals":[{"idMeal":"4","strMeal":"Oats A","strMealThumb":"http://img/4","strSource":"http://src/4","strIngredient1":"oats"}]}`,
		}
		fmt.Fprint(w, meal[r.URL.Query().Get("i")])
	})
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *theMealDBFixture) record(path string) {
	f.pathMux.Lock()
	defer f.pathMux.Unlock()
	f.paths = append(f.paths, path)
}

func (f *theMealDBFixture) requestedPaths() []string {
	f.pathMux.Lock()
	defer f.pathMux.Unlock()
	return slices.Clone(f.paths)
}

func (f *theMealDBFixture) provider(t *testing.T) *TheMealDBProvider {
	t.Helper()
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		Provider: "themealdb",
		URL:      f.server.URL,
		Timeout:  5,
	}
	return &TheMealDBProvider{Logger: &logger, Config: config, HTTPClient: f.server.Client()}
}

func TestTheMealDBProvider_FetchRecipes_CategoryAndText(t *testing.T) {
	fixture := newTheMealDBFixture(t)
	provider := fixture.provider(t)

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

func TestTheMealDBProvider_FetchRecipes_ProductNameReachesTheIngredientFilter(t *testing.T) {
	fixture := newTheMealDBFixture(t)
	provider := fixture.provider(t)

	recipes, _, err := provider.FetchRecipes([]string{"quaker oats"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
	}
	if recipes[0].ID != "4" {
		t.Errorf("recipe ID = %q, want 4", recipes[0].ID)
	}
	if recipes[0].SearchTerm != "oats" {
		t.Errorf("SearchTerm = %q, want oats", recipes[0].SearchTerm)
	}
	if !slices.Contains(fixture.requestedPaths(), "/filter.php?i=oats") {
		t.Errorf("requested paths = %v, want the ingredient filter for oats", fixture.requestedPaths())
	}
}

func TestTheMealDBProvider_FetchRecipes_CategoryAlias(t *testing.T) {
	fixture := newTheMealDBFixture(t)
	provider := fixture.provider(t)

	recipes, _, err := provider.FetchRecipes([]string{"breakfasts"})
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("FetchRecipes() returned %d recipes, want 1", len(recipes))
	}
	if recipes[0].SearchTerm != "breakfasts" {
		t.Errorf("SearchTerm = %q, want the keyword breakfasts", recipes[0].SearchTerm)
	}
	if !slices.Contains(fixture.requestedPaths(), "/filter.php?c=breakfast") {
		t.Errorf("requested paths = %v, want the breakfast category filter", fixture.requestedPaths())
	}
}

func TestTheMealDBProvider_FetchRecipes_SearchBudgetBoundsTheRun(t *testing.T) {
	fixture := newTheMealDBFixture(t)
	provider := fixture.provider(t)

	keywords := make([]string, 0, 30)
	for i := range 30 {
		keywords = append(keywords, fmt.Sprintf("product %d", i))
	}
	_, complete, err := provider.FetchRecipes(keywords)
	if err != nil {
		t.Fatalf("FetchRecipes() error = %v", err)
	}
	if complete {
		t.Error("FetchRecipes() reported a complete run although the search budget ran out")
	}
	searches := 0
	for _, path := range fixture.requestedPaths() {
		if strings.HasPrefix(path, "/filter.php") || strings.HasPrefix(path, "/search.php") {
			searches++
		}
	}
	if searches > maxSearchRequestsPerRun {
		t.Errorf("run made %d searches, want at most %d", searches, maxSearchRequestsPerRun)
	}
}

func TestMealDBCategoriesFor(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		want    []string
	}{
		{name: "a TheMealDB category names itself", keyword: "chicken", want: []string{"chicken"}},
		{name: "a plural category names its singular", keyword: "breakfasts", want: []string{"breakfast"}},
		{name: "an OpenFoodFacts category maps onto a meal category", keyword: "fishes and their products", want: []string{"seafood"}},
		{name: "a dairy category maps onto the categories that use it", keyword: "dairies", want: []string{"breakfast", "dessert"}},
		{name: "an unknown keyword names no category", keyword: "chocolate spread", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mealDBCategoriesFor(tt.keyword)
			if !slices.Equal(got, tt.want) {
				t.Errorf("mealDBCategoriesFor(%q) = %v, want %v", tt.keyword, got, tt.want)
			}
		})
	}
}

func TestIngredientTerms(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		want    []string
	}{
		{
			name:    "the head noun comes first",
			keyword: "quaker oats",
			want:    []string{"quaker oats", "oats"},
		},
		{
			name:    "a modifier is no candidate",
			keyword: "whole dark chocolate",
			want:    []string{"whole dark chocolate", "chocolate"},
		},
		{
			name:    "a pack size is no candidate",
			keyword: "tomato puree 400g",
			want:    []string{"puree", "tomato"},
		},
		{
			name:    "a single word is its own candidate",
			keyword: "milk",
			want:    []string{"milk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ingredientTerms(tt.keyword)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ingredientTerms(%q) = %v, want %v", tt.keyword, got, tt.want)
			}
		})
	}
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
