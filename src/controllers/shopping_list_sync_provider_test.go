package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"
)

// TestNewShoppingListSyncProvider verifies factory construction and rejection of unknown providers.
func TestNewShoppingListSyncProvider(t *testing.T) {
	t.Run("constructs noop, todoist, and bring providers", func(t *testing.T) {
		for _, providerName := range []string{
			util.ShoppingListProviderNoop,
			util.ShoppingListProviderTodoist,
			util.ShoppingListProviderBring,
		} {
			p, err := NewShoppingListSyncProvider(providerName, nil, nil, nil)
			if err != nil {
				t.Fatalf("NewShoppingListSyncProvider(%q): %v", providerName, err)
			}
			if p.Name() != providerName {
				t.Errorf("Name() = %q, want %q", p.Name(), providerName)
			}
		}
	})

	t.Run("rejects unknown provider", func(t *testing.T) {
		_, err := NewShoppingListSyncProvider("invalid", nil, nil, nil)
		if !errors.Is(err, apperrors.ErrIntegrationInvalidProvider) {
			t.Errorf("err = %v, want %v", err, apperrors.ErrIntegrationInvalidProvider)
		}
	})
}

// TestNoopShoppingListSyncProvider verifies offline push and pull behavior of the no-op provider.
func TestNoopShoppingListSyncProvider(t *testing.T) {
	noop := &NoopShoppingListSyncProvider{
		CheckedState: map[string]bool{
			"noop-1": true,
			"noop-2": false,
		},
	}
	integration := &dbModel.HouseholdIntegration{HouseholdID: 1, Provider: util.ShoppingListProviderNoop}

	t.Run("test connection and push/pull lifecycle", func(t *testing.T) {
		if err := noop.TestConnection(context.Background(), integration); err != nil {
			t.Fatalf("TestConnection: %v", err)
		}

		items := []ShoppingListSyncPayload{
			{ID: 1, Name: "Milk", Quantity: 2, Unit: "L"},
			{ID: 2, Name: "Eggs", Quantity: 12},
		}
		refs := []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 1, ExternalID: "noop-1"},
			{ShoppingListItemID: 3, ExternalID: "noop-3"},
		}

		pushRes, err := noop.Push(context.Background(), integration, items, refs)
		if err != nil {
			t.Fatalf("Push: %v", err)
		}
		if pushRes.Created[2] != "noop-2" {
			t.Errorf("Created[2] = %q, want noop-2", pushRes.Created[2])
		}
		if len(pushRes.Updated) != 1 || pushRes.Updated[0] != 1 {
			t.Errorf("Updated = %v, want [1]", pushRes.Updated)
		}
		if len(pushRes.Deleted) != 1 || pushRes.Deleted[0] != "noop-3" {
			t.Errorf("Deleted = %v, want [noop-3]", pushRes.Deleted)
		}

		checked, err := noop.PullChecked(context.Background(), integration, []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 1, ExternalID: "noop-1"},
			{ShoppingListItemID: 2, ExternalID: "noop-2"},
		})
		if err != nil {
			t.Fatalf("PullChecked: %v", err)
		}
		if !checked["noop-1"] || checked["noop-2"] {
			t.Errorf("checked = %v, want noop-1=true, noop-2=false", checked)
		}
	})
}

// TestTodoistSyncProvider verifies Todoist REST API v2 requests and responses against an httptest server.
func TestTodoistSyncProvider(t *testing.T) {
	var closedTasks []string
	var deletedTasks []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer todoist-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/proj-1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"proj-1","name":"Groceries"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/tasks":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"task-new","content":"Milk (2 L)","is_completed":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/tasks/task-existing":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"task-existing","content":"Bread","is_completed":false}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/close"):
			closedTasks = append(closedTasks, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reopen"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/tasks/task-removed":
			deletedTasks = append(deletedTasks, "task-removed")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/tasks":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"task-new","content":"Milk (2 L)","is_completed":false}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := &TodoistSyncProvider{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}
	integration := &dbModel.HouseholdIntegration{
		HouseholdID: 1,
		Provider:    util.ShoppingListProviderTodoist,
		AccessToken: "todoist-secret",
		ListID:      "proj-1",
	}

	t.Run("test connection succeeds with valid token and fails without credentials", func(t *testing.T) {
		if err := provider.TestConnection(context.Background(), integration); err != nil {
			t.Fatalf("TestConnection: %v", err)
		}
		if err := provider.TestConnection(context.Background(), &dbModel.HouseholdIntegration{}); !errors.Is(err, apperrors.ErrIntegrationMissingCredentials) {
			t.Errorf("err = %v, want %v", err, apperrors.ErrIntegrationMissingCredentials)
		}
	})

	t.Run("push creates, updates, closes, and deletes tasks", func(t *testing.T) {
		items := []ShoppingListSyncPayload{
			{ID: 10, Name: "Milk", Quantity: 2, Unit: "L", Checked: true, Notes: "Organic"},
			{ID: 11, Name: "Bread", Quantity: 1, Checked: false},
		}
		refs := []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 11, ExternalID: "task-existing"},
			{ShoppingListItemID: 12, ExternalID: "task-removed"},
		}
		res, err := provider.Push(context.Background(), integration, items, refs)
		if err != nil {
			t.Fatalf("Push: %v", err)
		}
		if res.Created[10] != "task-new" {
			t.Errorf("Created[10] = %q, want task-new", res.Created[10])
		}
		if len(res.Updated) != 1 || res.Updated[0] != 11 {
			t.Errorf("Updated = %v, want [11]", res.Updated)
		}
		if len(res.Deleted) != 1 || res.Deleted[0] != "task-removed" {
			t.Errorf("Deleted = %v, want [task-removed]", res.Deleted)
		}
		if len(closedTasks) == 0 {
			t.Error("expected checked new task to be closed")
		}
		if len(deletedTasks) != 1 {
			t.Errorf("deletedTasks = %v, want [task-removed]", deletedTasks)
		}
	})

	t.Run("pull marks missing active tasks as completed", func(t *testing.T) {
		refs := []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 10, ExternalID: "task-new"},
			{ShoppingListItemID: 11, ExternalID: "task-existing"},
		}
		checked, err := provider.PullChecked(context.Background(), integration, refs)
		if err != nil {
			t.Fatalf("PullChecked: %v", err)
		}
		if checked["task-new"] {
			t.Error("expected task-new (present in active list with is_completed=false) to be unchecked")
		}
		if !checked["task-existing"] {
			t.Error("expected task-existing (omitted from active list) to be marked checked")
		}
	})
}

