package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func TestNotificationRepository_WithLogger(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()
	repo := NewNotificationRepositoryWithLogger(db, &logger)

	if repo.DB != db {
		t.Error("NewNotificationRepositoryWithLogger() did not set DB")
	}
	if repo.Logger == nil {
		t.Error("NewNotificationRepositoryWithLogger() did not set Logger")
	}

	// The logger-backed repository must be fully functional.
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	token, err := repo.GenerateMailDigestUnsubscribeToken(user.ID)
	if err != nil {
		t.Fatalf("GenerateMailDigestUnsubscribeToken() error = %v", err)
	}
	if token == "" {
		t.Error("GenerateMailDigestUnsubscribeToken() = empty token")
	}
}

func TestNotificationRepository_AcceptInvitationPassthrough(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)
	invitee := testutil.CreateTestUser(db, 0)

	inv, err := repo.CreateInvitation(household.ID, inviter.ID, "passthrough-accept@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	if err := repo.AcceptInvitation(inv.Token, "passthrough-accept@example.com", invitee.ID); err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}

	var user authentication.User
	if err := db.First(&user, invitee.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if user.HouseholdID != household.ID {
		t.Errorf("user HouseholdID = %d, want %d", user.HouseholdID, household.ID)
	}

	got, err := repo.GetInvitationByToken(inv.Token)
	if err != nil {
		t.Fatalf("GetInvitationByToken() error = %v", err)
	}
	if got.Status != dbModel.InvitationStatusAccepted {
		t.Errorf("invitation Status = %q, want accepted", got.Status)
	}
}

func TestNotificationRepository_MarkInvitationSentTxPassthrough(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	inv, err := repo.CreateInvitation(household.ID, inviter.ID, "tx-sent@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		return repo.MarkInvitationSentTx(tx, inv.ID)
	})
	if err != nil {
		t.Fatalf("MarkInvitationSentTx() error = %v", err)
	}

	var updated dbModel.HouseholdInvitation
	if err := db.First(&updated, inv.ID).Error; err != nil {
		t.Fatalf("reload invitation: %v", err)
	}
	if updated.SentAt == nil {
		t.Error("SentAt not set by MarkInvitationSentTx")
	}
	if updated.SendAttempts != 1 {
		t.Errorf("SendAttempts = %d, want 1", updated.SendAttempts)
	}

	pending, err := repo.GetPendingInvitationsNotSent(time.Hour)
	if err != nil {
		t.Fatalf("GetPendingInvitationsNotSent() error = %v", err)
	}
	for _, p := range pending {
		if p.ID == inv.ID {
			t.Error("sent invitation still returned by GetPendingInvitationsNotSent")
		}
	}
}

func TestNotificationRepository_GetPublicHouseholds(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	householdA := testutil.CreateTestHousehold(db, 0)
	householdB := testutil.CreateTestHousehold(db, 0)
	testutil.CreateTestUser(db, householdA.ID)
	testutil.CreateTestUser(db, householdA.ID)
	testutil.CreateTestUser(db, householdB.ID)

	t.Run("returns households with member counts", func(t *testing.T) {
		households, err := repo.GetPublicHouseholds(0)
		if err != nil {
			t.Fatalf("GetPublicHouseholds() error = %v", err)
		}
		if len(households) != 2 {
			t.Fatalf("GetPublicHouseholds() returned %d, want 2", len(households))
		}
		counts := map[uint]int{}
		for _, h := range households {
			counts[h.ID] = h.MemberCount
		}
		if counts[householdA.ID] != 2 {
			t.Errorf("householdA MemberCount = %d, want 2", counts[householdA.ID])
		}
		if counts[householdB.ID] != 1 {
			t.Errorf("householdB MemberCount = %d, want 1", counts[householdB.ID])
		}
	})

	t.Run("excludes the given household", func(t *testing.T) {
		households, err := repo.GetPublicHouseholds(householdA.ID)
		if err != nil {
			t.Fatalf("GetPublicHouseholds() error = %v", err)
		}
		if len(households) != 1 || households[0].ID != householdB.ID {
			t.Errorf("GetPublicHouseholds() = %+v, want only householdB", households)
		}
	})
}

