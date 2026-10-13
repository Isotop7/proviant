package v1

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"
)

func TestGenerateCalendarToken(t *testing.T) {
	token, err := generateCalendarToken()
	if err != nil {
		t.Fatalf("generateCalendarToken: %v", err)
	}
	if len(token) != CalendarTokenLength*2 {
		t.Errorf("token length = %d, want %d", len(token), CalendarTokenLength*2)
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Errorf("token is not hex: %v", err)
	}
	other, _ := generateCalendarToken()
	if token == other {
		t.Errorf("tokens must be unique")
	}
}

func TestBuildCalendarURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{"", "/api/v1/calendar/export.ics?token=abc"},
		{"https://proviant.example.com", "https://proviant.example.com/api/v1/calendar/export.ics?token=abc"},
		{"https://proviant.example.com/", "https://proviant.example.com/api/v1/calendar/export.ics?token=abc"},
	}
	for _, tt := range tests {
		if got := buildCalendarURL(tt.base, "abc"); got != tt.want {
			t.Errorf("buildCalendarURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestCalendarConfigHelpers(t *testing.T) {
	env := setupHandlerTest(t)

	t.Run("defaults without config", func(t *testing.T) {
		if got := getCalendarExpiryDays(env.Ctx); got != DefaultCalendarExpiryDays {
			t.Errorf("expiryDays = %d, want %d", got, DefaultCalendarExpiryDays)
		}
		if got := getCalendarExpiringSoonDays(env.Ctx); got != CalendarExpiringSoonDays {
			t.Errorf("expiringSoonDays = %d, want %d", got, CalendarExpiringSoonDays)
		}
	})

	t.Run("custom config values", func(t *testing.T) {
		config := &configuration.ProviantConfiguration{}
		config.Calendar.TokenExpiryDays = 30
		config.Calendar.ExpiringSoonDays = 7
		env.Ctx.Set(util.ContextKeyProviantConfig, config)
		if got := getCalendarExpiryDays(env.Ctx); got != 30 {
			t.Errorf("expiryDays = %d, want 30", got)
		}
		if got := getCalendarExpiringSoonDays(env.Ctx); got != 7 {
			t.Errorf("expiringSoonDays = %d, want 7", got)
		}
	})
}

func TestCreateCalendarToken(t *testing.T) {
	t.Run("creates token with default expiry", func(t *testing.T) {
		env := setupHandlerTest(t)

		CreateCalendarToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp CalendarTokenResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Token) != CalendarTokenLength*2 {
			t.Errorf("token length = %d, want %d", len(resp.Token), CalendarTokenLength*2)
		}
		if !strings.Contains(resp.URL, resp.Token) {
			t.Errorf("url %q does not contain token", resp.URL)
		}
		wantExpiry := time.Now().AddDate(0, 0, DefaultCalendarExpiryDays)
		if resp.ExpiresAt.Before(wantExpiry.Add(-time.Hour)) || resp.ExpiresAt.After(wantExpiry.Add(time.Hour)) {
			t.Errorf("expiresAt = %v, want ~%v", resp.ExpiresAt, wantExpiry)
		}
		var stored authentication.CalendarToken
		if err := env.DB.Where("user_id = ?", env.User.ID).First(&stored).Error; err != nil {
			t.Fatalf("load token: %v", err)
		}
		if stored.Token != resp.Token {
			t.Errorf("stored token = %q, want %q", stored.Token, resp.Token)
		}
	})

	t.Run("uses configured base URL and expiry", func(t *testing.T) {
		env := setupHandlerTest(t)
		config := &configuration.ProviantConfiguration{}
		config.Server.BaseURL = "https://proviant.example.com"
		config.Calendar.TokenExpiryDays = 30
		env.Ctx.Set(util.ContextKeyProviantConfig, config)

		CreateCalendarToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp CalendarTokenResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if !strings.HasPrefix(resp.URL, "https://proviant.example.com/") {
			t.Errorf("url = %q, want base prefix", resp.URL)
		}
		wantExpiry := time.Now().AddDate(0, 0, 30)
		if resp.ExpiresAt.After(wantExpiry.Add(time.Hour)) || resp.ExpiresAt.Before(wantExpiry.Add(-time.Hour)) {
			t.Errorf("expiresAt = %v, want ~%v", resp.ExpiresAt, wantExpiry)
		}
	})

	t.Run("create error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.CalendarTokens.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
		appCtx := SetupTestAppContext(ctx, 1)

		CreateCalendarToken(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestRotateCalendarToken(t *testing.T) {
	env := setupHandlerTest(t)
	env.DB.Create(&authentication.CalendarToken{
		UserID:    env.User.ID,
		Token:     "old-token",
		ExpiresAt: time.Now().AddDate(0, 0, 10),
	})

	RotateCalendarToken(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
	}
	var resp CalendarTokenResponse
	if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "old-token" {
		t.Errorf("token was not rotated")
	}
	var count int64
	env.DB.Model(&authentication.CalendarToken{}).Where("user_id = ?", env.User.ID).Count(&count)
	if count != 1 {
		t.Errorf("token count = %d, want 1 (old invalidated)", count)
	}
	var oldCount int64
	env.DB.Model(&authentication.CalendarToken{}).Where("token = ?", "old-token").Count(&oldCount)
	if oldCount != 0 {
		t.Errorf("old token still present")
	}
}

func TestDeleteCalendarTokenHandler(t *testing.T) {
	t.Run("deletes token", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&authentication.CalendarToken{
			UserID:    env.User.ID,
			Token:     "tok",
			ExpiresAt: time.Now().AddDate(0, 0, 10),
		})

		DeleteCalendarToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&authentication.CalendarToken{}).Where("user_id = ?", env.User.ID).Count(&count)
		if count != 0 {
			t.Errorf("token count = %d, want 0", count)
		}
	})

	t.Run("delete without token still succeeds", func(t *testing.T) {
		env := setupHandlerTest(t)

		DeleteCalendarToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", env.W.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.CalendarTokens.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		appCtx := SetupTestAppContext(ctx, 1)

		DeleteCalendarToken(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestGetCalendarTokenStatusHandler(t *testing.T) {
	t.Run("no token reports hasToken false", func(t *testing.T) {
		env := setupHandlerTest(t)

		GetCalendarTokenStatus(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["hasToken"] != false {
			t.Errorf("response = %v, want hasToken false", resp)
		}
	})

	t.Run("existing token reports status", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&authentication.CalendarToken{
			UserID:    env.User.ID,
			Token:     "tok",
			ExpiresAt: time.Now().AddDate(0, 0, 200),
		})

		GetCalendarTokenStatus(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["hasToken"] != true {
			t.Errorf("response = %v, want hasToken true", resp)
		}
		if url, _ := resp["url"].(string); !strings.Contains(url, "tok") {
			t.Errorf("url = %v, want token", resp["url"])
		}
		if resp["isExpiringSoon"] != false {
			t.Errorf("isExpiringSoon = %v, want false", resp["isExpiringSoon"])
		}
	})

	t.Run("soon-expiring token reports isExpiringSoon", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&authentication.CalendarToken{
			UserID:    env.User.ID,
			Token:     "tok",
			ExpiresAt: time.Now().AddDate(0, 0, 5),
		})

		GetCalendarTokenStatus(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var resp map[string]any
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp["isExpiringSoon"] != true {
			t.Errorf("isExpiringSoon = %v, want true", resp["isExpiringSoon"])
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.CalendarTokens.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		appCtx := SetupTestAppContext(ctx, 1)

		GetCalendarTokenStatus(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}
