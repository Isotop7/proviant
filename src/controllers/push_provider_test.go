package controllers

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// stubVAPIDKeyProvider serves fixed VAPID keys (or an error) for webpush tests.
type stubVAPIDKeyProvider struct {
	publicKey  string
	privateKey string
	err        error
}

func (s *stubVAPIDKeyProvider) GetVAPIDKeys() (string, string, error) {
	return s.publicKey, s.privateKey, s.err
}

// newTestWebPushSubscription builds a syntactically valid push subscription
// (real P-256 key pair + auth secret) plus fresh VAPID keys, so
// webpush.SendNotification can encrypt and POST to the test endpoint.
func newTestWebPushSubscription(t *testing.T, endpoint string) (subscriptionJSON, vapidPublicKey, vapidPrivateKey string) {
	t.Helper()
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys: %v", err)
	}
	subKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate subscription key: %v", err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatalf("generate auth secret: %v", err)
	}
	subscription := webpush.Subscription{
		Endpoint: endpoint,
		Keys: webpush.Keys{
			P256dh: base64.StdEncoding.EncodeToString(subKey.PublicKey().Bytes()),
			Auth:   base64.StdEncoding.EncodeToString(auth),
		},
	}
	data, err := json.Marshal(subscription)
	if err != nil {
		t.Fatalf("marshal subscription: %v", err)
	}
	return string(data), publicKey, privateKey
}

func TestWebPushGetProviderType(t *testing.T) {
	provider := &WebPushNotificationProvider{}
	if got := provider.GetProviderType(); got != "webpush" {
		t.Errorf("GetProviderType() = %q, want %q", got, "webpush")
	}
}

func TestWebPushIsConfigured(t *testing.T) {
	provider := &WebPushNotificationProvider{}
	if provider.IsConfigured() {
		t.Error("IsConfigured() = true, want false without VAPID keys")
	}
	provider.VAPIDPublicKey = "pub"
	if provider.IsConfigured() {
		t.Error("IsConfigured() = true, want false with only the public key")
	}
	provider.VAPIDPrivateKey = "priv"
	if !provider.IsConfigured() {
		t.Error("IsConfigured() = false, want true with both keys")
	}
}

