package database

import (
	stderrors "errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// TestProductRepository_UnknownUserErrors pins the getUserHouseholdID error
// path shared by every household-scoped query: an unknown user id must
// surface the lookup error, not an empty result.
func TestProductRepository_UnknownUserErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)

	t.Run("queries error for unknown user", func(t *testing.T) {
		if _, err := repo.GetUserProductsBulk(9999, 10); err == nil {
			t.Error("GetUserProductsBulk(9999) error = nil")
		}
		if _, err := repo.GetUserProductsByIDs(9999, []uint{1}); err == nil {
			t.Error("GetUserProductsByIDs(9999) error = nil")
		}
		if _, err := repo.GetUserArchivedProductsBulk(9999, 10); err == nil {
			t.Error("GetUserArchivedProductsBulk(9999) error = nil")
		}
		if _, err := repo.GetActiveProductProjections(9999, 0); err == nil {
			t.Error("GetActiveProductProjections(9999) error = nil")
		}
		if _, err := repo.GetArchivedProductProjections(9999); err == nil {
			t.Error("GetArchivedProductProjections(9999) error = nil")
		}
		if _, err := repo.GetUserArchivedProductsByIDs(9999, []uint{1}); err == nil {
			t.Error("GetUserArchivedProductsByIDs(9999) error = nil")
		}
		if _, err := repo.GetUserProductsBulkByBarcodes(9999, []string{"1"}); err == nil {
			t.Error("GetUserProductsBulkByBarcodes(9999) error = nil")
		}
		if _, err := repo.GetExpiredProductsCount(9999); err == nil {
			t.Error("GetExpiredProductsCount(9999) error = nil")
		}
		if _, err := repo.GetActiveProductsCount(9999); err == nil {
			t.Error("GetActiveProductsCount(9999) error = nil")
		}
		if _, err := repo.GetArchivedProductsCount(9999); err == nil {
			t.Error("GetArchivedProductsCount(9999) error = nil")
		}
		if _, err := repo.GetUniqueArchivedProductsCount(9999); err == nil {
			t.Error("GetUniqueArchivedProductsCount(9999) error = nil")
		}
		if _, err := repo.GetProductCategoryBreakdown(9999); err == nil {
			t.Error("GetProductCategoryBreakdown(9999) error = nil")
		}
		if _, err := repo.GetExpiryTrend(9999); err == nil {
			t.Error("GetExpiryTrend(9999) error = nil")
		}
		if _, err := repo.GetExpiringSoonProducts(9999, 7); err == nil {
			t.Error("GetExpiringSoonProducts(9999) error = nil")
		}
		if _, err := repo.GetExpiringInDays(9999, 7); err == nil {
			t.Error("GetExpiringInDays(9999) error = nil")
		}
		if _, err := repo.GetExpiringSoonCount(9999, 7); err == nil {
			t.Error("GetExpiringSoonCount(9999) error = nil")
		}
		if _, _, err := repo.GetActiveExpiryCounts(9999, time.Now(), 7); err == nil {
			t.Error("GetActiveExpiryCounts(9999) error = nil")
		}
		if _, err := repo.GetUserActiveProductsFiltered(9999, &from, &to); err == nil {
			t.Error("GetUserActiveProductsFiltered(9999) error = nil")
		}
		if _, err := repo.GetUserArchivedProductsFiltered(9999, &from, &to); err == nil {
			t.Error("GetUserArchivedProductsFiltered(9999) error = nil")
		}
		if _, err := repo.SearchProducts(ProductName, "x", "", "", 9999); err == nil {
			t.Error("SearchProducts(9999) error = nil")
		}
		if _, err := repo.SearchProductProjections(ProductName, "x", "", "", 9999, 0); err == nil {
			t.Error("SearchProductProjections(9999) error = nil")
		}
		if err := repo.CreateProduct(9999, &dbModel.Product{ProductName: "X"}); err == nil {
			t.Error("CreateProduct(9999) error = nil")
		}
		if _, err := repo.CreateProductsBulk(9999, []ImportedProduct{{Product: dbModel.Product{ProductName: "X"}}}); err == nil {
			t.Error("CreateProductsBulk(9999) error = nil")
		}
	})

	t.Run("user without household is invalid", func(t *testing.T) {
		householdLess := testutil.CreateTestUser(db, 0)
		if _, err := repo.GetActiveProductsCount(householdLess.ID); err != errors.ErrInvalidUserData {
			t.Errorf("GetActiveProductsCount() error = %v, want %v", err, errors.ErrInvalidUserData)
		}
	})
}

