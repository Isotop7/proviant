// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	apperrors "codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// GetConsumptionRate returns the estimated weekly consumption rate for a product,
// based on the last 90 days of consumed (archived) products with the same barcode
// (or product name as fallback). Requires at least 2 samples spread over at
// least 7 days.
// @Summary      Get consumption rate estimate
// @Description  Returns the household's average weekly consumption for a product
// @Description  based on archived consumed samples within the last 90 days.
// @Description  Requires at least 2 samples spanning at least 7 days; otherwise
// @Description  HasEstimate is false.
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  apiModel.ConsumptionRateResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/{id}/consumption-rate [get]
func GetConsumptionRate(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	product, err := appCtx.Repos.Products.GetProductIdentity(productID, appCtx.UserID)
	if err != nil {
		respondProductNotFound(ctx, appCtx, productID)
		return
	}

	if product.HouseholdID == 0 {
		ctx.JSON(http.StatusOK, apiModel.ConsumptionRateResponse{
			ProductID:   productID,
			HasEstimate: false,
		})
		return
	}

	rate, calcErr := appCtx.Consumption.ComputeConsumptionRate(product.HouseholdID, appCtx.UserID, product.Barcode, product.ProductName)
	if calcErr != nil {
		appCtx.Logger.Error().Msgf("ComputeConsumptionRate: %s", calcErr)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.ConsumptionRateResponse{
		ProductID:    productID,
		HasEstimate:  rate.HasEstimate,
		PerWeek:      rate.PerWeek,
		Unit:         rate.Unit,
		SampleCount:  rate.SampleCount,
		DaysCovered:  rate.DaysCovered,
		LastConsumed: rate.LastConsumed,
		Display:      rate.Display,
	})
}

// GetRestockSuggestion returns a suggested restock quantity for a product,
// preferring the consumption rate (when enough history exists) and falling
// back to the min-stock deficit. Returns "none" when no real deficit exists.
// @Summary      Get restock quantity suggestion
// @Description  Returns a suggested quantity to add to the shopping list for a
// @Description  product. Source is "consumption_rate" when at least 2 consumed
// @Description  samples spanning at least 7 days exist within the last 90 days,
// @Description  otherwise "min_stock" (minStockAmount - currentAmount) when
// @Description  the current amount is below the minimum, or "none".
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  apiModel.RestockSuggestionResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/{id}/restock-suggestion [get]
func GetRestockSuggestion(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	product, err := appCtx.Repos.Products.GetProductIdentity(productID, appCtx.UserID)
	if err != nil {
		respondProductNotFound(ctx, appCtx, productID)
		return
	}

	if product.HouseholdID == 0 {
		ctx.JSON(http.StatusOK, apiModel.RestockSuggestionResponse{
			ProductID:     productID,
			ProductName:   product.ProductName,
			HasSuggestion: false,
			Source:        util.ConsumptionSourceNone,
		})
		return
	}

	suggestion := appCtx.Consumption.ComputeRestockSuggestionFromProduct(product.HouseholdID, appCtx.UserID, &product)

	ctx.JSON(http.StatusOK, apiModel.RestockSuggestionResponse{
		ProductID:     productID,
		ProductName:   suggestion.ProductName,
		HasSuggestion: suggestion.Source != util.ConsumptionSourceNone,
		HasEstimate:   suggestion.HasEstimate,
		SuggestedQty:  suggestion.SuggestedQty,
		Unit:          suggestion.Unit,
		Source:        suggestion.Source,
		WeeklyRate:    suggestion.WeeklyRate,
		SampleCount:   suggestion.SampleCount,
		Display:        suggestion.Display,
		PerWeekDisplay: suggestion.PerWeekDisplay,
	})
}

func respondProductNotFound(ctx *gin.Context, appCtx *AppContext, productID uint) {
	appCtx.Logger.Warn().Msgf(apperrors.FormatProductNotFound, productID)
	ctx.JSON(http.StatusNotFound, api.APIResponse{
		Message: fmt.Sprintf(apperrors.FormatProductWithIDNotFound, productID),
		Action:  MsgCheckProductIdTryAgain,
	})
}
