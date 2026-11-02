package database

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func testPasswordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}

func TestTOTPRepositoryConsumeRecoveryCodeIsSingleUse(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTOTPRepository(db)

	user := testutil.CreateTestUser(db, 0)
	hash := database.HashTwoFactorRecoveryCode("ABCDEFGHJK")

	if err := repo.CreateRecoveryCodes(user.ID, []string{hash}); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}

	consumed, err := repo.ConsumeRecoveryCode(user.ID, hash, time.Now())
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if !consumed {
		t.Fatal("ConsumeRecoveryCode() did not consume the code")
	}

	consumed, err = repo.ConsumeRecoveryCode(user.ID, hash, time.Now())
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if consumed {
		t.Error("ConsumeRecoveryCode() consumed the same code twice")
	}
}

func TestTOTPRepositoryConsumeRecoveryCodeIgnoresOtherUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTOTPRepository(db)

	owner := testutil.CreateTestUser(db, 0)
	other := testutil.CreateTestUser(db, 0)
	hash := database.HashTwoFactorRecoveryCode("ABCDEFGHJK")

	if err := repo.CreateRecoveryCodes(owner.ID, []string{hash}); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}

	consumed, err := repo.ConsumeRecoveryCode(other.ID, hash, time.Now())
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}
	if consumed {
		t.Error("ConsumeRecoveryCode() consumed a code of another user")
	}
}

func TestTOTPRepositoryCountUnusedRecoveryCodes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTOTPRepository(db)

	user := testutil.CreateTestUser(db, 0)
	hashes := []string{
		database.HashTwoFactorRecoveryCode("AAAAAAAAAA"),
		database.HashTwoFactorRecoveryCode("BBBBBBBBBB"),
	}
	if err := repo.CreateRecoveryCodes(user.ID, hashes); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}

	count, err := repo.CountUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes() error = %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	if _, err := repo.ConsumeRecoveryCode(user.ID, hashes[0], time.Now()); err != nil {
		t.Fatalf("ConsumeRecoveryCode() error = %v", err)
	}

	count, err = repo.CountUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes() error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// A recovery code is a single-use credential: it is removed for good, so no
// hash of an issued code outlives it.
func TestUserRepositoryDisableTwoFactorRemovesTheRecoveryCodes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := NewUserRepository(db)
	totpRepo := NewTOTPRepository(db)

	user := testutil.CreateTestUser(db, 0)
	if err := totpRepo.CreateRecoveryCodes(user.ID, []string{database.HashTwoFactorRecoveryCode("AAAAAAAAAA")}); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}
	if _, err := userRepo.EnableTwoFactor(user.ID, "encrypted"); err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}

	// The codes of a disabled second factor must not survive it.
	if err := userRepo.DisableTwoFactor(user.ID); err != nil {
		t.Fatalf("DisableTwoFactor() error = %v", err)
	}

	codes, err := totpRepo.GetRecoveryCodesByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetRecoveryCodesByUserID() error = %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("len(codes) = %d, want 0", len(codes))
	}

	var remaining int64
	if err := db.Unscoped().Model(&database.TwoFactorRecoveryCode{}).
		Where("user_id = ?", user.ID).
		Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d recovery code rows are still in the table, want 0", remaining)
	}
}

func TestUserRepositoryAdvanceTwoFactorStepRejectsReplay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)

	advanced, err := repo.AdvanceTwoFactorStep(user.ID, 100)
	if err != nil {
		t.Fatalf("AdvanceTwoFactorStep() error = %v", err)
	}
	if !advanced {
		t.Fatal("AdvanceTwoFactorStep() did not store the step")
	}

	advanced, err = repo.AdvanceTwoFactorStep(user.ID, 100)
	if err != nil {
		t.Fatalf("AdvanceTwoFactorStep() error = %v", err)
	}
	if advanced {
		t.Error("AdvanceTwoFactorStep() stored the same step twice")
	}

	advanced, err = repo.AdvanceTwoFactorStep(user.ID, 101)
	if err != nil {
		t.Fatalf("AdvanceTwoFactorStep() error = %v", err)
	}
	if !advanced {
		t.Error("AdvanceTwoFactorStep() did not store a newer step")
	}
}

func TestUserRepositoryVerifyPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)
	user.Password = testPasswordHash(t, "correct-password")
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	valid, err := repo.VerifyPassword(user.ID, "correct-password")
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !valid {
		t.Error("VerifyPassword() rejected the correct password")
	}

	valid, err = repo.VerifyPassword(user.ID, "wrong-password")
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if valid {
		t.Error("VerifyPassword() accepted a wrong password")
	}
}

