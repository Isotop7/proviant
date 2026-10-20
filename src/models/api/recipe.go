package api

import (
	dbModel "github.com/Isotop7/proviant/models/database"
)

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

// RecipeIngredientInput is one ingredient row of a recipe create/update request.
// The portion is the amount the recipe calls for, as free text ("200 g"), and
// is display metadata: cooking a recipe consumes a pack of every product it
// links, so it never decides what leaves the stock.
type RecipeIngredientInput struct {
	ProductID *uint  `json:"productId"`
	Name      string `json:"name" binding:"required,max=255"`
	Portion   string `json:"portion" binding:"max=40"`
	SortOrder int    `json:"sortOrder"`
}

// RecipeIngredientResponse is the client-facing view of a recipe ingredient.
type RecipeIngredientResponse struct {
	ID        uint   `json:"id"`
	ProductID *uint  `json:"productId"`
	Name      string `json:"name"`
	Portion   string `json:"portion"`
	SortOrder int    `json:"sortOrder"`
}

// CreateRecipeRequest is the request body for POST /api/v1/recipes.
type CreateRecipeRequest struct {
	Name         string                  `json:"name" binding:"required,max=255"`
	Description  string                  `json:"description"`
	Instructions string                  `json:"instructions"`
	Servings     int                     `json:"servings" binding:"min=0"`
	SourceURL    string                  `json:"sourceUrl"`
	ImageURL     string                  `json:"imageUrl"`
	Ingredients  []RecipeIngredientInput `json:"ingredients" binding:"max=100"`
}

// UpdateRecipeRequest is the request body for PATCH /api/v1/recipes/{id}.
type UpdateRecipeRequest struct {
	Name         *string                  `json:"name" binding:"omitempty,max=255"`
	Description  *string                  `json:"description"`
	Instructions *string                  `json:"instructions"`
	Servings     *int                     `json:"servings" binding:"omitempty,min=1"`
	SourceURL    *string                  `json:"sourceUrl"`
	ImageURL     *string                  `json:"imageUrl"`
	Ingredients  *[]RecipeIngredientInput `json:"ingredients" binding:"omitempty,max=100"`
}

// CookRecipeRequest is the request body for POST /api/v1/recipes/{id}/cook.
// Servings states how many people the dish is cooked for. The stock is
// counted in packs, so the requested servings scale the pack count: a cook
// for twice the recipe's own servings takes two packs of every product the
// recipe links.
type CookRecipeRequest struct {
	Servings int `json:"servings" binding:"min=0"`
}

// RecipeResponse is the client-facing view of a saved recipe.
type RecipeResponse struct {
	ID           uint                       `json:"id"`
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	Instructions string                     `json:"instructions"`
	Servings     int                        `json:"servings"`
	SourceURL    string                     `json:"sourceUrl"`
	ImageURL     string                     `json:"imageUrl"`
	CreatedBy    uint                       `json:"createdBy"`
	Ingredients  []RecipeIngredientResponse `json:"ingredients"`
}

// NewRecipeResponse projects a saved recipe onto the wire shape.
func NewRecipeResponse(recipe *dbModel.Recipe) RecipeResponse {
	if recipe == nil {
		return RecipeResponse{}
	}
	resp := RecipeResponse{
		ID:           recipe.ID,
		Name:         recipe.Name,
		Description:  recipe.Description,
		Instructions: recipe.Instructions,
		Servings:     recipe.Servings,
		SourceURL:    recipe.SourceURL,
		ImageURL:     recipe.ImageURL,
		CreatedBy:    recipe.CreatedBy,
		Ingredients:  make([]RecipeIngredientResponse, len(recipe.Ingredients)),
	}
	for i := range recipe.Ingredients {
		resp.Ingredients[i] = RecipeIngredientResponse{
			ID:        recipe.Ingredients[i].ID,
			ProductID: recipe.Ingredients[i].ProductID,
			Name:      recipe.Ingredients[i].Name,
			Portion:   recipe.Ingredients[i].Portion,
			SortOrder: recipe.Ingredients[i].SortOrder,
		}
	}
	return resp
}

// NewRecipeResponses projects a slice of saved recipes onto the wire shape.
func NewRecipeResponses(recipes []dbModel.Recipe) []RecipeResponse {
	if recipes == nil {
		return nil
	}
	out := make([]RecipeResponse, len(recipes))
	for i := range recipes {
		out[i] = NewRecipeResponse(&recipes[i])
	}
	return out
}
