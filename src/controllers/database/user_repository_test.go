package database

import (
	"strings"
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

func TestUserRepository_UpdateUserReceiptScanSettings(t *testing.T) {
	t.Run("persists preferences and key together without clobbering other fields", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.DisplayName = "Original Name"
		user.NotificationPreferences = authentication.NotificationPreferences{EmailEnabled: true}
		if err := db.Save(&user).Error; err != nil {
			t.Fatalf("save user: %v", err)
		}

		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
			Model:           "user-model",
			Timeout:         300,
		}
		apiKey := "user-key"
		if err := repo.UpdateUserReceiptScanSettings(user.ID, prefs, &apiKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.ReceiptScanPreferences.OverrideEnabled != prefs.OverrideEnabled ||
			reloaded.ReceiptScanPreferences.Endpoint != prefs.Endpoint ||
			reloaded.ReceiptScanPreferences.Model != prefs.Model ||
			reloaded.ReceiptScanPreferences.Timeout != prefs.Timeout {
			t.Errorf("prefs = %+v, want %+v", reloaded.ReceiptScanPreferences, prefs)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "user-key" {
			t.Errorf("APIKey = %q, want %q", reloaded.ReceiptScanPreferences.APIKey, "user-key")
		}
		if reloaded.DisplayName != "Original Name" {
			t.Errorf("DisplayName = %q, want %q (unchanged)", reloaded.DisplayName, "Original Name")
		}
		if !reloaded.NotificationPreferences.EmailEnabled {
			t.Error("NotificationPreferences clobbered by receipt scan update")
		}
	})

	t.Run("nil apiKey keeps the stored key", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{APIKey: "seeded-key"}
		if err := db.Save(&user).Error; err != nil {
			t.Fatalf("save user: %v", err)
		}

		prefs := authentication.ReceiptScanPreferences{OverrideEnabled: true, Timeout: 300}
		if err := repo.UpdateUserReceiptScanSettings(user.ID, prefs, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "seeded-key" {
			t.Errorf("APIKey = %q, want %q (kept; a keep-the-key save must not replay the column)", reloaded.ReceiptScanPreferences.APIKey, "seeded-key")
		}
	})

	t.Run("empty non-nil apiKey clears the stored key", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		seedKey := "existing-key"
		if err := repo.UpdateUserReceiptScanSettings(user.ID, authentication.ReceiptScanPreferences{}, &seedKey); err != nil {
			t.Fatalf("seed key: %v", err)
		}

		clearKey := ""
		if err := repo.UpdateUserReceiptScanSettings(user.ID, authentication.ReceiptScanPreferences{OverrideEnabled: true}, &clearKey); err != nil {
			t.Fatalf("clear key: %v", err)
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "" {
			t.Errorf("APIKey = %q, want empty (cleared)", reloaded.ReceiptScanPreferences.APIKey)
		}
		if !reloaded.ReceiptScanPreferences.OverrideEnabled {
			t.Error("OverrideEnabled = false, want true (cleared in same call)")
		}
	})

	t.Run("idempotent save of identical values succeeds", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewUserRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		prefs := authentication.ReceiptScanPreferences{OverrideEnabled: true, Timeout: 300}
		if err := repo.UpdateUserReceiptScanSettings(user.ID, prefs, nil); err != nil {
			t.Fatalf("first update: %v", err)
		}
		if err := repo.UpdateUserReceiptScanSettings(user.ID, prefs, nil); err != nil {
			t.Fatalf("second update: %v", err)
		}
	})
}

func TestUserRepository_UpdateUser_DoesNotClobberReceiptScanSettings(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	prefs := authentication.ReceiptScanPreferences{
		OverrideEnabled: true,
		Endpoint:        "https://user.example.com/v1",
		Model:           "user-model",
		Timeout:         300,
	}
	apiKey := "stored-key"
	if err := repo.UpdateUserReceiptScanSettings(user.ID, prefs, &apiKey); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}
	// The key is stored by its own parameter, so the expected stored state is
	// the seeded prefs plus the key.
	storedPrefs := prefs
	storedPrefs.APIKey = "stored-key"

	// Capture the UPDATE statement UpdateUser's full-record save produces, so
	// the test asserts the write set directly instead of racing a second
	// writer against the save (which deadlocks on SQLite's write lock).
	var updateSQL string
	if err := db.Callback().Update().After("gorm:update").Register("test:capture_update_sql", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Model == nil {
			return
		}
		if _, ok := tx.Statement.Model.(*authentication.User); !ok {
			return
		}
		updateSQL = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register hook: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove("test:capture_update_sql")
	})

	// The caller holds a stale snapshot whose receipt scan preferences differ
	// from the stored ones (old code copied them into the save).
	stale, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("load stale user: %v", err)
	}
	stale.DisplayName = "Renamed"
	stale.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
		OverrideEnabled: false,
		Endpoint:        "https://stale.example.com/v1",
		APIKey:          "resurrected-key",
		Timeout:         1,
	}
	if err := repo.UpdateUser(user.ID, &stale); err != nil {
		t.Fatalf("update user: %v", err)
	}

	var reloaded authentication.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.DisplayName != "Renamed" {
		t.Errorf("DisplayName = %q, want %q (regular update must persist)", reloaded.DisplayName, "Renamed")
	}
	if reloaded.ReceiptScanPreferences != storedPrefs {
		t.Errorf("prefs = %+v, want %+v (full-record save must not replay receipt scan columns)", reloaded.ReceiptScanPreferences, storedPrefs)
	}
	if reloaded.ReceiptScanPreferences.APIKey != "stored-key" {
		t.Errorf("APIKey = %q, want %q (stale key replayed by full-record save)", reloaded.ReceiptScanPreferences.APIKey, "stored-key")
	}
	for _, column := range []string{"receipt_scan_override_enabled", "receipt_scan_endpoint", "receipt_scan_model", "receipt_scan_timeout", "receipt_scan_api_key"} {
		if strings.Contains(updateSQL, column) {
			t.Errorf("update SQL writes receipt scan column %q: %s", column, updateSQL)
		}
	}
}
