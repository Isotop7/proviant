package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
)

func TestGetWebPushVAPIDPublicKey(t *testing.T) {
	t.Run("returns configured key", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&dbModel.WebPushConfig{PublicKey: "pub-key", PrivateKey: "priv-key"})

		GetWebPushVAPIDPublicKey(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp WebPushVAPIDPublicKeyResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.PublicKey != "pub-key" {
			t.Errorf("publicKey = %q, want pub-key", resp.PublicKey)
		}
	})

	t.Run("generates and stores key when missing", func(t *testing.T) {
		env := setupHandlerTest(t)

		GetWebPushVAPIDPublicKey(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp WebPushVAPIDPublicKeyResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.PublicKey == "" {
			t.Errorf("publicKey empty, want generated key")
		}
		var count int64
		env.DB.Model(&dbModel.WebPushConfig{}).Count(&count)
		if count != 1 {
			t.Errorf("config rows = %d, want 1 (generated)", count)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Notifications.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		appCtx := SetupTestAppContext(ctx, 1)

		GetWebPushVAPIDPublicKey(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestSubscribeWebPushNotifications(t *testing.T) {
	t.Run("saves subscription", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{
			"endpoint": "https://push.example.com/sub/1",
			"keys":     map[string]any{"p256dh": "key", "auth": "secret"},
		})

		SubscribeWebPushNotifications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("load user: %v", err)
		}
		if !stored.NotificationPreferences.WebPushEnabled {
			t.Errorf("webPushEnabled = false, want true")
		}
		if stored.NotificationPreferences.WebPushSubscriptionJSON == "" {
			t.Errorf("subscription JSON empty")
		}
	})

	t.Run("missing keys return 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{"endpoint": "https://push.example.com/sub/1"})

		SubscribeWebPushNotifications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing endpoint returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, map[string]any{
			"keys": map[string]any{"p256dh": "key", "auth": "secret"},
		})

		SubscribeWebPushNotifications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Notifications.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		testutil.CreateTestRequest(ctx, map[string]any{
			"endpoint": "https://push.example.com/sub/1",
			"keys":     map[string]any{"p256dh": "key", "auth": "secret"},
		})
		appCtx := SetupTestAppContext(ctx, 1)

		SubscribeWebPushNotifications(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestUnsubscribeWebPushNotifications(t *testing.T) {
	t.Run("clears subscription", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.User.NotificationPreferences.WebPushEnabled = true
		env.User.NotificationPreferences.WebPushSubscriptionJSON = `{"endpoint":"x"}`
		env.DB.Save(env.User)

		UnsubscribeWebPushNotifications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("load user: %v", err)
		}
		if stored.NotificationPreferences.WebPushEnabled {
			t.Errorf("webPushEnabled = true, want false")
		}
		if stored.NotificationPreferences.WebPushSubscriptionJSON != "" {
			t.Errorf("subscription JSON = %q, want empty", stored.NotificationPreferences.WebPushSubscriptionJSON)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Notifications.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		appCtx := SetupTestAppContext(ctx, 1)

		UnsubscribeWebPushNotifications(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestConstructWebPushSubscriptionJSON(t *testing.T) {
	got := constructWebPushSubscriptionJSON("https://push.example.com/sub", "p256dh-key", "auth-secret")
	want := `{"endpoint":"https://push.example.com/sub","keys":{"p256dh":"p256dh-key","auth":"auth-secret"}}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Errorf("result is not valid JSON: %v", err)
	}
}
