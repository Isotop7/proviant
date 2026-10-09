package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// cookMaxConsumeAttempts bounds service-level retries of ConsumeProductPartial
// after the repo's own single optimistic-lock retry; the last attempt still
// losing the race surfaces ErrProductConcurrentModification to the caller.
const cookMaxConsumeAttempts = 3

type ProductService struct {
	repos  *database.RepositoryContainer
	logger *zerolog.Logger
}

func NewProductService(repos *database.RepositoryContainer, logger *zerolog.Logger) *ProductService {
	return &ProductService{
		repos:  repos,
		logger: logger,
	}
}

func (s *ProductService) recordSavingsEvent(userID uint, product *dbModel.Product, eventType string) {
	if householdID, err := s.repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		if err := s.repos.Savings.RecordSavingsEvent(householdID, product, eventType); err != nil {
			s.logger.Error().Msgf("RecordSavingsEvent (%s): %s", eventType, err)
		}
	}
}

func (s *ProductService) recordActivityLog(userID uint, householdID uint, action string, productID uint, productName string, quantity int) {
	if householdID == 0 {
		return
	}
	go func() {
		entry := s.activityLogEntry(userID, householdID, action, s.activityLogUserName(userID), productID, productName, quantity)
		s.createActivityLog(entry)
	}()
}

// recordActivityLogs writes the activity entries of a bulk action behind a
// single worker goroutine that inserts sequentially: one entry per restored
// row must not fan out one goroutine (and one user lookup) per row the way the
// single-product path does. Everything, including the user lookup, runs off
// the request path.
func (s *ProductService) recordActivityLogs(userID uint, householdID uint, action string, products []dbModel.Product) {
	if householdID == 0 || len(products) == 0 {
		return
	}
	go func() {
		userName := s.activityLogUserName(userID)
		for i := range products {
			s.createActivityLog(s.activityLogEntry(userID, householdID, action, userName, products[i].ID, products[i].ProductName, 1))
		}
	}()
}

// activityLogUserName resolves the actor's display name for an activity entry.
func (s *ProductService) activityLogUserName(userID uint) string {
	if user, err := s.repos.Users.GetUserByID(userID); err == nil {
		return user.DisplayName
	}
	return ""
}

// activityLogEntry builds one entry; userName is resolved once per batch by
// the caller via activityLogUserName.
func (s *ProductService) activityLogEntry(
	userID uint, householdID uint, action string, userName string,
	productID uint, productName string, quantity int,
) *dbModel.ActivityLog {
	return &dbModel.ActivityLog{
		HouseholdID: householdID,
		UserID:      &userID,
		UserName:    userName,
		Action:      action,
		ProductID:   productID,
		ProductName: productName,
		Quantity:    quantity,
		Timestamp:   time.Now(),
	}
}

// createActivityLog inserts the entry with a bounded timeout. Callers run it
// either in its own goroutine (single product) or sequentially in a bulk
// worker.
func (s *ProductService) createActivityLog(entry *dbModel.ActivityLog) {
	if s.repos.ActivityLogs == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repos.ActivityLogs.Create(ctx, entry); err != nil {
		s.logger.Error().Msgf("recordActivityLog: %s", err)
	}
}

func (s *ProductService) ConsumeProduct(productID, userID uint) error {
	product, err := s.repos.Products.GetProductByID(productID, userID)
	if err != nil {
		return err
	}

	if err := s.repos.Products.ConsumeProduct(productID, userID); err != nil {
		return err
	}

	go s.recordSavingsEvent(userID, &product, "consumed")

	if householdID, err := s.repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		go s.recordActivityLog(userID, householdID, dbModel.ActivityActionConsume, productID, product.ProductName, 1)
	}
	return nil
}

func (s *ProductService) WasteProduct(productID, userID uint) error {
	product, err := s.repos.Products.GetProductByID(productID, userID)
	if err != nil {
		return err
	}

	if err := s.repos.Products.WasteProduct(productID, userID); err != nil {
		return err
	}

	if householdID, err := s.repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		if err := s.repos.Streaks.RecordWasteEvent(householdID); err != nil {
			s.logger.Error().Msgf("WasteProduct: failed to record waste event for streak: %s", err)
		}
		go s.recordActivityLog(userID, householdID, dbModel.ActivityActionWaste, productID, product.ProductName, 1)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEventContext(ctx, "product.wasted", map[string]any{
				"productId": productID,
			})
		}
	}()

	go s.recordSavingsEvent(userID, &product, "wasted")
	return nil
}

