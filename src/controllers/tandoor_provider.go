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

// TandoorProvider fetches recipes from a self-hosted Tandoor instance.
type TandoorProvider struct {
	Logger     *zerolog.Logger
	Config     configuration.RecipeAPIConfiguration
	HTTPClient *http.Client
}

// Name returns the provider identifier.
func (p *TandoorProvider) Name() string {
	return p.Config.Provider
}

// tandoorListItem is one entry of the paginated recipe list response.
type tandoorListItem struct {
	ID    int           `json:"id"`
	Slug  string        `json:"slug"`
	Name  string        `json:"name"`
	Image string        `json:"image"`
	Steps []tandoorStep `json:"steps"`
}

// tandoorStep holds ingredients for one step.
type tandoorStep struct {
	Ingredients []tandoorIngredient `json:"ingredients"`
}

// tandoorIngredient references a food within a step.
type tandoorIngredient struct {
	Food tandoorFood `json:"food"`
}

// tandoorFood is the referenced food object.
type tandoorFood struct {
	Name string `json:"name"`
}

// FetchRecipes searches Tandoor per keyword and resolves details once per unique recipe.
func (p *TandoorProvider) FetchRecipes(keywords []string) ([]Recipe, bool, error) {
	ctx, cancel := recipeFetchContext(p.Config)
	defer cancel()

	// Search phase: collect unique recipes across all keywords so overlapping
	// keywords do not trigger duplicate detail requests.
	uniqueItems, okSearches := searchUnique(ctx, p.Logger, keywords, p.searchRecipes,
		func(item tandoorListItem) (int, bool) { return item.ID, item.ID != 0 },
		maxRecipesPerRun,
	)

	// Resolution phase: use list steps when present, otherwise one detail
	// request per unique recipe (the list serializer may omit steps).
	needDetails := make([]tandoorListItem, 0, len(uniqueItems))
	direct := make([]Recipe, 0, len(uniqueItems))
	for i := range uniqueItems {
		item := &uniqueItems[i]
		ingredients := collectTandoorIngredients(item.Steps)
		if len(ingredients) == 0 {
			needDetails = append(needDetails, *item)
			continue
		}
		direct = append(direct, p.toRecipe(item, ingredients))
	}

	detailRecipes, failedDetails := fetchDetailsConcurrently(ctx, needDetails, detailFetchWorkers,
		func(ctx context.Context, item tandoorListItem) (Recipe, error) {
			return p.fetchDetails(ctx, &item)
		},
		func(item tandoorListItem, err error) {
			p.Logger.Debug().Int("id", item.ID).Err(err).Msg("failed to fetch Tandoor recipe details")
		})
	// Recipes resolved from the list payload never entered the detail phase:
	// they are usable results, so the availability decision accounts for them
	// instead of counting them as failures.
	outcome := fetchOutcome{
		totalSearches:         len(keywords),
		okSearches:            okSearches,
		resolvedWithoutDetail: len(direct),
		totalDetails:          len(needDetails),
		failedDetails:         failedDetails,
	}
	if err := outcome.unavailable(); err != nil {
		return nil, false, err
	}

	return deduplicateRecipes(append(direct, detailRecipes...)), outcome.complete(), nil
}

// searchRecipes queries the Tandoor recipe list for a keyword.
func (p *TandoorProvider) searchRecipes(ctx context.Context, keyword string) ([]tandoorListItem, error) {
	searchURL := fmt.Sprintf("%s/api/recipe/?search=%s&page_size=%d", strings.TrimSuffix(p.Config.URL, "/"), url.QueryEscape(keyword), maxRecipesPerKeyword)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+p.Config.APIKey)
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
		Results []tandoorListItem `json:"results"`
	}
	if err := decodeJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// fetchDetails retrieves the full recipe including ingredients.
func (p *TandoorProvider) fetchDetails(ctx context.Context, item *tandoorListItem) (Recipe, error) {
	detailURL := fmt.Sprintf("%s/api/recipe/%d/", strings.TrimSuffix(p.Config.URL, "/"), item.ID)
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return Recipe{}, err
	}
	req.Header.Set("Authorization", "Token "+p.Config.APIKey)
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

	var detail tandoorListItem
	if err := decodeJSONResponse(resp, &detail); err != nil {
		return Recipe{}, err
	}
	if detail.Name == "" {
		detail.Name = item.Name
	}
	if detail.Slug == "" {
		detail.Slug = item.Slug
	}
	// Some versions omit the id on the detail payload; fall back to the list
	// entry so deduplication cannot collapse recipes. toRecipe formats item.ID,
	// so leaving it zero would make every detail-resolved recipe share the key
	// "0" and deduplicateRecipes would keep only one of them. searchUnique
	// already rejects list entries with a zero id, so this always yields one.
	if detail.ID == 0 {
		detail.ID = item.ID
	}

	return p.toRecipe(&detail, collectTandoorIngredients(detail.Steps)), nil
}

// toRecipe builds a normalized Recipe from a Tandoor item.
func (p *TandoorProvider) toRecipe(item *tandoorListItem, ingredients []string) Recipe {
	imageURL := ""
	// Only absolute URLs are safe; relative media paths would need the
	// instance base URL plus media root handling.
	if strings.HasPrefix(item.Image, "http://") || strings.HasPrefix(item.Image, "https://") {
		imageURL = item.Image
	}
	return Recipe{
		ID:          fmt.Sprintf("%d", item.ID),
		Title:       item.Name,
		ImageURL:    imageURL,
		SourceURL:   strings.TrimSuffix(p.Config.URL, "/") + "/recipe/" + item.Slug + "/",
		Ingredients: ingredients,
	}
}

// collectTandoorIngredients extracts deduplicated food names across all steps.
func collectTandoorIngredients(steps []tandoorStep) []string {
	seen := make(map[string]bool)
	ingredients := make([]string, 0)
	for _, step := range steps {
		for _, ing := range step.Ingredients {
			name := strings.TrimSpace(ing.Food.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			ingredients = append(ingredients, name)
		}
	}
	return ingredients
}