func TestNotificationRepository_GetWasteStatsForHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Deleted this month (counts as wasted regardless of reason).
	deleted := testutil.CreateTestProduct(db, household.ID, user.ID)
	deleted.RemovalReason = dbModel.RemovalReasonWasted
	if err := db.Save(deleted).Error; err != nil {
		t.Fatalf("save deleted product: %v", err)
	}
	if err := db.Delete(deleted).Error; err != nil {
		t.Fatalf("delete product: %v", err)
	}
	// Pin deleted_at to the frozen window instant: GetWasteStatsForHousehold
	// derives its window from `now`, while db.Delete writes wall-clock time,
	// so a month-edge straddle would put deleted_at outside [start, end).
	if err := db.Model(&dbModel.Product{}).Unscoped().
		Where("id = ?", deleted.ID).
		Update("deleted_at", now).Error; err != nil {
		t.Fatalf("pin deleted_at: %v", err)
	}

	// Active product expiring this month (counts as expired waste).
	expiring := testutil.CreateTestProduct(db, household.ID, user.ID)
	expiring.ExpireAt = now
	if err := db.Save(expiring).Error; err != nil {
		t.Fatalf("save expiring product: %v", err)
	}

	// Deleted last month: previous-month denominator/numerator only.
	lastMonth := monthStart.Add(-time.Hour)
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "9999999999999", "Old", "pcs", 1, lastMonth)
	var oldProduct dbModel.Product
	if err := db.Unscoped().Where("barcode = ?", "9999999999999").First(&oldProduct).Error; err != nil {
		t.Fatalf("reload old product: %v", err)
	}
	if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", oldProduct.ID).
		Update("created_at", lastMonth.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}

	stats, err := repo.GetWasteStatsForHousehold(household.ID, now)
	if err != nil {
		t.Fatalf("GetWasteStatsForHousehold() error = %v", err)
	}

	if stats.HouseholdID != household.ID {
		t.Errorf("HouseholdID = %d, want %d", stats.HouseholdID, household.ID)
	}
	if stats.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", stats.DeletedCount)
	}
	if stats.ExpiredCount != 1 {
		t.Errorf("ExpiredCount = %d, want 1", stats.ExpiredCount)
	}
	if stats.WastedCount != 2 {
		t.Errorf("WastedCount = %d, want 2", stats.WastedCount)
	}
	if stats.PrevWastedCount != 1 {
		t.Errorf("PrevWastedCount = %d, want 1", stats.PrevWastedCount)
	}
	if stats.MonthLabel != monthStart.Format("January 2006") {
		t.Errorf("MonthLabel = %q, want %q", stats.MonthLabel, monthStart.Format("January 2006"))
	}
	// Active this month: the expiring product only (the deleted one was
	// created and deleted inside the window, so created_at < monthEnd but
	// deleted_at >= monthStart — it still counts as active-at-some-point).
	if stats.WasteRatePct <= 0 {
		t.Errorf("WasteRatePct = %v, want > 0", stats.WasteRatePct)
	}

	t.Run("empty household reports zeros with neutral delta", func(t *testing.T) {
		empty := testutil.CreateTestHousehold(db, 0)
		stats, err := repo.GetWasteStatsForHousehold(empty.ID, now)
		if err != nil {
			t.Fatalf("GetWasteStatsForHousehold() error = %v", err)
		}
		if stats.WastedCount != 0 || stats.WasteRatePct != 0 {
			t.Errorf("stats = %+v, want zero waste", stats)
		}
		if stats.DeltaSymbol != "=" || stats.DeltaColor != "#6A9580" {
			t.Errorf("Delta = %v %v, want neutral = #6A9580", stats.DeltaSymbol, stats.DeltaColor)
		}
	})
}