func TestProductRepository_GetProductByIDErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
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
		if _, err := repo.GetProductByID(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetProductByID(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("cross-household access is rejected", func(t *testing.T) {
		if _, err := repo.GetProductByID(foreign.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductByID() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		if _, err := repo.GetProductByID(privateProduct.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductByID() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if _, err := repo.GetProductByID(product.ID, 9999); err == nil {
			t.Error("GetProductByID() with unknown user: error = nil, want error")
		}
	})
}

func TestProductRepository_GetArchivedProductByIDErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, err := repo.GetArchivedProductByID(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetArchivedProductByID(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, err := repo.GetArchivedProductByID(9999, user.ID); err == nil {
			t.Error("GetArchivedProductByID(9999) error = nil, want error")
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if _, err := repo.GetArchivedProductByID(product.ID, 9999); err == nil {
			t.Error("GetArchivedProductByID() with unknown user: error = nil, want error")
		}
	})

	t.Run("sees soft-deleted rows", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := db.Delete(product).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}
		found, err := repo.GetArchivedProductByID(product.ID, user.ID)
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
	repo := NewProductRepository(db)
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

	if repo.UserHasProductAccess(user.ID, 0) {
		t.Error("UserHasProductAccess(0) = true, want false")
	}
	if repo.UserHasProductAccess(user.ID, -1) {
		t.Error("UserHasProductAccess(-1) = true, want false")
	}
	if repo.UserHasProductAccess(9999, int(product.ID)) {
		t.Error("UserHasProductAccess() with unknown user = true, want false")
	}
	if repo.UserHasProductAccess(user.ID, int(foreign.ID)) {
		t.Error("UserHasProductAccess() cross-household = true, want false")
	}
	if repo.UserHasProductAccess(user.ID, int(privateProduct.ID)) {
		t.Error("UserHasProductAccess() private other-owner = true, want false")
	}
	if !repo.UserHasProductAccess(owner.ID, int(privateProduct.ID)) {
		t.Error("UserHasProductAccess() private owner = false, want true")
	}
}

func TestProductRepository_UpdateProductQuantity(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	setup := func(t *testing.T, amount int) dbModel.Product {
		t.Helper()
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Quantity = amount
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		return *product
	}

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, _, err := repo.UpdateProductQuantity(0, user.ID, 1, nil); err != gorm.ErrNotImplemented {
			t.Errorf("UpdateProductQuantity(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, _, err := repo.UpdateProductQuantity(9999, user.ID, 1, nil); err == nil {
			t.Error("UpdateProductQuantity(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
		if _, _, err := repo.UpdateProductQuantity(foreign.ID, user.ID, 1, nil); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProductQuantity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("positive delta increases amount", func(t *testing.T) {
		product := setup(t, 2)
		deleted, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, nil)
		if err != nil {
			t.Fatalf("UpdateProductQuantity() error = %v", err)
		}
		if deleted {
			t.Error("UpdateProductQuantity() deleted = true, want false")
		}
		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 5 {
			t.Errorf("Amount = %d, want 5", updated.Quantity)
		}
	})

	t.Run("negative delta clamps at zero and hard-deletes", func(t *testing.T) {
		product := setup(t, 2)
		deleted, _, err := repo.UpdateProductQuantity(product.ID, user.ID, -5, nil)
		if err != nil {
			t.Fatalf("UpdateProductQuantity() error = %v", err)
		}
		if !deleted {
			t.Error("UpdateProductQuantity() deleted = false, want true at zero stock")
		}
		var count int64
		db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Error("zero-stock product was not hard-deleted")
		}
	})

	// Both goroutines zero the row and both find it at zero, so exactly one
	// DELETE lands. The loser's RowsAffected is 0, and the read-back it
	// triggers is what keeps the loser reporting deleted=true — the product
	// really is gone, and this delta helped zero it.
	//
	// A goroutine can also be scheduled only after the winner finished
	// everything, hard DELETE included. Its first read then finds no row,
	// and the call answers with the same ErrRecordNotFound an unknown id
	// gets: the two are indistinguishable from the database alone, and the
	// handler answers both alike with a 404 that the outbox drops, since a
	// delta for a deleted product can never apply. Both answers report the
	// row gone; neither leaves it behind.
	t.Run("concurrent decrements delete once and both report it", func(t *testing.T) {
		product := setup(t, 2)
		var wg sync.WaitGroup
		var mu sync.Mutex
		deletedCount := 0
		vanishedCount := 0
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				deleted, _, err := repo.UpdateProductQuantity(product.ID, user.ID, -2, nil)
				if stderrors.Is(err, gorm.ErrRecordNotFound) {
					mu.Lock()
					vanishedCount++
					mu.Unlock()
					return
				}
				if err != nil {
					t.Errorf("UpdateProductQuantity() error = %v", err)
					return
				}
				if !deleted {
					t.Error("UpdateProductQuantity() deleted = false, want true — the zeroing delta must report the row gone")
				}
				mu.Lock()
				deletedCount++
				mu.Unlock()
			}()
		}
		close(start)
		wg.Wait()

		if deletedCount+vanishedCount != 2 {
			t.Errorf("outcomes = %d deleted + %d vanished, want 2 total", deletedCount, vanishedCount)
		}
		if deletedCount == 0 {
			t.Error("no goroutine reported the delete — the zeroing delta must land at least once")
		}
		var count int64
		db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Error("the zeroed product survived the delete")
		}
	})

	t.Run("a replayed mutation id is a no-op", func(t *testing.T) {
		product := setup(t, 2)
		mutationID := "mutation-1"

		if _, applied, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		} else if !applied {
			t.Error("first UpdateProductQuantity() applied = false, want true")
		}
		deleted, applied, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID)
		if err != nil {
			t.Fatalf("replayed UpdateProductQuantity() error = %v", err)
		}
		if deleted {
			t.Error("replayed UpdateProductQuantity() deleted = true, want false")
		}
		if applied {
			t.Error("replayed UpdateProductQuantity() applied = true, want false — a replay the caller cannot tell apart repeats its side effects")
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 5 {
			t.Errorf("Amount = %d, want 5 — the replay applied a second time", updated.Quantity)
		}
	})

	t.Run("a different mutation id applies again", func(t *testing.T) {
		product := setup(t, 2)
		first := "mutation-1"
		second := "mutation-2"

		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &first); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &second); err != nil {
			t.Fatalf("second UpdateProductQuantity() error = %v", err)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 8 {
			t.Errorf("Amount = %d, want 8", updated.Quantity)
		}
	})

	// The bug this pins: one stored token per product only refuses a replay
	// of the most recent mutation, so M1 → M2 → replay M1 applied M1 a
	// second time. Two drains of one outbox interleave exactly that way
	// whenever one of them loses a request.
	t.Run("a replay of an older mutation id applies nothing", func(t *testing.T) {
		product := setup(t, 2)
		first := "mutation-1"
		second := "mutation-2"

		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &first); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &second); err != nil {
			t.Fatalf("second UpdateProductQuantity() error = %v", err)
		}

		deleted, applied, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &first)
		if err != nil {
			t.Fatalf("replayed UpdateProductQuantity() error = %v", err)
		}
		if deleted || applied {
			t.Errorf("UpdateProductQuantity() = (%v, %v), want (false, false)", deleted, applied)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 8 {
			t.Errorf("Amount = %d, want 8 — the superseded replay applied a second time", updated.Quantity)
		}
	})

	// A unique index on mutation_id alone would make the second call look
	// like a replay of the first and silently drop a real delta.
	t.Run("the same mutation id on two products is two claims", func(t *testing.T) {
		first := setup(t, 2)
		second := setup(t, 2)
		shared := "mutation-shared"

		if _, applied, err := repo.UpdateProductQuantity(first.ID, user.ID, 3, &shared); err != nil || !applied {
			t.Fatalf("UpdateProductQuantity() = (%v, %v), want applied", applied, err)
		}
		if _, applied, err := repo.UpdateProductQuantity(second.ID, user.ID, 3, &shared); err != nil || !applied {
			t.Fatalf("UpdateProductQuantity() on the second product = (%v, %v), want applied", applied, err)
		}
	})

	t.Run("an empty mutation id is not an idempotency token", func(t *testing.T) {
		product := setup(t, 2)
		empty := ""

		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 1, &empty); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 1, &empty); err != nil {
			t.Fatalf("second UpdateProductQuantity() error = %v", err)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 4 {
			t.Errorf("Amount = %d, want 4 — a stored empty token would have made the second call a replay", updated.Quantity)
		}
		var claims int64
		db.Model(&dbModel.ProductMutationClaim{}).Where("product_id = ?", product.ID).Count(&claims)
		if claims != 0 {
			t.Errorf("claims = %d, want 0 — an empty token must not be claimable, or a blank id dedupes every later delta", claims)
		}
	})

	// Both goroutines can read LastOfflineMutationID before either writes:
	// only the guard inside the UPDATE itself makes that interleaving safe.
	t.Run("a replay racing the original applies once", func(t *testing.T) {
		product := setup(t, 0)
		mutationID := "mutation-race"
		start := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		appliedCount := 0
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, applied, err := repo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID)
				if err != nil {
					t.Errorf("UpdateProductQuantity() error = %v", err)
				}
				if applied {
					mu.Lock()
					appliedCount++
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		if appliedCount != 1 {
			t.Errorf("applied count = %d, want 1 — the loser must report the replay", appliedCount)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 3 {
			t.Errorf("Amount = %d, want 3 — the racing replay applied a second time", updated.Quantity)
		}
	})

	// The amount moves in a single statement precisely so this holds: a
	// read-then-Save loses whichever write reads the stale amount.
	t.Run("concurrent deltas on one product all land", func(t *testing.T) {
		product := setup(t, 0)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, _, err := repo.UpdateProductQuantity(product.ID, user.ID, 1, nil); err != nil {
					t.Errorf("UpdateProductQuantity() error = %v", err)
				}
			}()
		}
		wg.Wait()

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if updated.Quantity != 8 {
			t.Errorf("Amount = %d, want 8", updated.Quantity)
		}
	})
}

