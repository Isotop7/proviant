package database

import (
	stderrors "errors"
	"sync"
	"testing"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestProductRepository_ConsumeProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "To Consume",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	err := stockRepo.ConsumeProduct(product.ID, user.ID)
	if err != nil {
		t.Fatalf("ConsumeProduct() error = %v", err)
	}

	var consumed dbModel.Product
	err = db.Unscoped().First(&consumed, product.ID).Error
	if err != nil {
		t.Fatalf("Product not found after consume: %v", err)
	}
	if !consumed.DeletedAt.Valid {
		t.Error("ConsumeProduct() did not soft-delete the product")
	}
	if consumed.RemovalReason != dbModel.RemovalReasonConsumed {
		t.Errorf("ConsumeProduct() RemovalReason = %q, want %q", consumed.RemovalReason, dbModel.RemovalReasonConsumed)
	}
}
func TestProductRepository_ConsumeProductPartial(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *ProductStockRepository, *authentication.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		stockRepo := NewProductStockRepository(db)

		household := dbModel.Household{Name: "Test Household"}
		db.Create(&household)

		user := authentication.User{
			Username:    "testuser",
			Password:    "password",
			MailAddress: "test@example.com",
			HouseholdID: household.ID,
		}
		db.Create(&user)
		return db, stockRepo, &user
	}

	createProduct := func(db *gorm.DB, householdID uint, name string, amount int) dbModel.Product {
		product := dbModel.Product{
			ProductName: name,
			Barcode:     "1234567890123",
			HouseholdID: householdID,
			Quantity:    amount,
		}
		db.Create(&product)
		return product
	}

	t.Run("partial reduce keeps product active", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Partial", 5)

		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&product, 2, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 2 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 2", consumed)
		}
		if fullyConsumed {
			t.Error("partial consume must not report fullyConsumed")
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("product not found after partial consume: %v", err)
		}
		if updated.Quantity != 3 {
			t.Errorf("Amount = %d, want 3", updated.Quantity)
		}
		if updated.DeletedAt.Valid {
			t.Error("partial consume must not soft-delete the product")
		}
	})

	t.Run("amount equal to current amount fully consumes", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Equal", 3)

		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&product, 3, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 3 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 3", consumed)
		}
		if !fullyConsumed {
			t.Error("full consume must report fullyConsumed")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid {
			t.Error("full consume did not soft-delete the product")
		}
		if archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Errorf("RemovalReason = %q, want %q", archived.RemovalReason, dbModel.RemovalReasonConsumed)
		}
	})

	t.Run("amount above current amount clamps to full consume", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Overshoot", 2)

		consumed, _, _, err := stockRepo.ConsumeProductPartial(&product, 5, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 2 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 2", consumed)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("overshoot must fully consume (soft-delete with reason consumed)")
		}
	})

	t.Run("amount zero fully consumes", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Zero", 4)

		consumed, _, _, err := stockRepo.ConsumeProductPartial(&product, 0, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 4 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 4", consumed)
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid {
			t.Error("amount 0 must fully consume the product")
		}
	})

	t.Run("legacy row without GORM timestamps still consumes", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		// Rows written by raw SQL or predating the timestamp columns do not
		// round-trip through the driver's time binding; the guarded writes
		// must still apply to them.
		product := createProduct(db, user.HouseholdID, "Legacy", 3)
		if err := db.Exec("UPDATE products SET updated_at = NULL WHERE id = ?", product.ID).Error; err != nil {
			t.Fatalf("raw update: %v", err)
		}

		var current dbModel.Product
		if err := db.First(&current, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&current, 3, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 3 || !fullyConsumed {
			t.Errorf("consumed/fullyConsumed = %d/%v, want 3/true", consumed, fullyConsumed)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, stockRepo, _ := setup(t)

		missing := dbModel.Product{Quantity: 1}
		missing.ID = 9999
		consumed, _, _, err := stockRepo.ConsumeProductPartial(&missing, 1, nil)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("ConsumeProductPartial() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
		if consumed != 0 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 0", consumed)
		}
	})

	t.Run("stale fast path falls back when stock dropped below requested amount", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Raced", 5)
		// Simulate another request having consumed units before our update runs
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("quantity", 1)

		stale := product

		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&stale, 2, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 1 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 1 (remaining stock)", consumed)
		}
		if !fullyConsumed {
			t.Error("fallback after concurrent reduction must fully consume remaining stock")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after fallback consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("fallback must soft-delete with reason consumed")
		}
	})

	t.Run("stale read above current amount consumes remaining", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Stale clamp", 5)
		// Another request reduced the stock after the caller's read
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("quantity", 1)

		stale := product

		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&stale, 5, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 1 {
			t.Errorf("ConsumeProductPartial() consumed = %d, want 1 (current stock, not stale read)", consumed)
		}
		if !fullyConsumed {
			t.Error("overshoot against current stock must fully consume")
		}

		var archived dbModel.Product
		if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
			t.Fatalf("product not found after full consume: %v", err)
		}
		if !archived.DeletedAt.Valid || archived.RemovalReason != dbModel.RemovalReasonConsumed {
			t.Error("stale overshoot must soft-delete with reason consumed")
		}
	})

	t.Run("stale read falls back to partial reduce", func(t *testing.T) {
		db, stockRepo, user := setup(t)
		product := createProduct(db, user.HouseholdID, "Stale partial", 3)
		// Another request increased the stock after the caller's read, so
		// the requested amount no longer covers the whole stock
		db.Model(&dbModel.Product{}).Where("id = ?", product.ID).Update("quantity", 5)

		stale := product

		consumed, fullyConsumed, _, err := stockRepo.ConsumeProductPartial(&stale, 3, nil)
		if err != nil {
			t.Fatalf("ConsumeProductPartial() error = %v", err)
		}
		if consumed != 3 || fullyConsumed {
			t.Errorf("ConsumeProductPartial() = (%d, %v), want (3, false)", consumed, fullyConsumed)
		}

		var updated dbModel.Product
		if err := db.First(&updated, product.ID).Error; err != nil {
			t.Fatalf("product not found after partial consume: %v", err)
		}
		if updated.Quantity != 2 {
			t.Errorf("Amount = %d, want 2", updated.Quantity)
		}
		if updated.DeletedAt.Valid {
			t.Error("fallback partial reduce must not soft-delete the product")
		}
	})
}
func TestProductRepository_WasteProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "To Waste",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	err := stockRepo.WasteProduct(product.ID, user.ID, "")
	if err != nil {
		t.Fatalf("WasteProduct() error = %v", err)
	}

	var wasted dbModel.Product
	err = db.Unscoped().First(&wasted, product.ID).Error
	if err != nil {
		t.Fatalf("WasteProduct() hard-deleted the product — record must be retained for reporting")
	}
	if !wasted.DeletedAt.Valid {
		t.Error("WasteProduct() did not soft-delete the product")
	}
	if wasted.RemovalReason != dbModel.RemovalReasonWasted {
		t.Errorf("WasteProduct() RemovalReason = %q, want %q", wasted.RemovalReason, dbModel.RemovalReasonWasted)
	}
}
func TestProductRepository_WasteProduct_ConcurrentModification(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "To Waste",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)

	// Simulate a concurrent request claiming the row between the repository's
	// read and its guarded write: set a removal reason before any UPDATE on
	// products runs, so the unclaimed-row precondition fails.
	db.Callback().Update().Before("gorm:before_update").Register("test:concurrent_write", func(tx *gorm.DB) {
		_, _ = tx.Statement.ConnPool.ExecContext(tx.Statement.Context,
			"UPDATE products SET removal_reason = ? WHERE id = ?", dbModel.RemovalReasonConsumed, product.ID)
	})

	err := stockRepo.WasteProduct(product.ID, user.ID, "")
	if err != errors.ErrProductConcurrentModification {
		t.Fatalf("WasteProduct() error = %v, want %v", err, errors.ErrProductConcurrentModification)
	}

	var untouched dbModel.Product
	if getErr := db.Unscoped().First(&untouched, product.ID).Error; getErr != nil {
		t.Fatalf("product record vanished: %v", getErr)
	}
	if untouched.DeletedAt.Valid {
		t.Error("concurrent-modification failure must roll back the soft-delete")
	}
	if untouched.RemovalReason == dbModel.RemovalReasonWasted {
		t.Error("concurrent-modification failure must roll back the removal-reason update")
	}
}

