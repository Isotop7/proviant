package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNotificationRecipientInfo_OmitsSecretFields pins the json:"-" defense on
// the internal recipient shape, so no future handler can marshal the secrets it
// carries.
func TestNotificationRecipientInfo_OmitsSecretFields(t *testing.T) {
	info := NotificationRecipientInfo{
		NtfyToken: "SECRET-NTFY", TelegramChatID: "chat-1",
		TelegramBotToken: "SECRET-BOT", WebPushSubscriptionJSON: `{"endpoint":"x"}`,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"NtfyToken", "TelegramChatID", "TelegramBotToken", "WebPushSubscriptionJSON",
		"ntfyToken", "telegramChatId", "telegramBotToken", "webPushSubscriptionJSON",
	} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if strings.Contains(string(data), "SECRET-") {
		t.Errorf("payload %s contains a secret value, want none", data)
	}
}
