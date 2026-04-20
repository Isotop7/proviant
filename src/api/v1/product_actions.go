// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
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
func ConsumeProduct(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	if err := productRepo.ConsumeProduct(productID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "Product not found"})
			return
		}
		logger.Error().Msgf("ConsumeProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: err.Error()})
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
func WasteProduct(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	if err := productRepo.WasteProduct(productID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "Product not found"})
			return
		}
		logger.Error().Msgf("WasteProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: err.Error()})
		return
	}

	go func() {
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEvent("product.wasted", map[string]any{
				"productId": productID,
			})
		}
	}()

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as wasted", productID)})
}
