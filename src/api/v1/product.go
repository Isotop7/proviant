// v1 implements version 1 of the proviant API
package v1

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"strconv"
	"strings"
	"time"

	"image/draw"
	_ "image/jpeg"
	_ "image/png"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// GetProducts returns the products of a user
// @Summary      Return a list of products
// @Description  Return a list of products of user
// @Tags         product
// @Produce      json
// @Success      200  {object}  []database.Product
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products [get]
func GetProducts(ctx *gin.Context) {
	// Get logger instance from context
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		logger.Error().Msg(api.ResponseErrLoggerContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	products, productBulkErr := dbController.GetUserProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Error getting products of user"})
		return
	} else {
		ctx.JSON(http.StatusOK, products)
		return
	}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	products, productBulkErr := dbController.GetUserArchivedProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Error getting products of user"})
		return
	} else {
		ctx.JSON(http.StatusOK, products)
		return
	}
}

// GetProduct return a single product of a user
// @Summary      Returns a single product
// @Description  Returns a single product of user
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  database.Product
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/product/{id} [get]
func GetProduct(ctx *gin.Context) {
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get product from database
	product, getError := dbController.GetProductByID(productID, userID)

	switch getError {
	// No error: return product
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database (mismatched userID in JWT <> DB)", productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductForUserNotFound, productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductNotFound, productID)})
		return
	}
}

// GetProductsByBarcode returns a list of products of a user matching a barcode
// @Summary      Returns a list of products
// @Description  Returns a list of products of user matching the given barcode
// @Tags         product
// @Produce      json
// @Param        barcode   path      int  true  "Barcode"
// @Success      200  {object}  []database.Product
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/productsByBarcode [get]
func GetProductsByBarcode(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter barcode
	barcodeParam := ctx.Param("barcode")
	var barcode int
	var convErr error
	if barcode, convErr = strconv.Atoi(barcodeParam); convErr != nil {
		logger.Warn().Msgf("Requested barcode '%s' is invalid", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is invalid", barcodeParam)})
		return
	}

	// Check for valid EAN-13
	if barcode < 1000000000000 || barcode > 10000000000000 {
		logger.Warn().Msgf("Requested barcode '%d' is invalid EAN-13 code", barcode)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%d' is an invalid EAN-13 code", barcode)})
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get product from database
	products, getError := dbController.GetUserProductsBulkByBarcode(userID, barcode)

	switch getError {
	// No error: return product
	case nil:
		ctx.JSON(http.StatusOK, products)
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Products with barcode '%d' for user were not found in database (mismatched userID in JWT <> DB)", barcode)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Products with barcode '%d' for user were not found", barcode)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Products with barcode '%d' were not found in database", barcode)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Products with barcode '%d' were not found", barcode)})
		return
	}
}

// CreateProduct creates a new product of a user
// @Summary      	Creates a new product
// @Description  	Creates a new product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			product	body	database.Product	true	"Product"
// @Success      	201  {object}  database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products [post]
func CreateProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims["id"].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	// Get and parse body to product
	var product dbModel.Product
	if err := ctx.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	// Check for required parameters
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "barcode missing"})
		return
	}

	// Get OpenFoodFacts API controller from context
	offacntrl, offaErr := ctx.MustGet("offacntrl").(controllers.OpenFoodFactsAPIControllerInterface)
	if !offaErr {
		logger.Error().Msg("Failed to get controller from context")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to get controller from context"})
		return
	}
	// Get product data from API
	var apiProduct dbModel.Product
	apiProduct, err := offacntrl.GetDataset(product.Barcode)
	if err == nil {
		// Preserve timestamps
		apiProduct.ScannedAt = time.Now()
		apiProduct.ExpireAt = product.ExpireAt
		product = apiProduct
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Create product in database
	createResult := dbController.CreateProduct(userID, &product)
	if createResult != nil {
		logger.Error().Msgf("Error creating product: %s", createResult)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: createResult.Error()})
		return
	} else {
		ctx.JSON(http.StatusCreated, product)
		return
	}
}

// UpdateProduct updates a product of a user
// @Summary      	Updates a product
// @Description  	Updates a product with new values
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Param			product	body	database.Product	true	"Product"
// @Success      	200  {object}  database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id} [patch]
func UpdateProduct(ctx *gin.Context) {
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

	// Get and parse body to product
	var product dbModel.ProductDTOPatch
	if err := ctx.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Update product in database
	updateErr := dbController.UpdateProduct(productID, userID, &product)

	switch updateErr {
	// No error => product was updated
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	// Requested product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: updateErr.Error()})
		return
	}
}

