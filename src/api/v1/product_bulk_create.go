package v1

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// bulkSideEffectSlots bounds the number of concurrent bulk-create side-effect
// workers (activity log + webhook) across all requests.
var bulkSideEffectSlots = make(chan struct{}, 4)

// bulkSideEffectSlotWait bounds how long a side-effect worker waits for a free
// slot before giving up. Long enough to ride out a brief saturation spike,
// short enough that a stuck worker cannot pin a batch indefinitely.
const bulkSideEffectSlotWait = 2 * time.Second

// bulkSideEffectsDropped counts product batches whose activity log entries and
// webhooks were discarded because no side-effect slot became free in time.
// Dropped side effects are otherwise invisible to consumers; the saturation
// Error log below carries the running total, which is the observable metric
// for monitoring.
var bulkSideEffectsDropped atomic.Int64

// bulkSideEffectBatches counts in-flight side-effect worker goroutines.
// Tests use waitBulkSideEffectsIdle so DB cleanup cannot close the database
// under a still-running worker.
var bulkSideEffectBatches atomic.Int64

// BulkCreateProducts creates multiple products in one request. Processing is
// per-item and partial success is allowed: every submitted item reports its
// own outcome, a failing item never blocks the others.
// @Summary       Create multiple products
// @Description   Creates products from a list of drafts with per-item results. Partial success is allowed; each item reports created or failed with a reason.
// @Tags          product
// @Accept        json
// @Produce       json
// @Param         items  body  apiModel.BulkCreateRequest  true  "Product drafts"
// @Success       200  {object}  apiModel.BulkCreateResponse
// @Failure       400  {object}  api.APIResponse
// @Failure       500  {object}  api.APIResponse
// @Router        /api/v1/products/bulk [post]
func BulkCreateProducts(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	var req apiModel.BulkCreateRequest
	// Transport-level body cap before binding: 100 items × unbounded string
	// fields would otherwise be fully buffered in memory before the per-item
	// clamps ever run (same pattern as ScanReceipt).
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, util.ReceiptBulkMaxBodyBytes)
	if bindErr := ctx.ShouldBindJSON(&req); bindErr != nil {
		if importBodyTooLarge(bindErr) {
			logger.Warn().Msgf("Bulk create: request body too large: %s", bindErr)
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	if len(req.Items) == 0 {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "items missing"})
		return
	}
	if len(req.Items) > util.ReceiptBulkMaxItems {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("too many items (max %d)", util.ReceiptBulkMaxItems)})
		return
	}

	// One household lookup per request, not per item
	validLocationIDs := make(map[uint]bool)
	locations, locErr := appCtx.Repos.StorageLocations.GetByHousehold(appCtx.UserID)
	if locErr != nil {
		// With the lookup failed the map stays empty and every item carrying
		// a location is rejected as "unknown storage location" — that reason
		// is misleading without this log line pointing at the real cause.
		logger.Error().Msgf("Bulk create: storage location lookup failed: %s", locErr)
	} else {
		for i := range locations {
			validLocationIDs[locations[i].ID] = true
		}
	}

	// Household and display name feed the activity log; look them up once per
	// request, not per item. A failing household lookup only disables activity
	// logging — it never blocks product creation.
	bulkActorInfo := bulkActor{userID: appCtx.UserID}
	householdID, householdErr := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if householdErr != nil {
		logger.Warn().Msgf("Bulk create: activity log disabled, household lookup failed: %s", householdErr)
	} else {
		bulkActorInfo.householdID = householdID
		bulkActorInfo.householdKnown = true
	}
	if user, userErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID); userErr == nil {
		bulkActorInfo.userName = user.DisplayName
	} else {
		logger.Warn().Msgf("Bulk create: activity log will use empty user name, user lookup failed: %s", userErr)
	}

	results := make([]apiModel.BulkItemResult, 0, len(req.Items))
	createdProducts := make([]*dbModel.Product, 0, len(req.Items))
	for i := range req.Items {
		product, result := bulkCreateItem(appCtx, i, &req.Items[i], validLocationIDs)
		if result.Status == apiModel.BulkItemStatusCreated {
			logger.Info().Msgf("Bulk create: item %d created product %d", i, *result.ProductID)
			createdProducts = append(createdProducts, product)
		} else {
			logger.Warn().Msgf("Bulk create: item %d failed: %s", i, result.Reason)
		}
		results = append(results, result)
	}

	// Side effects run off the request path. A package-level semaphore bounds
	// total in-flight workers: without it, a bulk-create flood would pile up
	// unbounded goroutines contending on the DB (SQLite especially) that the
	// sequential CreateProduct calls above just avoided.
	if len(createdProducts) > 0 {
		bulkSideEffectBatches.Add(1)
		go func() {
			defer bulkSideEffectBatches.Add(-1)
			// Gin's recovery middleware does not cover goroutines: a panic in
			// webhook or activity-log code here would take down the whole
			// server. Contain it — the products are already created and the
			// response is on its way either way.
			defer func() {
				if r := recover(); r != nil {
					appCtx.Logger.Error().Msgf("Bulk create: side-effect worker panicked: %v", r)
				}
			}()
			// The slot is acquired and released per item, never for the whole
			// batch: a 100-item batch with a 5s activity-log timeout each would
			// otherwise monopolize a slot for minutes and starve every other
			// batch's side effects. Releasing between items lets concurrent
			// batches interleave.
			for _, product := range createdProducts {
				select {
				case bulkSideEffectSlots <- struct{}{}:
				case <-time.After(bulkSideEffectSlotWait):
					// No slot freed within the wait: drop this batch's
					// remaining side effects rather than queue unboundedly.
					// Products exist either way; the loss must be visible, so
					// this logs at Error level, not Warn/Debug.
					dropped := bulkSideEffectsDropped.Add(1)
					appCtx.Logger.Error().
						Int("items", len(createdProducts)).
						Int64("batchesDroppedTotal", dropped).
						Msg("Bulk create: no side-effect worker slot free, dropping activity log and webhooks for this batch")
					return
				}
				// Per-item closure: if activity-log or webhook code panics
				// while the slot is held, the deferred release still runs —
				// otherwise the token would leak and permanently shrink the
				// semaphore — before the panic reaches the outer recover.
				func() {
					defer func() { <-bulkSideEffectSlots }()
					logBulkActivity(appCtx, product, bulkActorInfo)
					fireBulkProductCreated(appCtx, product)
				}()
			}
		}()
	}

	ctx.JSON(http.StatusOK, apiModel.BulkCreateResponse{Results: results})
}

