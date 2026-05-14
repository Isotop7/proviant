// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func formatProductIDs(ids []uint) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ";")
}

// GetArchivedProducts returns the archived products of a user
// @Summary      Return a list of archived products
// @Description  Return a list of archived products of user
// @Tags         product
// @Produce      json
// @Success      200  {object}  []database.Product
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/archived [get]
func GetArchivedProducts(ctx *gin.Context, appCtx *AppContext) {
	q, ok := ParseProductListQuery(ctx)
	if !ok {
		return
	}
	limit := q.Limit

	products, productBulkErr := appCtx.Repos.Products.GetUserArchivedProductsBulk(appCtx.UserID, limit)
	if productBulkErr != nil {
		appCtx.Logger.Warn().Msgf("Error getting products of user: %s", productBulkErr)
		if productBulkErr == errors.ErrInvalidUserData || productBulkErr == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{
				Message: "Unable to retrieve archived products. Please check your account.",
				Action:  "Ensure you are logged in with a valid household",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
		}
		return
	}
	ctx.JSON(http.StatusOK, products)
}

// RestoreProduct restores an archived product of a user
// @Summary      	Restores a product
// @Description  	Restores an archived product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id}/restore [post]
func RestoreProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	if err := appCtx.Products.RestoreProduct(productID, appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msgf("Error restoring product: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.RestoreFailedError())
		return
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was restored", productID)})
}

// BulkRestoreProducts restores a list of products of a user
// @Summary      	Restores a list of product
// @Description  	Restores a list of product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			productIDs	body	[]int				true	"Product IDs"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/bulkRestore [post]
func BulkRestoreProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	if err := appCtx.Products.BulkRestoreProducts(products.ProductIDs, appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were restored", formatProductIDs(products.ProductIDs))})
}
