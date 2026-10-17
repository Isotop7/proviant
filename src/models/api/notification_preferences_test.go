package api

import (
	"encoding/json"
	"testing"

	"github.com/Isotop7/proviant/models/authentication"
)

// TestNotificationPreferencesStruct_OmitsSecretFields pins the json:"-" defense
// that keeps any handler returning the model safe without per-endpoint editing.
func TestNotificationPreferencesStruct_OmitsSecretFields(t *testing.T) {
	prefs := authentication.NotificationPreferences{
		NtfyToken: "SECRET-NTFY", TelegramBotToken: "SECRET-BOT",
		TelegramChatID: "chat-1", TelegramLinkToken: "link-1",
		TelegramBotUsername: "bot", WebPushSubscriptionJSON: `{"endpoint":"x"}`,
	}

	data, err := json.Marshal(prefs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"ntfyToken", "telegramBotToken", "telegramChatId", "telegramLinkToken",
		"telegramBotUsername", "webPushSubscriptionJSON",
		"NtfyToken", "TelegramBotToken", "TelegramChatID", "TelegramLinkToken",
		"TelegramBotUsername", "WebPushSubscriptionJSON",
	} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
}

// TestUpdateNotificationPreferencesRequest_BindsSecretFields guards the other
// direction of the split: the request shape must still populate the secrets the
// model tags no longer bind.
func TestUpdateNotificationPreferencesRequest_BindsSecretFields(t *testing.T) {
	body := []byte(`{"ntfyToken":"SECRET-NTFY","telegramBotToken":"SECRET-BOT","notificationThresholdDays":3}`)

	var request UpdateNotificationPreferencesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	preferences := request.ToNotificationPreferences()

	if preferences.NtfyToken != "SECRET-NTFY" {
		t.Errorf("NtfyToken = %q, want %q", preferences.NtfyToken, "SECRET-NTFY")
	}
	if preferences.TelegramBotToken != "SECRET-BOT" {
		t.Errorf("TelegramBotToken = %q, want %q", preferences.TelegramBotToken, "SECRET-BOT")
	}
	if preferences.NotificationThresholdDays != 3 {
		t.Errorf("NotificationThresholdDays = %d, want 3", preferences.NotificationThresholdDays)
	}
}
