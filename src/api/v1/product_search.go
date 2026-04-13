// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

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

	productRepo := database.NewProductRepository(dbHandle)
	products, getError := productRepo.GetUserProductsBulkByBarcode(userID, barcode)

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

// SearchProducts returns a list of products based on a query
// @Summary      	Search products
// @Description  	Returns a list of products based on a query
// @Tags         	product
// @Produce      	json
// @Param        	queryParam  query  string  true  	"Search field (product_name, barcode, category, storage_location)"
// @Param        	queryValue  query  string  true  	"Search value"
// @Param        	sort        query  string  false  "Sort field"  default(product_name)
// @Param        	order       query  string  false  "Sort order (asc, desc)"  default(asc)
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
	products, productErr := productRepo.SearchProducts(enumParam, queryValue, sort, order, userID)
	if productErr != nil {
		logger.Error().Msgf("Error getting products: %s", productErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting products"})
		return
	}

	ctx.JSON(http.StatusOK, products)
}
