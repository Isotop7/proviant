package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/errors"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestProductRepository_UnknownUserErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	householdRepo := NewProductHouseholdRepository(db)
	listRepo := NewProductListRepository(db)
	productRepo := NewProductRepository(db)
	searchRepo := NewProductSearchRepository(db)
	statsRepo := NewProductStatsRepository(db)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)

	t.Run("queries error for unknown user", func(t *testing.T) {
		if _, err := listRepo.GetUserProductsBulk(9999, 10); err == nil {
			t.Error("GetUserProductsBulk(9999) error = nil")
		}
		if _, err := listRepo.GetUserProductsByIDs(9999, []uint{1}); err == nil {
			t.Error("GetUserProductsByIDs(9999) error = nil")
		}
		if _, err := listRepo.GetUserArchivedProductsBulk(9999, 10, 0); err == nil {
			t.Error("GetUserArchivedProductsBulk(9999) error = nil")
		}
		if _, err := listRepo.GetActiveProductProjections(9999, 0); err == nil {
			t.Error("GetActiveProductProjections(9999) error = nil")
		}
		if _, err := listRepo.GetArchivedProductProjections(9999); err == nil {
			t.Error("GetArchivedProductProjections(9999) error = nil")
		}
		if _, err := listRepo.GetUserArchivedProductsByIDs(9999, []uint{1}); err == nil {
			t.Error("GetUserArchivedProductsByIDs(9999) error = nil")
		}
		if _, err := listRepo.GetUserProductsBulkByBarcodes(9999, []string{"1"}); err == nil {
			t.Error("GetUserProductsBulkByBarcodes(9999) error = nil")
		}
		if _, err := statsRepo.GetExpiredProductsCount(9999, time.Now()); err == nil {
			t.Error("GetExpiredProductsCount(9999) error = nil")
		}
		if _, err := statsRepo.GetActiveProductsCount(9999); err == nil {
			t.Error("GetActiveProductsCount(9999) error = nil")
		}
		if _, err := statsRepo.GetArchivedProductsCount(9999); err == nil {
			t.Error("GetArchivedProductsCount(9999) error = nil")
		}
		if _, err := statsRepo.GetUniqueArchivedProductsCount(9999); err == nil {
			t.Error("GetUniqueArchivedProductsCount(9999) error = nil")
		}
		if _, err := statsRepo.GetProductCategoryBreakdown(9999); err == nil {
			t.Error("GetProductCategoryBreakdown(9999) error = nil")
		}
		if _, err := statsRepo.GetExpiryTrend(9999, time.Now()); err == nil {
			t.Error("GetExpiryTrend(9999) error = nil")
		}
		if _, err := statsRepo.GetExpiringSoonProducts(9999, 7, time.Now()); err == nil {
			t.Error("GetExpiringSoonProducts(9999) error = nil")
		}
		if _, err := householdRepo.GetExpiringInDays(9999, 7); err == nil {
			t.Error("GetExpiringInDays(9999) error = nil")
		}
		if _, err := statsRepo.GetExpiringSoonCount(9999, 7, time.Now()); err == nil {
			t.Error("GetExpiringSoonCount(9999) error = nil")
		}
		if _, _, err := statsRepo.GetActiveExpiryCounts(9999, time.Now(), 7); err == nil {
			t.Error("GetActiveExpiryCounts(9999) error = nil")
		}
		if _, err := householdRepo.GetUserActiveProductsFiltered(9999, &from, &to); err == nil {
			t.Error("GetUserActiveProductsFiltered(9999) error = nil")
		}
		if _, err := householdRepo.GetUserArchivedProductsFiltered(9999, &from, &to); err == nil {
			t.Error("GetUserArchivedProductsFiltered(9999) error = nil")
		}
		if _, err := searchRepo.SearchProducts(ProductName, "x", "", "", 9999, 0, 0); err == nil {
			t.Error("SearchProducts(9999) error = nil")
		}
		if _, err := searchRepo.SearchProductProjections(ProductName, "x", "", "", 9999, 0); err == nil {
			t.Error("SearchProductProjections(9999) error = nil")
		}
		if err := productRepo.CreateProduct(9999, &dbModel.Product{ProductName: "X"}); err == nil {
			t.Error("CreateProduct(9999) error = nil")
		}
		if _, err := productRepo.CreateProductsBulk(9999, []ImportedProduct{{Product: dbModel.Product{ProductName: "X"}}}); err == nil {
			t.Error("CreateProductsBulk(9999) error = nil")
		}
	})

	t.Run("user without household is invalid", func(t *testing.T) {
		householdLess := testutil.CreateTestUser(db, 0)
		if _, err := statsRepo.GetActiveProductsCount(householdLess.ID); err != errors.ErrInvalidUserData {
			t.Errorf("GetActiveProductsCount() error = %v, want %v", err, errors.ErrInvalidUserData)
		}
	})
}

