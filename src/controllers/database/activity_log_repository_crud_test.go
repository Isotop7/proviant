package database

import (
	"context"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestActivityLogRepository_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewActivityLogRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	ctx := context.Background()

	t.Run("zero timestamp is set to now", func(t *testing.T) {
		log := dbModel.ActivityLog{
			HouseholdID: household.ID,
			UserID:      &user.ID,
			UserName:    user.Username,
			Action:      dbModel.ActivityActionAdd,
			ProductName: "Milk",
			Quantity:    1,
		}
		if err := repo.Create(ctx, &log); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if log.ID == 0 {
			t.Error("Create() did not set ID")
		}
		if log.Timestamp.IsZero() {
			t.Error("Create() left zero Timestamp unset")
		}
		if time.Since(log.Timestamp) > time.Minute {
			t.Errorf("Timestamp = %v, want roughly now", log.Timestamp)
		}
	})

	t.Run("explicit timestamp is preserved", func(t *testing.T) {
		stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		log := dbModel.ActivityLog{
			HouseholdID: household.ID,
			UserName:    user.Username,
			Action:      dbModel.ActivityActionConsume,
			ProductName: "Bread",
			Timestamp:   stamp,
		}
		if err := repo.Create(ctx, &log); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if !log.Timestamp.Equal(stamp) {
			t.Errorf("Timestamp = %v, want %v", log.Timestamp, stamp)
		}
	})
}

func TestActivityLogRepository_GetByHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewActivityLogRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		log := dbModel.ActivityLog{
			HouseholdID: household.ID,
			UserName:    "testuser",
			Action:      dbModel.ActivityActionAdd,
			ProductName: "Product",
			Timestamp:   base.Add(time.Duration(i) * time.Minute),
		}
		if err := db.Create(&log).Error; err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}
	other := dbModel.ActivityLog{HouseholdID: otherHousehold.ID, UserName: "other", Action: dbModel.ActivityActionAdd, ProductName: "X", Timestamp: base}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("seed create: %v", err)
	}

	t.Run("returns household logs newest first", func(t *testing.T) {
		logs, err := repo.GetByHousehold(ctx, household.ID, 10, 0)
		if err != nil {
			t.Fatalf("GetByHousehold() error = %v", err)
		}
		if len(logs) != 5 {
			t.Fatalf("GetByHousehold() returned %d logs, want 5", len(logs))
		}
		for i := 1; i < len(logs); i++ {
			if logs[i-1].Timestamp.Before(logs[i].Timestamp) {
				t.Error("GetByHousehold() not ordered by timestamp DESC")
			}
		}
	})

	t.Run("limit and offset page through results", func(t *testing.T) {
		logs, err := repo.GetByHousehold(ctx, household.ID, 2, 2)
		if err != nil {
			t.Fatalf("GetByHousehold() error = %v", err)
		}
		if len(logs) != 2 {
			t.Fatalf("GetByHousehold() returned %d logs, want 2", len(logs))
		}
	})

	t.Run("empty household returns no logs", func(t *testing.T) {
		logs, err := repo.GetByHousehold(ctx, 9999, 10, 0)
		if err != nil {
			t.Fatalf("GetByHousehold() error = %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("GetByHousehold() returned %d logs, want 0", len(logs))
		}
	})
}

func TestActivityLogRepository_GetByHouseholdCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewActivityLogRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		log := dbModel.ActivityLog{
			HouseholdID: household.ID,
			UserName:    "testuser",
			Action:      dbModel.ActivityActionWaste,
			ProductName: "Product",
		}
		if err := db.Create(&log).Error; err != nil {
			t.Fatalf("seed create: %v", err)
		}
	}

	count, err := repo.GetByHouseholdCount(ctx, household.ID)
	if err != nil {
		t.Fatalf("GetByHouseholdCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("GetByHouseholdCount() = %d, want 3", count)
	}

	count, err = repo.GetByHouseholdCount(ctx, 9999)
	if err != nil {
		t.Fatalf("GetByHouseholdCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("GetByHouseholdCount() = %d, want 0", count)
	}
}
