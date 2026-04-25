// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	products, productBulkErr := productRepo.GetUserProductsBulk(userID, limit)
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

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
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
	product, getError := productRepo.GetProductByID(uint(productID), userID)

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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
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
		// Preserve request fields
		apiProduct.ScannedAt = time.Now()
		apiProduct.ExpireAt = product.ExpireAt
		apiProduct.Amount = product.Amount
		product = apiProduct
	}

	productRepo := database.NewProductRepository(dbHandle)
	createResult := productRepo.CreateProduct(userID, &product)
	if createResult != nil {
		logger.Error().Msgf("Error creating product: %s", createResult)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: createResult.Error()})
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
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id} [patch]
func UpdateProduct(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
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

	// Get and parse body to product
	var product dbModel.ProductDTOPatch
	if err := ctx.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	updateErr := productRepo.UpdateProduct(uint(productID), userID, &product)

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
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id}/amount [patch]
func UpdateProductAmount(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
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

	// Get and parse body
	var amountDTO apiModel.ProductAmountDTO
	if err := ctx.ShouldBindJSON(&amountDTO); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	deleted, updateErr := productRepo.UpdateProductAmount(uint(productID), userID, amountDTO.Delta)

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
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID)})
		return
	default:
		logger.Error().Msgf("Error updating product amount: %s", updateErr)
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

	productID, ok := parseIntParam(ctx, logger, "id")
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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	deleteResult := productRepo.DeleteProduct(uint(productID), userID, archiveOnly)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: deleteResult.Error()})
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
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id}/expire [post]
func SetExpireAt(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	productID, ok := parseIntParam(ctx, logger, "id")
	if !ok {
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

	// Get and parse body to timestamp
	var expireAt dbModel.Timestamp
	var bindErr error
	if bindErr = ctx.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: bindErr.Error()})
		return
	}

	productRepo := database.NewProductRepository(dbHandle)
	product, getErr := productRepo.GetProductByID(uint(productID), userID)
	if getErr != nil {
		logger.Error().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID)})
		return
	}

	updateErr := productRepo.SetProductExpireAt(uint(productID), userID, expireAt)

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
