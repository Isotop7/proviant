package controllers

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"
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
			got := extractPointerIngredients(tt.ings...)
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
			got := ctrl.matchRecipesToProducts(tt.recipes, tt.products, nil)
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

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

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

	source := ProductSource{
		All:         func() ([]dbModel.Product, error) { return nil, nil },
		Fingerprint: func() (string, error) { return "0:", nil },
	}
	suggestions, err := ctrl.GetSuggestions([]dbModel.Product{}, source, 5, false)
	if err != nil {
		t.Errorf("GetSuggestions() error = %v", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("GetSuggestions() with empty expiring products returned %d suggestions, want 0", len(suggestions))
	}
}

func TestMatchRecipesToProducts_ExpiryRanking(t *testing.T) {
	ctrl := newTestRecipeController()
	now := time.Now()

	recipes := []Recipe{
		{ID: "soonest", Title: "Uses Soonest Product", Ingredients: []string{"chicken"}},
		{ID: "later", Title: "Uses Later Product", Ingredients: []string{"rice"}},
	}
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 101}, ProductName: "Chicken", ExpireAt: now.AddDate(0, 0, 1)},
		{Model: gorm.Model{ID: 102}, ProductName: "Rice", ExpireAt: now.AddDate(0, 0, 6)},
	}
	expiring := []dbModel.Product{products[0], products[1]}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, expiring)

	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].ID != "soonest" {
		t.Errorf("suggestions[0].ID = %q, want %q (product expiring tomorrow ranks first)", suggestions[0].ID, "soonest")
	}
	if suggestions[0].ExpiryPoints <= suggestions[1].ExpiryPoints {
		t.Errorf("expiry points not descending: %d then %d", suggestions[0].ExpiryPoints, suggestions[1].ExpiryPoints)
	}
}

// Expiry ranking must score every product an ingredient matched, not only the
// exact-name subset that authorizes the cook action. Mealie reports ingredients
// as free text ("2 cloves garlic, minced"), so its matches land on a substring
// rank; gating the score on rankExactName scored those recipes 0 and silently
// demoted them below every exact-hit recipe.
func TestMatchRecipesToProducts_ExpiryRankingCountsNonExactMatches(t *testing.T) {
	ctrl := newTestRecipeController()
	now := time.Now()

	recipes := []Recipe{
		// Only a substring match: product is "Garlic", ingredient is
		// "garlic minced" after quantity stripping.
		{ID: "loose", Title: "Loose Match On Expiring Product", Ingredients: []string{"garlic minced"}},
		// Only an exact match, on a product expiring much later.
		{ID: "exact", Title: "Exact Match On Distant Product", Ingredients: []string{"rice"}},
	}
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 301}, ProductName: "Garlic", ExpireAt: now.AddDate(0, 0, 1)},
		{Model: gorm.Model{ID: 302}, ProductName: "Rice", ExpireAt: now.AddDate(0, 0, 6)},
	}
	expiring := []dbModel.Product{products[0], products[1]}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, expiring)

	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].ID != "loose" {
		t.Errorf("suggestions[0].ID = %q, want %q (a non-exact match on the soonest product must still score)",
			suggestions[0].ID, "loose")
	}
	if suggestions[0].ExpiryPoints <= suggestions[1].ExpiryPoints {
		t.Errorf("expiry points not descending: %d then %d", suggestions[0].ExpiryPoints, suggestions[1].ExpiryPoints)
	}
	// The cook action keeps its stricter gate: only the exact match is consumable.
	if len(suggestions[0].MatchedProductIDs) != 0 {
		t.Errorf("loose match exposed cook IDs %v, want none", suggestions[0].MatchedProductIDs)
	}
	if len(suggestions[1].MatchedProductIDs) != 1 || suggestions[1].MatchedProductIDs[0] != 302 {
		t.Errorf("exact match exposed cook IDs %v, want [302]", suggestions[1].MatchedProductIDs)
	}
}