func TestProductRepository_GetProductByIDErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateProduct.IsPrivate = true
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, err := productRepo.GetProductByID(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetProductByID(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("cross-household access is rejected", func(t *testing.T) {
		if _, err := productRepo.GetProductByID(foreign.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductByID() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		if _, err := productRepo.GetProductByID(privateProduct.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductByID() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if _, err := productRepo.GetProductByID(product.ID, 9999); err == nil {
			t.Error("GetProductByID() with unknown user: error = nil, want error")
		}
	})
}
func TestProductRepository_GetArchivedProductByIDErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, err := productRepo.GetArchivedProductByID(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetArchivedProductByID(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, err := productRepo.GetArchivedProductByID(9999, user.ID); err == nil {
			t.Error("GetArchivedProductByID(9999) error = nil, want error")
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if _, err := productRepo.GetArchivedProductByID(product.ID, 9999); err == nil {
			t.Error("GetArchivedProductByID() with unknown user: error = nil, want error")
		}
	})

	t.Run("sees soft-deleted rows", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := db.Delete(product).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}
		found, err := productRepo.GetArchivedProductByID(product.ID, user.ID)
		if err != nil {
			t.Fatalf("GetArchivedProductByID() error = %v", err)
		}
		if !found.DeletedAt.Valid {
			t.Error("GetArchivedProductByID() did not return the soft-deleted row")
		}
	})
}
func TestProductRepository_UserHasProductAccessErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateProduct.IsPrivate = true
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	if productRepo.UserHasProductAccess(user.ID, 0) {
		t.Error("UserHasProductAccess(0) = true, want false")
	}
	if productRepo.UserHasProductAccess(user.ID, -1) {
		t.Error("UserHasProductAccess(-1) = true, want false")
	}
	if productRepo.UserHasProductAccess(9999, int(product.ID)) {
		t.Error("UserHasProductAccess() with unknown user = true, want false")
	}
	if productRepo.UserHasProductAccess(user.ID, int(foreign.ID)) {
		t.Error("UserHasProductAccess() cross-household = true, want false")
	}
	if productRepo.UserHasProductAccess(user.ID, int(privateProduct.ID)) {
		t.Error("UserHasProductAccess() private other-owner = true, want false")
	}
	if !productRepo.UserHasProductAccess(owner.ID, int(privateProduct.ID)) {
		t.Error("UserHasProductAccess() private owner = false, want true")
	}
}
func TestProductRepository_GetProductBySyncID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	otherUser := testutil.CreateTestUser(db, otherHousehold.ID)

	syncID := "shared-sync-id"
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ClientSyncID = &syncID
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("an empty id never resolves", func(t *testing.T) {
		if _, err := productRepo.GetProductBySyncID(user.ID, ""); err != gorm.ErrRecordNotFound {
			t.Errorf("GetProductBySyncID(\"\") error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		if _, err := productRepo.GetProductBySyncID(user.ID, "nope"); err != gorm.ErrRecordNotFound {
			t.Errorf("GetProductBySyncID() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("the owning household resolves it", func(t *testing.T) {
		found, err := productRepo.GetProductBySyncID(user.ID, syncID)
		if err != nil {
			t.Fatalf("GetProductBySyncID() error = %v", err)
		}
		if found.ID != product.ID {
			t.Errorf("GetProductBySyncID() ID = %d, want %d", found.ID, product.ID)
		}
	})

	t.Run("another household with the same id is not found", func(t *testing.T) {
		if _, err := productRepo.GetProductBySyncID(otherUser.ID, syncID); err != gorm.ErrRecordNotFound {
			t.Errorf("GetProductBySyncID() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	// A unique index on client_sync_id alone made this insert fail, so a
	// client that derived its sync id from anything but a random UUID hit a
	// cross-tenant constraint violation.
	t.Run("two households may hold the same sync id", func(t *testing.T) {
		foreign := testutil.CreateTestProduct(db, otherHousehold.ID, otherUser.ID)
		foreign.ClientSyncID = &syncID
		if err := db.Save(foreign).Error; err != nil {
			t.Errorf("second household could not reuse the sync id: %v", err)
		}
	})

	// The unique index spans soft-deleted rows: a miss here would send a
	// replay into an insert that violates it instead of reporting the row.
	t.Run("an archived product still resolves", func(t *testing.T) {
		if err := db.Delete(product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
		found, err := productRepo.GetProductBySyncID(user.ID, syncID)
		if err != nil {
			t.Fatalf("GetProductBySyncID() error = %v", err)
		}
		if found.ID != product.ID {
			t.Errorf("GetProductBySyncID() ID = %d, want %d", found.ID, product.ID)
		}
	})
}
func TestProductRepository_UpdateProductErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	t.Run("zero id is not implemented", func(t *testing.T) {
		if err := productRepo.UpdateProduct(0, user.ID, &dbModel.ProductDTOPatch{}); err != gorm.ErrNotImplemented {
			t.Errorf("UpdateProduct(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("invalid opened lifecycle is rejected", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		future := time.Now().Add(72 * time.Hour)
		if err := productRepo.UpdateProduct(product.ID, user.ID, &dbModel.ProductDTOPatch{OpenedAt: &future}); err != errors.ErrInvalidOpenedLifecycle {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrInvalidOpenedLifecycle)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if err := productRepo.UpdateProduct(9999, user.ID, &dbModel.ProductDTOPatch{}); err == nil {
			t.Error("UpdateProduct(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
		if err := productRepo.UpdateProduct(foreign.ID, user.ID, &dbModel.ProductDTOPatch{}); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
		privateProduct.IsPrivate = true
		if err := db.Save(privateProduct).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		if err := productRepo.UpdateProduct(privateProduct.ID, user.ID, &dbModel.ProductDTOPatch{}); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("unattributed product is claimed by the caller", func(t *testing.T) {
		product := dbModel.Product{ProductName: "Orphan", Barcode: "6000000000001", HouseholdID: household.ID}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("seed orphan product: %v", err)
		}
		patch := dbModel.ProductDTOPatch{ProductName: "Claimed"}
		if err := productRepo.UpdateProduct(product.ID, user.ID, &patch); err != nil {
			t.Fatalf("UpdateProduct() error = %v", err)
		}
		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.UserID != user.ID {
			t.Errorf("UserID = %d, want %d (claimed by caller)", updated.UserID, user.ID)
		}
		if updated.ProductName != "Claimed" {
			t.Errorf("ProductName = %q, want Claimed", updated.ProductName)
		}
	})
}
func TestProductRepository_DeleteProductHardDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("archiveOnly=false removes the row entirely", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := productRepo.DeleteProduct(product.ID, user.ID, false); err != nil {
			t.Fatalf("DeleteProduct() error = %v", err)
		}
		var count int64
		db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Error("DeleteProduct(archiveOnly=false) left the row behind")
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if err := productRepo.DeleteProduct(9999, user.ID, true); err == nil {
			t.Error("DeleteProduct(9999) error = nil, want error")
		}
	})
}
func TestProductRepository_RestoreAndBulkRestore(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("restore unknown product errors", func(t *testing.T) {
		if err := productRepo.RestoreProduct(9999, user.ID); err == nil {
			t.Error("RestoreProduct(9999) error = nil, want error")
		}
	})

	t.Run("bulk restore returns no errors on success", func(t *testing.T) {
		p1 := testutil.CreateTestProduct(db, household.ID, user.ID)
		p2 := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := db.Delete(p1).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}
		if err := db.Delete(p2).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}

		errs := productRepo.BulkRestoreProducts([]uint{p1.ID, p2.ID}, user.ID)
		if len(errs) != 0 {
			t.Fatalf("BulkRestoreProducts() returned %d errors, want 0", len(errs))
		}
		for _, id := range []uint{p1.ID, p2.ID} {
			var restored dbModel.Product
			if err := db.First(&restored, id).Error; err != nil {
				t.Errorf("product %d not restored: %v", id, err)
			}
		}
	})
}
func TestProductRepository_SetProductExpireAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	t.Run("success updates the expiry", func(t *testing.T) {
		newExpiry := time.Now().Add(72 * time.Hour).Truncate(time.Second)
		expireAt := dbModel.Timestamp{Timestamp: dbModel.Date(newExpiry)}
		if err := productRepo.SetProductExpireAt(product.ID, user.ID, expireAt); err != nil {
			t.Fatalf("SetProductExpireAt() error = %v", err)
		}
		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if !updated.ExpireAt.Equal(newExpiry) {
			t.Errorf("ExpireAt = %v, want %v", updated.ExpireAt, newExpiry)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		expireAt := dbModel.Timestamp{Timestamp: dbModel.Date(time.Now())}
		if err := productRepo.SetProductExpireAt(9999, user.ID, expireAt); err == nil {
			t.Error("SetProductExpireAt(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		expireAt := dbModel.Timestamp{Timestamp: dbModel.Date(time.Now())}
		if err := productRepo.SetProductExpireAt(foreign.ID, user.ID, expireAt); err != errors.ErrMismatcherUserID {
			t.Errorf("SetProductExpireAt() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})
}
func TestProductRepository_MarkProductOpenedErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("unknown user errors", func(t *testing.T) {
		if _, _, _, err := productRepo.MarkProductOpened(1, 9999, time.Now(), false); err == nil {
			t.Error("MarkProductOpened() with unknown user: error = nil, want error")
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, _, _, err := productRepo.MarkProductOpened(9999, user.ID, time.Now(), false); err == nil {
			t.Error("MarkProductOpened(9999) error = nil, want error")
		}
	})
}
