package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
)

// TestLabelledProductResponse_LabelStartsWithProductName guards the contract
// getSelectedProductName depends on: it splits the option text on ";" and reads
// segment 0 as the product name.
func TestLabelledProductResponse_LabelStartsWithProductName(t *testing.T) {
	product := dbModel.Product{ProductName: "Oat Milk"}

	label := NewLabelledProductResponse(&product).Label

	if !strings.HasPrefix(label, product.ProductName+";") {
		t.Errorf("label = %q, want it to start with %q followed by ';'", label, product.ProductName)
	}

	firstSegment := strings.Split(label, ";")[0]
	if strings.TrimSpace(firstSegment) != product.ProductName {
		t.Errorf("segment 0 = %q, want %q", firstSegment, product.ProductName)
	}
}

// TestLabelledProductResponse_LabelTimeFormat pins the format string that
// proviant.formatDate used, so the dropdown renders identically.
func TestLabelledProductResponse_LabelTimeFormat(t *testing.T) {
	stamp := time.Date(2026, 10, 11, 14, 30, 45, 0, time.UTC)
	product := dbModel.Product{
		ProductName: "Oat Milk",
		ExpireAt:    stamp,
		NotifiedAt:  stamp,
	}
	product.CreatedAt = stamp

	label := NewLabelledProductResponse(&product).Label

	if !strings.Contains(label, "Created: 2026-10-11 14:30:45") {
		t.Errorf("label = %q, want it to contain the created timestamp", label)
	}
	if !strings.Contains(label, "Expire at: 2026-10-11 14:30:45") {
		t.Errorf("label = %q, want it to contain the expiry timestamp", label)
	}
}

// TestProductResponse_OmitsGORMInternals covers what the raw database.Product
// shipped that the DTO must not.
func TestProductResponse_OmitsGORMInternals(t *testing.T) {
	product := dbModel.Product{ProductName: "Oat Milk", Barcode: "4006381333931"}
	product.ID = 7

	data, err := json.Marshal(NewProductResponse(&product))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"CreatedAt", "UpdatedAt", "DeletedAt", "label", "ID"} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if decoded["id"] != float64(7) {
		t.Errorf("id = %v, want 7", decoded["id"])
	}
	if decoded["productName"] != "Oat Milk" {
		t.Errorf("productName = %v, want Oat Milk", decoded["productName"])
	}
}

// TestNewProductResponses_OmitsLabel keeps list responses off the label path.
func TestNewProductResponses_OmitsLabel(t *testing.T) {
	responses := NewProductResponses([]dbModel.Product{{ProductName: "Oat Milk"}})

	if len(responses) != 1 {
		t.Fatalf("len = %d, want 1", len(responses))
	}
	if responses[0].Label != "" {
		t.Errorf("label = %q, want empty", responses[0].Label)
	}
}

// TestNewProductResponses_AcceptsPointerSlice covers the []*dbModel.Product
// repository alongside the []dbModel.Product ones.
func TestNewProductResponses_AcceptsPointerSlice(t *testing.T) {
	product := &dbModel.Product{ProductName: "Oat Milk"}
	product.ID = 11

	responses := NewProductResponses([]*dbModel.Product{product, nil})

	if len(responses) != 2 {
		t.Fatalf("len = %d, want 2", len(responses))
	}
	if responses[0].ID != 11 || responses[0].ProductName != "Oat Milk" {
		t.Errorf("responses[0] = %+v, want id 11 / Oat Milk", responses[0])
	}
	if responses[1].ProductName != "" {
		t.Errorf("responses[1] = %+v, want the zero value for a nil element", responses[1])
	}
}

