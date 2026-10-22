package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Isotop7/proviant/models/authentication"
)

// TestDashboardLayoutRoundTrip pins the storage contract of the dashboard
// arrangement: what the user stores is what the user reads back.
func TestDashboardLayoutRoundTrip(t *testing.T) {
	t.Run("a stored layout is read back unchanged", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest(`{"version":1,"tiles":{"hero":{"order":0,"w":3,"h":2},"chart-waste":{"order":5,"w":6,"h":3}}}`)

		UpdateUserDashboardLayout(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}

		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if stored.DashboardLayout == "" {
			t.Fatal("DashboardLayout is empty, want the stored layout")
		}

		ctx, w := env.freshCtx(env.User.ID)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/dashboard-layout", nil)

		GetUserDashboardLayout(ctx, env.AppCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var decoded dashboardLayout
		if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Version != dashboardLayoutVersion {
			t.Errorf("version = %d, want %d", decoded.Version, dashboardLayoutVersion)
		}
		hero, exists := decoded.Tiles["hero"]
		if !exists {
			t.Fatal("tile hero is missing from the stored layout")
		}
		if hero.Order != 0 || hero.W != 3 || hero.H != 2 {
			t.Errorf("hero = %+v, want order 0, w 3, h 2", hero)
		}
		waste, exists := decoded.Tiles["chart-waste"]
		if !exists {
			t.Fatal("tile chart-waste is missing from the stored layout")
		}
		if waste.Order != 5 || waste.W != 6 || waste.H != 3 {
			t.Errorf("chart-waste = %+v, want order 5, w 6, h 3", waste)
		}
	})

	t.Run("a user without a layout gets the default arrangement", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/dashboard-layout", nil)

		GetUserDashboardLayout(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
		}
		var decoded dashboardLayout
		if err := json.Unmarshal(env.W.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Version != dashboardLayoutVersion {
			t.Errorf("version = %d, want %d", decoded.Version, dashboardLayoutVersion)
		}
		if len(decoded.Tiles) != 0 {
			t.Errorf("tiles = %v, want none", decoded.Tiles)
		}
	})

	t.Run("an empty tile map resets the arrangement", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = newRawJSONRequest(`{"version":1,"tiles":{"hero":{"order":0,"w":3,"h":2}}}`)
		UpdateUserDashboardLayout(env.Ctx, env.AppCtx)

		ctx, w := env.freshCtx(env.User.ID)
		ctx.Request = newRawJSONRequest(`{"version":1,"tiles":{}}`)

		UpdateUserDashboardLayout(ctx, env.AppCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		var decoded dashboardLayout
		if err := json.Unmarshal([]byte(stored.DashboardLayout), &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(decoded.Tiles) != 0 {
			t.Errorf("tiles = %v, want none", decoded.Tiles)
		}
	})
}

// TestDashboardLayoutIsolation guards the ownership of the arrangement: a
// layout belongs to one user and no other.
func TestDashboardLayoutIsolation(t *testing.T) {
	env := setupHandlerTest(t)
	member := env.addMember(t, "layout-member")

	env.Ctx.Request = newRawJSONRequest(`{"version":1,"tiles":{"hero":{"order":0,"w":3,"h":2}}}`)
	UpdateUserDashboardLayout(env.Ctx, env.AppCtx)
	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", env.W.Code, http.StatusOK)
	}

	ctx, w := env.freshCtx(member.ID)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/dashboard-layout", nil)

	GetUserDashboardLayout(ctx, SetupTestAppContext(ctx, member.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var decoded dashboardLayout
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Tiles) != 0 {
		t.Errorf("tiles = %v, want none for the second user", decoded.Tiles)
	}
}

// TestDashboardLayoutRejectsInvalidPayloads covers the validation the
// frontend could not have produced.
func TestDashboardLayoutRejectsInvalidPayloads(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed body", body: "{invalid"},
		{name: "unknown version", body: `{"version":99,"tiles":{}}`},
		{name: "column span above the grid", body: `{"version":1,"tiles":{"hero":{"order":0,"w":7,"h":1}}}`},
		{name: "zero column span", body: `{"version":1,"tiles":{"hero":{"order":0,"w":0,"h":1}}}`},
		{name: "row span above the limit", body: `{"version":1,"tiles":{"hero":{"order":0,"w":1,"h":99}}}`},
		{name: "negative order", body: `{"version":1,"tiles":{"hero":{"order":-1,"w":1,"h":1}}}`},
		{name: "tile id with a forbidden character", body: `{"version":1,"tiles":{"hero hero":{"order":0,"w":1,"h":1}}}`},
		{name: "body above the size limit", body: `{"version":1,"tiles":{"hero":{"order":0,"w":1,"h":1},"pad":"` + strings.Repeat("x", dashboardLayoutMaxBytes) + `"}}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := setupHandlerTest(t)
			env.Ctx.Request = newRawJSONRequest(testCase.body)

			UpdateUserDashboardLayout(env.Ctx, env.AppCtx)

			if env.W.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", env.W.Code, http.StatusBadRequest)
			}
			var stored authentication.User
			if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
				t.Fatalf("reload user: %v", err)
			}
			if stored.DashboardLayout != "" {
				t.Errorf("DashboardLayout = %q, want it left untouched", stored.DashboardLayout)
			}
		})
	}
}

// TestDashboardLayoutSurvivesAGenericUserUpdate guards the ownership of the
// column: a save of another user field must not drop a stored arrangement.
func TestDashboardLayoutSurvivesAGenericUserUpdate(t *testing.T) {
	env := setupHandlerTest(t)
	env.Ctx.Request = newRawJSONRequest(`{"version":1,"tiles":{"hero":{"order":0,"w":3,"h":2}}}`)
	UpdateUserDashboardLayout(env.Ctx, env.AppCtx)

	user, getErr := env.AppCtx.Repos.Users.GetUserByID(env.User.ID)
	if getErr != nil {
		t.Fatalf("get user: %v", getErr)
	}
	user.DisplayName = "Renamed"
	if updateErr := env.AppCtx.Repos.Users.UpdateUser(user.ID, &user); updateErr != nil {
		t.Fatalf("update user: %v", updateErr)
	}

	var stored authentication.User
	if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.DashboardLayout == "" {
		t.Error("DashboardLayout is empty, want the stored layout to survive")
	}
	if stored.DisplayName != "Renamed" {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, "Renamed")
	}
}

// TestDashboardLayoutRejectsTooManyTiles keeps a single request from filling
// the column with entries that are not tiles.
func TestDashboardLayoutRejectsTooManyTiles(t *testing.T) {
	env := setupHandlerTest(t)
	tiles := make([]string, 0, dashboardLayoutMaxTiles+1)
	for i := 0; i <= dashboardLayoutMaxTiles; i++ {
		tiles = append(tiles, fmt.Sprintf(`"tile-%d":{"order":%d,"w":1,"h":1}`, i, i))
	}
	body := `{"version":1,"tiles":{` + strings.Join(tiles, ",") + `}}`
	env.Ctx.Request = newRawJSONRequest(body)

	UpdateUserDashboardLayout(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", env.W.Code, http.StatusBadRequest)
	}
}
