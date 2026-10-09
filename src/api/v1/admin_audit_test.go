package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// TestGetAuditLogsScoping covers the handler half of the audit-log tenant fix:
// entries are filtered by the household the role middleware stamped into the
// context, and a missing stamp fails closed instead of returning everything.
func TestGetAuditLogsScoping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	householdA := testutil.CreateTestHousehold(db, 0)
	householdB := testutil.CreateTestHousehold(db, 0)
	userA := testutil.CreateTestUser(db, householdA.ID)
	userB := testutil.CreateTestUser(db, householdB.ID)

	repos := database.NewRepositoryContainer(db, nil)
	entryTime := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	for _, userID := range []uint{userA.ID, userB.ID} {
		id := userID
		if err := repos.AuditLogs.Create(context.Background(), &dbModel.AuditLog{
			Timestamp: entryTime,
			UserID:    &id,
			Action:    dbModel.AuditActionLoginSuccess,
			IPAddress: "10.0.0.1",
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	logger := zerolog.Nop()
	newAppCtx := func(userID uint) *AppContext {
		return &AppContext{Logger: &logger, DB: db, Repos: repos, UserID: userID}
	}

	t.Run("returns only the caller's household entries", func(t *testing.T) {
		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log", nil)
		ctx.Set(util.ContextKeyHouseholdID, householdA.ID)

		GetAuditLogs(ctx, newAppCtx(userA.ID))

		if w.Code != http.StatusOK {
			t.Fatalf("Status = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var logs []dbModel.AuditLog
		if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 {
			t.Fatalf("got %d entries, want 1", len(logs))
		}
		if logs[0].HouseholdID == nil || *logs[0].HouseholdID != householdA.ID {
			t.Errorf("entry attributed to %v, want household %d", logs[0].HouseholdID, householdA.ID)
		}
	})

	t.Run("missing household stamp fails closed", func(t *testing.T) {
		ctx, w := repomocks.SetupGinContextWithDB(db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log", nil)

		GetAuditLogs(ctx, newAppCtx(userA.ID))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestGetAuditLogsRejectsMalformedDate keeps a bad ?date= at 400: the
// repository's time.Parse failure is client input, not a server fault.
func TestGetAuditLogsRejectsMalformedDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	repos := database.NewRepositoryContainer(db, nil)
	logger := zerolog.Nop()

	ctx, w := repomocks.SetupGinContextWithDB(db)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?date=not-a-date", nil)
	ctx.Set(util.ContextKeyHouseholdID, household.ID)

	GetAuditLogs(ctx, &AppContext{Logger: &logger, DB: db, Repos: repos, UserID: user.ID})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %v, want %v; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