func TestMatchRecipesToProducts_ExpiryTieBrokenByMatchPercent(t *testing.T) {
	ctrl := newTestRecipeController()
	now := time.Now()

	recipes := []Recipe{
		{ID: "partial", Title: "Partial Match", Ingredients: []string{"chicken", "garlic"}},
		{ID: "full", Title: "Full Match", Ingredients: []string{"chicken"}},
	}
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 201}, ProductName: "Chicken", ExpireAt: now.AddDate(0, 0, 1)},
	}
	expiring := []dbModel.Product{products[0]}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, expiring)

	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].ID != "full" {
		t.Errorf("suggestions[0].ID = %q, want %q (equal expiry points, higher MatchPercent wins)", suggestions[0].ID, "full")
	}
}

func TestMatchRecipesToProducts_MatchedProductIDs(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Chicken Dish", Ingredients: []string{"chicken", "chicken breast", "garlic"}},
	}
	// Both chicken ingredients match the same product → one deduplicated ID.
	// Product 301 carries an expiry date so it is in the expiring set.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 301}, ProductName: "Chicken", Categories: "poultry", ExpireAt: time.Now().AddDate(0, 0, 2)},
		{Model: gorm.Model{ID: 302}, ProductName: "Garlic"},
	}
	expiring := []dbModel.Product{products[0]}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, expiring)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	got := suggestions[0].MatchedProductIDs
	want := []uint{301, 302}
	if len(got) != len(want) {
		t.Fatalf("MatchedProductIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MatchedProductIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	if suggestions[0].ExpiryPoints <= 0 {
		t.Errorf("ExpiryPoints = %d, want > 0 (product 301 is in the expiring set)", suggestions[0].ExpiryPoints)
	}
	// "chicken" and "chicken breast" both match product 301; its points must
	// be counted once, not once per matched ingredient.
	if suggestions[0].ExpiryPoints > 7 {
		t.Errorf("ExpiryPoints = %d, want <= 7 (single product counted once)", suggestions[0].ExpiryPoints)
	}
}

func TestGetSuggestions_DisabledFeatureReturnsEmpty(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := &RecipeController{
		Logger:     &logger,
		RecipeRepo: nil,
		Config: configuration.RecipeAPIConfiguration{
			Provider: util.RecipeProviderMealie,
			URL:      "https://mealie.example.com",
			Timeout:  10,
		},
		disabled: true,
	}
	// A nil Provider would normally make fetchFromAPI fail; the disabled path
	// must short-circuit before any provider call so the endpoint stays a cheap
	// 200 instead of a 500 repeating a doomed outbound fan-out.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Milk", ExpireAt: time.Now().AddDate(0, 0, 1)},
	}
	called := false
	source := ProductSource{
		All: func() ([]dbModel.Product, error) {
			called = true
			return products, nil
		},
		Fingerprint: func() (string, error) { return "1:", nil },
	}

	suggestions, err := ctrl.GetSuggestions(products, source, 5, false)
	if err != nil {
		t.Errorf("GetSuggestions() error = %v, want nil", err)
	}
	if len(suggestions) != 0 {
		t.Errorf("GetSuggestions() disabled returned %d suggestions, want 0", len(suggestions))
	}
	if called {
		t.Error("GetSuggestions() disabled queried household products, want short-circuit")
	}
}

