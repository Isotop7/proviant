package database

import (
	stderrors "errors"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// failQueriesOnTable registers a GORM query callback that fails every query
// against the given table, so repository error branches that a healthy
// database never reaches (query/count failures) can be tested. The callback
// is removed when the test ends.
func failQueriesOnTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	name := "test:fail_query_" + table
	db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(stderrors.New("injected query failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Query().Remove(name) })
}

func failUpdatesOnTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	name := "test:fail_update_" + table
	db.Callback().Update().Before("gorm:before_update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(stderrors.New("injected update failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Update().Remove(name) })
}

func failCreatesOnTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	name := "test:fail_create_" + table
	db.Callback().Create().Before("gorm:before_create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(stderrors.New("injected create failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Create().Remove(name) })
}

func failDeletesOnTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	name := "test:fail_delete_" + table
	db.Callback().Delete().Before("gorm:before_delete").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(stderrors.New("injected delete failure"))
		}
	})
	t.Cleanup(func() { db.Callback().Delete().Remove(name) })
}

// failNthQueryOnTable fails only the n-th query against the given table, so
// error branches behind earlier successful queries can be targeted.
func failNthQueryOnTable(t *testing.T, db *gorm.DB, table string, n int) {
	t.Helper()
	name := "test:fail_nth_query_" + table
	count := 0
	db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			count++
			if count == n {
				_ = tx.AddError(stderrors.New("injected query failure"))
			}
		}
	})
	t.Cleanup(func() { db.Callback().Query().Remove(name) })
}

