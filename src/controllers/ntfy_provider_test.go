package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

func TestNtfyProvider_GetProviderType(t *testing.T) {
	n := &NtfyNotificationProvider{}
	if got := n.GetProviderType(); got != "ntfy" {
		t.Errorf("GetProviderType() = %v, want ntfy", got)
	}
}

func TestNtfyProvider_IsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config configuration.NtfyConfiguration
		want   bool
	}{
		{"both empty", configuration.NtfyConfiguration{}, false},
		{"url set", configuration.NtfyConfiguration{URL: "https://ntfy.sh"}, true},
		{"topic set", configuration.NtfyConfiguration{Topic: "mytopic"}, true},
		{"both set", configuration.NtfyConfiguration{URL: "https://ntfy.sh", Topic: "mytopic"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NtfyNotificationProvider{Configuration: tt.config}
			if got := n.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNtfyProvider_SendStreakMilestone(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		logger := zerolog.Nop()
		n := &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "milestone"},
			Logger:        &logger,
			HTTPClient:    &http.Client{Timeout: 5 * time.Second},
		}
		recipient := &models.NotificationRecipientInfo{}
		err := n.SendStreakMilestone(7, recipient)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
