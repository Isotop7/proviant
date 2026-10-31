package database

import (
	"sort"
	"strings"

	"github.com/Isotop7/proviant/errors"
	dbModel "github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// canonicalCategory is one canonical category key, prepared for matching.
type canonicalCategory struct {
	key        string
	normalized string
	price      dbModel.ProductCategoryPrice
}

// CategoryResolver resolves a free-text category string against the canonical
// category keys of the global ProductCategoryPrice table.
//
// The resolution is explicit: among all canonical keys that match the text,
// the longest one wins, and a tie is broken by the lexicographically smaller
// key. The rule never reads the order the rows come back in, so a category
// string always names the same category.
//
// A resolver is read-only after construction and is meant to be built once per
// request and shared by the goroutines that record savings events: the
// canonical key set is small and does not change while a request runs.
type CategoryResolver struct {
	categories []canonicalCategory
}

// LoadCategoryResolver reads the canonical categories and prepares them for
// resolution.
func LoadCategoryResolver(db *gorm.DB) (*CategoryResolver, error) {
	var prices []dbModel.ProductCategoryPrice
	if err := db.Find(&prices).Error; err != nil {
		return nil, err
	}
	return NewCategoryResolver(prices), nil
}

// NewCategoryResolver prepares a resolver from already loaded canonical rows.
func NewCategoryResolver(prices []dbModel.ProductCategoryPrice) *CategoryResolver {
	categories := make([]canonicalCategory, 0, len(prices))
	for i := range prices {
		normalized := normalizeCategoryKey(prices[i].CategoryKey)
		if normalized == "" {
			continue
		}
		categories = append(categories, canonicalCategory{
			key:        prices[i].CategoryKey,
			normalized: normalized,
			price:      prices[i],
		})
	}
	// Longest first, so the first match is the longest match; the
	// lexicographic tie-break makes the outcome independent of the order the
	// rows were read in.
	sort.Slice(categories, func(i, j int) bool {
		if len(categories[i].normalized) != len(categories[j].normalized) {
			return len(categories[i].normalized) > len(categories[j].normalized)
		}
		return categories[i].normalized < categories[j].normalized
	})
	return &CategoryResolver{categories: categories}
}

// Resolve returns the canonical category the free text names, and whether one
// was found.
func (r *CategoryResolver) Resolve(categories string) (*dbModel.ProductCategoryPrice, bool) {
	tokens := categoryTokens(categories)
	if len(tokens) == 0 {
		return nil, false
	}
	for i := range r.categories {
		for _, token := range tokens {
			if categoryMatches(token, r.categories[i].normalized) {
				return &r.categories[i].price, true
			}
		}
	}
	return nil, false
}

// ResolveKey returns the canonical key the free text names, and whether one
// was found.
func (r *CategoryResolver) ResolveKey(categories string) (string, bool) {
	price, ok := r.Resolve(categories)
	if !ok {
		return "", false
	}
	return price.CategoryKey, true
}

// Canonical returns the canonical row of a key.
func (r *CategoryResolver) Canonical(key string) (dbModel.ProductCategoryPrice, bool) {
	for i := range r.categories {
		if r.categories[i].key == key {
			return r.categories[i].price, true
		}
	}
	return dbModel.ProductCategoryPrice{}, false
}

// Keys returns the canonical keys, longest first.
func (r *CategoryResolver) Keys() []string {
	keys := make([]string, 0, len(r.categories))
	for i := range r.categories {
		keys = append(keys, r.categories[i].key)
	}
	return keys
}

// CanonicalCategoryKey validates a client-supplied category key. A missing or
// empty key clears the category; a key that names no canonical category is
// refused. Without a resolver there is no canonical set to check against, so
// the key is stored as it came.
func CanonicalCategoryKey(resolver *CategoryResolver, key *string) (*string, error) {
	if key == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*key)
	if trimmed == "" {
		return nil, nil
	}
	if resolver == nil {
		return &trimmed, nil
	}
	if _, ok := resolver.Canonical(trimmed); !ok {
		return nil, errors.ErrCategoryNotFound
	}
	return &trimmed, nil
}

// categoryTokens splits a free-text category string into the tokens a
// canonical key may match. A language tag ("en:dairy products") is dropped,
// and the token is lower-cased with dashes turned into spaces.
func categoryTokens(categories string) []string {
	raw := strings.Split(categories, ",")
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.TrimSpace(token)
		if idx := strings.Index(token, ":"); idx != -1 {
			token = strings.TrimSpace(token[idx+1:])
		}
		token = strings.ToLower(strings.ReplaceAll(token, "-", " "))
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// normalizeCategoryKey prepares a canonical key for matching.
func normalizeCategoryKey(key string) string {
	tokens := categoryTokens(key)
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

// categoryMatches reports whether a token names a canonical key. The
// containment runs both ways because Open Food Facts tags are multi-word:
// "dairy products" names the key "dairy products", and "dairy" names it too.
func categoryMatches(token, key string) bool {
	if token == "" || key == "" {
		return false
	}
	return strings.Contains(token, key) || strings.Contains(key, token)
}
