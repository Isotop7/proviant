package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	Logger *zerolog.Logger
	Config configuration.RecipeAPIConfiguration
	// RecipeRepo is the interface rather than the concrete type so the cache
	// read/write decisions — which decide whether a degraded provider run is
	// allowed to replace a good cached entry — can be tested without a database.
	RecipeRepo recipeRepo.RecipeRepositoryInterface
	Provider   RecipeProvider
	disabled   bool
}

// Recipe represents a normalized recipe from a recipe provider.
type Recipe struct {
	ID          string
	Title       string
	ImageURL    string
	SourceURL   string
	Ingredients []string
}

// RecipeSuggestion is a recipe matched to household products.
type RecipeSuggestion struct {
	ID                string                `json:"id"`
	Title             string                `json:"title"`
	ImageURL          string                `json:"imageUrl"`
	SourceURL         string                `json:"sourceUrl"`
	Ingredients       []api.IngredientMatch `json:"ingredients"`
	MatchedProducts   []string              `json:"matchedProducts"` // kept for backwards compatibility
	MatchedProductIDs []uint                `json:"matchedProductIds"`
	MissingCount      int                   `json:"missingCount"`
	TotalIngredients  int                   `json:"totalIngredients"`
	MatchPercent      float64               `json:"matchPercent"`
	ExpiryPoints      int                   `json:"-"` // internal ranking score, not serialized
}

// ToAPIResponse converts a RecipeSuggestion into its API response model.
// The field mapping lives here so struct extensions cannot silently drop
// fields from API responses.
func (s *RecipeSuggestion) ToAPIResponse() api.RecipeSuggestionResponse {
	return api.RecipeSuggestionResponse{
		ID:                s.ID,
		Title:             s.Title,
		ImageURL:          s.ImageURL,
		SourceURL:         s.SourceURL,
		Ingredients:       s.Ingredients,
		MatchedProducts:   s.MatchedProducts,
		MatchedProductIDs: s.MatchedProductIDs,
		MissingCount:      s.MissingCount,
		TotalIngredients:  s.TotalIngredients,
		MatchPercent:      s.MatchPercent,
	}
}

// productIndex holds one product's pre-normalized searchable text. The word
// tokens are split once here rather than per comparison: matchRank runs
// |recipes| × |ingredients| × |products| times, so re-splitting the same
// strings inside the matcher made the scan allocation-bound.
type productIndex struct {
	name       string
	nameWords  []string
	categories []string
	catWords   [][]string
	productID  uint
}

const (
	MsgFailedCloseResponseBody = "failed to close response body"
	MsgApiReturnWrapper        = "API returned %d"

	// maxRecipeKeywords caps the number of outbound provider search keywords
	// built from expiring products, so a large expiring set cannot multiply
	// the outbound request count unboundedly.
	maxRecipeKeywords = 10

	// negativeCacheTTL bounds how long an empty suggestion result stays
	// cached. A short TTL suppresses the repeated provider fetch runs an
	// uncached empty key would cause, while letting a household pick up new
	// suggestions soon after its products change.
	negativeCacheTTL = 15 * time.Minute
)

// sanitizeRecipeURL returns the URL unchanged when it is an absolute
// http(s) URL, and an empty string otherwise. Provider responses are
// remote-controlled content, so non-web schemes must never reach the frontend.
func sanitizeRecipeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return raw
}

// NewRecipeController creates a new controller. disabled short-circuits the
// whole feature: startup sets it when the configured provider can never
// authenticate, and the controller then serves no suggestions instead of
// issuing outbound requests that are guaranteed to fail.
func NewRecipeController(config configuration.RecipeAPIConfiguration, logger *zerolog.Logger, db *gorm.DB, disabled bool) *RecipeController {
	repo := recipeRepo.NewRecipeRepository(db)
	client := &http.Client{
		Timeout: time.Second * time.Duration(config.Timeout),
	}
	ctrl := &RecipeController{
		Logger:     logger,
		Config:     config,
		RecipeRepo: repo,
		disabled:   disabled,
	}
	if disabled {
		return ctrl
	}
	provider, err := NewRecipeProvider(config, logger, client)
	if err != nil {
		logger.Error().Err(err).Msgf("failed to initialize recipe provider %q", config.Provider)
		return ctrl
	}
	ctrl.Provider = provider
	return ctrl
}

