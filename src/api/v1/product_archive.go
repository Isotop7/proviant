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

// dedupProductIDs keeps the first occurrence of each id. A repeat would be
// attempted and reported twice, skewing the failed/success accounting below —
// the shared lookup is Unscoped (DeleteProduct needs active rows too), so a
// repeated id succeeds again instead of failing on the second attempt.
func dedupProductIDs(productIDs []uint) []uint {
	seen := make(map[uint]bool, len(productIDs))
	ids := make([]uint, 0, len(productIDs))
	for _, productID := range productIDs {
		if !seen[productID] {
			seen[productID] = true
			ids = append(ids, productID)
		}
	}
	return ids
}

// writeBulkActionResult maps one bulk action's outcome onto its response:
// 500 when a failure was server-side, 404 when every id failed on the
// caller's side, 200 otherwise — and a partial success is never reported as
// a blanket success, so the client always learns which ids did not land.
// failedIds rides on every non-trivial body; verb phrases the result, e.g.
// "restored" or "marked as consumed".
func writeBulkActionResult(ctx *gin.Context, appCtx *AppContext, ids, failed []uint, serverErr error, verb string) {
	if serverErr != nil {
		// Partial progress still happened, so the failed ids ride along with
		// the 500: the client must be able to tell what did not land instead
		// of retrying (and re-running the action) blindly.
		appCtx.Logger.Error().Msg(serverErr.Error())
		ctx.JSON(http.StatusInternalServerError, api.BulkActionResponse{
			APIResponse: api.InternalError(),
			FailedIDs:   failed,
		})
		return
	}
	allFailed := len(ids) > 0 && len(failed) == len(ids)
	switch {
	case allFailed:
		ctx.JSON(http.StatusNotFound, api.BulkActionResponse{
			APIResponse: api.APIResponse{
				Message: fmt.Sprintf("Products with ID '%s' were not %s (not found or not owned)", formatProductIDs(failed), verb),
			},
			FailedIDs: failed,
		})
	case len(failed) > 0:
		ctx.JSON(http.StatusOK, api.BulkActionResponse{
			APIResponse: api.APIResponse{
				Message: fmt.Sprintf("Products with ID '%s' were %s, except '%s'", formatProductIDs(ids), verb, formatProductIDs(failed)),
			},
			FailedIDs: failed,
		})
	default:
		ctx.JSON(http.StatusOK, api.BulkActionResponse{
			APIResponse: api.APIResponse{
				Message: fmt.Sprintf("Products with ID '%s' were %s", formatProductIDs(ids), verb),
			},
		})
	}
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
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id}/restore [post]
func RestoreProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	if err := appCtx.Products.RestoreProduct(productID, appCtx.UserID); err != nil {
		// Not-found and cross-tenant ids are 404s, not server faults.
		if err == gorm.ErrRecordNotFound || err == errors.ErrMismatcherUserID {
			appCtx.Logger.Warn().Msgf("Product with ID '%d' not found or not owned: %s", productID, err)
			ctx.JSON(http.StatusNotFound, api.APIResponse{
				Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
				Action:  MsgCheckProductIdTryAgain,
			})
			return
		}
		appCtx.Logger.Error().Msgf("Error restoring product: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.RestoreFailedError())
		return
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was restored", productID)})
}

// BulkRestoreProducts restores a list of products of a user
// @Summary      	Restores a list of product
// @Description  	Restores a list of product of a user. Reports the ids that were not restored: 404 when none could be restored (unknown or foreign ids), 200 with the failed ids in failedIds when only some fail, 500 (also carrying failedIds) when a failure was server-side.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			productIDs	body	[]int				true	"Product IDs"
// @Success      	200  {object}  api.BulkActionResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.BulkActionResponse
// @Failure      	500  {object}  api.BulkActionResponse
// @Router       	/api/v1/products/bulkRestore [post]
func BulkRestoreProducts(ctx *gin.Context, appCtx *AppContext) {
	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, appCtx.Logger, &products) {
		return
	}

	ids := dedupProductIDs(products.ProductIDs)
	failed, err := appCtx.Products.BulkRestoreProducts(ids, appCtx.UserID)
	writeBulkActionResult(ctx, appCtx, ids, failed, err, "restored")
}
