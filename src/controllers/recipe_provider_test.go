package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"
	"github.com/rs/zerolog"
)

func TestNewRecipeProvider_Dispatch(t *testing.T) {
	logger := zerolog.Nop()
	client := &http.Client{}

	tests := []struct {
		name     string
		provider string
		wantType any
		wantName string
	}{
		{
			name:     "TheMealDB",
			provider: util.RecipeProviderThemealDB,
			wantType: &TheMealDBProvider{},
			wantName: util.RecipeProviderThemealDB,
		},
		{
			name:     "Mealie",
			provider: util.RecipeProviderMealie,
			wantType: &MealieProvider{},
			wantName: util.RecipeProviderMealie,
		},
		{
			name:     "Tandoor",
			provider: util.RecipeProviderTandoor,
			wantType: &TandoorProvider{},
			wantName: util.RecipeProviderTandoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := configuration.RecipeAPIConfiguration{Provider: tt.provider}
			provider, err := NewRecipeProvider(config, &logger, client)
			if err != nil {
				t.Fatalf("NewRecipeProvider() error = %v", err)
			}
			if provider == nil {
				t.Fatal("NewRecipeProvider() returned nil provider")
			}
			wantType := tt.wantType
			switch wantType.(type) {
			case *TheMealDBProvider:
				if _, ok := provider.(*TheMealDBProvider); !ok {
					t.Errorf("provider type = %T, want *TheMealDBProvider", provider)
				}
			case *MealieProvider:
				if _, ok := provider.(*MealieProvider); !ok {
					t.Errorf("provider type = %T, want *MealieProvider", provider)
				}
			case *TandoorProvider:
				if _, ok := provider.(*TandoorProvider); !ok {
					t.Errorf("provider type = %T, want *TandoorProvider", provider)
				}
			}
			if provider.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", provider.Name(), tt.wantName)
			}
		})
	}
}

func TestNewRecipeProvider_UnknownProvider(t *testing.T) {
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{Provider: "spoonacular"}

	provider, err := NewRecipeProvider(config, &logger, nil)
	if provider != nil {
		t.Errorf("NewRecipeProvider() provider = %v, want nil", provider)
	}
	if !errors.Is(err, proviantErrors.ErrRecipeInvalidProvider) {
		t.Errorf("NewRecipeProvider() error = %v, want ErrRecipeInvalidProvider", err)
	}
}

func TestNewRecipeProvider_NilClientBuildsDefaultClient(t *testing.T) {
	logger := zerolog.Nop()
	config := configuration.RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB, Timeout: 5}

	provider, err := NewRecipeProvider(config, &logger, nil)
	if err != nil {
		t.Fatalf("NewRecipeProvider() error = %v", err)
	}
	meal, ok := provider.(*TheMealDBProvider)
	if !ok {
		t.Fatalf("provider type = %T, want *TheMealDBProvider", provider)
	}
	if meal.HTTPClient == nil {
		t.Error("provider HTTPClient not set when nil client passed")
	}
}

