package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func TestProductRepository_GetUserProductsBulkByBarcode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	match1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	match1.Barcode = "1234567890123"
	if err := db.Save(match1).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	match2 := testutil.CreateTestProduct(db, household.ID, user.ID)
	match2.Barcode = "1234567890123"
	if err := db.Save(match2).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	other := testutil.CreateTestProduct(db, household.ID, user.ID)
	other.Barcode = "5555555555555"
	if err := db.Save(other).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
	foreign.Barcode = "1234567890123"
	if err := db.Save(foreign).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("returns only own household matches", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkByBarcode(user.ID, 1234567890123)
		if err != nil {
			t.Fatalf("GetUserProductsBulkByBarcode() error = %v", err)
		}
		if len(products) != 2 {
			t.Fatalf("GetUserProductsBulkByBarcode() returned %d products, want 2", len(products))
		}
		for _, p := range products {
			if p.Barcode != "1234567890123" {
				t.Errorf("Barcode = %q, want 1234567890123", p.Barcode)
			}
		}
	})

	t.Run("unknown barcode returns nothing", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkByBarcode(user.ID, 42)
		if err != nil {
			t.Fatalf("GetUserProductsBulkByBarcode() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserProductsBulkByBarcode() returned %d products, want 0", len(products))
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		if _, err := repo.GetUserProductsBulkByBarcode(9999, 1234567890123); err == nil {
			t.Error("GetUserProductsBulkByBarcode(9999) error = nil, want error")
		}
	})
}

func TestProductRepository_GetProductIdentity(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ProductName = "Identity Product"
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateProduct := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateProduct.IsPrivate = true
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)

	t.Run("success returns identity fields", func(t *testing.T) {
		identity, err := repo.GetProductIdentity(product.ID, user.ID)
		if err != nil {
			t.Fatalf("GetProductIdentity() error = %v", err)
		}
		if identity.ProductName != "Identity Product" || identity.Barcode != product.Barcode {
			t.Errorf("GetProductIdentity() = %+v, want name and barcode", identity)
		}
	})

	t.Run("zero id is not implemented", func(t *testing.T) {
		if _, err := repo.GetProductIdentity(0, user.ID); err != gorm.ErrNotImplemented {
			t.Errorf("GetProductIdentity(0) error = %v, want %v", err, gorm.ErrNotImplemented)
		}
	})

	t.Run("cross-household access is rejected", func(t *testing.T) {
		if _, err := repo.GetProductIdentity(foreign.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductIdentity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("private product of another member is rejected", func(t *testing.T) {
		if _, err := repo.GetProductIdentity(privateProduct.ID, user.ID); err != errors.ErrMismatcherUserID {
			t.Errorf("GetProductIdentity() error = %v, want %v", err, errors.ErrMismatcherUserID)
		}
	})

	t.Run("owner can read own private product", func(t *testing.T) {
		if _, err := repo.GetProductIdentity(privateProduct.ID, owner.ID); err != nil {
			t.Errorf("GetProductIdentity() owner error = %v, want nil", err)
		}
	})

	t.Run("unknown product errors", func(t *testing.T) {
		if _, err := repo.GetProductIdentity(9999, user.ID); err == nil {
			t.Error("GetProductIdentity(9999) error = nil, want error")
		}
	})
}

func TestProductRepository_GetProductsExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	past := testutil.CreateTestProduct(db, household.ID, user.ID)
	past.ExpireAt = time.Now().Add(-48 * time.Hour)
	if err := db.Save(past).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	future := testutil.CreateTestProduct(db, household.ID, user.ID)
	future.ExpireAt = time.Now().Add(48 * time.Hour)
	if err := db.Save(future).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	// A zero expire_at means no expiry set — never expired (matches
	// GetExpiredProductsCount).
	undated := testutil.CreateTestProduct(db, household.ID, user.ID)
	undated.ExpireAt = time.Time{}
	if err := db.Save(undated).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	// The filter keeps products whose effective expiry lies in the past;
	// future-dated and undated ones stay out.
	expired, err := repo.GetProductsExpired(user.ID)
	if err != nil {
		t.Fatalf("GetProductsExpired() error = %v", err)
	}
	got := map[uint]bool{}
	for _, p := range expired {
		got[p.ID] = true
	}
	if len(expired) != 1 || !got[past.ID] {
		t.Errorf("GetProductsExpired() = %v, want past %d only", expired, past.ID)
	}
	if got[undated.ID] {
		t.Errorf("GetProductsExpired contains undated product %d", undated.ID)
	}
	if got[future.ID] {
		t.Errorf("GetProductsExpired() contains future product %d", future.ID)
	}

	if _, err := repo.GetProductsExpired(9999); err == nil {
		t.Error("GetProductsExpired(9999) error = nil, want error")
	}
}

func TestProductRepository_GetWasteThisMonth(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	// Wall-clock month the seeds below land in; the repository computes its
	// window from its own time.Now() at call time.
	seedMonth := now.Format("2006-01")

	// Wasted and deleted this month: counts.
	wasted := testutil.CreateTestProduct(db, household.ID, user.ID)
	wasted.RemovalReason = dbModel.RemovalReasonWasted
	if err := db.Save(wasted).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	if err := db.Delete(wasted).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}

	// Consumed this month: not waste.
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "1111111111111", "Consumed", "pcs", 1, now)

	// Wasted but deleted last month: not this month.
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "2222222222222", "OldWaste", "pcs", 1, monthStart.Add(-time.Hour))
	var oldWaste dbModel.Product
	if err := db.Unscoped().Where("barcode = ?", "2222222222222").First(&oldWaste).Error; err != nil {
		t.Fatalf("reload old waste: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", oldWaste.ID).
		Update("removal_reason", dbModel.RemovalReasonWasted).Error; err != nil {
		t.Fatalf("mark old product wasted: %v", err)
	}

	count, err := repo.GetWasteThisMonth(user.ID)
	if err != nil {
		t.Fatalf("GetWasteThisMonth() error = %v", err)
	}
	// Guard: the repository derives its window from wall-clock time at call
	// time. If the month flipped between seeding and the call, the seeded
	// deletion falls outside the window and count is 0 instead of 1.
	if time.Now().Format("2006-01") != seedMonth {
		t.Skip("month boundary crossed")
	}
	if count != 1 {
		t.Errorf("GetWasteThisMonth() = %d, want 1", count)
	}

	if _, err := repo.GetWasteThisMonth(9999); err == nil {
		t.Error("GetWasteThisMonth(9999) error = nil, want error")
	}
}