func TestProductRepository_QueryErrorInjection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)
	expireAt := databaseTimestamp(time.Now().Add(24 * time.Hour))

	t.Run("list and count queries surface query errors", func(t *testing.T) {
		failQueriesOnTable(t, db, "products")

		if _, err := repo.GetUserProductsBulk(user.ID, 0); err == nil {
			t.Error("GetUserProductsBulk() error = nil")
		}
		if _, err := repo.GetUserProductsByIDs(user.ID, []uint{product.ID}); err == nil {
			t.Error("GetUserProductsByIDs() error = nil")
		}
		if _, err := repo.GetUserArchivedProductsBulk(user.ID, 0); err == nil {
			t.Error("GetUserArchivedProductsBulk() error = nil")
		}
		if _, err := repo.GetActiveProductProjections(user.ID, 0); err == nil {
			t.Error("GetActiveProductProjections() error = nil")
		}
		if _, err := repo.GetArchivedProductProjections(user.ID); err == nil {
			t.Error("GetArchivedProductProjections() error = nil")
		}
		if _, err := repo.GetUserArchivedProductsByIDs(user.ID, []uint{product.ID}); err == nil {
			t.Error("GetUserArchivedProductsByIDs() error = nil")
		}
		if _, err := repo.GetUserProductsBulkByBarcode(user.ID, 1234567890123); err == nil {
			t.Error("GetUserProductsBulkByBarcode() error = nil")
		}
		if _, err := repo.GetUserProductsBulkByBarcodes(user.ID, []string{"1234567890123"}); err == nil {
			t.Error("GetUserProductsBulkByBarcodes() error = nil")
		}
		if _, err := repo.SearchProducts(ProductName, "x", "", "", user.ID); err == nil {
			t.Error("SearchProducts() error = nil")
		}
		if _, err := repo.SearchProductProjections(ProductName, "x", "", "", user.ID, 0); err == nil {
			t.Error("SearchProductProjections() error = nil")
		}
		if _, err := repo.GetExpiredProductsCount(user.ID); err == nil {
			t.Error("GetExpiredProductsCount() error = nil")
		}
		if _, err := repo.GetActiveProductsCount(user.ID); err == nil {
			t.Error("GetActiveProductsCount() error = nil")
		}
		if _, err := repo.GetArchivedProductsCount(user.ID); err == nil {
			t.Error("GetArchivedProductsCount() error = nil")
		}
		if _, err := repo.GetUniqueArchivedProductsCount(user.ID); err == nil {
			t.Error("GetUniqueArchivedProductsCount() error = nil")
		}
		if _, err := repo.GetProductCategoryBreakdown(user.ID); err == nil {
			t.Error("GetProductCategoryBreakdown() error = nil")
		}
		if _, err := repo.GetExpiryTrend(user.ID); err == nil {
			t.Error("GetExpiryTrend() error = nil")
		}
		if _, err := repo.GetExpiringSoonProducts(user.ID, 7); err == nil {
			t.Error("GetExpiringSoonProducts() error = nil")
		}
		if _, err := repo.GetExpiringInDays(user.ID, 7); err == nil {
			t.Error("GetExpiringInDays() error = nil")
		}
		if _, err := repo.GetExpiringSoonCount(user.ID, 7); err == nil {
			t.Error("GetExpiringSoonCount() error = nil")
		}
		if _, _, err := repo.GetActiveExpiryCounts(user.ID, time.Now(), 7); err == nil {
			t.Error("GetActiveExpiryCounts() error = nil")
		}
		if _, err := repo.GetWasteThisMonth(user.ID); err == nil {
			t.Error("GetWasteThisMonth() error = nil")
		}
		if _, err := repo.GetSubThresholdProducts(user.ID); err == nil {
			t.Error("GetSubThresholdProducts() error = nil")
		}
		if _, err := repo.GetUserActiveProductsFiltered(user.ID, &from, &to); err == nil {
			t.Error("GetUserActiveProductsFiltered() error = nil")
		}
		if _, err := repo.GetUserArchivedProductsFiltered(user.ID, &from, &to); err == nil {
			t.Error("GetUserArchivedProductsFiltered() error = nil")
		}
		if _, err := repo.GetExpiringProductsByHousehold(household.ID, 7); err == nil {
			t.Error("GetExpiringProductsByHousehold() error = nil")
		}
		if _, err := repo.GetExpiringProductsForMailDigest(household.ID); err == nil {
			t.Error("GetExpiringProductsForMailDigest() error = nil")
		}
		if _, err := repo.GetConsumedSamples(household.ID, user.ID, "1234567890123", "", time.Now()); err == nil {
			t.Error("GetConsumedSamples() error = nil")
		}
	})

	t.Run("write paths surface write errors", func(t *testing.T) {
		failUpdatesOnTable(t, db, "products")

		if err := repo.UpdateProduct(product.ID, user.ID, &dbModel.ProductDTOPatch{ProductName: "Patched"}); err == nil {
			t.Error("UpdateProduct() error = nil")
		}
		if _, err := repo.UpdateProductAmount(product.ID, user.ID, 1); err == nil {
			t.Error("UpdateProductAmount() error = nil")
		}
		if err := repo.RestoreProduct(product.ID, user.ID); err == nil {
			t.Error("RestoreProduct() error = nil")
		}
		if errs := repo.BulkRestoreProducts([]uint{product.ID}, user.ID); len(errs) != 1 {
			t.Errorf("BulkRestoreProducts() returned %d errors, want 1", len(errs))
		}
		if err := repo.SetProductExpireAt(product.ID, user.ID, expireAt); err == nil {
			t.Error("SetProductExpireAt() error = nil")
		}
		if err := repo.SetProductNotifiedAt(product.ID); err == nil {
			t.Error("SetProductNotifiedAt() error = nil")
		}
		if err := repo.ConsumeProduct(product.ID, user.ID); err == nil {
			t.Error("ConsumeProduct() save error = nil")
		}
		if err := repo.WasteProduct(product.ID, user.ID); err == nil {
			t.Error("WasteProduct() error = nil")
		}
		if errs := repo.BulkWasteProducts([]uint{product.ID}, user.ID); len(errs) != 1 {
			t.Errorf("BulkWasteProducts() returned %d errors, want 1", len(errs))
		}
	})

	t.Run("create and delete paths surface write errors", func(t *testing.T) {
		failCreatesOnTable(t, db, "products")
		if err := repo.CreateProduct(user.ID, &dbModel.Product{ProductName: "Injected"}); err == nil {
			t.Error("CreateProduct() error = nil")
		}
		if _, err := repo.CreateProductsBulk(user.ID, []ImportedProduct{{Product: dbModel.Product{ProductName: "Injected"}}}); err == nil {
			t.Error("CreateProductsBulk() error = nil")
		}

		failDeletesOnTable(t, db, "products")
		if err := repo.DeleteProduct(product.ID, user.ID, true); err == nil {
			t.Error("DeleteProduct() error = nil")
		}
		if err := repo.ConsumeProduct(product.ID, user.ID); err == nil {
			t.Error("ConsumeProduct() delete error = nil")
		}
	})

	t.Run("bulk import fails when the location count query fails", func(t *testing.T) {
		failQueriesOnTable(t, db, "storage_locations")
		if _, err := repo.CreateProductsBulk(user.ID, []ImportedProduct{{Product: dbModel.Product{ProductName: "X"}}}); err == nil {
			t.Error("CreateProductsBulk() error = nil")
		}
	})

	t.Run("bulk import fails when location creation fails", func(t *testing.T) {
		failCreatesOnTable(t, db, "storage_locations")
		rows := []ImportedProduct{{Product: dbModel.Product{ProductName: "X"}, NewLocationName: "NewLoc"}}
		if _, err := repo.CreateProductsBulk(user.ID, rows); err == nil {
			t.Error("CreateProductsBulk() error = nil")
		}
	})

	t.Run("open food facts batch query surfaces errors", func(t *testing.T) {
		failQueriesOnTable(t, db, "open_food_facts_caches")
		if _, err := repo.GetOpenFoodFactsCachesByBarcodes([]string{"1"}); err == nil {
			t.Error("GetOpenFoodFactsCachesByBarcodes() error = nil")
		}
	})
}

