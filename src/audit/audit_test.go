package audit

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func setupContext(t *testing.T, setRepos bool, withRequest bool) (*gin.Context, *database.RepositoryContainer) {
	t.Helper()
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	if withRequest {
		ctx.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
		ctx.Request.RemoteAddr = "10.0.0.1:1234"
	}
	if setRepos {
		repos := database.NewRepositoryContainer(testutil.SetupTestDB(t), nil)
		ctx.Set(util.ContextKeyRepos, repos)
		return ctx, repos
	}
	return ctx, nil
}

func TestFromGin(t *testing.T) {
	t.Run("extracts all values", func(t *testing.T) {
		ctx, repos := setupContext(t, true, true)
		ctx.Set(util.ContextKeyRequestID, "req-1")

		values := FromGin(ctx)

		if values.Repos != repos {
			t.Errorf("Repos mismatch")
		}
		if values.IPAddress != "10.0.0.1:1234" {
			t.Errorf("IPAddress = %q, want %q", values.IPAddress, "10.0.0.1:1234")
		}
		if values.RequestID != "req-1" {
			t.Errorf("RequestID = %q, want %q", values.RequestID, "req-1")
		}
	})

	t.Run("missing requestID stays empty without panicking", func(t *testing.T) {
		ctx, _ := setupContext(t, true, true)

		values := FromGin(ctx)

		if values.RequestID != "" {
			t.Errorf("RequestID = %q, want empty", values.RequestID)
		}
	})

	t.Run("nil request does not panic", func(t *testing.T) {
		ctx, _ := setupContext(t, true, false)

		values := FromGin(ctx)

		if values.IPAddress != "" {
			t.Errorf("IPAddress = %q, want empty", values.IPAddress)
		}
	})

	t.Run("wrong repos type yields nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Set(util.ContextKeyRepos, "not a container")

		values := FromGin(ctx)

		if values.Repos != nil {
			t.Errorf("Repos = %v, want nil", values.Repos)
		}
	})
}

func TestValuesLog(t *testing.T) {
	t.Run("writes audit log entry", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		repos := database.NewRepositoryContainer(db, nil)

		values := Values{Repos: repos, IPAddress: "10.0.0.9", RequestID: "req-2"}
		values.Log(user.ID, dbModel.AuditActionLoginSuccess, map[string]string{"username": "alice"})

		var logs []dbModel.AuditLog
		if err := db.Find(&logs).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 {
			t.Fatalf("got %d audit logs, want 1", len(logs))
		}
		entry := logs[0]
		if entry.UserID == nil || *entry.UserID != user.ID {
			t.Errorf("UserID = %v, want %d", entry.UserID, user.ID)
		}
		if entry.Action != dbModel.AuditActionLoginSuccess {
			t.Errorf("Action = %q, want %q", entry.Action, dbModel.AuditActionLoginSuccess)
		}
		if entry.IPAddress != "10.0.0.9" {
			t.Errorf("IPAddress = %q, want %q", entry.IPAddress, "10.0.0.9")
		}
		if entry.RequestID != "req-2" {
			t.Errorf("RequestID = %q, want %q", entry.RequestID, "req-2")
		}
		if entry.Details != `{"username":"alice"}` {
			t.Errorf("Details = %q", entry.Details)
		}
	})

	t.Run("quotes in details stay valid JSON", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repos := database.NewRepositoryContainer(db, nil)

		Values{Repos: repos}.Log(0, dbModel.AuditActionLoginFailure, map[string]string{"reason": `bad "password"`})

		var entry dbModel.AuditLog
		if err := db.First(&entry).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Details != `{"reason":"bad \"password\""}` {
			t.Errorf("Details = %q, want valid JSON", entry.Details)
		}
	})

	t.Run("no-op without repos", func(t *testing.T) {
		values := Values{}
		values.Log(1, dbModel.AuditActionLoginFailure, map[string]string{}) // must not panic
	})

	t.Run("unserialisable details logged and entry still written", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repos := database.NewRepositoryContainer(db, nil)
		var buf bytes.Buffer
		logger := zerolog.New(&buf)

		Values{Repos: repos, Logger: &logger}.Log(0, dbModel.AuditActionLoginFailure, make(chan int))

		var entry dbModel.AuditLog
		if err := db.First(&entry).Error; err != nil {
			t.Fatalf("entry not written: %v", err)
		}
		if entry.Details != "" {
			t.Errorf("Details = %q, want empty", entry.Details)
		}
		if !strings.Contains(buf.String(), "failed to marshal details") {
			t.Errorf("expected warning in log output, got %q", buf.String())
		}
	})
}