// TestProductRepository_GetProductBySyncID pins the scoping the offline sync
// relies on: a client sync id identifies a product within one household, so two
// households may hold the same id and a foreign household's id never resolves.
func TestProductRepository_GetProductBySyncID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

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
		if _, err := repo.GetProductBySyncID(user.ID, ""); err != gorm.ErrRecordNotFound {
			t.Errorf("GetProductBySyncID(\"\") error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		if _, err := repo.GetProductBySyncID(user.ID, "nope"); err != gorm.ErrRecordNotFound {
			t.Errorf("GetProductBySyncID() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("the owning household resolves it", func(t *testing.T) {
		found, err := repo.GetProductBySyncID(user.ID, syncID)
		if err != nil {
			t.Fatalf("GetProductBySyncID() error = %v", err)
		}
		if found.ID != product.ID {
			t.Errorf("GetProductBySyncID() ID = %d, want %d", found.ID, product.ID)
		}
	})

	t.Run("another household with the same id is not found", func(t *testing.T) {
		if _, err := repo.GetProductBySyncID(otherUser.ID, syncID); err != gorm.ErrRecordNotFound {
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
		found, err := repo.GetProductBySyncID(user.ID, syncID)
		if err != nil {
			t.Fatalf("GetProductBySyncID() error = %v", err)
		}
		if found.ID != product.ID {
			t.Errorf("GetProductBySyncID() ID = %d, want %d", found.ID, product.ID)
		}
	})
}

func TestProductRepository_GetProductCategoryBreakdownEdges(t *testing.T) {
	seed := func(t *testing.T, categories ...string) (*gorm.DB, *ProductRepository, *authentication.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		for _, c := range categories {
			product := testutil.CreateTestProduct(db, household.ID, user.ID)
			product.Categories = c
			if err := db.Save(product).Error; err != nil {
				t.Fatalf("save product: %v", err)
			}
		}
		return db, repo, user
	}

	t.Run("empty and key-value categories", func(t *testing.T) {
		_, repo, user := seed(t, "", "en:dairy", "meat,fish", "  ")

		breakdown, err := repo.GetProductCategoryBreakdown(user.ID)
		if err != nil {
			t.Fatalf("GetProductCategoryBreakdown() error = %v", err)
		}
		if breakdown["Uncategorized"] != 2 {
			t.Errorf("Uncategorized = %d, want 2", breakdown["Uncategorized"])
		}
		if breakdown["dairy"] != 1 {
			t.Errorf("dairy = %d, want 1 (key stripped)", breakdown["dairy"])
		}
		if breakdown["meat"] != 1 {
			t.Errorf("meat = %d, want 1 (first of list)", breakdown["meat"])
		}
	})

	t.Run("more than eight categories folds the rest into Other", func(t *testing.T) {
		categories := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			categories = append(categories, fmt.Sprintf("cat-%d", i))
		}
		_, repo, user := seed(t, categories...)

		breakdown, err := repo.GetProductCategoryBreakdown(user.ID)
		if err != nil {
			t.Fatalf("GetProductCategoryBreakdown() error = %v", err)
		}
		if len(breakdown) != 9 {
			t.Fatalf("breakdown has %d keys, want 9 (8 + Other)", len(breakdown))
		}
		total := 0
		for name, count := range breakdown {
			if name != "Other" && count != 1 {
				t.Errorf("category %q = %d, want 1", name, count)
			}
			total += count
		}
		if breakdown["Other"] != 2 {
			t.Errorf("Other = %d, want 2", breakdown["Other"])
		}
		if total != 10 {
			t.Errorf("total = %d, want 10", total)
		}
	})
}

