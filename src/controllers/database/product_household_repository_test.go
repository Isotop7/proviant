package database

import (
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func TestProductRepository_FilteredProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	householdRepo := NewProductHouseholdRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	active := testutil.CreateTestProduct(db, household.ID, user.ID)
	oldActive := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(&dbModel.Product{}).Where("id = ?", oldActive.ID).Update("created_at", yesterday.Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate product: %v", err)
	}

	archived := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(archived).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	oldArchived := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Delete(oldArchived).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", oldArchived.ID).Update("deleted_at", yesterday.Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate deleted_at: %v", err)
	}

	t.Run("active filtered without bounds returns all active", func(t *testing.T) {
		products, err := householdRepo.GetUserActiveProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 2 {
			t.Errorf("GetUserActiveProductsFiltered() returned %d, want 2", len(products))
		}
	})

	t.Run("active filtered by from excludes older rows", func(t *testing.T) {
		products, err := householdRepo.GetUserActiveProductsFiltered(user.ID, &yesterday, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != active.ID {
			t.Errorf("GetUserActiveProductsFiltered() = %+v, want only product %d", products, active.ID)
		}
	})

	t.Run("active filtered by to excludes newer rows", func(t *testing.T) {
		products, err := householdRepo.GetUserActiveProductsFiltered(user.ID, nil, &yesterday)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != oldActive.ID {
			t.Errorf("GetUserActiveProductsFiltered() = %+v, want only product %d", products, oldActive.ID)
		}
	})

	t.Run("archived filtered without bounds returns all archived", func(t *testing.T) {
		products, err := householdRepo.GetUserArchivedProductsFiltered(user.ID, nil, nil)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 2 {
			t.Errorf("GetUserArchivedProductsFiltered() returned %d, want 2", len(products))
		}
	})

	t.Run("archived filtered by from excludes older deletions", func(t *testing.T) {
		products, err := householdRepo.GetUserArchivedProductsFiltered(user.ID, &yesterday, nil)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != archived.ID {
			t.Errorf("GetUserArchivedProductsFiltered() = %+v, want only product %d", products, archived.ID)
		}
	})

	t.Run("archived filtered by to excludes newer deletions", func(t *testing.T) {
		products, err := householdRepo.GetUserArchivedProductsFiltered(user.ID, nil, &yesterday)
		if err != nil {
			t.Fatalf("GetUserArchivedProductsFiltered() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != oldArchived.ID {
			t.Errorf("GetUserArchivedProductsFiltered() = %+v, want only product %d", products, oldArchived.ID)
		}
	})

	t.Run("window excluding everything returns empty", func(t *testing.T) {
		products, err := householdRepo.GetUserActiveProductsFiltered(user.ID, &tomorrow, nil)
		if err != nil {
			t.Fatalf("GetUserActiveProductsFiltered() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserActiveProductsFiltered() returned %d, want 0", len(products))
		}
	})
}

func TestProductRepository_GetExpiringProductsByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	householdRepo := NewProductHouseholdRepository(db)
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

	products, err := householdRepo.GetExpiringProductsByHousehold(household.ID, 7)
	if err != nil {
		t.Fatalf("GetExpiringProductsByHousehold() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != soon.ID {
		t.Errorf("GetExpiringProductsByHousehold() = %+v, want only product %d", products, soon.ID)
	}

	t.Run("wider window includes later product, sorted ascending", func(t *testing.T) {
		products, err := householdRepo.GetExpiringProductsByHousehold(household.ID, 30)
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
		products, err := householdRepo.GetExpiringProductsByHousehold(9999, 7)
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
	householdRepo := NewProductHouseholdRepository(db)
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

	products, err := householdRepo.GetProductsByHousehold(household.ID)
	if err != nil {
		t.Fatalf("GetProductsByHousehold() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("GetProductsByHousehold() returned %d, want 1 (public, active only)", len(products))
	}

	products, err = householdRepo.GetProductsByHousehold(9999)
	if err != nil {
		t.Fatalf("GetProductsByHousehold() error = %v", err)
	}
	if len(products) != 0 {
		t.Errorf("GetProductsByHousehold() returned %d, want 0", len(products))
	}
}
func TestProductRepository_GetSubThresholdProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	householdRepo := NewProductHouseholdRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	owner := testutil.CreateTestUser(db, household.ID)

	below := testutil.CreateTestProduct(db, household.ID, user.ID)
	below.Quantity = 1
	below.MinStockAmount = 5
	if err := db.Save(below).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	atThreshold := testutil.CreateTestProduct(db, household.ID, user.ID)
	atThreshold.Quantity = 5
	atThreshold.MinStockAmount = 5
	if err := db.Save(atThreshold).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	noThreshold := testutil.CreateTestProduct(db, household.ID, user.ID)
	noThreshold.Quantity = 0
	noThreshold.MinStockAmount = 0
	if err := db.Save(noThreshold).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	privateBelow := testutil.CreateTestProduct(db, household.ID, owner.ID)
	privateBelow.IsPrivate = true
	privateBelow.Quantity = 1
	privateBelow.MinStockAmount = 5
	if err := db.Save(privateBelow).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("returns only below-threshold visible products", func(t *testing.T) {
		products, err := householdRepo.GetSubThresholdProducts(user.ID)
		if err != nil {
			t.Fatalf("GetSubThresholdProducts() error = %v", err)
		}
		if len(products) != 1 || products[0].ID != below.ID {
			t.Errorf("GetSubThresholdProducts() = %+v, want only product %d", products, below.ID)
		}
	})

	t.Run("owner sees own private below-threshold product", func(t *testing.T) {
		products, err := householdRepo.GetSubThresholdProducts(owner.ID)
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
		if _, err := householdRepo.GetSubThresholdProducts(9999); err == nil {
			t.Error("GetSubThresholdProducts(9999) error = nil, want error")
		}
	})
}
