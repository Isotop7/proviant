package database

import "gorm.io/gorm"

// Recipe is a household-scoped saved recipe with structured ingredients.
type Recipe struct {
	gorm.Model
	HouseholdID  uint               `gorm:"index, not null"`
	Name         string             `gorm:"not null" json:"name"`
	Description  string             `json:"description"`
	Instructions string             `json:"instructions"`
	Servings     int                `gorm:"not null;default:1" json:"servings"`
	SourceURL    string             `json:"sourceUrl"`
	ImageURL     string             `json:"imageUrl"`
	CreatedBy    uint               `gorm:"not null;default:0" json:"createdBy"`
	Ingredients  []RecipeIngredient `gorm:"foreignKey:RecipeID" json:"ingredients"`
}

// RecipeIngredient is one recipe entry. Amount is expressed in Unit; a nil
// ProductID marks a free-text ingredient that is not linked to household stock.
type RecipeIngredient struct {
	gorm.Model
	RecipeID  uint   `gorm:"index, not null"`
	ProductID *uint  `gorm:"index"`
	Name      string `gorm:"not null" json:"name"`
	Amount    int    `json:"amount"`
	Unit      string `json:"unit"`
	SortOrder int    `json:"sortOrder"`
}
