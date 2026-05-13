package services

import (
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newTestProductService(db *gorm.DB) *ProductService {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db)
	return NewProductService(repos, &logger)
}

func TestProductService_ConsumeProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("success", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		err := svc.ConsumeProduct(product.ID, user.ID)
		if err != nil {
			t.Errorf("ConsumeProduct() error = %v, want nil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := svc.ConsumeProduct(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("ConsumeProduct() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})
}

func TestProductService_WasteProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("success", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		err := svc.WasteProduct(product.ID, user.ID)
		if err != nil {
			t.Errorf("WasteProduct() error = %v, want nil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := svc.WasteProduct(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("WasteProduct() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})
}

func TestProductService_BulkConsumeProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("partial failure", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		ids := []uint{product.ID, 9999}
		err := svc.BulkConsumeProducts(ids, user.ID)
		if err != nil {
			t.Errorf("BulkConsumeProducts() error = %v, want nil", err)
		}
	})
}

func TestProductService_BulkWasteProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("partial failure", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		ids := []uint{product.ID, 9999}
		err := svc.BulkWasteProducts(ids, user.ID)
		if err != nil {
			t.Errorf("BulkWasteProducts() error = %v, want nil", err)
		}
	})
}

func TestProductService_RestoreProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("success", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		_ = svc.ConsumeProduct(product.ID, user.ID)

		err := svc.RestoreProduct(product.ID, user.ID)
		if err != nil {
			t.Errorf("RestoreProduct() error = %v, want nil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := svc.RestoreProduct(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("RestoreProduct() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})
}

func TestProductService_BulkRestoreProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("success", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		_ = svc.ConsumeProduct(product.ID, user.ID)

		ids := []uint{product.ID}
		err := svc.BulkRestoreProducts(ids, user.ID)
		if err != nil {
			t.Errorf("BulkRestoreProducts() error = %v, want nil", err)
		}
	})
}

func TestProductService_DeleteProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)

	t.Run("archive only", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		err := svc.DeleteProduct(product.ID, user.ID, true)
		if err != nil {
			t.Errorf("DeleteProduct(archiveOnly=true) error = %v, want nil", err)
		}
	})

	t.Run("hard delete", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		product := testutil.CreateTestProduct(db, household.ID)

		err := svc.DeleteProduct(product.ID, user.ID, false)
		if err != nil {
			t.Errorf("DeleteProduct(archiveOnly=false) error = %v, want nil", err)
		}
	})
}