// v1 implements version 1 of the proviant API
package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	fmtErrBulkCreateLocations = "BulkCreateProducts: GetByHousehold: %s"
	fmtErrBulkCreateWrite     = "BulkCreateProducts: CreateProductsBulk: %s"
	fmtErrBulkCreateActivity  = "BulkCreateProducts: activity log: %s"

	// fmtBulkCreateProductSummary names the aggregate activity feed entry.
	fmtBulkCreateProductSummary = "%d products added via batch scan"
)

// BulkCreateProducts creates multiple products queued by the batch scan mode
// @Summary      Create multiple products in one batch
// @Description  Creates multiple products from a batch scan queue. Every item is validated on its own; valid items are inserted in one transaction while rejected ones are reported with an index and reason. A missing product name is re-resolved from the Open Food Facts cache and, within a bounded lookup budget, live; an Open Food Facts miss creates the product with an empty name, matching single-create behaviour. Duplicates of an existing household barcode are allowed.
// @Tags         product
// @Accept       json
// @Produce      json
// @Param        request  body  apiModel.BulkCreateProductsAPIModel  true  "Batch items"
// @Success      200  {object}  apiModel.BulkCreateResponse
// @Failure      400  {object}  apiModel.BulkCreateResponse  "Returned when no item was created (invalid body, empty items, over cap, or every item failed)"
// @Failure      500  {object}  apiModel.BulkCreateResponse  "Returned when no item was created and a server-side error occurred"
// @Router       /api/v1/products/bulk [post]
func BulkCreateProducts(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger
	repos := appCtx.Repos
	userID := appCtx.UserID

	configValue, configOk := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, isConfig := configValue.(*configuration.ProviantConfiguration)
	if !configOk || !isConfig || proviantConfig == nil {
		logger.Error().Msg("proviant config not found in context")
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}

	var request apiModel.BulkCreateProductsAPIModel
	if !bindJSON(ctx, logger, &request) {
		return
	}

	response := apiModel.BulkCreateResponse{
		Errors: make([]apiModel.BulkCreateItemError, 0),
	}

	if len(request.Items) == 0 {
		response.Message = "no items were submitted"
		ctx.JSON(http.StatusBadRequest, response)
		return
	}
	if len(request.Items) > util.BulkCreateMaxItems {
		response.Message = fmt.Sprintf("a batch may contain at most %d items", util.BulkCreateMaxItems)
		ctx.JSON(http.StatusBadRequest, response)
		return
	}

	locations, locationErr := repos.StorageLocations.GetByHousehold(userID)
	if locationErr != nil {
		logger.Error().Msgf(fmtErrBulkCreateLocations, locationErr)
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}
	knownLocations := make(map[uint]bool, len(locations))
	for i := range locations {
		knownLocations[locations[i].ID] = true
	}

	// Only items without a client-provided name may consume an Open Food Facts
	// lookup, so the resolver is seeded with just those barcodes.
	var resolver *importNameResolver
	for i := range request.Items {
		if request.Items[i].ProductName == "" {
			barcodes := make([]string, 0, len(request.Items))
			for j := range request.Items {
				if request.Items[j].ProductName == "" {
					barcodes = append(barcodes, request.Items[j].Barcode)
				}
			}
			resolver = newImportNameResolver(repos, barcodes, importOffDatasetGetter(ctx), proviantConfig.OpenFoodFacts.CacheEnabled, logger)
			break
		}
	}

	products := make([]database.ImportedProduct, 0, len(request.Items))
	for i := range request.Items {
		item := &request.Items[i]
		product, reason := parseBulkCreateItem(item, knownLocations, resolver)
		if reason != "" {
			logger.Debug().Msgf("BulkCreateProducts: item %d rejected: %s", i, reason)
			response.Errors = append(response.Errors, apiModel.BulkCreateItemError{
				Index: i, Barcode: item.Barcode, Reason: reason,
			})
			continue
		}
		product.StorageLocationID = item.StorageLocationID
		products = append(products, database.ImportedProduct{Product: product})
	}

	response.Created = len(products)
	response.Failed = len(response.Errors)

	if len(products) == 0 {
		// Nothing was written. The client asked for a write that did not
		// happen, so this is a bad request rather than a partial success.
		response.Message = "no products were created"
		ctx.JSON(http.StatusBadRequest, response)
		return
	}

	if _, createErr := repos.Products.CreateProductsBulk(userID, products); createErr != nil {
		logger.Error().Msgf(fmtErrBulkCreateWrite, createErr)
		response.Created = 0
		response.Failed = len(request.Items)
		response.Message = "failed to write products"
		ctx.JSON(http.StatusInternalServerError, response)
		return
	}

	logBulkCreateActivity(repos, logger, userID, len(products))

	if response.Failed > 0 {
		response.Message = fmt.Sprintf("%d created, %d failed", response.Created, response.Failed)
	} else {
		response.Message = fmt.Sprintf("%d products created", response.Created)
	}
	ctx.JSON(http.StatusOK, response)
}

