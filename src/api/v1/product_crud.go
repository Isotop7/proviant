// v1 implements version 1 of the proviant API
package v1

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	MsgCheckProductIdTryAgain           = "Check the product ID and try again"
	MsgFailedToGetControllerFromContext = "Failed to get controller from context"
)

// GetProducts returns the products of a user
// @Summary      Return a list of products
// @Description  Return a list of products of user
// @Tags         product
// @Produce      json
// @Success      200  {object}  []apiModel.ProductResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products [get]
func GetProducts(ctx *gin.Context, appCtx *AppContext) {
	q, ok := ParseProductListQuery(ctx)
	if !ok {
		return
	}
	limit := q.Limit

	if len(q.IDs) > 0 {
		products, idsErr := appCtx.Repos.Products.GetUserProductsByIDs(appCtx.UserID, q.IDs)
		if idsErr != nil {
			appCtx.Logger.Error().Msgf("Error getting products by IDs for user: %s", idsErr)
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		ctx.JSON(http.StatusOK, apiModel.NewProductResponses(products))
		return
	}

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
	ctx.JSON(http.StatusOK, apiModel.NewProductResponses(products))
}

// GetProduct return a single product of a user
// @Summary      Returns a single product
// @Description  Returns a single product of user
// @Tags         product
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  apiModel.ProductResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/{id} [get]
func GetProduct(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	product, getError := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)

	switch getError {
	case nil:
		ctx.JSON(http.StatusOK, apiModel.NewProductResponse(&product))
		return
	case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
		// Unknown and foreign ids fall through to the same body on purpose:
		// distinct messages would let a caller probe which sequential ids
		// exist in other households.
		appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, getError)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	default:
		// Anything else is a server-side failure (DB down, user row gone);
		// reporting it as 404 would hide it from monitoring.
		appCtx.Logger.Error().Msgf("Error fetching product: %s", getError)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
}

// syncIDVisibleTo reports whether the product a clientSyncId resolved to may be
// answered to userID. GetProductBySyncID scopes by household only, and private
// products are owner-only, so a known sync id must not turn into a read of
// another member's row.
func syncIDVisibleTo(existing *dbModel.Product, userID uint) bool {
	return !existing.IsPrivate || existing.UserID == userID
}