// bulkClientMiss reports whether a bulk-operation failure is the caller's
// problem (unknown, foreign or malformed id) rather than a server fault, so
// the handler can answer 4xx-with-failedIds instead of a blanket 500. Body
// ids never pass through parseUintPathParam, so id 0 reaches the
// repositories and surfaces as gorm.ErrNotImplemented — a bad request, not
// an outage.
// bulkClientMiss identifies per-id bulk failures that are the caller's or a
// concurrent writer's problem — bad/foreign ids, invalid data, or a retryable
// optimistic-lock race (ErrProductConcurrentModification) — rather than a
// server-side fault. These do not turn the bulk response into a 5xx; the id
// simply lands in failedIds for a retry.
func bulkClientMiss(err error) bool {
	return err == gorm.ErrRecordNotFound ||
		err == errors.ErrMismatcherUserID ||
		err == errors.ErrInvalidUserData ||
		err == gorm.ErrNotImplemented ||
		err == errors.ErrProductConcurrentModification
}

// BulkConsumeProducts archives productIDs and reports which ones were not
// touched. Mirrors BulkRestoreProducts: the failed list carries the ids the
// caller can fix, the error is non-nil only for a server-side failure, so the
// handler never reports success for a request that did not land.
func (s *ProductService) BulkConsumeProducts(productIDs []uint, userID uint) ([]uint, error) {
	householdID, _ := s.repos.Users.GetUserHouseholdByID(userID)

	failed := make([]uint, 0)
	var serverErr error
	for _, productID := range productIDs {
		product, err := s.repos.Products.GetProductByID(productID, userID)
		if err != nil {
			failed = append(failed, productID)
			s.logBulkFailure("BulkConsumeProducts", productID, err, &serverErr)
			continue
		}

		if consumeErr := s.repos.Products.ConsumeProduct(productID, userID); consumeErr != nil {
			failed = append(failed, productID)
			s.logBulkFailure("BulkConsumeProducts", productID, consumeErr, &serverErr)
			continue
		}

		go s.recordSavingsEvent(userID, &product, "consumed")
		if householdID > 0 {
			go s.recordActivityLog(userID, householdID, dbModel.ActivityActionConsume, productID, product.ProductName, 1)
		}
	}
	return failed, serverErr
}

// BulkWasteProducts hard-deletes productIDs and reports which ones were not
// touched. Same contract as BulkConsumeProducts.
func (s *ProductService) BulkWasteProducts(productIDs []uint, userID uint) ([]uint, error) {
	householdID, householdErr := s.repos.Users.GetUserHouseholdByID(userID)

	failed := make([]uint, 0)
	var serverErr error
	for _, productID := range productIDs {
		product, err := s.repos.Products.GetProductByID(productID, userID)
		if err != nil {
			failed = append(failed, productID)
			s.logBulkFailure("BulkWasteProducts", productID, err, &serverErr)
			continue
		}

		if wasteErr := s.repos.Products.WasteProduct(productID, userID); wasteErr != nil {
			failed = append(failed, productID)
			s.logBulkFailure("BulkWasteProducts", productID, wasteErr, &serverErr)
			continue
		}

		if householdErr == nil && householdID > 0 {
			if err := s.repos.Streaks.RecordWasteEvent(householdID); err != nil {
				s.logger.Error().Msgf("BulkWasteProducts: failed to record waste event for streak: %s", err)
			}
			go s.recordActivityLog(userID, householdID, dbModel.ActivityActionWaste, productID, product.ProductName, 1)
		}

		go func(pid uint) {
			if ws := controllers.GetWebhookService(); ws != nil {
				ws.FireEvent("product.wasted", map[string]any{
					"productId": pid,
				})
			}
		}(productID)

		go s.recordSavingsEvent(userID, &product, "wasted")
	}
	return failed, serverErr
}