// ProductSource supplies the household product data GetSuggestions needs.
// Both members are lazy so a cache hit never loads every product row.
type ProductSource struct {
	// All returns every matchable product of the household.
	All func() ([]dbModel.Product, error)
	// Fingerprint returns a cheap signature of that same set. It goes into the
	// cache key because cached suggestions carry MatchedProductIDs, which the
	// client turns into an irreversible "cook this" action: serving an entry
	// whose product set has since been renamed, recategorised, extended or
	// deleted from would consume products the match no longer justifies.
	Fingerprint func() (string, error)
}

// GetSuggestions returns up to limit recipe suggestions based on expiring
// products. When refresh is true, the suggestion cache is bypassed.
func (rc *RecipeController) GetSuggestions(
	expiringProducts []dbModel.Product, products ProductSource, limit int, refresh bool,
) ([]RecipeSuggestion, error) {
	if len(expiringProducts) == 0 {
		return []RecipeSuggestion{}, nil
	}
	// Startup disables the feature when the configured provider can never
	// authenticate (self-hosted provider without an API key). Returning an
	// empty list here keeps the endpoint a cheap 200 instead of a 500 that
	// repeats a doomed outbound fan-out on every request.
	if rc.disabled {
		rc.Logger.Debug().Msg("recipe suggestions are disabled by configuration")
		return []RecipeSuggestion{}, nil
	}

	expiringIDs := make([]uint, 0, len(expiringProducts))
	for i := range expiringProducts {
		expiringIDs = append(expiringIDs, expiringProducts[i].ID)
	}
	sort.Slice(expiringIDs, func(i, j int) bool { return expiringIDs[i] < expiringIDs[j] })

	providerName := rc.Config.Provider
	if rc.Provider != nil {
		providerName = rc.Provider.Name()
	}

	// A fingerprint failure must fail the cache lookup open rather than closed:
	// an unverifiable key could match an entry whose product IDs no longer
	// describe this household. Skipping the read sends the request down the
	// fresh path, which loads the products itself and reports its own error.
	//
	// Disabled caching skips the query outright. Neither the read nor the write
	// consults the key, so computing one would scan and hash the whole household
	// product set on every request for a value nothing reads.
	var fingerprint, cacheKey string
	cacheable := rc.Config.CacheEnabled
	if cacheable {
		var fingerprintErr error
		fingerprint, fingerprintErr = products.Fingerprint()
		cacheable = fingerprintErr == nil
		if cacheable {
			fingerprint = strings.TrimSpace(fingerprint)
			cacheKey = dbModel.GenerateCacheKey(providerName, rc.Config.URL, fingerprint, expiringIDs)
		}
	}

	if cacheable && !refresh {
		if suggestions, hit := rc.tryGetCache(cacheKey); hit {
			return rc.limitSuggestions(suggestions, limit), nil
		}
	}

	allProducts, err := products.All()
	if err != nil {
		rc.Logger.Error().Err(err).Msg("failed to get household products")
		return nil, err
	}

	keywords := rc.collectKeywords(expiringProducts)

	rawRecipes, complete, err := rc.fetchFromAPI(keywords)
	if err != nil {
		rc.Logger.Error().Err(err).Msg("failed to fetch recipes from API")
		return nil, errors.ErrRecipeAPIUnavailable
	}

	matched := rc.matchRecipesToProducts(rawRecipes, allProducts, expiringProducts)
	if len(matched) == 0 {
		// Store the empty result under a short negative TTL instead of
		// deleting the entry. An uncached key would make every later request,
		// including non-refresh ones, miss and repeat the full provider fetch
		// run until a recipe happens to match again. The upsert also replaces
		// a previous, now contradicted entry.
		if cacheable {
			rc.storeUnlessDowngrade(cacheKey, []RecipeSuggestion{}, negativeCacheTTL)
		}
		return []RecipeSuggestion{}, nil
	}

	for i := range matched {
		if matched[i].Ingredients == nil {
			matched[i].Ingredients = []api.IngredientMatch{}
		}
		if matched[i].MatchedProductIDs == nil {
			matched[i].MatchedProductIDs = []uint{}
		}
	}

	// A run that lost some keyword searches or detail lookups is a sparse view
	// of the provider, not its answer. Serving it is better than failing the
	// request, but caching it for the full TTL would keep a transient provider
	// hiccup visible for hours, so it expires with the negative TTL instead.
	ttl := time.Duration(rc.Config.CacheTTL) * time.Hour
	if !complete {
		rc.Logger.Debug().Msg("storing incomplete recipe suggestions under a short TTL")
		ttl = negativeCacheTTL
	}
	// An unverifiable product-set fingerprint produced a key that must never be
	// read back, so writing it would only litter the table with entries that no
	// lookup can reach.
	if cacheable {
		if complete {
			rc.storeCacheWithTTL(cacheKey, matched, ttl)
		} else {
			rc.storeUnlessDowngrade(cacheKey, matched, ttl)
		}
	}

	return rc.limitSuggestions(matched, limit), nil
}

