package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/testutil"
)

func TestProductRepository_GetConsumedSamplesEmptyInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	digestRepo := NewProductDigestRepository(db)

	if _, err := digestRepo.GetConsumedSamples(1, 1, "", "", time.Now()); err == nil {
		t.Error("GetConsumedSamples() with empty barcode and name: error = nil, want error")
	}
}

func TestProductRepository_GetExpiringProductsForMailDigest(t *testing.T) {
	db := testutil.SetupTestDB(t)
	digestRepo := NewProductDigestRepository(db)
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

	group, err := digestRepo.GetExpiringProductsForMailDigest(household.ID)
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
		group, err := digestRepo.GetExpiringProductsForMailDigest(9999)
		if err != nil {
			t.Fatalf("GetExpiringProductsForMailDigest() error = %v", err)
		}
		if len(group.Today)+len(group.ThisWeek)+len(group.NextWeek) != 0 {
			t.Errorf("group = %+v, want all empty", group)
		}
	})
}
