package v1

import (
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	db "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/util"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
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
func GetRecipeSuggestions(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	dbHandle, ok := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(500, api.Error(errors.ErrDatabaseContextNotFound))
		return
	}

	// Get user ID from JWT
	claims := jwt.ExtractClaims(ctx)
	userID64, ok := claims[static.TokenIdentityKey].(float64)
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(400, api.Error(errors.ErrUserIDFromToken))
		return
	}
	userID := uint(userID64)

	// Get household ID for user
	userRepo := db.NewUserRepository(dbHandle)
	householdID, err := userRepo.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		ctx.JSON(400, api.Error(errors.ErrUserHasNoHousehold))
		return
	}

	// Parse limit
	limitStr := ctx.DefaultQuery("limit", "6")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 10 {
		ctx.JSON(400, api.Error(errors.ErrInvalidQueryParameter))
		return
	}

	// Get expiring products (within 7 days)
	productRepo := db.NewProductRepository(dbHandle)
	expiringProducts, err := productRepo.GetExpiringProductsByHousehold(householdID, 7)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get expiring products")
		ctx.JSON(500, api.Error(errors.ErrDatabaseOperationFailed))
		return
	}

	// Get ALL household products for missing ingredient calculation
	allProducts, err := productRepo.GetProductsByHousehold(householdID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get household products")
		ctx.JSON(500, api.Error(errors.ErrDatabaseOperationFailed))
		return
	}

	if len(expiringProducts) == 0 {
		ctx.JSON(200, []apiModel.RecipeSuggestionResponse{})
		return
	}

	// Get recipe controller from context
	recipeCtrl, ok := ctx.MustGet("recipeController").(*controllers.RecipeController)
	if !ok {
		logger.Error().Msg("recipe controller not available in context")
		ctx.JSON(500, api.Error(errors.ErrRecipeAPIUnavailable))
		return
	}

	// Get suggestions
	suggestions, err := recipeCtrl.GetSuggestions(expiringProducts, allProducts, limit)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get recipe suggestions")
		ctx.JSON(500, api.Error(errors.ErrRecipeAPIUnavailable))
		return
	}

	// Map to API response
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