// parseBulkCreateItem converts one queued item into a product, or returns the
// reason it was rejected. An empty reason means the item is insertable. Unlike
// the CSV import there is no household-duplicate check: batch scan deliberately
// allows re-adding a barcode that is already on the shelf.
func parseBulkCreateItem(
	item *apiModel.BulkCreateProductItem,
	knownLocations map[uint]bool,
	resolver *importNameResolver,
) (dbModel.Product, string) {
	var product dbModel.Product

	reason := validateImportBarcode(&importRow{barcode: item.Barcode})
	if reason != "" {
		return product, reason
	}
	if item.Amount < 1 {
		return product, "amount must be at least 1"
	}
	if item.ExpireAt.IsZero() {
		return product, "expiry date is required"
	}
	if item.StorageLocationID != nil && !knownLocations[*item.StorageLocationID] {
		return product, "storage location does not belong to this household"
	}

	product = dbModel.Product{
		Barcode:     item.Barcode,
		ProductName: item.ProductName,
		ExpireAt:    item.ExpireAt,
		Amount:      item.Amount,
	}

	if product.ProductName == "" && resolver != nil {
		// An Open Food Facts miss is not an error here: single create also
		// stores the product with an empty name, so a blank reason string is
		// discarded and the barcode-only row is kept as-is.
		name, _ := resolver.resolve(item.Barcode)
		product.ProductName = name
	}

	return product, ""
}

// logBulkCreateActivity records the whole batch as a single feed entry, in the
// same fashion as the CSV import. The webhook is deliberately skipped for the
// same reason: per-product events would flood subscribers for a large batch.
func logBulkCreateActivity(repos *database.RepositoryContainer, logger *zerolog.Logger, userID uint, count int) {
	if repos.ActivityLogs == nil {
		return
	}
	go func() {
		householdID, householdErr := repos.Users.GetUserHouseholdByID(userID)
		if householdErr != nil {
			// An entry with HouseholdID 0 would be orphaned; dropping it beats
			// writing a feed row no feed ever renders.
			logger.Warn().Msgf(fmtErrBulkCreateActivity, householdErr)
			return
		}
		userName := ""
		if user, err := repos.Users.GetUserByID(userID); err == nil {
			userName = user.DisplayName
		}
		// ProductID stays 0: the feed links a row only when a product is
		// attached, and an aggregate entry has no single product.
		logEntry := &dbModel.ActivityLog{
			HouseholdID: householdID,
			UserID:      &userID,
			UserName:    userName,
			Action:      dbModel.ActivityActionAdd,
			ProductName: fmt.Sprintf(fmtBulkCreateProductSummary, count),
			Quantity:    count,
			Timestamp:   time.Now(),
		}
		ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repos.ActivityLogs.Create(ctxBg, logEntry); err != nil {
			logger.Warn().Msgf(fmtErrBulkCreateActivity, err)
		}
	}()
}