func TestProductRepository_ConsumeProductPartialErrors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	seed := func(t *testing.T, amount int) dbModel.Product {
		t.Helper()
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Amount = amount
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		return *product
	}

	t.Run("fast-path update failure surfaces", func(t *testing.T) {
		product := seed(t, 5)
		failUpdatesOnTable(t, db, "products")
		if _, _, err := repo.ConsumeProductPartial(&product, 2); err == nil {
			t.Error("ConsumeProductPartial() error = nil, want injected failure")
		}
	})

	t.Run("transactional partial update failure surfaces", func(t *testing.T) {
		// Stale read (3) below current stock (5) reaches the transactional
		// partial-reduce branch; the guarded UPDATE then fails.
		product := seed(t, 5)
		stale := product
		stale.Amount = 3
		failUpdatesOnTable(t, db, "products")
		if _, _, err := repo.ConsumeProductPartial(&stale, 3); err == nil {
			t.Error("ConsumeProductPartial() error = nil, want injected failure")
		}
	})

	t.Run("full-consume flag update failure surfaces", func(t *testing.T) {
		product := seed(t, 2)
		failUpdatesOnTable(t, db, "products")
		if _, _, err := repo.ConsumeProductPartial(&product, 5); err == nil {
			t.Error("ConsumeProductPartial() error = nil, want injected failure")
		}
		var reloaded dbModel.Product
		if err := db.First(&reloaded, product.ID).Error; err != nil {
			t.Fatalf("reload product: %v", err)
		}
		if reloaded.DeletedAt.Valid {
			t.Error("failed full consume must not archive the product")
		}
	})

	t.Run("concurrent modification of the guarded write is reported", func(t *testing.T) {
		product := seed(t, 5)
		stale := product
		stale.Amount = 3
		// Simulate a concurrent write landing between the transactional
		// re-read and the guarded UPDATE: bump updated_at so the
		// optimistic-lock precondition fails.
		db.Callback().Update().Before("gorm:before_update").Register("test:consume_partial_concurrent", func(tx *gorm.DB) {
			if tx.Statement.Table != "products" {
				return
			}
			_, _ = tx.Statement.ConnPool.ExecContext(tx.Statement.Context,
				"UPDATE products SET updated_at = ? WHERE id = ?", time.Now(), product.ID)
		})
		t.Cleanup(func() { db.Callback().Update().Remove("test:consume_partial_concurrent") })

		_, _, err := repo.ConsumeProductPartial(&stale, 3)
		if err != errors.ErrProductConcurrentModification {
			t.Errorf("ConsumeProductPartial() error = %v, want %v", err, errors.ErrProductConcurrentModification)
		}
	})
}

