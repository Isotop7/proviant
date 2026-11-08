package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
	"gorm.io/gorm"
)

func TestProductRepository_GetActiveProductsCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	statsRepo := NewProductStatsRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	for range 3 {
		product := dbModel.Product{
			ProductName: "Product",
			Barcode:     "1234567890123",
			HouseholdID: household.ID,
		}
		db.Create(&product)
	}

	count, err := statsRepo.GetActiveProductsCount(user.ID)
	if err != nil {
		t.Fatalf("GetActiveProductsCount() error = %v", err)
	}

	if count != 3 {
		t.Errorf("GetActiveProductsCount() = %d, want 3", count)
	}
}
func TestProductRepository_GetProductCategoryBreakdown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	statsRepo := NewProductStatsRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	products := []dbModel.Product{
		{ProductName: "P1", HouseholdID: household.ID, Categories: "dairy"},
		{ProductName: "P2", HouseholdID: household.ID, Categories: "dairy"},
		{ProductName: "P3", HouseholdID: household.ID, Categories: "meat"},
	}
	for i := range products {
		db.Create(&products[i])
	}

	breakdown, err := statsRepo.GetProductCategoryBreakdown(user.ID)
	if err != nil {
		t.Fatalf("GetProductCategoryBreakdown() error = %v", err)
	}

	if breakdown["dairy"] != 2 {
		t.Errorf("GetProductCategoryBreakdown() dairy = %d, want 2", breakdown["dairy"])
	}
	if breakdown["meat"] != 1 {
		t.Errorf("GetProductCategoryBreakdown() meat = %d, want 1", breakdown["meat"])
	}
}

func TestProductRepository_GetProductCategoryBreakdownEdges(t *testing.T) {
	seed := func(t *testing.T, categories ...string) (*gorm.DB, *ProductStatsRepository, *authentication.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		statsRepo := NewProductStatsRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		for _, c := range categories {
			product := testutil.CreateTestProduct(db, household.ID, user.ID)
			product.Categories = c
			if err := db.Save(product).Error; err != nil {
				t.Fatalf("save product: %v", err)
			}
		}
		return db, statsRepo, user
	}

	t.Run("empty and key-value categories", func(t *testing.T) {
		_, statsRepo, user := seed(t, "", "en:dairy", "meat,fish", "  ")

		breakdown, err := statsRepo.GetProductCategoryBreakdown(user.ID)
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
		for i := range 10 {
			categories = append(categories, fmt.Sprintf("cat-%d", i))
		}
		_, statsRepo, user := seed(t, categories...)

		breakdown, err := statsRepo.GetProductCategoryBreakdown(user.ID)
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

func TestGetActiveExpiryCounts(t *testing.T) {
	now := time.Now()
	criticalDays := 3

	t.Run("classifies expired and critical active products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		statsRepo := NewProductStatsRepository(db)
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

		expired, critical, err := statsRepo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 1 {
			t.Errorf("got expired=%d critical=%d, want 1 and 1", expired, critical)
		}
	})

	t.Run("an opened shelf life outranks a far printed expiry", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		statsRepo := NewProductStatsRepository(db)
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

		expired, critical, err := statsRepo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 1 and 0", expired, critical)
		}
	})

	t.Run("ignores archived and private products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		statsRepo := NewProductStatsRepository(db)
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

		expired, critical, err := statsRepo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 0 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 0 and 0", expired, critical)
		}
	})
}

