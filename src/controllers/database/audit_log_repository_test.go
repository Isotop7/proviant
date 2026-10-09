package database

import (
	"context"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

// TestAuditLogRepositoryHouseholdScoping proves audit entries are attributed
// at write time and read back only for their own household.
func TestAuditLogRepositoryHouseholdScoping(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewAuditLogRepository(db)
	ctx := context.Background()

	householdA := testutil.CreateTestHousehold(db, 0)
	householdB := testutil.CreateTestHousehold(db, 0)
	userA := testutil.CreateTestUser(db, householdA.ID)
	userB := testutil.CreateTestUser(db, householdB.ID)

	entryTime := time.Date(2026, time.March, 5, 12, 0, 0, 0, time.UTC)

	created := &database.AuditLog{
		Timestamp: entryTime,
		UserID:    &userA.ID,
		Action:    database.AuditActionLoginSuccess,
		IPAddress: "10.0.0.1",
	}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.HouseholdID == nil || *created.HouseholdID != householdA.ID {
		t.Errorf("HouseholdID = %v, want %d (stamped at write time)", created.HouseholdID, householdA.ID)
	}

	if err := repo.Create(ctx, &database.AuditLog{
		Timestamp: entryTime,
		UserID:    &userB.ID,
		Action:    database.AuditActionLoginSuccess,
		IPAddress: "10.0.0.2",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A failed login for a username nobody owns: no household to attribute it to.
	unknownUserID := uint(0)
	if err := repo.Create(ctx, &database.AuditLog{
		Timestamp: entryTime,
		UserID:    &unknownUserID,
		Action:    database.AuditActionLoginFailure,
		IPAddress: "10.0.0.3",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("each household sees only its own entries", func(t *testing.T) {
		for _, tc := range []struct {
			householdID uint
			wantCount   int
		}{
			{householdA.ID, 1},
			{householdB.ID, 1},
		} {
			logs, err := repo.GetAuditLogs(ctx, tc.householdID, 100)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(logs) != tc.wantCount {
				t.Errorf("household %d: got %d entries, want %d", tc.householdID, len(logs), tc.wantCount)
			}
			for _, log := range logs {
				if log.HouseholdID == nil || *log.HouseholdID != tc.householdID {
					t.Errorf("household %d: got entry attributed to %v", tc.householdID, log.HouseholdID)
				}
			}
		}
	})

	t.Run("unattributable entries are invisible to every household", func(t *testing.T) {
		for _, householdID := range []uint{householdA.ID, householdB.ID} {
			logs, err := repo.GetAuditLogs(ctx, householdID, 100)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, log := range logs {
				if log.Action == database.AuditActionLoginFailure {
					t.Errorf("household %d: unattributed login_failure returned", householdID)
				}
			}
		}
	})

	t.Run("date filter is scoped to the household too", func(t *testing.T) {
		logs, err := repo.GetAuditLogsByDate(ctx, householdA.ID, 100, "2026-03-05")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 {
			t.Errorf("got %d entries, want 1", len(logs))
		}

		logs, err = repo.GetAuditLogsByDate(ctx, householdA.ID, 100, "2026-03-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("got %d entries, want 0", len(logs))
		}
	})

	t.Run("caller supplied household id is never overwritten", func(t *testing.T) {
		explicitHousehold := uint(4242)
		log := &database.AuditLog{
			Timestamp:   entryTime,
			UserID:      &userB.ID,
			HouseholdID: &explicitHousehold,
			Action:      database.AuditActionAdminChanged,
			IPAddress:   "10.0.0.4",
		}
		if err := repo.Create(ctx, log); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *log.HouseholdID != explicitHousehold {
			t.Errorf("HouseholdID = %d, want %d", *log.HouseholdID, explicitHousehold)
		}
	})
}