func TestProductRepository_FilteredProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	active := testutil.CreateTestProduct(db, household.ID, user.ID)
	oldActive := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(&dbModel.Product{}).Where("id = ?", oldActive.ID).Update("created_at", yesterday.Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate product: %v", err)
	}

	archived := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(archived).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	oldArchived := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(oldArchived).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", oldArchived.ID).Update("deleted_at", yesterday.Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate deleted_at: %v", err)
	}

	t.Run("active filtered without bounds returns all active", func(t *testing.T) {
		products, err := repo.GetUserActiveProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 2 {
			t.Errorf("GetUserActiveProductsFiltered() returned %d, want 2", len(products))
		}
	})

	t.Run("active filtered by from excludes older rows", func(t *testing.T) {
		products, err := repo.GetUserActiveProductsFiltered(user.ID, &yesterday, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != active.ID {
			t.Errorf("GetUserActiveProductsFiltered() = %+v, want only product %d", products, active.ID)
		}
	})

	t.Run("active filtered by to excludes newer rows", func(t *testing.T) {
		products, err := repo.GetUserActiveProductsFiltered(user.ID, nil, &yesterday)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != oldActive.ID {
			t.Errorf("GetUserActiveProductsFiltered() = %+v, want only product %d", products, oldActive.ID)
		}
	})

	t.Run("archived filtered without bounds returns all archived", func(t *testing.T) {
		products, err := repo.GetUserArchivedProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 2 {
			t.Errorf("GetUserArchivedProductsFiltered() returned %d, want 2", len(products))
		}
	})

	t.Run("archived filtered by from excludes older deletions", func(t *testing.T) {
		products, err := repo.GetUserArchivedProductsFiltered(user.ID, &yesterday, nil)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != archived.ID {
			t.Errorf("GetUserArchivedProductsFiltered() = %+v, want only product %d", products, archived.ID)
		}
	})

	t.Run("archived filtered by to excludes newer deletions", func(t *testing.T) {
		products, err := repo.GetUserArchivedProductsFiltered(user.ID, nil, &yesterday)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != oldArchived.ID {
			t.Errorf("GetUserArchivedProductsFiltered() = %+v, want only product %d", products, oldArchived.ID)
		}
	})

	t.Run("window excluding everything returns empty", func(t *testing.T) {
		products, err := repo.GetUserActiveProductsFiltered(user.ID, &tomorrow, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserActiveProductsFiltered() returned %d, want 0", len(products))
		}
	})
}

