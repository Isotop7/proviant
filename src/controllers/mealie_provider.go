package controllers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

// MealieProvider fetches recipes from a self-hosted Mealie instance.
type MealieProvider struct {
	Logger     *zerolog.Logger
	Config     configuration.RecipeAPIConfiguration
	HTTPClient *http.Client
}

// Name returns the provider identifier.
func (p *MealieProvider) Name() string {
	return p.Config.Provider
}

// mealieSearchItem is one entry of the paginated recipe list response.
// Tolerant decoding: id/slug/name may be missing in some versions.
type mealieSearchItem struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// mealieRecipeDetail is the full recipe with ingredients.
type mealieRecipeDetail struct {
	ID               string             `json:"id"`
	Slug             string             `json:"slug"`
	Name             string             `json:"name"`
	RecipeIngredient []mealieIngredient `json:"recipeIngredient"`
}

// mealieIngredient tolerates version drift: originalText, note and display
// are free-text fields that may be missing or renamed across versions.
type mealieIngredient struct {
	OriginalText string `json:"originalText"`
	Note         string `json:"note"`
	Display      string `json:"display"`
}

// FetchRecipes searches Mealie per keyword and fetches details once per unique recipe.
func (p *MealieProvider) FetchRecipes(keywords []string) ([]Recipe, bool, error) {
	ctx, cancel := recipeFetchContext(p.Config)
	defer cancel()

	// Search phase: collect unique recipes across all keywords so overlapping
	// keywords do not trigger duplicate detail requests. The dedupe key is the
	// immutable ID, falling back to the slug only when a version omits it —
	// keying on the slug instead would drop every slugless item before the
	// detail phase and leave the ID fallbacks in fetchDetails unreachable.
	uniqueItems, okSearches := searchUnique(ctx, p.Logger, keywords, p.searchRecipes,
		func(item mealieSearchItem) (string, bool) {
			if item.ID != "" {
				return "id:" + item.ID, true
			}
			if item.Slug != "" {
				return "slug:" + item.Slug, true
			}
			return "", false
		},
		maxRecipesPerRun,
	)

	// Detail phase: bounded concurrent detail requests per unique recipe.
	recipes, failedDetails := fetchDetailsConcurrently(ctx, uniqueItems, detailFetchWorkers, p.fetchDetails,
		func(item mealieSearchItem, err error) {
			p.Logger.Debug().Str("slug", item.Slug).Err(err).Msg("failed to fetch Mealie recipe details")
		})
	outcome := fetchOutcome{
		totalSearches: len(keywords),
		okSearches:    okSearches,
		totalDetails:  len(uniqueItems),
		failedDetails: failedDetails,
	}
	if err := outcome.unavailable(); err != nil {
		return nil, false, err
	}

	return deduplicateRecipes(recipes), outcome.complete(), nil
}

// searchRecipes queries the Mealie recipe list for a keyword.
func (p *MealieProvider) searchRecipes(ctx context.Context, keyword string) ([]mealieSearchItem, error) {
	searchURL := fmt.Sprintf("%s/api/recipes?page=1&perPage=%d&search=%s", strings.TrimSuffix(p.Config.URL, "/"), maxRecipesPerKeyword, url.QueryEscape(keyword))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)
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
		Items []mealieSearchItem `json:"items"`
	}
	if err := decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// detailIdentifier returns the path segment identifying a recipe on the detail
// endpoint. Mealie's GET /api/recipes/{x} accepts either a UUID or a slug and
// resolves a UUID first, so the ID leads here for the same reason it leads as
// the dedupe key: it is immutable, while a slug is regenerated whenever the
// recipe is renamed. The slug is the fallback for versions that omit the ID.
// Both cannot be empty, because searchUnique's key function rejects such items
// before the detail phase.
func (p *MealieProvider) detailIdentifier(item mealieSearchItem) string {
	if item.ID != "" {
		return item.ID
	}
	return item.Slug
}

// fetchDetails retrieves the full recipe including ingredients.
func (p *MealieProvider) fetchDetails(ctx context.Context, item mealieSearchItem) (Recipe, error) {
	detailURL := fmt.Sprintf("%s/api/recipes/%s", strings.TrimSuffix(p.Config.URL, "/"), url.PathEscape(p.detailIdentifier(item)))
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return Recipe{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)
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

	var detail mealieRecipeDetail
	if err := decodeJSONResponse(resp, &detail); err != nil {
		return Recipe{}, err
	}
	if detail.Slug == "" {
		detail.Slug = item.Slug
	}
	if detail.Name == "" {
		detail.Name = item.Name
	}
	// Some versions omit the id on the detail payload; fall back to the list
	// entry and finally the slug so deduplication cannot collapse recipes.
	recipeID := detail.ID
	if recipeID == "" {
		recipeID = item.ID
	}
	if recipeID == "" {
		recipeID = detail.Slug
	}

	ingredients := make([]string, 0, len(detail.RecipeIngredient))
	for _, ing := range detail.RecipeIngredient {
		name := ing.OriginalText
		if name == "" {
			name = ing.Note
		}
		if name == "" {
			name = ing.Display
		}
		if strings.TrimSpace(name) != "" {
			ingredients = append(ingredients, name)
		}
	}

	p.Logger.Debug().Int("ingredient_count", len(ingredients)).Strs("ingredients", ingredients).Msg("Fetched Mealie recipe details")

	return Recipe{
		ID:          recipeID,
		Title:       detail.Name,
		ImageURL:    "",
		SourceURL:   strings.TrimSuffix(p.Config.URL, "/") + "/recipe/" + detail.Slug,
		Ingredients: ingredients,
	}, nil
}