func TestUserRepositoryEnableTwoFactorOnlyLandsOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)

	activated, err := repo.EnableTwoFactor(user.ID, "encrypted")
	if err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}
	if !activated {
		t.Fatal("the first EnableTwoFactor() did not activate the second factor")
	}

	// A second activation must not land: two concurrent confirmations would
	// otherwise both hand out recovery codes, and the second one would delete
	// the codes the first one already showed.
	activated, err = repo.EnableTwoFactor(user.ID, "other")
	if err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}
	if activated {
		t.Error("the second EnableTwoFactor() activated the second factor again")
	}

	stored, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if stored.TwoFactorSecret != "encrypted" {
		t.Errorf("secret = %q, want encrypted", stored.TwoFactorSecret)
	}
}

func TestTOTPRepositoryReplaceRecoveryCodes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTOTPRepository(db)

	user := testutil.CreateTestUser(db, 0)
	if err := repo.CreateRecoveryCodes(user.ID, []string{database.HashTwoFactorRecoveryCode("AAAAAAAAAA")}); err != nil {
		t.Fatalf("CreateRecoveryCodes() error = %v", err)
	}

	if err := repo.ReplaceRecoveryCodes(user.ID, []string{
		database.HashTwoFactorRecoveryCode("BBBBBBBBBB"),
		database.HashTwoFactorRecoveryCode("CCCCCCCCCC"),
	}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error = %v", err)
	}

	codes, err := repo.GetRecoveryCodesByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetRecoveryCodesByUserID() error = %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("len(codes) = %d, want 2", len(codes))
	}
	for _, code := range codes {
		if code.CodeHash == database.HashTwoFactorRecoveryCode("AAAAAAAAAA") {
			t.Error("the replaced recovery code is still stored")
		}
	}

	// The replaced codes are gone for good, not only hidden.
	var remaining int64
	if err := db.Unscoped().Model(&database.TwoFactorRecoveryCode{}).
		Where("user_id = ?", user.ID).
		Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 2 {
		t.Errorf("%d recovery code rows are in the table, want 2", remaining)
	}
}

// A generic save of the user row must leave the second factor alone. The
// enrolment paths own those columns, and a save that writes them back from a
// stale snapshot would wipe a secret the user just generated.
func TestUpdateUserKeepsTheTwoFactorState(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)
	if _, err := repo.EnableTwoFactor(user.ID, "encrypted"); err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}

	// A save that carries a stale snapshot of the user.
	stale := *user
	stale.DisplayName = "New Name"

	if err := repo.UpdateUser(user.ID, &stale); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	stored, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !stored.TwoFactorEnabled {
		t.Error("the save turned the second factor off")
	}
	if stored.TwoFactorSecret != "encrypted" {
		t.Errorf("secret = %q, want encrypted", stored.TwoFactorSecret)
	}
}

// The same holds for the password change, which writes the whole row.
func TestUpdateUserPasswordKeepsTheTwoFactorState(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)
	if _, err := repo.EnableTwoFactor(user.ID, "encrypted"); err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}

	login := &authentication.Login{Username: user.Username, Password: "a-new-password"}
	if err := repo.UpdateUserPassword(user.ID, login); err != nil {
		t.Fatalf("UpdateUserPassword() error = %v", err)
	}

	stored, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !stored.TwoFactorEnabled {
		t.Error("the password change turned the second factor off")
	}
	if stored.TwoFactorSecret != "encrypted" {
		t.Errorf("secret = %q, want encrypted", stored.TwoFactorSecret)
	}
}

func TestUserRepositoryEnableAndDisableTwoFactor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := testutil.CreateTestUser(db, 0)

	activated, err := repo.EnableTwoFactor(user.ID, "encrypted")
	if err != nil {
		t.Fatalf("EnableTwoFactor() error = %v", err)
	}
	if !activated {
		t.Fatal("EnableTwoFactor() did not activate the second factor")
	}

	stored, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !stored.TwoFactorEnabled {
		t.Error("the second factor was not enabled")
	}
	if stored.TwoFactorSecret != "encrypted" {
		t.Errorf("secret = %q, want encrypted", stored.TwoFactorSecret)
	}

	if err := repo.DisableTwoFactor(user.ID); err != nil {
		t.Fatalf("DisableTwoFactor() error = %v", err)
	}

	stored, err = repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if stored.TwoFactorEnabled {
		t.Error("the second factor was not disabled")
	}
	if stored.TwoFactorSecret != "" {
		t.Errorf("secret = %q, want an empty string", stored.TwoFactorSecret)
	}
}