func TestWebPushEnsureVAPIDKeys(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("preset keys are kept", func(t *testing.T) {
		provider := &WebPushNotificationProvider{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", Logger: &logger}
		if err := provider.ensureVAPIDKeys(); err != nil {
			t.Errorf("ensureVAPIDKeys() = %v, want nil", err)
		}
		if provider.VAPIDPublicKey != "pub" || provider.VAPIDPrivateKey != "priv" {
			t.Error("ensureVAPIDKeys() overwrote preset keys")
		}
	})

	t.Run("repository error propagates", func(t *testing.T) {
		provider := &WebPushNotificationProvider{
			NotificationRepo: &stubVAPIDKeyProvider{err: errors.New("no keys")},
			Logger:           &logger,
		}
		if err := provider.ensureVAPIDKeys(); err == nil {
			t.Error("ensureVAPIDKeys() = nil, want repository error")
		}
	})

	t.Run("repository keys are loaded", func(t *testing.T) {
		provider := &WebPushNotificationProvider{
			NotificationRepo: &stubVAPIDKeyProvider{publicKey: "pub", privateKey: "priv"},
			Logger:           &logger,
		}
		if err := provider.ensureVAPIDKeys(); err != nil {
			t.Errorf("ensureVAPIDKeys() = %v, want nil", err)
		}
		if provider.VAPIDPublicKey != "pub" || provider.VAPIDPrivateKey != "priv" {
			t.Error("ensureVAPIDKeys() did not load the repository keys")
		}
	})
}

func TestWebPushSendNotification(t *testing.T) {
	logger := zerolog.Nop()
	product := &dbModel.Product{
		Model:       gorm.Model{ID: 1},
		ProductName: "Milk",
		ExpireAt:    time.Now().Add(48 * time.Hour),
	}

	t.Run("VAPID key error propagates", func(t *testing.T) {
		provider := &WebPushNotificationProvider{
			NotificationRepo: &stubVAPIDKeyProvider{err: errors.New("no keys")},
			Logger:           &logger,
		}
		if err := provider.SendNotification(product, "{}"); err == nil {
			t.Error("SendNotification() = nil, want VAPID key error")
		}
	})

	t.Run("invalid recipient type", func(t *testing.T) {
		provider := &WebPushNotificationProvider{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", Logger: &logger}
		if err := provider.SendNotification(product, 42); err == nil {
			t.Error("SendNotification() = nil, want recipient type error")
		}
	})

	t.Run("invalid subscription JSON", func(t *testing.T) {
		provider := &WebPushNotificationProvider{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", Logger: &logger}
		if err := provider.SendNotification(product, "{not json"); err == nil {
			t.Error("SendNotification() = nil, want subscription JSON error")
		}
	})

	t.Run("success posts to the endpoint", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			if r.Header.Get("Authorization") == "" {
				t.Error("VAPID Authorization header missing")
			}
			if r.Header.Get("TTL") == "" {
				t.Error("TTL header missing")
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		subscriptionJSON, publicKey, privateKey := newTestWebPushSubscription(t, server.URL)
		provider := &WebPushNotificationProvider{
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			Logger:          &logger,
		}
		if err := provider.SendNotification(product, subscriptionJSON); err != nil {
			t.Errorf("SendNotification() = %v, want nil", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("endpoint hits = %d, want 1", got)
		}
	})

	t.Run("endpoint error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGone)
		}))
		defer server.Close()

		subscriptionJSON, publicKey, privateKey := newTestWebPushSubscription(t, server.URL)
		provider := &WebPushNotificationProvider{
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			Logger:          &logger,
		}
		if err := provider.SendNotification(product, subscriptionJSON); err == nil {
			t.Error("SendNotification() = nil, want status error")
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		subscriptionJSON, publicKey, privateKey := newTestWebPushSubscription(t, "http://127.0.0.1:1/push")
		provider := &WebPushNotificationProvider{
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			Logger:          &logger,
		}
		if err := provider.SendNotification(product, subscriptionJSON); err == nil {
			t.Error("SendNotification() = nil, want transport error")
		}
	})
}

func TestFormatWebPushExpiryDays(t *testing.T) {
	if got := formatWebPushExpiryDays("in 3 days"); got != "in 3 days" {
		t.Errorf("formatWebPushExpiryDays(string) = %q, want unchanged", got)
	}
	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{name: "three days out", input: time.Now().Add(72 * time.Hour), want: "expires in 3 days"},
		{name: "tomorrow", input: time.Now().Add(24 * time.Hour), want: "expires tomorrow"},
		{name: "today", input: time.Now().Add(time.Hour), want: "expires today"},
		{name: "past expiry", input: time.Now().Add(-48 * time.Hour), want: "expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatWebPushExpiryDays(tt.input); got != tt.want {
				t.Errorf("formatWebPushExpiryDays(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
	// The product expiry is a time.Time and formats as a real day count.
	product := &dbModel.Product{ExpireAt: time.Now().Add(72 * time.Hour)}
	if got := formatWebPushExpiryDays(product.ExpireAt); got != "expires in 3 days" {
		t.Errorf("formatWebPushExpiryDays(product.ExpireAt) = %q, want %q", got, "expires in 3 days")
	}
	// Unsupported inputs keep the vague fallback.
	if got := formatWebPushExpiryDays(42); got != "expires soon" {
		t.Errorf("formatWebPushExpiryDays(int) = %q, want %q", got, "expires soon")
	}
}

// compile-time check that the stub satisfies the key provider.
var _ WebPushKeyProvider = (*stubVAPIDKeyProvider)(nil)
