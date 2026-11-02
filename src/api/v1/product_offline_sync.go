package v1

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SyncOfflineProducts creates queued mobile entries idempotently.
// @Summary       Sync offline product entries
// @Description   Creates queued drafts. Replayed clientSyncId values return the existing product. The queue is insert-only: a queued draft that duplicates a row of the household — the same barcode in the same storage location with the same effective expiry — is written as a row of its own, because the sync runs unattended and a duplicate cannot be answered with a question.
// @Tags          product
// @Accept        json
// @Produce       json
// @Param         items  body  apiModel.OfflineSyncRequest  true  "Queued drafts"
// @Success       200  {object}  apiModel.OfflineSyncResponse
// @Failure       400  {object}  api.APIResponse
// @Failure       500  {object}  api.APIResponse
// @Router        /api/v1/products/sync-offline [post]
func SyncOfflineProducts(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	var req apiModel.OfflineSyncRequest
	// The body cap is configuration, not a constant: the outbox
	// client splits its batches at the value injected into the page,
	// so both sides move together instead of drifting apart on the
	// next operator change.
	maxBodyBytes := util.ReceiptBulkMaxBodyBytes
	if syncConfig := offlineSyncConfig(ctx); syncConfig != nil && syncConfig.MaxBatchBytes > 0 {
		maxBodyBytes = syncConfig.MaxBatchBytes
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, int64(maxBodyBytes))
	if bindErr := ctx.ShouldBindJSON(&req); bindErr != nil {
		// An over-sized batch is a transport refusal, not a malformed draft:
		// reporting it as a parse failure sends the outbox down the
		// permanent-failure path and parks every item in it.
		if importBodyTooLarge(bindErr) {
			logger.Warn().Msgf("Offline sync: request body too large: %s", bindErr)
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		logger.Error().Msgf("%s: %s", errors.FormatGenericError, bindErr.Error())
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrParseBody)
		return
	}
	if len(req.Items) == 0 {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBulkCreateItemsMissing)
		return
	}
	if len(req.Items) > util.ReceiptBulkMaxItems {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("too many items (max %d)", util.ReceiptBulkMaxItems)})
		return
	}

	validLocationIDs := make(map[uint]bool)
	locations, locErr := appCtx.Repos.StorageLocations.GetByHousehold(appCtx.UserID)
	if locErr != nil {
		logger.Error().Msgf("Offline sync: storage location lookup failed: %s", locErr)
	} else {
		for i := range locations {
			validLocationIDs[locations[i].ID] = true
		}
	}

	bulkActorInfo := bulkActor{userID: appCtx.UserID}
	householdID, householdErr := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if householdErr != nil {
		logger.Warn().Msgf("Offline sync: activity log disabled, household lookup failed: %s", householdErr)
	} else {
		bulkActorInfo.householdID = householdID
		bulkActorInfo.householdKnown = true
	}
	if user, userErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID); userErr == nil {
		bulkActorInfo.userName = user.DisplayName
	}

	// The canonical categories and the household's shelf-life defaults are read
	// once for the whole batch, so the category of every queued item is
	// checked against the same set.
	categories := loadBatchCategories(appCtx)

	// The fallback name below is written into the row and outlives this request, so
	// it is resolved once for the whole batch — and it is resolved in the language
	// active at sync time, not at render time, so a user who switches language
	// afterwards keeps the name they queued under.
	userLanguage, householdLanguage, _ := appCtx.Repos.Users.GetUserDisplayPreferencesByID(appCtx.UserID)
	lang := i18n.ResolveLanguage(userLanguage, householdLanguage, ctx.GetHeader("Accept-Language"))

	names := newImportNameResolver(appCtx.Repos, unnamedBarcodes(req.Items), importOffDatasetGetter(ctx), offCacheEnabled(ctx, logger), logger)

	results := make([]apiModel.OfflineSyncResult, 0, len(req.Items))
	createdProducts := make([]*dbModel.Product, 0, len(req.Items))
	for i := range req.Items {
		draft := &req.Items[i]
		clientSyncID := strings.TrimSpace(draft.ClientSyncID)
		if clientSyncID == "" {
			results = append(results, terminalSyncResult(i, "", "client sync id is missing"))
			continue
		}
		if len(clientSyncID) > util.OfflineSyncIDMaxLength {
			results = append(results, terminalSyncResult(i, truncateDraftField(clientSyncID, util.OfflineSyncIDEchoLength), "client sync id is too long"))
			continue
		}

		existing, syncErr := appCtx.Repos.Products.GetProductBySyncID(appCtx.UserID, clientSyncID)
		if syncErr == nil {
			// GetProductBySyncID scopes by household only, so a private product
			// owned by another member resolves here too. Reporting it as synced
			// would drop this caller's queued product without ever creating it.
			if !syncIDVisibleTo(&existing, appCtx.UserID) {
				logger.Warn().Msg("Offline sync: client sync id belongs to another member's private product")
				results = append(results, terminalSyncResult(i, clientSyncID, "client sync id belongs to another product"))
				continue
			}
			pid := existing.ID
			results = append(results, apiModel.OfflineSyncResult{Index: i, ClientSyncID: clientSyncID, Status: apiModel.OfflineSyncStatusAlreadySynced, ProductID: &pid})
			continue
		}
		// Any lookup failure, not just a clean miss, falls through to the create
		// attempt below and is only reported as a failure if that finds nothing
		// either. Marking the item failed on the first transient database error
		// would poison a queued create the outbox would otherwise retry.
		if !stderrors.Is(syncErr, gorm.ErrRecordNotFound) {
			logger.Error().Msgf("Offline sync: lookup failed, falling through to create: %s", syncErr)
		}

		if strings.TrimSpace(draft.ProductName) == "" && strings.TrimSpace(draft.Barcode) != "" {
			// One resolver for the whole batch: it answers from the Open Food
			// Facts cache first and spends a bounded number of live lookups, so a
			// full backlog of barcode-only drafts cannot turn one request into
			// util.ReceiptBulkMaxItems sequential calls to the external API.
			barcode := strings.TrimSpace(draft.Barcode)
			if fields, reason := names.resolve(barcode); reason == "" {
				draft.ProductName = fields.productName
				if strings.TrimSpace(draft.Categories) == "" {
					draft.Categories = fields.categories
				}
			}
		}
		if strings.TrimSpace(draft.ProductName) == "" {
			if barcode := strings.TrimSpace(draft.Barcode); barcode != "" {
				draft.ProductName = i18n.T(lang, "products.offlineFallbackName", barcode)
			} else {
				draft.ProductName = i18n.T(lang, "products.offlineFallbackNameNoBarcode")
			}
		}

		product, result := bulkCreateItem(appCtx, i, &draft.BulkProductDraft, validLocationIDs, clientSyncID, categories)
		if result.Status != apiModel.BulkItemStatusCreated {
			// A retry may race with another worker that already inserted this
			// client sync id; report the existing row instead of a failure.
			//
			// Only for a failure a retry can fix. A terminal one is a property of
			// the draft — an unknown location, a rejected price, a blank name —
			// and no other worker inserted anything, so the lookup can only ever
			// come back empty; it would cost a query on the failures this endpoint
			// sees most of.
			if !result.Terminal {
				existing, lookupErr := appCtx.Repos.Products.GetProductBySyncID(appCtx.UserID, clientSyncID)
				if lookupErr == nil && syncIDVisibleTo(&existing, appCtx.UserID) {
					pid := existing.ID
					results = append(results, apiModel.OfflineSyncResult{Index: i, ClientSyncID: clientSyncID, Status: apiModel.OfflineSyncStatusAlreadySynced, ProductID: &pid})
					continue
				}
			}
			results = append(results, apiModel.OfflineSyncResult{Index: i, ClientSyncID: clientSyncID, Status: apiModel.OfflineSyncStatusFailed, Reason: result.Reason, Terminal: result.Terminal})
			continue
		}
		pid := product.ID
		results = append(results, apiModel.OfflineSyncResult{Index: i, ClientSyncID: clientSyncID, Status: apiModel.OfflineSyncStatusCreated, ProductID: &pid})
		createdProducts = append(createdProducts, product)
	}

	if len(createdProducts) > 0 {
		bulkSideEffectBatches.Add(1)
		go func() {
			defer bulkSideEffectBatches.Add(-1)
			defer func() {
				if r := recover(); r != nil {
					appCtx.Logger.Error().Msgf("Offline sync: side-effect worker panicked: %v", r)
				}
			}()
			for _, product := range createdProducts {
				select {
				case bulkSideEffectSlots <- struct{}{}:
				case <-time.After(bulkSideEffectSlotWait):
					dropped := bulkSideEffectsDropped.Add(1)
					appCtx.Logger.Error().
						Int("items", len(createdProducts)).
						Int64("batchesDroppedTotal", dropped).
						Msg("Offline sync: no side-effect worker slot free, dropping activity log and webhooks")
					return
				}
				func() {
					defer func() { <-bulkSideEffectSlots }()
					logBulkActivity(appCtx, product, bulkActorInfo)
					fireBulkProductCreated(appCtx, product)
				}()
			}
		}()
	}

	ctx.JSON(http.StatusOK, apiModel.OfflineSyncResponse{Results: results})
}

