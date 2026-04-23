package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	recipeRepo "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// RecipeController handles recipe suggestions from external APIs.
type RecipeController struct {
	Logger     *zerolog.Logger
	Config     configuration.RecipeAPIConfiguration
	RecipeRepo *recipeRepo.RecipeRepository
	HTTPClient *http.Client
}

// Recipe represents a normalized recipe from TheMealDB.
type Recipe struct {
	ID          string
	Title       string
	ImageURL    string
	SourceURL   string
	Ingredients []string
}

// RecipeSuggestion is a recipe matched to household products.
type RecipeSuggestion struct {
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	ImageURL         string                `json:"imageUrl"`
	SourceURL        string                `json:"sourceUrl"`
	Ingredients      []api.IngredientMatch `json:"ingredients"`
	MatchedProducts  []string              `json:"matchedProducts"` // kept for backwards compatibility
	MissingCount     int                   `json:"missingCount"`
	TotalIngredients int                   `json:"totalIngredients"`
	MatchPercent     float64               `json:"matchPercent"`
}

// NewRecipeController creates a new controller.
func NewRecipeController(config configuration.RecipeAPIConfiguration, logger *zerolog.Logger, db *gorm.DB) *RecipeController {
	repo := recipeRepo.NewRecipeRepository(db)
	client := &http.Client{
		Timeout: time.Second * time.Duration(config.Timeout),
	}
	return &RecipeController{
		Logger:     logger,
		Config:     config,
		RecipeRepo: repo,
		HTTPClient: client,
	}
}

// GetSuggestions returns up to limit recipe suggestions based on expiring products.
func (rc *RecipeController) GetSuggestions(expiringProducts []dbModel.Product, allProducts []dbModel.Product, limit int) ([]RecipeSuggestion, error) {
	if len(expiringProducts) == 0 {
		return []RecipeSuggestion{}, nil
	}

	// Build cache key from sorted product IDs (expiring set)
	expiringIDs := make([]uint, 0, len(expiringProducts))
	for i := range expiringProducts {
		p := &expiringProducts[i]
		expiringIDs = append(expiringIDs, p.ID)
	}
	sort.Slice(expiringIDs, func(i, j int) bool { return expiringIDs[i] < expiringIDs[j] })

	cacheKey := dbModel.GenerateCacheKey(rc.Config.Provider, expiringIDs)

	// Try cache
	if rc.Config.CacheEnabled {
		cached, err := rc.RecipeRepo.GetCacheByQueryHash(cacheKey)
		if err == nil && !cached.IsExpired() {
			var suggestions []RecipeSuggestion
			if err := json.Unmarshal(cached.ResponseJSON, &suggestions); err == nil {
				// Detect stale cache entries (created before Ingredients field existed)
				stale := false
				for i := range suggestions {
					s := &suggestions[i]
					if len(s.Ingredients) == 0 && s.TotalIngredients > 0 {
						stale = true
						break
					}
				}
				if !stale {
					if err := rc.RecipeRepo.UpdateCacheHit(cacheKey); err != nil {
						rc.Logger.Warn().Err(err).Msg("failed to update cache hit count")
					}
					rc.Logger.Debug().Msg("recipe suggestions cache hit")
					return rc.limitSuggestions(suggestions, limit), nil
				}
				rc.Logger.Debug().Msg("stale cache detected, refreshing")
			}
		}
	}

	// Fetch from API: aggregate categories and keywords from expiring products
	keywords := rc.collectKeywords(expiringProducts)

	// Fetch recipes from TheMealDB (by category first, then by keyword if needed)
	rawRecipes, err := rc.fetchFromAPI(keywords)
	if err != nil {
		rc.Logger.Error().Err(err).Msg("failed to fetch recipes from API")
		return nil, errors.ErrRecipeAPIUnavailable
	}

	// Match recipes against ALL household products
	matched := rc.matchRecipesToProducts(rawRecipes, allProducts)
	if len(matched) == 0 {
		return []RecipeSuggestion{}, nil
	}

	// Defensive: ensure Ingredients is never nil (should be set by matchRecipesToProducts)
	for i := range matched {
		if matched[i].Ingredients == nil {
			matched[i].Ingredients = []api.IngredientMatch{}
		}
	}

	// Serialize and cache
	data, err := json.Marshal(matched)
	if err != nil {
		rc.Logger.Warn().Err(err).Msg("failed to marshal recipe suggestions for caching")
	} else if rc.Config.CacheEnabled {
		ttl := time.Duration(rc.Config.CacheTTL) * time.Hour
		cacheEntry := dbModel.RecipeCache{
			QueryHash:    cacheKey,
			Provider:     rc.Config.Provider,
			ResponseJSON: data,
			ExpiresAt:    time.Now().Add(ttl),
		}
		if err := rc.RecipeRepo.CreateCache(&cacheEntry); err != nil {
			rc.Logger.Warn().Err(err).Msg("failed to store recipe cache entry")
		}
	}

	return rc.limitSuggestions(matched, limit), nil
}

