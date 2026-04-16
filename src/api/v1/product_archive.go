// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func convertStringIDsToInts(ids []string) ([]int, error) {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
}

func joinErrors(errs []database.BulkOperationError) string {
	var b strings.Builder
	for i := range errs {
		b.WriteString(errs[i].Error())
	}
	return b.String()
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
func GetArchivedProducts(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter limit
	limitParam := ctx.Query("limit")
	var limit int
	// Check if limit was found in query
	if limitParam != "" {
		var parseError error
		if limit, parseError = strconv.Atoi(limitParam); parseError != nil {
			logger.Warn().Msgf("Invalid limit '%d' was specified", limit)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Limit '%d' is invalid", limit)})
			return
		}
	} else {
		// Set default limit
		limit = 0
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	products, productBulkErr := productRepo.GetUserArchivedProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Error getting products of user"})
		return
	} else {
		ctx.JSON(http.StatusOK, products)
		return
	}
}

// BulkDeleteProducts deletes a list of products of a user
// @Summary      	Deletes a list of products
// @Description  	Deletes a list of products of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			productIDs	body	[]int				true	"Product IDs"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/bulkDelete [delete]
func BulkDeleteProducts(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse body to list of product IDs
	var products apiModel.BulkProductsAPIModel
	if err := ctx.ShouldBindJSON(&products); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	convertedProductIDs, convErr := convertStringIDsToInts(products.ProductIDs)
	if convErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	bulkDeleteResultError := productRepo.BulkDeleteProducts(convertedProductIDs, userID)
	if len(bulkDeleteResultError) > 0 {
		msg := joinErrors(bulkDeleteResultError)
		logger.Error().Msg(msg)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: msg})
		return
	}
	strProductIDs := make([]string, len(convertedProductIDs))
	for i, v := range convertedProductIDs {
		strProductIDs[i] = strconv.Itoa(v)
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were deleted", strings.Join(strProductIDs, ";"))})
}

// BulkArchiveProducts archives a list of products of a user
// @Summary      	Archives a list of products
// @Description  	Archives a list of products of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			productIDs	body	[]int				true	"Product IDs"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/bulkArchive [delete]
func BulkArchiveProducts(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse body to list of product IDs
	var products apiModel.BulkProductsAPIModel
	if err := ctx.ShouldBindJSON(&products); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	convertedProductIDs, convErr := convertStringIDsToInts(products.ProductIDs)
	if convErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	bulkArchiveError := productRepo.BulkArchiveProducts(convertedProductIDs, userID)
	if len(bulkArchiveError) > 0 {
		msg := joinErrors(bulkArchiveError)
		logger.Error().Msg(msg)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: msg})
		return
	}
	strProductIDs := make([]string, len(convertedProductIDs))
	for i, v := range convertedProductIDs {
		strProductIDs[i] = strconv.Itoa(v)
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were archived", strings.Join(strProductIDs, ";"))})
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
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id}/restore [post]
func RestoreProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatInvalidRequestId, idParam)})
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	restoreResult := productRepo.RestoreProduct(productID, userID)
	if restoreResult != nil {
		logger.Error().Msgf("Error restoring product: %s", restoreResult)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: restoreResult.Error()})
		return
	} else {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was restored", productID)})
		return
	}
}

// BulkRestoreProducts restores a list of products of a user
// @Summary      	Restores a list of product
// @Description  	Restores a list of product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			productIDs	body	[]int				true	"Product IDs"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/bulkRestore [post]
func BulkRestoreProducts(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse body to list of product IDs
	var products apiModel.BulkProductsAPIModel
	if err := ctx.ShouldBindJSON(&products); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	convertedProductIDs, convErr := convertStringIDsToInts(products.ProductIDs)
	if convErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	bulkRestoreError := productRepo.BulkRestoreProducts(convertedProductIDs, userID)
	if len(bulkRestoreError) > 0 {
		msg := joinErrors(bulkRestoreError)
		logger.Error().Msg(msg)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: msg})
		return
	}
	strProductIDs := make([]string, len(convertedProductIDs))
	for i, v := range convertedProductIDs {
		strProductIDs[i] = strconv.Itoa(v)
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were restored", strings.Join(strProductIDs, ";"))})
}
