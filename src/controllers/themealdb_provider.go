package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

// mealDBCategories are the categories TheMealDB serves. A keyword naming one of
// them is searched through the category filter, which returns every meal of
// that category — the only TheMealDB endpoint that reaches a household by the
// kind of thing it keeps rather than by the name of a dish.
var mealDBCategories = []string{
	"beef", "breakfast", "chicken", "dessert", "goat", "lamb", "miscellaneous",
	"pasta", "pork", "seafood", "side", "starter", "vegan", "vegetarian",
}

// mealDBCategoryAliases maps the category vocabulary of the product database
// onto TheMealDB's own categories. OpenFoodFacts spells its categories in
// prose ("Breakfasts", "Fishes and their products", "Dairies"), so a keyword
// that names a TheMealDB category only does so by accident; the aliases carry
// that knowledge, which belongs to this provider and nowhere else.
//
// The list is ordered: the first category that returns meals wins, so the more
// specific category comes first.
var mealDBCategoryAliases = map[string][]string{
	"breakfasts":                        {"breakfast"},
	"fishes and their products":         {"seafood"},
	"fish and fish products":            {"seafood"},
	"canned fish":                       {"seafood"},
	"plant based foods":                 {"vegetarian"},
	"plant based foods and beverages":   {"vegetarian"},
	"fruits and vegetables based foods": {"vegetarian"},
	"vegetables":                        {"vegetarian"},
	"dairies":                           {"breakfast", "dessert"},
	"dairy products":                    {"breakfast", "dessert"},
	"fermented milk products":           {"breakfast", "dessert"},
	"fermented foods":                   {"breakfast", "dessert"},
	"milk and cream":                    {"breakfast", "dessert"},
	"cheese":                            {"breakfast", "dessert"},
	"spreads":                           {"miscellaneous", "starter"},
	"sweet spreads":                     {"miscellaneous", "starter"},
	"frozen foods":                      {"side", "starter"},
}

const (
	maxSearchRequestsPerRun = 40
	maxIngredientTerms = 2
	maxIngredientTermWords = 3
	minIngredientTermLength = 3
	maxIngredientTermLength = 24
)

var ingredientTermStopWords = map[string]bool{
	"and": true, "with": true, "for": true, "from": true, "the": true, "of": true,
	"in": true, "on": true, "to": true, "plus": true, "fresh": true, "bio": true,
	"organic": true, "natural": true, "nature": true, "complete": true,
	"completes": true, "extra": true, "soft": true, "sans": true, "au": true,
	"aux": true, "der": true, "die": true, "das": true, "und": true, "mit": true,
	"ohne": true, "fur": true, "dark": true, "light": true, "plain": true,
	"salted": true, "unsalted": true, "sweet": true, "sweetened": true,
	"mixed": true, "assorted": true, "flavored": true, "flavoured": true,
	"premium": true, "classic": true, "protein": true, "diet": true,
	"whole": true, "chopped": true, "diced": true, "sliced": true,
	"grated": true, "minced": true, "crushed": true, "canned": true,
	"frozen": true, "dried": true, "roasted": true, "boiled": true,
	"cooked": true, "raw": true, "red": true, "green": true, "white": true,
	"yellow": true, "blue": true, "black": true, "brown": true,
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
	Term string
}

// searchBudget bounds the outbound searches of one fetch run. The searches of
// the keywords run concurrently, so the budget is claimed atomically.
type searchBudget struct {
	remaining atomic.Int64
	exhausted atomic.Bool
}

// take claims one search. It reports false once the budget is spent, and
// remembers that it ran out so the run can be reported as incomplete: a
// truncated answer is a partial view of the provider and must not be cached
// as its full answer.
func (b *searchBudget) take() bool {
	if b.remaining.Add(-1) < 0 {
		b.exhausted.Store(true)
		return false
	}
	return true
}

