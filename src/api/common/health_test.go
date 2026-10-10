package common

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func TestGetHealth(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	GetHealth(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}

	var response api.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response.Message != "ok" {
		t.Errorf("Message = %v, want 'ok'", response.Message)
	}
}

func TestGetHealthResponseFormat(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	GetHealth(ctx)

	if w.Header().Get(util.RequestHeaderContentType) != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %v, want 'application/json; charset=utf-8'", w.Header().Get(util.RequestHeaderContentType))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == nil {
		t.Error("expected 'message' key in response")
	}
	if response["message"] != "ok" {
		t.Errorf("message = %v, want 'ok'", response["message"])
	}
}

// TestGetReadiness asserts the probe reports ready while the database answers.
// It exercises the exported entry point, which shares one package-level cache
// across every request — reset it so this test never reads a stale answer left
// by another test in the file.
func TestGetReadiness(t *testing.T) {
	readinessResults = readinessCache{}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

	logger := zerolog.Nop()
	ctx.Set(util.ContextKeyLogger, &logger)
	ctx.Set(util.ContextKeyDBHandle, testutil.SetupTestDB(t))

	GetReadiness(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestGetReadinessDatabaseDown is the branch that matters to an orchestrator:
// an unreachable database must yield 503, not 200, so traffic is withdrawn
// from the instance instead of being routed into failing requests. Its own
// cache: a fresh answer is required here, and the shared package cache may
// hold "ready" from another test.
func TestGetReadinessDatabaseDown(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	ctx.Set(util.ContextKeyLogger, &logger)

	// A closed connection pool fails Ping, which is what a database outage
	// looks like to the process.
	db := testutil.SetupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if err = sqlDB.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	ctx.Set(util.ContextKeyDBHandle, db)

	getReadiness(ctx, &readinessCache{})

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}

	// api.RespondError echoes err.Error() back to the caller, and the caller is
	// unauthenticated: the body must be exactly the generic error, never the
	// database detail, and that detail must reach the log instead.
	var response api.APIResponse
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response.Message != errors.ErrServiceNotReady.Error() {
		t.Errorf("response body = %q, want only the generic %q", response.Message, errors.ErrServiceNotReady.Error())
	}
	if strings.Contains(w.Body.String(), "sql") || strings.Contains(w.Body.String(), "database") {
		t.Errorf("readiness response leaks internal detail: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "readiness probe database ping failed") {
		t.Errorf("ping failure not logged; logs = %s", logs.String())
	}
}

// TestGetReadinessClientCancelled pins the log level for an abandoned probe.
// Kubernetes' httpGet readinessProbe defaults to timeoutSeconds: 1, which is
// shorter than readinessPingTimeout, so a database slower than a second makes
// kubelet cancel the request. That is the caller giving up, not the database
// failing: logging it at Error would report an outage on a process whose
// database is merely slow.
func TestGetReadinessClientCancelled(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	requestContext, cancel := context.WithCancel(context.Background())
	ctx.Request, _ = http.NewRequestWithContext(requestContext, "GET", "/health/ready", http.NoBody)

	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	ctx.Set(util.ContextKeyLogger, &logger)
	ctx.Set(util.ContextKeyDBHandle, testutil.SetupTestDB(t))

	cancel()
	getReadiness(ctx, &readinessCache{})

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(logs.String(), "readiness probe abandoned by the client") {
		t.Errorf("cancelled probe not logged as client-caused; logs = %s", logs.String())
	}
	if strings.Contains(logs.String(), "database ping failed") {
		t.Errorf("client cancellation reported as a database failure: %s", logs.String())
	}
}

// TestGetReadinessAbsentDBHandle covers the misconfigured-context branch: no
// database handle in the context must be reported as unavailable, never panic
// on a missing key or a failed type assertion.
func TestGetReadinessAbsentDBHandle(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

	logger := zerolog.Nop()
	ctx.Set(util.ContextKeyLogger, &logger)

	GetReadiness(ctx)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
}

// TestGetReadinessNilDBHandle covers a present-but-nil handle: a typed nil
// satisfies the type assertion, so only the explicit nil check catches it.
func TestGetReadinessNilDBHandle(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

	logger := zerolog.Nop()
	ctx.Set(util.ContextKeyLogger, &logger)

	var absent *gorm.DB
	ctx.Set(util.ContextKeyDBHandle, absent)

	GetReadiness(ctx)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
}

// TestGetReadinessServesCachedAnswer pins the point of the cache: a second probe
// inside the TTL must be answered without touching the database. Closing the
// connection pool is the observable — a ping would fail, so a 200 here can only
// have come from the cached answer. Without the cache, a flood of probes is a
// flood of pings against a pool that sets no MaxOpenConns.
func TestGetReadinessServesCachedAnswer(t *testing.T) {
	cache := &readinessCache{}

	database := testutil.SetupTestDB(t)
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}

	probe := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

		logger := zerolog.Nop()
		ctx.Set(util.ContextKeyLogger, &logger)
		ctx.Set(util.ContextKeyDBHandle, database)

		getReadiness(ctx, cache)
		return w
	}

	if w := probe(); w.Code != http.StatusOK {
		t.Fatalf("first probe Status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	if err = sqlDB.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if w := probe(); w.Code != http.StatusOK {
		t.Errorf("probe inside the cache TTL Status = %d, want %d; a closed pool must still be answered from cache",
			w.Code, http.StatusOK)
	}

	// Past the TTL the probe must go back to the database, so a database that
	// dies between probes is still noticed on the next one.
	cache.mutex.Lock()
	cache.checkedAt = time.Now().Add(-readinessCacheTTL)
	cache.mutex.Unlock()

	if w := probe(); w.Code != http.StatusServiceUnavailable {
		t.Errorf("probe past the cache TTL Status = %d, want %d; a stale answer must not outlive the TTL",
			w.Code, http.StatusServiceUnavailable)
	}
}

// TestGetReadinessServesConcurrentFlood is the guard against a self-inflicted
// outage. The endpoint is unauthenticated, so an attacker can flood it; if the
// flood made a probe return 503, the orchestrator would pull a perfectly healthy
// instance out of rotation. Every probe in the burst must be answered 200.
func TestGetReadinessServesConcurrentFlood(t *testing.T) {
	cache := &readinessCache{}
	database := testutil.SetupTestDB(t)

	const flood = 50
	statuses := make([]int, flood)

	var waitGroup sync.WaitGroup
	for i := 0; i < flood; i++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()

			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request, _ = http.NewRequest("GET", "/health/ready", http.NoBody)

			logger := zerolog.Nop()
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, database)

			getReadiness(ctx, cache)
			statuses[index] = w.Code
		}(i)
	}
	waitGroup.Wait()

	for index, status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("probe %d of %d got %d, want %d — a flood must never starve the probe",
				index, flood, status, http.StatusOK)
		}
	}
}