func TestProductRepository_SearchErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)
	otherLocation := testutil.CreateTestStorageLocation(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ProductName = "Searchable"
	product.StorageLocationID = &location.ID
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("invalid search parameter", func(t *testing.T) {
		if _, err := repo.SearchProducts(InvalidParameter, "x", "", "", user.ID); err != errors.ErrDatabaseInvalidSearchParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSearchParameter)
		}
	})

	t.Run("invalid sort parameter", func(t *testing.T) {
		if _, err := repo.SearchProducts(ProductName, "x", "not-a-column", "", user.ID); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})

	t.Run("invalid sort direction", func(t *testing.T) {
		if _, err := repo.SearchProducts(ProductName, "x", "product_name", "sideways", user.ID); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProducts() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})

	t.Run("projections narrowed by storage location", func(t *testing.T) {
		products, err := repo.SearchProductProjections(ProductName, "Searchable", "", "", user.ID, location.ID)
		if err != nil {
			t.Fatalf("SearchProductProjections() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != product.ID {
			t.Errorf("SearchProductProjections() = %+v, want product %d", products, product.ID)
		}

		products, err = repo.SearchProductProjections(ProductName, "Searchable", "", "", user.ID, otherLocation.ID)
		if err != nil {
			t.Fatalf("SearchProductProjections() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("SearchProductProjections() with other location returned %d, want 0", len(products))
		}
	})

	t.Run("projections reject invalid sort", func(t *testing.T) {
		if _, err := repo.SearchProductProjections(ProductName, "x", "barcode; DROP TABLE products", "", user.ID, 0); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("SearchProductProjections() error = %v, want %v", err, errors.ErrDatabaseInvalidSortParameter)
		}
	})
}