func TestProductRepository_GetExpiringProductsByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	soon := testutil.CreateTestProduct(db, household.ID, user.ID)
	soon.ProductName = "Soon"
	soon.ExpireAt = time.Now().Add(24 * time.Hour)
	if err := db.Save(soon).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	later := testutil.CreateTestProduct(db, household.ID, user.ID)
	later.ProductName = "Later"
	later.ExpireAt = time.Now().Add(10 * 24 * time.Hour)
	if err := db.Save(later).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	privateProduct.IsPrivate = true
	privateProduct.ExpireAt = time.Now().Add(24 * time.Hour)
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	deleted := testutil.CreateTestProduct(db, household.ID, user.ID)
	deleted.ExpireAt = time.Now().Add(24 * time.Hour)
	if err := db.Save(deleted).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	if err := db.Delete(deleted).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	// Opened-shelf-life row whose effective expiry already passed: a SQL
	// candidate (opened_at inside the prune floor) that the Go fine-filter
	// must drop.
	openedAgo := time.Now().Add(-10 * 24 * time.Hour)
	openedDays := 5
	openedExpired := testutil.CreateTestProduct(db, household.ID, user.ID)
	openedExpired.OpenedAt = &openedAgo
	openedExpired.DaysAfterOpening = &openedDays
	if err := db.Save(openedExpired).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	products, err := repo.GetExpiringProductsByHousehold(household.ID, 7)
	if err != nil {
		t.Fatalf("GetExpiringProductsByHousehold() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != soon.ID {
		t.Errorf("GetExpiringProductsByHousehold() = %+v, want only product %d", products, soon.ID)
	}

	t.Run("wider window includes later product, sorted ascending", func(t *testing.T) {
		products, err := repo.GetExpiringProductsByHousehold(household.ID, 30)
		if err != nil {
			t.Fatalf("GetExpiringProductsByHousehold() error = %v", err)
		}
		if len(products) != 2 {
			t.Fatalf("GetExpiringProductsByHousehold() returned %d, want 2", len(products))
		}
		if products[0].ID != soon.ID || products[1].ID != later.ID {
			t.Errorf("order = [%d %d], want [%d %d] ascending", products[0].ID, products[1].ID, soon.ID, later.ID)
		}
	})

	t.Run("empty household returns nothing", func(t *testing.T) {
		products, err := repo.GetExpiringProductsByHousehold(9999, 7)
		if err != nil {
			t.Fatalf("GetExpiringProductsByHousehold() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetExpiringProductsByHousehold() returned %d, want 0", len(products))
		}
	})
}

