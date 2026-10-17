package v1

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Isotop7/proviant/api"
	apperrors "github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func recipeIngredientInputsToModels(inputs []apiModel.RecipeIngredientInput) []dbModel.RecipeIngredient {
	ingredients := make([]dbModel.RecipeIngredient, len(inputs))
	for i := range inputs {
		ingredients[i] = dbModel.RecipeIngredient{
			ProductID: inputs[i].ProductID,
			Name:      strings.TrimSpace(inputs[i].Name),
			Amount:    inputs[i].Amount,
			Unit:      inputs[i].Unit,
			SortOrder: inputs[i].SortOrder,
		}
	}
	return ingredients
}

// ListRecipes lists the saved recipes of the caller's household
// @Summary      Lists saved recipes
// @Description  Returns all saved recipes of the caller's household
// @Tags         recipes
// @Produce      json
// @Success      200 {array} apiModel.RecipeResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/recipes [get]
// @Security     BearerAuth
func ListRecipes(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	recipes, err := appCtx.Repos.Recipes.ListByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing recipes: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewRecipeResponses(recipes))
}

// CreateRecipe creates a saved recipe for the caller's household
// @Summary      Creates a saved recipe
// @Description  Creates a saved recipe with its ingredients; a nil productId marks a free-text ingredient
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        request body apiModel.CreateRecipeRequest true "Recipe with ingredients"
// @Success      201 {object} apiModel.RecipeResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/recipes [post]
// @Security     BearerAuth
func CreateRecipe(ctx *gin.Context, appCtx *AppContext) {
	var req apiModel.CreateRecipeRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrRecipeNameRequired)
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if req.Servings <= 0 {
		req.Servings = 1
	}

	recipe := &dbModel.Recipe{
		HouseholdID:  householdID,
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		Instructions: req.Instructions,
		Servings:     req.Servings,
		SourceURL:    req.SourceURL,
		ImageURL:     req.ImageURL,
		CreatedBy:    appCtx.UserID,
		Ingredients:  recipeIngredientInputsToModels(req.Ingredients),
	}

	if err := appCtx.Repos.Recipes.Create(recipe); err != nil {
		appCtx.Logger.Error().Msgf("Error creating recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusCreated, apiModel.NewRecipeResponse(recipe))
}

// GetRecipe returns one saved recipe with its ingredients
// @Summary      Gets a saved recipe
// @Description  Returns the saved recipe with the given ID including its ingredients
// @Tags         recipes
// @Produce      json
// @Param        id path int true "Recipe ID"
// @Success      200 {object} apiModel.RecipeResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/recipes/{id} [get]
// @Security     BearerAuth
func GetRecipe(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	recipe, err := appCtx.Repos.Recipes.GetByID(id, householdID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrRecipeNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error getting recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewRecipeResponse(recipe))
}

// UpdateRecipe updates a saved recipe and replaces its ingredients
// @Summary      Updates a saved recipe
// @Description  Updates the given fields of a saved recipe; supplying ingredients replaces them atomically
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        id path int true "Recipe ID"
// @Param        request body apiModel.UpdateRecipeRequest true "Fields to update"
// @Success      200 {object} apiModel.RecipeResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/recipes/{id} [patch]
// @Security     BearerAuth
func UpdateRecipe(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req apiModel.UpdateRecipeRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	recipe, err := appCtx.Repos.Recipes.GetByID(id, householdID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrRecipeNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error getting recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		recipe.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		recipe.Description = *req.Description
	}
	if req.Instructions != nil {
		recipe.Instructions = *req.Instructions
	}
	if req.Servings != nil {
		recipe.Servings = *req.Servings
	}
	if req.SourceURL != nil {
		recipe.SourceURL = *req.SourceURL
	}
	if req.ImageURL != nil {
		recipe.ImageURL = *req.ImageURL
	}
	if req.Ingredients != nil {
		recipe.Ingredients = recipeIngredientInputsToModels(*req.Ingredients)
	}

	if err := appCtx.Repos.Recipes.Update(recipe); err != nil {
		appCtx.Logger.Error().Msgf("Error updating recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewRecipeResponse(recipe))
}

// DeleteRecipe deletes a saved recipe and its ingredients
// @Summary      Deletes a saved recipe
// @Description  Deletes the saved recipe with the given ID including its ingredients
// @Tags         recipes
// @Produce      json
// @Param        id path int true "Recipe ID"
// @Success      200 {object} api.APIResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/recipes/{id} [delete]
// @Security     BearerAuth
func DeleteRecipe(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := appCtx.Repos.Recipes.Delete(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Recipe deleted"})
}

// CookRecipe consumes the ingredients of a saved recipe (cook workflow)
// @Summary      Cook a saved recipe
// @Description  Consumes the recipe's linked products by their scaled amounts. Unlinked ingredients and unit mismatches are reported as errors and skipped; an amount reaching the current stock fully consumes (archives) the product.
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        id path int true "Recipe ID"
// @Param        request body apiModel.CookRecipeRequest true "Servings to cook (0 = recipe servings)"
// @Success      200 {object} apiModel.CookResponse
// @Failure      400 {object} apiModel.CookResponse "Returned when no item was processed (invalid body or all ingredients failed)"
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse "Returned when no item was processed and a server-side error occurred"
// @Router       /api/v1/recipes/{id}/cook [post]
// @Security     BearerAuth
func CookRecipe(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req apiModel.CookRecipeRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	recipeService := services.NewRecipeService(appCtx.Repos, appCtx.Logger)
	consumed, partial, errs, internalErr := recipeService.CookRecipe(id, req.Servings, appCtx.UserID)

	if internalErr != nil {
		if errors.Is(internalErr, gorm.ErrRecordNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrRecipeNotFound)
			return
		}
		if len(errs) > 0 {
			appCtx.Logger.Warn().Msgf("CookRecipe: %d ingredient errors alongside internal error: %s", len(errs), strings.Join(errs, "; "))
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if consumed == 0 && partial == 0 {
		if len(errs) > 0 {
			ctx.JSON(http.StatusBadRequest, apiModel.CookResponse{Errors: errs})
			return
		}
	}

	ctx.JSON(http.StatusOK, apiModel.CookResponse{Consumed: consumed, Partial: partial, Errors: errs})
}
