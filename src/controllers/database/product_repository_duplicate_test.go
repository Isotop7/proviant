package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"gorm.io/gorm"
)

// The duplicate lookup stands between the user and a second row for the same
// stock. It must find the exact match, ignore everything else, and never
// disclose a row the caller may not see.
func TestFindDuplicateProduct(t *testing.T) {
	expiry := time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)

	setup := func(t *testing.T) (*gorm.DB, *ProductRepository, uint, uint, uint) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		location := testutil.CreateTestStorageLocation(db, household.ID)
		return db, repo, user.ID, household.ID, location.ID
	}

	draft := func(barcode string, expireAt time.Time, locationID *uint) *database.Product {
		return &database.Product{Barcode: barcode, ExpireAt: expireAt, StorageLocationID: locationID}
	}

	t.Run("an exact match is found", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		existing := database.Product{
			Barcode:           "4006381333931",
			ProductName:       "Yogurt",
			HouseholdID:       householdID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
			Quantity:          2,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		found, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if !isDuplicate {
			t.Fatal("isDuplicate = false, want true")
		}
		if found.ID != existing.ID {
			t.Errorf("found.ID = %d, want %d", found.ID, existing.ID)
		}
	})

	t.Run("a row with a padded barcode is a match", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		existing := database.Product{
			Barcode:           " 4006381333931 ",
			HouseholdID:       householdID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		found, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if !isDuplicate {
			t.Error("isDuplicate = false, want true for a padded barcode")
		}
		if found.ID != existing.ID {
			t.Errorf("found.ID = %d, want %d", found.ID, existing.ID)
		}
	})

	t.Run("a differing expiry is not a match", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry.AddDate(0, 0, 1), &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false for a differing expiry")
		}
	})

	t.Run("a differing location is not a match", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		otherLocationID := locationID + 1
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &otherLocationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false for a differing location")
		}
	})

	t.Run("an archived row is not a match", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := db.Delete(&existing).Error; err != nil {
			t.Fatalf("archive: %v", err)
		}

		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false for an archived row")
		}
	})

	t.Run("another member's private row is neither matched nor disclosed", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		other := testutil.CreateTestUser(db, householdID)
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            other.ID,
			IsPrivate:         true,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		found, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false; the caller must not learn about another member's private row")
		}
		if found.ID != 0 {
			t.Errorf("found.ID = %d, want 0", found.ID)
		}
	})

	t.Run("the caller's own private row is a match", func(t *testing.T) {
		db, repo, userID, householdID, locationID := setup(t)
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       householdID,
			UserID:            userID,
			IsPrivate:         true,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if !isDuplicate {
			t.Error("isDuplicate = false, want true for the caller's own private row")
		}
	})

	t.Run("a row of another household is not a match", func(t *testing.T) {
		db, repo, userID, _, locationID := setup(t)
		otherHousehold := testutil.CreateTestHousehold(db, 0)
		existing := database.Product{
			Barcode:           "4006381333931",
			HouseholdID:       otherHousehold.ID,
			UserID:            userID,
			ExpireAt:          expiry,
			StorageLocationID: &locationID,
		}
		if err := db.Create(&existing).Error; err != nil {
			t.Fatalf("create: %v", err)
		}

		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("4006381333931", expiry, &locationID))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false for a row of another household")
		}
	})

	t.Run("a draft without a barcode matches nothing", func(t *testing.T) {
		_, repo, userID, _, _ := setup(t)
		_, isDuplicate, err := repo.FindDuplicateProduct(userID, draft("", expiry, nil))
		if err != nil {
			t.Fatalf("FindDuplicateProduct: %v", err)
		}
		if isDuplicate {
			t.Error("isDuplicate = true, want false for a draft without a barcode")
		}
	})
}