// waitBulkSideEffectsIdle blocks until every spawned side-effect worker has
// finished, returning false when the timeout elapses first. Tests call it so
// DB cleanup cannot close the database under a still-running worker.
//
//nolint:unused // called from product_bulk_create_test.go; linter runs with tests:false
func waitBulkSideEffectsIdle(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for bulkSideEffectBatches.Load() > 0 {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
	return true
}

// bulkActor carries the once-per-request user info the activity log needs.
type bulkActor struct {
	userID         uint
	householdID    uint
	householdKnown bool
	userName       string
}

// bulkCreateItem validates one draft and creates it. Validation failures and
// DB errors are item-level outcomes, never request-level ones. The created
// product is returned for the caller's side-effect worker; nil on failure.
func bulkCreateItem(appCtx *AppContext, index int, draft *apiModel.BulkProductDraft, validLocationIDs map[uint]bool) (*dbModel.Product, apiModel.BulkItemResult) {
	// Overlong names are clamped, not rejected: the receipt scan path clamps
	// names the same way and the other string fields are truncated, so a
	// direct API client gets the same permissive-but-bounded treatment.
	productName := truncateDraftField(strings.TrimSpace(draft.ProductName), util.ReceiptItemMaxNameLength)
	if productName == "" {
		return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: "product name is missing"}
	}
	amount := draft.Amount
	if amount < 1 {
		amount = 1
	}
	if amount > util.ReceiptItemMaxAmount {
		amount = util.ReceiptItemMaxAmount
	}

	expireAt := time.Time{}
	if strings.TrimSpace(draft.ExpireAt) != "" {
		parsed, parseErr := time.Parse(util.DefaultDateFormatParseStr, draft.ExpireAt)
		if parseErr != nil {
			// Truncate before echoing: draft.ExpireAt is unvalidated client
			// input and %q-escaping a ~100KB string would blow it up to
			// ~600KB in the response body and Warn log.
			return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: fmt.Sprintf("invalid expiry date %q", truncateDraftField(draft.ExpireAt, 64))}
		}
		expireAt = parsed
	}

	// Mirrors the receipt scan clamp: a direct API client must not push an
	// unbounded or negative price into the DB, where it would poison the
	// savings calculations that sum PriceOverride * Amount.
	if draft.PriceOverride != nil && (*draft.PriceOverride < 0 || *draft.PriceOverride > util.ReceiptItemMaxPrice) {
		return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: fmt.Sprintf("invalid price %.2f (0 to %.0f allowed)", *draft.PriceOverride, util.ReceiptItemMaxPrice)}
	}

	product := dbModel.Product{
		ProductName:   productName,
		Barcode:       truncateDraftField(strings.TrimSpace(draft.Barcode), util.CsvImportMaxBarcodeLength),
		Amount:        amount,
		Unit:          truncateDraftField(strings.TrimSpace(draft.Unit), util.ReceiptItemMaxUnitLength),
		Categories:    truncateDraftField(strings.TrimSpace(draft.Categories), util.ReceiptItemMaxCategoriesLength),
		ExpireAt:      expireAt,
		PriceOverride: draft.PriceOverride,
		IsPrivate:     draft.IsPrivate,
	}
	if draft.StorageLocationID != nil {
		if !validLocationIDs[*draft.StorageLocationID] {
			return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: "unknown storage location"}
		}
		product.StorageLocationID = draft.StorageLocationID
	}

	if err := appCtx.Repos.Products.CreateProduct(appCtx.UserID, &product); err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: "user not found"}
		}
		appCtx.Logger.Error().Msgf("Bulk create: item %d create failed: %s", index, err)
		return nil, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusFailed, Reason: "database error"}
	}

	productID := product.ID
	return &product, apiModel.BulkItemResult{Index: index, Status: apiModel.BulkItemStatusCreated, ProductID: &productID}
}