func TestProductRepository_GetProductsByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	testutil.CreateTestProduct(db, household.ID, user.ID)
	privateProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	privateProduct.IsPrivate = true
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	deleted := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(deleted).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}

	products, err := repo.GetProductsByHousehold(household.ID)
	if err != nil {
		t.Fatalf("GetProductsByHousehold() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("GetProductsByHousehold() returned %d, want 1 (public, active only)", len(products))
	}

	products, err = repo.GetProductsByHousehold(9999)
	if err != nil {
		t.Fatalf("GetProductsByHousehold() error = %v", err)
	}
	if len(products) != 0 {
		t.Errorf("GetProductsByHousehold() returned %d, want 0", len(products))
	}
}

func TestProductRepository_GetSubThresholdProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	below := testutil.CreateTestProduct(db, household.ID, user.ID)
	below.Amount = 1
	below.MinStockAmount = 5
	if err := db.Save(below).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	atThreshold := testutil.CreateTestProduct(db, household.ID, user.ID)
	atThreshold.Amount = 5
	atThreshold.MinStockAmount = 5
	if err := db.Save(atThreshold).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	noThreshold := testutil.CreateTestProduct(db, household.ID, user.ID)
	noThreshold.Amount = 0
	noThreshold.MinStockAmount = 0
	if err := db.Save(noThreshold).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateBelow := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateBelow.IsPrivate = true
	privateBelow.Amount = 1
	privateBelow.MinStockAmount = 5
	if err := db.Save(privateBelow).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("returns only below-threshold visible products", func(t *testing.T) {
		products, err := repo.GetSubThresholdProducts(user.ID)
		if err != nil {
			t.Fatalf("GetSubThresholdProducts() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != below.ID {
			t.Errorf("GetSubThresholdProducts() = %+v, want only product %d", products, below.ID)
		}
	})

	t.Run("owner sees own private below-threshold product", func(t *testing.T) {
		products, err := repo.GetSubThresholdProducts(owner.ID)
		if err != nil {
			t.Fatalf("GetSubThresholdProducts() error = %v", err)
		}
		// The owner sees the household's public below-threshold product plus
		// their own private one.
		if len(products) != 2 {
			t.Fatalf("GetSubThresholdProducts() returned %d, want 2", len(products))
		}
		found := false
		for _, p := range products {
			if p.ID == privateBelow.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("GetSubThresholdProducts() = %+v, want private product %d included", products, privateBelow.ID)
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		if _, err := repo.GetSubThresholdProducts(9999); err == nil {
			t.Error("GetSubThresholdProducts(9999) error = nil, want error")
		}
	})
}

func TestProductRepository_GetExpiringProductsForMailDigest(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	// Wall-clock day the seeds below land in; the repository computes
	// startOfToday from its own time.Now() at call time.
	seedDay := time.Now().Format("2006-01-02")

	today := testutil.CreateTestProduct(db, household.ID, user.ID)
	today.ProductName = "Today"
	today.ExpireAt = time.Now()
	if err := db.Save(today).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	thisWeek := testutil.CreateTestProduct(db, household.ID, user.ID)
	thisWeek.ProductName = "ThisWeek"
	thisWeek.ExpireAt = time.Now().Add(3 * 24 * time.Hour)
	if err := db.Save(thisWeek).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	nextWeek := testutil.CreateTestProduct(db, household.ID, user.ID)
	nextWeek.ProductName = "NextWeek"
	nextWeek.ExpireAt = time.Now().Add(10 * 24 * time.Hour)
	if err := db.Save(nextWeek).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	alreadyExpired := testutil.CreateTestProduct(db, household.ID, user.ID)
	alreadyExpired.ExpireAt = time.Now().Add(-24 * time.Hour)
	if err := db.Save(alreadyExpired).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	tooFar := testutil.CreateTestProduct(db, household.ID, user.ID)
	tooFar.ExpireAt = time.Now().Add(20 * 24 * time.Hour)
	if err := db.Save(tooFar).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	privateProduct.IsPrivate = true
	privateProduct.ExpireAt = time.Now()
	if err := db.Save(privateProduct).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	// Opened-shelf-life row whose effective expiry is already in the past:
	// a SQL candidate the Go fine-filter must drop.
	openedAgo := time.Now().Add(-10 * 24 * time.Hour)
	openedDays := 5
	openedExpired := testutil.CreateTestProduct(db, household.ID, user.ID)
	openedExpired.OpenedAt = &openedAgo
	openedExpired.DaysAfterOpening = &openedDays
	if err := db.Save(openedExpired).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	group, err := repo.GetExpiringProductsForMailDigest(household.ID)
	if err != nil {
		t.Fatalf("GetExpiringProductsForMailDigest() error = %v", err)
	}

	// Guard: the Today bucket derives from startOfToday at call time. If the
	// calendar day flipped between seeding and the call, the "Today" product
	// falls below the window and lands in no bucket.
	if time.Now().Format("2006-01-02") != seedDay {
		t.Skip("day boundary crossed")
	}

	if len(group.Today) != 1 || group.Today[0].ID != today.ID {
		t.Errorf("Today = %+v, want only product %d", group.Today, today.ID)
	}
	if len(group.ThisWeek) != 1 || group.ThisWeek[0].ID != thisWeek.ID {
		t.Errorf("ThisWeek = %+v, want only product %d", group.ThisWeek, thisWeek.ID)
	}
	if len(group.NextWeek) != 1 || group.NextWeek[0].ID != nextWeek.ID {
		t.Errorf("NextWeek = %+v, want only product %d", group.NextWeek, nextWeek.ID)
	}

	t.Run("empty household returns empty groups", func(t *testing.T) {
		group, err := repo.GetExpiringProductsForMailDigest(9999)
		if err != nil {
			t.Fatalf("GetExpiringProductsForMailDigest() error = %v", err)
		}
		if len(group.Today)+len(group.ThisWeek)+len(group.NextWeek) != 0 {
			t.Errorf("group = %+v, want all empty", group)
		}
	})
}

func TestProductRepository_BulkConsumeProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	p1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	p2 := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("consumes valid ids and reports failures", func(t *testing.T) {
		errs := repo.BulkConsumeProducts([]uint{p1.ID, p2.ID, 9999}, user.ID)
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
		if errs := repo.BulkConsumeProducts(nil, user.ID); len(errs) != 0 {
			t.Errorf("BulkConsumeProducts(nil) returned %d errors, want 0", len(errs))
		}
	})
}

