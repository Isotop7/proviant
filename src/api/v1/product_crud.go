// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const (
	MsgCheckProductIdTryAgain           = "Check the product ID and try again"
	FmtProductNotFoundOrNoAccess        = "Product with ID '%d' was not found or you do not have access"
	MsgFailedToGetControllerFromContext = "Failed to get controller from context"
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
	logger, loggerOk := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	if !loggerOk {
		logger.Error().Msg(api.ResponseErrLoggerContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	limit, ok := parseLimitParam(ctx, logger)
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

	products, productBulkErr := repos.Products.GetUserProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		if productBulkErr == errors.ErrInvalidUserData || productBulkErr == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{
				Message: "Unable to retrieve products. Please check your account.",
				Action:  "Ensure you are logged in with a valid household",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
		}
		return
	}
	ctx.JSON(http.StatusOK, products)
}

// GetProduct return a single product of a user
// @Summary      Returns a single product
// @Description  Returns a single product of user
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  database.Product
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/product/{id} [get]
func GetProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

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

	product, getError := repos.Products.GetProductByID(productID, userID)

	switch getError {
	// No error: return product
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database (mismatched userID in JWT <> DB)", productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(FmtProductNotFoundOrNoAccess, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	// Unspecified error
	default:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	// Get and parse body to product
	var product dbModel.Product
	if !bindJSON(ctx, logger, &product) {
		return
	}

	// Check for required parameters
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "barcode missing"})
		return
	}

	// Get OpenFoodFacts API controller from context
	offacntrl, offaErr := ctx.MustGet("offacntrl").(controllers.DatasetGetter)
	if !offaErr {
		logger.Error().Msg(MsgFailedToGetControllerFromContext)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: MsgFailedToGetControllerFromContext})
		return
	}
	// Get product data from API
	var apiProduct dbModel.Product
	apiProduct, err := offacntrl.GetDataset(product.Barcode)
	if err == nil {
		// Preserve request fields
		apiProduct.ScannedAt = time.Now()
		apiProduct.ExpireAt = product.ExpireAt
		apiProduct.Amount = product.Amount
		product = apiProduct
	}

	createResult := repos.Products.CreateProduct(userID, &product)
	if createResult != nil {
		logger.Error().Msgf("Error creating product: %s", createResult)
		ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
		return
	}

	go func() {
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEvent("product.created", map[string]any{
				"id":          product.ID,
				"productName": product.ProductName,
				"barcode":     product.Barcode,
				"expireAt":    product.ExpireAt,
				"amount":      product.Amount,
				"unit":        product.Unit,
				"householdId": product.HouseholdID,
			})
		}
	}()

	ctx.JSON(http.StatusCreated, product)
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
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id} [patch]
func UpdateProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

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

	// Get and parse body to product
	var product dbModel.ProductDTOPatch
	if !bindJSON(ctx, logger, &product) {
		return
	}

	updateErr := repos.Products.UpdateProduct(productID, userID, &product)

	switch updateErr {
	// No error => product was updated
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	// Requested product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}
}

// UpdateProductAmount updates the amount of a product by a given delta.
// If the resulting amount is <= 0, the product is hard-deleted.
// @Summary      	Update product amount
// @Description  	Applies a delta to a product's amount. Hard-deletes the product when amount reaches 0.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int							true  	"Product ID"
// @Param        	delta	body	api.ProductAmountDTO		true	"Amount delta"
// @Success      	200  {object}  database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id}/amount [patch]
func UpdateProductAmount(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

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

	// Get and parse body
	var amountDTO apiModel.ProductAmountDTO
	if !bindJSON(ctx, logger, &amountDTO) {
		return
	}

	deleted, updateErr := repos.Products.UpdateProductAmount(productID, userID, amountDTO.Delta)

	switch updateErr {
	case nil:
		if deleted {
			go func() {
				if ws := controllers.GetWebhookService(); ws != nil {
					ws.FireEvent("product.wasted", map[string]any{
						"productId": productID,
					})
				}
			}()
			ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d deleted (amount reached 0)", productID)})
		} else {
			ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d amount updated", productID)})
		}
		return
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		logger.Error().Msgf("Error updating product amount: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	productID, ok := parseUintPathParam(ctx, logger, "id")
	if !ok {
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

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	deleteResult := repos.Products.DeleteProduct(productID, userID, archiveOnly)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		ctx.JSON(http.StatusInternalServerError, api.DeleteFailedError())
		return
	} else {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was deleted", productID)})
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
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id}/expire [post]
func SetExpireAt(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

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

	// Get and parse body to timestamp
	var expireAt dbModel.Timestamp
	var bindErr error
	if bindErr = ctx.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	product, getErr := repos.Products.GetProductByID(productID, userID)
	if getErr != nil {
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	}

	updateErr := repos.Products.SetProductExpireAt(productID, userID, expireAt)

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
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database: %s", productID, updateErr)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(FmtProductNotFoundOrNoAccess, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}
}
