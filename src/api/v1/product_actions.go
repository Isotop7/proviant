// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"

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

	productID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
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

	product, fetchErr := repos.Products.GetProductByID(productID, userID)

	if err := repos.Products.ConsumeProduct(productID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "Product not found"})
			return
		}
		logger.Error().Msgf("ConsumeProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if fetchErr == nil {
		go recordHouseholdSavingsEvent(repos, logger, userID, product, "consumed")
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as consumed", productID)})
}

func recordHouseholdSavingsEvent(repos *database.RepositoryContainer, logger *zerolog.Logger, userID uint, product dbModel.Product, eventType string) {
	if householdID, err := repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		if err := repos.Savings.RecordSavingsEvent(householdID, &product, eventType); err != nil {
			logger.Error().Msgf("RecordSavingsEvent (%s): %s", eventType, err)
		}
	}
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

	productID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
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

	product, fetchErr := repos.Products.GetProductByID(productID, userID)

	if err := repos.Products.WasteProduct(productID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "Product not found"})
			return
		}
		logger.Error().Msgf("WasteProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	// Record waste event for household streak tracking
	if householdID, err := repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		if err := repos.Streaks.RecordWasteEvent(householdID); err != nil {
			logger.Error().Msgf("WasteProduct: failed to record waste event for streak: %s", err)
		}
	}

	go func() {
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEvent("product.wasted", map[string]any{
				"productId": productID,
			})
		}
	}()

	if fetchErr == nil {
		go recordHouseholdSavingsEvent(repos, logger, userID, product, "wasted")
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as wasted", productID)})
}
