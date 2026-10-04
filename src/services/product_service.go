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
	userName := ""
	if user, err := s.repos.Users.GetUserByID(userID); err == nil {
		userName = user.DisplayName
	}
	logEntry := &dbModel.ActivityLog{
		HouseholdID: householdID,
		UserID:      &userID,
		UserName:    userName,
		Action:      action,
		ProductID:   productID,
		ProductName: productName,
		Quantity:    quantity,
		Timestamp:   time.Now(),
	}
	go func() {
		if s.repos.ActivityLogs == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repos.ActivityLogs.Create(ctx, logEntry); err != nil {
			s.logger.Error().Msgf("recordActivityLog: %s", err)
		}
	}()
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

func (s *ProductService) BulkConsumeProducts(productIDs []uint, userID uint) error {
	householdID, _ := s.repos.Users.GetUserHouseholdByID(userID)

	for _, productID := range productIDs {
		product, err := s.repos.Products.GetProductByID(productID, userID)
		if err != nil {
			s.logger.Warn().Msgf("BulkConsumeProducts: product %d not found", productID)
			continue
		}

		if consumeErr := s.repos.Products.ConsumeProduct(productID, userID); consumeErr != nil {
			if consumeErr == gorm.ErrRecordNotFound {
				s.logger.Warn().Msgf("BulkConsumeProducts: product %d not found", productID)
			} else {
				s.logger.Error().Msgf("BulkConsumeProducts: %s", consumeErr)
			}
			continue
		}

		go s.recordSavingsEvent(userID, &product, "consumed")
		if householdID > 0 {
			go s.recordActivityLog(userID, householdID, dbModel.ActivityActionConsume, productID, product.ProductName, 1)
		}
	}
	return nil
}

func (s *ProductService) BulkWasteProducts(productIDs []uint, userID uint) error {
	householdID, householdErr := s.repos.Users.GetUserHouseholdByID(userID)

	for _, productID := range productIDs {
		product, err := s.repos.Products.GetProductByID(productID, userID)
		if err != nil {
			s.logger.Warn().Msgf("BulkWasteProducts: product %d not found", productID)
			continue
		}

		if wasteErr := s.repos.Products.WasteProduct(productID, userID); wasteErr != nil {
			if wasteErr == gorm.ErrRecordNotFound {
				s.logger.Warn().Msgf("BulkWasteProducts: product %d not found", productID)
			} else {
				s.logger.Error().Msgf("BulkWasteProducts: %s", wasteErr)
			}
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
	return nil
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
	switch {
	case err == gorm.ErrRecordNotFound:
		return "not found"
	case err == errors.ErrProductConcurrentModification:
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

func (s *ProductService) BulkRestoreProducts(productIDs []uint, userID uint) error {
	householdID, _ := s.repos.Users.GetUserHouseholdByID(userID)
	errs := s.repos.Products.BulkRestoreProducts(productIDs, userID)
	for idx, err := range errs {
		s.logger.Error().Msg(err.Error())
		if idx < len(productIDs) {
			productID := productIDs[idx]
			product, err := s.repos.Products.GetArchivedProductByID(productID, userID)
			if err == nil && householdID > 0 {
				go s.recordActivityLog(userID, householdID, dbModel.ActivityActionRestore, productID, product.ProductName, 1)
			}
		}
	}
	return nil
}

func (s *ProductService) DeleteProduct(productID, userID uint, archiveOnly bool) error {
	return s.repos.Products.DeleteProduct(productID, userID, archiveOnly)
}
