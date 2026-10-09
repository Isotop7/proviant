package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestEscapeICalText(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{`back\slash`, `back\\slash`},
		{"semi;colon", `semi\;colon`},
		{"com,ma", `com\,ma`},
		{"new\nline", `new\nline`},
	}
	for _, tt := range tests {
		if got := escapeICalText(tt.in); got != tt.want {
			t.Errorf("escapeICalText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatICALDateAndTimestamp(t *testing.T) {
	tm := time.Date(2026, 10, 9, 15, 4, 5, 0, time.FixedZone("CEST", 2*60*60))
	if got := formatICALDate(tm); got != "20261009" {
		t.Errorf("formatICALDate = %q, want 20261009", got)
	}
	if got := formatICALTimestamp(tm); got != "20261009T130405Z" {
		t.Errorf("formatICALTimestamp = %q, want 20261009T130405Z", got)
	}
}

func TestFormatProductDescription(t *testing.T) {
	product := dbModel.Product{ProductName: "Milk"}
	if got := formatProductDescription(&product); got != "Milk" {
		t.Errorf("got %q, want Milk", got)
	}

	product.Amount = 2
	product.Unit = "l"
	if got := formatProductDescription(&product); got != "Milk - 2 l" {
		t.Errorf("got %q, want 'Milk - 2 l'", got)
	}

	product.Unit = ""
	if got := formatProductDescription(&product); got != "Milk - 2" {
		t.Errorf("got %q, want 'Milk - 2'", got)
	}

	product.StorageLocation = &dbModel.StorageLocation{Name: "Fridge"}
	if got := formatProductDescription(&product); got != "Milk - 2 @ Fridge" {
		t.Errorf("got %q, want 'Milk - 2 @ Fridge'", got)
	}
}

func TestGenerateVEVENT(t *testing.T) {
	expireAt := time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)
	product := dbModel.Product{
		ProductName: "Milk, fresh; organic",
		Amount:      1,
		Unit:        "l",
		ExpireAt:    expireAt,
	}
	product.ID = 42

	event := generateVEVENT(&product)
	for _, want := range []string{
		"BEGIN:VEVENT",
		"UID:42@proviant",
		"DTSTART;VALUE=DATE:20261224",
		"SUMMARY:Milk\\, fresh\\; organic",
		"DESCRIPTION:Milk\\, fresh\\; organic - 1 l",
		"BEGIN:VALARM",
		"TRIGGER:-P1D",
		"END:VEVENT",
	} {
		if !strings.Contains(event, want) {
			t.Errorf("event missing %q:\n%s", want, event)
		}
	}
	if !strings.Contains(event, "DTSTAMP:") {
		t.Errorf("event missing DTSTAMP:\n%s", event)
	}
}

func setupCalendarExportTest(t *testing.T) (*handlerTestEnv, *authentication.CalendarToken) {
	env := setupHandlerTest(t)
	token := authentication.CalendarToken{
		UserID:    env.User.ID,
		Token:     "test-calendar-token",
		ExpiresAt: time.Now().AddDate(0, 0, 30),
	}
	if err := env.DB.Create(&token).Error; err != nil {
		t.Fatalf("seed calendar token: %v", err)
	}
	return env, &token
}

func TestExportICalendar(t *testing.T) {
	t.Run("missing token returns 401", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics", nil)

		ExportICalendar(env.Ctx)

		if env.W.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", env.W.Code)
		}
	})

	t.Run("unknown token returns 401", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics?token=bogus", nil)

		ExportICalendar(env.Ctx)

		if env.W.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", env.W.Code)
		}
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		env := setupHandlerTest(t)
		expired := authentication.CalendarToken{
			UserID:    env.User.ID,
			Token:     "expired-token",
			ExpiresAt: time.Now().AddDate(0, 0, -1),
		}
		env.DB.Create(&expired)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics?token=expired-token", nil)

		ExportICalendar(env.Ctx)

		if env.W.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", env.W.Code)
		}
	})

	t.Run("valid token returns feed with expiring product", func(t *testing.T) {
		env, token := setupCalendarExportTest(t)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.ProductName = "Soon Gone"
		product.ExpireAt = time.Now().Add(10 * 24 * time.Hour)
		env.DB.Save(product)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics?token="+token.Token, nil)

		ExportICalendar(env.Ctx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		if ct := env.W.Header().Get("Content-Type"); !strings.Contains(ct, "text/calendar") {
			t.Errorf("content-type = %q, want text/calendar", ct)
		}
		body := env.W.Body.String()
		for _, want := range []string{"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VEVENT", "Soon Gone", "END:VCALENDAR"} {
			if !strings.Contains(body, want) {
				t.Errorf("feed missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("valid token without expiring products returns empty feed", func(t *testing.T) {
		env, token := setupCalendarExportTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/calendar/export.ics?token="+token.Token, nil)

		ExportICalendar(env.Ctx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		body := env.W.Body.String()
		if !strings.Contains(body, "BEGIN:VCALENDAR") || strings.Contains(body, "BEGIN:VEVENT") {
			t.Errorf("feed should be an empty calendar:\n%s", body)
		}
	})
}
