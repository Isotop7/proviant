package api

// IngredientMatch represents a single ingredient with its matched status
type IngredientMatch struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

// RecipeSuggestionResponse represents a single recipe suggestion for expiring products
type RecipeSuggestionResponse struct {
	ID                string            `json:"id"`
	Title             string            `json:"title"`
	ImageURL          string            `json:"imageUrl"`
	SourceURL         string            `json:"sourceUrl"`
	Ingredients       []IngredientMatch `json:"ingredients"`
	MatchedProducts   []string          `json:"matchedProducts"`   // deprecated, kept for compatibility
	MatchedProductIDs []uint            `json:"matchedProductIds"` // product IDs whose names/categories matched ingredients
	MissingCount      int               `json:"missingCount"`
	TotalIngredients  int               `json:"totalIngredients"`
	MatchPercent      float64           `json:"matchPercent"` // 0-100
}