// storeUnlessDowngrade writes suggestions only when they would not replace a
// better entry already cached under the same key.
//
// CreateCache is an unconditional upsert on query_hash, so a degraded run — one
// that matched nothing, or that lost part of the provider fan-out — would
// otherwise overwrite a complete result and keep the household on the degraded
// answer for the whole TTL of the entry it destroyed. A degraded run is
// inherently a transient observation, whereas an entry already holding real
// suggestions for this exact provider, URL, product-set fingerprint and
// expiring set is not: nothing about the inputs changed for the two to disagree.
func (rc *RecipeController) storeUnlessDowngrade(cacheKey string, suggestions []RecipeSuggestion, ttl time.Duration) {
	populated, err := rc.RecipeRepo.HasPopulatedCache(cacheKey)
	if err != nil {
		// An inconclusive check must not block the write: a later cleanup
		// removes the entry either way, and storing costs nothing but a row.
		rc.Logger.Warn().Err(err).Msg("failed to check for an existing recipe cache entry, storing anyway")
		rc.storeCacheWithTTL(cacheKey, suggestions, ttl)
		return
	}
	if populated {
		rc.Logger.Debug().Msg("keeping the existing recipe cache entry instead of overwriting it with a degraded result")
		return
	}
	rc.storeCacheWithTTL(cacheKey, suggestions, ttl)
}

// tryGetCache returns cached suggestions and true on a valid, non-stale hit.
func (rc *RecipeController) tryGetCache(cacheKey string) ([]RecipeSuggestion, bool) {
	if !rc.Config.CacheEnabled {
		return nil, false
	}
	cached, err := rc.RecipeRepo.GetCacheByQueryHash(cacheKey)
	if err != nil || cached.IsExpired() {
		return nil, false
	}
	var suggestions []RecipeSuggestion
	if err := json.Unmarshal(cached.ResponseJSON, &suggestions); err != nil {
		return nil, false
	}
	// Staleness is judged by the model, so the read and the guarded write
	// cannot drift apart on what counts as servable.
	stale, staleErr := cached.HasStaleSuggestions()
	if staleErr != nil {
		return nil, false
	}
	if stale {
		rc.Logger.Debug().Msg("stale cache detected, refreshing")
		return nil, false
	}
	if err := rc.RecipeRepo.UpdateCacheHit(cacheKey); err != nil {
		rc.Logger.Warn().Err(err).Msg("failed to update cache hit count")
	}
	rc.Logger.Debug().Msg("recipe suggestions cache hit")
	return suggestions, true
}