// terminalSyncResult is a failed item whose reason cannot be changed by
// resubmitting the queued draft, so the outbox should hand it to the user
// instead of replaying it into the same failure.
func terminalSyncResult(index int, clientSyncID, reason string) apiModel.OfflineSyncResult {
	return apiModel.OfflineSyncResult{
		Index: index, ClientSyncID: clientSyncID,
		Status: apiModel.OfflineSyncStatusFailed, Reason: reason, Terminal: true,
	}
}

// unnamedBarcodes collects every distinct barcode whose draft still needs a name
// from Open Food Facts. The resolver loads its cache with one query keyed by this
// list, so it has to be complete before the item loop starts.
func unnamedBarcodes(items []apiModel.OfflineSyncDraft) []string {
	seen := make(map[string]bool, len(items))
	barcodes := make([]string, 0, len(items))
	for i := range items {
		if strings.TrimSpace(items[i].ProductName) != "" {
			continue
		}
		barcode := strings.TrimSpace(items[i].Barcode)
		if barcode == "" || seen[barcode] {
			continue
		}
		seen[barcode] = true
		barcodes = append(barcodes, barcode)
	}
	return barcodes
}

// offlineSyncConfig resolves the queued-sync configuration the
// request runs under, or nil when no config reached the context —
// the caller then falls back to the built-in defaults.
func offlineSyncConfig(ctx *gin.Context) *configuration.OfflineSyncConfiguration {
	value, ok := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, isConfig := value.(*configuration.ProviantConfiguration)
	if !ok || !isConfig || proviantConfig == nil {
		return nil
	}
	return &proviantConfig.OfflineSync
}

// offCacheEnabled reports whether Open Food Facts caching is on. A config that
// is missing or malformed resolves every name live rather than failing the
// whole sync — a cache miss costs a lookup, not correctness.
func offCacheEnabled(ctx *gin.Context, logger *zerolog.Logger) bool {
	value, ok := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, isConfig := value.(*configuration.ProviantConfiguration)
	if !ok || !isConfig || proviantConfig == nil {
		logger.Warn().Msg("Offline sync: Open Food Facts cache disabled, proviant config not found in context")
		return false
	}
	return proviantConfig.OpenFoodFacts.CacheEnabled
}