func TestBareIngredientName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "quantity and unit are stripped", input: "500 g chicken breast", want: "chicken breast"},
		{name: "count unit is stripped", input: "2 cloves garlic", want: "garlic"},
		{name: "filler words are stripped", input: "a pinch of salt", want: "salt"},
		{name: "volume unit is stripped", input: "200 ml milk", want: "milk"},
		{name: "glued quantity is stripped", input: "200ml milk", want: "milk"},
		{name: "fraction is stripped", input: "1/2 cup sugar", want: "sugar"},
		{name: "bare name is unchanged", input: "chicken", want: "chicken"},
		{name: "unit-like word inside a name is kept", input: "olive oil", want: "olive oil"},
		{name: "unit-like word at the end is kept", input: "coconut milk", want: "coconut milk"},
		{name: "name starting with a unit letter is kept", input: "cinnamon", want: "cinnamon"},
		{name: "quantity only collapses to empty", input: "500 g", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bareIngredientName(tt.input); got != tt.want {
				t.Errorf("bareIngredientName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Mealie reports ingredients as full lines ("500 g chicken breast") while
// Tandoor and TheMealDB report bare names. Without quantity-aware matching the
// exact-name rank never fires for Mealie, so the cook action and the
// expiry-first ranking silently no-op for that provider.
func TestMatchRecipesToProducts_QuantityBearingIngredientMatchesExactly(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Garlic Chicken", Ingredients: []string{"500 g chicken breast", "2 cloves garlic"}},
	}
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 501}, ProductName: "Chicken Breast", ExpireAt: time.Now().AddDate(0, 0, 1)},
		{Model: gorm.Model{ID: 502}, ProductName: "Garlic"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, products)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	got := suggestions[0].MatchedProductIDs
	if len(got) != 2 || got[0] != 501 || got[1] != 502 {
		t.Errorf("MatchedProductIDs = %v, want [501 502] for quantity-bearing ingredients", got)
	}
	// Only the expiring product contributes ranking points.
	if suggestions[0].ExpiryPoints <= 0 {
		t.Errorf("ExpiryPoints = %d, want > 0", suggestions[0].ExpiryPoints)
	}
}

// A quantity-only ingredient must not match everything: bareIngredientName can
// return an empty string, and strings.Contains with an empty needle is always
// true.
func TestMatchRecipesToProducts_QuantityOnlyIngredientMatchesNothing(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Water", Ingredients: []string{"500 g"}},
	}
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 601}, ProductName: "Milk", ExpireAt: time.Now().AddDate(0, 0, 1)},
	}

	if suggestions := ctrl.matchRecipesToProducts(recipes, products, products); len(suggestions) != 0 {
		t.Errorf("expected no suggestions for a quantity-only ingredient, got %d", len(suggestions))
	}
}

func TestBuildExpiryPoints_ClampedToSevenDays(t *testing.T) {
	now := time.Now()

	products := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ExpireAt: now.Add(-48 * time.Hour)}, // already expired → clamped to 0 days left
		{Model: gorm.Model{ID: 2}, ExpireAt: now.AddDate(0, 0, 30)},    // far out → clamped to 7 days left
		{Model: gorm.Model{ID: 3}, ExpireAt: time.Time{}},              // no date → no entry
	}

	points := buildExpiryPoints(products)

	if points[1] != 7 {
		t.Errorf("points[1] = %d, want 7 (expired product has maximum urgency)", points[1])
	}
	if points[2] != 0 {
		t.Errorf("points[2] = %d, want 0 (30 days out clamps to zero points)", points[2])
	}
	if _, ok := points[3]; ok {
		t.Errorf("points[3] exists, want no entry for product without expiry date")
	}
}

func TestMatchRecipesToProducts_UnnamedProductMatchesNothing(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Milk Dish", Ingredients: []string{"milk", "butter"}},
	}
	// A blank product name normalizes to "" and must never match any
	// ingredient, and must never be archived via the cook action.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 401}, ProductName: "   "},
		{Model: gorm.Model{ID: 402}, ProductName: "Milk"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	got := suggestions[0].MatchedProductIDs
	if len(got) != 1 || got[0] != 402 {
		t.Errorf("MatchedProductIDs = %v, want [402] (unnamed product must not be matched)", got)
	}
	for _, ing := range suggestions[0].Ingredients {
		if ing.Name == "butter" && ing.Matched {
			t.Errorf("ingredient %q matched, want no match for blank-name product", ing.Name)
		}
	}
}

