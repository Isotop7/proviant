package controllers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

func TestNtfyProvider_IsConfigured(t *testing.T) {
	tests := []struct {
		name string
		config configuration.NtfyConfiguration
		want  bool
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

func TestNtfyProvider_SendNotification(t *testing.T) {
	t.Skip("template not available in test environment")
}

func TestNtfyProvider_SendStreakMilestone(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		var receivedBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		n := &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "milestone"},
			Logger:        &logger,
			HTTPClient:    &http.Client{Timeout: 5 * time.Second},
		}
		recipient := &models.NotificationRecipientInfo{NtfyToken: "testtoken"}
		err := n.SendStreakMilestone(7, recipient)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(receivedBody, "7-day") {
			t.Errorf("body = %v, want contain 7-day", receivedBody)
		}
	})

	t.Run("non-2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		n := &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "milestone"},
			Logger:        &logger,
			HTTPClient:    &http.Client{Timeout: 5 * time.Second},
		}
		recipient := &models.NotificationRecipientInfo{}
		err := n.SendStreakMilestone(30, recipient)
		if err == nil {
			t.Error("expected error for non-2xx")
		}
	})

	t.Run("transport error", func(t *testing.T) {
		n := &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: "http://unreachable", Topic: "milestone"},
			Logger:        &logger,
			HTTPClient:    &http.Client{Timeout: 5 * time.Second},
		}
		recipient := &models.NotificationRecipientInfo{}
		err := n.SendStreakMilestone(100, recipient)
		if err == nil {
			t.Error("expected transport error")
		}
	})
}
