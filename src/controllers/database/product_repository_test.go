package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestProductRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetProductByID not found", func(t *testing.T) {
		repo := NewProductRepository(db)

		_, err := repo.GetProductByID(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetUserProductsBulk returns empty for no products", func(t *testing.T) {
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		products, err := repo.GetUserProductsBulk(testUser.ID, 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("len(products) = %v, want 0", len(products))
		}
	})

	t.Run("GetMatchableProductsByHousehold populates only the matched columns", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(1)
		product := testutil.CreateTestProduct(db, householdID)
		product.ProductName = "Chicken"
		product.Categories = "meat"
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		row := rows[0]
		if row.ID != product.ID {
			t.Errorf("row.ID = %d, want %d", row.ID, product.ID)
		}
		if row.ProductName != "Chicken" {
			t.Errorf("row.ProductName = %q, want %q", row.ProductName, "Chicken")
		}
		if row.Categories != "meat" {
			t.Errorf("row.Categories = %q, want %q", row.Categories, "meat")
		}
		// The projection deliberately leaves everything else zero so a caller
		// reading an unselected column gets an obviously wrong value instead of
		// a plausible one.
		if row.Amount != 0 || row.Barcode != "" || !row.ExpireAt.IsZero() {
			t.Errorf("unselected columns are not zero: amount=%d barcode=%q expireAt=%v",
				row.Amount, row.Barcode, row.ExpireAt)
		}
	})

	t.Run("GetMatchableProductsByHousehold excludes private and deleted products", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(2)
		visible := testutil.CreateTestProduct(db, householdID)
		visible.ProductName = "Visible"
		if err := db.Save(visible).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		private := testutil.CreateTestProduct(db, householdID)
		private.ProductName = "Private"
		private.IsPrivate = true
		if err := db.Save(private).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		removed := testutil.CreateTestProduct(db, householdID)
		removed.ProductName = "Removed"
		if err := db.Delete(removed).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].ProductName != "Visible" {
			t.Errorf("got %d rows (%+v), want only the visible product", len(rows), rows)
		}
	})
}

func TestGetActiveExpiryCounts(t *testing.T) {
	now := time.Now()
	criticalDays := 3

	t.Run("classifies expired and critical active products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		expiredProduct := testutil.CreateTestProduct(db, household.ID)
		expiredProduct.ProductName = "Expired"
		expiredProduct.ExpireAt = now.AddDate(0, 0, -1)
		if err := db.Save(expiredProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		criticalProduct := testutil.CreateTestProduct(db, household.ID)
		criticalProduct.ProductName = "Critical"
		criticalProduct.ExpireAt = now.AddDate(0, 0, 1)
		if err := db.Save(criticalProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		freshProduct := testutil.CreateTestProduct(db, household.ID)
		freshProduct.ProductName = "Fresh"
		freshProduct.ExpireAt = now.AddDate(0, 0, 30)
		if err := db.Save(freshProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 1 {
			t.Errorf("got expired=%d critical=%d, want 1 and 1", expired, critical)
		}
	})

	t.Run("an opened shelf life outranks a far printed expiry", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Printed expiry is months out, but opening it two days ago with a
		// one-day shelf life makes it expired. The SQL candidate filter has to
		// admit the row through the opened branch, not the printed one.
		openedAt := now.AddDate(0, 0, -2)
		daysAfterOpening := 1
		product := testutil.CreateTestProduct(db, household.ID)
		product.ExpireAt = now.AddDate(0, 6, 0)
		product.OpenedAt = &openedAt
		product.DaysAfterOpening = &daysAfterOpening
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 1 and 0", expired, critical)
		}
	})

	t.Run("ignores archived and private products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Soft-deleted: the archived view still renders it as a consumed row,
		// but it must not badge the sidebar.
		archived := testutil.CreateTestProduct(db, household.ID)
		archived.ProductName = "Archived"
		archived.ExpireAt = now.AddDate(0, 0, -5)
		if err := db.Save(archived).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.Delete(archived).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Private to another user in the same household.
		other := testutil.CreateTestUser(db, household.ID)
		private := testutil.CreateTestProduct(db, household.ID, other.ID)
		private.ProductName = "Private"
		private.IsPrivate = true
		private.ExpireAt = now.AddDate(0, 0, -5)
		if err := db.Save(private).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 0 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 0 and 0", expired, critical)
		}
	})
}
