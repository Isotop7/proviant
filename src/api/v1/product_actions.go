// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	MsgProductNotFound               = "Product not found"
	MsgProductConcurrentModification = "Product was changed by another request, please retry"
)

// ConsumeProduct marks a product as consumed (soft-delete/archive, no product.wasted event)
// @Summary      Mark product as consumed
// @Description  Soft-deletes (archives) a product without firing a product.wasted webhook event
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/{id}/consume [post]
func ConsumeProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	if err := appCtx.Products.ConsumeProduct(productID, appCtx.UserID); err != nil {
		// A foreign id is as not-found as a missing one; only real server
		// faults belong in the 500 branch.
		if err == gorm.ErrRecordNotFound || err == errors.ErrMismatcherUserID {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		appCtx.Logger.Error().Msgf("ConsumeProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as consumed", productID)})
}

// WasteProduct marks a product as wasted (hard-delete, fires product.wasted webhook event)
// @Summary      Mark product as wasted
// @Description  Hard-deletes a product and fires the product.wasted webhook event
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/{id}/waste [post]
func WasteProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	if err := appCtx.Products.WasteProduct(productID, appCtx.UserID); err != nil {
		if err == gorm.ErrRecordNotFound || err == errors.ErrMismatcherUserID {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		if err == errors.ErrProductConcurrentModification {
			appCtx.Logger.Warn().Msgf("WasteProduct: product %d changed concurrently", productID)
			ctx.JSON(http.StatusConflict, api.APIResponse{Message: MsgProductConcurrentModification})
			return
		}
		appCtx.Logger.Error().Msgf("WasteProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as wasted", productID)})
}