func TestFetchOutcome(t *testing.T) {
	tests := []struct {
		name                  string
		totalSearches         int
		okSearches            int
		resolvedWithoutDetail int
		totalDetails          int
		failedDetails         int
		wantErr               bool
		wantComplete          bool
	}{
		{
			name:          "all searches succeeded with no detail phase",
			totalSearches: 3,
			okSearches:    3,
			wantErr:       false,
			wantComplete:  true,
		},
		{
			name:          "no keywords attempted is not an outage",
			totalSearches: 0,
			okSearches:    0,
			wantErr:       false,
			wantComplete:  true,
		},
		{
			name:          "every search failed is an outage",
			totalSearches: 3,
			okSearches:    0,
			wantErr:       true,
			wantComplete:  false,
		},
		{
			// A partial run stays usable — failing the whole request would
			// make a slow provider a hard 500 and amplify every retry — but it
			// is incomplete, so the caller caches it only briefly.
			name:          "one search of three failed is usable but incomplete",
			totalSearches: 3,
			okSearches:    2,
			wantErr:       false,
			wantComplete:  false,
		},
		{
			name:          "every detail resolution failed is an outage",
			totalSearches: 2,
			okSearches:    2,
			totalDetails:  4,
			failedDetails: 4,
			wantErr:       true,
			wantComplete:  false,
		},
		{
			name:          "some detail resolutions succeeded is usable",
			totalSearches: 2,
			okSearches:    2,
			totalDetails:  4,
			failedDetails: 1,
			wantErr:       false,
			wantComplete:  false,
		},
		{
			name:          "all detail resolutions succeeded is complete",
			totalSearches: 2,
			okSearches:    2,
			totalDetails:  4,
			failedDetails: 0,
			wantErr:       false,
			wantComplete:  true,
		},
		{
			// Tandoor's list payload resolves some recipes without a detail
			// request. Those are usable results, so a total detail-phase
			// failure must not discard them and 500 the request.
			name:                  "detail phase fully failed but the list already resolved recipes is usable",
			totalSearches:         2,
			okSearches:            2,
			resolvedWithoutDetail: 2,
			totalDetails:          4,
			failedDetails:         4,
			wantErr:               false,
			wantComplete:          false,
		},
		{
			// Pins the ordering inside unavailable(): a search-phase outage is
			// decided first, before already-resolved recipes are considered.
			name:                  "search outage wins over already-resolved recipes",
			totalSearches:         3,
			okSearches:            0,
			resolvedWithoutDetail: 1,
			wantErr:               true,
			wantComplete:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := fetchOutcome{
				totalSearches:         tt.totalSearches,
				okSearches:            tt.okSearches,
				resolvedWithoutDetail: tt.resolvedWithoutDetail,
				totalDetails:          tt.totalDetails,
				failedDetails:         tt.failedDetails,
			}

			err := outcome.unavailable()
			if tt.wantErr {
				if !errors.Is(err, proviantErrors.ErrRecipeAPIUnavailable) {
					t.Errorf("unavailable() error = %v, want ErrRecipeAPIUnavailable", err)
				}
			} else if err != nil {
				t.Errorf("unavailable() error = %v, want nil", err)
			}

			complete := outcome.complete()
			if complete != tt.wantComplete {
				t.Errorf("complete() = %v, want %v", complete, tt.wantComplete)
			}
		})
	}
}

// uniformHits builds one keyword result set per entry in sizes, each holding
// that many items with globally unique IDs, so nothing deduplicates across
// keywords unless a test deliberately repeats IDs.
func uniformHits(sizes ...int) [][]string {
	hits := make([][]string, len(sizes))
	id := 0
	for i, size := range sizes {
		hits[i] = make([]string, 0, size)
		for range size {
			hits[i] = append(hits[i], fmt.Sprintf("item-%d", id))
			id++
		}
	}
	return hits
}

// keywordOfItem maps every item of the given keyword result sets back to its
// keyword, so a test can count the round-robin spread without parsing IDs.
func keywordOfItem(hits [][]string) map[string]string {
	owner := make(map[string]string)
	for i := range hits {
		for _, item := range hits[i] {
			owner[item] = fmt.Sprintf("keyword-%d", i)
		}
	}
	return owner
}

