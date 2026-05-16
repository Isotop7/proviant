package services

import (
	"context"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

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
		HouseholdID:  householdID,
		UserID:       &userID,
		UserName:     userName,
		Action:       action,
		ProductID:    productID,
		ProductName:  productName,
		Quantity:     quantity,
		Timestamp:    time.Now(),
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
