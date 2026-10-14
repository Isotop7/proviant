package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// TestHouseholdIntegrationsCRUD verifies household admin authorization, write-only token semantics, and provider enablement gates.
func TestHouseholdIntegrationsCRUD(t *testing.T) {
	env := setupHandlerTest(t)

	t.Run("create integration stores token without echoing it in response", func(t *testing.T) {
		ctx, w := env.freshCtx(env.User.ID)
		appCtx := SetupTestAppContext(ctx, env.User.ID)
		ctx.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderTodoist}}
		ctx.Request = newRawJSONRequest(`{"enabled":true,"accessToken":"SECRET-TODOIST-TOKEN","listId":"proj-42","listName":"Groceries"}`)

		CreateHouseholdIntegration(ctx, appCtx)
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "SECRET-TODOIST-TOKEN") {
			t.Fatalf("response leaked secret token: %s", w.Body.String())
		}

		var resp apiModel.HouseholdIntegrationResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !resp.Enabled || !resp.AccessTokenConfigured || resp.ListID != "proj-42" {
			t.Errorf("unexpected response: %+v", resp)
		}

		stored, err := env.AppCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(env.Household.ID, util.ShoppingListProviderTodoist)
		if err != nil {
			t.Fatalf("GetByHouseholdAndProvider: %v", err)
		}
		if stored.AccessToken != "SECRET-TODOIST-TOKEN" {
			t.Errorf("stored.AccessToken = %q, want SECRET-TODOIST-TOKEN", stored.AccessToken)
		}
	})

	t.Run("patch without token preserves stored token and clearAccessToken wipes it", func(t *testing.T) {
		ctx, w := env.freshCtx(env.User.ID)
		appCtx := SetupTestAppContext(ctx, env.User.ID)
		ctx.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderTodoist}}
		ctx.Request = httptest.NewRequest(http.MethodPatch, "/x", bytesReaderString(`{"listName":"Weekly Groceries"}`))
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateHouseholdIntegration(ctx, appCtx)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		stored, _ := env.AppCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(env.Household.ID, util.ShoppingListProviderTodoist)
		if stored.AccessToken != "SECRET-TODOIST-TOKEN" || stored.ListName != "Weekly Groceries" {
			t.Errorf("stored = %+v, want token preserved and listName updated", stored)
		}

		ctxClear, wClear := env.freshCtx(env.User.ID)
		appCtxClear := SetupTestAppContext(ctxClear, env.User.ID)
		ctxClear.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderTodoist}}
		ctxClear.Request = httptest.NewRequest(http.MethodPatch, "/x", bytesReaderString(`{"clearAccessToken":true}`))
		ctxClear.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		UpdateHouseholdIntegration(ctxClear, appCtxClear)
		if wClear.Code != http.StatusOK {
			t.Fatalf("clear status = %d, want 200", wClear.Code)
		}

		storedAfterClear, _ := env.AppCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(env.Household.ID, util.ShoppingListProviderTodoist)
		if storedAfterClear.AccessToken != "" {
			t.Errorf("AccessToken after clear = %q, want empty", storedAfterClear.AccessToken)
		}
	})

	t.Run("list integrations omits secrets", func(t *testing.T) {
		ctx, w := env.freshCtx(env.User.ID)
		appCtx := SetupTestAppContext(ctx, env.User.ID)

		ListHouseholdIntegrations(ctx, appCtx)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var list []apiModel.HouseholdIntegrationResponse
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(list) != 1 || list[0].Provider != util.ShoppingListProviderTodoist {
			t.Errorf("list = %+v, want 1 todoist entry", list)
		}
	})

	t.Run("non-admin member is rejected with 403", func(t *testing.T) {
		member := env.addMember(t, "regular-member")
		ctx, w := env.freshCtx(member.ID)
		appCtx := SetupTestAppContext(ctx, member.ID)

		ListHouseholdIntegrations(ctx, appCtx)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("unknown provider and instance-disabled provider are rejected with 400", func(t *testing.T) {
		ctxInvalid, wInvalid := env.freshCtx(env.User.ID)
		appCtxInvalid := SetupTestAppContext(ctxInvalid, env.User.ID)
		ctxInvalid.Params = gin.Params{{Key: "provider", Value: "unknown"}}
		ctxInvalid.Request = newRawJSONRequest(`{"enabled":true}`)

		CreateHouseholdIntegration(ctxInvalid, appCtxInvalid)
		if wInvalid.Code != http.StatusBadRequest {
			t.Errorf("invalid provider status = %d, want 400", wInvalid.Code)
		}

		ctxDisabled, wDisabled := env.freshCtx(env.User.ID)
		ctxDisabled.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{
			ShoppingListSync: configuration.ShoppingListSyncConfiguration{
				Enabled:        true,
				BringEnabled:   false,
				TodoistEnabled: true,
			},
		})
		appCtxDisabled := SetupTestAppContext(ctxDisabled, env.User.ID)
		ctxDisabled.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderBring}}
		ctxDisabled.Request = newRawJSONRequest(`{"enabled":true}`)

		CreateHouseholdIntegration(ctxDisabled, appCtxDisabled)
		if wDisabled.Code != http.StatusBadRequest {
			t.Errorf("disabled provider status = %d, want 400", wDisabled.Code)
		}
	})

	t.Run("delete removes integration and returns 404 when already deleted", func(t *testing.T) {
		ctx, w := env.freshCtx(env.User.ID)
		appCtx := SetupTestAppContext(ctx, env.User.ID)
		ctx.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderTodoist}}

		DeleteHouseholdIntegration(ctx, appCtx)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		ctxAgain, wAgain := env.freshCtx(env.User.ID)
		appCtxAgain := SetupTestAppContext(ctxAgain, env.User.ID)
		ctxAgain.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderTodoist}}

		DeleteHouseholdIntegration(ctxAgain, appCtxAgain)
		if wAgain.Code != http.StatusNotFound {
			t.Errorf("second delete status = %d, want 404", wAgain.Code)
		}
	})
}

