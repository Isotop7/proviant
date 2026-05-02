// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
func GetExpired(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	products, getExpiredErr := repos.Products.GetProductsExpired(userID)

	if getExpiredErr != nil {
		logger.Error().Msgf("Error getting expired products: %s", getExpiredErr)
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
func GetProductSummary(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
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

	expiringSoonCount, err := repos.Products.GetExpiringSoonCount(userID, 7)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiringSoonProducts, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiringSoon})
		return
	}

	expiredCount, err := repos.Products.GetExpiredProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiredProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expired count"})
		return
	}

	totalActive, err := repos.Products.GetActiveProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetActiveProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingActiveCount})
		return
	}

	wasteThisMonth, err := repos.Products.GetWasteThisMonth(userID)
	if err != nil {
		logger.Error().Msgf("GetWasteThisMonth: %s", err)
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
func GetProductStats(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
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

	totalActive, err := repos.Products.GetActiveProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetActiveProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingActiveCount})
		return
	}

	wasteCount, err := repos.Products.GetExpiredProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiredProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingWasteCount})
		return
	}

	var wastePercent float64
	if totalActive > 0 {
		wastePercent = float64(wasteCount) / float64(totalActive) * 100
	}

	expiringSoonDays := 7
	if user, userErr := repos.Products.GetUserByID(userID); userErr == nil && user.NotificationPreferences.NotificationThresholdDays > 0 {
		expiringSoonDays = user.NotificationPreferences.NotificationThresholdDays
	}

	expiringSoon, err := repos.Products.GetExpiringSoonProducts(userID, expiringSoonDays)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiringSoonProducts, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiringSoon})
		return
	}

	categories, err := repos.Products.GetProductCategoryBreakdown(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetProductCategoryBreakdown, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingCategoryBreakdown})
		return
	}

	expiryTrend, err := repos.Products.GetExpiryTrend(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiryTrend, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiryTrend})
		return
	}

	archivedProducts, err := repos.Products.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		logger.Error().Msgf("GetUserArchivedProductsBulk: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing archived count"})
		return
	}
	totalArchived := len(archivedProducts)

	uniqueArchivedMap, err := repos.Products.GetArchivedProductsGroupedByBarcode(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetArchivedProductsGroupedByBarcode, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingUniqueArchivedCount})
		return
	}
	uniqueArchived := len(uniqueArchivedMap)

	lastInsertedProduct := getLastInsertedProductName(repos, userID)

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