// CreateProduct creates a new product of a user
// @Summary      	Creates a new product
// @Description  	Creates a new product of a user. When the body carries a clientSyncId that already exists in the household, the existing product is returned with 200 instead of creating a duplicate. A clientSyncId held by another member's private product is answered with 409: the id is taken, but the row is not the caller's to read.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param			product	body	database.Product	true	"Product"
// @Success      	201  {object}  apiModel.ProductResponse
// @Success      	200  {object}  apiModel.ProductResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	409  {object}  api.APIResponse
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

	var clientSyncID string
	if product.ClientSyncID != nil {
		clientSyncID = strings.TrimSpace(*product.ClientSyncID)
	}
	if len(clientSyncID) > util.OfflineSyncIDMaxLength {
		logger.Warn().Msgf("Body carries an over-long clientSyncId (%d bytes)", len(clientSyncID))
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrClientSyncIDTooLong)
		return
	}
	if clientSyncID == "" {
		product.ClientSyncID = nil
	} else {
		// Stored trimmed, so a replayed padded id resolves to the same row
		// instead of missing the lookup and colliding on the unique index.
		product.ClientSyncID = &clientSyncID
	}

	if clientSyncID != "" {
		existing, syncErr := repos.Products.GetProductBySyncID(userID, clientSyncID)
		switch {
		case syncErr == nil && syncIDVisibleTo(&existing, userID):
			ctx.JSON(http.StatusOK, apiModel.NewProductResponse(&existing))
			return
		case syncErr == nil:
			// The id is taken by a row this caller may not see, and the unique
			// index on (household, sync id) forbids a second one: refuse rather
			// than fall through to an insert that would fail.
			logger.Warn().Msg("clientSyncId collides with another member's private product")
			api.RespondError(ctx, http.StatusConflict, errors.ErrClientSyncIDConflict)
			return
		case !stderrors.Is(syncErr, gorm.ErrRecordNotFound):
			logger.Error().Msgf("Error looking up product by client sync id: %s", syncErr)
			ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
			return
		}
	}

	if product.Barcode == "" {
		logger.Warn().Msgf("Body is missing barcode")
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBarcodeMissing)
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
		apiProduct.ClientSyncID = product.ClientSyncID
		product = apiProduct
	}

	createResult := repos.Products.CreateProduct(userID, &product)
	if createResult != nil {
		// Client-supplied OpenedAt/DaysAfterOpening failing validation is a
		// bad request, not a server fault.
		if createResult == errors.ErrInvalidOpenedLifecycle {
			logger.Warn().Msgf("Rejected product create: %s", createResult)
			ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail(createResult.Error()))
			return
		}
		// A concurrent retry of the same clientSyncId loses the race to the
		// unique index instead of reaching the lookup above — report the row
		// that won rather than a 500 for a create that did happen. The lookup
		// also runs on create errors that are not that race: telling a unique
		// violation from a dead connection needs driver-specific error
		// parsing, and one extra failing query on the 500 path is the price.
		// ErrInvalidOpenedLifecycle has already returned, and the validation
		// above it rejects before any insert.
		if clientSyncID != "" {
			existing, lookupErr := repos.Products.GetProductBySyncID(userID, clientSyncID)
			if lookupErr == nil && !syncIDVisibleTo(&existing, userID) {
				logger.Warn().Msg("clientSyncId collides with another member's private product")
				api.RespondError(ctx, http.StatusConflict, errors.ErrClientSyncIDConflict)
				return
			}
			if lookupErr == nil {
				ctx.JSON(http.StatusOK, apiModel.NewProductResponse(&existing))
				return
			}
		}
		logger.Error().Msgf("Error creating product: %s", createResult)
		ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
		return
	}

	go func() {
		householdID, _ := repos.Users.GetUserHouseholdByID(userID)
		userName := ""
		if user, err := repos.Users.GetUserByID(userID); err == nil {
			userName = user.DisplayName
		}
		logEntry := &dbModel.ActivityLog{
			HouseholdID: householdID,
			UserID:      &userID,
			UserName:    userName,
			Action:      dbModel.ActivityActionAdd,
			ProductID:   product.ID,
			ProductName: product.ProductName,
			Quantity:    1,
			Timestamp:   time.Now(),
		}
		if repos.ActivityLogs == nil {
			return
		}
		ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repos.ActivityLogs.Create(ctxBg, logEntry)
	}()

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

	ctx.JSON(http.StatusCreated, apiModel.NewProductResponse(&product))
}