func TestInvitationRepository_ErrorInjection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	t.Run("user lookup failure is not swallowed as not-found", func(t *testing.T) {
		failQueriesOnTable(t, db, "users")
		_, err := repo.CreateInvitation(household.ID, inviter.ID, "inject@example.com")
		if err == nil {
			t.Error("CreateInvitation() error = nil, want injected failure")
		}
	})

	t.Run("existing-invitation lookup failure is not swallowed as not-found", func(t *testing.T) {
		failQueriesOnTable(t, db, "household_invitations")
		_, err := repo.CreateInvitation(household.ID, inviter.ID, "inject@example.com")
		if err == nil {
			t.Error("CreateInvitation() error = nil, want injected failure")
		}
	})

	t.Run("invitation insert failure surfaces", func(t *testing.T) {
		failCreatesOnTable(t, db, "household_invitations")
		_, err := repo.CreateInvitation(household.ID, inviter.ID, "inject@example.com")
		if err == nil {
			t.Error("CreateInvitation() error = nil, want injected failure")
		}
	})

	t.Run("token lookup failure other than not-found surfaces", func(t *testing.T) {
		failQueriesOnTable(t, db, "household_invitations")
		if _, err := repo.GetInvitationByToken("any"); err == nil {
			t.Error("GetInvitationByToken() error = nil, want injected failure")
		}
	})

	t.Run("accept rolls back when the user update fails", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "accept-inject@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		invitee := testutil.CreateTestUser(db, 0)

		failUpdatesOnTable(t, db, "users")
		if err := repo.AcceptInvitation(inv.Token, "accept-inject@example.com", invitee.ID); err == nil {
			t.Error("AcceptInvitation() error = nil, want injected failure")
		}
		var reloaded dbModel.HouseholdInvitation
		if err := db.First(&reloaded, inv.ID).Error; err != nil {
			t.Fatalf("reload invitation: %v", err)
		}
		if reloaded.Status != dbModel.InvitationStatusPending {
			t.Errorf("invitation Status = %q, want pending after rollback", reloaded.Status)
		}
	})

	t.Run("accept rolls back when the invitation update fails", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "accept-inject2@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		invitee := testutil.CreateTestUser(db, 0)

		failUpdatesOnTable(t, db, "household_invitations")
		if err := repo.AcceptInvitation(inv.Token, "accept-inject2@example.com", invitee.ID); err == nil {
			t.Error("AcceptInvitation() error = nil, want injected failure")
		}
	})

	t.Run("cancel surfaces update failure", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "cancel-inject@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		failUpdatesOnTable(t, db, "household_invitations")
		if err := repo.CancelInvitation(inv.ID, inviter.ID); err == nil {
			t.Error("CancelInvitation() error = nil, want injected failure")
		}
	})

	t.Run("tx variant surfaces insert failure", func(t *testing.T) {
		failCreatesOnTable(t, db, "household_invitations")
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx-inject@example.com")
			return err
		})
		if err == nil {
			t.Error("CreateInvitationTx() error = nil, want injected failure")
		}
	})

	t.Run("tx variant surfaces lookup failures", func(t *testing.T) {
		failQueriesOnTable(t, db, "users")
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx-inject2@example.com")
			return err
		})
		if err == nil {
			t.Error("CreateInvitationTx() user lookup error = nil, want injected failure")
		}
	})

	t.Run("tx variant surfaces existing-invitation lookup failure", func(t *testing.T) {
		failQueriesOnTable(t, db, "household_invitations")
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx-inject3@example.com")
			return err
		})
		if err == nil {
			t.Error("CreateInvitationTx() invitation lookup error = nil, want injected failure")
		}
	})

	t.Run("cancel surfaces member lookup failure", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "cancel-inject2@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		failQueriesOnTable(t, db, "users")
		if err := repo.CancelInvitation(inv.ID, inviter.ID); err == nil {
			t.Error("CancelInvitation() error = nil, want injected failure")
		}
	})
}