func TestProductRepository_UpdateProductQuantity(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)
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
		if _, _, err := stockRepo.UpdateProductQuantity(0, user.ID, 1, nil); err != gorm.ErrNotImplemented {
			t.Errorf("UpdateProductQuantity(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, _, err := stockRepo.UpdateProductQuantity(9999, user.ID, 1, nil); err == nil {
			t.Error("UpdateProductQuantity(9999) error = nil, want error")
		}
	})

	t.Run("cross-household is rejected", func(t *testing.T) {
		foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
		if _, _, err := stockRepo.UpdateProductQuantity(foreign.ID, user.ID, 1, nil); err != errors.ErrMismatcherUserID {
			t.Errorf("UpdateProductQuantity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("positive delta increases amount", func(t *testing.T) {
		product := setup(t, 2)
		deleted, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, nil)
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
		deleted, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, -5, nil)
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
		for range 2 {
			wg.Go(func() {
				<-start
				deleted, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, -2, nil)
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
			})
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

		if _, applied, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		} else if !applied {
			t.Error("first UpdateProductQuantity() applied = false, want true")
		}
		deleted, applied, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID)
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

		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &first); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &second); err != nil {
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

		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &first); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &second); err != nil {
			t.Fatalf("second UpdateProductQuantity() error = %v", err)
		}

		deleted, applied, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &first)
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

		if _, applied, err := stockRepo.UpdateProductQuantity(first.ID, user.ID, 3, &shared); err != nil || !applied {
			t.Fatalf("UpdateProductQuantity() = (%v, %v), want applied", applied, err)
		}
		if _, applied, err := stockRepo.UpdateProductQuantity(second.ID, user.ID, 3, &shared); err != nil || !applied {
			t.Fatalf("UpdateProductQuantity() on the second product = (%v, %v), want applied", applied, err)
		}
	})

	t.Run("an empty mutation id is not an idempotency token", func(t *testing.T) {
		product := setup(t, 2)
		empty := ""

		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 1, &empty); err != nil {
			t.Fatalf("first UpdateProductQuantity() error = %v", err)
		}
		if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 1, &empty); err != nil {
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
		for range 2 {
			wg.Go(func() {
				<-start
				_, applied, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 3, &mutationID)
				if err != nil {
					t.Errorf("UpdateProductQuantity() error = %v", err)
				}
				if applied {
					mu.Lock()
					appliedCount++
					mu.Unlock()
				}
			})
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
		for range 8 {
			wg.Go(func() {
				if _, _, err := stockRepo.UpdateProductQuantity(product.ID, user.ID, 1, nil); err != nil {
					t.Errorf("UpdateProductQuantity() error = %v", err)
				}
			})
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
func TestProductRepository_ConsumeAndWasteNotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	if err := stockRepo.ConsumeProduct(9999, user.ID); err == nil {
		t.Error("ConsumeProduct(9999) error = nil, want error")
	}
	if err := stockRepo.WasteProduct(9999, user.ID, ""); err == nil {
		t.Error("WasteProduct(9999) error = nil, want error")
	}
}

func TestProductRepository_BulkConsumeProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	p1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	p2 := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("consumes valid ids and reports failures", func(t *testing.T) {
		errs := stockRepo.BulkConsumeProducts([]uint{p1.ID, p2.ID, 9999}, user.ID)
		if len(errs) != 1 {
			t.Fatalf("BulkConsumeProducts() returned %d errors, want 1", len(errs))
		}
		if errs[0].ProductID() != 9999 {
			t.Errorf("errs[0].ProductID() = %d, want 9999", errs[0].ProductID())
		}
		for _, id := range []uint{p1.ID, p2.ID} {
			var consumed dbModel.Product
			if err := db.Unscoped().First(&consumed, id).Error; err != nil {
				t.Fatalf("reload product %d: %v", id, err)
			}
			if !consumed.DeletedAt.Valid || consumed.RemovalReason != dbModel.RemovalReasonConsumed {
				t.Errorf("product %d not consumed: %+v", id, consumed)
			}
		}
	})

	t.Run("empty input returns no errors", func(t *testing.T) {
		if errs := stockRepo.BulkConsumeProducts(nil, user.ID); len(errs) != 0 {
			t.Errorf("BulkConsumeProducts(nil) returned %d errors, want 0", len(errs))
		}
	})
}
func TestProductRepository_BulkWasteProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	stockRepo := NewProductStockRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	p1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	p2 := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("wastes valid ids and reports failures", func(t *testing.T) {
		errs := stockRepo.BulkWasteProducts([]uint{p1.ID, 9999, p2.ID}, user.ID, "")
		if len(errs) != 1 {
			t.Fatalf("BulkWasteProducts() returned %d errors, want 1", len(errs))
		}
		if errs[0].ProductID() != 9999 {
			t.Errorf("errs[0].ProductID() = %d, want 9999", errs[0].ProductID())
		}
		for _, id := range []uint{p1.ID, p2.ID} {
			var wasted dbModel.Product
			if err := db.Unscoped().First(&wasted, id).Error; err != nil {
				t.Fatalf("reload product %d: %v", id, err)
			}
			if !wasted.DeletedAt.Valid || wasted.RemovalReason != dbModel.RemovalReasonWasted {
				t.Errorf("product %d not wasted: %+v", id, wasted)
			}
		}
	})

	t.Run("empty input returns no errors", func(t *testing.T) {
		if errs := stockRepo.BulkWasteProducts(nil, user.ID, ""); len(errs) != 0 {
			t.Errorf("BulkWasteProducts(nil) returned %d errors, want 0", len(errs))
		}
	})
}
