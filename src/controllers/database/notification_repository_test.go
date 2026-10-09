package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestNotificationRepository_ThresholdAndPreferences(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)

	t.Run("max threshold days is zero without users", func(t *testing.T) {
		if got := repo.GetMaxNotificationThresholdDays(); got != 0 {
			t.Errorf("GetMaxNotificationThresholdDays() = %d, want 0", got)
		}
	})

	t.Run("max threshold days reflects the highest user setting", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.NotificationPreferences.NotificationThresholdDays = 14
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		if got := repo.GetMaxNotificationThresholdDays(); got != 14 {
			t.Errorf("GetMaxNotificationThresholdDays() = %d, want 14", got)
		}
	})

	t.Run("member mail addresses for known household", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testutil.CreateTestUser(db, household.ID)
		testutil.CreateTestUser(db, household.ID)

		mails, err := repo.GetHouseholdMembersMailAddressesByID(household.ID)
		if err != nil {
			t.Fatalf("GetHouseholdMembersMailAddressesByID() error = %v", err)
		}
		if len(mails) != 2 {
			t.Errorf("mails = %v, want 2 entries", mails)
		}
	})

	t.Run("member mail addresses for unknown household errors", func(t *testing.T) {
		if _, err := repo.GetHouseholdMembersMailAddressesByID(9999); err == nil {
			t.Error("GetHouseholdMembersMailAddressesByID(9999) error = nil, want error")
		}
	})

	t.Run("member notification preferences for known household", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.NotificationPreferences.EmailEnabled = true
		user.NotificationPreferences.TelegramEnabled = true
		user.NotificationPreferences.TelegramChatID = "chat-1"
		user.NotificationPreferences.TelegramBotToken = "bot-1"
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		prefs, err := repo.GetHouseholdMembersNotificationPreferences(household.ID)
		if err != nil {
			t.Fatalf("GetHouseholdMembersNotificationPreferences() error = %v", err)
		}
		if len(prefs) != 1 {
			t.Fatalf("prefs = %v, want 1 entry", prefs)
		}
		if !prefs[0].EmailEnabled || !prefs[0].TelegramEnabled {
			t.Errorf("prefs[0] = %+v, want email and telegram enabled", prefs[0])
		}
		if prefs[0].TelegramChatID != "chat-1" {
			t.Errorf("TelegramChatID = %q, want chat-1", prefs[0].TelegramChatID)
		}
	})

	t.Run("member notification preferences for unknown household errors", func(t *testing.T) {
		if _, err := repo.GetHouseholdMembersNotificationPreferences(9999); err == nil {
			t.Error("GetHouseholdMembersNotificationPreferences(9999) error = nil, want error")
		}
	})
}

func TestNotificationRepository_SetProductNotifiedAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)

	if err := repo.SetProductNotifiedAt(product.ID); err != nil {
		t.Fatalf("SetProductNotifiedAt() error = %v", err)
	}
	var updated dbModel.Product
	if err := db.First(&updated, product.ID).Error; err != nil {
		t.Fatalf("failed to reload product: %v", err)
	}
	if updated.NotifiedAt.IsZero() {
		t.Error("NotifiedAt is zero after SetProductNotifiedAt")
	}

	if err := repo.SetProductNotifiedAt(9999); err == nil {
		t.Error("SetProductNotifiedAt(9999) error = nil, want error")
	}
}

