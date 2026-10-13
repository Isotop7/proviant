package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func TestPATRepository_CreateAndGet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewPATRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	expiry := time.Now().Add(24 * time.Hour)

	t.Run("create assigns id and stores fields", func(t *testing.T) {
		pat, err := repo.CreatePAT(user.ID, "CI token", "hash-1", &expiry, "read write")
		if err != nil {
			t.Fatalf("CreatePAT() error = %v", err)
		}
		if pat.ID == 0 {
			t.Error("CreatePAT() did not set ID")
		}
		if pat.UserID != user.ID || pat.Name != "CI token" || pat.TokenHash != "hash-1" || pat.Scopes != "read write" {
			t.Errorf("CreatePAT() = %+v, fields not stored", pat)
		}
		if pat.ExpiresAt == nil || !pat.ExpiresAt.Equal(expiry) {
			t.Errorf("ExpiresAt = %v, want %v", pat.ExpiresAt, expiry)
		}
	})

	t.Run("duplicate token hash violates unique index", func(t *testing.T) {
		if _, err := repo.CreatePAT(user.ID, "dup-a", "hash-dup", nil, ""); err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		if _, err := repo.CreatePAT(user.ID, "dup-b", "hash-dup", nil, ""); err == nil {
			t.Error("CreatePAT() with duplicate hash: error = nil, want error")
		}
	})

	t.Run("get by token hash", func(t *testing.T) {
		if _, err := repo.CreatePAT(user.ID, "by-hash", "hash-get", nil, ""); err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		found, err := repo.GetPATByTokenHash("hash-get")
		if err != nil {
			t.Fatalf("GetPATByTokenHash() error = %v", err)
		}
		if found.UserID != user.ID {
			t.Errorf("GetPATByTokenHash() UserID = %d, want %d", found.UserID, user.ID)
		}
	})

	t.Run("get by token hash not found", func(t *testing.T) {
		_, err := repo.GetPATByTokenHash("missing")
		if err != gorm.ErrRecordNotFound {
			t.Errorf("GetPATByTokenHash() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("get by id", func(t *testing.T) {
		pat, err := repo.CreatePAT(user.ID, "by-id", "hash-2", nil, "")
		if err != nil {
			t.Fatalf("CreatePAT() error = %v", err)
		}
		found, err := repo.GetPATByID(pat.ID)
		if err != nil {
			t.Fatalf("GetPATByID() error = %v", err)
		}
		if found.TokenHash != "hash-2" {
			t.Errorf("GetPATByID() TokenHash = %q, want hash-2", found.TokenHash)
		}
		if _, err := repo.GetPATByID(9999); err != gorm.ErrRecordNotFound {
			t.Errorf("GetPATByID(9999) error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("list by user ordered by created_at desc", func(t *testing.T) {
		// A dedicated user keeps the count independent of the sibling subtests.
		listUser := testutil.CreateTestUser(db, household.ID)
		if _, err := repo.CreatePAT(listUser.ID, "first", "hash-list-1", nil, ""); err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		if _, err := repo.CreatePAT(listUser.ID, "second", "hash-list-2", nil, ""); err != nil {
			t.Fatalf("seed PAT: %v", err)
		}

		pats, err := repo.GetPATsByUserID(listUser.ID)
		if err != nil {
			t.Fatalf("GetPATsByUserID() error = %v", err)
		}
		if len(pats) != 2 {
			t.Fatalf("GetPATsByUserID() returned %d pats, want 2", len(pats))
		}
		if pats[0].CreatedAt.Before(pats[1].CreatedAt) {
			t.Error("GetPATsByUserID() not ordered by created_at DESC")
		}

		empty, err := repo.GetPATsByUserID(9999)
		if err != nil {
			t.Fatalf("GetPATsByUserID() error = %v", err)
		}
		if len(empty) != 0 {
			t.Errorf("GetPATsByUserID() returned %d pats, want 0", len(empty))
		}
	})
}

func TestPATRepository_DeleteAndLastUsed(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewPATRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	other := testutil.CreateTestUser(db, household.ID)

	pat, err := repo.CreatePAT(user.ID, "to-delete", "hash-del", nil, "")
	if err != nil {
		t.Fatalf("CreatePAT() error = %v", err)
	}

	t.Run("update last used sets timestamp", func(t *testing.T) {
		if err := repo.UpdateLastUsed(pat.ID); err != nil {
			t.Fatalf("UpdateLastUsed() error = %v", err)
		}
		found, err := repo.GetPATByID(pat.ID)
		if err != nil {
			t.Fatalf("GetPATByID() error = %v", err)
		}
		if found.LastUsedAt == nil || found.LastUsedAt.IsZero() {
			t.Error("UpdateLastUsed() did not set LastUsedAt")
		}
	})

	t.Run("delete by another user reports not found", func(t *testing.T) {
		if err := repo.DeletePAT(pat.ID, other.ID); err != gorm.ErrRecordNotFound {
			t.Errorf("DeletePAT() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("delete unknown id reports not found", func(t *testing.T) {
		if err := repo.DeletePAT(9999, user.ID); err != gorm.ErrRecordNotFound {
			t.Errorf("DeletePAT() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("delete removes the token", func(t *testing.T) {
		if err := repo.DeletePAT(pat.ID, user.ID); err != nil {
			t.Fatalf("DeletePAT() error = %v", err)
		}
		if _, err := repo.GetPATByID(pat.ID); err == nil {
			t.Error("GetPATByID() after delete: error = nil, want error")
		}
	})
}