func TestPATRepository_ErrorInjection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewPATRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("query and write failures surface", func(t *testing.T) {
		failQueriesOnTable(t, db, "personal_access_tokens")
		if _, err := repo.GetPATsByUserID(user.ID); err == nil {
			t.Error("GetPATsByUserID() error = nil")
		}
		if _, err := repo.GetPATByTokenHash("hash"); err == nil {
			t.Error("GetPATByTokenHash() error = nil, want error")
		}
		if _, err := repo.GetPATByID(1); err == nil {
			t.Error("GetPATByID() error = nil, want error")
		}
	})

	t.Run("create failure surfaces", func(t *testing.T) {
		failCreatesOnTable(t, db, "personal_access_tokens")
		if _, err := repo.CreatePAT(user.ID, "n", "h", nil, ""); err == nil {
			t.Error("CreatePAT() error = nil, want injected failure")
		}
	})

	t.Run("delete failure surfaces", func(t *testing.T) {
		pat, err := repo.CreatePAT(user.ID, "n", "hash-del-inject", nil, "")
		if err != nil {
			t.Fatalf("CreatePAT() error = %v", err)
		}
		failDeletesOnTable(t, db, "personal_access_tokens")
		if err := repo.DeletePAT(pat.ID, user.ID); err == nil {
			t.Error("DeletePAT() error = nil, want injected failure")
		}
	})

	t.Run("update failure surfaces", func(t *testing.T) {
		pat, err := repo.CreatePAT(user.ID, "n", "hash-upd-inject", nil, "")
		if err != nil {
			t.Fatalf("CreatePAT() error = %v", err)
		}
		failUpdatesOnTable(t, db, "personal_access_tokens")
		if err := repo.UpdateLastUsed(pat.ID); err == nil {
			t.Error("UpdateLastUsed() error = nil, want injected failure")
		}
	})
}

func TestRecipeRepository_ErrorInjection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRecipeRepository(db)

	t.Run("non-not-found lookup failure makes cache unpopulated with error", func(t *testing.T) {
		failQueriesOnTable(t, db, "recipe_caches")
		populated, err := repo.HasPopulatedCache("hash")
		if err == nil {
			t.Error("HasPopulatedCache() error = nil, want injected failure")
		}
		if populated {
			t.Error("HasPopulatedCache() = true, want false on error")
		}
		if _, err := repo.GetCacheByQueryHash("hash"); err == nil {
			t.Error("GetCacheByQueryHash() error = nil, want injected failure")
		}
	})

	t.Run("create failure surfaces", func(t *testing.T) {
		failCreatesOnTable(t, db, "recipe_caches")
		entry := dbModel.RecipeCache{QueryHash: "hash", Provider: "themealdb", ResponseJSON: []byte(`[]`), ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.CreateCache(&entry); err == nil {
			t.Error("CreateCache() error = nil, want injected failure")
		}
	})

	t.Run("hit update failure surfaces", func(t *testing.T) {
		entry := dbModel.RecipeCache{QueryHash: "hash-hit", Provider: "themealdb", ResponseJSON: []byte(`[]`), ExpiresAt: time.Now().Add(time.Hour)}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		failUpdatesOnTable(t, db, "recipe_caches")
		if err := repo.UpdateCacheHit("hash-hit"); err == nil {
			t.Error("UpdateCacheHit() error = nil, want injected failure")
		}
	})

	t.Run("cleanup failure surfaces", func(t *testing.T) {
		failDeletesOnTable(t, db, "recipe_caches")
		if err := repo.CleanupExpiredCaches(); err == nil {
			t.Error("CleanupExpiredCaches() error = nil, want injected failure")
		}
	})
}

