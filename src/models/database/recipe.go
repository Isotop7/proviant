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

// RecipeIngredient is one recipe entry. Portion is the portion the recipe
// calls for, as free text ("200 g", "1 cup", "a pinch"). It is never
// converted and never computed with: a recipe states how much of something a
// dish needs, not how many packs of it are in the cupboard. A nil ProductID
// marks a free-text ingredient that is not linked to household stock.
//
// The portion is display metadata. Cooking a recipe consumes a pack of every
// product it links, scaled by the servings, so the portion never decides what
// leaves the stock.
type RecipeIngredient struct {
	gorm.Model
	RecipeID  uint   `gorm:"index, not null"`
	ProductID *uint  `gorm:"index"`
	Name      string `gorm:"not null" json:"name"`
	Portion   string `json:"portion"`
	SortOrder int    `json:"sortOrder"`
}
