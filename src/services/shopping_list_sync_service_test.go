package services

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
)

// TestShoppingListSyncService verifies end-to-end household shopping list push, pull, deletion, and error handling.
func TestShoppingListSyncService(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repos := database.NewRepositoryContainer(db, nil)

	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	_ = db.Save(user).Error

	cfg := &configuration.ShoppingListSyncConfiguration{
		Enabled:        true,
		BringEnabled:   false,
		TodoistEnabled: true,
	}

	noop := &controllers.NoopShoppingListSyncProvider{
		CheckedState: map[string]bool{},
	}
	svc := NewShoppingListSyncService(repos, nil, cfg)
	svc.SetProviderFactory(func(_ string, _ *dbModel.HouseholdIntegration, _ *zerolog.Logger, _ *http.Client) (controllers.ShoppingListSyncProvider, error) {
		return noop, nil
	})

	integration := &dbModel.HouseholdIntegration{
		HouseholdID: household.ID,
		Provider:    util.ShoppingListProviderTodoist,
		Enabled:     true,
		AccessToken: "SECRET-TOKEN-123",
		ListID:      "proj-1",
	}
	if err := repos.HouseholdIntegrations.Create(integration); err != nil {
		t.Fatalf("create integration: %v", err)
	}

	item1 := &dbModel.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 2, Unit: "L", CreatedBy: user.ID}
	item2 := &dbModel.ShoppingListItem{HouseholdID: household.ID, Name: "Apples", Quantity: 6, CreatedBy: user.ID}
	_ = repos.ShoppingListItems.Create(item1)
	_ = repos.ShoppingListItems.Create(item2)

	t.Run("initial sync pushes items, creates refs, and sets LastSyncAt", func(t *testing.T) {
		if err := svc.SyncHousehold(household.ID); err != nil {
			t.Fatalf("SyncHousehold: %v", err)
		}

		refs, err := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if err != nil {
			t.Fatalf("list refs: %v", err)
		}
		if len(refs) != 2 {
			t.Fatalf("len(refs) = %d, want 2", len(refs))
		}

		storedInt, err := repos.HouseholdIntegrations.GetByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if err != nil {
			t.Fatalf("get integration: %v", err)
		}
		if storedInt.LastSyncAt == nil || storedInt.LastError != "" {
			t.Errorf("stored integration = %+v, want LastSyncAt set and LastError empty", storedInt)
		}
	})

	t.Run("rerun does not duplicate refs and pulls remote checked state back", func(t *testing.T) {
		refs, _ := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		for i := range refs {
			if refs[i].ShoppingListItemID == item1.ID {
				noop.CheckedState[refs[i].ExternalID] = true
			}
		}

		if err := svc.SyncHousehold(household.ID); err != nil {
			t.Fatalf("second SyncHousehold: %v", err)
		}

		refsAfter, _ := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if len(refsAfter) != 2 {
			t.Errorf("len(refsAfter) = %d, want 2 (no duplicates)", len(refsAfter))
		}

		updatedItem1, err := repos.ShoppingListItems.GetByID(item1.ID, household.ID)
		if err != nil {
			t.Fatalf("GetByID item1: %v", err)
		}
		if !updatedItem1.Checked {
			t.Error("expected item1.Checked to be updated to true from remote pull")
		}
	})

	t.Run("deleting local item removes remote entry and deletes sync ref", func(t *testing.T) {
		if err := repos.ShoppingListItems.Delete(item2.ID, household.ID); err != nil {
			t.Fatalf("Delete item2: %v", err)
		}

		if err := svc.SyncHousehold(household.ID); err != nil {
			t.Fatalf("SyncHousehold after delete: %v", err)
		}

		refs, _ := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if len(refs) != 1 || refs[0].ShoppingListItemID != item1.ID {
			t.Errorf("refs = %+v, want only item1 ref remaining", refs)
		}
	})

	t.Run("provider error records scrubbed LastError and retains existing refs", func(t *testing.T) {
		noop.PushErr = errors.New("auth failed for SECRET-TOKEN-123")
		defer func() { noop.PushErr = nil }()

		err := svc.SyncAll()
		if err == nil {
			t.Fatal("expected error from SyncAll when provider fails")
		}

		storedInt, _ := repos.HouseholdIntegrations.GetByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if storedInt.LastError == "" {
			t.Error("expected LastError to be recorded")
		}
		if strings.Contains(storedInt.LastError, "SECRET-TOKEN-123") {
			t.Errorf("LastError leaked secret token: %q", storedInt.LastError)
		}

		refs, _ := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(household.ID, util.ShoppingListProviderTodoist)
		if len(refs) != 1 {
			t.Errorf("len(refs) = %d, want 1 retained after failed push", len(refs))
		}
	})

	t.Run("instance-disabled provider is skipped by SyncHousehold", func(t *testing.T) {
		bringInt := &dbModel.HouseholdIntegration{
			HouseholdID: household.ID,
			Provider:    util.ShoppingListProviderBring,
			Enabled:     true,
			AccessToken: "bring-tok",
			ListID:      "list-1",
		}
		_ = repos.HouseholdIntegrations.Create(bringInt)

		beforePushCount := noop.PushCount
		if err := svc.SyncHousehold(household.ID); err != nil {
			t.Fatalf("SyncHousehold: %v", err)
		}
		if noop.PushCount != beforePushCount+1 {
			t.Errorf("PushCount delta = %d, want 1 (bring should be skipped because BringEnabled=false)", noop.PushCount-beforePushCount)
		}
	})
}
