// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BulkConsumeProducts marks multiple products as consumed (soft-delete, no product.wasted event)
// @Summary      Mark products as consumed
// @Description  Soft-deletes (archives) multiple products without firing product.wasted webhook events
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        productIDs  body  []int  true  "Product IDs"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/bulkConsume [post]
func BulkConsumeProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	for _, productID := range products.ProductIDs {
		product, err := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
		if err != nil {
			continue
		}

		if consumeErr := appCtx.Repos.Products.ConsumeProduct(productID, appCtx.UserID); consumeErr != nil {
			if consumeErr == gorm.ErrRecordNotFound {
				appCtx.Logger.Warn().Msgf("BulkConsumeProducts: product %d not found", productID)
			} else {
				appCtx.Logger.Error().Msgf("BulkConsumeProducts: %s", consumeErr)
			}
			continue
		}

		go recordHouseholdSavingsEvent(appCtx, &product, "consumed")
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("%d products marked as consumed", len(products.ProductIDs))})
}

// BulkWasteProducts marks multiple products as wasted (hard-delete, fires product.wasted webhook per product)
// @Summary      Mark products as wasted
// @Description  Hard-deletes multiple products and fires the product.wasted webhook event per product
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        productIDs  body  []int  true  "Product IDs"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/bulkWaste [post]
func BulkWasteProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	householdID, householdErr := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)

	for _, productID := range products.ProductIDs {
		product, err := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
		if err != nil {
			continue
		}

		if wasteErr := appCtx.Repos.Products.WasteProduct(productID, appCtx.UserID); wasteErr != nil {
			if wasteErr == gorm.ErrRecordNotFound {
				appCtx.Logger.Warn().Msgf("BulkWasteProducts: product %d not found", productID)
			} else {
				appCtx.Logger.Error().Msgf("BulkWasteProducts: %s", wasteErr)
			}
			continue
		}

		if householdErr == nil && householdID > 0 {
			if err := appCtx.Repos.Streaks.RecordWasteEvent(householdID); err != nil {
				appCtx.Logger.Error().Msgf("BulkWasteProducts: failed to record waste event for streak: %s", err)
			}
		}

		go func(pid uint) {
			if ws := controllers.GetWebhookService(); ws != nil {
				ws.FireEvent("product.wasted", map[string]any{
					"productId": pid,
				})
			}
		}(productID)

		go recordHouseholdSavingsEvent(appCtx, &product, "wasted")
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("%d products marked as wasted", len(products.ProductIDs))})
}
