// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	products, getExpiredErr := productRepo.GetProductsExpired(userID)

	// Check for error or return products
	if getExpiredErr != nil {
		logger.Error().Msgf("Error getting expired products: %s", getExpiredErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting expired products"})
		return
	} else {
		ctx.JSON(http.StatusOK, products)
		return
	}
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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)

	totalActive, err := productRepo.GetActiveProductsCount(userID)
	if err != nil {
		logger.Error().Msgf("GetActiveProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing active product count"})
		return
	}

	wasteCount, err := productRepo.GetExpiredProductsCount(userID)
	if err != nil {
		logger.Error().Msgf("GetExpiredProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing waste count"})
		return
	}

	var wastePercent float64
	if totalActive > 0 {
		wastePercent = float64(wasteCount) / float64(totalActive) * 100
	}

	expiringSoonDays := 7
	if user, userErr := productRepo.GetUserByID(userID); userErr == nil && user.NotificationPreferences.NotificationThresholdDays > 0 {
		expiringSoonDays = user.NotificationPreferences.NotificationThresholdDays
	}

	expiringSoon, err := productRepo.GetExpiringSoonProducts(userID, expiringSoonDays)
	if err != nil {
		logger.Error().Msgf("GetExpiringSoonProducts: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expiring soon products"})
		return
	}

	categories, err := productRepo.GetProductCategoryBreakdown(userID)
	if err != nil {
		logger.Error().Msgf("GetProductCategoryBreakdown: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing category breakdown"})
		return
	}

	expiryTrend, err := productRepo.GetExpiryTrend(userID)
	if err != nil {
		logger.Error().Msgf("GetExpiryTrend: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expiry trend"})
		return
	}

	archivedProducts, err := productRepo.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		logger.Error().Msgf("GetUserArchivedProductsBulk: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing archived count"})
		return
	}
	totalArchived := len(archivedProducts)

	uniqueArchivedMap, err := productRepo.GetArchivedProductsGroupedByBarcode(userID)
	if err != nil {
		logger.Error().Msgf("GetArchivedProductsGroupedByBarcode: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing unique archived count"})
		return
	}
	uniqueArchived := len(uniqueArchivedMap)

	var lastInsertedProduct string
	householdID, householdErr := productRepo.GetUserHouseholdByID(userID)
	if householdErr == nil && householdID > 0 {
		lastProduct, lastErr := productRepo.GetLastInsertedProduct(householdID)
		if lastErr == nil && lastProduct.ID != 0 {
			lastInsertedProduct = lastProduct.ProductName
		}
	}

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
