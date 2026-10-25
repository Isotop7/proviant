package api

import dbModel "github.com/Isotop7/proviant/models/database"

// HouseholdCategoryResponse is the client-facing view of one category of a
// household's list: the canonical category with the household's overrides
// applied.
type HouseholdCategoryResponse struct {
	CategoryKey             string `json:"categoryKey"`
	DisplayName             string `json:"displayName"`
	SortOrder               int    `json:"sortOrder"`
	Hidden                  bool   `json:"hidden"`
	DefaultShelfLifeDays    *int   `json:"defaultShelfLifeDays,omitempty"`
	DefaultDaysAfterOpening *int   `json:"defaultDaysAfterOpening,omitempty"`
}

// NewHouseholdCategoryResponse projects a household category onto the wire
// shape.
func NewHouseholdCategoryResponse(category *dbModel.HouseholdCategoryView) HouseholdCategoryResponse {
	if category == nil {
		return HouseholdCategoryResponse{}
	}
	return HouseholdCategoryResponse{
		CategoryKey:             category.CategoryKey,
		DisplayName:             category.DisplayName,
		SortOrder:               category.SortOrder,
		Hidden:                  category.Hidden,
		DefaultShelfLifeDays:    category.DefaultShelfLifeDays,
		DefaultDaysAfterOpening: category.DefaultDaysAfterOpening,
	}
}

// NewHouseholdCategoryResponses projects a slice of household categories.
func NewHouseholdCategoryResponses(categories []dbModel.HouseholdCategoryView) []HouseholdCategoryResponse {
	if categories == nil {
		return nil
	}
	out := make([]HouseholdCategoryResponse, len(categories))
	for i := range categories {
		out[i] = NewHouseholdCategoryResponse(&categories[i])
	}
	return out
}
