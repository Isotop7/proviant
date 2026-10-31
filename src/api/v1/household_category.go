// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"
	"net/url"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
)

// householdCategoryRequest is what a household changes about a canonical
// category. A field the request did not carry leaves the stored value alone;
// a shelf life of zero clears the stored one.
type householdCategoryRequest struct {
	DisplayName             *string `json:"displayName"`
	SortOrder               *int    `json:"sortOrder"`
	Hidden                  *bool   `json:"hidden"`
	DefaultShelfLifeDays    *int    `json:"defaultShelfLifeDays"`
	DefaultDaysAfterOpening *int    `json:"defaultDaysAfterOpening"`
}

// householdCategoryOrderRequest is the household's order over its categories.
type householdCategoryOrderRequest struct {
	Keys []string `json:"keys"`
}

// ListHouseholdCategories returns the household's category list: every
// canonical category with the household's overrides applied.
// @Summary      List household categories
// @Description  Returns every canonical category with the household's display name, order, visibility and shelf-life defaults applied.
// @Tags         household
// @Produce      json
// @Success      200  {array}   apiModel.HouseholdCategoryResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/categories [get]
func ListHouseholdCategories(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := householdIDFor(ctx, appCtx)
	if !ok {
		return
	}

	categories, err := appCtx.Repos.HouseholdCategories.List(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing household categories: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdCategoryResponses(categories))
}

// UpdateHouseholdCategory stores the household's override of one canonical
// category.
// @Summary      Update a household category
// @Description  Stores the household's display name, order, visibility and shelf-life defaults for one canonical category. A shelf life of zero clears the stored value.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        categoryKey  path      string                      true  "Canonical category key"
// @Param        body         body      householdCategoryRequest   true  "Category overrides"
// @Success      200          {object}  apiModel.HouseholdCategoryResponse
// @Failure      400          {object}  api.APIResponse
// @Failure      404          {object}  api.APIResponse
// @Failure      500          {object}  api.APIResponse
// @Router       /api/v1/household/categories/{categoryKey} [patch]
func UpdateHouseholdCategory(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := householdIDFor(ctx, appCtx)
	if !ok {
		return
	}

	categoryKey, decodeErr := url.PathUnescape(ctx.Param("categoryKey"))
	if decodeErr != nil || categoryKey == "" {
		appCtx.Logger.Warn().Msgf("Invalid category key '%s'", ctx.Param("categoryKey"))
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrCategoryNotFound)
		return
	}

	var req householdCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf("%s: %s", errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	category, err := appCtx.Repos.HouseholdCategories.Upsert(householdID, categoryKey, database.HouseholdCategoryOverride{
		DisplayName:             req.DisplayName,
		SortOrder:               req.SortOrder,
		Hidden:                  req.Hidden,
		DefaultShelfLifeDays:    req.DefaultShelfLifeDays,
		DefaultDaysAfterOpening: req.DefaultDaysAfterOpening,
	})
	if err != nil {
		respondCategoryError(ctx, appCtx, "updating", err)
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdCategoryResponse(&category))
}

// ReorderHouseholdCategories writes the household's order over its
// categories.
// @Summary      Reorder household categories
// @Description  Stores the household's order over the canonical categories. The listed keys are written in the order they are given.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        body  body      householdCategoryOrderRequest  true  "Category order"
// @Success      200   {object}  api.APIResponse
// @Failure      400   {object}  api.APIResponse
// @Failure      404   {object}  api.APIResponse
// @Failure      500   {object}  api.APIResponse
// @Router       /api/v1/household/categories/order [put]
func ReorderHouseholdCategories(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := householdIDFor(ctx, appCtx)
	if !ok {
		return
	}

	var req householdCategoryOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf("%s: %s", errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	if err := appCtx.Repos.HouseholdCategories.Reorder(householdID, req.Keys); err != nil {
		respondCategoryError(ctx, appCtx, "reordering", err)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Category order saved"})
}

// applyHouseholdCategory resolves the product's canonical category and fills
// the shelf life the household gave that category when the request carried
// none. A date the client sent is never overwritten — the user may have
// corrected the suggestion.
//
// It answers false when the request has to be refused, which it has already
// reported by then.
func applyHouseholdCategory(ctx *gin.Context, appCtx *AppContext, product *dbModel.Product) bool {
	resolver, err := appCtx.Repos.Savings.LoadCategoryResolver()
	if err != nil {
		appCtx.Logger.Error().Msgf("Could not load the canonical categories: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return false
	}

	categoryKey, categoryErr := database.CanonicalCategoryKey(resolver, product.CategoryKey)
	if categoryErr != nil {
		api.RespondError(ctx, http.StatusBadRequest, categoryErr)
		return false
	}
	if categoryKey == nil {
		if resolved, ok := resolver.ResolveKey(product.Categories); ok {
			categoryKey = &resolved
		}
	}
	product.CategoryKey = categoryKey

	if categoryKey == nil {
		return true
	}

	defaults, defaultsErr := householdCategoryDefaults(appCtx)
	if defaultsErr != nil {
		appCtx.Logger.Warn().Msgf("Could not load the household category defaults: %s", defaultsErr)
		return true
	}
	database.ApplyHouseholdShelfLife(product, defaults, categoryKey)
	return true
}

// batchCategories is the category context of one request: the canonical
// categories the products are checked against and the shelf-life defaults the
// household gave them. A request that handles many products reads it once.
type batchCategories struct {
	resolver *database.CategoryResolver
	defaults map[string]database.HouseholdCategoryDefaults
}

// loadBatchCategories reads the category context of the caller's household.
func loadBatchCategories(appCtx *AppContext) batchCategories {
	batch := batchCategories{}
	resolver, err := appCtx.Repos.Savings.LoadCategoryResolver()
	if err != nil {
		appCtx.Logger.Warn().Msgf("Could not load the canonical categories: %s", err)
	} else {
		batch.resolver = resolver
	}
	defaults, defaultsErr := householdCategoryDefaults(appCtx)
	if defaultsErr != nil {
		appCtx.Logger.Warn().Msgf("Could not load the household category defaults: %s", defaultsErr)
		return batch
	}
	batch.defaults = defaults
	return batch
}

// householdCategoryDefaults reads the shelf lives the household gave its
// categories. A caller without a household has no defaults.
func householdCategoryDefaults(appCtx *AppContext) (map[string]database.HouseholdCategoryDefaults, error) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		return nil, err
	}
	if householdID == 0 {
		return nil, nil
	}
	return appCtx.Repos.HouseholdCategories.Defaults(householdID)
}

// householdIDFor resolves the household of the calling user.
func householdIDFor(ctx *gin.Context, appCtx *AppContext) (uint, bool) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		appCtx.Logger.Error().Msgf("Could not resolve the household of user %d: %v", appCtx.UserID, err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return 0, false
	}
	return householdID, true
}

// respondCategoryError maps a repository error onto its status code.
func respondCategoryError(ctx *gin.Context, appCtx *AppContext, action string, err error) {
	switch err {
	case errors.ErrCategoryNotFound:
		api.RespondError(ctx, http.StatusNotFound, err)
		return
	case errors.ErrCategoryShelfLifeOutOfRange, errors.ErrCategoryDaysAfterOpeningOutOfRange:
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	default:
		appCtx.Logger.Error().Msgf("Error %s household categories: %s", action, err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
}
