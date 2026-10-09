package database

import (
	stderrors "errors"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

var errRollbackForTest = stderrors.New("rollback for test")

func TestInvitationRepository_CreateInvitationCrud(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	t.Run("success sets token, status and expiry", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "crud@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		if inv.ID == 0 || inv.Token == "" {
			t.Errorf("CreateInvitation() = %+v, want ID and token set", inv)
		}
		if inv.Status != dbModel.InvitationStatusPending {
			t.Errorf("Status = %q, want pending", inv.Status)
		}
		if time.Until(inv.ExpiresAt) <= 6*24*time.Hour {
			t.Errorf("ExpiresAt = %v, want about 7 days out", inv.ExpiresAt)
		}
	})

	t.Run("inviter outside the household is not authorized", func(t *testing.T) {
		outsider := testutil.CreateTestUser(db, 0)
		_, err := repo.CreateInvitation(household.ID, outsider.ID, "x@example.com")
		if err != errors.ErrInvitationNotAuthorized {
			t.Errorf("CreateInvitation() error = %v, want %v", err, errors.ErrInvitationNotAuthorized)
		}
	})

	t.Run("duplicate pending invitation is rejected", func(t *testing.T) {
		// Seed locally so the subtest stands alone.
		if _, err := repo.CreateInvitation(household.ID, inviter.ID, "dup@example.com"); err != nil {
			t.Fatalf("seed invitation: %v", err)
		}
		_, err := repo.CreateInvitation(household.ID, inviter.ID, "dup@example.com")
		if err != errors.ErrDuplicateInvitation {
			t.Errorf("CreateInvitation() error = %v, want %v", err, errors.ErrDuplicateInvitation)
		}
	})

	t.Run("after cancel a new invitation is allowed", func(t *testing.T) {
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "reinvite@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		if err := repo.CancelInvitation(inv.ID, inviter.ID); err != nil {
			t.Fatalf("CancelInvitation() error = %v", err)
		}
		if _, err := repo.CreateInvitation(household.ID, inviter.ID, "reinvite@example.com"); err != nil {
			t.Errorf("CreateInvitation() after cancel error = %v, want nil", err)
		}
	})
}

func TestInvitationRepository_CreateInvitationTx(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	t.Run("creates and rolls back with the surrounding transaction", func(t *testing.T) {
		var inv dbModel.HouseholdInvitation
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			inv, err = repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx@example.com")
			if err != nil {
				return err
			}
			return errRollbackForTest
		})
		if err != errRollbackForTest {
			t.Fatalf("transaction error = %v, want %v", err, errRollbackForTest)
		}
		if inv.ID == 0 {
			t.Fatal("CreateInvitationTx() did not return the invitation")
		}
		var count int64
		db.Model(&dbModel.HouseholdInvitation{}).Where("token = ?", inv.Token).Count(&count)
		if count != 0 {
			t.Error("rolled-back transaction left the invitation behind")
		}
	})

	t.Run("committed transaction persists the invitation", func(t *testing.T) {
		var inv dbModel.HouseholdInvitation
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			inv, err = repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx2@example.com")
			return err
		})
		if err != nil {
			t.Fatalf("CreateInvitationTx() error = %v", err)
		}
		found, err := repo.GetInvitationByToken(inv.Token)
		if err != nil {
			t.Fatalf("GetInvitationByToken() error = %v", err)
		}
		if found.ID != inv.ID {
			t.Errorf("found ID = %d, want %d", found.ID, inv.ID)
		}
	})

	t.Run("inviter outside the household is not authorized", func(t *testing.T) {
		outsider := testutil.CreateTestUser(db, 0)
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := repo.CreateInvitationTx(tx, household.ID, outsider.ID, "tx3@example.com")
			return err
		})
		if err != errors.ErrInvitationNotAuthorized {
			t.Errorf("CreateInvitationTx() error = %v, want %v", err, errors.ErrInvitationNotAuthorized)
		}
	})

	t.Run("duplicate pending invitation is rejected", func(t *testing.T) {
		// Seed a committed invitation locally so the subtest stands alone.
		if _, err := repo.CreateInvitation(household.ID, inviter.ID, "tx-dup@example.com"); err != nil {
			t.Fatalf("seed invitation: %v", err)
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := repo.CreateInvitationTx(tx, household.ID, inviter.ID, "tx-dup@example.com")
			return err
		})
		if err != errors.ErrDuplicateInvitation {
			t.Errorf("CreateInvitationTx() error = %v, want %v", err, errors.ErrDuplicateInvitation)
		}
	})
}