func TestMatchRecipesToProducts_LooseSubstringDoesNotArchive(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Nut Dish", Ingredients: []string{"nut"}},
	}
	// "nut" only appears as a loose substring inside "coconut", so the
	// product must stay display-matched but must not be selected for the
	// destructive cook action.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 501}, ProductName: "Coconut"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if got := suggestions[0].MatchedProductIDs; len(got) != 0 {
		t.Errorf("MatchedProductIDs = %v, want no IDs for loose substring match", got)
	}
	if len(suggestions[0].MatchedProducts) != 1 {
		t.Errorf("MatchedProducts = %v, want the display match kept", suggestions[0].MatchedProducts)
	}
}

func TestMatchRecipesToProducts_WholeWordMatchDoesNotCook(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Milk Dish", Ingredients: []string{"milk"}},
	}
	// "milk" is a whole word inside "Coconut Milk" and "Milk Chocolate", but
	// neither product is the milk the recipe names. Both stay display-matched;
	// neither may be selected for the irreversible cook action.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 701}, ProductName: "Coconut Milk"},
		{Model: gorm.Model{ID: 702}, ProductName: "Milk Chocolate"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if got := suggestions[0].MatchedProductIDs; len(got) != 0 {
		t.Errorf("MatchedProductIDs = %v, want no IDs: only an exact name match may select a product for cooking", got)
	}
	if len(suggestions[0].MatchedProducts) != 1 {
		t.Errorf("MatchedProducts = %v, want the display match kept", suggestions[0].MatchedProducts)
	}
}

func TestMatchRecipesToProducts_CategoryMatchDoesNotCook(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Beef Dish", Ingredients: []string{"beef"}},
	}
	// A category tag is curated metadata, not a product identity: matching an
	// ingredient against "beef" must not let the cook action consume an
	// arbitrarily chosen product of that category.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 801}, ProductName: "Steak", Categories: "beef"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if got := suggestions[0].MatchedProductIDs; len(got) != 0 {
		t.Errorf("MatchedProductIDs = %v, want no IDs for a category-only match", got)
	}
}

func TestMatchRecipesToProducts_MostSpecificProductWins(t *testing.T) {
	ctrl := newTestRecipeController()

	recipes := []Recipe{
		{ID: "1", Title: "Milk Dish", Ingredients: []string{"milk"}},
	}
	// "oat milk" appears before "milk" in the unordered product list; the
	// exact name match must win over the earlier whole-word hit.
	products := []dbModel.Product{
		{Model: gorm.Model{ID: 601}, ProductName: "Oat Milk"},
		{Model: gorm.Model{ID: 602}, ProductName: "Milk"},
	}

	suggestions := ctrl.matchRecipesToProducts(recipes, products, nil)

	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	got := suggestions[0].MatchedProductIDs
	if len(got) != 1 || got[0] != 602 {
		t.Errorf("MatchedProductIDs = %v, want [602] (exact name beats earlier whole-word hit)", got)
	}
}

// stubRecipeProvider returns a fixed recipe set, standing in for a real
// provider so the cache-write decisions can be tested without network I/O.
type stubRecipeProvider struct {
	name     string
	recipes  []Recipe
	complete bool
}

func (s *stubRecipeProvider) Name() string { return s.name }

func (s *stubRecipeProvider) FetchRecipes([]string) ([]Recipe, bool, error) {
	return s.recipes, s.complete, nil
}

func newCachingRecipeController(provider RecipeProvider, populatedCache bool) (*RecipeController, *repomocks.MockRecipeRepository) {
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{
		URL:          "https://provider.test/api",
		Timeout:      10,
		CacheEnabled: true,
		CacheTTL:     24,
		Provider:     util.RecipeProviderThemealDB,
	}
	repo := &repomocks.MockRecipeRepository{PopulatedCache: populatedCache}
	return &RecipeController{
		Logger:     &logger,
		Config:     config,
		RecipeRepo: repo,
		Provider:   provider,
	}, repo
}

func cachingTestProducts() ([]dbModel.Product, ProductSource) {
	expiring := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Chicken", Categories: "meat", ExpireAt: time.Now().AddDate(0, 0, 2)},
	}
	all := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Chicken", Categories: "meat", ExpireAt: time.Now().AddDate(0, 0, 2)},
		{Model: gorm.Model{ID: 2}, ProductName: "Rice", Categories: "grain"},
	}
	source := ProductSource{
		All:         func() ([]dbModel.Product, error) { return all, nil },
		Fingerprint: func() (string, error) { return "2:2026-10-03", nil },
	}
	return expiring, source
}

