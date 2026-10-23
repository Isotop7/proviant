package services

import (
	"testing"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

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

// A cook of a recipe costs one pack of every linked ingredient: the stock is
// counted in packs, and the servings scale the dish, not the stock.
func TestRecipeService_CookRecipeScalesThePackCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestRecipeService(db)
	household := testutil.CreateTestHousehold(db, 1)
	user := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(product).Update("quantity", 3).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	recipe := &dbModel.Recipe{
		HouseholdID: household.ID,
		Name:        "Test Recipe",
		Servings:    2,
		CreatedBy:   user.ID,
		Ingredients: []dbModel.RecipeIngredient{
			{ProductID: &product.ID, Name: product.ProductName, Portion: "200 g"},
		},
	}
	if err := db.Create(recipe).Error; err != nil {
		t.Fatalf("create recipe: %v", err)
	}

	// Twice the serving count costs two packs: the stock is counted in packs,
	// so the ratio of the requested servings to the recipe's own becomes the
	// number of packs.
	consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 4, user.ID)
	if internalErr != nil {
		t.Fatalf("internalErr = %v, want nil", internalErr)
	}
	if consumed != 0 || partial != 1 || len(errs) != 0 {
		t.Errorf("consumed/partial/errs = %d/%d/%v, want 0/1/[]", consumed, partial, errs)
	}
	if reloaded := reloadProduct(t, db, product.ID); reloaded.Quantity != 1 {
		t.Errorf("count = %d, want 1 (three packs - two cooked)", reloaded.Quantity)
	}
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

	t.Run("cooks a pack of every linked ingredient", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 10})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Portion: "2 g",
		})

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 || len(errs) != 0 {
			t.Errorf("consumed/partial/errs = %d/%d/%v, want 0/1/[]", consumed, partial, errs)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Quantity != 9 {
			t.Errorf("product count = %d, want 9 (10 - one pack)", reloaded.Quantity)
		}
	})

	t.Run("more servings than the recipe scales the pack count", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 10})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Portion: "2 g",
		})

		_, partial, _, internalErr := svc.CookRecipe(recipe.ID, 6, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if partial != 1 {
			t.Errorf("partial = %d, want 1", partial)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Quantity != 7 {
			t.Errorf("product count = %d, want 7 (10 - three packs for three times the servings)", reloaded.Quantity)
		}
	})

	t.Run("the ingredient unit is documentation and never blocks the cook", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 3000})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Portion: "1 kg",
		})

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 || len(errs) != 0 {
			t.Errorf("consumed/partial/errs = %d/%d/%v, want 0/1/[]", consumed, partial, errs)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Quantity != 2999 {
			t.Errorf("product count = %d, want 2999 (3000 - one pack)", reloaded.Quantity)
		}
	})

	t.Run("zero servings cooks the recipe serving count", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 10})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: product.ProductName, Portion: "2 g",
		})

		_, partial, _, internalErr := svc.CookRecipe(recipe.ID, 0, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if partial != 1 {
			t.Errorf("partial = %d, want 1", partial)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Quantity != 9 {
			t.Errorf("product count = %d, want 9 (10 - one pack)", reloaded.Quantity)
		}
	})

	t.Run("reports gaps for unlinked ingredients", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 10})
		recipe := seedRecipe(t, db, householdID, userID, 2,
			dbModel.RecipeIngredient{ProductID: &product.ID, Name: "Flour", Portion: "2 g"},
			dbModel.RecipeIngredient{Name: "Salt", Portion: "1 g"},
		)

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 {
			t.Errorf("consumed/partial = %d/%d, want 0/1", consumed, partial)
		}
		if len(errs) != 1 {
			t.Errorf("errs = %v, want 1 gap entry", errs)
		}
	})

	t.Run("an ingredient without an amount is still cooked", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"quantity": 5})
		recipe := seedRecipe(t, db, householdID, userID, 2, dbModel.RecipeIngredient{
			ProductID: &product.ID, Name: "Salt", Portion: "0 g",
		})

		consumed, partial, errs, internalErr := svc.CookRecipe(recipe.ID, 2, userID)

		if internalErr != nil {
			t.Fatalf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 {
			t.Errorf("consumed/partial = %d/%d, want 0/1", consumed, partial)
		}
		if len(errs) != 0 {
			t.Errorf("errs = %v, want none", errs)
		}
		reloaded := reloadProduct(t, db, product.ID)
		if reloaded.Quantity != 4 {
			t.Errorf("product count = %d, want 4 (five packs - one cooked)", reloaded.Quantity)
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
