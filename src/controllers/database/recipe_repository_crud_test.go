package database

import (
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestRecipeRepository_CreateAndGetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	t.Run("returns recipe with ingredients ordered by sort order", func(t *testing.T) {
		recipe := &dbModel.Recipe{
			HouseholdID:  1,
			Name:         "Pancakes",
			Description:  "Fluffy pancakes",
			Instructions: "Mix and fry.",
			Servings:     4,
			Ingredients: []dbModel.RecipeIngredient{
				{Name: "Eggs", Portion: "2 pcs", SortOrder: 1},
				{Name: "Flour", Portion: "250 g", SortOrder: 0},
			},
		}
		if err := repo.Create(recipe); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(recipe.ID, 1)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Name != "Pancakes" {
			t.Errorf("Name = %q, want %q", got.Name, "Pancakes")
		}
		if len(got.Ingredients) != 2 {
			t.Fatalf("ingredients = %d, want 2", len(got.Ingredients))
		}
		if got.Ingredients[0].Name != "Flour" || got.Ingredients[1].Name != "Eggs" {
			t.Errorf("ingredient order = [%s, %s], want [Flour, Eggs]", got.Ingredients[0].Name, got.Ingredients[1].Name)
		}
	})

	t.Run("wrong household is not found", func(t *testing.T) {
		recipe := testutil.CreateTestRecipe(db, 1)
		if _, err := repo.GetByID(recipe.ID, 2); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByID() error = %v, want gorm.ErrRecordNotFound", err)
		}
	})
}

func TestRecipeRepository_ListByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	t.Run("lists only own household recipes by name", func(t *testing.T) {
		testutil.CreateTestRecipe(db, 1)
		other := testutil.CreateTestRecipe(db, 2)
		other.Name = "Foreign"
		if err := db.Save(other).Error; err != nil {
			t.Fatalf("save other recipe: %v", err)
		}

		recipes, err := repo.ListByHousehold(1)
		if err != nil {
			t.Fatalf("ListByHousehold() error = %v", err)
		}
		if len(recipes) != 1 {
			t.Fatalf("recipes = %d, want 1", len(recipes))
		}
		if recipes[0].HouseholdID != 1 {
			t.Errorf("HouseholdID = %d, want 1", recipes[0].HouseholdID)
		}
	})
}

func TestRecipeRepository_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	t.Run("replaces ingredients atomically", func(t *testing.T) {
		recipe := testutil.CreateTestRecipe(db, 1)
		recipe.Name = "Updated"
		recipe.Ingredients = []dbModel.RecipeIngredient{
			{Name: "Sugar", Portion: "50 g", SortOrder: 0},
		}
		if err := repo.Update(recipe); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(recipe.ID, 1)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Name != "Updated" {
			t.Errorf("Name = %q, want %q", got.Name, "Updated")
		}
		if len(got.Ingredients) != 1 || got.Ingredients[0].Name != "Sugar" {
			t.Errorf("ingredients = %+v, want exactly [Sugar]", got.Ingredients)
		}

		var replacedCount int64
		if err := db.Unscoped().Model(&dbModel.RecipeIngredient{}).Where("recipe_id = ?", recipe.ID).Count(&replacedCount).Error; err != nil {
			t.Fatalf("count ingredients: %v", err)
		}
		if replacedCount != 1 {
			t.Errorf("ingredient rows = %d, want 1 (replaced rows must not accumulate)", replacedCount)
		}
	})
}

func TestRecipeRepository_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	t.Run("deletes recipe and ingredients", func(t *testing.T) {
		recipe := testutil.CreateTestRecipe(db, 1)
		if err := repo.Delete(recipe.ID, 1); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := repo.GetByID(recipe.ID, 1); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByID() error = %v, want gorm.ErrRecordNotFound", err)
		}
		var ingredientCount int64
		if err := db.Model(&dbModel.RecipeIngredient{}).Where("recipe_id = ?", recipe.ID).Count(&ingredientCount).Error; err != nil {
			t.Fatalf("count ingredients: %v", err)
		}
		if ingredientCount != 0 {
			t.Errorf("ingredient rows = %d, want 0", ingredientCount)
		}
	})

	t.Run("wrong household delete is a no-op", func(t *testing.T) {
		recipe := testutil.CreateTestRecipe(db, 1)
		if err := repo.Delete(recipe.ID, 2); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := repo.GetByID(recipe.ID, 1); err != nil {
			t.Errorf("GetByID() error = %v, want recipe to survive", err)
		}
	})
}
