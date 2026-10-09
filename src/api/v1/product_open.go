// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	apperrors "codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OpenProductRequest is the body for POST /api/v1/products/{id}/open.
// Force=true overwrites an existing OpenedAt timestamp after confirmation.
type OpenProductRequest struct {
	Force bool `json:"force"`
}

// OpenProductResponse is returned on success (200) or on the "already opened"
// confirmation prompt (409).
type OpenProductResponse struct {
	Product     any       `json:"product"`
	OpenedAt    time.Time `json:"openedAt"`
	TriggeredBy string    `json:"triggeredBy,omitempty"`
}

// OpenProduct marks a product as opened (sets OpenedAt to now).
//
// Behaviour:
//   - First call returns 200 with the updated product.
//   - If the product is already opened and the request does NOT include
//     `force: true`, returns 409 with the previous OpenedAt and the
//     current product. The frontend shows a confirm dialog; the user
//     confirms by re-submitting with `force: true`, which overwrites
//     OpenedAt to the new "now" value.
//
// @Summary      Mark product as opened
// @Description  Sets the OpenedAt timestamp on a product. Returns 409 with the existing OpenedAt if the product is already opened, prompting the frontend to confirm. Re-submit with `force: true` to overwrite.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        id    path      int                 true   "Product ID"
// @Param        body  body      OpenProductRequest  false  "Force overwrite flag"
// @Success      200   {object}  database.Product
// @Success      409   {object}  OpenProductResponse
// @Failure      400   {object}  api.APIResponse
// @Failure      404   {object}  api.APIResponse
// @Failure      500   {object}  api.APIResponse
// @Router       /api/v1/products/{id}/open [post]
func OpenProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req OpenProductRequest
	// Body is optional. Read it first so chunked / Content-Length-less
	// requests are still parsed (gin's ShouldBindJSON returns "invalid
	// request" for a nil body and would miss chunked JSON).
	if ctx.Request.Body != nil {
		body, readErr := io.ReadAll(ctx.Request.Body)
		if readErr != nil {
			appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), readErr.Error())
			ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
				ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
				return
			}
		}
	}

	product, previous, changed, err := appCtx.Repos.Products.MarkProductOpened(productID, appCtx.UserID, time.Now(), req.Force)
	switch err {
	case nil:
		// fall through
	case gorm.ErrRecordNotFound, apperrors.ErrMismatcherUserID:
		// One body for unknown and foreign ids alike: distinct messages
		// would let a caller probe which sequential ids exist in other
		// households.
		appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, err)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(apperrors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		appCtx.Logger.Error().Msgf("Error marking product as opened: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}

	// If already opened and not forcing, return 409 so the UI can prompt.
	// `changed=false` is the authoritative conflict signal from the
	// repository (handles the concurrent-open race the old read-then-save
	// could not). The embedded product reflects the pre-save state.
	if !changed {
		conflictProduct := product
		if previous != nil {
			conflictProduct.OpenedAt = previous
		}
		var openedAt time.Time
		if previous != nil {
			openedAt = *previous
		}
		ctx.JSON(http.StatusConflict, OpenProductResponse{
			Product:     conflictProduct,
			OpenedAt:    openedAt,
			TriggeredBy: string(product.TriggerForExpireAt()),
		})
		return
	}

	ctx.JSON(http.StatusOK, product)
}
