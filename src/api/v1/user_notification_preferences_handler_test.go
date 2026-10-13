package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
)

// TestNotificationPreferencesRoundTrip pins the bind contract that flipping the
// secret fields to json:"-" would break: request secrets must reach the stored
// preferences.
func TestNotificationPreferencesRoundTrip(t *testing.T) {
	t.Run("secrets in the request body are bound and persisted", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest(`{"ntfyToken":"SECRET-NTFY","telegramBotToken":"SECRET-BOT","notificationThresholdDays":3}`)

		UpdateUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if stored.NotificationPreferences.NtfyToken != "SECRET-NTFY" {
			t.Errorf("NtfyToken = %q, want %q", stored.NotificationPreferences.NtfyToken, "SECRET-NTFY")
		}
		if stored.NotificationPreferences.TelegramBotToken != "SECRET-BOT" {
			t.Errorf("TelegramBotToken = %q, want %q", stored.NotificationPreferences.TelegramBotToken, "SECRET-BOT")
		}
		if stored.NotificationPreferences.NotificationThresholdDays != 3 {
			t.Errorf("NotificationThresholdDays = %d, want 3", stored.NotificationPreferences.NotificationThresholdDays)
		}
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest("{invalid")

		UpdateUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", env.W.Code, http.StatusBadRequest)
		}
	})

	t.Run("negative threshold is rejected", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest(`{"notificationThresholdDays":-1}`)

		UpdateUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", env.W.Code, http.StatusBadRequest)
		}
	})
}

// TestUpdateUserNotificationPreferencesPreservesManagedSecrets guards the merge
// rules: empty secret fields keep their stored value and the web push
// subscription stays under the webpush endpoints' control.
func TestUpdateUserNotificationPreferencesPreservesManagedSecrets(t *testing.T) {
	t.Run("empty secret fields keep the stored values", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.User.NotificationPreferences.NtfyToken = "STORED-NTFY"
		env.User.NotificationPreferences.TelegramBotToken = "STORED-BOT"
		env.User.NotificationPreferences.TelegramChatID = "chat-1"
		env.User.NotificationPreferences.TelegramLinkToken = "link-1"
		env.User.NotificationPreferences.WebPushEnabled = true
		env.User.NotificationPreferences.WebPushSubscriptionJSON = `{"endpoint":"x"}`
		if err := env.DB.Save(env.User).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
		env.Ctx.Request = newRawJSONRequest(`{"ntfyEnabled":true,"ntfyUrl":"https://ntfy.example","ntfyTopic":"home","webPushEnabled":true}`)

		UpdateUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if stored.NotificationPreferences.NtfyToken != "STORED-NTFY" {
			t.Errorf("NtfyToken = %q, want %q", stored.NotificationPreferences.NtfyToken, "STORED-NTFY")
		}
		if stored.NotificationPreferences.TelegramBotToken != "STORED-BOT" {
			t.Errorf("TelegramBotToken = %q, want %q", stored.NotificationPreferences.TelegramBotToken, "STORED-BOT")
		}
		if stored.NotificationPreferences.TelegramChatID != "chat-1" {
			t.Errorf("TelegramChatID = %q, want %q", stored.NotificationPreferences.TelegramChatID, "chat-1")
		}
		if stored.NotificationPreferences.TelegramLinkToken != "link-1" {
			t.Errorf("TelegramLinkToken = %q, want %q", stored.NotificationPreferences.TelegramLinkToken, "link-1")
		}
		if stored.NotificationPreferences.WebPushSubscriptionJSON != `{"endpoint":"x"}` {
			t.Errorf("WebPushSubscriptionJSON = %q, want the stored subscription", stored.NotificationPreferences.WebPushSubscriptionJSON)
		}
		if !stored.NotificationPreferences.WebPushEnabled {
			t.Error("WebPushEnabled = false, want true")
		}
	})

	t.Run("submitted secret fields replace the stored values", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.User.NotificationPreferences.NtfyToken = "STORED-NTFY"
		env.User.NotificationPreferences.TelegramBotToken = "STORED-BOT"
		if err := env.DB.Save(env.User).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
		env.Ctx.Request = newRawJSONRequest(`{"ntfyToken":"NEW-NTFY","telegramBotToken":"NEW-BOT"}`)

		UpdateUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if stored.NotificationPreferences.NtfyToken != "NEW-NTFY" {
			t.Errorf("NtfyToken = %q, want %q", stored.NotificationPreferences.NtfyToken, "NEW-NTFY")
		}
		if stored.NotificationPreferences.TelegramBotToken != "NEW-BOT" {
			t.Errorf("TelegramBotToken = %q, want %q", stored.NotificationPreferences.TelegramBotToken, "NEW-BOT")
		}
	})
}

// TestGetUserNotificationPreferencesRedactsSecrets covers the read shape: no
// secret field may appear in the response, only the configured booleans.
func TestGetUserNotificationPreferencesRedactsSecrets(t *testing.T) {
	t.Run("secrets are reported as configured flags only", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.User.NotificationPreferences.NtfyToken = "SECRET-NTFY"
		env.User.NotificationPreferences.TelegramBotToken = "SECRET-BOT"
		env.User.NotificationPreferences.TelegramChatID = "chat-1"
		if err := env.DB.Save(env.User).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/notification-preferences", nil)

		GetUserNotificationPreferences(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}
		var decoded map[string]any
		if err := json.Unmarshal(env.W.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, key := range []string{"ntfyToken", "telegramBotToken", "telegramChatId", "telegramLinkToken", "telegramBotUsername", "webPushSubscriptionJSON"} {
			if _, exists := decoded[key]; exists {
				t.Errorf("key %q present, want it omitted", key)
			}
		}
		for key, want := range map[string]bool{
			"ntfyTokenConfigured":   true,
			"telegramBotConfigured": true,
			"telegramLinked":        true,
		} {
			if decoded[key] != want {
				t.Errorf("%s = %v, want %v", key, decoded[key], want)
			}
		}
	})
}
