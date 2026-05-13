// v1 implements version 1 of the proviant API
package v1

import (
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
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
func GetProducts(ctx *gin.Context, appCtx *AppContext) {
	q, ok := ParseProductListQuery(ctx)
	if !ok {
		return
	}
	limit := q.Limit

	products, productBulkErr := appCtx.Repos.Products.GetUserProductsBulk(appCtx.UserID, limit)
	if productBulkErr != nil {
		appCtx.Logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
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
func GetProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	product, getError := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)

	switch getError {
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	case errors.ErrMismatcherUserID:
		appCtx.Logger.Warn().Msgf("Product with ID '%d' for user was not found in database (mismatched userID in JWT <> DB)", productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(FmtProductNotFoundOrNoAccess, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
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
func CreateProduct(ctx *gin.Context, appCtx *AppContext) {
	repos := appCtx.Repos
	userID := appCtx.UserID
	logger := appCtx.Logger

	var product dbModel.Product
	if !bindJSON(ctx, logger, &product) {
		return
	}

	if product.Barcode == "" {
		logger.Warn().Msgf("Body is missing barcode")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "barcode missing"})
		return
	}

	offacntrl, offaErr := ctx.MustGet("offacntrl").(controllers.DatasetGetter)
	if !offaErr {
		logger.Error().Msg(MsgFailedToGetControllerFromContext)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: MsgFailedToGetControllerFromContext})
		return
	}
	var apiProduct dbModel.Product
	apiProduct, err := offacntrl.GetDataset(product.Barcode)
	if err == nil {
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
func UpdateProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var product dbModel.ProductDTOPatch
	if !bindJSON(ctx, appCtx.Logger, &product) {
		return
	}

	updateErr := appCtx.Repos.Products.UpdateProduct(productID, appCtx.UserID, &product)

	switch updateErr {
	case nil:
		ctx.JSON(http.StatusOK, product)
		return
	case gorm.ErrRecordNotFound:
		appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		appCtx.Logger.Error().Msgf("Error saving product: %s", updateErr)
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
func UpdateProductAmount(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var amountDTO apiModel.ProductAmountDTO
	if !bindJSON(ctx, appCtx.Logger, &amountDTO) {
		return
	}

	deleted, updateErr := appCtx.Repos.Products.UpdateProductAmount(productID, appCtx.UserID, amountDTO.Delta)

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
		appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		appCtx.Logger.Error().Msgf("Error updating product amount: %s", updateErr)
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
func DeleteProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var archiveOnly bool
	var err error
	archiveOnlyParam, archiveOnlyParamExists := ctx.GetQuery("archiveOnly")
	if !archiveOnlyParamExists {
		archiveOnly = false
	} else {
		if archiveOnly, err = strconv.ParseBool(archiveOnlyParam); err != nil {
			appCtx.Logger.Warn().Msgf("Invalid archiveOnly '%s' was specified", archiveOnlyParam)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("archiveOnly '%s' is invalid", archiveOnlyParam)})
			return
		}
	}

	deleteResult := appCtx.Repos.Products.DeleteProduct(productID, appCtx.UserID, archiveOnly)
	if deleteResult != nil {
		appCtx.Logger.Error().Msgf("Error deleting product: %s", deleteResult)
		ctx.JSON(http.StatusInternalServerError, api.DeleteFailedError())
		return
	}
	ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product with ID '%d' was deleted", productID)})
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
func SetExpireAt(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var expireAt dbModel.Timestamp
	var err error
	if err = ctx.ShouldBindJSON(&expireAt); err != nil {
		appCtx.Logger.Warn().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	product, getErr := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	}

	updateErr := appCtx.Repos.Products.SetProductExpireAt(productID, appCtx.UserID, expireAt)

	switch updateErr {
	case nil:
		expireDTO := dbModel.ProductDTOExpire{
			ID:       product.ID,
			Barcode:  product.Barcode,
			ExpireAt: expireAt.Timestamp,
		}
		ctx.JSON(http.StatusOK, expireDTO)
		return
	case gorm.ErrRecordNotFound:
		appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	case errors.ErrMismatcherUserID:
		appCtx.Logger.Warn().Msgf("Product with ID '%d' for user was not found in database: %s", productID, updateErr)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(FmtProductNotFoundOrNoAccess, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		appCtx.Logger.Error().Msgf("Error saving product: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}
}
