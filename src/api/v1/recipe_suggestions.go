package v1

import (
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	db "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

// GetRecipeSuggestions returns recipe suggestions based on expiring products
// @Summary      Recipe suggestions
// @Description  Returns up to 6 recipe suggestions matching products expiring within 7 days
// @Tags         recipes
// @Produce      json
// @Param        limit  query  int  false  "Number of suggestions (default 6, max 10)"
// @Success      200  {array}  apiModel.RecipeSuggestionResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/recipes/suggestions [get]
func GetRecipeSuggestions(ctx *gin.Context, appCtx *AppContext) {
	userRepo := db.NewUserRepository(appCtx.DB)
	householdID, err := userRepo.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		api.RespondError(ctx, 400, errors.ErrUserHasNoHousehold)
		return
	}

	limitStr := ctx.DefaultQuery("limit", "6")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 10 {
		api.RespondError(ctx, 400, errors.ErrInvalidQueryParameter)
		return
	}

	productRepo := db.NewProductRepository(appCtx.DB)
	expiringProducts, err := productRepo.GetExpiringProductsByHousehold(householdID, 7)
	if err != nil {
		appCtx.Logger.Error().Err(err).Msg("failed to get expiring products")
		api.RespondError(ctx, 500, errors.ErrDatabaseOperationFailed)
		return
	}

	allProducts, err := productRepo.GetProductsByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Err(err).Msg("failed to get household products")
		api.RespondError(ctx, 500, errors.ErrDatabaseOperationFailed)
		return
	}

	if len(expiringProducts) == 0 {
		ctx.JSON(200, []apiModel.RecipeSuggestionResponse{})
		return
	}

	recipeCtrl, ok := ctx.MustGet("recipeController").(*controllers.RecipeController)
	if !ok {
		appCtx.Logger.Error().Msg("recipe controller not available in context")
		api.RespondError(ctx, 500, errors.ErrRecipeAPIUnavailable)
		return
	}

	suggestions, err := recipeCtrl.GetSuggestions(expiringProducts, allProducts, limit)
	if err != nil {
		appCtx.Logger.Error().Err(err).Msg("failed to get recipe suggestions")
		api.RespondError(ctx, 500, errors.ErrRecipeAPIUnavailable)
		return
	}

	response := make([]apiModel.RecipeSuggestionResponse, len(suggestions))
	for i := range suggestions {
		s := &suggestions[i]
		response[i] = apiModel.RecipeSuggestionResponse{
			ID:               s.ID,
			Title:            s.Title,
			ImageURL:         s.ImageURL,
			SourceURL:        s.SourceURL,
			Ingredients:      s.Ingredients,
			MatchedProducts:  s.MatchedProducts,
			MissingCount:     s.MissingCount,
			TotalIngredients: s.TotalIngredients,
			MatchPercent:     s.MatchPercent,
		}
	}

	ctx.JSON(200, response)
}