// collectKeywords builds a set of category tokens and product name tokens from expiring products.
func (rc *RecipeController) collectKeywords(products []dbModel.Product) []string {
	keywordSet := make(map[string]struct{})

	for i := range products {
		p := &products[i]
		// Add normalized product name tokens
		name := normalizeString(p.ProductName)
		if name != "" {
			keywordSet[name] = struct{}{}
		}

		// Add category tokens (comma-separated)
		for _, cat := range strings.Split(p.Categories, ",") {
			cat = normalizeString(cat)
			if cat != "" && cat != "food" && cat != "de:lebensmittel" {
				keywordSet[cat] = struct{}{}
			}
		}
	}

	keywords := make([]string, 0, len(keywordSet))
	for k := range keywordSet {
		keywords = append(keywords, k)
	}
	return keywords
}

// fetchFromAPI retrieves recipes from TheMealDB using keywords.
// Strategy: try category-based search first for known categories, then fallback to text search.
func (rc *RecipeController) fetchFromAPI(keywords []string) ([]Recipe, error) {
	var allRecipes []Recipe

	// TheMealDB: try category-based search for known categories
	knownCategories := []string{"beef", "chicken", "pork", "lamb", "vegan", "vegetarian", "fish", "seafood", "pasta", "dessert", "breakfast"}
	categoryRecipes := make(map[string][]Recipe)

	for _, kw := range keywords {
		// If keyword is a known category, fetch by category
		isCategory := false
		for _, cat := range knownCategories {
			if kw == cat || strings.Contains(kw, cat) || strings.Contains(cat, kw) {
				isCategory = true
				recipes, err := rc.fetchByCategory(cat)
				if err == nil && len(recipes) > 0 {
					categoryRecipes[cat] = recipes
				}
				break
			}
		}
		if isCategory {
			continue
		}

		// General text search for ingredient or dish name
		recipes, err := rc.searchByText(kw)
		if err == nil && len(recipes) > 0 {
			allRecipes = append(allRecipes, recipes...)
		}
	}

	// Merge category-based recipes (avoid duplicates by ID)
	seen := make(map[string]bool)
	for _, recipes := range categoryRecipes {
		for _, r := range recipes {
			if !seen[r.ID] {
				seen[r.ID] = true
				allRecipes = append(allRecipes, r)
			}
		}
	}

	// Deduplicate allRecipes by ID
	unique := make([]Recipe, 0, len(allRecipes))
	seen = make(map[string]bool)
	for _, r := range allRecipes {
		if !seen[r.ID] {
			seen[r.ID] = true
			unique = append(unique, r)
		}
	}

	return unique, nil
}