// TestNewProductResponses_PreservesNilAsNil keeps an empty result encoding as
// null rather than [].
func TestNewProductResponses_PreservesNilAsNil(t *testing.T) {
	data, err := json.Marshal(NewProductResponses[dbModel.Product](nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("nil slice encoded as %s, want null", data)
	}
}

// TestHouseholdUserResponse_OmitsSecretsAndGORMInternals covers the fields the
// raw authentication.User exposed to every household admin.
func TestHouseholdUserResponse_OmitsSecretsAndGORMInternals(t *testing.T) {
	data, err := json.Marshal(HouseholdUserResponse{
		ID: 3, Username: "alice", DisplayName: "Alice", MailAddress: "a@example.com", Role: "admin",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{
		"telegramBotToken", "ntfyToken", "password", "HouseholdID", "Household",
		"NotificationPreferences", "ReceiptScanPreferences", "FailedLoginAttempts",
		"LockedUntil", "ID", "CreatedAt", "UpdatedAt", "DeletedAt",
	} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if decoded["id"] != float64(3) {
		t.Errorf("id = %v, want 3", decoded["id"])
	}
}

// TestNewHouseholdUserResponse_DerivesTelegramBools checks the booleans that
// stand in for the withheld notification secrets.
func TestNewHouseholdUserResponse_DerivesTelegramBools(t *testing.T) {
	user := authentication.User{Username: "alice"}
	user.ID = 3
	user.NotificationPreferences.TelegramBotToken = "SECRET-BOT-TOKEN"

	resp := NewHouseholdUserResponse(&user)

	if !resp.TelegramConfigured {
		t.Error("telegramConfigured = false, want true when a bot token is stored")
	}
	if resp.TelegramLinked {
		t.Error("telegramLinked = true, want false when no chat id is stored")
	}
}

// TestNotificationPreferencesResponse_OmitsSecrets covers the fields the raw
// authentication.NotificationPreferences serialized into every preferences read.
func TestNotificationPreferencesResponse_OmitsSecrets(t *testing.T) {
	prefs := authentication.NotificationPreferences{
		NtfyToken: "SECRET-NTFY", TelegramBotToken: "SECRET-BOT",
		TelegramChatID: "chat-1", TelegramLinkToken: "link-1",
		TelegramBotUsername: "bot", WebPushSubscriptionJSON: `{"endpoint":"x"}`,
	}
	prefs.NotificationThresholdDays = 3

	data, err := json.Marshal(NewNotificationPreferencesResponse(&prefs))
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
	for key, want := range map[string]bool{
		"ntfyTokenConfigured":   true,
		"telegramBotConfigured": true,
		"telegramLinked":        true,
	} {
		if decoded[key] != want {
			t.Errorf("%s = %v, want %v", key, decoded[key], want)
		}
	}
}

// TestStorageLocationResponse_OmitsGORMInternals covers what the raw
// database.StorageLocation shipped.
func TestStorageLocationResponse_OmitsGORMInternals(t *testing.T) {
	location := dbModel.StorageLocation{Name: "Fridge", Icon: "🧊", SortOrder: 2}
	location.ID = 4

	data, err := json.Marshal(NewStorageLocationResponse(&location))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"CreatedAt", "UpdatedAt", "DeletedAt", "householdId", "Household", "ID"} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if decoded["id"] != float64(4) {
		t.Errorf("id = %v, want 4", decoded["id"])
	}
}

// TestAuditLogResponse_OmitsGORMInternals covers what the raw database.AuditLog
// shipped.
func TestAuditLogResponse_OmitsGORMInternals(t *testing.T) {
	log := dbModel.AuditLog{Action: "login_success", IPAddress: "127.0.0.1", Details: "ok", RequestID: "r1"}
	log.ID = 9

	data, err := json.Marshal(NewAuditLogResponse(&log))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"CreatedAt", "UpdatedAt", "DeletedAt"} {
		if _, exists := decoded[key]; exists {
			t.Errorf("key %q present, want it omitted", key)
		}
	}
	if decoded["id"] != float64(9) {
		t.Errorf("id = %v, want 9", decoded["id"])
	}
}