func TestProductRepository_CreateProductsBulkEdges(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("empty rows return nil", func(t *testing.T) {
		created, err := repo.CreateProductsBulk(user.ID, nil)
		if err != nil || created != nil {
			t.Errorf("CreateProductsBulk(nil) = %v, %v; want nil, nil", created, err)
		}
	})

	// The rejection is whole: no products and no locations were written.
	t.Run("too many distinct new locations is rejected whole", func(t *testing.T) {
		rows := make([]ImportedProduct, 0, util.CsvImportMaxNewLocations+1)
		for i := 0; i < util.CsvImportMaxNewLocations+1; i++ {
			rows = append(rows, ImportedProduct{
				Product:         dbModel.Product{ProductName: fmt.Sprintf("P%d", i), Barcode: fmt.Sprintf("77%011d", i)},
				NewLocationName: fmt.Sprintf("Location %d", i),
			})
		}
		_, err := repo.CreateProductsBulk(user.ID, rows)
		if err != errors.ErrImportTooManyLocations {
			t.Errorf("CreateProductsBulk() error = %v, want %v", err, errors.ErrImportTooManyLocations)
		}

		var productCount, locationCount int64
		db.Model(&dbModel.Product{}).Where(util.QueryHouseholdId, household.ID).Count(&productCount)
		db.Model(&dbModel.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount)
		if productCount != 0 || locationCount != 0 {
			t.Errorf("rejected import wrote %d products and %d locations", productCount, locationCount)
		}
	})

	t.Run("shared location names create one location", func(t *testing.T) {
		rows := []ImportedProduct{
			{Product: dbModel.Product{ProductName: "A", Barcode: "8000000000001"}, NewLocationName: "Pantry"},
			{Product: dbModel.Product{ProductName: "B", Barcode: "8000000000002"}, NewLocationName: "pantry "},
		}
		created, err := repo.CreateProductsBulk(user.ID, rows)
		if err != nil {
			t.Fatalf("CreateProductsBulk() error = %v", err)
		}
		if len(created) != 1 || created[0] != "Pantry" {
			t.Errorf("created = %v, want [Pantry]", created)
		}
		var locationCount int64
		db.Model(&dbModel.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount)
		if locationCount != 1 {
			t.Errorf("locations = %d, want 1 (case/space-insensitive dedupe)", locationCount)
		}
	})
}