// storeCacheWithTTL marshals suggestions and writes a cache entry with an
// explicit TTL; logs on failure. Callers choose the TTL: the configured value
// for a complete provider run, the short negativeCacheTTL for an incomplete
// one or for an empty result.
func (rc *RecipeController) storeCacheWithTTL(cacheKey string, suggestions []RecipeSuggestion, ttl time.Duration) {
	data, err := json.Marshal(suggestions)
	if err != nil {
		rc.Logger.Warn().Err(err).Msg("failed to marshal recipe suggestions for caching")
		return
	}
	if !rc.Config.CacheEnabled {
		return
	}
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

// collectKeywords builds a capped, ordered set of category tokens and product
// name tokens from expiring products. Product order (expiry-soonest first) is
// preserved so capping keeps the most urgent products' keywords.
func (rc *RecipeController) collectKeywords(products []dbModel.Product) []string {
	keywordSet := make(map[string]struct{})
	keywords := make([]string, 0, maxRecipeKeywords)

	for i := range products {
		if len(keywords) >= maxRecipeKeywords {
			return keywords
		}
		p := &products[i]
		name := normalizeString(p.ProductName)
		if name != "" {
			if _, exists := keywordSet[name]; !exists {
				keywordSet[name] = struct{}{}
				keywords = append(keywords, name)
			}
		}
		for _, cat := range strings.Split(p.Categories, ",") {
			if len(keywords) >= maxRecipeKeywords {
				return keywords
			}
			cat = normalizeString(cat)
			if cat != "" && cat != "food" && cat != "de:lebensmittel" {
				if _, exists := keywordSet[cat]; !exists {
					keywordSet[cat] = struct{}{}
					keywords = append(keywords, cat)
				}
			}
		}
	}
	return keywords
}

// fetchFromAPI retrieves recipes from the configured provider using keywords.
// The boolean reports whether the run covered every keyword and detail lookup.
func (rc *RecipeController) fetchFromAPI(keywords []string) ([]Recipe, bool, error) {
	if rc.Provider == nil {
		return nil, false, errors.ErrRecipeInvalidProvider
	}
	return rc.Provider.FetchRecipes(keywords)
}

// deduplicateRecipes removes duplicate Recipe entries by ID, preserving first occurrence.
func deduplicateRecipes(recipes []Recipe) []Recipe {
	seen := make(map[string]bool, len(recipes))
	unique := make([]Recipe, 0, len(recipes))
	for _, r := range recipes {
		if !seen[r.ID] {
			seen[r.ID] = true
			unique = append(unique, r)
		}
	}
	return unique
}

// matchRecipesToProducts matches recipes against the full product list and returns suggestions.
// expiringProducts seeds the expiry-first ranking score.
func (rc *RecipeController) matchRecipesToProducts(recipes []Recipe, allProducts []dbModel.Product, expiringProducts []dbModel.Product) []RecipeSuggestion {
	suggestions := make([]RecipeSuggestion, 0)
	normalized := buildNormalizedIndex(allProducts)
	expiryPointsByProductID := buildExpiryPoints(expiringProducts)

	for _, recipe := range recipes {
		rc.Logger.Debug().Str("recipe", recipe.Title).Int("ingredients_total", len(recipe.Ingredients)).Msg("Matching recipe")
		ingredientMatches := make([]api.IngredientMatch, 0, len(recipe.Ingredients))
		var matched []string
		// rankedIDs holds every product an ingredient matched at any strength and
		// drives the expiry ranking, so a recipe whose matches are whole-word or
		// category hits still scores. cookIDs is the exact-name subset only,
		// because only those identify the product confidently enough for the
		// irreversible cook action.
		var rankedIDs []uint
		var cookIDs []uint

		for _, ingredient := range recipe.Ingredients {
			match, rank := ingredientMatched(normalizeString(ingredient), normalized)
			ingredientMatches = append(ingredientMatches, api.IngredientMatch{
				Name:    ingredient,
				Matched: match != nil,
			})
			if match == nil {
				continue
			}
			matched = append(matched, ingredient)
			if match.productID == 0 {
				continue
			}
			rankedIDs = append(rankedIDs, match.productID)
			// Only an exact name match identifies the product confidently
			// enough for the destructive cook action. Whole-word and
			// substring hits select a product the recipe does not name —
			// "milk" inside "Coconut Milk" — and consuming it would be
			// irreversible and wrong. Loose hits stay display-only.
			if rank == rankExactName {
				cookIDs = append(cookIDs, match.productID)
			}
		}

		if len(matched) > 0 {
			missing := len(recipe.Ingredients) - len(matched)
			matchPct := float64(len(matched)) / float64(len(recipe.Ingredients)) * 100.0
			points, _ := expiryPointsFor(rankedIDs, expiryPointsByProductID)
			_, idPoints := expiryPointsFor(cookIDs, expiryPointsByProductID)
			suggestions = append(suggestions, RecipeSuggestion{
				ID:                recipe.ID,
				Title:             recipe.Title,
				ImageURL:          sanitizeRecipeURL(recipe.ImageURL),
				SourceURL:         sanitizeRecipeURL(recipe.SourceURL),
				Ingredients:       ingredientMatches,
				MatchedProducts:   matched,
				MatchedProductIDs: idPoints,
				MissingCount:      missing,
				TotalIngredients:  len(recipe.Ingredients),
				MatchPercent:      matchPct,
				ExpiryPoints:      points,
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].ExpiryPoints != suggestions[j].ExpiryPoints {
			return suggestions[i].ExpiryPoints > suggestions[j].ExpiryPoints
		}
		if suggestions[i].MatchPercent == suggestions[j].MatchPercent {
			return suggestions[i].MissingCount < suggestions[j].MissingCount
		}
		return suggestions[i].MatchPercent > suggestions[j].MatchPercent
	})

	return suggestions
}

// buildExpiryPoints maps product IDs of expiring products to their expiry points.
// Points = ExpiryRankingHorizonDays - ExpiryRankingDaysLeft(EffectiveExpireAt()),
// so the most urgent product scores highest and anything past the horizon scores 0.
func buildExpiryPoints(expiringProducts []dbModel.Product) map[uint]int {
	now := time.Now()
	points := make(map[uint]int, len(expiringProducts))
	for i := range expiringProducts {
		p := &expiringProducts[i]
		if p.ID == 0 {
			continue
		}
		expireAt := p.EffectiveExpireAt()
		if expireAt.IsZero() {
			continue
		}
		points[p.ID] = dbModel.ExpiryRankingHorizonDays - dbModel.ExpiryRankingDaysLeft(expireAt, now)
	}
	return points
}

// expiryPointsFor sums expiry points over the unique matched product IDs and
// returns the deduplicated ID list alongside the total.
func expiryPointsFor(matchedIDs []uint, pointsByProductID map[uint]int) (int, []uint) {
	seen := make(map[uint]bool, len(matchedIDs))
	uniqueIDs := make([]uint, 0, len(matchedIDs))
	total := 0
	for _, id := range matchedIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		uniqueIDs = append(uniqueIDs, id)
		total += pointsByProductID[id]
	}
	if uniqueIDs == nil {
		uniqueIDs = []uint{}
	}
	return total, uniqueIDs
}

// buildNormalizedIndex builds a searchable index of normalized product names and categories.
func buildNormalizedIndex(allProducts []dbModel.Product) []productIndex {
	normalized := make([]productIndex, len(allProducts))
	for i := range allProducts {
		p := &allProducts[i]
		var cats []string
		for _, c := range strings.Split(p.Categories, ",") {
			if nc := normalizeString(c); nc != "" {
				cats = append(cats, nc)
			}
		}
		normalized[i] = productIndex{
			name:       normalizeString(p.ProductName),
			categories: cats,
			productID:  p.ID,
		}
		normalized[i].nameWords = strings.Fields(normalized[i].name)
		normalized[i].catWords = make([][]string, len(cats))
		for j := range cats {
			normalized[i].catWords[j] = strings.Fields(cats[j])
		}
	}
	return normalized
}

// Match strength ranks, lower is stronger. Only rankExactName identifies a
// product for the destructive cook action; every weaker rank counts for
// display only. Category word matches outrank loose name substrings because
// curated tags are less prone to false positives ("nut" in "coconut").
const (
	rankExactName = iota
	rankWordName
	rankWordCategory
	rankSubstringName
	rankSubstringCategory
)

// ingredientMatched returns the best-matching product index entry for a
// normalized ingredient plus its match strength, or (nil, -1) when nothing
// matches. Candidates are ranked instead of taken first-hit so the entry
// chosen for the destructive cook action is the most specific product, not an
// arbitrary early row of an unordered query result.
func ingredientMatched(normIng string, normalized []productIndex) (*productIndex, int) {
	if normIng == "" {
		return nil, -1
	}
	// Providers disagree on ingredient granularity: Mealie reports the full
	// line ("500 g chicken breast"), Tandoor and TheMealDB report the bare
	// name ("chicken"). Both forms are compared so the exact-name rank — the
	// one that selects the product the irreversible cook action consumes —
	// fires for every provider instead of only the bare-name ones.
	bareIng := bareIngredientName(normIng)
	if bareIng == "" {
		bareIng = normIng
	}
	var best *productIndex
	bestRank := -1
	for i := range normalized {
		np := &normalized[i]
		rank := matchRank(normIng, bareIng, np)
		if rank < 0 {
			continue
		}
		if bestRank < 0 || rank < bestRank {
			best, bestRank = np, rank
		}
	}
	return best, bestRank
}

// matchRank scores how strongly an ingredient matches one index entry; -1
// means no match. normIng is the normalized ingredient and bareIng its
// quantity-stripped form. Empty names never match: strings.Contains with an
// empty needle is always true and would tie every ingredient to a nameless
// product.
func matchRank(normIng, bareIng string, np *productIndex) int {
	best := -1
	if np.name != "" {
		switch {
		case np.name == normIng || np.name == bareIng:
			return rankExactName
		case containsWholeWord(np.nameWords, normIng) || containsWholeWord(np.nameWords, bareIng):
			best = rankWordName
		case strings.Contains(np.name, normIng) || strings.Contains(normIng, np.name) ||
			strings.Contains(np.name, bareIng) || strings.Contains(bareIng, np.name):
			best = rankSubstringName
		}
	}
	for j := range np.categories {
		rank := -1
		switch {
		case containsWholeWord(np.catWords[j], normIng) || containsWholeWord(np.catWords[j], bareIng):
			rank = rankWordCategory
		case strings.Contains(np.categories[j], normIng) || strings.Contains(normIng, np.categories[j]) ||
			strings.Contains(np.categories[j], bareIng) || strings.Contains(bareIng, np.categories[j]):
			rank = rankSubstringCategory
		}
		if rank >= 0 && (best < 0 || rank < best) {
			best = rank
		}
	}
	return best
}

// ingredientQuantityTokens are the leading tokens dropped when reducing a raw
// ingredient line to the bare name of the food it refers to: measures, counts
// and filler. Only a leading run is stripped, so names that merely contain a
// unit-like word are untouched ("chicken", "olive oil", "coconut milk").
var ingredientQuantityTokens = map[string]bool{
	"g": true, "gr": true, "gram": true, "grams": true,
	"kg": true, "kilo": true, "kilos": true, "mg": true,
	"ml": true, "l": true, "ltr": true, "liter": true, "liters": true, "litres": true,
	"oz": true, "fl": true, "lb": true, "lbs": true, "pound": true, "pounds": true,
	"tsp": true, "tsps": true, "teaspoon": true, "teaspoons": true,
	"tbsp": true, "tbsps": true, "tbs": true, "tablespoon": true, "tablespoons": true,
	"cup": true, "cups": true, "c": true,
	"pinch": true, "pinches": true, "dash": true, "dashes": true,
	"handful": true, "handfuls": true, "splash": true, "splashes": true,
	"clove": true, "cloves": true, "slice": true, "slices": true,
	"can": true, "cans": true, "tin": true, "tins": true,
	"jar": true, "jars": true, "bottle": true, "bottles": true,
	"packet": true, "packets": true, "package": true, "packages": true, "pkg": true,
	"bunch": true, "bunches": true, "sprig": true, "sprigs": true,
	"piece": true, "pieces": true, "head": true, "heads": true,
	"stick": true, "sticks": true, "cube": true, "cubes": true,
	"a": true, "an": true, "the": true, "of": true,
}

// isIngredientQuantityToken reports whether a whitespace-delimited token is a
// quantity, a measure or filler that may precede the food name.
func isIngredientQuantityToken(token string) bool {
	token = strings.Trim(token, ".,()")
	if token == "" {
		return true
	}
	if ingredientQuantityTokens[token] {
		return true
	}
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return true
	}
	// Fractions such as "1/2".
	if numerator, denominator, found := strings.Cut(token, "/"); found {
		_, numeratorErr := strconv.ParseFloat(numerator, 64)
		_, denominatorErr := strconv.ParseFloat(denominator, 64)
		return numeratorErr == nil && denominatorErr == nil
	}
	// Glued quantities such as "200ml" or "1tbsp": a numeric prefix followed by
	// a unit. A leading digit is never part of a food name.
	if split := strings.IndexFunc(token, func(r rune) bool { return r < '0' || r > '9' }); split > 0 {
		if _, err := strconv.ParseFloat(token[:split], 64); err == nil {
			return true
		}
	}
	return false
}

// bareIngredientName drops the leading quantity run from a normalized
// ingredient, turning "500 g chicken breast" into "chicken breast" while
// leaving a name-only ingredient such as "chicken" unchanged.
func bareIngredientName(normIng string) string {
	fields := strings.Fields(normIng)
	cut := 0
	for cut < len(fields) && isIngredientQuantityToken(fields[cut]) {
		cut++
	}
	if cut == 0 {
		return normIng
	}
	return strings.Join(fields[cut:], " ")
}

// containsWholeWord reports whether needle appears as a whole word among the
// pre-split words of a name or category. Names and categories are normalized
// (lowercase, punctuation stripped, hyphens as spaces), so whitespace is a
// sufficient boundary and the tokens can be computed once at index time.
func containsWholeWord(words []string, needle string) bool {
	for _, field := range words {
		if field == needle {
			return true
		}
	}
	return false
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
