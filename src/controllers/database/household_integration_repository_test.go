package database_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// TestHouseholdIntegrationRepository verifies CRUD and upsert operations on household integrations.
func TestHouseholdIntegrationRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := database.NewHouseholdIntegrationRepository(db)

	t.Run("create and get by household and provider", func(t *testing.T) {
		integration := &dbModel.HouseholdIntegration{
			HouseholdID:  1,
			Provider:     util.ShoppingListProviderTodoist,
			Enabled:      true,
			AccessToken:  "secret-token",
			AccountEmail: "user@example.com",
			ListID:       "proj-1",
		}
		if err := repo.Create(integration); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.GetByHouseholdAndProvider(1, util.ShoppingListProviderTodoist)
		if err != nil {
			t.Fatalf("GetByHouseholdAndProvider: %v", err)
		}
		if got.AccessToken != "secret-token" || got.ListID != "proj-1" || !got.Enabled {
			t.Errorf("unexpected integration: %+v", got)
		}
	})

	t.Run("get unknown returns ErrIntegrationNotFound", func(t *testing.T) {
		_, err := repo.GetByHouseholdAndProvider(99, util.ShoppingListProviderBring)
		if !errors.Is(err, apperrors.ErrIntegrationNotFound) {
			t.Errorf("err = %v, want %v", err, apperrors.ErrIntegrationNotFound)
		}
	})

	t.Run("upsert updates existing and list enabled filters disabled", func(t *testing.T) {
		bring := &dbModel.HouseholdIntegration{
			HouseholdID: 1,
			Provider:    util.ShoppingListProviderBring,
			Enabled:     false,
			AccessToken: "bring-token",
			ListID:      "list-1",
		}
		if err := repo.Upsert(bring); err != nil {
			t.Fatalf("Upsert new: %v", err)
		}

		bring.ListID = "list-2"
		if err := repo.Upsert(bring); err != nil {
			t.Fatalf("Upsert existing: %v", err)
		}

		all, err := repo.ListByHousehold(1)
		if err != nil {
			t.Fatalf("ListByHousehold: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("len(all) = %d, want 2", len(all))
		}

		enabled, err := repo.ListEnabled()
		if err != nil {
			t.Fatalf("ListEnabled: %v", err)
		}
		if len(enabled) != 1 || enabled[0].Provider != util.ShoppingListProviderTodoist {
			t.Errorf("enabled = %+v, want only todoist", enabled)
		}
	})

	t.Run("delete removes row and allows re-creating same provider", func(t *testing.T) {
		if err := repo.Delete(1, util.ShoppingListProviderBring); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if err := repo.Delete(1, util.ShoppingListProviderBring); !errors.Is(err, apperrors.ErrIntegrationNotFound) {
			t.Errorf("second Delete err = %v, want %v", err, apperrors.ErrIntegrationNotFound)
		}

		recreated := &dbModel.HouseholdIntegration{
			HouseholdID: 1,
			Provider:    util.ShoppingListProviderBring,
			Enabled:     true,
			AccessToken: "new-bring-token",
			ListID:      "list-3",
		}
		if err := repo.Upsert(recreated); err != nil {
			t.Fatalf("Upsert after delete: %v", err)
		}
	})
}

// TestShoppingListItemSyncRefRepository verifies upsert, listing, and deletion of external item references.
func TestShoppingListItemSyncRefRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := database.NewShoppingListItemSyncRefRepository(db)

	t.Run("upsert, list, and delete external refs", func(t *testing.T) {
		ref := &dbModel.ShoppingListItemSyncRef{
			HouseholdID:        1,
			ShoppingListItemID: 10,
			Provider:           util.ShoppingListProviderTodoist,
			ExternalID:         "ext-10",
			ListID:             "proj-1",
			SyncedAt:           time.Now().UTC(),
			Status:             util.SyncRefStatusSynced,
		}
		if err := repo.Upsert(ref); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		ref.ExternalID = "ext-10-updated"
		if err := repo.Upsert(ref); err != nil {
			t.Fatalf("Upsert update: %v", err)
		}

		refs, err := repo.ListByHouseholdAndProvider(1, util.ShoppingListProviderTodoist)
		if err != nil {
			t.Fatalf("ListByHouseholdAndProvider: %v", err)
		}
		if len(refs) != 1 || refs[0].ExternalID != "ext-10-updated" {
			t.Fatalf("refs = %+v, want 1 updated ref", refs)
		}

		if err := repo.DeleteByExternalID(1, util.ShoppingListProviderTodoist, "ext-10-updated"); err != nil {
			t.Fatalf("DeleteByExternalID: %v", err)
		}
		refs, _ = repo.ListByHouseholdAndProvider(1, util.ShoppingListProviderTodoist)
		if len(refs) != 0 {
			t.Errorf("len(refs) = %d, want 0", len(refs))
		}
	})
}
