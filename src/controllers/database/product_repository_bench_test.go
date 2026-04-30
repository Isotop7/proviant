package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

const (
	seedHouseholdCount       = 10
	seedProductsPerHousehold = 1000
	seedTotalProducts        = seedHouseholdCount * seedProductsPerHousehold
)

func seedTestData(t *testing.T, db *gorm.DB) (userIDs []uint) {
	t.Helper()

	// Create households and users
	for i := 0; i < seedHouseholdCount; i++ {
		household := dbModel.Household{
			Name:    "Test Household " + string(rune('A'+i)),
			AdminID: 0,
		}
		if err := db.Create(&household).Error; err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		user := authentication.User{
			Username:    "testuser" + string(rune('A'+i)),
			MailAddress: "test" + string(rune('A'+i)) + "@example.com",
			HouseholdID: household.ID,
			Password:    "password",
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		userIDs = append(userIDs, user.ID)

		// Create storage location for the household
		storageLoc := dbModel.StorageLocation{
			HouseholdID: household.ID,
			Name:        "Fridge",
			Icon:        "🧊",
			SortOrder:   0,
		}
		if err := db.Create(&storageLoc).Error; err != nil {
			t.Fatalf("failed to create storage location: %v", err)
		}

		// Seed products for this household
		for j := 0; j < seedProductsPerHousehold; j++ {
			product := dbModel.Product{
				HouseholdID:       household.ID,
				ProductName:       "Test Product " + string(rune('A'+i)) + "-" + string(rune('0'+j%10)),
				Barcode:           string(rune('0'+j%10)) + string(rune('0'+(j/10)%10)) + string(rune('0'+(j/100)%10)) + "000000000",
				ExpireAt:          time.Now().AddDate(0, 0, j%30),
				ScannedAt:         time.Now(),
				Amount:            1,
				Unit:              "pcs",
				StorageLocationID: &storageLoc.ID,
			}
			if err := db.Create(&product).Error; err != nil {
				t.Fatalf("failed to create product: %v", err)
			}
		}
	}
	return userIDs
}

func TestProductListPerformance10k(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userIDs := seedTestData(t, db)
	repo := NewProductRepository(db)

	// Use first user for testing
	userID := userIDs[0]

	// Warm up query to avoid cold start overhead
	_, _ = repo.GetUserProductsBulk(userID, 0)

	start := time.Now()
	products, err := repo.GetUserProductsBulk(userID, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetUserProductsBulk failed: %v", err)
	}
	if len(products) != seedProductsPerHousehold {
		t.Errorf("expected %d products, got %d", seedProductsPerHousehold, len(products))
	}
	// Allow 150ms to account for in-memory sort of 1000 products
	if elapsed >= 150*time.Millisecond {
		t.Errorf("product list query exceeded budget: got %v, want <150ms", elapsed)
	}
}

func BenchmarkGetUserProductsBulk(b *testing.B) {
	db := testutil.SetupTestDB(&testing.T{})
	userIDs := seedTestData(&testing.T{}, db)
	repo := NewProductRepository(db)
	userID := userIDs[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetUserProductsBulk(userID, 0)
		if err != nil {
			b.Fatalf("GetUserProductsBulk failed: %v", err)
		}
	}
}

func BenchmarkGetExpiringInDays(b *testing.B) {
	db := testutil.SetupTestDB(&testing.T{})
	userIDs := seedTestData(&testing.T{}, db)
	repo := NewProductRepository(db)
	userID := userIDs[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetExpiringInDays(userID, 7)
		if err != nil {
			b.Fatalf("GetExpiringInDays failed: %v", err)
		}
	}
}

func BenchmarkGetUserProductsBulkByBarcode(b *testing.B) {
	db := testutil.SetupTestDB(&testing.T{})
	userIDs := seedTestData(&testing.T{}, db)
	repo := NewProductRepository(db)
	userID := userIDs[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetUserProductsBulkByBarcode(userID, 123)
		if err != nil {
			b.Fatalf("GetUserProductsBulkByBarcode failed: %v", err)
		}
	}
}

func BenchmarkGetExpiringSoonCount(b *testing.B) {
	db := testutil.SetupTestDB(&testing.T{})
	userIDs := seedTestData(&testing.T{}, db)
	repo := NewProductRepository(db)
	userID := userIDs[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetExpiringSoonCount(userID, 7)
		if err != nil {
			b.Fatalf("GetExpiringSoonCount failed: %v", err)
		}
	}
}
