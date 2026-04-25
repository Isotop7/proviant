package database

import (
	"testing"

	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestHouseholdRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetHouseholdByID", func(t *testing.T) {
		repo := NewHouseholdRepository(db)
		household := testutil.CreateTestHousehold(db, 0)

		foundHousehold, err := repo.GetHouseholdByID(household.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if foundHousehold.ID != household.ID {
			t.Errorf("household.ID = %v, want %v", foundHousehold.ID, household.ID)
		}
	})

	t.Run("GetHouseholdByID not found", func(t *testing.T) {
		repo := NewHouseholdRepository(db)

		_, err := repo.GetHouseholdByID(9999)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})
}