// TestBringSyncProvider verifies Bring! list batch push and checked-state pull against an httptest server.
func TestBringSyncProvider(t *testing.T) {
	var lastBatch bringBatchUpdateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bring-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/bringlists/list-uuid-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"uuid":"list-uuid-1","purchase":[{"name":"Milk","specification":"2 L"}],"recently":[{"name":"Bread","specification":"1"}]}`))
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&lastBatch)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	provider := &BringSyncProvider{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}
	integration := &dbModel.HouseholdIntegration{
		HouseholdID: 1,
		Provider:    util.ShoppingListProviderBring,
		AccessToken: "bring-secret",
		ListID:      "list-uuid-1",
	}

	t.Run("validates credentials and list ID", func(t *testing.T) {
		if err := provider.TestConnection(context.Background(), integration); err != nil {
			t.Fatalf("TestConnection: %v", err)
		}
		if err := provider.TestConnection(context.Background(), &dbModel.HouseholdIntegration{AccessToken: "tok"}); !errors.Is(err, apperrors.ErrIntegrationMissingListID) {
			t.Errorf("err = %v, want %v", err, apperrors.ErrIntegrationMissingListID)
		}
	})

	t.Run("push sends purchase, recently, and remove batches", func(t *testing.T) {
		items := []ShoppingListSyncPayload{
			{ID: 1, Name: "Milk", Quantity: 2, Unit: "L", Checked: false, Notes: "Whole"},
			{ID: 2, Name: "Bread", Quantity: 1, Checked: true},
		}
		refs := []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 1, ExternalID: "Milk"},
			{ShoppingListItemID: 3, ExternalID: "Butter"},
		}
		res, err := provider.Push(context.Background(), integration, items, refs)
		if err != nil {
			t.Fatalf("Push: %v", err)
		}
		if res.Created[2] != "Bread" {
			t.Errorf("Created[2] = %q, want Bread", res.Created[2])
		}
		if len(lastBatch.Purchase) != 1 || lastBatch.Purchase[0].Name != "Milk" {
			t.Errorf("Purchase = %+v, want Milk", lastBatch.Purchase)
		}
		if len(lastBatch.Recently) != 1 || lastBatch.Recently[0].Name != "Bread" {
			t.Errorf("Recently = %+v, want Bread", lastBatch.Recently)
		}
		if len(lastBatch.Remove) != 1 || lastBatch.Remove[0].Name != "Butter" {
			t.Errorf("Remove = %+v, want Butter", lastBatch.Remove)
		}
	})

	t.Run("pull maps purchase to unchecked and recently to checked", func(t *testing.T) {
		refs := []dbModel.ShoppingListItemSyncRef{
			{ShoppingListItemID: 1, ExternalID: "Milk"},
			{ShoppingListItemID: 2, ExternalID: "Bread"},
		}
		checked, err := provider.PullChecked(context.Background(), integration, refs)
		if err != nil {
			t.Fatalf("PullChecked: %v", err)
		}
		if checked["Milk"] {
			t.Error("expected Milk in purchase to be unchecked")
		}
		if !checked["Bread"] {
			t.Error("expected Bread in recently to be checked")
		}
	})
}

// TestShoppingListSyncController verifies early return when disabled and secret scrubbing on errors.
func TestShoppingListSyncController(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("dispatch and run once return early when disabled", func(t *testing.T) {
		ctrl := NewShoppingListSyncController(nil, &configuration.ShoppingListSyncConfiguration{Enabled: false}, db)
		ctrl.Dispatch()
		if err := ctrl.RunOnce(context.Background()); err != nil {
			t.Errorf("RunOnce when disabled: %v", err)
		}
	})

	t.Run("ScrubIntegrationError redacts access and refresh tokens", func(t *testing.T) {
		integration := &dbModel.HouseholdIntegration{
			AccessToken:  "super-secret-access",
			RefreshToken: "super-secret-refresh",
		}
		rawErr := errors.New("upstream rejected token super-secret-access and refresh super-secret-refresh")
		scrubbed := ScrubIntegrationError(rawErr, integration)
		if strings.Contains(scrubbed, "super-secret-access") || strings.Contains(scrubbed, "super-secret-refresh") {
			t.Errorf("scrubbed error still contains secrets: %q", scrubbed)
		}
		if !strings.Contains(scrubbed, "[REDACTED]") {
			t.Errorf("scrubbed = %q, want [REDACTED] placeholder", scrubbed)
		}
	})
}
