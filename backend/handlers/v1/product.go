package v1

import (
	"expiro/backend/controllers"
	"expiro/backend/models/database"
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

	dbController := controllers.DatabaseController{DB: db}
	products := dbController.GetProductsBulk(limit)

	c.JSON(http.StatusOK, products)
}

func GetProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")

	var id int
	var convErr error
	if id, convErr = strconv.Atoi(idParam); convErr != nil {
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

	dbController := controllers.DatabaseController{DB: db}
	product, getError := dbController.GetProductByID(id)

	if getError != nil {
		logger.Error().Msgf("Product with ID '%d' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", id)})
		return
	}

	c.JSON(http.StatusOK, product)
}

func CreateProduct(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
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
	createResult := dbController.CreateProduct(&product)
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
	var id int
	var convErr error

	if id, convErr = strconv.Atoi(idParam); convErr != nil {
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
	updateErr := dbController.UpdateProduct(&product, id)

	if updateErr == nil {
		c.JSON(http.StatusOK, product)
		return
	} else if updateErr == gorm.ErrRecordNotFound {
		logger.Error().Msgf("Product with ID '%d' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", id)})
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
	var id int
	var convErr error

	if id, convErr = strconv.Atoi(idParam); convErr != nil {
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

	dbController := controllers.DatabaseController{DB: db}
	deleteResult := dbController.DeleteProduct(id)
	if deleteResult != nil {
		logger.Error().Msgf("Error deleting product: %s", deleteResult)
		c.JSON(http.StatusInternalServerError, gin.H{"message": deleteResult})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Product with ID '%d' was deleted", id)})
	}
}

func SetExpireAt(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	idParam := c.Param("id")
	var id int
	var convErr error

	if id, convErr = strconv.Atoi(idParam); convErr != nil {
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

	var expireAt database.Timestamp
	var bindErr error
	if bindErr = c.ShouldBindJSON(&expireAt); bindErr != nil {
		logger.Error().Msgf("Error parsing body: %s", bindErr.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": bindErr.Error()})
		fmt.Println(bindErr.Error())
		return
	}

	dbController := controllers.DatabaseController{DB: db}
	barcode, updateErr := dbController.SetProductExpireAt(id, expireAt)

	if updateErr == nil || barcode != "" {
		expireDTO := database.ProductDTOExpire{
			Barcode:  barcode,
			ExpireAt: expireAt.Timestamp,
		}
		c.JSON(http.StatusOK, expireDTO)
		return
	} else if updateErr == gorm.ErrRecordNotFound {
		logger.Error().Msgf("Product with ID '%d' was not found in database", id)
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Product with id '%d' was not found", id)})
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

	dbController := controllers.DatabaseController{DB: db}
	products := dbController.GetProductsExpired()
	c.JSON(http.StatusOK, products)
}
