package services

import (
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func reloadProduct(t *testing.T, db *gorm.DB, productID uint) dbModel.Product {
	t.Helper()
	var product dbModel.Product
	if err := db.First(&product, productID).Error; err != nil {
		t.Fatalf("reload product: %v", err)
	}
	return product
}

func newTestRecipeService(db *gorm.DB) *RecipeService {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)
	return NewRecipeService(repos, &logger)
}

func TestScaleIngredientAmount(t *testing.T) {
	t.Run("scales proportionally and rounds half up", func(t *testing.T) {
		if got := scaleIngredientAmount(2, 4, 2); got != 4 {
			t.Errorf("scaleIngredientAmount(2, 4, 2) = %d, want 4", got)
		}
		if got := scaleIngredientAmount(1, 2, 3); got != 1 {
			t.Errorf("scaleIngredientAmount(1, 2, 3) = %d, want 1", got)
		}
		if got := scaleIngredientAmount(1, 1, 3); got != 0 {
			t.Errorf("scaleIngredientAmount(1, 1, 3) = %d, want 0", got)
		}
	})

	t.Run("zero servings cooks the base serving count", func(t *testing.T) {
		if got := scaleIngredientAmount(5, 0, 2); got != 5 {
			t.Errorf("scaleIngredientAmount(5, 0, 2) = %d, want 5", got)
		}
	})

	t.Run("invalid base servings is treated as one", func(t *testing.T) {
		if got := scaleIngredientAmount(3, 2, 0); got != 6 {
			t.Errorf("scaleIngredientAmount(3, 2, 0) = %d, want 6", got)
		}
	})
}

func TestRecipeService_CookRecipe(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *RecipeService, uint, uint) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		svc := newTestRecipeService(db)
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		return db, svc, household.ID, user.ID
	}

	seedRecipe := func(t *testing.T, db *gorm.DB, householdID, userID uint, servings int, ingredients ...dbModel.RecipeIngredient) *dbModel.Recipe {
		t.Helper()
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Test Recipe",
			Servings:    servings,
			CreatedBy:   userID,
			Ingredients: ingredients,
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		return recipe
	}

	t.Run("scales ingredient amounts to the requested servings", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"amount": 10, "unit": "g"})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Amount: 2, Unit: "g",
		})

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 4, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 || len(errs) != 0 {
			t.Errorf("consumed/partial/errs = %d/%d/%v, want 0/1/[]", consumed, partial, errs)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Amount != 6 {
			t.Errorf("product amount = %d, want 6 (10 - 2*4/2)", reloaded.Amount)
		}
	})

	t.Run("zero servings cooks the recipe serving count", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"amount": 10, "unit": "g"})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Amount: 2, Unit: "g",
		})

		_, partial, _, internalErr := svc.CookRecipe(recipe.ID, 0, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if partial != 1 {
			t.Errorf("partial = %d, want 1", partial)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Amount != 8 {
			t.Errorf("product amount = %d, want 8 (10 - 2)", reloaded.Amount)
		}
	})

	t.Run("reports gaps for unlinked and unit-mismatched ingredients", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"amount": 10, "unit": "g"})
		other := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(other).Updates(map[string]any{"amount": 10, "unit": "ml"})
		recipe := seedRecipe(t, db, householdID, userID, 2,
			dbModel.RecipeIngredient{ProductID: &product.ID, Name: "Flour", Amount: 2, Unit: "g"},
			dbModel.RecipeIngredient{Name: "Salt", Amount: 1, Unit: "g"},
			dbModel.RecipeIngredient{ProductID: &other.ID, Name: "Cream", Amount: 2, Unit: "g"},
		)

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 {
			t.Errorf("consumed/partial = %d/%d, want 0/1", consumed, partial)
		}
		if len(errs) != 2 {
			t.Errorf("errs = %v, want 2 gap entries", errs)
		}
	})

	t.Run("ingredient without amount is reported as gap", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"amount": 5, "unit": "g"})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: "Salt", Amount: 0, Unit: "g",
		})

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 0 {
			t.Errorf("consumed/partial = %d/%d, want 0/0", consumed, partial)
		}
		if len(errs) != 1 {
			t.Errorf("errs = %v, want one gap entry", errs)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Amount != 5 {
			t.Errorf("product amount = %d, want 5 (untouched)", reloaded.Amount)
		}
	})

	t.Run("unknown recipe reports record not found", func(t *testing.T) {
		_, svc, _, userID := setup(t)

		_, _, _, internalErr := svc.CookRecipe(9999, 2, userID)

		if internalErr != gorm.ErrRecordNotFound {
			t.Errorf("internalErr = %v, want gorm.ErrRecordNotFound", internalErr)
		}
	})
}
