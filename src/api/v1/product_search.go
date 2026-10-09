// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
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
func GetProductsByBarcode(ctx *gin.Context, appCtx *AppContext) {
	barcodeParam := ctx.Param("barcode")

	if len(barcodeParam) != 13 || !validEAN13Format(barcodeParam) {
		appCtx.Logger.Warn().Msgf("Requested barcode '%s' is invalid EAN-13 code", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is an invalid EAN-13 code", barcodeParam)})
		return
	}

	if !validEAN13Checksum(barcodeParam) {
		appCtx.Logger.Warn().Msgf("Requested barcode '%s' has invalid EAN-13 checksum", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is an invalid EAN-13 code", barcodeParam)})
		return
	}

	barcode, err := strconv.Atoi(barcodeParam)
	if err != nil {
		appCtx.Logger.Warn().Msgf("Requested barcode '%s' is invalid", barcodeParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Barcode '%s' is invalid", barcodeParam)})
		return
	}

	products, getError := appCtx.Repos.Products.GetUserProductsBulkByBarcode(appCtx.UserID, barcode)

	switch getError {
	case nil:
		ctx.JSON(http.StatusOK, products)
		return
	case errors.ErrInvalidUserData, gorm.ErrRecordNotFound:
		// The caller's own account state is unusable (no household / user row
		// gone): a bad request, not a server fault. Unknown barcodes never
		// reach this branch — they come back as an empty 200.
		appCtx.Logger.Warn().Msgf("Products with barcode '%d' lookup rejected: %s", barcode, getError)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Products with barcode '%d' were not found", barcode)})
		return
	default:
		appCtx.Logger.Error().Msgf("Products with barcode '%d' lookup failed: %s", barcode, getError)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
}

// SearchProducts returns a list of products based on a query
// @Summary      	Search products
// @Description  	Returns a list of products based on a query
// @Tags         	product
// @Produce      	json
// @Param        	queryParam  query  string  true  	"Search field (product_name, barcode)"
// @Param        	queryValue  query  string  true  	"Search value"
// @Param        	sort        query  string  false  "Sort field (product_name, expire_at, created_at, scanned_at, notified_at, barcode)"  default(product_name)
// @Param        	order       query  string  false  "Sort order (asc, desc)"  default(asc)
// @Success      	200  {object}  []database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/search [GET]
func SearchProducts(ctx *gin.Context, appCtx *AppContext) {
	q, ok := ParseProductSearchQuery(ctx)
	if !ok {
		return
	}

	enumParam := database.SearchParameterEnumFromString(q.QueryParam)
	if enumParam == database.InvalidParameter {
		appCtx.Logger.Warn().Msg(errors.ErrProductSearchInvalidQuery.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "No valid search parameters found"})
		return
	}

	products, productErr := appCtx.Repos.Products.SearchProducts(enumParam, q.QueryValue, q.Sort, q.Order, appCtx.UserID)
	if productErr != nil {
		// The caller picked the sort column/direction and owns their account
		// row: those misses are client input, not server faults. The query
		// binding rejects most bad sort values first, but the repo allowlist
		// can lag it — that drift must not read as a 500 either.
		switch productErr {
		case errors.ErrDatabaseInvalidSortParameter:
			// The binding rejects most bad sort values first, but the repo
			// allowlist can lag it — a public error message, safe to echo.
			appCtx.Logger.Warn().Msgf("%s: %s", errors.MsgErrGettingProducts, productErr)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: productErr.Error()})
			return
		case gorm.ErrRecordNotFound:
			// The caller's own account row is gone: report the account state,
			// never gorm's internal "record not found".
			appCtx.Logger.Warn().Msgf("%s: %s", errors.MsgErrGettingProducts, productErr)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserData.Error()})
			return
		default:
			appCtx.Logger.Error().Msgf("%s: %s", errors.MsgErrGettingProducts, productErr)
			ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrGettingProducts})
			return
		}
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
