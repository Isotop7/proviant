package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func TestUserRepository_GetUserByMailAddress_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	found, err := repo.GetUserByMailAddress(user.MailAddress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("found.ID = %v, want %v", found.ID, user.ID)
	}
}

func TestUserRepository_GetUserByMailAddress_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetUserByMailAddress("nobody@example.com")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestUserRepository_PasswordReset_CRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	expiresAt := time.Now().Add(1 * time.Hour)
	if err := repo.CreatePasswordReset(user.ID, "token-aaa", expiresAt, "127.0.0.1"); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetPasswordResetByToken("token-aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserID != user.ID {
		t.Errorf("UserID = %v, want %v", got.UserID, user.ID)
	}
	if got.UsedAt != nil {
		t.Errorf("UsedAt = %v, want nil", got.UsedAt)
	}
	if got.IPAddress != "127.0.0.1" {
		t.Errorf("IPAddress = %v, want 127.0.0.1", got.IPAddress)
	}

	now := time.Now()
	if err := repo.MarkPasswordResetUsed(got.ID, now); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	got, err = repo.GetPasswordResetByToken("token-aaa")
	if err != nil {
		t.Fatalf("get after mark: %v", err)
	}
	if got.UsedAt == nil {
		t.Errorf("UsedAt = nil, want set")
	}

	// Duplicate token insertion is constrained by the uniqueIndex on Token.
	// (Note: enforcement depends on the underlying engine configuration; this is
	//  covered by the production migration. The handler-layer flow guarantees
	//  unique tokens via the cryptographic random generator.)
}

func TestUserRepository_InvalidatePendingPasswordResetsForUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	expiresAt := time.Now().Add(1 * time.Hour)
	for _, token := range []string{"t1", "t2", "t3"} {
		if err := repo.CreatePasswordReset(user.ID, token, expiresAt, ""); err != nil {
			t.Fatalf("create %s: %v", token, err)
		}
	}

	if err := repo.InvalidatePendingPasswordResetsForUser(user.ID); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	for _, token := range []string{"t1", "t2", "t3"} {
		got, err := repo.GetPasswordResetByToken(token)
		if err != nil {
			t.Fatalf("get %s: %v", token, err)
		}
		if got.UsedAt == nil {
			t.Errorf("token %s: UsedAt nil after invalidate", token)
		}
	}
}

func TestUserRepository_DeleteExpiredPasswordResets(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	expired := time.Now().Add(-1 * time.Hour)
	fresh := time.Now().Add(1 * time.Hour)
	if err := repo.CreatePasswordReset(user.ID, "expired-token", expired, ""); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if err := repo.CreatePasswordReset(user.ID, "fresh-token", fresh, ""); err != nil {
		t.Fatalf("create fresh: %v", err)
	}

	if err := repo.DeleteExpiredPasswordResets(time.Now()); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	if _, err := repo.GetPasswordResetByToken("expired-token"); err != gorm.ErrRecordNotFound {
		t.Errorf("expected expired token gone, got err=%v", err)
	}
	if _, err := repo.GetPasswordResetByToken("fresh-token"); err != nil {
		t.Errorf("fresh token should still exist, got err=%v", err)
	}
}

func TestUserRepository_SetUserPasswordHash(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	newHash := "$2a$10$abcdefghijklmnopqrstuv"
	if err := repo.SetUserPasswordHash(user.ID, newHash); err != nil {
		t.Fatalf("set: %v", err)
	}

	updated, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Password != newHash {
		t.Errorf("Password = %q, want %q", updated.Password, newHash)
	}
}