// UpdateProduct updates a product of a user
// @Summary      	Updates a product
// @Description  	Updates a product with new values
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Param			product	body	database.ProductDTOPatch	true	"Product"
// @Success      	200  {object}  database.ProductDTOPatch
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id} [patch]
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
	case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
		// One body for unknown and foreign ids alike: distinct messages
		// would let a caller probe which sequential ids exist in other
		// households.
		appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, updateErr)
		ctx.JSON(http.StatusNotFound, api.APIResponse{
			Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
			Action:  MsgCheckProductIdTryAgain,
		})
		return
	case errors.ErrInvalidOpenedLifecycle:
		// validateOpenedLifecycle rejected client-supplied OpenedAt /
		// DaysAfterOpening — user input, not a server fault.
		appCtx.Logger.Warn().Msgf("Rejected product update for %d: %s", productID, updateErr)
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail(updateErr.Error()))
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
// @Success      	200  {object}  api.APIResponse
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
	if amountDTO.ClientMutationID != nil {
		trimmed := strings.TrimSpace(*amountDTO.ClientMutationID)
		if len(trimmed) > util.OfflineSyncIDMaxLength {
			appCtx.Logger.Warn().Msg("Body carries an over-long clientMutationId")
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrClientMutationIDTooLong)
			return
		}
		// Trimmed here so the token stored and the token replayed compare
		// equal; an all-whitespace id becomes nil and never dedupes.
		amountDTO.ClientMutationID = &trimmed
	}

	product, err := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound || err == errors.ErrMismatcherUserID {
			appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
			ctx.JSON(http.StatusNotFound, api.APIResponse{
				Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
				Action:  MsgCheckProductIdTryAgain,
			})
			return
		}
		appCtx.Logger.Error().Msgf("Error fetching product before amount update: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}

	deleted, applied, updateErr := appCtx.Repos.Products.UpdateProductAmount(productID, appCtx.UserID, amountDTO.Delta, amountDTO.ClientMutationID)

	switch updateErr {
	case nil:
		// A replayed mutation id changed nothing, so it owes no activity entry,
		// no webhook and no deletion of its own.
		if applied {
			householdID, _ := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
			userName := ""
			if user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID); err == nil {
				userName = user.DisplayName
			}
			go func() {
				quantity := amountDTO.Delta
				if quantity < 0 {
					quantity = -quantity
				}
				logEntry := &dbModel.ActivityLog{
					HouseholdID: householdID,
					UserID:      &appCtx.UserID,
					UserName:    userName,
					Action:      dbModel.ActivityActionAmountChange,
					ProductID:   productID,
					ProductName: product.ProductName,
					Quantity:    quantity,
					Timestamp:   time.Now(),
				}
				if appCtx.Repos.ActivityLogs == nil {
					return
				}
				ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = appCtx.Repos.ActivityLogs.Create(ctxBg, logEntry)
			}()
		}
		if applied && deleted {
			go func() {
				if ws := controllers.GetWebhookService(); ws != nil {
					ws.FireEvent("product.wasted", map[string]any{
						"productId": productID,
					})
				}
			}()
			ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d deleted (amount reached 0)", productID)})
			return
		}
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d amount updated", productID)})
		return
	case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
		// One body for unknown and foreign ids alike: distinct messages
		// would let a caller probe which sequential ids exist in other
		// households.
		appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, updateErr)
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
// @Router       	/api/v1/products/{id} [delete]
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

	if err := appCtx.Products.DeleteProduct(productID, appCtx.UserID, archiveOnly); err != nil {
		// A missing or foreign product is the caller's problem (404), not a
		// server fault — the default branch used to report both as 500.
		switch err {
		case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
			// One body for unknown and foreign ids alike: distinct messages
			// would let a caller probe which sequential ids exist in other
			// households.
			appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, err)
			ctx.JSON(http.StatusNotFound, api.APIResponse{
				Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
				Action:  MsgCheckProductIdTryAgain,
			})
			return
		default:
			appCtx.Logger.Error().Msgf("Error deleting product: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.DeleteFailedError())
			return
		}
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
// @Router       	/api/v1/products/{id}/expire [post]
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
		if getErr == gorm.ErrRecordNotFound || getErr == errors.ErrMismatcherUserID {
			appCtx.Logger.Warn().Msgf(errors.FormatProductNotFound, productID)
			ctx.JSON(http.StatusNotFound, api.APIResponse{
				Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, productID),
				Action:  MsgCheckProductIdTryAgain,
			})
			return
		}
		appCtx.Logger.Error().Msgf("Error fetching product before expiry update: %s", getErr)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
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
	case gorm.ErrRecordNotFound, errors.ErrMismatcherUserID:
		// One body for unknown and foreign ids alike: distinct messages
		// would let a caller probe which sequential ids exist in other
		// households.
		appCtx.Logger.Warn().Msgf("Product with ID '%d' not accessible: %s", productID, updateErr)
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
