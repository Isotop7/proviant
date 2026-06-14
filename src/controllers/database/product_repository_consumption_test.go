package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestProductRepository_GetConsumedSamples(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	admin := authentication.User{Username: "a", Password: "x", MailAddress: "a@x"}
	db.Create(&admin)
	hh := dbModel.Household{Name: "H", AdminID: admin.ID}
	db.Create(&hh)
	admin.HouseholdID = hh.ID
	db.Save(&admin)

	other := authentication.User{Username: "o", Password: "x", MailAddress: "o@x", HouseholdID: hh.ID}
	db.Create(&other)

	since := time.Now().AddDate(0, 0, -30)
	now := time.Now()

	t.Run("filters by household, reason, time window, and barcode", func(t *testing.T) {
		barcode := "1111111111111"
		// Use a local seed with reason control to test the wasted filter.
		seedConsumedWithReason(t, db, hh.ID, admin.ID, false, barcode, "Milk", dbModel.RemovalReasonConsumed, 1, "L", now.Add(-7*24*time.Hour))
		seedConsumedWithReason(t, db, hh.ID, admin.ID, false, barcode, "Milk", dbModel.RemovalReasonConsumed, 1, "L", now.Add(-3*24*time.Hour))
		seedConsumedWithReason(t, db, hh.ID, admin.ID, false, "2222222222222", "Bread", dbModel.RemovalReasonConsumed, 1, "pcs", now.Add(-5*24*time.Hour))
		seedConsumedWithReason(t, db, hh.ID, admin.ID, false, barcode, "Milk", dbModel.RemovalReasonWasted, 1, "L", now.Add(-2*24*time.Hour))

		// Other household — must NOT be returned.
		otherHousehold := testutil.CreateTestHousehold(db, 0)
		seedConsumedWithReason(t, db, otherHousehold.ID, 0, false, barcode, "Milk", dbModel.RemovalReasonConsumed, 1, "L", now.Add(-1*24*time.Hour))

		products, err := repo.GetConsumedSamples(hh.ID, admin.ID, barcode, "Milk", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 2 {
			t.Errorf("len(products) = %d, want 2", len(products))
		}
		for _, p := range products {
			if p.Barcode != barcode {
				t.Errorf("unexpected barcode %q", p.Barcode)
			}
			if p.RemovalReason != dbModel.RemovalReasonConsumed {
				t.Errorf("unexpected reason %q", p.RemovalReason)
			}
		}
	})

	t.Run("excludes products deleted before the window", func(t *testing.T) {
		barcode := "3333333333333"
		old := now.AddDate(0, 0, -120)
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, barcode, "Yogurt", "pcs", 1, old)

		products, err := repo.GetConsumedSamples(hh.ID, admin.ID, barcode, "Yogurt", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("len(products) = %d, want 0", len(products))
		}
	})

	t.Run("returns ordered by deleted_at ascending", func(t *testing.T) {
		barcode := "4444444444444"
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, barcode, "Butter", "pcs", 1, now.Add(-1*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, barcode, "Butter", "pcs", 1, now.Add(-10*24*time.Hour))

		products, err := repo.GetConsumedSamples(hh.ID, admin.ID, barcode, "Butter", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 2 {
			t.Fatalf("len(products) = %d, want 2", len(products))
		}
		if products[0].DeletedAt.Time.After(products[1].DeletedAt.Time) {
			t.Errorf("expected ascending order, got %v then %v", products[0].DeletedAt.Time, products[1].DeletedAt.Time)
		}
	})

	t.Run("privacy: other user's private consumption of same barcode is hidden", func(t *testing.T) {
		privateBarcode := "5555555555555"
		testutil.SeedConsumedProduct(t, db, hh.ID, other.ID, true, privateBarcode, "Tea", "pcs", 1, now.Add(-7*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, other.ID, true, privateBarcode, "Tea", "pcs", 1, now.Add(-3*24*time.Hour))

		productsOther, err := repo.GetConsumedSamples(hh.ID, other.ID, privateBarcode, "Tea", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(productsOther) != 2 {
			t.Errorf("other user: len = %d, want 2 (own private samples)", len(productsOther))
		}

		productsAdmin, err := repo.GetConsumedSamples(hh.ID, admin.ID, privateBarcode, "Tea", since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(productsAdmin) != 0 {
			t.Errorf("admin: len = %d, want 0 (other user's private samples must be hidden)", len(productsAdmin))
		}
	})

	t.Run("falls back to product_name when barcode is empty", func(t *testing.T) {
		name := "Loose Tomatoes"
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "", name, "kg", 1, now.Add(-7*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "", name, "kg", 1, now.Add(-2*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, hh.ID, admin.ID, false, "", "Loose Carrots", "kg", 1, now.Add(-3*24*time.Hour))

		products, err := repo.GetConsumedSamples(hh.ID, admin.ID, "", name, since)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 2 {
			t.Errorf("len(products) = %d, want 2", len(products))
		}
	})

	t.Run("rejects empty barcode and name", func(t *testing.T) {
		_, err := repo.GetConsumedSamples(hh.ID, admin.ID, "", "", since)
		if err == nil {
			t.Errorf("expected error for empty barcode+name, got nil")
		}
	})
}

// seedConsumedWithReason is a small wrapper that lets the repo test
// produce entries with a non-default removalReason (e.g. "wasted") for
// negative-path coverage.
func seedConsumedWithReason(t *testing.T, db *gorm.DB, householdID, userID uint, isPrivate bool, barcode, name, reason string, amount int, unit string, deletedAt time.Time) {
	t.Helper()
	p := dbModel.Product{
		ProductName:   name,
		Barcode:       barcode,
		HouseholdID:   householdID,
		UserID:        userID,
		IsPrivate:     isPrivate,
		Amount:        amount,
		Unit:          unit,
		RemovalReason: reason,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if err := db.Delete(&p).Error; err != nil {
		t.Fatalf("seed soft-delete: %v", err)
	}
	if err := db.Model(&dbModel.Product{}).Unscoped().
		Where("id = ?", p.ID).
		Update("deleted_at", deletedAt).Error; err != nil {
		t.Fatalf("seed update deleted_at: %v", err)
	}
}