func TestInvitationRepository_GetPendingInvitationsForHousehold(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	pending1, err := repo.CreateInvitation(household.ID, inviter.ID, "p1@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if _, err := repo.CreateInvitation(household.ID, inviter.ID, "p2@example.com"); err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	cancelled, err := repo.CreateInvitation(household.ID, inviter.ID, "p3@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if err := repo.CancelInvitation(cancelled.ID, inviter.ID); err != nil {
		t.Fatalf("CancelInvitation() error = %v", err)
	}

	pending, err := repo.GetPendingInvitationsForHousehold(household.ID)
	if err != nil {
		t.Fatalf("GetPendingInvitationsForHousehold() error = %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("GetPendingInvitationsForHousehold() returned %d, want 2", len(pending))
	}
	for _, inv := range pending {
		if inv.Status != dbModel.InvitationStatusPending {
			t.Errorf("invitation %d status = %q, want pending", inv.ID, inv.Status)
		}
		if inv.ID == cancelled.ID {
			t.Error("cancelled invitation returned as pending")
		}
	}
	if pending[0].ID == pending[1].ID {
		t.Errorf("duplicate rows returned: both have ID %d", pending[0].ID)
	}
	if pending[0].ID != pending1.ID && pending[1].ID != pending1.ID {
		t.Errorf("pending invitations = [%d %d], want to include %d", pending[0].ID, pending[1].ID, pending1.ID)
	}
}

func TestInvitationRepository_AcceptInvitation(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *InvitationRepository, dbModel.HouseholdInvitation, *authentication.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		repo := NewInvitationRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		inviter := testutil.CreateTestUser(db, household.ID)
		inv, err := repo.CreateInvitation(household.ID, inviter.ID, "accept@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		invitee := testutil.CreateTestUser(db, 0)
		return db, repo, inv, invitee
	}

	t.Run("unknown token is not found", func(t *testing.T) {
		_, repo, _, invitee := setup(t)
		if err := repo.AcceptInvitation("no-such-token", "accept@example.com", invitee.ID); err != errors.ErrInvitationNotFound {
			t.Errorf("AcceptInvitation() error = %v, want %v", err, errors.ErrInvitationNotFound)
		}
	})

	t.Run("success moves the user into the household", func(t *testing.T) {
		db, repo, inv, invitee := setup(t)
		if err := repo.AcceptInvitation(inv.Token, "accept@example.com", invitee.ID); err != nil {
			t.Fatalf("AcceptInvitation() error = %v", err)
		}

		var user authentication.User
		if err := db.First(&user, invitee.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if user.HouseholdID != inv.HouseholdID {
			t.Errorf("user HouseholdID = %d, want %d", user.HouseholdID, inv.HouseholdID)
		}
		if user.Role != authentication.RoleMember {
			t.Errorf("user Role = %q, want %q", user.Role, authentication.RoleMember)
		}

		updated, err := repo.GetInvitationByToken(inv.Token)
		if err != nil {
			t.Fatalf("GetInvitationByToken() error = %v", err)
		}
		if updated.Status != dbModel.InvitationStatusAccepted {
			t.Errorf("invitation Status = %q, want accepted", updated.Status)
		}
	})

	t.Run("already accepted invitation is rejected", func(t *testing.T) {
		_, repo, inv, invitee := setup(t)
		if err := repo.AcceptInvitation(inv.Token, "accept@example.com", invitee.ID); err != nil {
			t.Fatalf("AcceptInvitation() error = %v", err)
		}
		if err := repo.AcceptInvitation(inv.Token, "accept@example.com", invitee.ID); err != errors.ErrInvitationAlreadyUsed {
			t.Errorf("AcceptInvitation() error = %v, want %v", err, errors.ErrInvitationAlreadyUsed)
		}
	})

	t.Run("cancelled invitation is rejected", func(t *testing.T) {
		_, repo, inv, invitee := setup(t)
		if err := repo.CancelInvitation(inv.ID, inv.InviterID); err != nil {
			t.Fatalf("CancelInvitation() error = %v", err)
		}
		if err := repo.AcceptInvitation(inv.Token, "accept@example.com", invitee.ID); err != errors.ErrInvitationCancelled {
			t.Errorf("AcceptInvitation() error = %v, want %v", err, errors.ErrInvitationCancelled)
		}
	})

	t.Run("expired invitation is rejected and marked expired", func(t *testing.T) {
		db, repo, inv, invitee := setup(t)
		past := time.Now().Add(-time.Hour)
		if err := db.Model(&dbModel.HouseholdInvitation{}).Where("id = ?", inv.ID).Update("expires_at", past).Error; err != nil {
			t.Fatalf("expire invitation: %v", err)
		}
		if err := repo.AcceptInvitation(inv.Token, "accept@example.com", invitee.ID); err != errors.ErrInvitationExpired {
			t.Errorf("AcceptInvitation() error = %v, want %v", err, errors.ErrInvitationExpired)
		}
		var updated dbModel.HouseholdInvitation
		if err := db.First(&updated, inv.ID).Error; err != nil {
			t.Fatalf("reload invitation: %v", err)
		}
		if updated.Status != dbModel.InvitationStatusExpired {
			t.Errorf("invitation Status = %q, want expired", updated.Status)
		}
	})

	t.Run("email mismatch is rejected", func(t *testing.T) {
		_, repo, inv, invitee := setup(t)
		if err := repo.AcceptInvitation(inv.Token, "other@example.com", invitee.ID); err != errors.ErrInvitationEmailMismatch {
			t.Errorf("AcceptInvitation() error = %v, want %v", err, errors.ErrInvitationEmailMismatch)
		}
	})
}

func TestInvitationRepository_CancelInvitationCrud(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	inv, err := repo.CreateInvitation(household.ID, inviter.ID, "cancel@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	t.Run("member of the household can cancel", func(t *testing.T) {
		if err := repo.CancelInvitation(inv.ID, inviter.ID); err != nil {
			t.Fatalf("CancelInvitation() error = %v", err)
		}
		var updated dbModel.HouseholdInvitation
		if err := db.First(&updated, inv.ID).Error; err != nil {
			t.Fatalf("reload invitation: %v", err)
		}
		if updated.Status != dbModel.InvitationStatusCancelled {
			t.Errorf("Status = %q, want cancelled", updated.Status)
		}
	})

	t.Run("user outside the household is not authorized", func(t *testing.T) {
		inv2, err := repo.CreateInvitation(household.ID, inviter.ID, "cancel2@example.com")
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		outsider := testutil.CreateTestUser(db, 0)
		if err := repo.CancelInvitation(inv2.ID, outsider.ID); err != errors.ErrInvitationNotAuthorized {
			t.Errorf("CancelInvitation() error = %v, want %v", err, errors.ErrInvitationNotAuthorized)
		}
	})
}

func TestInvitationRepository_MarkInvitationExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewInvitationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	inv, err := repo.CreateInvitation(household.ID, inviter.ID, "expire@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	if err := repo.MarkInvitationExpired(inv.ID); err != nil {
		t.Fatalf("MarkInvitationExpired() error = %v", err)
	}
	var updated dbModel.HouseholdInvitation
	if err := db.First(&updated, inv.ID).Error; err != nil {
		t.Fatalf("reload invitation: %v", err)
	}
	if updated.Status != dbModel.InvitationStatusExpired {
		t.Errorf("Status = %q, want expired", updated.Status)
	}

	pending, err := repo.GetPendingInvitationsForHousehold(household.ID)
	if err != nil {
		t.Fatalf("GetPendingInvitationsForHousehold() error = %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("GetPendingInvitationsForHousehold() returned %d, want 0", len(pending))
	}
}
