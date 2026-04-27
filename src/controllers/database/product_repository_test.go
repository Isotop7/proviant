package database

import (
	"testing"

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
}