func TestProductRepository_GetWasteThisMonth(t *testing.T) {
	db := testutil.SetupTestDB(t)
	statsRepo := NewProductStatsRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	// The 31st is the day the month arithmetic breaks on, so the anchor is
	// pinned to one: the window the count covers is the anchor's month on
	// every day of the year.
	now := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)
	monthStart := util.MonthStart(now)

	// Wasted and deleted this month: counts.
	wasted := testutil.CreateTestProduct(db, household.ID, user.ID)
	wasted.RemovalReason = dbModel.RemovalReasonWasted
	if err := db.Save(wasted).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	if err := db.Delete(wasted).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", wasted.ID).
		Update("deleted_at", now).Error; err != nil {
		t.Fatalf("pin the deletion to the anchor: %v", err)
	}

	// Consumed this month: not waste.
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "1111111111111", "Consumed", 1, now)

	// Wasted but deleted last month: not this month.
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "2222222222222", "OldWaste", 1, monthStart.Add(-time.Hour))
	var oldWaste dbModel.Product
	if err := db.Unscoped().Where("barcode = ?", "2222222222222").First(&oldWaste).Error; err != nil {
		t.Fatalf("reload old waste: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", oldWaste.ID).
		Update("removal_reason", dbModel.RemovalReasonWasted).Error; err != nil {
		t.Fatalf("mark old product wasted: %v", err)
	}

	count, err := statsRepo.GetWasteThisMonth(user.ID, now)
	if err != nil {
		t.Fatalf("GetWasteThisMonth() error = %v", err)
	}
	if count != 1 {
		t.Errorf("GetWasteThisMonth() = %d, want 1", count)
	}

	// The anchor picks the month, not the wall clock: the same rows are
	// outside the window of a later month.
	otherMonth, err := statsRepo.GetWasteThisMonth(user.ID, monthStart.AddDate(0, 2, 0))
	if err != nil {
		t.Fatalf("GetWasteThisMonth() error = %v", err)
	}
	if otherMonth != 0 {
		t.Errorf("GetWasteThisMonth() for a later month = %d, want 0", otherMonth)
	}

	if _, err := statsRepo.GetWasteThisMonth(9999, now); err == nil {
		t.Error("GetWasteThisMonth(9999) error = nil, want error")
	}
}

// TestProductRepository_GetWasteThisMonthCountsEveryWaste verifies that a partial waste counts once and a full waste is not counted twice.
func TestProductRepository_GetWasteThisMonthCountsEveryWaste(t *testing.T) {
	db := testutil.SetupTestDB(t)
	statsRepo := NewProductStatsRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	other := authentication.User{
		Username:    "othermember",
		Password:    "testpassword123",
		MailAddress: "other@example.com",
		HouseholdID: household.ID,
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}

	writeLedger := func(product *dbModel.Product, eventType string) {
		record := dbModel.SavingsRecord{
			HouseholdID: household.ID,
			ProductID:   product.ID,
			ProductName: product.ProductName,
			EventType:   eventType,
			PriceEUR:    2.5,
			CO2Kg:       1,
			Amount:      1,
		}
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("create savings record: %v", err)
		}
	}

	// A full waste: the product is archived and the ledger holds the event.
	full := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(full).Update("removal_reason", dbModel.RemovalReasonWasted).Error; err != nil {
		t.Fatalf("claim product: %v", err)
	}
	if err := db.Delete(full).Error; err != nil {
		t.Fatalf("archive product: %v", err)
	}
	writeLedger(full, dbModel.SavingsEventTypeWasted)

	// A partial waste: the product stays active, only the ledger knows.
	partial := testutil.CreateTestProduct(db, household.ID, user.ID)
	writeLedger(partial, dbModel.SavingsEventTypeWasted)

	// A consumed product is not waste.
	consumed := testutil.CreateTestProduct(db, household.ID, user.ID)
	writeLedger(consumed, dbModel.SavingsEventTypeConsumed)

	// The private product of another member stays out of this member's count.
	private := testutil.CreateTestProduct(db, household.ID, other.ID)
	private.IsPrivate = true
	if err := db.Save(private).Error; err != nil {
		t.Fatalf("save private product: %v", err)
	}
	writeLedger(private, dbModel.SavingsEventTypeWasted)

	count, err := statsRepo.GetWasteThisMonth(user.ID, time.Now())
	if err != nil {
		t.Fatalf("GetWasteThisMonth() error = %v", err)
	}
	if count != 2 {
		t.Errorf("GetWasteThisMonth() = %d, want 2", count)
	}

	otherCount, err := statsRepo.GetWasteThisMonth(other.ID, time.Now())
	if err != nil {
		t.Fatalf("GetWasteThisMonth(other) error = %v", err)
	}
	if otherCount != 3 {
		t.Errorf("GetWasteThisMonth(other) = %d, want 3", otherCount)
	}
}
