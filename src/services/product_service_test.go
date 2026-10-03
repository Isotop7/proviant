package services

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newTestProductService(db *gorm.DB) *ProductService {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db)
	return NewProductService(repos, &logger)
}

// waitFor polls until check returns true or a short deadline passes.
// Activity log and savings event writes happen in background goroutines, so
// their counts are eventually consistent; waiting also prevents those
// goroutines from racing test teardown (which closes the DB under them).
func waitFor(t *testing.T, what string, check func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		done, err := check()
		if err != nil {
			t.Fatalf("query %s: %v", what, err)
		}
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForActivityLogs(t *testing.T, db *gorm.DB, householdID uint, want int) []dbModel.ActivityLog {
	t.Helper()
	var logs []dbModel.ActivityLog
	waitFor(t, "activity log entries", func() (bool, error) {
		if err := db.Where("household_id = ?", householdID).Find(&logs).Error; err != nil {
			return false, err
		}
		// Exact match: background writes only ever add up to `want`
		// entries, so == is the stable final state and keeps later
		// length assertions race-free.
		return len(logs) == want, nil
	})
	return logs
}

// waitForSavingsEvents polls until the expected number of savings events
// exists for the household. Savings events are written in background
// goroutines; waiting prevents the goroutine from racing test teardown
// (which closes the DB under them).
func waitForSavingsEvents(t *testing.T, db *gorm.DB, householdID uint, want int) {
	t.Helper()
	waitFor(t, "savings events", func() (bool, error) {
		var count int64
		if err := db.Model(&dbModel.SavingsRecord{}).Where("household_id = ?", householdID).Count(&count).Error; err != nil {
			return false, err
		}
		return count == int64(want), nil
	})
}

type cookTestIDs struct {
	householdID uint
	userID      uint
}

func TestProductService_CookProducts(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *ProductService, cookTestIDs) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		svc := newTestProductService(db)
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		return db, svc, cookTestIDs{householdID: household.ID, userID: user.ID}
	}

	t.Run("mixed batch with partial, full and missing", func(t *testing.T) {
		db, svc, ids := setup(t)

		partialProduct := testutil.CreateTestProduct(db, ids.householdID, ids.userID)
		db.Model(partialProduct).Update("amount", 4)
		fullProduct := testutil.CreateTestProduct(db, ids.householdID, ids.userID)
		db.Model(fullProduct).Update("amount", 1)

		items := []apiModel.CookItemAPIModel{
			{ProductID: partialProduct.ID, Amount: 2},
			{ProductID: fullProduct.ID, Amount: 0},
			{ProductID: 9999, Amount: 1},
		}
		consumed, partial, errs, internalErr := svc.CookProducts(items, ids.userID)

		if internalErr != nil {
			t.Errorf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 1 {
			t.Errorf("consumed = %d, want 1", consumed)
		}
		if partial != 1 {
			t.Errorf("partial = %d, want 1", partial)
		}
		if len(errs) != 1 {
			t.Errorf("errs = %v, want 1 entry", errs)
		}

		var stillActive dbModel.Product
		if err := db.First(&stillActive, partialProduct.ID).Error; err != nil {
			t.Fatalf("partial product not found: %v", err)
		}
		if stillActive.Amount != 2 {
			t.Errorf("partial product Amount = %d, want 2", stillActive.Amount)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, fullProduct.ID).Error; err != nil {
			t.Fatalf("fully consumed product not found: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("fully consumed product must be archived with reason consumed")
		}

		logs := waitForActivityLogs(t, db, ids.householdID, 2)
		waitForSavingsEvents(t, db, ids.householdID, 1)
		for _, log := range logs {
			if log.Action != dbModel.ActivityActionCook {
				t.Errorf("activity log Action = %q, want %q", log.Action, dbModel.ActivityActionCook)
			}
			expectedQuantity := 2
			if log.ProductID == fullProduct.ID {
				expectedQuantity = 1
			}
			if log.Quantity != expectedQuantity {
				t.Errorf("activity log Quantity = %d, want %d", log.Quantity, expectedQuantity)
			}
		}
	})

	t.Run("full consume via exact amount", func(t *testing.T) {
		db, svc, ids := setup(t)

		product := testutil.CreateTestProduct(db, ids.householdID, ids.userID)
		db.Model(product).Update("amount", 3)

		consumed, partial, errs, internalErr := svc.CookProducts([]apiModel.CookItemAPIModel{{ProductID: product.ID, Amount: 3}}, ids.userID)

		if internalErr != nil {
			t.Errorf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 1 || partial != 0 || len(errs) != 0 {
			t.Errorf("CookProducts() = (%d, %d, %v), want (1, 0, no errors)", consumed, partial, errs)
		}
		waitForActivityLogs(t, db, ids.householdID, 1)
		waitForSavingsEvents(t, db, ids.householdID, 1)
	})

	t.Run("duplicate product entries are merged", func(t *testing.T) {
		db, svc, ids := setup(t)

		product := testutil.CreateTestProduct(db, ids.householdID, ids.userID)
		db.Model(product).Update("amount", 5)

		items := []apiModel.CookItemAPIModel{
			{ProductID: product.ID, Amount: 2},
			{ProductID: product.ID, Amount: 1},
		}
		consumed, partial, errs, internalErr := svc.CookProducts(items, ids.userID)

		if internalErr != nil {
			t.Errorf("internalErr = %v, want nil", internalErr)
		}
		if consumed != 0 || partial != 1 || len(errs) != 0 {
			t.Errorf("CookProducts() = (%d, %d, %v), want (0, 1, no errors)", consumed, partial, errs)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("product not found: %v", err)
		}
		if updated.Amount != 2 {
			t.Errorf("Amount = %d, want 2 (3 consumed from merged entry)", updated.Amount)
		}

		logs := waitForActivityLogs(t, db, ids.householdID, 1)
		if len(logs) != 1 {
			t.Fatalf("activity logs = %d entries, want 1 (merged into one)", len(logs))
		}
		if logs[0].Quantity != 3 {
			t.Errorf("activity log Quantity = %d, want 3", logs[0].Quantity)
		}
	})

	t.Run("error messages are client-friendly", func(t *testing.T) {
		db, svc, ids := setup(t)

		product := testutil.CreateTestProduct(db, ids.householdID, ids.userID)
		db.Model(product).Update("amount", 5)

		_, _, errs, internalErr := svc.CookProducts([]apiModel.CookItemAPIModel{
			{ProductID: product.ID, Amount: 1},
			{ProductID: 9999, Amount: 1},
		}, ids.userID)

		if internalErr != nil {
			t.Errorf("internalErr = %v, want nil (not-found is client-facing)", internalErr)
		}

		if len(errs) != 1 {
			t.Fatalf("errs = %v, want 1 entry", errs)
		}
		if errs[0] != "product 9999 not found" {
			t.Errorf("errs[0] = %q, want %q (no raw driver error text)", errs[0], "product 9999 not found")
		}
	})

	t.Run("unresolvable household surfaces as internal error", func(t *testing.T) {
		db, svc, ids := setup(t)

		product := testutil.CreateTestProduct(db, ids.householdID, ids.userID)

		consumed, partial, errs, internalErr := svc.CookProducts([]apiModel.CookItemAPIModel{
			{ProductID: product.ID, Amount: 1},
		}, 99999)

		if internalErr == nil {
			t.Error("internalErr = nil, want error (household resolution failure is server-side)")
		}
		if consumed != 0 || partial != 0 || len(errs) != 0 {
			t.Errorf("CookProducts() = (%d, %d, %v), want (0, 0, no errors)", consumed, partial, errs)
		}

		var stillActive dbModel.Product
		if err := db.First(&stillActive, product.ID).Error; err != nil {
			t.Fatalf("product not found: %v", err)
		}
		if stillActive.DeletedAt.Valid {
			t.Error("product must not be consumed when household cannot be resolved")
		}
	})
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
