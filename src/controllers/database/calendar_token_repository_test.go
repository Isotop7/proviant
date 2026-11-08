package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestCalendarTokenRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewCalendarTokenRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	ct := authentication.CalendarToken{
		UserID:    user.ID,
		Token:     "cal-token-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	t.Run("create and get by token", func(t *testing.T) {
		if err := repo.Create(&ct); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if ct.ID == 0 {
			t.Error("Create() did not set ID")
		}
		found, err := repo.GetByToken("cal-token-1")
		if err != nil {
			t.Fatalf("GetByToken() error = %v", err)
		}
		if found.UserID != user.ID {
			t.Errorf("GetByToken() UserID = %d, want %d", found.UserID, user.ID)
		}
	})

	t.Run("get by token not found", func(t *testing.T) {
		if _, err := repo.GetByToken("missing"); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByToken() error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		expired := authentication.CalendarToken{
			UserID:    user.ID,
			Token:     "cal-token-expired",
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		if err := repo.Create(&expired); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := repo.GetByToken("cal-token-expired"); err != errors.ErrTokenExpired {
			t.Errorf("GetByToken() error = %v, want %v", err, errors.ErrTokenExpired)
		}
	})

	t.Run("get by user id", func(t *testing.T) {
		// A dedicated user keeps this independent of the sibling subtests.
		tokenUser := testutil.CreateTestUser(db, household.ID)
		seed := authentication.CalendarToken{
			UserID:    tokenUser.ID,
			Token:     "cal-token-user",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := repo.Create(&seed); err != nil {
			t.Fatalf("seed token: %v", err)
		}
		found, err := repo.GetByUserID(tokenUser.ID)
		if err != nil {
			t.Fatalf("GetByUserID() error = %v", err)
		}
		// GetByUserID orders by primary key, so the earliest-created token wins.
		if found.Token != "cal-token-user" {
			t.Errorf("GetByUserID() Token = %q, want %q", found.Token, "cal-token-user")
		}
		if _, err := repo.GetByUserID(9999); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByUserID(9999) error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("update persists changes", func(t *testing.T) {
		// Seed a token in this subtest so it does not rely on an earlier one.
		updateUser := testutil.CreateTestUser(db, household.ID)
		token := authentication.CalendarToken{
			UserID:    updateUser.ID,
			Token:     "cal-token-update",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := repo.Create(&token); err != nil {
			t.Fatalf("seed token: %v", err)
		}
		token.ExpiresAt = time.Now().Add(48 * time.Hour)
		if err := repo.Update(&token); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		found, err := repo.GetByToken("cal-token-update")
		if err != nil {
			t.Fatalf("GetByToken() error = %v", err)
		}
		if time.Until(found.ExpiresAt) <= 24*time.Hour {
			t.Errorf("ExpiresAt = %v, want updated to ~48h out", found.ExpiresAt)
		}
	})

	t.Run("delete by user id removes all tokens", func(t *testing.T) {
		if err := repo.DeleteByUserID(user.ID); err != nil {
			t.Fatalf("DeleteByUserID() error = %v", err)
		}
		if _, err := repo.GetByUserID(user.ID); err != gorm.ErrRecordNotFound {
			t.Errorf("GetByUserID() after delete error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})
}
