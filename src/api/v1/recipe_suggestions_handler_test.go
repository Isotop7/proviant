package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/rs/zerolog"
)

// fakeRecipeProvider implements controllers.RecipeProvider for handler tests.
type fakeRecipeProvider struct {
	recipes []controllers.Recipe
	err     error
}

func (f *fakeRecipeProvider) Name() string { return "fake" }

func (f *fakeRecipeProvider) FetchRecipes(keywords []string) ([]controllers.Recipe, bool, error) {
	return f.recipes, true, f.err
}

// setupRecipeController installs a RecipeController with the given provider.
func setupRecipeController(env *handlerTestEnv, provider controllers.RecipeProvider) {
	logger := zerolog.Nop()
	recipeCtrl := &controllers.RecipeController{
		Logger:     &logger,
		Config:     configuration.RecipeAPIConfiguration{Provider: "fake"},
		RecipeRepo: repomocks.NewMockRepositoryContainer().Recipes,
		Provider:   provider,
	}
	env.Ctx.Set("recipeController", recipeCtrl)
}

// seedExpiringProduct creates a product expiring in 3 days.
func seedExpiringProduct(t *testing.T, env *handlerTestEnv, name string) {
	t.Helper()
	product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
	product.ProductName = name
	product.ExpireAt = time.Now().Add(3 * 24 * time.Hour)
	if err := env.DB.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
}

func TestGetRecipeSuggestions(t *testing.T) {
	t.Run("user without household returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		lonely := testutil.CreateTestUser(env.DB, 0)
		env.useMember(lonely)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("invalid limit returns 400", func(t *testing.T) {
		for _, target := range []string{"?limit=abc", "?limit=0", "?limit=11"} {
			env := setupHandlerTest(t)
			env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions"+target, nil)

			GetRecipeSuggestions(env.Ctx, env.AppCtx)

			if env.W.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want 400", target, env.W.Code)
			}
		}
	})

	t.Run("no expiring products returns empty list", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp []apiModel.RecipeSuggestionResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 0 {
			t.Errorf("suggestions = %d, want 0", len(resp))
		}
	})

	t.Run("missing recipe controller returns 500", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedExpiringProduct(t, env, "Milk")
		env.Ctx.Set("recipeController", "not-a-recipe-controller")
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})

	t.Run("returns matched suggestions", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedExpiringProduct(t, env, "Milk")
		setupRecipeController(env, &fakeRecipeProvider{
			recipes: []controllers.Recipe{
				{ID: "1", Title: "Milk Soup", Ingredients: []string{"milk", "water"}},
				{ID: "2", Title: "Unrelated", Ingredients: []string{"quinoa"}},
			},
		})
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions?limit=5", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp []apiModel.RecipeSuggestionResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("suggestions = %d, want 1 (only the matching recipe)", len(resp))
		}
		if resp[0].Title != "Milk Soup" {
			t.Errorf("title = %q, want Milk Soup", resp[0].Title)
		}
		if resp[0].MatchPercent != 50 {
			t.Errorf("matchPercent = %f, want 50", resp[0].MatchPercent)
		}
		if len(resp[0].MatchedProducts) != 1 || resp[0].MatchedProducts[0] != "milk" {
			t.Errorf("matchedProducts = %v, want [milk]", resp[0].MatchedProducts)
		}
	})

	t.Run("provider error returns 500", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedExpiringProduct(t, env, "Milk")
		setupRecipeController(env, &fakeRecipeProvider{err: errors.New("provider down")})
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})

	t.Run("nil provider returns 500", func(t *testing.T) {
		env := setupHandlerTest(t)
		seedExpiringProduct(t, env, "Milk")
		setupRecipeController(env, nil)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/suggestions", nil)

		GetRecipeSuggestions(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})
}
