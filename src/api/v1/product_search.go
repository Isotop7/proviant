// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	barcodeParam := ctx.Param("barcode")

	if len(barcodeParam) != 13 || !validEAN13Format(barcodeParam) {
		logger.Warn().Msgf("Requested barcode '%s' is invalid EAN-13 code", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is an invalid EAN-13 code", barcodeParam)})
		return
	}

	if !validEAN13Checksum(barcodeParam) {
		logger.Warn().Msgf("Requested barcode '%s' has invalid EAN-13 checksum", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is an invalid EAN-13 code", barcodeParam)})
		return
	}

	barcode, err := strconv.Atoi(barcodeParam)
	if err != nil {
		logger.Warn().Msgf("Requested barcode '%s' is invalid", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is invalid", barcodeParam)})
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

	products, getError := repos.Products.GetUserProductsBulkByBarcode(userID, barcode)

	switch getError {
	// No error: return product
	case nil:
		ctx.JSON(http.StatusOK, products)
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Warn().Msgf("Products with barcode '%d' for user were not found in database (mismatched userID in JWT <> DB)", barcode)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Products with barcode '%d' for user were not found", barcode)})
		return
	// Unspecified error
	default:
		logger.Warn().Msgf("Products with barcode '%d' were not found in database", barcode)
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Get search parameters
	var queryParam = ctx.DefaultQuery("queryParam", "product_name")
	var queryValue = ctx.DefaultQuery("queryValue", "")
	var sort = ctx.DefaultQuery("sort", "product_name")
	var order = ctx.DefaultQuery("order", "asc")

	enumParam := database.SearchParameterEnumFromString(queryParam)
	if enumParam == database.InvalidParameter {
		// If no supported parameter was found, exit
		logger.Warn().Msg(errors.ErrProductSearchInvalidQuery.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "No valid search parameters found"})
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

	products, productErr := repos.Products.SearchProducts(enumParam, queryValue, sort, order, userID)
	if productErr != nil {
		logger.Error().Msgf("%s: %s", errors.MsgErrGettingProducts, productErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrGettingProducts})
		return
	}

	ctx.JSON(http.StatusOK, products)
}

func validEAN13Format(barcode string) bool {
	for _, c := range barcode {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func validEAN13Checksum(barcode string) bool {
	sum := 0
	for i, c := range barcode {
		digit := int(c - '0')
		if i%2 == 0 {
			sum += digit * 3
		} else {
			sum += digit
		}
	}
	return sum%10 == 0
}