func TestProductRepository_BulkWasteProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	p1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	p2 := testutil.CreateTestProduct(db, household.ID, user.ID)

	t.Run("wastes valid ids and reports failures", func(t *testing.T) {
		errs := repo.BulkWasteProducts([]uint{p1.ID, 9999, p2.ID}, user.ID)
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
		if errs := repo.BulkWasteProducts(nil, user.ID); len(errs) != 0 {
			t.Errorf("BulkWasteProducts(nil) returned %d errors, want 0", len(errs))
		}
	})
}

func TestFilterByEffectiveExpiryWindow(t *testing.T) {
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	inWindow := dbModel.Product{ExpireAt: time.Now()}
	beforeWindow := dbModel.Product{ExpireAt: start.Add(-time.Hour)}
	afterWindow := dbModel.Product{ExpireAt: end.Add(time.Hour)}
	undated := dbModel.Product{}
	// Opened-shelf-life row whose effective expiry fell before the window.
	openedAgo := time.Now().Add(-10 * 24 * time.Hour)
	openedDays := 5
	openedExpired := dbModel.Product{OpenedAt: &openedAgo, DaysAfterOpening: &openedDays}

	filtered := filterByEffectiveExpiryWindow(
		[]dbModel.Product{inWindow, beforeWindow, afterWindow, undated, openedExpired},
		start, end,
	)
	if len(filtered) != 1 || !filtered[0].ExpireAt.Equal(inWindow.ExpireAt) {
		t.Errorf("filterByEffectiveExpiryWindow() = %+v, want only the in-window product", filtered)
	}

	if got := filterByEffectiveExpiryWindow(nil, start, end); len(got) != 0 {
		t.Errorf("filterByEffectiveExpiryWindow(nil) returned %d, want 0", len(got))
	}
}

