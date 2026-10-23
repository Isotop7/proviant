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
// @Description  	Creates a new product of a user. The body carries the pack count and the size of one pack ("1 l"); a count below one is stored as one. The price is the price of one pack, so the savings of the product are that price times the number of packs. When the body carries a clientSyncId that already exists in the household, the existing product is returned with 200 instead of creating a duplicate. A clientSyncId held by another member's private product is answered with 409: the id is taken, but the row is not the caller's to read.
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

	// The pack count is optional on a create, so a count outside the range a
	// product may hold is stored as the nearest count that is. A patch
	// refuses such a count instead, because there the client said what it
	// meant.
	product.Quantity = util.ClampProductQuantity(product.Quantity)

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
		apiProduct.Quantity = product.Quantity
		// The barcode knows the size of a pack, but a client that named one
		// knows better.
		if product.PackSize != "" {
			apiProduct.PackSize = product.PackSize
		}
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
				"quantity":    product.Quantity,
				"householdId": product.HouseholdID,
			})
		}
	}()

	ctx.JSON(http.StatusCreated, apiModel.NewProductResponse(&product))
}

// updateMode is how a product patch is written. A merge keeps the stored
// value of a field the request did not carry, a replace clears it.
type updateMode bool

const (
	mergeFields   updateMode = false
	replaceFields updateMode = true
)

// UpdateProduct updates a product of a user
// @Summary      	Updates a product
// @Description  	Updates a product with new values. A field the request does not carry keeps its stored value; a pointer field sent as null is cleared. The pack count must be at least 1. The pack size is free text and may be set to an empty string to clear it. The price is the price of one pack, so the savings of the product are that price times the number of packs.
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
	updateProduct(ctx, appCtx, mergeFields)
}

// UpdateProductReplace replaces a product of a user
// @Summary      	Replaces a product
// @Description  	Replaces a product with new values. A field the request does not carry is cleared, so the request states the whole product. The pack count must be at least 1. The pack size is free text and may be set to an empty string to clear it. The price is the price of one pack, so the savings of the product are that price times the number of packs.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int					true  	"Product ID"
// @Param			product	body	database.ProductDTOPatch	true	"Product"
// @Success      	200  {object}  database.ProductDTOPatch
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id} [put]
func UpdateProductReplace(ctx *gin.Context, appCtx *AppContext) {
	updateProduct(ctx, appCtx, replaceFields)
}

