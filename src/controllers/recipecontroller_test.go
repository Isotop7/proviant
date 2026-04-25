package controllers

import (
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newTestRecipeController() *RecipeController {
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		URL:          "https://www.themealdb.com/api/json/v1/1",
		Timeout:      10,
		CacheEnabled: false,
		CacheTTL:     24,
		Provider:     "themealdb",
	}
	return &RecipeController{
		Logger:     &logger,
		Config:     config,
		RecipeRepo: nil,
		HTTPClient: nil,
	}
}

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Lowercase conversion",
			input: "HELLO",
			want:  "hello",
		},
		{
			name:  "Hyphen replacement",
			input: "ice-cream",
			want:  "ice cream",
		},
		{
			name:  "Apostrophe removal",
			input: "chicken's",
			want:  "chickens",
		},
		{
			name:  "Dot removal",
			input: "chicken.breast",
			want:  "chickenbreast",
		},
		{
			name:  "Multiple transformations",
			input: "CHICKEN-BREAST'S",
			want:  "chicken breasts",
		},
		{
			name:  "Trim whitespace",
			input: "  chicken  ",
			want:  "chicken",
		},
		{
			name:  "Empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractIngredients(t *testing.T) {
	ctrl := newTestRecipeController()

	tests := []struct {
		name string
		ings []*string
		want []string
	}{
		{
			name: "All nil pointers",
			ings: []*string{nil, nil, nil},
			want: []string{},
		},
		{
			name: "Some nil pointers",
			ings: []*string{strPtr("chicken"), nil, strPtr("rice")},
			want: []string{"chicken", "rice"},
		},
		{
			name: "Empty strings filtered",
			ings: []*string{strPtr("chicken"), strPtr(""), strPtr("rice")},
			want: []string{"chicken", "rice"},
		},
		{
			name: "All valid",
			ings: []*string{strPtr("chicken"), strPtr("garlic"), strPtr("salt")},
			want: []string{"chicken", "garlic", "salt"},
		},
		{
			name: "Nil and empty mixed",
			ings: []*string{nil, strPtr(""), strPtr("onion")},
			want: []string{"onion"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.extractIngredients(tt.ings...)
			if len(got) != len(tt.want) {
				t.Errorf("extractIngredients() returned %d ingredients, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractIngredients()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestCollectKeywords(t *testing.T) {
	ctrl := newTestRecipeController()

	tests := []struct {
		name     string
		products []dbModel.Product
		wantMin  int
	}{
		{
			name:     "Empty products",
			products: []dbModel.Product{},
			wantMin:  0,
		},
		{
			name: "Single product",
			products: []dbModel.Product{
				{ProductName: "Chicken Breast", Categories: "meat,poultry"},
			},
			wantMin: 1,
		},
		{
			name: "Multiple products with categories",
			products: []dbModel.Product{
				{ProductName: "Chicken", Categories: "meat"},
				{ProductName: "Rice", Categories: "grain"},
			},
			wantMin: 2,
		},
		{
			name: "Filtered categories",
			products: []dbModel.Product{
				{ProductName: "Chicken", Categories: "food,de:lebensmittel"},
			},
			wantMin: 1,
		},
		{
			name: "Empty product names filtered",
			products: []dbModel.Product{
				{ProductName: "", Categories: "meat"},
			},
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.collectKeywords(tt.products)
			if len(got) < tt.wantMin {
				t.Errorf("collectKeywords() returned %d keywords, want at least %d", len(got), tt.wantMin)
			}
		})
	}
}

func TestMatchRecipesToProducts(t *testing.T) {
	ctrl := newTestRecipeController()

	tests := []struct {
		name     string
		recipes  []Recipe
		products []dbModel.Product
		wantMin  int
	}{
		{
			name:     "No recipes",
			recipes:  []Recipe{},
			products: []dbModel.Product{{ProductName: "chicken"}},
			wantMin:  0,
		},
		{
			name:     "No products",
			recipes:  []Recipe{{ID: "1", Title: "Chicken Recipe", Ingredients: []string{"chicken", "garlic"}}},
			products: []dbModel.Product{},
			wantMin:  0,
		},
		{
			name: "Exact ingredient match",
			recipes: []Recipe{
				{ID: "1", Title: "Chicken Dish", Ingredients: []string{"chicken", "garlic"}},
			},
			products: []dbModel.Product{
				{ProductName: "Chicken"},
				{ProductName: "Garlic"},
			},
			wantMin: 1,
		},
		{
			name: "Partial match",
			recipes: []Recipe{
				{ID: "1", Title: "Chicken Dish", Ingredients: []string{"chicken", "garlic", "salt"}},
			},
			products: []dbModel.Product{
				{ProductName: "Chicken"},
			},
			wantMin: 1,
		},
		{
			name: "Category match",
			recipes: []Recipe{
				{ID: "1", Title: "Beef Dish", Ingredients: []string{"beef"}},
			},
			products: []dbModel.Product{
				{ProductName: "Steak", Categories: "beef"},
			},
			wantMin: 1,
		},
		{
			name: "No match",
			recipes: []Recipe{
				{ID: "1", Title: "Chicken Dish", Ingredients: []string{"chicken"}},
			},
			products: []dbModel.Product{
				{ProductName: "Beef"},
			},
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.matchRecipesToProducts(tt.recipes, tt.products)
			if len(got) < tt.wantMin {
				t.Errorf("matchRecipesToProducts() returned %d suggestions, want at least %d", len(got), tt.wantMin)
			}
		})
	}
}

func TestMatchRecipesToProducts_Sorting(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Low Match", Ingredients: []string{"chicken", "beef", "pork"}},
		{ID: "2", Title: "High Match", Ingredients: []string{"chicken"}},
		{ID: "3", Title: "Medium Match", Ingredients: []string{"chicken", "beef"}},
	}
	products := []dbModel.Product{
		{ProductName: "chicken"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products)

	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %d", len(suggestions))
	}

	if suggestions[0].MatchPercent < suggestions[1].MatchPercent {
		t.Error("expected suggestions sorted by match percentage descending")
	}
}

func TestLimitSuggestions(t *testing.T) {
	ctrl := newTestRecipeController()

	suggestions := []RecipeSuggestion{
		{ID: "1", Title: "Recipe 1"},
		{ID: "2", Title: "Recipe 2"},
		{ID: "3", Title: "Recipe 3"},
		{ID: "4", Title: "Recipe 4"},
		{ID: "5", Title: "Recipe 5"},
		{ID: "6", Title: "Recipe 6"},
		{ID: "7", Title: "Recipe 7"},
		{ID: "8", Title: "Recipe 8"},
		{ID: "9", Title: "Recipe 9"},
		{ID: "10", Title: "Recipe 10"},
		{ID: "11", Title: "Recipe 11"},
	}

	tests := []struct {
		name    string
		input   []RecipeSuggestion
		limit   int
		wantLen int
	}{
		{
			name:    "Default limit (6)",
			input:   suggestions,
			limit:   0,
			wantLen: 6,
		},
		{
			name:    "Custom limit",
			input:   suggestions,
			limit:   3,
			wantLen: 3,
		},
		{
			name:    "Limit above max (10)",
			input:   suggestions,
			limit:   15,
			wantLen: 10,
		},
		{
			name:    "Empty input",
			input:   []RecipeSuggestion{},
			limit:   5,
			wantLen: 0,
		},
		{
			name:    "Input smaller than limit",
			input:   suggestions[:3],
			limit:   10,
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.limitSuggestions(tt.input, tt.limit)
			if len(got) != tt.wantLen {
				t.Errorf("limitSuggestions() returned %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestGetSuggestions_EmptyExpiring(t *testing.T) {
	ctrl := newTestRecipeController()

	suggestions, err := ctrl.GetSuggestions([]dbModel.Product{}, []dbModel.Product{}, 5)
	if err != nil {
		t.Errorf("GetSuggestions() error = %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("GetSuggestions() with empty expiring products returned %d suggestions, want 0", len(suggestions))
	}
}

var _ *gorm.DB = nil
