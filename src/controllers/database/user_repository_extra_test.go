package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestUserRepository_HouseholdLookups(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	t.Run("GetUserHouseholdByID", func(t *testing.T) {
		got, err := repo.GetUserHouseholdByID(user.ID)
		if err != nil || got != household.ID {
			t.Errorf("GetUserHouseholdByID() = %d, %v; want %d", got, err, household.ID)
		}
		if _, err := repo.GetUserHouseholdByID(9999); err == nil {
			t.Error("GetUserHouseholdByID(9999) error = nil, want error")
		}
	})

	t.Run("GetUserHouseholdRole returns the user role", func(t *testing.T) {
		got, err := repo.GetUserHouseholdRole(user.ID)
		if err != nil {
			t.Fatalf("GetUserHouseholdRole() error = %v", err)
		}
		if got != authentication.RoleMember {
			t.Errorf("GetUserHouseholdRole() = %q, want %q", got, authentication.RoleMember)
		}
	})

	t.Run("UpdateUserHouseholdRole only touches matching household", func(t *testing.T) {
		if err := repo.UpdateUserHouseholdRole(user.ID, household.ID, authentication.RoleAdmin); err != nil {
			t.Fatalf("UpdateUserHouseholdRole() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.Role != authentication.RoleAdmin {
			t.Errorf("role = %q, want admin", updated.Role)
		}

		// Wrong household must not change anything.
		if err := repo.UpdateUserHouseholdRole(user.ID, household.ID+100, authentication.RoleMember); err != nil {
			t.Fatalf("UpdateUserHouseholdRole() error = %v", err)
		}
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.Role != authentication.RoleAdmin {
			t.Errorf("role changed despite household mismatch: %q", updated.Role)
		}
	})

	t.Run("GetUsersByHouseholdID and GetHouseholdByID", func(t *testing.T) {
		users, err := repo.GetUsersByHouseholdID(household.ID)
		if err != nil {
			t.Fatalf("GetUsersByHouseholdID() error = %v", err)
		}
		if len(users) != 1 {
			t.Errorf("users = %d, want 1", len(users))
		}

		got, err := repo.GetHouseholdByID(household.ID)
		if err != nil || got.ID != household.ID {
			t.Errorf("GetHouseholdByID() = %v, %v; want household %d", got, err, household.ID)
		}
	})
}

func TestUserRepository_CreateUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	newUser := authentication.User{
		Username:    "newuser",
		Password:    "plaintext-password",
		MailAddress: "new@example.com",
	}
	if err := repo.CreateUser(&newUser); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	var created authentication.User
	if err := db.First(&created, newUser.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if created.Password == "plaintext-password" {
		t.Error("CreateUser() stored plaintext password")
	}
	if bcrypt.CompareHashAndPassword([]byte(created.Password), []byte("plaintext-password")) != nil {
		t.Error("stored password does not verify against plaintext")
	}
	if created.HouseholdID == 0 {
		t.Error("CreateUser() did not assign a household")
	}

	// Household exists, is administered by the new user, and has default locations.
	var household dbModel.Household
	if err := db.First(&household, created.HouseholdID).Error; err != nil {
		t.Fatalf("failed to load household: %v", err)
	}
	if household.AdminID != created.ID {
		t.Errorf("household admin = %d, want %d", household.AdminID, created.ID)
	}

	var locations []dbModel.StorageLocation
	if err := db.Where("household_id = ?", household.ID).Find(&locations).Error; err != nil {
		t.Fatalf("failed to load locations: %v", err)
	}
	if len(locations) != 3 {
		t.Errorf("locations = %d, want 3 defaults", len(locations))
	}

	// Onboarding state was created.
	var state dbModel.OnboardingState
	if err := db.Where("user_id = ?", created.ID).First(&state).Error; err != nil {
		t.Errorf("onboarding state not created: %v", err)
	}
}

func TestUserRepository_UpdateUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)

	t.Run("invalid user id errors", func(t *testing.T) {
		if err := repo.UpdateUser(0, user); err != errors.ErrInvalidUserID {
			t.Errorf("error = %v, want %v", err, errors.ErrInvalidUserID)
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		if err := repo.UpdateUser(9999, user); err != gorm.ErrRecordNotFound {
			t.Errorf("error = %v, want %v", err, gorm.ErrRecordNotFound)
		}
	})

	t.Run("updates profile fields and preferences", func(t *testing.T) {
		user.DisplayName = "New Display"
		user.MailAddress = "updated@example.com"
		user.NotificationPreferences.EmailEnabled = false
		if err := repo.UpdateUser(user.ID, user); err != nil {
			t.Fatalf("UpdateUser() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.DisplayName != "New Display" || updated.MailAddress != "updated@example.com" {
			t.Errorf("profile not updated: %+v", updated)
		}
		if updated.NotificationPreferences.EmailEnabled {
			t.Error("notification preferences not updated")
		}
	})
}

func TestUserRepository_PasswordAndAdminFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)

	t.Run("UpdateUserPassword with matching username", func(t *testing.T) {
		if err := repo.UpdateUserPassword(user.ID, &authentication.Login{Username: user.Username, Password: "brand-new-password"}); err != nil {
			t.Fatalf("UpdateUserPassword() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("brand-new-password")) != nil {
			t.Error("password not updated correctly")
		}
	})

	t.Run("UpdateUserPassword with wrong username errors", func(t *testing.T) {
		err := repo.UpdateUserPassword(user.ID, &authentication.Login{Username: "wrong", Password: "x"})
		if err != errors.ErrMismatchedUsername {
			t.Errorf("error = %v, want %v", err, errors.ErrMismatchedUsername)
		}
	})

	t.Run("UpdateUserPassword with invalid id errors", func(t *testing.T) {
		if err := repo.UpdateUserPassword(0, &authentication.Login{Username: "x", Password: "y"}); err == nil {
			t.Error("UpdateUserPassword(0) error = nil, want error")
		}
	})

	t.Run("UpdateAdminUserFields changes username and mail", func(t *testing.T) {
		if err := repo.UpdateAdminUserFields(user.ID, "renamed", "renamed@example.com"); err != nil {
			t.Fatalf("UpdateAdminUserFields() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.Username != "renamed" || updated.MailAddress != "renamed@example.com" {
			t.Errorf("fields = %q, %q; want renamed", updated.Username, updated.MailAddress)
		}
	})

	t.Run("UpdateUsername and UpdateDisplayName", func(t *testing.T) {
		if err := repo.UpdateUsername(user.ID, "renamed-again"); err != nil {
			t.Fatalf("UpdateUsername() error = %v", err)
		}
		if err := repo.UpdateDisplayName(user.ID, "The Renamed"); err != nil {
			t.Fatalf("UpdateDisplayName() error = %v", err)
		}
		var updated authentication.User
		if err := db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.Username != "renamed-again" || updated.DisplayName != "The Renamed" {
			t.Errorf("username/display = %q/%q, want renamed-again/The Renamed", updated.Username, updated.DisplayName)
		}
	})
}

func TestUserRepository_LoginLockout(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)

	t.Run("account not locked initially", func(t *testing.T) {
		locked, _ := repo.IsAccountLocked(user.ID, 3, 5)
		if locked {
			t.Error("IsAccountLocked() = true, want false")
		}
	})

	t.Run("failed attempts below threshold do not lock", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			if err := repo.RecordFailedLoginAttempt(user.ID, 3, 5); err != nil {
				t.Fatalf("RecordFailedLoginAttempt() error = %v", err)
			}
		}
		locked, _ := repo.IsAccountLocked(user.ID, 3, 5)
		if locked {
			t.Error("IsAccountLocked() = true below threshold, want false")
		}
	})

	t.Run("failed attempts at threshold lock the account", func(t *testing.T) {
		if err := repo.RecordFailedLoginAttempt(user.ID, 3, 5); err != nil {
			t.Fatalf("RecordFailedLoginAttempt() error = %v", err)
		}
		locked, remaining := repo.IsAccountLocked(user.ID, 3, 5)
		if !locked {
			t.Fatal("IsAccountLocked() = false at threshold, want true")
		}
		if remaining <= 0 {
			t.Errorf("remaining = %v, want positive", remaining)
		}
	})

	t.Run("ResetFailedLoginAttempts clears the lockout", func(t *testing.T) {
		if err := repo.ResetFailedLoginAttempts(user.ID); err != nil {
			t.Fatalf("ResetFailedLoginAttempts() error = %v", err)
		}
		locked, _ := repo.IsAccountLocked(user.ID, 3, 5)
		if locked {
			t.Error("IsAccountLocked() = true after reset, want false")
		}
	})
}