// DeleteProduct deletes a product of a user
// @Summary      	Deletes a product
// @Description  	Deletes a product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Param        	archiveOnly	query	bool				false	"Archive only"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id} [delete]
func DeleteProduct(ctx *gin.Context) {
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

	var archiveOnly bool
	var parseError error
	// Get and parse parameter archiveOnly
	archiveOnlyParam, archiveOnlyParamExists := ctx.GetQuery("archiveOnly")
	// Check if archiveOnlyParam is supplied
	if !archiveOnlyParamExists {
		// Default to hard deletion
		archiveOnly = false
	} else {
		// Try to parse archiveOnlyParam as a boolean
		if archiveOnly, parseError = strconv.ParseBool(archiveOnlyParam); parseError != nil {
			logger.Warn().Msgf("Invalid archiveOnly '%s' was specified", archiveOnlyParam)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("archiveOnly '%s' is invalid", archiveOnlyParam)})
			return
		}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Delete product from database
	deleteResult := dbController.DeleteProduct(productID, userID, archiveOnly)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: deleteResult.Error()})
		return
	} else {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was deleted", productID)})
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

	// Try to get int values
	var convertedProductIDs []int
	for _, id := range products.ProductIDs {
		if productID, convErr := strconv.Atoi(id); convErr != nil {
			logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
			return
		} else {
			convertedProductIDs = append(convertedProductIDs, productID)
		}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Delete product from database
	bulkDeleteResultError := dbController.BulkDeleteProducts(convertedProductIDs, userID)
	// Check for errors
	if len(bulkDeleteResultError) > 0 {
		// If error is not nil, log error and return error response as one string
		var errorOutput string
		for _, bulkDeleteResult := range bulkDeleteResultError {
			errorOutput += bulkDeleteResult.Error()
		}
		logger.Error().Msg(errorOutput)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errorOutput})
		return
	} else {
		strProductIDs := make([]string, len(convertedProductIDs))
		for i, v := range convertedProductIDs {
			strProductIDs[i] = strconv.Itoa(v)
		}
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were deleted", strings.Join(strProductIDs, ";"))})
		return
	}
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

	// Try to get int values
	var convertedProductIDs []int
	for _, id := range products.ProductIDs {
		if productID, convErr := strconv.Atoi(id); convErr != nil {
			logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
			return
		} else {
			convertedProductIDs = append(convertedProductIDs, productID)
		}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Bulk archive products
	bulkArchiveError := dbController.BulkArchiveProducts(convertedProductIDs, userID)
	// Check for errors
	if len(bulkArchiveError) > 0 {
		// If error is not nil, log error and return error response as one string
		var errorOutput string
		for _, bulkArchiveResult := range bulkArchiveError {
			errorOutput += bulkArchiveResult.Error()
		}
		logger.Error().Msg(errorOutput)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errorOutput})
		return
	} else {
		strProductIDs := make([]string, len(convertedProductIDs))
		for i, v := range convertedProductIDs {
			strProductIDs[i] = strconv.Itoa(v)
		}
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were deleted", strings.Join(strProductIDs, ";"))})
		return
	}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Delete product from database
	restoreResult := dbController.RestoreProduct(productID, userID)
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

	// Try to get int values
	var convertedProductIDs []int
	for _, id := range products.ProductIDs {
		if productID, convErr := strconv.Atoi(id); convErr != nil {
			logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), convErr.Error())
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: convErr.Error()})
			return
		} else {
			convertedProductIDs = append(convertedProductIDs, productID)
		}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Bulk restore products
	bulkRestoreError := dbController.BulkRestoreProducts(convertedProductIDs, userID)
	// Check for errors
	if len(bulkRestoreError) > 0 {
		// If error is not nil, log error and return error response as one string
		var errorOutput string
		for _, bulkRestoreResult := range bulkRestoreError {
			errorOutput += bulkRestoreResult.Error()
		}
		logger.Error().Msg(errorOutput)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errorOutput})
		return
	} else {
		strProductIDs := make([]string, len(convertedProductIDs))
		for i, v := range convertedProductIDs {
			strProductIDs[i] = strconv.Itoa(v)
		}
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Products with ID '%s' were restored", strings.Join(strProductIDs, ";"))})
		return
	}
}

// SetExpireAt updates the expire date of a product of a user
// @Summary      	Updates the expire date
// @Description  	Updates the expire date of a product
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   		path	int					true  	"Product ID"
// @Param			timestamp	body	database.Timestamp	true	"Timestamp"
// @Success      	200  {object}  database.ProductDTOExpire
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id}/expire [post]
func SetExpireAt(ctx *gin.Context) {
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

	// Get and parse body to timestamp
	var expireAt dbModel.Timestamp
	var bindErr error
	if bindErr = ctx.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: bindErr.Error()})
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	product, getErr := dbController.GetProductByID(productID, userID)
	if getErr != nil {
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID)})
		return
	}

	// Update expire date of product
	updateErr := dbController.SetProductExpireAt(productID, userID, expireAt)

	switch updateErr {
	// No error => product was updated and dto is returned
	case nil:
		expireDTO := dbModel.ProductDTOExpire{
			ID:       product.ID,
			Barcode:  product.Barcode,
			ExpireAt: expireAt.Timestamp,
		}
		ctx.JSON(http.StatusOK, expireDTO)
		return
	// Product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID)})
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database: %s", productID, updateErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: updateErr.Error()})
		return
	}
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get expired products of user from database
	products, getExpiredErr := dbController.GetProductsExpired(userID)

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

