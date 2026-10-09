// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

func getLastInsertedProductName(repos *database.RepositoryContainer, userID uint) string {
	householdID, householdErr := repos.Products.GetUserHouseholdByID(userID)
	if householdErr != nil || householdID == 0 {
		return ""
	}
	lastProduct, lastErr := repos.Products.GetLastInsertedProduct(householdID)
	if lastErr != nil || lastProduct.ID == 0 {
		return ""
	}
	return lastProduct.ProductName
}

// GetExpired returns the list of all expired products of a user
// @Summary      	Gets expired products
// @Description  	Gets a list of expired products of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  []database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/expired [get]
func GetExpired(ctx *gin.Context, appCtx *AppContext) {
	products, getExpiredErr := appCtx.Repos.Products.GetProductsExpired(appCtx.UserID)

	if getExpiredErr != nil {
		appCtx.Logger.Error().Msgf("Error getting expired products: %s", getExpiredErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting expired products"})
		return
	}
	ctx.JSON(http.StatusOK, products)
}

// GetProductSummary returns a lightweight count summary for Home Assistant sensor polling
// @Summary      Return product summary
// @Description  Returns expiring-soon count, expired count, total active count, and waste-this-month count in one request
// @Tags         product
// @Produce      json
// @Success      200  {object}  apiModel.ProductSummaryResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/summary [get]
func GetProductSummary(ctx *gin.Context, appCtx *AppContext) {
	expiringSoonCount, err := appCtx.Repos.Products.GetExpiringSoonCount(appCtx.UserID, 7)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetExpiringSoonProducts, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiringSoon})
		return
	}

	expiredCount, err := appCtx.Repos.Products.GetExpiredProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetExpiredProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expired count"})
		return
	}

	totalActive, err := appCtx.Repos.Products.GetActiveProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetActiveProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingActiveCount})
		return
	}

	wasteThisMonth, err := appCtx.Repos.Products.GetWasteThisMonth(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetWasteThisMonth: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing monthly waste count"})
		return
	}

	ctx.JSON(http.StatusOK, apiModel.ProductSummaryResponse{
		ExpiringSoonCount: expiringSoonCount,
		ExpiredCount:      expiredCount,
		TotalActive:       totalActive,
		WasteThisMonth:    wasteThisMonth,
	})
}

// GetProductStats returns aggregated product statistics for the authenticated user
// @Summary      Return product statistics
// @Description  Returns waste rate, top archived products, category breakdown and expiry trend
// @Tags         product
// @Produce      json
// @Success      200  {object}  apiModel.ProductStatsResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/stats [get]
func GetProductStats(ctx *gin.Context, appCtx *AppContext) {
	totalActive, err := appCtx.Repos.Products.GetActiveProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetActiveProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingActiveCount})
		return
	}

	wasteCount, err := appCtx.Repos.Products.GetExpiredProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetExpiredProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingWasteCount})
		return
	}

	var wastePercent float64
	if totalActive > 0 {
		wastePercent = float64(wasteCount) / float64(totalActive) * 100
	}

	expiringSoonDays := 7
	if user, userErr := appCtx.Repos.Products.GetUserByID(appCtx.UserID); userErr == nil && user.NotificationPreferences.NotificationThresholdDays > 0 {
		expiringSoonDays = user.NotificationPreferences.NotificationThresholdDays
	}

	expiringSoon, err := appCtx.Repos.Products.GetExpiringSoonProducts(appCtx.UserID, expiringSoonDays)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetExpiringSoonProducts, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiringSoon})
		return
	}

	categories, err := appCtx.Repos.Products.GetProductCategoryBreakdown(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetProductCategoryBreakdown, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingCategoryBreakdown})
		return
	}

	expiryTrend, err := appCtx.Repos.Products.GetExpiryTrend(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf(errors.FmtErrGetExpiryTrend, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiryTrend})
		return
	}

	// Counts, not row loads: the archived rows are large and the endpoint only
	// ever reported their totals.
	totalArchived, err := appCtx.Repos.Products.GetArchivedProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetArchivedProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing archived count"})
		return
	}

	uniqueArchived, err := appCtx.Repos.Products.GetUniqueArchivedProductsCount(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetUniqueArchivedProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingUniqueArchivedCount})
		return
	}

	lastInsertedProduct := getLastInsertedProductName(appCtx.Repos, appCtx.UserID)

	ctx.JSON(http.StatusOK, apiModel.ProductStatsResponse{
		WasteCount:          wasteCount,
		WastePercent:        wastePercent,
		TotalActive:         totalActive,
		TotalArchived:       totalArchived,
		UniqueArchived:      uniqueArchived,
		LastInsertedProduct: lastInsertedProduct,
		ExpiringSoon:        expiringSoon,
		ExpiringSoonDays:    expiringSoonDays,
		Categories:          categories,
		ExpiryTrend:         expiryTrend,
	})
}
