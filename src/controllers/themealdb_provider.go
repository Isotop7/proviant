package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

var knownMealCategories = []string{"beef", "chicken", "pork", "lamb", "vegan", "vegetarian",
	"fish", "seafood", "pasta", "dessert", "breakfast",
}

// TheMealDBProvider fetches recipes from TheMealDB (free API, no auth).
type TheMealDBProvider struct {
	Logger     *zerolog.Logger
	Config     configuration.RecipeAPIConfiguration
	HTTPClient *http.Client
}

// Name returns the provider identifier.
func (p *TheMealDBProvider) Name() string {
	return p.Config.Provider
}

// baseURL returns the configured API URL without a trailing slash.
func (p *TheMealDBProvider) baseURL() string {
	return strings.TrimSuffix(p.Config.URL, "/")
}

// mealStub is a meal summary collected across keywords before the shared
// detail pass, so no meal is ever looked up twice.
type mealStub struct {
	ID       string
	Title    string
	ImageURL string
	Source   string
}

// FetchRecipes retrieves recipes from TheMealDB using keywords.
// Strategy: try category-based search first for known categories, then fallback
// to text search. Meal summaries are deduplicated by ID across keywords before
// a single bounded detail pass, so no meal is looked up twice.
func (p *TheMealDBProvider) FetchRecipes(keywords []string) ([]Recipe, bool, error) {
	ctx, cancel := recipeFetchContext(p.Config)
	defer cancel()

	search := func(ctx context.Context, kw string) ([]mealStub, error) {
		if cat, ok := matchKnownCategory(kw); ok {
			return p.fetchCategoryStubs(ctx, cat)
		}
		return p.fetchTextStubs(ctx, kw)
	}

	stubs, okSearches := searchUnique(ctx, p.Logger, keywords, search,
		func(item mealStub) (string, bool) { return item.ID, item.ID != "" },
		maxRecipesPerRun,
	)

	recipes, failedDetails := fetchDetailsConcurrently(ctx, stubs, detailFetchWorkers, p.fetchDetailsStub,
		func(item mealStub, err error) {
			p.Logger.Debug().Str("id", item.ID).Err(err).Msg("failed to fetch meal details")
		})
	outcome := fetchOutcome{
		totalSearches: len(keywords),
		okSearches:    okSearches,
		totalDetails:  len(stubs),
		failedDetails: failedDetails,
	}
	if err := outcome.unavailable(); err != nil {
		return nil, false, err
	}

	return deduplicateRecipes(recipes), outcome.complete(), nil
}

// matchKnownCategory returns the known meal category that kw maps to, or ("", false).
// The category must appear as a whole word inside the keyword; plain substring
// matching would misroute short keywords (e.g. "ham" matching "lamb").
func matchKnownCategory(kw string) (string, bool) {
	for _, cat := range knownMealCategories {
		if kw == cat || strings.HasPrefix(kw, cat+" ") ||
			strings.HasSuffix(kw, " "+cat) || strings.Contains(kw, " "+cat+" ") {
			return cat, true
		}
	}
	return "", false
}

// fetchCategoryStubs retrieves meal summaries for a given category.
func (p *TheMealDBProvider) fetchCategoryStubs(ctx context.Context, category string) ([]mealStub, error) {
	fetchURL := fmt.Sprintf("%s/filter.php?c=%s", p.baseURL(), url.QueryEscape(category))
	return p.fetchStubs(ctx, fetchURL)
}

// fetchTextStubs searches meal summaries by text query (typically ingredient).
func (p *TheMealDBProvider) fetchTextStubs(ctx context.Context, query string) ([]mealStub, error) {
	fetchURL := fmt.Sprintf("%s/search.php?s=%s", p.baseURL(), url.QueryEscape(query))
	return p.fetchStubs(ctx, fetchURL)
}

// fetchStubs retrieves meal summaries without triggering detail lookups.
func (p *TheMealDBProvider) fetchStubs(ctx context.Context, fetchURL string) ([]mealStub, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.Logger.Debug().Err(err).Msg(MsgFailedCloseResponseBody)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(MsgApiReturnWrapper, resp.StatusCode)
	}

	var result struct {
		Meals []json.RawMessage `json:"meals"`
	}
	if err := decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}

	stubs := make([]mealStub, 0, len(result.Meals))
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
		stubs = append(stubs, mealStub{
			ID:       meal.IDMeal,
			Title:    meal.StrMeal,
			ImageURL: meal.StrMealThumb,
			Source:   meal.StrSource,
		})
	}
	return stubs, nil
}

// fetchDetailsStub retrieves the full recipe including ingredients for a stub.
func (p *TheMealDBProvider) fetchDetailsStub(ctx context.Context, stub mealStub) (Recipe, error) {
	recipe, err := p.fetchDetails(ctx, stub.ID)
	if err != nil {
		return Recipe{}, err
	}
	if recipe.Title == "" {
		recipe.Title = stub.Title
	}
	if recipe.ImageURL == "" {
		recipe.ImageURL = stub.ImageURL
	}
	if recipe.SourceURL == "" {
		recipe.SourceURL = stub.Source
	}
	return recipe, nil
}

// fetchDetails retrieves the full recipe including ingredients.
func (p *TheMealDBProvider) fetchDetails(ctx context.Context, mealID string) (Recipe, error) {
	fetchURL := fmt.Sprintf("%s/lookup.php?i=%s", p.baseURL(), url.QueryEscape(mealID))
	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return Recipe{}, err
	}
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return Recipe{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.Logger.Debug().Err(err).Msg(MsgFailedCloseResponseBody)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return Recipe{}, fmt.Errorf(MsgApiReturnWrapper, resp.StatusCode)
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
	if err := decodeJSONResponse(resp, &result); err != nil {
		return Recipe{}, err
	}
	if len(result.Meals) == 0 {
		return Recipe{}, fmt.Errorf("no meal found")
	}
	m := result.Meals[0]

	ingredients := extractPointerIngredients(
		m.StrIngredient1, m.StrIngredient2, m.StrIngredient3, m.StrIngredient4,
		m.StrIngredient5, m.StrIngredient6, m.StrIngredient7, m.StrIngredient8,
		m.StrIngredient9, m.StrIngredient10, m.StrIngredient11, m.StrIngredient12,
		m.StrIngredient13, m.StrIngredient14, m.StrIngredient15, m.StrIngredient16,
		m.StrIngredient17, m.StrIngredient18, m.StrIngredient19, m.StrIngredient20,
	)

	p.Logger.Debug().Int("ingredient_count", len(ingredients)).Strs("ingredients", ingredients).Msg("Fetched recipe details")

	recipe := Recipe{
		ID:          m.IDMeal,
		Title:       m.StrMeal,
		ImageURL:    m.StrMealThumb,
		SourceURL:   m.StrSource,
		Ingredients: ingredients,
	}
	return recipe, nil
}

// extractPointerIngredients converts up to 20 string pointers into a non-empty string slice.
func extractPointerIngredients(ings ...*string) []string {
	out := make([]string, 0, len(ings))
	for _, ptr := range ings {
		if ptr != nil && *ptr != "" {
			out = append(out, *ptr)
		}
	}
	return out
}