// ScanProduct returns a barcode based on an image
// @Summary      	Scan product
// @Description  	Returns the barcode of a product in an uploaded image
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  database.ProductDTOBarcode
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/scan [post]
func ScanProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Create variables
	var decodedBarcode string

	// Create decoding context
	decodingTimeout := static.BarcodeDecodingTimeout
	decodingContext, cancel := context.WithTimeout(context.Background(), decodingTimeout)
	defer cancel()
	// Create communication channel for go routine
	decodingProcessChannel := make(chan bool)

	// Decode image go routine
	go func() {
		// Read file from form or signal error
		file, formErr := ctx.FormFile("image")
		if formErr != nil {
			logger.Error().Msgf("Error reading image from body: %s", formErr.Error())
			ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrNoBarcodeFoundInImage))
			decodingProcessChannel <- false
			return
		}

		// Open file from form or signal error
		src, openErr := file.Open()
		if openErr != nil {
			logger.Error().Msgf("Error opening file: %s", openErr.Error())
			ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrNoBarcodeFoundInImage))
			decodingProcessChannel <- false
			return
		}

		// Decode file from form as image or signal error
		img, format, decodeErr := image.Decode(src)
		if decodeErr != nil {
			logger.Error().Msgf("Error decoding image as type %s: %s", format, decodeErr.Error())
			ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrNoBarcodeFoundInImage))
			decodingProcessChannel <- false
			return
		} else {
			logger.Info().Msgf("Decoded image of type '%s' from body", format)
		}

		// Convert image to gray scale image
		convertedImage := image.NewGray(img.Bounds())
		draw.Draw(convertedImage, convertedImage.Bounds(), img, img.Bounds().Min, draw.Src)

		// Convert gray scaled image to binary bitmap or signal error
		bmp, bmpErr := gozxing.NewBinaryBitmapFromImage(convertedImage)
		if bmpErr != nil {
			logger.Error().Msgf("Error converting image to bitmap: %s", bmpErr.Error())
			ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrNoBarcodeFoundInImage))
			decodingProcessChannel <- false
			return
		}

		// Create EAN13 scanner and hints
		scanner := oned.NewEAN13Reader()
		hints := map[gozxing.DecodeHintType]any{
			gozxing.DecodeHintType_TRY_HARDER:             true,
			gozxing.DecodeHintType_ALLOWED_EAN_EXTENSIONS: true,
			gozxing.DecodeHintType_ALSO_INVERTED:          true,
		}

		// Decode image and try to find barcode
		code, scanErr := scanner.Decode(bmp, hints)
		if scanErr != nil {
			logger.Error().Msgf("Error decoding image when finding barcode: %s", scanErr.Error())
			ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrNoBarcodeFoundInImage))
			return
		} else {
			// If barcode is found, return it
			decodedBarcode = code.GetText()
		}
		// Signal success
		decodingProcessChannel <- true
	}()

	// Wait for success or timeout
	select {
	case <-decodingContext.Done():
		// Timeout was reached, error is returned
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrBarcodeDecodeTimeoutExceeded))
		return
	case success := <-decodingProcessChannel:
		// Timeout was not reached and channel signaled success on decoding barcode
		if success {
			ctx.JSON(http.StatusOK, dbModel.ProductDTOBarcode{Barcode: decodedBarcode})
			return
		}
	}

	// Return if timeout was not reached but channel did not signal success
	ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrNoBarcodeFoundInImage))
}

// SearchProducts returns a list of products based on an query
// @Summary      	Search products
// @Description  	Returns a list of products based on a query
// @Tags         	product
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Success      	200  {object}  []database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/search [GET]
func SearchProducts(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get search parameters
	var queryParam = ctx.DefaultQuery("queryParam", "product_name")
	var queryValue = ctx.DefaultQuery("queryValue", "")
	var sort = ctx.DefaultQuery("sort", "product_name")
	var order = ctx.DefaultQuery("order", "asc")

	enumParam := database.SearchParameterEnumFromString(queryParam)
	if enumParam == database.InvalidParameter {
		// If no supported parameter was found, exit
		logger.Error().Msg(errors.ErrProductSearchInvalidQuery.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "No valid search parameters found"})
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting products"})
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	products, productErr := dbController.SearchProducts(enumParam, queryValue, sort, order, userID)
	if productErr != nil {
		logger.Error().Msgf("Error getting products: %s", productErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting products"})
		return
	}

	ctx.JSON(http.StatusOK, products)
}
