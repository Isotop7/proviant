// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

// BulkConsumeProducts marks multiple products as consumed (soft-delete, no product.wasted event)
// @Summary      Mark multiple products as consumed
// @Description  Soft-deletes (archives) multiple products without firing product.wasted webhook events. Reports the ids that were not touched: 404 when none could be consumed (unknown or foreign ids), 200 with the failed ids in failedIds when only some fail, 500 (also carrying failedIds) when a failure was server-side.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        productIDs  body  []int  true  "Product IDs"
// @Success      200  {object}  api.BulkActionResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.BulkActionResponse
// @Failure      500  {object}  api.BulkActionResponse
// @Router       /api/v1/products/bulkConsume [post]
func BulkConsumeProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	ids := dedupProductIDs(products.ProductIDs)
	failed, err := appCtx.Products.BulkConsumeProducts(ids, appCtx.UserID)
	writeBulkActionResult(ctx, appCtx, ids, failed, err, "marked as consumed")
}

// BulkWasteProducts marks multiple products as wasted (hard-delete, fires product.wasted webhook per product)
// @Summary      Mark multiple products as wasted
// @Description  Hard-deletes multiple products and fires the product.wasted webhook per product. Reports the ids that were not touched: 404 when none could be wasted (unknown or foreign ids), 200 with the failed ids in failedIds when only some fail, 500 (also carrying failedIds) when a failure was server-side.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        productIDs  body  []int  true  "Product IDs"
// @Success      200  {object}  api.BulkActionResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.BulkActionResponse
// @Failure      500  {object}  api.BulkActionResponse
// @Router       /api/v1/products/bulkWaste [post]
func BulkWasteProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	ids := dedupProductIDs(products.ProductIDs)
	failed, err := appCtx.Products.BulkWasteProducts(ids, appCtx.UserID)
	writeBulkActionResult(ctx, appCtx, ids, failed, err, "marked as wasted")
}

// CookProducts consumes multiple products, partially or fully (cook workflow)
// @Summary      Cook products
// @Description  Consumes the given products by reducing their amounts. An amount of 0 or one that reaches or exceeds the current amount fully consumes (archives) the product.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        request  body  apiModel.CookProductsAPIModel  true  "Cook items"
// @Success      200  {object}  apiModel.CookResponse
// @Failure      400  {object}  apiModel.CookResponse  "Returned when no item was processed (invalid body or all items failed)"
// @Failure      500  {object}  api.APIResponse  "Returned when no item was processed and a server-side error occurred"
// @Router       /api/v1/products/cook [post]
func CookProducts(ctx *gin.Context, appCtx *AppContext) {
	var cookRequest apiModel.CookProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &cookRequest) {
		return
	}

	consumed, partial, errs, internalErr := appCtx.Products.CookProducts(cookRequest.Items, appCtx.UserID)

	if consumed == 0 && partial == 0 {
		if internalErr != nil {
			if len(errs) > 0 {
				appCtx.Logger.Warn().Msgf("CookProducts: %d item errors alongside internal error: %s", len(errs), strings.Join(errs, "; "))
			}
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		if len(errs) > 0 {
			ctx.JSON(http.StatusBadRequest, apiModel.CookResponse{Errors: errs})
			return
		}
	}

	ctx.JSON(http.StatusOK, apiModel.CookResponse{Consumed: consumed, Partial: partial, Errors: errs})
}
