// v1 implements version 1 of the expiro API
package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"gitlab.com/Isotop7/expiro/api"
	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/configuration/static"
	"gitlab.com/Isotop7/expiro/models/database"

	jwt "github.com/appleboy/gin-jwt/v2"
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
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter limit
	limitParam := ctx.Query("limit")
	var limit int
	var parseError error
	if limit, parseError = strconv.Atoi(limitParam); parseError != nil {
		logger.Warn().Msgf("Invalid limit '%d' was specified", limit)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Limit '%d' is invalid", limit)})
		return
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
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
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
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("ID '%s' is invalid", idParam)})
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
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
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
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	}
}

// CreateProduct creates a new product of a user
// @Summary      	Creates a new product
// @Description  	Creates a new product of a user
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   path      int  true  "Product ID"
// @Param			product	body	database.Product	true	"Product"
// @Success      	201  {object}  database.Product
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/product/{id} [post]
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
	var product database.Product
	if err := ctx.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	// Check for required parameters
	// TODO: Do we need this or can we change struct annotation to required?
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "barcode missing"})
		return
	}

	// Get OpenFoodFacts API controller from context
	offacntrl, offaErr := ctx.MustGet("offacntrl").(controllers.OpenFoodFactsAPIController)
	if !offaErr {
		logger.Error().Msg("Failed to get controller from context")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to get controller from context"})
		return
	}
	// Get product data from API
	var apiProduct database.Product
	apiProduct, err := offacntrl.GetDataset(product.Barcode)
	if err == nil {
		product = apiProduct
	}

	// Create database controller
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
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
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("ID '%s' is invalid", idParam)})
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
	var product database.Product
	if err := ctx.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	// Check for required parameters
	// TODO: Do we need this or can we change struct annotation to required?
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "barcode missing"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
	// Update product in database
	updateErr := dbController.UpdateProduct(productID, userID, &product)

	switch updateErr {
	// No error => product was updates
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	// Requested product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' was not found", productID)})
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
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("ID '%s' is invalid", idParam)})
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
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
	// Delete product from database
	deleteResult := dbController.DeleteProduct(productID, userID)
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

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("ID '%s' is invalid", idParam)})
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
	var expireAt database.Timestamp
	var bindErr error
	if bindErr = ctx.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf("Error parsing body: %s", bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: bindErr.Error()})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
	product, getErr := dbController.GetProductByID(productID, userID)
	if getErr != nil {
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	}

	// Update expire date of product
	updateErr := dbController.SetProductExpireAt(productID, userID, expireAt)

	switch updateErr {
	// No error => product was updated and dto is returned
	case nil:
		expireDTO := database.ProductDTOExpire{
			ID:       product.ID,
			Barcode:  product.Barcode,
			ExpireAt: expireAt.Timestamp,
		}
		ctx.JSON(http.StatusOK, expireDTO)
		return
	// Product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("Product with id '%d' was not found", productID)})
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
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
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
