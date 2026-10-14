package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestMealPlanRepository_CreateAndGetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewMealPlanRepository(db)

	t.Run("returns created entry", func(t *testing.T) {
		date := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
		entry := testutil.CreateTestMealPlanEntry(db, 1, 5, date, 2)

		got, err := repo.GetByID(entry.ID, 1)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.RecipeID != 5 || got.Servings != 2 {
			t.Errorf("entry = %+v, want recipe 5 with 2 servings", got)
		}
		if !got.Date.Equal(date) {
			t.Errorf("Date = %v, want %v", got.Date, date)
		}
	})

	t.Run("wrong household is not found", func(t *testing.T) {
		entry := testutil.CreateTestMealPlanEntry(db, 1, 5, time.Now(), 1)
		if _, err := repo.GetByID(entry.ID, 2); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByID() error = %v, want gorm.ErrRecordNotFound", err)
		}
	})
}

func TestMealPlanRepository_ListByRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewMealPlanRepository(db)

	t.Run("returns entries within the inclusive range", func(t *testing.T) {
		base := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
		testutil.CreateTestMealPlanEntry(db, 1, 5, base, 1)
		testutil.CreateTestMealPlanEntry(db, 1, 6, base.Add(24*time.Hour), 1)
		testutil.CreateTestMealPlanEntry(db, 1, 7, base.Add(48*time.Hour), 1)

		entries, err := repo.ListByRange(1, base, base.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("ListByRange() error = %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("entries = %d, want 2", len(entries))
		}
		if entries[0].RecipeID != 5 || entries[1].RecipeID != 6 {
			t.Errorf("entry order = [%d, %d], want [5, 6]", entries[0].RecipeID, entries[1].RecipeID)
		}
	})

	t.Run("excludes other households", func(t *testing.T) {
		base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		testutil.CreateTestMealPlanEntry(db, 2, 5, base, 1)

		entries, err := repo.ListByRange(1, base, base.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("ListByRange() error = %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("entries = %d, want 0", len(entries))
		}
	})
}

func TestMealPlanRepository_UpdateAndDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewMealPlanRepository(db)

	t.Run("update persists changed fields", func(t *testing.T) {
		entry := testutil.CreateTestMealPlanEntry(db, 1, 5, time.Now(), 1)
		entry.Servings = 4
		entry.Notes = "guests"
		if err := repo.Update(entry); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		got, err := repo.GetByID(entry.ID, 1)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Servings != 4 || got.Notes != "guests" {
			t.Errorf("entry = %+v, want 4 servings with notes", got)
		}
	})

	t.Run("delete removes only own household entry", func(t *testing.T) {
		entry := testutil.CreateTestMealPlanEntry(db, 1, 5, time.Now(), 1)
		if err := repo.Delete(entry.ID, 2); err != nil {
			t.Fatalf("Delete() wrong household error = %v", err)
		}
		if _, err := repo.GetByID(entry.ID, 1); err != nil {
			t.Errorf("GetByID() error = %v, want entry to survive", err)
		}
		if err := repo.Delete(entry.ID, 1); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := repo.GetByID(entry.ID, 1); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByID() error = %v, want gorm.ErrRecordNotFound", err)
		}
	})
}