func TestProductRepository_UpdateProductErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	t.Run("zero id is not implemented", func(t *testing.T) {
		if err := repo.UpdateProduct(0, user.ID, &dbModel.ProductDTOPatch{}); err != gorm.ErrNotImplemented {
			t.Errorf("UpdateProduct(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("invalid opened lifecycle is rejected", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		future := time.Now().Add(72 * time.Hour)
		if err := repo.UpdateProduct(product.ID, user.ID, &dbModel.ProductDTOPatch{OpenedAt: &future}); err != errors.ErrInvalidOpenedLifecycle {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrInvalidOpenedLifecycle)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if err := repo.UpdateProduct(9999, user.ID, &dbModel.ProductDTOPatch{}); err == nil {
			t.Error("UpdateProduct(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
		if err := repo.UpdateProduct(foreign.ID, user.ID, &dbModel.ProductDTOPatch{}); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
		privateProduct.IsPrivate = true
		if err := db.Save(privateProduct).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		if err := repo.UpdateProduct(privateProduct.ID, user.ID, &dbModel.ProductDTOPatch{}); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProduct() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("unattributed product is claimed by the caller", func(t *testing.T) {
		product := dbModel.Product{ProductName: "Orphan", Barcode: "6000000000001", HouseholdID: household.ID}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("seed orphan product: %v", err)
		}
		patch := dbModel.ProductDTOPatch{ProductName: "Claimed"}
		if err := repo.UpdateProduct(product.ID, user.ID, &patch); err != nil {
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
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("archiveOnly=false removes the row entirely", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		if err := repo.DeleteProduct(product.ID, user.ID, false); err != nil {
			t.Fatalf("DeleteProduct() error = %v", err)
		}
		var count int64
		db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).Count(&count)
		if count != 0 {
			t.Error("DeleteProduct(archiveOnly=false) left the row behind")
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if err := repo.DeleteProduct(9999, user.ID, true); err == nil {
			t.Error("DeleteProduct(9999) error = nil, want error")
		}
	})
}

func TestProductRepository_RestoreAndBulkRestore(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("restore unknown product errors", func(t *testing.T) {
		if err := repo.RestoreProduct(9999, user.ID); err == nil {
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

		errs := repo.BulkRestoreProducts([]uint{p1.ID, p2.ID}, user.ID)
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
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	t.Run("success updates the expiry", func(t *testing.T) {
		newExpiry := time.Now().Add(72 * time.Hour).Truncate(time.Second)
		expireAt := dbModel.Timestamp{Timestamp: dbModel.Date(newExpiry)}
		if err := repo.SetProductExpireAt(product.ID, user.ID, expireAt); err != nil {
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
		if err := repo.SetProductExpireAt(9999, user.ID, expireAt); err == nil {
			t.Error("SetProductExpireAt(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		expireAt := dbModel.Timestamp{Timestamp: dbModel.Date(time.Now())}
		if err := repo.SetProductExpireAt(foreign.ID, user.ID, expireAt); err != errors.ErrMismatcherUserID {
			t.Errorf("SetProductExpireAt() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})
}

func TestProductRepository_MarkProductOpenedErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("unknown user errors", func(t *testing.T) {
		if _, _, _, err := repo.MarkProductOpened(1, 9999, time.Now(), false); err == nil {
			t.Error("MarkProductOpened() with unknown user: error = nil, want error")
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, _, _, err := repo.MarkProductOpened(9999, user.ID, time.Now(), false); err == nil {
			t.Error("MarkProductOpened(9999) error = nil, want error")
		}
	})
}

func TestProductRepository_ConsumeAndWasteNotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	if err := repo.ConsumeProduct(9999, user.ID); err == nil {
		t.Error("ConsumeProduct(9999) error = nil, want error")
	}
	if err := repo.WasteProduct(9999, user.ID); err == nil {
		t.Error("WasteProduct(9999) error = nil, want error")
	}
}

func TestProductRepository_GetConsumedSamplesEmptyInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	if _, err := repo.GetConsumedSamples(1, 1, "", "", time.Now()); err == nil {
		t.Error("GetConsumedSamples() with empty barcode and name: error = nil, want error")
	}
}

func TestProductRepository_GetTopArchivedProductsEdges(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("no archived products returns empty", func(t *testing.T) {
		top, err := repo.GetTopArchivedProducts(user.ID, 10)
		if err != nil {
			t.Fatalf("GetTopArchivedProducts() error = %v", err)
		}
		if len(top) != 0 {
			t.Errorf("GetTopArchivedProducts() returned %d, want 0", len(top))
		}
	})

	t.Run("groups by barcode and respects the limit", func(t *testing.T) {
		seedArchived := func(t *testing.T, barcode string) {
			t.Helper()
			product := testutil.CreateTestProduct(db, household.ID, user.ID)
			product.Barcode = barcode
			if err := db.Save(product).Error; err != nil {
				t.Fatalf("save product: %v", err)
			}
			if err := db.Delete(product).Error; err != nil {
				t.Fatalf("delete product: %v", err)
			}
		}
		seedArchived(t, "4000000000001")
		seedArchived(t, "4000000000001")
		seedArchived(t, "4000000000002")

		top, err := repo.GetTopArchivedProducts(user.ID, 10)
		if err != nil {
			t.Fatalf("GetTopArchivedProducts() error = %v", err)
		}
		if len(top) != 2 {
			t.Fatalf("GetTopArchivedProducts() returned %d, want 2 distinct barcodes", len(top))
		}
		if top[0].Barcode != "4000000000001" {
			t.Errorf("top[0].Barcode = %q, want the twice-archived barcode", top[0].Barcode)
		}

		top, err = repo.GetTopArchivedProducts(user.ID, 1)
		if err != nil {
			t.Fatalf("GetTopArchivedProducts() error = %v", err)
		}
		if len(top) != 1 {
			t.Errorf("GetTopArchivedProducts(limit=1) returned %d, want 1", len(top))
		}
	})
}

func TestProductRepository_GetUserProductsBulkByBarcodesChunking(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)

	// More than one chunk (bulkQueryChunkSize = 500): the matching barcode
	// sits in the second chunk.
	barcodes := make([]string, 0, bulkQueryChunkSize+1)
	for i := 0; i < bulkQueryChunkSize; i++ {
		barcodes = append(barcodes, fmt.Sprintf("5%012d", i))
	}
	barcodes = append(barcodes, product.Barcode)

	products, err := repo.GetUserProductsBulkByBarcodes(user.ID, barcodes)
	if err != nil {
		t.Fatalf("GetUserProductsBulkByBarcodes() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != product.ID {
		t.Errorf("GetUserProductsBulkByBarcodes() = %+v, want product %d", products, product.ID)
	}
}

func TestProductRepository_GetActiveProductProjectionsByLocation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)

	inLocation := testutil.CreateTestProduct(db, household.ID, user.ID)
	inLocation.StorageLocationID = &location.ID
	if err := db.Save(inLocation).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	projections, err := repo.GetActiveProductProjections(user.ID, location.ID)
	if err != nil {
		t.Fatalf("GetActiveProductProjections() error = %v", err)
	}
	if len(projections) != 1 || projections[0].ID != inLocation.ID {
		t.Errorf("GetActiveProductProjections() = %+v, want only product %d", projections, inLocation.ID)
	}

	projections, err = repo.GetActiveProductProjections(user.ID, 0)
	if err != nil {
		t.Fatalf("GetActiveProductProjections() error = %v", err)
	}
	if len(projections) != 2 {
		t.Errorf("GetActiveProductProjections() without location returned %d, want 2", len(projections))
	}
}
