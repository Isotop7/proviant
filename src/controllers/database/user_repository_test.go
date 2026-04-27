package database

import (
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
	"gorm.io/gorm"
)

func TestUserRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetUserByUsername", func(t *testing.T) {
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		foundUser, err := repo.GetUserByUsername(user.Username)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if foundUser.ID != user.ID {
			t.Errorf("user.ID = %v, want %v", foundUser.ID, user.ID)
		}
	})

	t.Run("GetUserByUsername not found", func(t *testing.T) {
		repo := NewUserRepository(db)

		_, err := repo.GetUserByUsername("nonexistentuser")
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetUserByID", func(t *testing.T) {
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		foundUser, err := repo.GetUserByID(user.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if foundUser.ID != user.ID {
			t.Errorf("user.ID = %v, want %v", foundUser.ID, user.ID)
		}
	})

	t.Run("GetUserByID not found", func(t *testing.T) {
		repo := NewUserRepository(db)

		_, err := repo.GetUserByID(9999)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("UserExistsByUsername returns true for existing user", func(t *testing.T) {
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		_ = user

		exists := repo.UserExistsByUsername(&authentication.User{Username: "testuser"})
		if !exists {
			t.Error("expected user to exist")
		}
	})

	t.Run("UserExistsByUsername returns false for non-existing user", func(t *testing.T) {
		repo := NewUserRepository(db)

		exists := repo.UserExistsByUsername(&authentication.User{Username: "nonexistentuser"})
		if exists {
			t.Error("expected user not to exist")
		}
	})

	t.Run("UserExistsByMailAddress returns true for existing email", func(t *testing.T) {
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		_ = user

		exists := repo.UserExistsByMailAddress(&authentication.User{MailAddress: "test@example.com"})
		if !exists {
			t.Error("expected user to exist")
		}
	})
}
