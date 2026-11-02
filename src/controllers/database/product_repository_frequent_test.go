package database

import (
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// archivePurchase creates a product and archives it, pinning its creation date
// so the recency of a purchase is the one the test chose.
func archivePurchase(t *testing.T, db *gorm.DB, householdID, userID uint, barcode, name, packSize string, categoryKey string, locationID *uint, isPrivate bool, createdAt time.Time) {
	t.Helper()
	product := dbModel.Product{
		Barcode:       barcode,
		ProductName:   name,
		PackSize:      packSize,
		HouseholdID:   householdID,
		UserID:        userID,
		IsPrivate:     isPrivate,
		Quantity:      1,
		RemovalReason: dbModel.RemovalReasonConsumed,
	}
	if categoryKey != "" {
		product.CategoryKey = &categoryKey
	}
	product.StorageLocationID = locationID
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := db.Delete(&product).Error; err != nil {
		t.Fatalf("archive product: %v", err)
	}
	if err := db.Model(&dbModel.Product{}).Unscoped().
		Where("id = ?", product.ID).
		Update("created_at", createdAt).Error; err != nil {
		t.Fatalf("pin created_at: %v", err)
	}
}

func TestProductRepository_GetFrequentlyBoughtProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)

	now := time.Now()

	t.Run("an empty archive yields an empty list", func(t *testing.T) {
		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		if len(frequent) != 0 {
			t.Errorf("GetFrequentlyBoughtProducts() returned %d, want 0", len(frequent))
		}
	})

	t.Run("a product bought once never appears", func(t *testing.T) {
		archivePurchase(t, db, household.ID, user.ID, "4000000000009", "Olive Oil", "1 l", "", nil, false, now.AddDate(0, 0, -1))

		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		if len(frequent) != 0 {
			t.Errorf("GetFrequentlyBoughtProducts() returned %d, want 0 for a one-off purchase", len(frequent))
		}
	})

	t.Run("the strip carries the household's own naming", func(t *testing.T) {
		archivePurchase(t, db, household.ID, user.ID, "4006381333931", "Milk", "1 l", "dairy products", &location.ID, false, now.AddDate(0, 0, -2))
		archivePurchase(t, db, household.ID, user.ID, "4006381333931", "Milk", "1 l", "dairy products", &location.ID, false, now.AddDate(0, 0, -1))

		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		if len(frequent) != 1 {
			t.Fatalf("GetFrequentlyBoughtProducts() returned %d, want 1", len(frequent))
		}
		entry := frequent[0]
		if entry.Barcode != "4006381333931" {
			t.Errorf("barcode = %q, want 4006381333931", entry.Barcode)
		}
		if entry.ProductName != "Milk" {
			t.Errorf("name = %q, want Milk", entry.ProductName)
		}
		if entry.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", entry.PackSize)
		}
		if entry.CategoryKey == nil || *entry.CategoryKey != "dairy products" {
			t.Errorf("category = %v, want dairy products", entry.CategoryKey)
		}
		if entry.StorageLocationID == nil || *entry.StorageLocationID != location.ID {
			t.Errorf("storage location = %v, want %d", entry.StorageLocationID, location.ID)
		}
		if entry.PurchaseCount != 2 {
			t.Errorf("purchase count = %d, want 2", entry.PurchaseCount)
		}
	})

	t.Run("another household's archive is not part of the list", func(t *testing.T) {
		otherHousehold := testutil.CreateTestHousehold(db, 0)
		otherUser := testutil.CreateTestUser(db, otherHousehold.ID)
		archivePurchase(t, db, otherHousehold.ID, otherUser.ID, "4000000000010", "Foreign Milk", "1 l", "", nil, false, now.AddDate(0, 0, -1))
		archivePurchase(t, db, otherHousehold.ID, otherUser.ID, "4000000000010", "Foreign Milk", "1 l", "", nil, false, now.AddDate(0, 0, -1))

		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		for _, entry := range frequent {
			if entry.Barcode == "4000000000010" {
				t.Fatalf("the list carries the barcode of another household")
			}
		}
	})

	t.Run("another member's private product never appears and never counts", func(t *testing.T) {
		member := testutil.CreateTestUser(db, household.ID)
		archivePurchase(t, db, household.ID, member.ID, "4000000000011", "Private Coffee", "500 g", "", nil, true, now.AddDate(0, 0, -1))
		archivePurchase(t, db, household.ID, member.ID, "4000000000011", "Private Coffee", "500 g", "", nil, true, now.AddDate(0, 0, -1))

		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		for _, entry := range frequent {
			if entry.Barcode == "4000000000011" {
				t.Fatalf("the list carries another member's private product")
			}
		}

		// The private row must not count towards a barcode the household
		// shares either: one shared purchase plus one private purchase is
		// still one purchase.
		archivePurchase(t, db, household.ID, user.ID, "4000000000012", "Shared Bread", "500 g", "", nil, false, now.AddDate(0, 0, -1))
		archivePurchase(t, db, household.ID, member.ID, "4000000000012", "Shared Bread", "500 g", "", nil, true, now.AddDate(0, 0, -1))

		frequent, err = repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		for _, entry := range frequent {
			if entry.Barcode == "4000000000012" {
				t.Fatalf("a private purchase pushed a once-bought barcode into the list")
			}
		}
	})

	t.Run("the caller's own private product is a staple of their own", func(t *testing.T) {
		archivePurchase(t, db, household.ID, user.ID, "4000000000013", "Private Tea", "100 g", "", nil, true, now.AddDate(0, 0, -1))
		archivePurchase(t, db, household.ID, user.ID, "4000000000013", "Private Tea", "100 g", "", nil, true, now.AddDate(0, 0, -1))

		frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
		if err != nil {
			t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
		}
		found := false
		for _, entry := range frequent {
			if entry.Barcode == "4000000000013" {
				found = true
			}
		}
		if !found {
			t.Errorf("the caller's own private product is missing from the list")
		}
	})
}