// FetchRecipes retrieves recipes from TheMealDB using keywords.
// Strategy: category search for a keyword naming a category, then the
// ingredient filter for the food word inside a keyword, then a meal-name
// search as the last resort. Meal summaries are deduplicated by ID across
// keywords before a single bounded detail pass, so no meal is looked up twice.
func (p *TheMealDBProvider) FetchRecipes(keywords []string) ([]Recipe, bool, error) {
	ctx, cancel := recipeFetchContext(p.Config)
	defer cancel()

	budget := &searchBudget{}
	budget.remaining.Store(maxSearchRequestsPerRun)

	search := func(ctx context.Context, kw string) ([]*mealStub, error) {
		for _, category := range mealDBCategoriesFor(kw) {
			if !budget.take() {
				return nil, nil
			}
			stubs, err := p.fetchCategoryStubs(ctx, category)
			if err != nil {
				return nil, err
			}
			if len(stubs) > 0 {
				return stampStubs(stubs, kw), nil
			}
		}
		for _, term := range ingredientTerms(kw) {
			if !budget.take() {
				return nil, nil
			}
			stubs, err := p.fetchIngredientStubs(ctx, term)
			if err != nil {
				return nil, err
			}
			if len(stubs) > 0 {
				return stampStubs(stubs, term), nil
			}
		}
		if !budget.take() {
			return nil, nil
		}
		stubs, err := p.fetchTextStubs(ctx, kw)
		if err != nil {
			return nil, err
		}
		return stampStubs(stubs, kw), nil
	}

	stubs, okSearches := searchUnique(ctx, p.Logger, keywords, search,
		func(item *mealStub) (string, bool) { return item.ID, item.ID != "" },
		maxRecipesPerRun,
	)

	recipes, failedDetails := fetchDetailsConcurrently(ctx, stubs, detailFetchWorkers, p.fetchDetailsStub,
		func(item *mealStub, err error) {
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

	return deduplicateRecipes(recipes), outcome.complete() && !budget.exhausted.Load(), nil
}

// stampStubs records the search term on every stub of one search.
func stampStubs(stubs []*mealStub, term string) []*mealStub {
	for i := range stubs {
		stubs[i].Term = term
	}
	return stubs
}

// mealDBCategoriesFor returns the TheMealDB categories a keyword names, most
// specific first. A keyword naming none of them yields no categories.
func mealDBCategoriesFor(kw string) []string {
	normalized := normalizeString(kw)
	if normalized == "" {
		return nil
	}
	if slices.Contains(mealDBCategories, normalized) {
		return []string{normalized}
	}
	return mealDBCategoryAliases[normalized]
}

// ingredientTerms returns the ingredient names to try for one keyword. The
// keyword itself is tried first because a product may be named after the food
// it is ("Butter"), then its words, shortest first: the head noun of a
// product name is the food it is ("Quaker Oats" → "oats", "chopped tomatoes"
// → "tomatoes"), and a short word is that head noun more often than a long
// one is.
func ingredientTerms(keyword string) []string {
	normalized := normalizeString(keyword)
	if normalized == "" {
		return nil
	}
	words := strings.Fields(normalized)

	terms := make([]string, 0, maxIngredientTerms)
	if len(words) <= maxIngredientTermWords && !containsDigit(words) {
		terms = append(terms, normalized)
	}

	candidates := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) < minIngredientTermLength || len(word) > maxIngredientTermLength {
			continue
		}
		if ingredientTermStopWords[word] {
			continue
		}
		if containsDigit([]string{word}) {
			continue
		}
		candidates = append(candidates, word)
	}
	slices.SortStableFunc(candidates, func(a, b string) int {
		return len(a) - len(b)
	})
	for _, candidate := range candidates {
		if len(terms) >= maxIngredientTerms {
			break
		}
		if slices.Contains(terms, candidate) {
			continue
		}
		terms = append(terms, candidate)
	}
	return terms
}

// containsDigit reports whether any of the words carries a digit. A word with
// a digit is a weight or a pack size, never an ingredient name.
func containsDigit(words []string) bool {
	for _, word := range words {
		if strings.ContainsFunc(word, func(r rune) bool { return r >= '0' && r <= '9' }) {
			return true
		}
	}
	return false
}

// fetchCategoryStubs retrieves meal summaries for a given category.
func (p *TheMealDBProvider) fetchCategoryStubs(ctx context.Context, category string) ([]*mealStub, error) {
	fetchURL := fmt.Sprintf("%s/filter.php?c=%s", p.baseURL(), url.QueryEscape(category))
	return p.fetchStubs(ctx, fetchURL)
}

// fetchIngredientStubs retrieves meal summaries for every meal listing the
// given ingredient. The endpoint matches an ingredient name exactly, which is
// what reaches a household whose products are named after the food they are
// rather than after a dish.
func (p *TheMealDBProvider) fetchIngredientStubs(ctx context.Context, ingredient string) ([]*mealStub, error) {
	fetchURL := fmt.Sprintf("%s/filter.php?i=%s", p.baseURL(), url.QueryEscape(ingredient))
	return p.fetchStubs(ctx, fetchURL)
}

// fetchTextStubs searches meal summaries by text query (typically ingredient).
func (p *TheMealDBProvider) fetchTextStubs(ctx context.Context, query string) ([]*mealStub, error) {
	fetchURL := fmt.Sprintf("%s/search.php?s=%s", p.baseURL(), url.QueryEscape(query))
	return p.fetchStubs(ctx, fetchURL)
}

// fetchStubs retrieves meal summaries without triggering detail lookups.
func (p *TheMealDBProvider) fetchStubs(ctx context.Context, fetchURL string) ([]*mealStub, error) {
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

	stubs := make([]*mealStub, 0, len(result.Meals))
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
		stubs = append(stubs, &mealStub{
			ID:       meal.IDMeal,
			Title:    meal.StrMeal,
			ImageURL: meal.StrMealThumb,
			Source:   meal.StrSource,
		})
	}
	return stubs, nil
}

// fetchDetailsStub retrieves the full recipe including ingredients for a stub.
func (p *TheMealDBProvider) fetchDetailsStub(ctx context.Context, stub *mealStub) (Recipe, error) {
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
	recipe.SearchTerm = stub.Term
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