// fireBulkProductCreated emits the product.created webhook, mirroring the
// single-create path. Failures are the webhook service's concern; the
// product exists either way.
func fireBulkProductCreated(appCtx *AppContext, product *dbModel.Product) {
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
}

// logBulkActivity records the created item in the household activity feed,
// mirroring the single-create path. Failures are logged and ignored: the
// product exists either way.
func logBulkActivity(appCtx *AppContext, product *dbModel.Product, actor bulkActor) {
	if !actor.householdKnown {
		// Without a household the activity row would be orphaned (household 0)
		// and never show up in any feed — skip it entirely. The lookup failure
		// itself was already logged once per request.
		return
	}
	userID := actor.userID
	logEntry := &dbModel.ActivityLog{
		HouseholdID: actor.householdID,
		UserID:      &userID,
		UserName:    actor.userName,
		Action:      dbModel.ActivityActionAdd,
		ProductID:   product.ID,
		ProductName: product.ProductName,
		Quantity:    product.Amount,
		Timestamp:   time.Now(),
	}
	ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := appCtx.Repos.ActivityLogs.Create(ctxBg, logEntry); err != nil {
		appCtx.Logger.Warn().Msgf("Bulk create: activity log failed: %s", err)
	}
}

// truncateDraftField bounds a client-supplied draft string to max UTF-8
// bytes without splitting a multi-byte rune. A drafts endpoint accepts
// permissive input by design; without this an oversized barcode, unit or
// category string lands unbounded in the database.
func truncateDraftField(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
