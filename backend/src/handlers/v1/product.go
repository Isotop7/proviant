// v1 implements version 1 of the expiro backend API
package v1

import (
	"fmt"
	"net/http"
	"strconv"

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
// GET /api/v1/products
func GetProducts(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter limit
	limitParam := c.Query("limit")
	var limit int
	var parseError error
	if limit, parseError = strconv.Atoi(limitParam); parseError != nil {
		logger.Warn().Msgf("Invalid limit '%d' was specified", limit)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Limit '%d' is invalid", limit)})
		return
	}

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Get products of user from database with optional limit
	products, productBulkErr := dbController.GetUserProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting products of user"})
		return
	} else {
		c.JSON(http.StatusOK, products)
		return
	}
}

// GetProduct return a single product of a user
// GET /api/v1/product
func GetProduct(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := c.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Get product from database
	product, getError := dbController.GetProductByID(productID, userID)

	switch getError {
	// No error: return product
	case nil:
		c.JSON(http.StatusOK, product)
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database (mismatched userID in JWT <> DB)", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	}
}

// CreateProduct creates a new product of a user
// POST /api/v1/product/:id
func CreateProduct(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims["id"].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Get and parse body to product
	var product database.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Check for required parameters
	// TODO: Do we need this or can we change struct annotation to required?
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"message": "barcode missing"})
		return
	}

	// Get OpenFoodFacts API controller from context
	offacntrl, ok := c.MustGet("offacntrl").(controllers.OpenFoodFactsAPIController)
	if !ok {
		logger.Error().Msg("Failed to get controller from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get controller from context"})
		return
	}
	// Get product data from API
	var apiProduct database.Product
	apiProduct, err := offacntrl.GetDataset(product.Barcode)
	if err == nil {
		product = apiProduct
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Create product in database
	createResult := dbController.CreateProduct(userID, &product)
	if createResult != nil {
		logger.Error().Msgf("Error creating product: %s", createResult)
		c.JSON(http.StatusInternalServerError, gin.H{"message": createResult})
		return
	} else {
		c.JSON(http.StatusCreated, product)
		return
	}
}

// UpdateProduct updates a product of a user
// PATCH /api/v1/product/:id
func UpdateProduct(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := c.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Get and parse body to product
	var product database.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})

		return
	}

	// Check for required parameters
	// TODO: Do we need this or can we change struct annotation to required?
	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"message": "barcode missing"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Update product in database
	updateErr := dbController.UpdateProduct(productID, userID, &product)

	switch updateErr {
	// No error => product was updates
	case nil:
		c.JSON(http.StatusOK, product)
		return
	// Requested product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": updateErr})
		return
	}
}

// DeleteProduct deletes a product of a user
// DELETE /api/v1/product/:id
func DeleteProduct(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := c.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Delete product from database
	deleteResult := dbController.DeleteProduct(productID, userID)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		c.JSON(http.StatusInternalServerError, gin.H{"message": deleteResult})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Product with ID '%d' was deleted", productID)})
		return
	}
}

// SetExpireAt updates the expire date of a product of a user
// POST /api/v1/product/:id/expire
func SetExpireAt(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := c.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Get and parse body to timestamp
	var expireAt database.Timestamp
	var bindErr error
	if bindErr = c.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf("Error parsing body: %s", bindErr.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": bindErr.Error()})
		fmt.Println(bindErr.Error())
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	product, getErr := dbController.GetProductByID(productID, userID)
	if getErr != nil {
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
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
		c.JSON(http.StatusOK, expireDTO)
		return
	// Product was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	// User id from claims not matching user id of product in database
	case errors.ErrMismatcherUserID:
		logger.Error().Msgf("Product with ID '%d' for user was not found in database: %s", productID, updateErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving product: %s", updateErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": updateErr})
		return
	}
}

// GetExpired returns the list of all expired products of a user
// GET /api/v1/products/expired
func GetExpired(c *gin.Context) {
	// Get zerolog instance from context
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DB: db}
	// Get expired products of user from database
	products, getExpiredErr := dbController.GetProductsExpired(userID)

	// Check for error or return products
	if getExpiredErr != nil {
		logger.Error().Msgf("Error getting expired products: %s", getExpiredErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting expired products"})
		return
	} else {
		c.JSON(http.StatusOK, products)
		return
	}
}
