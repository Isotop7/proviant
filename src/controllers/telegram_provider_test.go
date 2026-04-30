package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
)

func TestTelegramProvider_GetProviderType(t *testing.T) {
	tp := &TelegramNotificationProvider{}
	if got := tp.GetProviderType(); got != "telegram" {
		t.Errorf("GetProviderType() = %v, want telegram", got)
	}
}

func TestTelegramProvider_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		botToken string
		want     bool
	}{
		{"empty token", "", false},
		{"has token", "token123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := &TelegramNotificationProvider{BotToken: tt.botToken}
			if got := tp.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTelegramProvider_SendNotification(t *testing.T) {
	t.Run("invalid recipient type", func(t *testing.T) {
		tp := &TelegramNotificationProvider{HTTPClient: &http.Client{Timeout: 5 * time.Second}}
		product := &dbModel.Product{ProductName: "Test"}
		err := tp.SendNotification(product, 123) // not a string
		if err == nil {
			t.Error("expected error for invalid recipient type")
		}
	})

	t.Run("empty chat ID", func(t *testing.T) {
		tp := &TelegramNotificationProvider{HTTPClient: &http.Client{Timeout: 5 * time.Second}}
		product := &dbModel.Product{ProductName: "Test"}
		err := tp.SendNotification(product, "") // empty chat ID
		if err == nil {
			t.Error("expected error for empty chat ID")
		}
	})
}

func TestTelegramProvider_SendMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// TelegramNotificationProvider uses hardcoded "https://api.telegram.org"
		// To test properly, we'd need to modify the production code
		// For now, skip this test
		t.Skip("Telegram provider uses hardcoded API URL")
	})
}