// logBulkFailure records one per-id bulk failure at the right level (client
// misses, including retryable optimistic-lock races, are Warn so the Error
// stream keeps its meaning) and remembers the first server-side cause for the
// caller's 5xx decision.
func (s *ProductService) logBulkFailure(op string, productID uint, err error, serverErr *error) {
	if bulkClientMiss(err) {
		s.logger.Warn().Msgf("%s: product %d rejected (bad id or concurrent change): %s", op, productID, err)
		return
	}
	s.logger.Error().Msgf("%s: product %d: %s", op, productID, err)
	if *serverErr == nil {
		*serverErr = err
	}
}

// CookProducts consumes the given products, partially or fully. errs carries
// client-facing per-item failures; internalErr carries the first server-side
// failure (household resolution, DB errors) so the handler can answer 5xx
// instead of blaming the client.
func (s *ProductService) CookProducts(items []apiModel.CookItemAPIModel, userID uint) (consumed, partial int, errs []string, internalErr error) {
	householdID, householdErr := s.repos.Users.GetUserHouseholdByID(userID)
	if householdErr != nil {
		// Without a household, activity logs and savings events would
		// silently diverge from the other consume paths — fail the whole
		// request instead of consuming without a trace.
		s.logger.Error().Msgf("CookProducts: could not resolve household for user %d: %s", userID, householdErr)
		return 0, 0, nil, householdErr
	}

	// Merge duplicate product entries by summing their amounts, so two
	// rows for the same product cook as one combined consume instead of
	// the second one failing with "not found" after the first archived it.
	merged := make([]apiModel.CookItemAPIModel, 0, len(items))
	mergedIndex := make(map[uint]int, len(items))
	for _, item := range items {
		if idx, ok := mergedIndex[item.ProductID]; ok {
			// Overflow-safe sum: a merged amount can never wrap to a
			// negative value, which the repository would treat as a
			// full consume.
			if merged[idx].Amount > math.MaxInt-item.Amount {
				merged[idx].Amount = math.MaxInt
			} else {
				merged[idx].Amount += item.Amount
			}
			s.logger.Debug().Msgf("CookProducts: merged duplicate entry for product %d", item.ProductID)
			continue
		}
		mergedIndex[item.ProductID] = len(merged)
		merged = append(merged, item)
	}
	items = merged

	for _, item := range items {
		product, err := s.repos.Products.GetProductByID(item.ProductID, userID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				s.logger.Warn().Msgf("CookProducts: product %d not found", item.ProductID)
				errs = append(errs, fmt.Sprintf("product %d not found", item.ProductID))
			} else {
				s.logger.Error().Msgf("CookProducts: product %d lookup failed: %s", item.ProductID, err)
				errs = append(errs, fmt.Sprintf("product %d lookup failed", item.ProductID))
				if internalErr == nil {
					internalErr = err
				}
			}
			continue
		}

		consumedAmount, fullyConsumed, cookErr := s.repos.Products.ConsumeProductPartial(&product, item.Amount)
		// The repo retries guarded writes once internally; a second
		// optimistic-lock failure means the row is still hot. Re-attempting
		// against the same product pointer is safe — the repo re-reads
		// current state inside its transaction — so a busy product fails
		// the whole cook only after every attempt lost the race.
		for retry := 1; cookErr == errors.ErrProductConcurrentModification && retry < cookMaxConsumeAttempts; retry++ {
			s.logger.Debug().Msgf("CookProducts: product %d changed concurrently, retrying (%d/%d)", item.ProductID, retry, cookMaxConsumeAttempts-1)
			consumedAmount, fullyConsumed, cookErr = s.repos.Products.ConsumeProductPartial(&product, item.Amount)
		}
		if cookErr != nil {
			if cookErr == gorm.ErrRecordNotFound || cookErr == errors.ErrProductConcurrentModification {
				s.logger.Warn().Msgf("CookProducts: %s", cookErr)
			} else {
				s.logger.Error().Msgf("CookProducts: %s", cookErr)
				if internalErr == nil {
					internalErr = cookErr
				}
			}
			errs = append(errs, fmt.Sprintf("%s: %s", product.ProductName, cookErrorMessage(cookErr)))
			continue
		}

		if fullyConsumed {
			consumed++
			// Record savings against the actually archived amount, not the
			// caller's read: the repo re-reads stock inside its transaction,
			// so a concurrent reduction would otherwise overstate the
			// SavingsRecord amount, price and CO2 figures.
			product.Amount = consumedAmount
			go s.recordSavingsEvent(userID, &product, "consumed")
		} else {
			partial++
		}

		go s.recordActivityLog(userID, householdID, dbModel.ActivityActionCook, item.ProductID, product.ProductName, consumedAmount)
	}
	return consumed, partial, errs, internalErr
}