func TestSortByEffectiveExpiryZeroHandling(t *testing.T) {
	dated := dbModel.Product{ExpireAt: time.Now().Add(24 * time.Hour)}
	undated := dbModel.Product{}

	products := []dbModel.Product{undated, dated}
	SortByEffectiveExpiry(products)
	if !products[0].ExpireAt.Equal(dated.ExpireAt) || !products[1].ExpireAt.IsZero() {
		t.Error("SortByEffectiveExpiry() = dated product first, undated last")
	}

	bothUndated := []dbModel.Product{{}, {}}
	SortByEffectiveExpiry(bothUndated) // must not panic on all-zero expiries
}

func TestCalendarTokenRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewCalendarTokenRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	ct := authentication.CalendarToken{
		UserID:    user.ID,
		Token:     "cal-token-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	t.Run("create and get by token", func(t *testing.T) {
		if err := repo.Create(&ct); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if ct.ID == 0 {
			t.Error("Create() did not set ID")
		}
		found, err := repo.GetByToken("cal-token-1")
		if err != nil {
			t.Fatalf("GetByToken() error = %v", err)
		}
		if found.UserID != user.ID {
			t.Errorf("GetByToken() UserID = %d, want %d", found.UserID, user.ID)
		}
	})

	t.Run("get by token not found", func(t *testing.T) {
		if _, err := repo.GetByToken("missing"); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByToken() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		expired := authentication.CalendarToken{
			UserID:    user.ID,
			Token:     "cal-token-expired",
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		if err := repo.Create(&expired); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := repo.GetByToken("cal-token-expired"); err != errors.ErrTokenExpired {
			t.Errorf("GetByToken() error = %v, want %v", err, errors.ErrTokenExpired)
		}
	})

	t.Run("get by user id", func(t *testing.T) {
		// A dedicated user keeps this independent of the sibling subtests.
		tokenUser := testutil.CreateTestUser(db, household.ID)
		seed := authentication.CalendarToken{
			UserID:    tokenUser.ID,
			Token:     "cal-token-user",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := repo.Create(&seed); err != nil {
			t.Fatalf("seed token: %v", err)
		}
		found, err := repo.GetByUserID(tokenUser.ID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		// GetByUserID orders by primary key, so the earliest-created token wins.
		if found.Token != "cal-token-user" {
			t.Errorf("GetByUserID() Token = %q, want %q", found.Token, "cal-token-user")
		}
		if _, err := repo.GetByUserID(9999); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByUserID(9999) error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("update persists changes", func(t *testing.T) {
		// Seed a token in this subtest so it does not rely on an earlier one.
		updateUser := testutil.CreateTestUser(db, household.ID)
		token := authentication.CalendarToken{
			UserID:    updateUser.ID,
			Token:     "cal-token-update",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := repo.Create(&token); err != nil {
			t.Fatalf("seed token: %v", err)
		}
		token.ExpiresAt = time.Now().Add(48 * time.Hour)
		if err := repo.Update(&token); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		found, err := repo.GetByToken("cal-token-update")
		if err != nil {
			t.Fatalf("GetByToken() error = %v", err)
		}
		if time.Until(found.ExpiresAt) <= 24*time.Hour {
			t.Errorf("ExpiresAt = %v, want updated to ~48h out", found.ExpiresAt)
		}
	})

	t.Run("delete by user id removes all tokens", func(t *testing.T) {
		if err := repo.DeleteByUserID(user.ID); err != nil {
			t.Fatalf("DeleteByUserID() error = %v", err)
		}
		if _, err := repo.GetByUserID(user.ID); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByUserID() after delete error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})
}