// updateProduct writes a product patch. A merge keeps the stored value of a
// field the request did not carry; a replace clears it, so the request states
// the whole product.
func updateProduct(ctx *gin.Context, appCtx *AppContext, mode updateMode) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var product dbModel.ProductDTOPatch
	if !bindJSON(ctx, appCtx.Logger, &product) {
		return
	}
	if mode == replaceFields {
		product.MarkAllPresent()
	}

	if product.Has("quantity") {
		switch {
		case product.Quantity < util.MinProductQuantity:
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidProductQuantity)
			return
		case product.Quantity > util.MaxProductQuantity:
			api.RespondError(ctx, http.StatusBadRequest,
				fmt.Errorf("%w: %d", errors.ErrProductQuantityTooLarge, util.MaxProductQuantity))
			return
		}
	}

	updateErr := appCtx.Repos.Products.UpdateProduct(productID, appCtx.UserID, &product)

	switch updateErr {
	case nil:
		stored, readErr := appCtx.Repos.Products.GetProductByID(productID, appCtx.UserID)
		if readErr != nil {
			ctx.JSON(http.StatusOK, product)
			return
		}
		response := productResponse(&stored, &product)
		// A request that carried the pack size decided it, even when it
		// decided to clear it: the cache must not put it back.
		if !product.Has("packSize") {
			if packSize := fillCachedPackSize(appCtx, productID, stored.Barcode, stored.PackSize); packSize != "" {
				response.PackSize = packSize
			}
		}
		ctx.JSON(http.StatusOK, response)
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

// productResponse answers a patch with the stored product, not the request
// body: a field the request did not carry keeps its stored value, so echoing
// the request would report a count of zero for a name-only patch. A product
// that cannot be read falls back to the request body.
func productResponse(stored *dbModel.Product, patch *dbModel.ProductDTOPatch) dbModel.ProductDTOPatch {
	if stored == nil {
		return *patch
	}
	return dbModel.ProductDTOPatch{
		ID:                   stored.ID,
		ProductName:          stored.ProductName,
		Categories:           stored.Categories,
		Countries:            stored.Countries,
		ImageURL:             stored.ImageURL,
		ExpireAt:             stored.ExpireAt,
		Quantity:             stored.Quantity,
		PackSize:             stored.PackSize,
		StorageLocationID:    stored.StorageLocationID,
		NotificationLeadDays: stored.NotificationLeadDays,
		MinStockAmount:       stored.MinStockAmount,
		IsPrivate:            stored.IsPrivate,
		PriceOverride:        stored.PriceOverride,
		OpenedAt:             stored.OpenedAt,
		DaysAfterOpening:     stored.DaysAfterOpening,
	}
}

// fillCachedPackSize copies the pack size of the barcode's Open Food Facts
// cache entry onto a product that does not have one yet. It returns the pack
// size it stored, so the answer can report it. The lookup reads the cache
// only: a live request per edit would turn every save into an outbound call,
// and the pack size is display metadata. The backfill controller owns the
// live lookups.
func fillCachedPackSize(appCtx *AppContext, productID uint, barcode, currentPackSize string) string {
	if currentPackSize != "" || barcode == "" {
		return ""
	}
	cached, cacheErr := appCtx.Repos.Products.GetOpenFoodFactsCacheByBarcode(barcode)
	if cacheErr != nil || cached.PackSize == "" {
		return ""
	}
	if setErr := appCtx.Repos.Products.SetProductPackSize(productID, cached.PackSize); setErr != nil {
		appCtx.Logger.Warn().Msgf("Error storing the pack size of product %d: %s", productID, setErr)
		return ""
	}
	return cached.PackSize
}

// UpdateProductAmount updates the pack count of a product by a given delta.
//
// Deprecated: the endpoint moved to /quantity. The old path stays as an alias
// so a client that has not migrated keeps working.
//
// @Summary      	Update product quantity (deprecated alias of /quantity)
// @Description  	Deprecated alias of PATCH /api/v1/products/{id}/quantity. Applies a delta to a product's pack count. Hard-deletes the product when the count reaches 0.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int							true  	"Product ID"
// @Param        	delta	body	api.ProductQuantityDTO		true	"Quantity delta"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id}/amount [patch]
func UpdateProductAmount(ctx *gin.Context, appCtx *AppContext) {
	UpdateProductQuantity(ctx, appCtx)
}

// UpdateProductQuantity updates the pack count of a product by a given delta.
// If the resulting count is <= 0, the product is hard-deleted.
// @Summary      	Update product quantity
// @Description  	Applies a delta to a product's pack count. Hard-deletes the product when the count reaches 0. A delta outside the range a pack count may span is refused, and the count never leaves that range.
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Param        	id   	path	int							true  	"Product ID"
// @Param        	delta	body	api.ProductQuantityDTO		true	"Quantity delta"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	404  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/{id}/quantity [patch]
func UpdateProductQuantity(ctx *gin.Context, appCtx *AppContext) {
	productID, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var quantityDTO apiModel.ProductQuantityDTO
	if !bindJSON(ctx, appCtx.Logger, &quantityDTO) {
		return
	}
	// A delta larger than the whole range a pack count may span is refused
	// instead of being applied: it cannot change the count by more than the
	// count's own range, and a delta that large overflows the column on a
	// database with a 32-bit integer.
	if quantityDTO.Delta > util.MaxProductQuantity || quantityDTO.Delta < -util.MaxProductQuantity {
		appCtx.Logger.Warn().Msgf("Body carries an out-of-range quantity delta: %d", quantityDTO.Delta)
		api.RespondError(ctx, http.StatusBadRequest,
			fmt.Errorf("%w: %d", errors.ErrProductQuantityDeltaOutOfRange, quantityDTO.Delta))
		return
	}
	if quantityDTO.ClientMutationID != nil {
		trimmed := strings.TrimSpace(*quantityDTO.ClientMutationID)
		if len(trimmed) > util.OfflineSyncIDMaxLength {
			appCtx.Logger.Warn().Msg("Body carries an over-long clientMutationId")
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrClientMutationIDTooLong)
			return
		}
		// Trimmed here so the token stored and the token replayed compare
		// equal; an all-whitespace id becomes nil and never dedupes.
		quantityDTO.ClientMutationID = &trimmed
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
		appCtx.Logger.Error().Msgf("Error fetching product before quantity update: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.UpdateFailedError())
		return
	}

	deleted, applied, updateErr := appCtx.Repos.Products.UpdateProductQuantity(productID, appCtx.UserID, quantityDTO.Delta, quantityDTO.ClientMutationID)

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
				quantity := quantityDTO.Delta
				if quantity < 0 {
					quantity = -quantity
				}
				logEntry := &dbModel.ActivityLog{
					HouseholdID: householdID,
					UserID:      &appCtx.UserID,
					UserName:    userName,
					Action:      dbModel.ActivityActionQuantityChange,
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
			ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d deleted (quantity reached 0)", productID)})
			return
		}
		ctx.JSON(http.StatusOK, api.APIResponse{Message: fmt.Sprintf("Product %d quantity updated", productID)})
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
		appCtx.Logger.Error().Msgf("Error updating product quantity: %s", updateErr)
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
