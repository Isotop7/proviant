// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
func BulkConsumeProducts(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, logger, &products) {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	for _, productID := range products.ProductIDs {
		product, fetchErr := repos.Products.GetProductByID(productID, userID)

		if err := repos.Products.ConsumeProduct(productID, userID); err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warn().Msgf("BulkConsumeProducts: product %d not found", productID)
			} else {
				logger.Error().Msgf("BulkConsumeProducts: %s", err)
			}
			continue
		}

		if fetchErr == nil {
			go recordHouseholdSavingsEvent(repos, logger, userID, &product, "consumed")
		}
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
func BulkWasteProducts(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	var products apiModel.BulkProductsAPIModel
	if !bindJSON(ctx, logger, &products) {
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	householdID, householdErr := repos.Users.GetUserHouseholdByID(userID)

	for _, productID := range products.ProductIDs {
		product, fetchErr := repos.Products.GetProductByID(productID, userID)

		if err := repos.Products.WasteProduct(productID, userID); err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warn().Msgf("BulkWasteProducts: product %d not found", productID)
			} else {
				logger.Error().Msgf("BulkWasteProducts: %s", err)
			}
			continue
		}

		if householdErr == nil && householdID > 0 {
			if err := repos.Streaks.RecordWasteEvent(householdID); err != nil {
				logger.Error().Msgf("BulkWasteProducts: failed to record waste event for streak: %s", err)
			}
		}

		go func(pid uint) {
			if ws := controllers.GetWebhookService(); ws != nil {
				ws.FireEvent("product.wasted", map[string]any{
					"productId": pid,
				})
			}
		}(productID)

		if fetchErr == nil {
			go recordHouseholdSavingsEvent(repos, logger, userID, &product, "wasted")
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("%d products marked as wasted", len(products.ProductIDs))})
}
