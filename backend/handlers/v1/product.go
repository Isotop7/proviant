package v1

import (
	"expiro/backend/controllers"
	"expiro/backend/models"
	"fmt"
	"net/http"
	"strconv"

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
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Limit '%d' is invalid", limit)})
			return
		}
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	var products []models.Product
	if limit > 0 {
		db.Limit(limit).Find(&products)
	} else {
		db.Find(&products)
	}

	c.JSON(http.StatusOK, products)
}

func GetProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("ID '%s' is invalid", id)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	var product models.Product
	getError := db.First(&product, id)

	if getError.Error != nil || product.ID <= 0 {
		logger.Error().Msgf("Product with ID '%s' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Product with id '%s' was not found", id)})
		return
	}

	c.JSON(http.StatusOK, product)
}

func CreateProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		fmt.Println(err.Error())
		return
	}

	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"error": "barcode missing"})
		return
	}

	cntrl, ok := c.MustGet("cntrl").(controllers.OpenFoodFactsAPIController)
	if !ok {
		logger.Error().Msg("Failed to get controller from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get controller from context"})
		return
	}
	var apiProduct models.Product
	apiProduct, err := cntrl.GetDataset(db, product.Barcode)
	if err == nil {
		product = apiProduct
	}

	createResult := db.Create(&product)
	if createResult.Error != nil {
		logger.Error().Msgf("Error creating product: %s", createResult.Error.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": createResult.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func UpdateProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("ID '%s' is invalid", id)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		fmt.Println(err.Error())
		return
	}

	if product.Barcode == "" {
		logger.Error().Msgf("Body is missing barcode")
		c.JSON(http.StatusBadRequest, gin.H{"error": "barcode missing"})
		return
	}

	var dbProduct models.Product
	getError := db.First(&dbProduct, id)

	dbProduct.Barcode = product.Barcode
	dbProduct.ProductName = product.ProductName
	dbProduct.Categories = product.Categories
	dbProduct.Countries = product.Countries
	dbProduct.ImageURL = product.ImageURL
	dbProduct.ExpireAt = product.ExpireAt

	if getError.Error != nil || product.ID <= 0 {
		logger.Error().Msgf("Product with ID '%s' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Product with id '%s' was not found", id)})
		return
	}

	saveResult := db.Save(&dbProduct)
	if saveResult.Error != nil {
		logger.Error().Msgf("Error saving product: %s", saveResult.Error.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": saveResult.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, product)
}

func DeleteProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("ID '%s' is invalid", id)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	deleteResult := db.Delete(&models.Product{}, id)
	if deleteResult.Error != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult.Error.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": deleteResult.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": fmt.Sprintf("Product with ID '%s' was deleted", id)})
}

func SetExpireAt(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	id := c.Param("id")

	if _, err := strconv.Atoi(id); err != nil {
		logger.Warn().Msgf("Requested ID '%s' is invalid", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("ID '%s' is invalid", id)})
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database from context"})
		return
	}

	var expireAt models.Timestamp
	if err := c.ShouldBindJSON(&expireAt); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		fmt.Println(err.Error())
		return
	}

	var dbProduct models.Product
	getError := db.First(&dbProduct, id)

	if getError.Error != nil {
		logger.Error().Msgf("Product with ID '%s' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Product with id '%s' was not found", id)})
		return
	}

	dbProduct.ExpireAt = expireAt.Timestamp

	saveResult := db.Save(&dbProduct)
	if saveResult.Error != nil {
		logger.Error().Msgf("Error saving product: %s", saveResult.Error.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": saveResult.Error.Error()})
		return
	}

	expireDTO := models.ProductDTOExpire{
		Barcode:  dbProduct.Barcode,
		ExpireAt: dbProduct.ExpireAt,
	}

	c.JSON(http.StatusOK, expireDTO)
}
