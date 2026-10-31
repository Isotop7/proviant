package controllers

import (
	"strings"
	"testing"
	"time"

	"github.com/Isotop7/proviant/i18n"
	dbModel "github.com/Isotop7/proviant/models/database"
)

// TestNotificationTemplatesRender guards the shipped notification templates
// against a broken reference: a template that fails to parse would only
// surface when a mail is sent, long after the change that broke it.
//
// The two templates that render domain objects — notification/expiry_mail_digest.html
// and notification/monthly_waste_report.html — are covered by the providers
// that build their data, not here.
func TestNotificationTemplatesRender(t *testing.T) {
	now := time.Now()
	expiry := now.Format("2006-01-02 15:04")

	cases := []struct {
		path     string
		contains string
		data     any
	}{
		{
			path:     "notification/two_factor_enabled.html",
			contains: "authenticator",
			data: struct {
				Username string
				Email    string
			}{Username: "alice", Email: "alice@example.com"},
		},
		{
			path:     "notification/two_factor_removed.html",
			contains: "authenticator",
			data: struct {
				Username string
				Email    string
			}{Username: "alice", Email: "alice@example.com"},
		},
		{
			path:     "notification/verification.html",
			contains: "Proviant",
			data: struct {
				Username  string
				MagicLink string
				ExpiresAt string
				Email     string
			}{Username: "alice", MagicLink: "https://example.com", ExpiresAt: expiry, Email: "alice@example.com"},
		},
		{
			path:     "notification/password_reset.html",
			contains: "Proviant",
			data: struct {
				Username  string
				ResetLink string
				ExpiresAt string
				Email     string
			}{Username: "alice", ResetLink: "https://example.com", ExpiresAt: expiry, Email: "alice@example.com"},
		},
		{
			path:     "notification/invitation.html",
			contains: "Proviant",
			data: struct {
				InviterName   string
				HouseholdName string
				MagicLink     string
				Email         string
			}{InviterName: "bob", HouseholdName: "Shared", MagicLink: "https://example.com", Email: "alice@example.com"},
		},
		{
			path:     "notification/expired.html",
			contains: "Milk",
			data: struct {
				ProductName     string
				Barcode         string
				ID              string
				EffectiveExpiry time.Time
			}{ProductName: "Milk", Barcode: "4006381333931", ID: "7", EffectiveExpiry: now},
		},
		{
			path:     "notification/expired.txt",
			contains: "Milk",
			data: struct {
				ProductName     string
				Barcode         string
				ID              string
				EffectiveExpiry time.Time
			}{ProductName: "Milk", Barcode: "4006381333931", ID: "7", EffectiveExpiry: now},
		},
		{
			path:     "notification/streak_milestone.html",
			contains: "7-day waste-free streak",
			data: struct {
				Milestone int
			}{Milestone: 7},
		},
		{
			path:     "notification/streak_reset.html",
			contains: "Your 5-day streak has ended",
			data: struct {
				PreviousStreak int
			}{PreviousStreak: 5},
		},
		{
			path:     "notification/expiry_mail_digest.html",
			contains: "Proviant",
			data: struct {
				HouseholdName    string
				GeneratedDate    string
				ProductsToday    []dbModel.Product
				ProductsThisWeek []dbModel.Product
				ProductsNextWeek []dbModel.Product
				UnsubscribeURL   string
			}{HouseholdName: "Shared", GeneratedDate: now.Format("January 2, 2006")},
		},
	}

	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			parsed, err := parseNotificationTemplate(tt.path, "en")
			if err != nil {
				t.Fatalf("parseNotificationTemplate() error = %v", err)
			}

			var body strings.Builder
			if err := parsed.Execute(&body, tt.data); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(body.String(), tt.contains) {
				t.Errorf("the rendered mail does not mention %q", tt.contains)
			}
		})
	}
}

// TestTwoFactorNotificationSubjectIsTranslated pins the subject of the
// two-factor mails to a translated string, so a missing catalog key would
// not ship the raw key to the account owner.
func TestTwoFactorNotificationSubjectIsTranslated(t *testing.T) {
	for _, key := range []string{
		"notify.subject.twoFactorEnabled",
		"notify.subject.twoFactorRemoved",
	} {
		subject := i18n.T("en", key)
		if subject == key {
			t.Errorf("the subject key %q is not in the catalog", key)
		}
	}
}