func TestUserRepository_EmailVerification(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)
	expires := time.Now().Add(24 * time.Hour)

	if err := repo.CreateEmailVerification(user.ID, "verify-token", expires); err != nil {
		t.Fatalf("CreateEmailVerification() error = %v", err)
	}

	verification, err := repo.GetEmailVerificationByToken("verify-token")
	if err != nil {
		t.Fatalf("GetEmailVerificationByToken() error = %v", err)
	}
	if verification.Status != dbModel.EmailVerificationStatusPending {
		t.Errorf("status = %q, want pending", verification.Status)
	}
	if _, err := repo.GetEmailVerificationByToken("missing"); err == nil {
		t.Error("GetEmailVerificationByToken(missing) error = nil, want error")
	}

	verifiedAt := time.Now()
	if err := repo.UpdateUserEmailVerified(user.ID, verifiedAt); err != nil {
		t.Errorf("UpdateUserEmailVerified() error = %v", err)
	}
	var updated authentication.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.EmailVerifiedAt == nil {
		t.Error("EmailVerifiedAt not set")
	}

	if err := repo.UpdateEmailVerification(user.ID, &verifiedAt); err != nil {
		t.Errorf("UpdateEmailVerification() error = %v", err)
	}
	if err := repo.UpdateEmailVerification(user.ID, nil); err != nil {
		t.Errorf("UpdateEmailVerification(nil) error = %v", err)
	}

	if err := repo.UpdateEmailVerificationStatus("verify-token", dbModel.EmailVerificationStatusVerified); err != nil {
		t.Errorf("UpdateEmailVerificationStatus() error = %v", err)
	}
	updatedVerification, err := repo.GetEmailVerificationByToken("verify-token")
	if err != nil {
		t.Fatalf("GetEmailVerificationByToken() after status update error = %v", err)
	}
	if updatedVerification.Status != dbModel.EmailVerificationStatusVerified {
		t.Errorf("verification status = %q, want confirmed", updatedVerification.Status)
	}
}

