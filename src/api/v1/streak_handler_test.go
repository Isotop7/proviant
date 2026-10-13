package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
)

func TestGetStreak(t *testing.T) {
	t.Run("creates fresh streak for household", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/streak", nil)

		GetStreak(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.StreakResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CurrentStreak != 0 || resp.LongestStreak != 0 {
			t.Errorf("streak = %+v, want 0/0", resp)
		}
		var count int64
		env.DB.Model(&dbModel.WasteStreak{}).Where("household_id = ?", env.Household.ID).Count(&count)
		if count != 1 {
			t.Errorf("streak rows = %d, want 1 (created)", count)
		}
	})

	t.Run("returns existing streak", func(t *testing.T) {
		env := setupHandlerTest(t)
		lastWasted := time.Now().Add(-48 * time.Hour)
		env.DB.Create(&dbModel.WasteStreak{
			HouseholdID:     env.Household.ID,
			CurrentStreak:   3,
			LongestStreak:   5,
			LastCheckedDate: time.Now(),
			LastWastedDate:  &lastWasted,
		})
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/streak", nil)

		GetStreak(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.StreakResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CurrentStreak != 3 || resp.LongestStreak != 5 {
			t.Errorf("streak = %+v, want 3/5", resp)
		}
	})

	t.Run("user without household returns zero streak", func(t *testing.T) {
		env := setupHandlerTest(t)
		lonely := testutil.CreateTestUser(env.DB, 0)
		env.useMember(lonely)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/streak", nil)

		GetStreak(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.StreakResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.CurrentStreak != 0 || resp.LongestStreak != 0 {
			t.Errorf("streak = %+v, want 0/0", resp)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.HouseholdID = 1
		m.Streaks.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/streak", nil)
		appCtx := SetupTestAppContext(ctx, 1)

		GetStreak(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}