// TestSearchUniqueRespectsCap pins maxRecipesPerRun: the pooled recipes may
// never exceed the cap, and the round-robin merge must still spread them across
// keywords. Before the per-keyword cap check one pass could append one item per
// keyword, so 3 keywords × 7 hits produced 21 pooled items against a cap of 20
// and every surplus item became an extra outbound detail request.
func TestSearchUniqueRespectsCap(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name      string
		hits      [][]string
		maxItems  int
		wantItems int
	}{
		{
			name:      "three keywords of seven hits overshoot without the per-keyword cap",
			hits:      uniformHits(7, 7, 7),
			maxItems:  20,
			wantItems: 20,
		},
		{
			// Passes before the fix: 1 + 4*6 = 25 pooled items.
			name:      "lopsided hit counts overshoot without the per-keyword cap",
			hits:      uniformHits(1, 4, 4, 4, 4, 4, 4),
			maxItems:  20,
			wantItems: 20,
		},
		{
			name:      "ten keywords of two hits fill the cap exactly",
			hits:      uniformHits(2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
			maxItems:  20,
			wantItems: 20,
		},
		{
			name:      "a short pool is not padded",
			hits:      uniformHits(6),
			maxItems:  20,
			wantItems: 6,
		},
		{
			name:      "zero cap collects nothing",
			hits:      uniformHits(7, 7, 7),
			maxItems:  0,
			wantItems: 0,
		},
		{
			name:      "items identical across keywords deduplicate instead of overfilling",
			hits:      [][]string{{"a", "b", "c", "d", "e", "f", "g"}, {"a", "b", "c", "d", "e", "f", "g"}, {"a", "b", "c", "d", "e", "f", "g"}},
			maxItems:  20,
			wantItems: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := make([]string, len(tt.hits))
			byKeyword := make(map[string][]string, len(tt.hits))
			for i := range tt.hits {
				keywords[i] = fmt.Sprintf("keyword-%d", i)
				byKeyword[keywords[i]] = tt.hits[i]
			}
			search := func(_ context.Context, keyword string) ([]string, error) {
				return byKeyword[keyword], nil
			}

			items, okSearches := searchUnique(context.Background(), &logger, keywords, search,
				func(item string) (string, bool) { return item, item != "" }, tt.maxItems)

			if len(items) > tt.maxItems {
				t.Errorf("searchUnique() collected %d items, want at most %d", len(items), tt.maxItems)
			}
			if len(items) != tt.wantItems {
				t.Errorf("searchUnique() collected %d items, want %d", len(items), tt.wantItems)
			}
			if okSearches != len(keywords) {
				t.Errorf("searchUnique() okSearches = %d, want %d", okSearches, len(keywords))
			}
		})
	}

	t.Run("round-robin spread survives the cap", func(t *testing.T) {
		hits := uniformHits(7, 7, 7)
		keywords := []string{"keyword-0", "keyword-1", "keyword-2"}
		byKeyword := make(map[string][]string, len(keywords))
		for i := range hits {
			byKeyword[keywords[i]] = hits[i]
		}
		search := func(_ context.Context, keyword string) ([]string, error) {
			return byKeyword[keyword], nil
		}

		items, _ := searchUnique(context.Background(), &logger, keywords, search,
			func(item string) (string, bool) { return item, item != "" }, 20)
		if len(items) != 20 {
			t.Fatalf("searchUnique() collected %d items, want 20", len(items))
		}
		owner := keywordOfItem(hits)
		perKeyword := make(map[string]int, len(keywords))
		for _, item := range items {
			perKeyword[owner[item]]++
		}
		// No keyword may supply the whole budget, and every keyword must still
		// contribute: 20 items over 3 keywords of 7 hits ends at 7/7/6.
		for _, keyword := range keywords {
			count := perKeyword[keyword]
			if count > 7 {
				t.Errorf("keyword %q supplied %d items, want at most 7 (round-robin)", keyword, count)
			}
			if count < 6 {
				t.Errorf("keyword %q supplied %d items, want at least 6 (round-robin)", keyword, count)
			}
		}
	})
}

func TestSearchUniqueCountsFailedSearches(t *testing.T) {
	logger := zerolog.Nop()
	keywords := []string{"ok", "broken"}
	search := func(_ context.Context, keyword string) ([]string, error) {
		if keyword == "broken" {
			return nil, errors.New("boom")
		}
		return []string{"a", "b"}, nil
	}

	items, okSearches := searchUnique(context.Background(), &logger, keywords, search,
		func(item string) (string, bool) { return item, item != "" }, maxRecipesPerRun)

	if len(items) != 2 {
		t.Errorf("searchUnique() collected %d items, want 2", len(items))
	}
	if okSearches != 1 {
		t.Errorf("searchUnique() okSearches = %d, want 1", okSearches)
	}
}
