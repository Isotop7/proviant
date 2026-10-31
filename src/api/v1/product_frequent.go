// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"github.com/Isotop7/proviant/api"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// GetFrequentlyBoughtProducts returns the staples the household buys
// repeatedly.
// @Summary      Return frequently bought products
// @Description  Returns the archived products the household bought at least twice, most frequent first and most recent second. Every entry carries the name, the pack size, the category and the storage location the household itself used, so a suggestion fills the add-product form with what this household actually buys. Another member's private product is never part of the list and never counts towards a barcode the household shares. The list is empty when the household has no such product.
// @Tags         product
// @Produce      json
// @Success      200  {object}  []apiModel.FrequentProduct
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/frequently-bought [get]
func GetFrequentlyBoughtProducts(ctx *gin.Context, appCtx *AppContext) {
	products, err := appCtx.Repos.Products.GetFrequentlyBoughtProducts(appCtx.UserID, util.FrequentProductLimit)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetFrequentlyBoughtProducts: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if products == nil {
		products = []apiModel.FrequentProduct{}
	}
	ctx.JSON(http.StatusOK, products)
}
