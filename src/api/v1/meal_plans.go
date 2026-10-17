package v1

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Isotop7/proviant/api"
	apperrors "github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func parseMealPlanDate(ctx *gin.Context, logger *zerolog.Logger, raw string) (time.Time, bool) {
	date, err := apiModel.ParseMealPlanDate(raw)
	if err != nil {
		logger.Warn().Msgf("Invalid meal plan date '%s'", raw)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrMealPlanInvalidDate)
		return time.Time{}, false
	}
	return date, true
}

func mealPlanRecipeNames(appCtx *AppContext, householdID uint) map[uint]string {
	names := make(map[uint]string)
	recipes, err := appCtx.Repos.Recipes.ListByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Warn().Msgf("Error loading recipe names for meal plan: %s", err)
		return names
	}
	for i := range recipes {
		names[recipes[i].ID] = recipes[i].Name
	}
	return names
}

// ListMealPlanEntries lists the planned meals in a date range
// @Summary      Lists planned meals
// @Description  Returns all meal plan entries of the caller's household within the given date range
// @Tags         mealplans
// @Produce      json
// @Param        from query string true "Range start (YYYY-MM-DD)"
// @Param        to query string true "Range end (YYYY-MM-DD)"
// @Success      200 {array} apiModel.MealPlanEntryResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/mealplans [get]
// @Security     BearerAuth
func ListMealPlanEntries(ctx *gin.Context, appCtx *AppContext) {
	from, ok := parseMealPlanDate(ctx, appCtx.Logger, ctx.Query("from"))
	if !ok {
		return
	}
	to, ok := parseMealPlanDate(ctx, appCtx.Logger, ctx.Query("to"))
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	entries, err := appCtx.Repos.MealPlans.ListByRange(householdID, from, to)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing meal plan entries: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewMealPlanEntryResponses(entries, mealPlanRecipeNames(appCtx, householdID)))
}

// CreateMealPlanEntry schedules a recipe on one day of the meal plan
// @Summary      Creates a planned meal
// @Description  Schedules a saved recipe on the given date; servings of 0 use the recipe's own serving count
// @Tags         mealplans
// @Accept       json
// @Produce      json
// @Param        request body apiModel.CreateMealPlanEntryRequest true "Planned meal"
// @Success      201 {object} apiModel.MealPlanEntryResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/mealplans [post]
// @Security     BearerAuth
func CreateMealPlanEntry(ctx *gin.Context, appCtx *AppContext) {
	var req apiModel.CreateMealPlanEntryRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	date, ok := parseMealPlanDate(ctx, appCtx.Logger, req.Date)
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	recipe, err := appCtx.Repos.Recipes.GetByID(req.RecipeID, householdID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrRecipeNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error getting recipe: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if req.Servings <= 0 {
		req.Servings = recipe.Servings
	}

	entry := &dbModel.MealPlanEntry{
		HouseholdID: householdID,
		Date:        date,
		RecipeID:    recipe.ID,
		Servings:    req.Servings,
		Notes:       req.Notes,
		CreatedBy:   appCtx.UserID,
	}

	if err := appCtx.Repos.MealPlans.Create(entry); err != nil {
		appCtx.Logger.Error().Msgf("Error creating meal plan entry: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusCreated, apiModel.NewMealPlanEntryResponse(entry, recipe.Name))
}

// UpdateMealPlanEntry updates a planned meal
// @Summary      Updates a planned meal
// @Description  Updates date, recipe, servings and notes of a meal plan entry
// @Tags         mealplans
// @Accept       json
// @Produce      json
// @Param        id path int true "Meal plan entry ID"
// @Param        request body apiModel.UpdateMealPlanEntryRequest true "Fields to update"
// @Success      200 {object} apiModel.MealPlanEntryResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/mealplans/{id} [patch]
// @Security     BearerAuth
func UpdateMealPlanEntry(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req apiModel.UpdateMealPlanEntryRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	entry, err := appCtx.Repos.MealPlans.GetByID(id, householdID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrMealPlanEntryNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error getting meal plan entry: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if req.Date != nil {
		date, ok := parseMealPlanDate(ctx, appCtx.Logger, *req.Date)
		if !ok {
			return
		}
		entry.Date = date
	}
	if req.RecipeID != nil && *req.RecipeID != entry.RecipeID {
		recipe, err := appCtx.Repos.Recipes.GetByID(*req.RecipeID, householdID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				api.RespondError(ctx, http.StatusNotFound, apperrors.ErrRecipeNotFound)
				return
			}
			appCtx.Logger.Error().Msgf("Error getting recipe: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		entry.RecipeID = recipe.ID
	}
	if req.Servings != nil {
		entry.Servings = *req.Servings
	}
	if req.Notes != nil {
		entry.Notes = *req.Notes
	}

	if err := appCtx.Repos.MealPlans.Update(entry); err != nil {
		appCtx.Logger.Error().Msgf("Error updating meal plan entry: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewMealPlanEntryResponse(entry, mealPlanRecipeNames(appCtx, householdID)[entry.RecipeID]))
}

// DeleteMealPlanEntry removes a planned meal
// @Summary      Deletes a planned meal
// @Description  Deletes the meal plan entry with the given ID
// @Tags         mealplans
// @Produce      json
// @Param        id path int true "Meal plan entry ID"
// @Success      200 {object} api.APIResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/mealplans/{id} [delete]
// @Security     BearerAuth
func DeleteMealPlanEntry(ctx *gin.Context, appCtx *AppContext) {
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

	if err := appCtx.Repos.MealPlans.Delete(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting meal plan entry: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Meal plan entry deleted"})
}

// GenerateMealPlanShoppingList turns planned meals into shopping list entries
// @Summary      Generates shopping list from meal plan
// @Description  Aggregates the ingredients of all meals planned within the date range and creates shopping list entries for the missing amounts (stock on hand subtracted when units match)
// @Tags         mealplans
// @Accept       json
// @Produce      json
// @Param        request body apiModel.GenerateShoppingListRequest true "Date range (YYYY-MM-DD)"
// @Success      200 {object} api.APIResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/mealplans/shopping-list [post]
// @Security     BearerAuth
func GenerateMealPlanShoppingList(ctx *gin.Context, appCtx *AppContext) {
	var req apiModel.GenerateShoppingListRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	from, ok := parseMealPlanDate(ctx, appCtx.Logger, req.From)
	if !ok {
		return
	}
	to, ok := parseMealPlanDate(ctx, appCtx.Logger, req.To)
	if !ok {
		return
	}
	if to.Before(from) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrMealPlanInvalidRange)
		return
	}

	mealPlanService := services.NewMealPlanService(appCtx.Repos, appCtx.Logger)
	created, err := mealPlanService.GenerateShoppingList(from, to, appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error generating shopping list from meal plan: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: strconv.Itoa(created) + " items added"})
}
