package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func apiDate(t *testing.T, raw string) time.Time {
	t.Helper()
	date, err := apiModel.ParseMealPlanDate(raw)
	if err != nil {
		t.Fatalf("parse date %q: %v", raw, err)
	}
	return date
}

func TestListMealPlanEntries(t *testing.T) {
	t.Run("returns entries with recipe names", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		testutil.CreateTestMealPlanEntry(env.DB, env.Household.ID, recipe.ID, apiDate(t, "2026-10-12"), 2, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mealplans?from=2026-10-12&to=2026-10-18", nil)

		ListMealPlanEntries(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var entries []apiModel.MealPlanEntryResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &entries); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(entries))
		}
		if entries[0].RecipeName != "Test Recipe" {
			t.Errorf("RecipeName = %q, want %q", entries[0].RecipeName, "Test Recipe")
		}
		if entries[0].Date != "2026-10-12" {
			t.Errorf("Date = %q, want %q", entries[0].Date, "2026-10-12")
		}
	})

	t.Run("invalid date is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mealplans?from=yesterday&to=2026-10-18", nil)

		ListMealPlanEntries(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestCreateMealPlanEntry(t *testing.T) {
	t.Run("creates entry with recipe servings default", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{
			"date":     "2026-10-12",
			"recipeId": recipe.ID,
			"servings": 0,
		})

		CreateMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var entry apiModel.MealPlanEntryResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &entry); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if entry.Servings != recipe.Servings {
			t.Errorf("Servings = %d, want recipe default %d", entry.Servings, recipe.Servings)
		}
		if entry.Date != "2026-10-12" {
			t.Errorf("Date = %q, want %q", entry.Date, "2026-10-12")
		}
	})

	t.Run("unknown recipe is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"date": "2026-10-12", "recipeId": 9999})

		CreateMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid date is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"date": "next tuesday", "recipeId": recipe.ID})

		CreateMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateMealPlanEntry(t *testing.T) {
	t.Run("updates servings and notes", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		entry := testutil.CreateTestMealPlanEntry(env.DB, env.Household.ID, recipe.ID, apiDate(t, "2026-10-12"), 2, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 6, "notes": "guests"})
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(entry.ID)}}

		UpdateMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var got apiModel.MealPlanEntryResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Servings != 6 || got.Notes != "guests" {
			t.Errorf("entry = %+v, want 6 servings with notes", got)
		}
	})

	t.Run("unknown entry is not found", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"servings": 6})
		env.Ctx.Params = gin.Params{{Key: "id", Value: "9999"}}

		UpdateMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}

func TestDeleteMealPlanEntry(t *testing.T) {
	t.Run("deletes entry", func(t *testing.T) {
		env := setupHandlerTest(t)
		recipe := testutil.CreateTestRecipe(env.DB, env.Household.ID, env.User.ID)
		entry := testutil.CreateTestMealPlanEntry(env.DB, env.Household.ID, recipe.ID, apiDate(t, "2026-10-12"), 2, env.User.ID)
		env.Ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/mealplans/"+itoa(entry.ID), nil)
		env.Ctx.Params = gin.Params{{Key: "id", Value: itoa(entry.ID)}}

		DeleteMealPlanEntry(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var count int64
		if err := env.DB.Model(&dbModel.MealPlanEntry{}).Where("id = ?", entry.ID).Count(&count).Error; err != nil {
			t.Fatalf("count entries: %v", err)
		}
		if count != 0 {
			t.Errorf("entry rows = %d, want 0", count)
		}
	})
}

func TestGenerateMealPlanShoppingList(t *testing.T) {
	t.Run("adds missing ingredients and subtracts stock", func(t *testing.T) {
		env := setupHandlerTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = "Flour"
		product.Amount = 100
		product.Unit = "g"
		if err := env.DB.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		recipe := &dbModel.Recipe{
			HouseholdID: env.Household.ID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   env.User.ID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 250, Unit: "g", SortOrder: 0},
				{Name: "Salt", Amount: 5, Unit: "g", SortOrder: 1},
			},
		}
		if err := env.DB.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(env.DB, env.Household.ID, recipe.ID, apiDate(t, "2026-10-12"), 2, env.User.ID)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"from": "2026-10-12", "to": "2026-10-18"})

		GenerateMealPlanShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		items, err := env.AppCtx.Repos.ShoppingListItems.ListByHousehold(env.Household.ID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d, want 2", len(items))
		}
		byName := make(map[string]dbModel.ShoppingListItem)
		for i := range items {
			byName[items[i].Name] = items[i]
		}
		if flour, ok := byName["Flour"]; !ok || flour.Quantity != 150 {
			t.Errorf("flour item = %+v, want quantity 150 (250 planned - 100 in stock)", flour)
		}
		if salt, ok := byName["Salt"]; !ok || salt.Quantity != 5 {
			t.Errorf("salt item = %+v, want quantity 5", salt)
		}
	})

	t.Run("reversed range is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"from": "2026-10-18", "to": "2026-10-12"})

		GenerateMealPlanShoppingList(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}
