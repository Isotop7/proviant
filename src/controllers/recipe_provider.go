package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"
	"github.com/rs/zerolog"
)

const (
	// maxRecipesPerKeyword caps the page size requested from a provider per
	// keyword search.
	maxRecipesPerKeyword = 20

	// maxRecipesPerRun caps the total unique recipes pooled across all
	// keywords before the detail phase; the round-robin merge in searchUnique
	// spreads this budget across keywords instead of letting the first
	// keyword fill it alone.
	maxRecipesPerRun = 20

	// detailFetchWorkers bounds the concurrent detail requests per fetch run.
	detailFetchWorkers = 4

	// maxProviderResponseBytes caps a single provider response body. The HTTP
	// client timeout bounds request duration, not bytes, so a misconfigured
	// URL, a proxy error page or a compromised self-hosted instance could
	// otherwise stream an arbitrarily large body into memory.
	maxProviderResponseBytes = 5 << 20 // 5 MiB
)

// decodeJSONResponse decodes a provider response body under a hard size cap.
// Exceeding the cap surfaces as a decode error, which the calling provider
// treats like any other failed request.
func decodeJSONResponse(resp *http.Response, target any) error {
	return json.NewDecoder(io.LimitReader(resp.Body, maxProviderResponseBytes)).Decode(target)
}

// RecipeProvider fetches normalized recipes from an external recipe service.
type RecipeProvider interface {
	// Name returns the provider identifier (e.g. util.RecipeProviderThemealDB).
	Name() string
	// FetchRecipes retrieves recipes matching the given keywords. The boolean
	// reports whether the run was complete: every keyword search and every
	// detail lookup succeeded. An incomplete run is still worth returning
	// (partial degradation beats a hard failure), but the caller must not
	// cache it as if it were the provider's full answer.
	FetchRecipes(keywords []string) (recipes []Recipe, complete bool, err error)
}

// recipeFetchContext returns a context bounding one whole provider fetch run.
// The overall budget is twice the per-request timeout, clamped to a 20-30
// second band, so multi-keyword runs stay possible while a slow or
// unreachable provider cannot pin the suggestions handler for minutes.
func recipeFetchContext(config configuration.RecipeAPIConfiguration) (context.Context, context.CancelFunc) {
	overall := time.Duration(config.Timeout) * time.Second * 2
	if overall < 20*time.Second {
		overall = 20 * time.Second
	}
	if overall > 30*time.Second {
		overall = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), overall)
}

// NewRecipeProvider builds the recipe provider selected by the configuration.
func NewRecipeProvider(config configuration.RecipeAPIConfiguration, logger *zerolog.Logger, client *http.Client) (RecipeProvider, error) {
	if client == nil {
		client = &http.Client{
			Timeout: time.Second * time.Duration(config.Timeout),
		}
	}
	switch config.Provider {
	case util.RecipeProviderThemealDB:
		return &TheMealDBProvider{
			Logger:     logger,
			Config:     config,
			HTTPClient: client,
		}, nil
	case util.RecipeProviderMealie:
		return &MealieProvider{
			Logger:     logger,
			Config:     config,
			HTTPClient: client,
		}, nil
	case util.RecipeProviderTandoor:
		return &TandoorProvider{
			Logger:     logger,
			Config:     config,
			HTTPClient: client,
		}, nil
	default:
		return nil, errors.ErrRecipeInvalidProvider
	}
}