// The strip is ordered by frequency first and recency second: a product the
// household buys often beats one it buys just as often but bought long ago.
func TestProductRepository_GetFrequentlyBoughtProductsOrder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	now := time.Now()
	old := now.AddDate(0, 0, -30)
	recent := now.AddDate(0, 0, -1)

	archivePurchase(t, db, household.ID, user.ID, "4000000000020", "Often", "", "", nil, false, old)
	archivePurchase(t, db, household.ID, user.ID, "4000000000020", "Often", "", "", nil, false, old)
	archivePurchase(t, db, household.ID, user.ID, "4000000000020", "Often", "", "", nil, false, recent)

	archivePurchase(t, db, household.ID, user.ID, "4000000000021", "Twice Old", "", "", nil, false, old)
	archivePurchase(t, db, household.ID, user.ID, "4000000000021", "Twice Old", "", "", nil, false, old)

	archivePurchase(t, db, household.ID, user.ID, "4000000000022", "Twice New", "", "", nil, false, recent)
	archivePurchase(t, db, household.ID, user.ID, "4000000000022", "Twice New", "", "", nil, false, recent)

	frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
	if err != nil {
		t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
	}
	if len(frequent) != 3 {
		t.Fatalf("GetFrequentlyBoughtProducts() returned %d, want 3", len(frequent))
	}

	want := []string{"4000000000020", "4000000000022", "4000000000021"}
	for i, barcode := range want {
		if frequent[i].Barcode != barcode {
			t.Errorf("entry %d = %q, want %q", i, frequent[i].Barcode, barcode)
		}
	}
}

// The ranking is one grouped query, so the number of statements does not grow
// with the size of the pantry.
func TestProductRepository_GetFrequentlyBoughtProductsQueryCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	now := time.Now()
	for i := range 50 {
		barcode := "410000000" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		archivePurchase(t, db, household.ID, user.ID, barcode, "Bulk", "", "", nil, false, now)
		archivePurchase(t, db, household.ID, user.ID, barcode, "Bulk", "", "", nil, false, now)
	}

	counter := testutil.NewQueryCounter()
	counting := db.Session(&gorm.Session{Logger: counter})
	repo := NewProductRepository(counting)

	frequent, err := repo.GetFrequentlyBoughtProducts(user.ID, util.FrequentProductLimit)
	if err != nil {
		t.Fatalf("GetFrequentlyBoughtProducts() error = %v", err)
	}
	if len(frequent) != util.FrequentProductLimit {
		t.Fatalf("GetFrequentlyBoughtProducts() returned %d, want %d", len(frequent), util.FrequentProductLimit)
	}

	if got := counter.CountOf("products"); got != 1 {
		t.Errorf("statements against the products table = %d, want 1 for 100 archived rows", got)
	}
}
