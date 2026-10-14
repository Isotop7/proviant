package database

import (
	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
)

// RecipeRepositoryInterface persists household-scoped saved recipes with their ingredients.
type RecipeRepositoryInterface interface {
	Create(recipe *database.Recipe) error
	GetByID(id, householdID uint) (*database.Recipe, error)
	ListByHousehold(householdID uint) ([]database.Recipe, error)
	Update(recipe *database.Recipe) error
	Delete(id, householdID uint) error
}

var _ RecipeRepositoryInterface = (*RecipeRepository)(nil)

type RecipeRepository struct {
	DB *gorm.DB
}

func NewRecipeRepository(db *gorm.DB) *RecipeRepository {
	return &RecipeRepository{DB: db}
}

// Create stores a recipe together with its ingredient rows.
func (r *RecipeRepository) Create(recipe *database.Recipe) error {
	return r.DB.Create(recipe).Error
}

// GetByID retrieves a recipe with its ingredients ordered by SortOrder.
func (r *RecipeRepository) GetByID(id, householdID uint) (*database.Recipe, error) {
	var recipe database.Recipe
	err := r.DB.Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).Where("id = ? AND household_id = ?", id, householdID).First(&recipe).Error
	if err != nil {
		return nil, err
	}
	return &recipe, nil
}

// ListByHousehold retrieves all recipes of a household without their ingredients.
func (r *RecipeRepository) ListByHousehold(householdID uint) ([]database.Recipe, error) {
	var recipes []database.Recipe
	err := r.DB.Where("household_id = ?", householdID).Order("name ASC").Find(&recipes).Error
	return recipes, err
}

// Update replaces the recipe fields and its ingredient rows in one transaction,
// so a half-written ingredient set can never be observed by a later cook.
func (r *RecipeRepository) Update(recipe *database.Recipe) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Ingredients").Save(recipe).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("recipe_id = ?", recipe.ID).Delete(&database.RecipeIngredient{}).Error; err != nil {
			return err
		}
		if len(recipe.Ingredients) == 0 {
			return nil
		}
		for i := range recipe.Ingredients {
			recipe.Ingredients[i].RecipeID = recipe.ID
			recipe.Ingredients[i].ID = 0
		}
		return tx.Create(&recipe.Ingredients).Error
	})
}

// Delete removes a recipe and its ingredient rows in one transaction.
func (r *RecipeRepository) Delete(id, householdID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND household_id = ?", id, householdID).Delete(&database.Recipe{}).Error; err != nil {
			return err
		}
		return tx.Where("recipe_id = ?", id).Delete(&database.RecipeIngredient{}).Error
	})
}