func TestNotificationRepository_InvitationPassthrough(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	inviter := testutil.CreateTestUser(db, household.ID)

	invitation, err := repo.CreateInvitation(household.ID, inviter.ID, "passthrough@example.com")
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	got, err := repo.GetInvitationByToken(invitation.Token)
	if err != nil {
		t.Fatalf("GetInvitationByToken() error = %v", err)
	}
	if got.ID != invitation.ID {
		t.Errorf("GetInvitationByToken() id = %d, want %d", got.ID, invitation.ID)
	}

	invitations, err := repo.GetInvitationsForHousehold(household.ID, inviter.ID)
	if err != nil {
		t.Fatalf("GetInvitationsForHousehold() error = %v", err)
	}
	if len(invitations) != 1 {
		t.Errorf("GetInvitationsForHousehold() = %d invitations, want 1", len(invitations))
	}

	if err := repo.MarkInvitationSendFailed(invitation.ID); err != nil {
		t.Errorf("MarkInvitationSendFailed() error = %v", err)
	}
	if err := repo.MarkInvitationSent(invitation.ID); err != nil {
		t.Errorf("MarkInvitationSent() error = %v", err)
	}

	pending, err := repo.GetPendingInvitationsNotSent(time.Hour)
	if err != nil {
		t.Fatalf("GetPendingInvitationsNotSent() error = %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("GetPendingInvitationsNotSent() = %d, want 0 after mark sent", len(pending))
	}

	if err := repo.CancelInvitation(invitation.ID, inviter.ID); err != nil {
		t.Errorf("CancelInvitation() error = %v", err)
	}
}

func TestNotificationRepository_UserAndHouseholdPassthrough(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	gotUser, err := repo.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if gotUser.ID != user.ID {
		t.Errorf("GetUserByID() id = %d, want %d", gotUser.ID, user.ID)
	}

	gotHousehold, err := repo.GetHouseholdByID(household.ID)
	if err != nil {
		t.Fatalf("GetHouseholdByID() error = %v", err)
	}
	if gotHousehold.ID != household.ID {
		t.Errorf("GetHouseholdByID() id = %d, want %d", gotHousehold.ID, household.ID)
	}

	onboarding := dbModel.OnboardingState{UserID: user.ID}
	if err := db.Create(&onboarding).Error; err != nil {
		t.Fatalf("failed to create onboarding state: %v", err)
	}
	state, err := repo.GetOnboardingState(user.ID)
	if err != nil {
		t.Fatalf("GetOnboardingState() error = %v", err)
	}
	if state.UserID != user.ID {
		t.Errorf("state.UserID = %d, want %d", state.UserID, user.ID)
	}

	if err := repo.MarkNotificationsSetup(user.ID); err != nil {
		t.Errorf("MarkNotificationsSetup() error = %v", err)
	}
	if err := repo.MarkHouseholdStepDone(user.ID); err != nil {
		t.Errorf("MarkHouseholdStepDone() error = %v", err)
	}
	if err := repo.MarkOnboardingComplete(user.ID); err != nil {
		t.Errorf("MarkOnboardingComplete() error = %v", err)
	}
}

func TestNotificationRepository_Telegram(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	if err := repo.SetTelegramLinkToken(user.ID, "link-token"); err != nil {
		t.Fatalf("SetTelegramLinkToken() error = %v", err)
	}
	found, err := repo.FindUserByTelegramLinkToken("link-token")
	if err != nil {
		t.Fatalf("FindUserByTelegramLinkToken() error = %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("FindUserByTelegramLinkToken() id = %d, want %d", found.ID, user.ID)
	}

	if err := repo.SetTelegramChatID(user.ID, "chat-42"); err != nil {
		t.Errorf("SetTelegramChatID() error = %v", err)
	}
	if err := repo.SetTelegramBotUsername(user.ID, "proviant_bot"); err != nil {
		t.Errorf("SetTelegramBotUsername() error = %v", err)
	}

	var updated authentication.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.NotificationPreferences.TelegramChatID != "chat-42" {
		t.Errorf("TelegramChatID = %q, want chat-42", updated.NotificationPreferences.TelegramChatID)
	}
	if updated.NotificationPreferences.TelegramLinkToken != "" {
		t.Errorf("TelegramLinkToken = %q, want empty after successful link", updated.NotificationPreferences.TelegramLinkToken)
	}

	var tokenUsers []authentication.User
	if err := db.Model(&authentication.User{}).Where("id = ?", user.ID).Update("telegram_bot_token", "bot-token").Error; err != nil {
		t.Fatalf("failed to set bot token: %v", err)
	}
	tokenUsers, err = repo.GetAllUsersWithTelegramBotToken()
	if err != nil {
		t.Fatalf("GetAllUsersWithTelegramBotToken() error = %v", err)
	}
	if len(tokenUsers) != 1 || tokenUsers[0].ID != user.ID {
		t.Errorf("GetAllUsersWithTelegramBotToken() = %v, want user %d", tokenUsers, user.ID)
	}
}

func TestNotificationRepository_WebPush(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	if err := repo.SaveWebPushSubscription(user.ID, `{"endpoint":"https://push"}`); err != nil {
		t.Fatalf("SaveWebPushSubscription() error = %v", err)
	}
	var updated authentication.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !updated.NotificationPreferences.WebPushEnabled || updated.NotificationPreferences.WebPushSubscriptionJSON == "" {
		t.Errorf("web push not saved: %+v", updated.NotificationPreferences)
	}

	if err := repo.DeleteWebPushSubscription(user.ID); err != nil {
		t.Errorf("DeleteWebPushSubscription() error = %v", err)
	}
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.NotificationPreferences.WebPushEnabled || updated.NotificationPreferences.WebPushSubscriptionJSON != "" {
		t.Errorf("web push not cleared: %+v", updated.NotificationPreferences)
	}
}

func TestNotificationRepository_VAPIDKeys(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)

	pub, priv, err := repo.GetVAPIDKeys()
	if err != nil {
		t.Fatalf("GetVAPIDKeys() error = %v", err)
	}
	if pub == "" || priv == "" {
		t.Errorf("GetVAPIDKeys() = %q, %q; want non-empty keys", pub, priv)
	}

	// Second call must return the persisted keys, not regenerate.
	pub2, priv2, err := repo.GetVAPIDKeys()
	if err != nil {
		t.Fatalf("GetVAPIDKeys() second call error = %v", err)
	}
	if pub2 != pub || priv2 != priv {
		t.Error("GetVAPIDKeys() generated different keys on second call")
	}
}

func TestNotificationRepository_MailDigest(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	user.NotificationPreferences.EmailEnabled = true
	user.NotificationPreferences.MailDigestFrequency = authentication.MailDigestFrequencyDaily
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	token, err := repo.GenerateMailDigestUnsubscribeToken(user.ID)
	if err != nil {
		t.Fatalf("GenerateMailDigestUnsubscribeToken() error = %v", err)
	}
	if token == "" {
		t.Error("GenerateMailDigestUnsubscribeToken() = empty token")
	}

	found, err := repo.GetUserByMailDigestUnsubscribeToken(token)
	if err != nil {
		t.Fatalf("GetUserByMailDigestUnsubscribeToken() error = %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("GetUserByMailDigestUnsubscribeToken() id = %d, want %d", found.ID, user.ID)
	}

	targets, err := repo.GetHouseholdsWithMailDigestEnabled()
	if err != nil {
		t.Fatalf("GetHouseholdsWithMailDigestEnabled() error = %v", err)
	}
	if len(targets) != 1 || targets[0].HouseholdID != household.ID {
		t.Fatalf("GetHouseholdsWithMailDigestEnabled() = %+v, want household %d", targets, household.ID)
	}
	if len(targets[0].Users) != 1 || targets[0].Users[0].MailDigestToken != token {
		t.Errorf("digest users = %+v, want user with token %q", targets[0].Users, token)
	}

	if err := repo.DeleteMailDigestUnsubscribeToken(token); err != nil {
		t.Errorf("DeleteMailDigestUnsubscribeToken() error = %v", err)
	}
	if _, err := repo.GetUserByMailDigestUnsubscribeToken(token); err == nil {
		t.Error("GetUserByMailDigestUnsubscribeToken() after delete: error = nil, want error")
	}
}

func TestNotificationRepository_HouseholdsWithMonthlyWasteReport(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	user.NotificationPreferences.EmailEnabled = true
	user.NotificationPreferences.MonthlyWasteReportEnabled = true
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	targets, err := repo.GetHouseholdsWithMonthlyWasteReportEnabled()
	if err != nil {
		t.Fatalf("GetHouseholdsWithMonthlyWasteReportEnabled() error = %v", err)
	}
	if len(targets) != 1 || targets[0].HouseholdID != household.ID {
		t.Fatalf("GetHouseholdsWithMonthlyWasteReportEnabled() = %+v, want household %d", targets, household.ID)
	}
	if len(targets[0].Recipients) != 1 || targets[0].Recipients[0] != user.MailAddress {
		t.Errorf("recipients = %v, want [%s]", targets[0].Recipients, user.MailAddress)
	}
	if targets[0].HouseholdName != household.Name {
		t.Errorf("HouseholdName = %q, want %q", targets[0].HouseholdName, household.Name)
	}
}

func TestNotificationRepository_ExpiredAndNotificationPending(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewNotificationRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	// Expired product, never notified — must be picked up.
	expired := testutil.CreateTestProduct(db, household.ID, user.ID)
	expired.ExpireAt = time.Now().Add(-24 * time.Hour)
	if err := db.Save(expired).Error; err != nil {
		t.Fatalf("failed to save product: %v", err)
	}

	// Recently notified product — filtered out by the sleep interval.
	notified := testutil.CreateTestProduct(db, household.ID, user.ID)
	notified.ExpireAt = time.Now().Add(-24 * time.Hour)
	notified.NotifiedAt = time.Now()
	if err := db.Save(notified).Error; err != nil {
		t.Fatalf("failed to save product: %v", err)
	}

	// Far-future product — outside the look-ahead horizon.
	future := testutil.CreateTestProduct(db, household.ID, user.ID)
	future.ExpireAt = time.Now().Add(90 * 24 * time.Hour)
	if err := db.Save(future).Error; err != nil {
		t.Fatalf("failed to save product: %v", err)
	}

	pending, err := repo.GetProductsExpiredAndNotificationPending(23*time.Hour, 7)
	if err != nil {
		t.Fatalf("GetProductsExpiredAndNotificationPending() error = %v", err)
	}
	if len(pending) != 1 || pending[0].ID != expired.ID {
		t.Errorf("pending = %+v, want only product %d", pending, expired.ID)
	}
}
