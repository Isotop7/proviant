package v1

import (
	"expiro/controllers"
	"expiro/errors"
	"expiro/models/configuration/static"
	"expiro/models/database"
	"fmt"
	"net/http"
	"strconv"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func GetProducts(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	limit := -1
	var parseError error
	if limitParam := c.Query("limit"); limitParam != "" {
		if limit, parseError = strconv.Atoi(limitParam); parseError != nil {
			logger.Warn().Msgf("Invalid limit '%d' was specified", limit)
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Limit '%d' is invalid", limit)})
			return
		}
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	products, productBulkErr := dbController.GetUserProductsBulk(userID, limit)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting products of user"})
		return
	} else {
		c.JSON(http.StatusOK, products)
	}
}

func GetProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")

	var convErr error
	var productID int
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	product, getError := dbController.GetProductByID(productID, userID)

	if getError != nil {
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	} else if getError == errors.ErrMismatcherUserID {
		logger.Error().Msgf("Product with ID '%d' for user was not found in database (mismatched userID in JWT <> DB)", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	} else {
		c.JSON(http.StatusOK, product)
		return
	}
}

func CreateProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims["id"].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	var product database.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"message": "barcode missing"})
		return
	}

	cntrl, ok := c.MustGet("cntrl").(controllers.OpenFoodFactsAPIController)
	if !ok {
		logger.Error().Msg("Failed to get controller from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get controller from context"})
		return
	}
	var apiProduct database.Product
	apiProduct, err := cntrl.GetDataset(product.Barcode)
	if err == nil {
		product = apiProduct
	}

	dbController := controllers.DatabaseController{DB: db}
	createResult := dbController.CreateProduct(userID, &product)
	if createResult != nil {
		logger.Error().Msgf("Error creating product: %s", createResult)
		c.JSON(http.StatusInternalServerError, gin.H{"message": createResult})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func UpdateProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")
	var productID int
	var convErr error

	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	var product database.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})

		return
	}

	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"message": "barcode missing"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	updateErr := dbController.UpdateProduct(productID, userID, &product)

	if updateErr == nil {
		c.JSON(http.StatusOK, product)
		return
	} else if updateErr == gorm.ErrRecordNotFound {
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	} else {
		logger.Error().Msgf("Error saving product: %s", updateErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": updateErr})
		return
	}
}

func DeleteProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")
	var productID int
	var convErr error

	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	deleteResult := dbController.DeleteProduct(productID, userID)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		c.JSON(http.StatusInternalServerError, gin.H{"message": deleteResult})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Product with ID '%d' was deleted", productID)})
	}
}

func SetExpireAt(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")
	var productID int
	var convErr error

	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("ID '%s' is invalid", idParam)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	var expireAt database.Timestamp
	var bindErr error
	if bindErr = c.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf("Error parsing body: %s", bindErr.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": bindErr.Error()})
		fmt.Println(bindErr.Error())
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	barcode, updateErr := dbController.SetProductExpireAt(productID, userID, expireAt)

	if updateErr == nil || barcode != "" {
		expireDTO := database.ProductDTOExpire{
			Barcode:  barcode,
			ExpireAt: expireAt.Timestamp,
		}
		c.JSON(http.StatusOK, expireDTO)
		return
	} else if updateErr == gorm.ErrRecordNotFound {
		logger.Error().Msgf("Product with ID '%d' was not found in database", productID)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", productID)})
		return
	} else if updateErr == errors.ErrMismatcherUserID {
		logger.Error().Msgf("Product with ID '%d' for user was not found in database: %s", productID, updateErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' for user was not found", productID)})
		return
	} else {
		logger.Error().Msgf("Error saving product: %s", updateErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": updateErr})
		return
	}
}

func GetExpired(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	claims := jwt.ExtractClaims(c)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg("Error getting user id from JWT token")
		c.JSON(http.StatusBadRequest, gin.H{"message": "Error getting user id from JWT token"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	products, getExpiredErr := dbController.GetProductsExpired(userID)

	if getExpiredErr != nil {
		logger.Error().Msgf("Error getting expired products: %s", getExpiredErr)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting expired products"})
		return
	} else {
		c.JSON(http.StatusOK, products)
	}
}