var matchingRecipe = []Recipe{
	{ID: "r1", Title: "Chicken Rice", Ingredients: []string{"chicken", "rice"}},
}

// A degraded run must never replace a good entry. CreateCache is an
// unconditional upsert on query_hash, so an empty or partial result used to
// overwrite a complete one and hold the household on the degraded answer for
// the whole TTL of the entry it destroyed.
func TestGetSuggestions_DoesNotDowngradeAPopulatedCacheEntry(t *testing.T) {
	t.Run("empty result keeps an existing good entry", func(t *testing.T) {
		provider := &stubRecipeProvider{name: util.RecipeProviderThemealDB, recipes: []Recipe{}, complete: true}
		ctrl, repo := newCachingRecipeController(provider, true)
		expiring, source := cachingTestProducts()

		suggestions, err := ctrl.GetSuggestions(expiring, source, 6, false)
		if err != nil {
			t.Fatalf("GetSuggestions() error = %v", err)
		}
		if len(suggestions) != 0 {
			t.Errorf("got %d suggestions, want 0 (the provider returned nothing)", len(suggestions))
		}
		if len(repo.CacheWrites) != 0 {
			t.Errorf("stored %d cache entries, want 0 — an empty result must not replace a populated entry", len(repo.CacheWrites))
		}
	})

	t.Run("partial result keeps an existing good entry", func(t *testing.T) {
		provider := &stubRecipeProvider{name: util.RecipeProviderThemealDB, recipes: matchingRecipe, complete: false}
		ctrl, repo := newCachingRecipeController(provider, true)
		expiring, source := cachingTestProducts()

		if _, err := ctrl.GetSuggestions(expiring, source, 6, false); err != nil {
			t.Fatalf("GetSuggestions() error = %v", err)
		}
		if len(repo.CacheWrites) != 0 {
			t.Errorf("stored %d cache entries, want 0 — an incomplete result must not replace a populated entry", len(repo.CacheWrites))
		}
	})

	t.Run("complete result still replaces an existing entry", func(t *testing.T) {
		provider := &stubRecipeProvider{name: util.RecipeProviderThemealDB, recipes: matchingRecipe, complete: true}
		ctrl, repo := newCachingRecipeController(provider, true)
		expiring, source := cachingTestProducts()

		suggestions, err := ctrl.GetSuggestions(expiring, source, 6, false)
		if err != nil {
			t.Fatalf("GetSuggestions() error = %v", err)
		}
		if len(suggestions) == 0 {
			t.Fatal("got 0 suggestions, want at least 1")
		}
		if len(repo.CacheWrites) != 1 {
			t.Errorf("stored %d cache entries, want 1 — a complete result is the best data and must win", len(repo.CacheWrites))
		}
	})
}

// With no populated entry the negative cache must still be written, otherwise
// an uncached key makes every later non-refresh request repeat the full
// provider fan-out.
func TestGetSuggestions_NegativeCacheStillWrittenWhenNothingPopulated(t *testing.T) {
	provider := &stubRecipeProvider{name: util.RecipeProviderThemealDB, recipes: []Recipe{}, complete: true}
	ctrl, repo := newCachingRecipeController(provider, false)
	expiring, source := cachingTestProducts()

	if _, err := ctrl.GetSuggestions(expiring, source, 6, false); err != nil {
		t.Fatalf("GetSuggestions() error = %v", err)
	}
	if len(repo.CacheWrites) != 1 {
		t.Fatalf("stored %d cache entries, want 1 — the empty result must be cached to suppress repeat fan-out", len(repo.CacheWrites))
	}
	if repo.CacheWrites[0].QueryHash == "" {
		t.Error("cached entry has an empty QueryHash")
	}
}
