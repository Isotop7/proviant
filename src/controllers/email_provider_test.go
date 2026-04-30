package controllers

import (
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
	gomail "gopkg.in/mail.v2"
)

func TestEmailProvider_IsConfigured(t *testing.T) {
	tests := []struct {
		name string
		config configuration.SMTPConfiguration
		want  bool
	}{
		{"empty host", configuration.SMTPConfiguration{}, false},
		{"host set port zero", configuration.SMTPConfiguration{Host: "smtp.example.com"}, false},
		{"both set", configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EmailNotificationProvider{Configuration: tt.config}
			if got := e.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmailProvider_SendNotification(t *testing.T) {
	logger := zerolog.Nop()
	e := &EmailNotificationProvider{
		Configuration: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "noreply@example.com",
		},
		Logger: &logger,
	}

	product := &dbModel.Product{
		ProductName: "Test Product",
		Barcode:     "1234567890123",
		ExpireAt:    time.Now().Add(24 * time.Hour),
	}

	t.Run("invalid recipient type", func(t *testing.T) {
		err := e.SendNotification(product, 123)
		if err == nil {
			t.Error("expected error for invalid recipient type")
		}
	})

	t.Run("success", func(t *testing.T) {
		// Override emailSendFunc to capture message
		var capturedMsg *gomail.Message
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			capturedMsg = m
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		err := e.SendNotification(product, "test@example.com")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedMsg == nil {
			t.Fatal("expected message captured")
		}
		if !strings.Contains(capturedMsg.GetHeader("Subject")[0], product.ProductName) {
			t.Error("subject should contain product name")
		}
	})
}

func TestEmailProvider_SendMonthlyWasteReport(t *testing.T) {
	e := &EmailNotificationProvider{
		Configuration: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "noreply@example.com",
		},
	}
	stats := &models.WasteStats{
		HouseholdName: "Test Household",
		MonthLabel:    "2026-04",
	}
	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()
		err := e.SendMonthlyWasteReport("test@example.com", stats)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEmailProvider_SendStreakMilestone(t *testing.T) {
	e := &EmailNotificationProvider{
		Configuration: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "noreply@example.com",
		},
	}
	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()
		err := e.SendStreakMilestone(7, "test@example.com")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEmailProvider_SendEmailVerificationEmail(t *testing.T) {
	e := &EmailNotificationProvider{
		Configuration: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "noreply@example.com",
		},
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()
		err := e.SendEmailVerificationEmail("test@example.com", "user1", "token123", "http://example.com", expiresAt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEmailProvider_SendInvitationEmail(t *testing.T) {
	e := &EmailNotificationProvider{
		Configuration: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			FromAddress: "noreply@example.com",
		},
	}
	invitation := &dbModel.HouseholdInvitation{
		Email:     "invitee@example.com",
		Token:     "inv-token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()
		err := e.SendInvitationEmail(invitation, "inviter", "Household", "http://example.com")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