// cookErrorMessage translates repository errors into client-facing wording
// instead of leaking raw driver messages like "record not found". Unknown
// errors are reported generically — the Error()-level log already carries
// the details.
func cookErrorMessage(err error) string {
	switch err {
	case gorm.ErrRecordNotFound:
		return "not found"
	case errors.ErrProductConcurrentModification:
		return "was changed by another request, please retry"
	default:
		return "internal error"
	}
}

func (s *ProductService) RestoreProduct(productID, userID uint) error {
	product, err := s.repos.Products.GetArchivedProductByID(productID, userID)
	if err != nil {
		return err
	}

	if err := s.repos.Products.RestoreProduct(productID, userID); err != nil {
		return err
	}

	if householdID, err := s.repos.Users.GetUserHouseholdByID(userID); err == nil && householdID > 0 {
		go s.recordActivityLog(userID, householdID, dbModel.ActivityActionRestore, productID, product.ProductName, 1)
	}
	return nil
}

// BulkRestoreProducts restores productIDs and reports which ones were not
// restored. The returned error is non-nil only when a failure was server-side
// (so the caller can surface a 500); unknown or foreign ids come back in the
// failed list instead, which keeps the client from being told "restored" for
// ids that were not.
func (s *ProductService) BulkRestoreProducts(productIDs []uint, userID uint) ([]uint, error) {
	householdID, _ := s.repos.Users.GetUserHouseholdByID(userID)
	errs := s.repos.Products.BulkRestoreProducts(productIDs, userID)
	// The error list is sparse: one entry per *failed* id, not per requested
	// id. Indexing productIDs by the error position logged restore activity
	// against the wrong products (and against failures), so track the failed
	// ids explicitly and derive the restored ones below.
	failed := make([]uint, 0, len(errs))
	failedSet := make(map[uint]bool, len(errs))
	var serverErr error
	for i := range errs {
		cause := errs[i].Err()
		// An id the caller can fix (unknown, foreign, malformed) is not a
		// server fault: warn, so the Error stream keeps its meaning.
		clientMiss := bulkClientMiss(cause)
		if clientMiss {
			s.logger.Warn().Msg(errs[i].Error())
		} else {
			s.logger.Error().Msg(errs[i].Error())
		}
		failed = append(failed, errs[i].ProductID())
		failedSet[errs[i].ProductID()] = true
		if cause != nil && !clientMiss && serverErr == nil {
			serverErr = cause
		}
	}
	// The ids that landed, deduplicated so a repeat is counted (and
	// messaged) once. Note the shared lookup is Unscoped — it also serves
	// DeleteProduct on active rows — so a repeated id succeeds again
	// instead of failing here.
	restored := make([]uint, 0, len(productIDs))
	seen := make(map[uint]bool, len(productIDs))
	for _, productID := range productIDs {
		if seen[productID] || failedSet[productID] {
			continue
		}
		seen[productID] = true
		restored = append(restored, productID)
	}
	if len(restored) == 0 {
		return failed, serverErr
	}
	// One batch read instead of a per-id re-read: the old loop cost two
	// queries per restored row on the request path plus one goroutine each.
	// It also confirms the rows really are active again before they are
	// logged as restored.
	products, err := s.repos.Products.GetUserProductsByIDs(userID, restored)
	if err != nil {
		// The restores may well have landed, but nothing was verified and no
		// activity was logged: a server-side read fault must surface as a
		// 500 instead of reporting an unconfirmed success.
		s.logger.Error().Msgf("BulkRestoreProducts: failed to verify %d restored products: %s", len(restored), err)
		if serverErr == nil {
			serverErr = err
		}
		return failed, serverErr
	}
	if len(products) != len(restored) {
		s.logger.Warn().Msgf("BulkRestoreProducts: verified %d of %d restored products for the activity log",
			len(products), len(restored))
	}
	s.recordActivityLogs(userID, householdID, dbModel.ActivityActionRestore, products)
	return failed, serverErr
}

func (s *ProductService) DeleteProduct(productID, userID uint, archiveOnly bool) error {
	return s.repos.Products.DeleteProduct(productID, userID, archiveOnly)
}