// fetchByCategory retrieves meals for a given category from TheMealDB.
func (rc *RecipeController) fetchByCategory(category string) ([]Recipe, error) {
	url := fmt.Sprintf("%s/filter.php?c=%s", rc.Config.URL, category)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := rc.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rc.Logger.Debug().Err(err).Msg("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result struct {
		Meals []struct {
			IDMeal       string `json:"idMeal"`
			StrMeal      string `json:"strMeal"`
			StrMealThumb string `json:"strMealThumb"`
			StrSource    string `json:"strSource"`
		} `json:"meals"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	recipes := make([]Recipe, 0, len(result.Meals))
	for _, m := range result.Meals {
		// Fetch full details for ingredients via lookup endpoint
		full, err := rc.fetchDetails(m.IDMeal)
		if err != nil {
			rc.Logger.Debug().Str("id", m.IDMeal).Err(err).Msg("failed to fetch meal details")
			continue
		}
		recipes = append(recipes, full)
	}
	return recipes, nil
}

// searchByText searches meals by text query (typically ingredient).
func (rc *RecipeController) searchByText(query string) ([]Recipe, error) {
	url := fmt.Sprintf("%s/search.php?s=%s", rc.Config.URL, query)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := rc.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rc.Logger.Debug().Err(err).Msg("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result struct {
		Meals []json.RawMessage `json:"meals"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	recipes := make([]Recipe, 0, len(result.Meals))
	for _, raw := range result.Meals {
		var meal struct {
			IDMeal       string `json:"idMeal"`
			StrMeal      string `json:"strMeal"`
			StrMealThumb string `json:"strMealThumb"`
			StrSource    string `json:"strSource"`
		}
		if err := json.Unmarshal(raw, &meal); err != nil {
			continue
		}
		full, err := rc.fetchDetails(meal.IDMeal)
		if err != nil {
			rc.Logger.Debug().Str("id", meal.IDMeal).Err(err).Msg("failed to fetch meal details")
			continue
		}
		recipes = append(recipes, full)
	}
	return recipes, nil
}

// fetchDetails retrieves the full recipe including ingredients.
func (rc *RecipeController) fetchDetails(mealID string) (Recipe, error) {
	url := fmt.Sprintf("%s/lookup.php?i=%s", rc.Config.URL, mealID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Recipe{}, err
	}
	resp, err := rc.HTTPClient.Do(req)
	if err != nil {
		return Recipe{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rc.Logger.Debug().Err(err).Msg("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return Recipe{}, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result struct {
		Meals []struct {
			IDMeal          string  `json:"idMeal"`
			StrMeal         string  `json:"strMeal"`
			StrMealThumb    string  `json:"strMealThumb"`
			StrSource       string  `json:"strSource"`
			StrIngredient1  *string `json:"strIngredient1"`
			StrIngredient2  *string `json:"strIngredient2"`
			StrIngredient3  *string `json:"strIngredient3"`
			StrIngredient4  *string `json:"strIngredient4"`
			StrIngredient5  *string `json:"strIngredient5"`
			StrIngredient6  *string `json:"strIngredient6"`
			StrIngredient7  *string `json:"strIngredient7"`
			StrIngredient8  *string `json:"strIngredient8"`
			StrIngredient9  *string `json:"strIngredient9"`
			StrIngredient10 *string `json:"strIngredient10"`
			StrIngredient11 *string `json:"strIngredient11"`
			StrIngredient12 *string `json:"strIngredient12"`
			StrIngredient13 *string `json:"strIngredient13"`
			StrIngredient14 *string `json:"strIngredient14"`
			StrIngredient15 *string `json:"strIngredient15"`
			StrIngredient16 *string `json:"strIngredient16"`
			StrIngredient17 *string `json:"strIngredient17"`
			StrIngredient18 *string `json:"strIngredient18"`
			StrIngredient19 *string `json:"strIngredient19"`
			StrIngredient20 *string `json:"strIngredient20"`
		} `json:"meals"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Recipe{}, err
	}
	if len(result.Meals) == 0 {
		return Recipe{}, fmt.Errorf("no meal found")
	}
	m := result.Meals[0]

	ingredients := rc.extractIngredients(
		m.StrIngredient1, m.StrIngredient2, m.StrIngredient3, m.StrIngredient4,
		m.StrIngredient5, m.StrIngredient6, m.StrIngredient7, m.StrIngredient8,
		m.StrIngredient9, m.StrIngredient10, m.StrIngredient11, m.StrIngredient12,
		m.StrIngredient13, m.StrIngredient14, m.StrIngredient15, m.StrIngredient16,
		m.StrIngredient17, m.StrIngredient18, m.StrIngredient19, m.StrIngredient20,
	)

	rc.Logger.Debug().Int("ingredient_count", len(ingredients)).Strs("ingredients", ingredients).Msg("Fetched recipe details")

	recipe := Recipe{
		ID:          m.IDMeal,
		Title:       m.StrMeal,
		ImageURL:    m.StrMealThumb,
		SourceURL:   m.StrSource,
		Ingredients: ingredients,
	}
	return recipe, nil
}

// extractIngredients converts up to 20 string pointers into a non-empty string slice.
func (rc *RecipeController) extractIngredients(ings ...*string) []string {
	out := make([]string, 0, len(ings))
	for _, p := range ings {
		if p != nil && *p != "" {
			out = append(out, *p)
		}
	}
	return out
}

// matchRecipesToProducts matches recipes against the full product list and returns suggestions.
func (rc *RecipeController) matchRecipesToProducts(recipes []Recipe, allProducts []dbModel.Product) []RecipeSuggestion {
	suggestions := make([]RecipeSuggestion, 0)

	// Build searchable index: lowercase name + categories per product
	type productIndex struct {
		name       string
		categories []string
	}
	normalized := make([]productIndex, len(allProducts))
	for i := range allProducts {
		p := &allProducts[i]
		normName := normalizeString(p.ProductName)
		var cats []string
		for _, c := range strings.Split(p.Categories, ",") {
			nc := normalizeString(c)
			if nc != "" {
				cats = append(cats, nc)
			}
		}
		normalized[i] = productIndex{name: normName, categories: cats}
	}

	for _, recipe := range recipes {
		rc.Logger.Debug().Str("recipe", recipe.Title).Int("ingredients_total", len(recipe.Ingredients)).Msg("Matching recipe")
		ingredientMatches := make([]api.IngredientMatch, 0, len(recipe.Ingredients))
		var matched []string

		for _, ingredient := range recipe.Ingredients {
			normIng := normalizeString(ingredient)
			isMatched := false
			if normIng != "" {
				for _, np := range normalized {
					// Match product name
					if strings.Contains(np.name, normIng) || strings.Contains(normIng, np.name) {
						isMatched = true
						break
					}
					// Match category
					for _, cat := range np.categories {
						if strings.Contains(cat, normIng) || strings.Contains(normIng, cat) {
							isMatched = true
							break
						}
					}
					if isMatched {
						break
					}
				}
			}
			ingredientMatches = append(ingredientMatches, api.IngredientMatch{
				Name:    ingredient,
				Matched: isMatched,
			})
			if isMatched {
				matched = append(matched, ingredient)
			}
		}

		if len(matched) > 0 {
			missing := len(recipe.Ingredients) - len(matched)
			matchPct := float64(len(matched)) / float64(len(recipe.Ingredients)) * 100.0
			suggestions = append(suggestions, RecipeSuggestion{
				ID:               recipe.ID,
				Title:            recipe.Title,
				ImageURL:         recipe.ImageURL,
				SourceURL:        recipe.SourceURL,
				Ingredients:      ingredientMatches,
				MatchedProducts:  matched,
				MissingCount:     missing,
				TotalIngredients: len(recipe.Ingredients),
				MatchPercent:     matchPct,
			})
		}
	}

	// Sort: match% desc, then missingCount asc
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].MatchPercent == suggestions[j].MatchPercent {
			return suggestions[i].MissingCount < suggestions[j].MissingCount
		}
		return suggestions[i].MatchPercent > suggestions[j].MatchPercent
	})

	return suggestions
}

// limitSuggestions returns at most limit items.
func (rc *RecipeController) limitSuggestions(suggestions []RecipeSuggestion, limit int) []RecipeSuggestion {
	if limit <= 0 {
		limit = 6
	}
	if limit > 10 {
		limit = 10
	}
	if len(suggestions) <= limit {
		return suggestions
	}
	return suggestions[:limit]
}

// normalizeString lowercases, strips punctuation, replaces hyphens with spaces.
func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.TrimSpace(s)
	return s
}