// searchUnique runs the per-keyword searches concurrently with a bounded
// worker pool, deduplicates items by key and caps the collected pool. Results
// are merged round-robin across keywords so every keyword contributes to the
// pool instead of the first keyword filling the whole budget. It also counts
// how many keyword searches succeeded so a total provider outage can be
// distinguished from "no matches".
func searchUnique[T any, K comparable](
	ctx context.Context, logger *zerolog.Logger, keywords []string,
	search func(context.Context, string) ([]T, error), keyFn func(T) (K, bool), maxItems int,
) (items []T, okSearches int) {
	type keywordResult struct {
		hits []T
		err  error
	}
	results := make([]keywordResult, len(keywords))
	sem := make(chan struct{}, detailFetchWorkers)
	var wg sync.WaitGroup
	for i, kw := range keywords {
		wg.Add(1)
		go func(i int, kw string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			hits, err := search(ctx, kw)
			results[i] = keywordResult{hits: hits, err: err}
		}(i, kw)
	}
	wg.Wait()

	seen := make(map[K]bool)
	items = make([]T, 0)
	cursors := make([]int, len(results))
	for len(items) < maxItems {
		progressed := false
		for i := range results {
			// Re-checked per keyword, not only per pass: one pass appends up
			// to one item per keyword, so checking only at the top of the pass
			// let the pool overshoot maxItems by up to len(results)-1, turning
			// every surplus item into an extra outbound detail request and
			// breaking the fan-out bound recipesBurst relies on.
			if len(items) >= maxItems {
				break
			}
			if results[i].err != nil {
				continue
			}
			hits := results[i].hits
			for cursors[i] < len(hits) {
				item := hits[cursors[i]]
				cursors[i]++
				key, valid := keyFn(item)
				if !valid || seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, item)
				progressed = true
				break
			}
		}
		if !progressed {
			break
		}
	}

	for i, kw := range keywords {
		if results[i].err != nil {
			logger.Debug().Str("keyword", kw).Err(results[i].err).Msg("failed to search provider")
			continue
		}
		okSearches++
	}
	return items, okSearches
}

// fetchOutcome summarizes one provider fetch run so the availability and
// completeness decisions live in a single place instead of being re-derived at
// each call site from positional ints that can be transposed silently.
type fetchOutcome struct {
	totalSearches int
	okSearches    int
	// resolvedWithoutDetail counts recipes completed from the search phase
	// alone: Tandoor's list payload sometimes already carries ingredients, so
	// they never enter the detail phase. They are usable results, so a
	// detail-phase outage verdict must never discard them.
	resolvedWithoutDetail int
	totalDetails          int
	failedDetails         int
}

// unavailable reports the provider as unavailable only when nothing usable came
// back: every keyword search failed, or every detail resolution failed and the
// search phase had already produced no recipe on its own.
//
// A single failed keyword search must NOT fail the whole run. Keywords are
// fanned out through a small worker pool under one shared deadline
// (recipeFetchContext), so a provider averaging more than a few hundred
// milliseconds per search pushes the last wave past the deadline while the rest
// succeeded. Treating that as a total outage turned partial degradation into a
// 500, and because the error path caches nothing, every retry repeated the full
// fan-out against the already-struggling upstream. Completeness is reported
// separately through complete, which is what decides the cache TTL.
func (o fetchOutcome) unavailable() error {
	if o.totalSearches > 0 && o.okSearches == 0 {
		return errors.ErrRecipeAPIUnavailable
	}
	if o.resolvedWithoutDetail > 0 {
		return nil
	}
	if o.totalDetails > 0 && o.failedDetails == o.totalDetails {
		return errors.ErrRecipeAPIUnavailable
	}
	return nil
}

// complete reports whether a provider run covered everything it was asked for.
// An incomplete run must be cached only briefly, otherwise a transient provider
// hiccup serves a sparse suggestion set for the full TTL.
func (o fetchOutcome) complete() bool {
	return o.okSearches == o.totalSearches && o.failedDetails == 0
}

// fetchDetailsConcurrently resolves recipes for the given items with a bounded
// worker pool, preserving input order in the result slice. It returns the
// resolved recipes and how many detail requests failed.
func fetchDetailsConcurrently[T any](
	ctx context.Context, items []T, workers int,
	fetch func(context.Context, T) (Recipe, error), onErr func(item T, err error),
) (recipes []Recipe, failedDetails int) {
	recipes = make([]Recipe, len(items))
	failed := make([]error, len(items))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			recipes[i], failed[i] = fetch(ctx, items[i])
		}(i)
	}
	wg.Wait()

	uniqueRecipes := make([]Recipe, 0, len(items))
	for i := range items {
		if failed[i] != nil {
			failedDetails++
			if onErr != nil {
				onErr(items[i], failed[i])
			}
			continue
		}
		uniqueRecipes = append(uniqueRecipes, recipes[i])
	}
	return uniqueRecipes, failedDetails
}