// TestHouseholdIntegrationTestAndSyncNow verifies test-connection and manual sync endpoints using the no-op provider.
func TestHouseholdIntegrationTestAndSyncNow(t *testing.T) {
	env := setupHandlerTest(t)

	integration := &dbModel.HouseholdIntegration{
		HouseholdID: env.Household.ID,
		Provider:    util.ShoppingListProviderNoop,
		Enabled:     true,
		AccessToken: "SECRET-NOOP-TOKEN",
		ListID:      "list-1",
	}
	if err := env.AppCtx.Repos.HouseholdIntegrations.Create(integration); err != nil {
		t.Fatalf("create integration: %v", err)
	}

	item := &dbModel.ShoppingListItem{
		HouseholdID: env.Household.ID,
		Name:        "Butter",
		Quantity:    1,
		CreatedBy:   env.User.ID,
	}
	if err := env.AppCtx.Repos.ShoppingListItems.Create(item); err != nil {
		t.Fatalf("create shopping list item: %v", err)
	}

	noop := &controllers.NoopShoppingListSyncProvider{
		CheckedState: map[string]bool{
			"noop-1": true,
		},
	}

	t.Run("test connection succeeds and sync now reconciles checked state", func(t *testing.T) {
		ctxTest, wTest := env.freshCtx(env.User.ID)
		appCtxTest := SetupTestAppContext(ctxTest, env.User.ID)
		appCtxTest.ShoppingListSync.SetProviderFactory(func(_ string, _ *dbModel.HouseholdIntegration, _ *zerolog.Logger, _ *http.Client) (controllers.ShoppingListSyncProvider, error) {
			return noop, nil
		})
		ctxTest.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderNoop}}

		TestHouseholdIntegration(ctxTest, appCtxTest)
		if wTest.Code != http.StatusOK {
			t.Fatalf("test status = %d, want 200", wTest.Code)
		}

		ctxSync, wSync := env.freshCtx(env.User.ID)
		appCtxSync := SetupTestAppContext(ctxSync, env.User.ID)
		appCtxSync.ShoppingListSync.SetProviderFactory(func(_ string, _ *dbModel.HouseholdIntegration, _ *zerolog.Logger, _ *http.Client) (controllers.ShoppingListSyncProvider, error) {
			return noop, nil
		})
		ctxSync.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderNoop}}

		SyncHouseholdIntegrationNow(ctxSync, appCtxSync)
		if wSync.Code != http.StatusOK {
			t.Fatalf("sync status = %d, want 200 (body: %s)", wSync.Code, wSync.Body.String())
		}

		updatedItem, err := env.AppCtx.Repos.ShoppingListItems.GetByID(item.ID, env.Household.ID)
		if err != nil {
			t.Fatalf("get item: %v", err)
		}
		if !updatedItem.Checked {
			t.Error("expected item.Checked to be true after SyncHouseholdIntegrationNow")
		}
	})

	t.Run("test connection failure returns 502 and scrubs token", func(t *testing.T) {
		noop.TestErr = errors.New("rejected SECRET-NOOP-TOKEN by remote")
		defer func() { noop.TestErr = nil }()

		ctxFail, wFail := env.freshCtx(env.User.ID)
		appCtxFail := SetupTestAppContext(ctxFail, env.User.ID)
		appCtxFail.ShoppingListSync.SetProviderFactory(func(_ string, _ *dbModel.HouseholdIntegration, _ *zerolog.Logger, _ *http.Client) (controllers.ShoppingListSyncProvider, error) {
			return noop, nil
		})
		ctxFail.Params = gin.Params{{Key: "provider", Value: util.ShoppingListProviderNoop}}

		TestHouseholdIntegration(ctxFail, appCtxFail)
		if wFail.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", wFail.Code)
		}
		if strings.Contains(wFail.Body.String(), "SECRET-NOOP-TOKEN") {
			t.Errorf("error response leaked secret token: %s", wFail.Body.String())
		}
	})
}
