package v1

import (
	"strconv"

	stderrors "errors"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	db "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
)

// GetRecipeSuggestions returns recipe suggestions based on expiring products
// @Summary      Recipe suggestions
// @Description  Returns up to 6 recipe suggestions matching products expiring within 7 days. Results are ranked by expiry proximity first, then match percentage. Set refresh=1 to bypass the suggestion cache.
// @Tags         recipes
// @Produce      json
// @Param        limit    query  int   false  "Number of suggestions (default 6, max 10)"
// @Param        refresh  query  int   false  "Set to 1 to bypass the cache and fetch fresh suggestions"
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

	refresh := ctx.Query("refresh") == "1"

	productRepo := db.NewProductRepository(appCtx.DB)
	// The window and the ranking horizon are the same number: a product further
	// out than the horizon scores 0 against every other out-of-window product, so
	// widening one without the other would rank those products arbitrarily.
	expiringProducts, err := productRepo.GetExpiringProductsByHousehold(householdID, dbModel.ExpiryRankingHorizonDays)
	if err != nil {
		appCtx.Logger.Error().Err(err).Msg("failed to get expiring products")
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

	// The full household product list is only needed to match fresh
	// suggestions; on a cache hit only the cheap fingerprint query runs.
	productSource := controllers.ProductSource{
		All: func() ([]dbModel.Product, error) {
			// Narrow projection: the matcher reads only ID, ProductName and
			// Categories, so loading every column of every household product
			// here is pure waste on the cache-miss path.
			return productRepo.GetMatchableProductsByHousehold(householdID)
		},
		Fingerprint: func() (string, error) {
			return productRepo.GetProductSetFingerprint(householdID)
		},
	}

	suggestions, err := recipeCtrl.GetSuggestions(expiringProducts, productSource, limit, refresh)
	if err != nil {
		// GetSuggestions has exactly two failure modes: a database error from
		// loading the household products, and ErrRecipeAPIUnavailable, which it
		// returns for every provider-side failure including a nil provider.
		if stderrors.Is(err, errors.ErrRecipeAPIUnavailable) {
			appCtx.Logger.Error().Err(err).Msg("failed to fetch recipe suggestions")
			api.RespondError(ctx, 500, errors.ErrRecipeAPIUnavailable)
		} else {
			appCtx.Logger.Error().Err(err).Msg("failed to get household products for recipe suggestions")
			api.RespondError(ctx, 500, errors.ErrDatabaseOperationFailed)
		}
		return
	}

	response := make([]apiModel.RecipeSuggestionResponse, len(suggestions))
	for i := range suggestions {
		response[i] = suggestions[i].ToAPIResponse()
	}

	ctx.JSON(200, response)
}