func TestUserRepository_OnboardingState(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)

	t.Run("EnsureOnboardingState creates missing state", func(t *testing.T) {
		if err := repo.EnsureOnboardingState(user.ID); err != nil {
			t.Fatalf("EnsureOnboardingState() error = %v", err)
		}
		state, err := repo.GetOnboardingState(user.ID)
		if err != nil {
			t.Fatalf("GetOnboardingState() error = %v", err)
		}
		if state.UserID != user.ID {
			t.Errorf("state.UserID = %d, want %d", state.UserID, user.ID)
		}
	})

	t.Run("EnsureOnboardingState is idempotent", func(t *testing.T) {
		if err := repo.EnsureOnboardingState(user.ID); err != nil {
			t.Fatalf("EnsureOnboardingState() second call error = %v", err)
		}
		var count int64
		if err := db.Model(&dbModel.OnboardingState{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
			t.Fatalf("failed to count onboarding states: %v", err)
		}
		if count != 1 {
			t.Errorf("onboarding states = %d, want 1", count)
		}
	})

	t.Run("Mark* flags flip the state fields", func(t *testing.T) {
		if err := repo.MarkProfileStepDone(user.ID); err != nil {
			t.Errorf("MarkProfileStepDone() error = %v", err)
		}
		if err := repo.MarkHouseholdStepDone(user.ID); err != nil {
			t.Errorf("MarkHouseholdStepDone() error = %v", err)
		}
		if err := repo.MarkOnboardingComplete(user.ID); err != nil {
			t.Errorf("MarkOnboardingComplete() error = %v", err)
		}
		state, err := repo.GetOnboardingState(user.ID)
		if err != nil {
			t.Fatalf("GetOnboardingState() error = %v", err)
		}
		if !state.ProfileStepDone || !state.HouseholdStepDone || !state.OnboardingCompleted {
			t.Errorf("state flags not set: %+v", state)
		}
	})
}

func TestUserRepository_DeleteAndGetByMail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	user := testutil.CreateTestUser(db, 0)

	found, err := repo.GetUserByMailAddress(user.MailAddress)
	if err != nil || found.ID != user.ID {
		t.Errorf("GetUserByMailAddress() = %v, %v; want user %d", found, err, user.ID)
	}
	if _, err := repo.GetUserByMailAddress("nobody@example.com"); err == nil {
		t.Error("GetUserByMailAddress(nobody) error = nil, want error")
	}

	if err := repo.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := repo.GetUserByID(user.ID); err == nil {
		t.Error("GetUserByID after delete: error = nil, want error")
	}
}