func TestNotificationRepository_ErrorInjection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()
	repo := NewNotificationRepositoryWithLogger(db, &logger)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("expired-products query failure surfaces", func(t *testing.T) {
		failQueriesOnTable(t, db, "products")
		if _, err := repo.GetProductsExpiredAndNotificationPending(time.Hour, 7); err == nil {
			t.Error("GetProductsExpiredAndNotificationPending() error = nil, want injected failure")
		}
	})

	t.Run("household member lookup failure surfaces", func(t *testing.T) {
		failQueriesOnTable(t, db, "users")
		if _, err := repo.GetHouseholdMembersMailAddressesByID(household.ID); err == nil {
			t.Error("GetHouseholdMembersMailAddressesByID() error = nil, want injected failure")
		}
		if _, err := repo.GetHouseholdMembersNotificationPreferences(household.ID); err == nil {
			t.Error("GetHouseholdMembersNotificationPreferences() error = nil, want injected failure")
		}
		if _, err := repo.GetHouseholdsWithMonthlyWasteReportEnabled(); err == nil {
			t.Error("GetHouseholdsWithMonthlyWasteReportEnabled() error = nil, want injected failure")
		}
		if _, err := repo.GetHouseholdsWithMailDigestEnabled(); err == nil {
			t.Error("GetHouseholdsWithMailDigestEnabled() error = nil, want injected failure")
		}
	})

	t.Run("waste stats count failure surfaces", func(t *testing.T) {
		failQueriesOnTable(t, db, "products")
		if _, err := repo.GetWasteStatsForHousehold(household.ID, time.Now()); err == nil {
			t.Error("GetWasteStatsForHousehold() error = nil, want injected failure")
		}
	})

	t.Run("each waste-stats count failure surfaces", func(t *testing.T) {
		// GetWasteStatsForHousehold issues six product counts in a fixed
		// order; failing each one in turn covers every error branch.
		for n := 1; n <= 6; n++ {
			db := testutil.SetupTestDB(t)
			failNthQueryOnTable(t, db, "products", n)
			household := testutil.CreateTestHousehold(db, 0)
			statsRepo := NewNotificationRepository(db)
			if _, err := statsRepo.GetWasteStatsForHousehold(household.ID, time.Now()); err == nil {
				t.Errorf("GetWasteStatsForHousehold() with failing query #%d: error = nil", n)
			}
		}
	})

	t.Run("notified-at save failure surfaces", func(t *testing.T) {
		failUpdatesOnTable(t, db, "products")
		if err := repo.SetProductNotifiedAt(product.ID); err == nil {
			t.Error("SetProductNotifiedAt() error = nil, want injected failure")
		}
	})

	t.Run("vapid key lookup failure other than not-found surfaces", func(t *testing.T) {
		failQueriesOnTable(t, db, "web_push_configs")
		if _, _, err := repo.GetVAPIDKeys(); err == nil {
			t.Error("GetVAPIDKeys() error = nil, want injected failure")
		}
	})

	t.Run("vapid key insert failure surfaces", func(t *testing.T) {
		failCreatesOnTable(t, db, "web_push_configs")
		if _, _, err := repo.GetVAPIDKeys(); err == nil {
			t.Error("GetVAPIDKeys() error = nil, want injected create failure")
		}
	})

	t.Run("unsubscribe token insert failure surfaces", func(t *testing.T) {
		failCreatesOnTable(t, db, "mail_digest_unsubscribe_tokens")
		if _, err := repo.GenerateMailDigestUnsubscribeToken(user.ID); err == nil {
			t.Error("GenerateMailDigestUnsubscribeToken() error = nil, want injected failure")
		}
	})

	t.Run("digest target skips users whose token lookup fails", func(t *testing.T) {
		user.NotificationPreferences.EmailEnabled = true
		user.NotificationPreferences.MailDigestFrequency = authentication.MailDigestFrequencyDaily
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save user: %v", err)
		}
		failQueriesOnTable(t, db, "mail_digest_unsubscribe_tokens")
		targets, err := repo.GetHouseholdsWithMailDigestEnabled()
		if err != nil {
			t.Fatalf("GetHouseholdsWithMailDigestEnabled() error = %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("targets = %d, want 1 household", len(targets))
		}
		if len(targets[0].Users) != 0 {
			t.Errorf("targets[0].Users = %+v, want empty (token lookup failed)", targets[0].Users)
		}
	})
}
