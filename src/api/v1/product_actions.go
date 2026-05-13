// v1 implements version 1 of the proviant API
package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	MsgProductNotFound = "Product not found"
)

func recordHouseholdSavingsEvent(appCtx *AppContext, product *dbModel.Product, eventType string) {
	if householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID); err == nil && householdID > 0 {
		if err := appCtx.Repos.Savings.RecordSavingsEvent(householdID, product, eventType); err != nil {
			appCtx.Logger.Error().Msgf("RecordSavingsEvent (%s): %s", eventType, err)
		}
	}
}

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

	product, err := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		appCtx.Logger.Error().Msgf("ConsumeProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := appCtx.Repos.Products.ConsumeProduct(productID, appCtx.UserID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		appCtx.Logger.Error().Msgf("ConsumeProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	go recordHouseholdSavingsEvent(appCtx, &product, "consumed")

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

	product, err := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		appCtx.Logger.Error().Msgf("WasteProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := appCtx.Repos.Products.WasteProduct(productID, appCtx.UserID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, api.APIResponse{Message: MsgProductNotFound})
			return
		}
		appCtx.Logger.Error().Msgf("WasteProduct: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID); err == nil && householdID > 0 {
		if err := appCtx.Repos.Streaks.RecordWasteEvent(householdID); err != nil {
			appCtx.Logger.Error().Msgf("WasteProduct: failed to record waste event for streak: %s", err)
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEventContext(ctx, "product.wasted", map[string]any{
				"productId": productID,
			})
		}
	}()

	go recordHouseholdSavingsEvent(appCtx, &product, "wasted")

	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d marked as wasted", productID)})
}
