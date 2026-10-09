package database

import (
	stderrors "errors"
	"testing"
	"time"

	provErrors "codeberg.org/isotop7/proviant/errors"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

// TestProductRepository_TopUps covers the remaining product repository
// surface: expiry windows, amount updates, Open Food Facts cache accessors,
// and the household/user passthroughs.
func TestProductRepository_TopUps(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	soon := testutil.CreateTestProduct(db, household.ID, user.ID)
	soon.ExpireAt = time.Now().Add(3 * 24 * time.Hour)
	if err := db.Save(soon).Error; err != nil {
		t.Fatalf("failed to save product: %v", err)
	}

	far := testutil.CreateTestProduct(db, household.ID, user.ID)
	far.ExpireAt = time.Now().Add(90 * 24 * time.Hour)
	if err := db.Save(far).Error; err != nil {
		t.Fatalf("failed to save product: %v", err)
	}

	t.Run("GetExpiringInDays returns only products inside the window", func(t *testing.T) {
		products, err := repo.GetExpiringInDays(user.ID, 7)
		if err != nil {
			t.Fatalf("GetExpiringInDays() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != soon.ID {
			t.Errorf("products = %+v, want only product %d", products, soon.ID)
		}
	})

	t.Run("GetExpiringInDays with unknown user errors", func(t *testing.T) {
		if _, err := repo.GetExpiringInDays(9999, 7); err == nil {
			t.Error("GetExpiringInDays(9999) error = nil, want error")
		}
	})

	t.Run("GetExpiringSoonCount counts window entries", func(t *testing.T) {
		count, err := repo.GetExpiringSoonCount(user.ID, 7)
		if err != nil {
			t.Fatalf("GetExpiringSoonCount() error = %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("GetLastInsertedProduct returns most recent row", func(t *testing.T) {
		product, err := repo.GetLastInsertedProduct(household.ID)
		if err != nil {
			t.Fatalf("GetLastInsertedProduct() error = %v", err)
		}
		if product.ID != far.ID {
			t.Errorf("product = %d, want %d", product.ID, far.ID)
		}
	})

	t.Run("GetLastNotifiedProduct returns zero NotifiedAt when nothing notified", func(t *testing.T) {
		product, err := repo.GetLastNotifiedProduct(household.ID)
		if err != nil {
			t.Fatalf("GetLastNotifiedProduct() error = %v", err)
		}
		if !product.NotifiedAt.IsZero() {
			t.Errorf("NotifiedAt = %v, want zero when nothing notified", product.NotifiedAt)
		}
	})

	t.Run("SetProductExpireAt updates the date", func(t *testing.T) {
		newDate := databaseTimestamp(time.Now().Add(10 * 24 * time.Hour))
		if err := repo.SetProductExpireAt(soon.ID, user.ID, newDate); err != nil {
			t.Fatalf("SetProductExpireAt() error = %v", err)
		}
		var updated dbModel.Product
		if err := db.First(&updated, soon.ID).Error; err != nil {
			t.Fatalf("failed to reload product: %v", err)
		}
		if !updated.ExpireAt.Equal(time.Time(newDate.Timestamp)) {
			t.Errorf("ExpireAt = %v, want %v", updated.ExpireAt, time.Time(newDate.Timestamp))
		}
	})

	t.Run("SetProductExpireAt rejects household mismatch", func(t *testing.T) {
		outsider := testutil.CreateTestUser(db, 0)
		if err := repo.SetProductExpireAt(soon.ID, outsider.ID, databaseTimestamp(time.Now())); err != provErrors.ErrMismatcherUserID {
			t.Errorf("error = %v, want %v", err, provErrors.ErrMismatcherUserID)
		}
	})

	t.Run("UpdateProductAmount changes amount", func(t *testing.T) {
		soon.Amount = 5
		if err := db.Save(soon).Error; err != nil {
			t.Fatalf("failed to save product: %v", err)
		}

		empty, err := repo.UpdateProductAmount(soon.ID, user.ID, -3)
		if err != nil {
			t.Fatalf("UpdateProductAmount() error = %v", err)
		}
		if empty {
			t.Errorf("empty = %v, want false after partial consume", empty)
		}
		var updated dbModel.Product
		if err := db.First(&updated, soon.ID).Error; err != nil {
			t.Fatalf("failed to reload product: %v", err)
		}
		if updated.Amount != 2 {
			t.Errorf("amount = %d, want 2", updated.Amount)
		}

		empty, err = repo.UpdateProductAmount(soon.ID, user.ID, 2)
		if err != nil {
			t.Fatalf("UpdateProductAmount() error = %v", err)
		}
		if empty {
			t.Errorf("empty = %v, want false after positive delta", empty)
		}
	})

	t.Run("UpdateProductAmount with invalid id errors", func(t *testing.T) {
		if _, err := repo.UpdateProductAmount(0, user.ID, 1); err == nil {
			t.Error("UpdateProductAmount(0) error = nil, want error")
		}
	})

	t.Run("SetProductNotifiedAt stamps the product", func(t *testing.T) {
		if err := repo.SetProductNotifiedAt(soon.ID); err != nil {
			t.Fatalf("SetProductNotifiedAt() error = %v", err)
		}
		var updated dbModel.Product
		if err := db.First(&updated, soon.ID).Error; err != nil {
			t.Fatalf("failed to reload product: %v", err)
		}
		if updated.NotifiedAt.IsZero() {
			t.Error("NotifiedAt not set")
		}
		if err := repo.SetProductNotifiedAt(9999); err == nil {
			t.Error("SetProductNotifiedAt(9999) error = nil, want error")
		}
	})

	t.Run("passthrough lookups", func(t *testing.T) {
		if _, err := repo.GetUserByID(user.ID); err != nil {
			t.Errorf("GetUserByID() error = %v", err)
		}
		if _, err := repo.GetUserHouseholdByID(user.ID); err != nil {
			t.Errorf("GetUserHouseholdByID() error = %v", err)
		}
		if _, err := repo.GetHouseholdByID(household.ID); err != nil {
			t.Errorf("GetHouseholdByID() error = %v", err)
		}
		users, err := repo.GetUsersByHouseholdID(household.ID)
		if err != nil || len(users) != 1 {
			t.Errorf("GetUsersByHouseholdID() = %v, %v; want 1 user", users, err)
		}
	})

	t.Run("GetUserActiveProductsFiltered with and without date bounds", func(t *testing.T) {
		all, err := repo.GetUserActiveProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(all) != 2 {
			t.Errorf("all = %d, want 2", len(all))
		}

		future := time.Now().Add(60 * 24 * time.Hour)
		bounded, err := repo.GetUserActiveProductsFiltered(user.ID, &future, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered(bounded) error = %v", err)
		}
		if len(bounded) != 0 {
			t.Errorf("bounded = %d, want 0", len(bounded))
		}
	})

	t.Run("GetUserArchivedProductsFiltered", func(t *testing.T) {
		if err := repo.DeleteProduct(far.ID, user.ID, true); err != nil {
			t.Fatalf("DeleteProduct() error = %v", err)
		}
		archived, err := repo.GetUserArchivedProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(archived) != 1 {
			t.Errorf("archived = %d, want 1", len(archived))
		}

		top, err := repo.GetTopArchivedProducts(user.ID, 10)
		if err != nil {
			t.Fatalf("GetTopArchivedProducts() error = %v", err)
		}
		if len(top) != 1 {
			t.Errorf("top = %d, want 1", len(top))
		}
	})
}

func TestProductRepository_OpenFoodFactsCache(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	entry := dbModel.OpenFoodFactsCache{
		Barcode:     "1234567890123",
		ProductName: "Cached Product",
	}
	if err := repo.CreateOpenFoodFactsCache(&entry); err != nil {
		t.Fatalf("CreateOpenFoodFactsCache() error = %v", err)
	}

	t.Run("GetOpenFoodFactsCacheByBarcode", func(t *testing.T) {
		got, err := repo.GetOpenFoodFactsCacheByBarcode("1234567890123")
		if err != nil || got.Barcode != "1234567890123" {
			t.Errorf("got = %v, %v; want barcode entry", got, err)
		}
		if _, err := repo.GetOpenFoodFactsCacheByBarcode("missing"); err == nil {
			t.Error("missing barcode: error = nil, want error")
		}
	})

	t.Run("GetOpenFoodFactsCachesByBarcodes batches and handles empty input", func(t *testing.T) {
		got, err := repo.GetOpenFoodFactsCachesByBarcodes([]string{"1234567890123", "9999999999999"})
		if err != nil {
			t.Fatalf("GetOpenFoodFactsCachesByBarcodes() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got = %d, want 1", len(got))
		}
		empty, err := repo.GetOpenFoodFactsCachesByBarcodes(nil)
		if err != nil || len(empty) != 0 {
			t.Errorf("empty input = %v, %v; want empty result", empty, err)
		}
	})

	t.Run("storage hint and image URL updates", func(t *testing.T) {
		if err := repo.UpdateOpenFoodFactsCacheImageURL("1234567890123", "https://example.com/img.jpg"); err != nil {
			t.Errorf("UpdateOpenFoodFactsCacheImageURL() error = %v", err)
		}
		if err := repo.UpdateOpenFoodFactsCacheStorageHint("1234567890123", "Fridge"); err != nil {
			t.Errorf("UpdateOpenFoodFactsCacheStorageHint() error = %v", err)
		}

		withoutHint, err := repo.GetOpenFoodFactsCacheWithoutStorageHint()
		if err != nil {
			t.Fatalf("GetOpenFoodFactsCacheWithoutStorageHint() error = %v", err)
		}
		if len(withoutHint) != 0 {
			t.Errorf("withoutHint = %d, want 0 after hint set", len(withoutHint))
		}

		withRemote, err := repo.GetOpenFoodFactsCacheWithRemoteImageURL()
		if err != nil {
			t.Fatalf("GetOpenFoodFactsCacheWithRemoteImageURL() error = %v", err)
		}
		if len(withRemote) != 1 {
			t.Errorf("withRemote = %d, want 1", len(withRemote))
		}
	})
}

func TestProductRepository_MarkProductOpened(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("first open succeeds", func(t *testing.T) {
		openedAt := time.Now()
		current, _, changed, err := repo.MarkProductOpened(product.ID, user.ID, openedAt, false)
		if err != nil {
			t.Fatalf("MarkProductOpened() error = %v", err)
		}
		if !changed {
			t.Error("changed = false, want true on first open")
		}
		if current.OpenedAt == nil {
			t.Error("OpenedAt not persisted")
		}
	})

	t.Run("second open without force reports conflict", func(t *testing.T) {
		_, _, changed, err := repo.MarkProductOpened(product.ID, user.ID, time.Now(), false)
		if err != nil {
			t.Fatalf("MarkProductOpened() error = %v", err)
		}
		if changed {
			t.Error("changed = true, want false for already-opened product without force")
		}
	})

	t.Run("reopen with force succeeds", func(t *testing.T) {
		_, _, changed, err := repo.MarkProductOpened(product.ID, user.ID, time.Now(), true)
		if err != nil {
			t.Fatalf("MarkProductOpened(force) error = %v", err)
		}
		if !changed {
			t.Error("changed = false, want true with force")
		}
	})

	t.Run("open for foreign user errors", func(t *testing.T) {
		outsider := testutil.CreateTestUser(db, 0)
		if _, _, _, err := repo.MarkProductOpened(product.ID, outsider.ID, time.Now(), true); err == nil {
			t.Error("foreign user open: error = nil, want error")
		}
	})
}

func TestBulkOperationError(t *testing.T) {
	cause := stderrors.New("boom")
	opErr := NewBulkOperationError(42, cause)
	if opErr.ProductID() != 42 {
		t.Errorf("ProductID() = %d, want 42", opErr.ProductID())
	}
	if opErr.Error() == "" {
		t.Error("Error() = empty string")
	}
	if opErr.Err() != cause {
		t.Errorf("Err() = %v, want %v", opErr.Err(), cause)
	}
}

func databaseTimestamp(t time.Time) dbModel.Timestamp {
	return dbModel.Timestamp{Timestamp: dbModel.Date(t)}
}
