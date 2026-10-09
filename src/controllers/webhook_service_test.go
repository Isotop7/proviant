package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/rs/zerolog"
)

func newTestWebhookService(t *testing.T) (*WebhookService, func()) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()
	svc := &WebhookService{
		DB:         db,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Logger:     &logger,
		Repo:       dbController.NewWebhookRepository(db),
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	return svc, cleanup
}

func seedWebhook(t *testing.T, svc *WebhookService, url, secret, events string, active bool) dbModel.Webhook {
	t.Helper()
	webhook := dbModel.Webhook{UserID: 1, URL: url, Secret: secret, Events: events, Active: active}
	if err := svc.DB.Create(&webhook).Error; err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
	return webhook
}

func waitForDeliveryLogs(t *testing.T, svc *WebhookService, webhookID uint, want int) []dbModel.WebhookDeliveryLog {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		logs, err := svc.Repo.GetDeliveryLogs(webhookID, 0)
		if err != nil {
			t.Fatalf("fetch delivery logs: %v", err)
		}
		if len(logs) >= want {
			return logs
		}
		time.Sleep(20 * time.Millisecond)
	}
	logs, _ := svc.Repo.GetDeliveryLogs(webhookID, 0)
	t.Fatalf("delivery logs = %d, want at least %d", len(logs), want)
	return nil
}

func TestInitAndGetWebhookService(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()
	InitWebhookService(db, &logger)
	// InitWebhookService consumes webhookOnce and stores webhookService
	// pointing at this test DB, which SetupTestDB's cleanup closes. Reset both
	// so later tests do not inherit a service bound to a closed DB.
	t.Cleanup(func() {
		webhookService = nil
		webhookOnce = sync.Once{}
	})
	svc := GetWebhookService()
	if svc == nil {
		t.Fatal("GetWebhookService() = nil after InitWebhookService")
	}
	if svc.Repo == nil || svc.HTTPClient == nil {
		t.Error("webhook service is missing its repository or HTTP client")
	}
}

func TestFireEventNoWebhooks(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	svc.FireEvent("product.expired", map[string]any{"id": 1})
	svc.FireEventContext(context.Background(), "product.expired", map[string]any{"id": 1})
}

func TestFireEventFetchError(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	// Close the pool underneath the service so the lookup fails.
	cleanup()
	svc.FireEvent("product.expired", map[string]any{"id": 1})
	svc.FireEventContext(context.Background(), "product.expired", map[string]any{"id": 1})
}

func TestFireEventDelivers(t *testing.T) {
	delivered := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Proviant-Signature")
		body := make([]byte, 0)
		buf := make([]byte, 1024)
		for {
			n, err := r.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil {
				break
			}
		}
		delivered <- sig + "|" + string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expired"]`, true)
	// An inactive webhook for the same event must not be called.
	inactive := seedWebhook(t, svc, "http://127.0.0.1:1/unreachable", "other", `["product.expired"]`, false)

	payload := map[string]any{"id": 7, "productName": "Milk"}
	svc.FireEvent("product.expired", payload)

	select {
	case got := <-delivered:
		sig, body, _ := strings.Cut(got, "|")
		if len(sig) != len("sha256=")+64 {
			t.Errorf("signature header = %q, want sha256=<64 hex chars>", sig)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Errorf("payload body = %q, not JSON: %v", body, err)
		}
		if decoded["productName"] != "Milk" {
			t.Errorf("payload = %v, want productName Milk", decoded)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was not delivered")
	}

	logs := waitForDeliveryLogs(t, svc, webhook.ID, 1)
	if logs[0].StatusCode != http.StatusOK || logs[0].Attempt != 1 || logs[0].Error != "" {
		t.Errorf("delivery log = %+v, want 200 on attempt 1 without error", logs[0])
	}

	inactiveLogs, err := svc.Repo.GetDeliveryLogs(inactive.ID, 10)
	if err != nil {
		t.Fatalf("fetch inactive delivery logs: %v", err)
	}
	if len(inactiveLogs) != 0 {
		t.Errorf("inactive webhook delivery logs = %d, want 0", len(inactiveLogs))
	}
}

func TestFireEventContextFetchesAndSpawns(t *testing.T) {
	delivered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay the response so the delivery provably completes after
		// FireEventContext has returned. The caller returning must not cancel
		// the in-flight delivery (the old code cancelled the shared context in
		// a deferred call).
		time.Sleep(50 * time.Millisecond)
		delivered <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expiring_soon"]`, true)

	// FireEventContext returns immediately; each spawned delivery gets its own
	// timeout context, so the delivery completes after the function returns.
	svc.FireEventContext(context.Background(), "product.expiring_soon", map[string]any{"id": 3})
	select {
	case <-delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was not delivered after FireEventContext returned")
	}
	// Wait for the delivery log write before cleanup() closes the DB, so the
	// spawned goroutine cannot write after the pool is closed.
	waitForDeliveryLogs(t, svc, webhook.ID, 1)
}

func TestDeliverWebhookContextLive(t *testing.T) {
	delivered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expired"]`, true)

	// A live context takes the default branch and delivers.
	svc.deliverWebhook(context.Background(), &webhook, "product.expired", map[string]any{"id": 1})
	select {
	case <-delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was not delivered")
	}
	logs := waitForDeliveryLogs(t, svc, webhook.ID, 1)
	if logs[0].StatusCode != http.StatusOK {
		t.Errorf("delivery log = %+v, want 200", logs[0])
	}
}

func TestDeliverWebhookContextCancelled(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expired"]`, true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.deliverWebhook(ctx, &webhook, "product.expired", map[string]any{"id": 1})
	// deliverWebhook is synchronous and short-circuits on the cancelled
	// context before issuing any request.
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("endpoint hits = %d, want 0 (cancelled context)", got)
	}
}

func TestDeliverWebhookSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received"))
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expired"]`, true)

	svc.deliverWebhook(context.Background(), &webhook, "product.expired", map[string]any{"id": 1})
	logs := waitForDeliveryLogs(t, svc, webhook.ID, 1)
	if logs[0].StatusCode != http.StatusOK || logs[0].ResponseBody != "received" {
		t.Errorf("delivery log = %+v, want 200 with the response body", logs[0])
	}
}

func TestDeliverWebhookRetriesOnFailure(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, server.URL, "s3cr3t", `["product.expired"]`, true)

	svc.deliverWebhook(context.Background(), &webhook, "product.expired", map[string]any{"id": 1})
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
	logs := waitForDeliveryLogs(t, svc, webhook.ID, 3)
	for i, log := range logs {
		if log.StatusCode != http.StatusInternalServerError {
			t.Errorf("log %d status = %d, want 500", i, log.StatusCode)
		}
	}
}

func TestDeliverWebhookUnreachableRetriesThenGivesUp(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, "http://127.0.0.1:1/down", "s3cr3t", `["product.expired"]`, true)

	svc.deliverWebhook(context.Background(), &webhook, "product.expired", map[string]any{"id": 1})
	logs := waitForDeliveryLogs(t, svc, webhook.ID, 3)
	for i, log := range logs {
		if log.Error == "" {
			t.Errorf("log %d has no error recorded, want the transport error", i)
		}
	}
}

func TestDeliverWebhookMarshalError(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	defer cleanup()
	webhook := seedWebhook(t, svc, "http://127.0.0.1:1/down", "s3cr3t", `["product.expired"]`, true)

	// A func value cannot be marshaled to JSON.
	svc.deliverWebhook(context.Background(), &webhook, "product.expired", map[string]any{"bad": func() {}})
	logs, err := svc.Repo.GetDeliveryLogs(webhook.ID, 0)
	if err != nil {
		t.Fatalf("fetch delivery logs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("delivery logs = %d, want 0 (marshal failure must not log)", len(logs))
	}
}

func TestDoDelivery(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	defer cleanup()

	t.Run("invalid URL errors", func(t *testing.T) {
		if _, _, err := svc.doDelivery(context.Background(), "://bad url", []byte("{}"), "sig"); err == nil {
			t.Error("doDelivery() = nil, want request creation error")
		}
	})

	t.Run("success returns status and body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
			}
			if r.Header.Get("X-Proviant-Signature") != "sha256=sig" {
				t.Errorf("signature header = %q", r.Header.Get("X-Proviant-Signature"))
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("queued"))
		}))
		defer server.Close()
		status, body, err := svc.doDelivery(context.Background(), server.URL, []byte("{}"), "sig")
		if err != nil {
			t.Fatalf("doDelivery() = %v", err)
		}
		if status != http.StatusAccepted || body != "queued" {
			t.Errorf("doDelivery() = %d, %q; want 202, %q", status, body, "queued")
		}
	})
}

func TestWebhookComputeSignature(t *testing.T) {
	svc, cleanup := newTestWebhookService(t)
	defer cleanup()

	payload := []byte(`{"id":1}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(payload)
	want := hex.EncodeToString(mac.Sum(nil))
	if got := svc.computeSignature("secret", payload); got != want {
		t.Errorf("computeSignature() = %q, want %q", got, want)
	}
	if got := svc.computeSignature("other", payload); got == want {
		t.Error("computeSignature() ignores the secret")
	}
}

func TestWebhookTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Errorf("truncateString() = %q, want unchanged", got)
	}
	if got := truncateString("a much longer string", 5); got != "a muc" {
		t.Errorf("truncateString() = %q, want %q", got, "a muc")
	}
}

func TestWebhookParseEvents(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "JSON array", input: `["product.expired","product.expiring_soon"]`, want: []string{"product.expired", "product.expiring_soon"}},
		{name: "JSON array with spaces", input: `[ "a" , "b" ]`, want: []string{"a", "b"}},
		{name: "bare list", input: `a,b,c`, want: []string{"a", "b", "c"}},
		{name: "single quoted", input: `"a"`, want: []string{"a"}},
		{name: "empty string", input: ``, want: nil},
		{name: "empty array", input: `[]`, want: nil},
		{name: "empty entries dropped", input: `a,,b`, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWebhookEvents(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseWebhookEvents(%q) = %q, want %q", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseWebhookEvents(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
