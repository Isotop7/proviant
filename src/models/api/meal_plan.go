package api

import (
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
)

// MealPlanEntryResponse is the client-facing view of a planned meal. Date is
// rendered as a YYYY-MM-DD string, the same format the request accepts.
type MealPlanEntryResponse struct {
	ID         uint   `json:"id"`
	Date       string `json:"date"`
	RecipeID   uint   `json:"recipeId"`
	RecipeName string `json:"recipeName"`
	Servings   int    `json:"servings"`
	Notes      string `json:"notes"`
	CreatedBy  uint   `json:"createdBy"`
}

// MealPlanDateFormat is the wire format of meal plan dates.
const MealPlanDateFormat = "2006-01-02"

// NewMealPlanEntryResponse projects a planned meal onto the wire shape.
func NewMealPlanEntryResponse(entry *dbModel.MealPlanEntry, recipeName string) MealPlanEntryResponse {
	if entry == nil {
		return MealPlanEntryResponse{}
	}
	return MealPlanEntryResponse{
		ID:         entry.ID,
		Date:       entry.Date.Format(MealPlanDateFormat),
		RecipeID:   entry.RecipeID,
		RecipeName: recipeName,
		Servings:   entry.Servings,
		Notes:      entry.Notes,
		CreatedBy:  entry.CreatedBy,
	}
}

// NewMealPlanEntryResponses projects a slice of planned meals onto the wire shape.
func NewMealPlanEntryResponses(entries []dbModel.MealPlanEntry, recipeNames map[uint]string) []MealPlanEntryResponse {
	if entries == nil {
		return nil
	}
	out := make([]MealPlanEntryResponse, len(entries))
	for i := range entries {
		out[i] = NewMealPlanEntryResponse(&entries[i], recipeNames[entries[i].RecipeID])
	}
	return out
}

// ParseMealPlanDate parses a YYYY-MM-DD wire date.
func ParseMealPlanDate(raw string) (time.Time, error) {
	return time.Parse(MealPlanDateFormat, raw)
}

// CreateMealPlanEntryRequest is the request body for POST /api/v1/mealplans.
type CreateMealPlanEntryRequest struct {
	Date     string `json:"date" binding:"required"`
	RecipeID uint   `json:"recipeId" binding:"required"`
	Servings int    `json:"servings" binding:"min=0"`
	Notes    string `json:"notes"`
}

// UpdateMealPlanEntryRequest is the request body for PATCH /api/v1/mealplans/{id}.
type UpdateMealPlanEntryRequest struct {
	Date     *string `json:"date"`
	RecipeID *uint   `json:"recipeId"`
	Servings *int    `json:"servings" binding:"omitempty,min=1"`
	Notes    *string `json:"notes"`
}

// GenerateShoppingListRequest is the request body for POST /api/v1/mealplans/shopping-list.
type GenerateShoppingListRequest struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}
