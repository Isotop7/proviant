package database

import (
	"testing"

	"gorm.io/gorm"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestUserRepository_GetUserByUsername_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
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
}

func TestUserRepository_GetUserByUsername_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetUserByUsername("nonexistentuser")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestUserRepository_GetUserByID_Found(t *testing.T) {
	db := testutil.SetupTestDB(t)
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
}

func TestUserRepository_GetUserByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetUserByID(9999)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestUserRepository_UserExistsByUsername_Exists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	testutil.CreateTestUser(db, household.ID)

	exists := repo.UserExistsByUsername(&authentication.User{Username: "testuser"})
	if !exists {
		t.Error("expected user to exist")
	}
}

func TestUserRepository_UserExistsByUsername_NotExists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	exists := repo.UserExistsByUsername(&authentication.User{Username: "nonexistentuser"})
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestUserRepository_UserExistsByMailAddress_Exists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	testutil.CreateTestUser(db, household.ID)

	exists := repo.UserExistsByMailAddress(&authentication.User{MailAddress: "test@example.com"})
	if !exists {
		t.Error("expected user to exist")
	}
}
