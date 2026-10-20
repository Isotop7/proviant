package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func createRecipeRequest() map[string]any {
	return map[string]any{
		"name":         "Pancakes",
		"description":  "Fluffy pancakes",
		"instructions": "Mix and fry.",
		"servings":     4,
		"ingredients": []map[string]any{
			{"name": "Flour", "quantity": 250, "unit": "g", "sortOrder": 0},
			{"name": "Eggs", "quantity": 2, "unit": "pcs", "sortOrder": 1},
		},
	}
}

func TestCreateRecipe(t *testing.T) {
	t.Run("creates recipe with ingredients", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, createRecipeRequest())

		CreateRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var recipe apiModel.RecipeResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &recipe); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if recipe.Name != "Pancakes" || recipe.Servings != 4 {
			t.Errorf("recipe = %+v, want Pancakes with 4 servings", recipe)
		}
		if len(recipe.Ingredients) != 2 {
			t.Errorf("ingredients = %d, want 2", len(recipe.Ingredients))
		}
	})

	t.Run("missing name is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": " "})

		CreateRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestListRecipes(t *testing.T) {
	t.Run("lists own household recipes", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes", nil)

		ListRecipes(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var recipes []apiModel.RecipeResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &recipes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(recipes) != 1 {
			t.Errorf("recipes = %d, want 1", len(recipes))
		}
	})
}

func TestGetRecipe(t *testing.T) {
	t.Run("returns recipe with ingredients", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+itoa(recipe.ID), nil)
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		GetRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var got apiModel.RecipeResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Name != "Test Recipe" {
			t.Errorf("Name = %q, want %q", got.Name, "Test Recipe")
		}
		if len(got.Ingredients) != 1 {
			t.Errorf("ingredients = %d, want 1", len(got.Ingredients))
		}
	})

	t.Run("unknown recipe is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/9999", nil)
		env.Ctx.Params = gin.Params{{Key: "id", Value: "9999"}}

		GetRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}

func TestUpdateRecipe(t *testing.T) {
	t.Run("updates fields and replaces ingredients", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{
			"name": "Renamed",
			"ingredients": []map[string]any{
				{"name": "Sugar", "quantity": 50, "unit": "g", "sortOrder": 0},
			},
		})
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		UpdateRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var got apiModel.RecipeResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Name != "Renamed" {
			t.Errorf("Name = %q, want %q", got.Name, "Renamed")
		}
		if len(got.Ingredients) != 1 || got.Ingredients[0].Name != "Sugar" {
			t.Errorf("ingredients = %+v, want exactly [Sugar]", got.Ingredients)
		}
	})

	t.Run("unknown recipe is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"name": "Renamed"})
		env.Ctx.Params = gin.Params{{Key: "id", Value: "9999"}}

		UpdateRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}

func TestDeleteRecipe(t *testing.T) {
	t.Run("deletes recipe", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/"+itoa(recipe.ID), nil)
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		DeleteRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var count int64
		if err := env.DB.Model(&dbModel.Recipe{}).Where("id = ?", recipe.ID).Count(&count).Error; err != nil {
			t.Fatalf("count recipes: %v", err)
		}
		if count != 0 {
			t.Errorf("recipe rows = %d, want 0", count)
		}
	})
}

func TestCookRecipe(t *testing.T) {
	seedCookRecipe := func(t *testing.T, env *handlerTestEnv, portion string) (*dbModel.Product, *dbModel.Recipe) {
		t.Helper()
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.Quantity = 5
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		recipe := &dbModel.Recipe{
			HouseholdID: env.Household.ID,
			Name:        "Soup",
			Servings:    2,
			CreatedBy:   env.User.ID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: product.ProductName, Portion: portion, SortOrder: 0},
			},
		}
		if err := env.DB.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		return product, recipe
	}

	t.Run("consumes linked ingredients", func(t *testing.T) {
		env := setupHandlerTest(t)
		product, recipe := seedCookRecipe(t, env, "2 g")
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 0})
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		CookRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var res apiModel.CookResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if res.Partial != 1 || res.Consumed != 0 {
			t.Errorf("consumed/partial = %d/%d, want 0/1", res.Consumed, res.Partial)
		}
		reloaded, err := env.AppCtx.Repos.Products.GetProductByID(product.ID, env.User.ID)
		if err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if reloaded.Quantity != 4 {
			t.Errorf("product count = %d, want 4 (five packs - one cooked)", reloaded.Quantity)
		}
	})

	t.Run("the ingredient portion never blocks the cook", func(t *testing.T) {
		env := setupHandlerTest(t)
		product, recipe := seedCookRecipe(t, env, "2 g")
		ingredient := recipe.Ingredients[0]
		ingredient.Portion = "2 kg"
		if err := env.DB.Save(&ingredient).Error; err != nil {
			t.Fatalf("save ingredient: %v", err)
		}
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 0})
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		CookRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var res apiModel.CookResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(res.Errors) != 0 {
			t.Errorf("errors = %v, want none", res.Errors)
		}
		reloaded, err := env.AppCtx.Repos.Products.GetProductByID(product.ID, env.User.ID)
		if err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if reloaded.Quantity != 4 {
			t.Errorf("product count = %d, want 4 (five packs - one cooked)", reloaded.Quantity)
		}
	})

	t.Run("unlinked ingredient is reported as gap", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 0})
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(recipe.ID)}}

		CookRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", env.W.Code, env.W.Body.String())
		}
		var res apiModel.CookResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(res.Errors) != 1 {
			t.Errorf("errors = %v, want one gap error", res.Errors)
		}
	})

	t.Run("unknown recipe is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 0})
		env.Ctx.Params = gin.Params{{Key: "id", Value: "9999"}}

		CookRecipe(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}
