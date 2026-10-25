// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const (
	MsgProductNotFound               = "Product not found"
	MsgProductConcurrentModification = "Product was changed by another request, please retry"
)

// maxProductActionBodyBytes bounds the body of a consume or waste action. The
// body carries a pack count and a mutation id, so a few hundred bytes are
// enough; an uncapped body would let a client make the server buffer an
// arbitrary amount of memory before the request is refused.
const maxProductActionBodyBytes = 4 << 10

func parseProductActionBody(ctx *gin.Context, logger *zerolog.Logger) (int, *string, bool) {
	var request apiModel.ProductActionDTO
	// An empty body means the whole stock; a present zero is invalid instead.
	if ctx.Request != nil && ctx.Request.Body != nil {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxProductActionBodyBytes)
		body, readErr := io.ReadAll(ctx.Request.Body)
		if readErr != nil {
			logger.Warn().Msgf("%s: %s", errors.ErrParseBody, readErr)
			ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
			return 0, nil, false
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &request); err != nil {
				logger.Warn().Msgf("%s: %s", errors.ErrParseBody, err)
				ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
				return 0, nil, false
			}
		}
	}

	quantity := 0
	if request.Quantity != nil {
		if *request.Quantity < 1 {
			logger.Warn().Msgf("Product action quantity must be positive, got %d", *request.Quantity)
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidProductQuantity)
			return 0, nil, false
		}
		// A count above the maximum pack count cannot be meant literally: the
		// removal clamps to the stock that is left, so the request would
		// archive the product instead of naming an impossible count. Every
		// other path that writes a pack count refuses it, so this one does too.
		if *request.Quantity > util.MaxProductQuantity {
			logger.Warn().Msgf("Product action quantity %d is above the maximum pack count", *request.Quantity)
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrProductQuantityTooLarge)
			return 0, nil, false
		}
		quantity = *request.Quantity
	}

	mutationID := request.ClientMutationID
	if mutationID != nil {
		trimmed := strings.TrimSpace(*mutationID)
		if len(trimmed) > util.OfflineSyncIDMaxLength {
			logger.Warn().Msg("Product action body carries an over-long clientMutationId")
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrClientMutationIDTooLong)
			return 0, nil, false
		}
		mutationID = nil
		if trimmed != "" {
			mutationID = &trimmed
		}
	}
	return quantity, mutationID, true
}

func respondProductActionError(ctx *gin.Context, appCtx *AppContext, productID uint, operation string, err error) {
	switch err {
	case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
	case errors.ErrProductConcurrentModification:
		appCtx.Logger.Warn().Msgf("%s: product %d changed concurrently", operation, productID)
		ctx.JSON(http.StatusConflict, api.APIResponse{Message: MsgProductConcurrentModification})
	default:
		appCtx.Logger.Error().Msgf("%s: %s", operation, err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
	}
}

// ConsumeProduct records a full or partial consume (soft-delete when all packs are gone)
// @Summary      Consume product packs
// @Description  Consumes the requested number of packs. Omitting the body or quantity consumes the whole stock. A partial consume leaves the remaining packs active; a full consume soft-deletes the row with removal reason consumed. The response carries the number of packs that actually left the stock.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        id      path  int                   true   "Product ID"
// @Param        action  body  apiModel.ProductActionDTO  false  "Pack quantity and optional offline mutation ID"
// @Success      200     {object}  api.ProductActionResponse
// @Failure      400     {object}  api.APIResponse
// @Failure      404     {object}  api.APIResponse
// @Failure      409     {object}  api.APIResponse
// @Failure      500     {object}  api.APIResponse
// @Router       /api/v1/products/{id}/consume [post]
func ConsumeProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}
	quantity, mutationID, ok := parseProductActionBody(ctx, appCtx.Logger)
	if !ok {
		return
	}

	removed, err := appCtx.Products.ConsumeProductPartial(productID, appCtx.UserID, quantity, mutationID)
	if err != nil {
		respondProductActionError(ctx, appCtx, productID, "ConsumeProduct", err)
		return
	}

	ctx.JSON(http.StatusOK, api.ProductActionResponse{
		APIResponse: api.APIResponse{Message: fmt.Sprintf("Product %d consume recorded", productID)},
		Removed:     removed,
	})
}

// WasteProduct records a full or partial waste (soft-delete and product.wasted event only when all packs are gone)
// @Summary      Waste product packs
// @Description  Wastes the requested number of packs. Omitting the body or quantity wastes the whole stock. A partial waste leaves the remaining packs active; a full waste soft-deletes the row with removal reason wasted and fires the product.wasted webhook. The response carries the number of packs that actually left the stock.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        id      path  int                   true   "Product ID"
// @Param        action  body  apiModel.ProductActionDTO  false  "Pack quantity and optional offline mutation ID"
// @Success      200     {object}  api.ProductActionResponse
// @Failure      400     {object}  api.APIResponse
// @Failure      404     {object}  api.APIResponse
// @Failure      409     {object}  api.APIResponse
// @Failure      500     {object}  api.APIResponse
// @Router       /api/v1/products/{id}/waste [post]
func WasteProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}
	quantity, mutationID, ok := parseProductActionBody(ctx, appCtx.Logger)
	if !ok {
		return
	}

	removed, err := appCtx.Products.WasteProductPartial(productID, appCtx.UserID, quantity, mutationID)
	if err != nil {
		respondProductActionError(ctx, appCtx, productID, "WasteProduct", err)
		return
	}

	ctx.JSON(http.StatusOK, api.ProductActionResponse{
		APIResponse: api.APIResponse{Message: fmt.Sprintf("Product %d waste recorded", productID)},
		Removed:     removed,
	})
}
