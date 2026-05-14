// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
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

	if err := appCtx.Products.BulkConsumeProducts(products.ProductIDs, appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msgf("BulkConsumeProducts: %s", err)
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

	if err := appCtx.Products.BulkWasteProducts(products.ProductIDs, appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msgf("BulkWasteProducts: %s", err)
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("%d products marked as wasted", len(products.ProductIDs))})
}
