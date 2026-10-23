package database

import (
	"testing"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// The pack size of the linked product travels with the shopping list row, so
// every consumer of the list reads it from the same place.
func TestListByHouseholdFillsThePackSize(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	linked := testutil.CreateTestProduct(db, household.ID, user.ID)
	linked.ProductName = "Milk"
	linked.PackSize = "1 l"
	if err := db.Save(linked).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	withoutSize := testutil.CreateTestProduct(db, household.ID, user.ID)
	withoutSize.ProductName = "Bread"
	if err := db.Save(withoutSize).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	repo := NewShoppingListItemRepository(db)
	items := []database.ShoppingListItem{
		{HouseholdID: household.ID, ProductID: &linked.ID, Name: "Milk", Quantity: 2, CreatedBy: user.ID},
		{HouseholdID: household.ID, ProductID: &withoutSize.ID, Name: "Bread", Quantity: 1, CreatedBy: user.ID},
		{HouseholdID: household.ID, Name: "Dish soap", Quantity: 1, CreatedBy: user.ID},
	}
	for i := range items {
		if err := db.Create(&items[i]).Error; err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}

	listed, err := repo.ListByHousehold(household.ID)
	if err != nil {
		t.Fatalf("ListByHousehold() error = %v", err)
	}
	sizes := make(map[string]string, len(listed))
	for i := range listed {
		sizes[listed[i].Name] = listed[i].PackSize
	}
	if sizes["Milk"] != "1 l" {
		t.Errorf("the pack size of the linked product = %q, want %q", sizes["Milk"], "1 l")
	}
	if sizes["Bread"] != "" {
		t.Errorf("the pack size of a product without one = %q, want empty", sizes["Bread"])
	}
	if sizes["Dish soap"] != "" {
		t.Errorf("the pack size of an unlinked item = %q, want empty", sizes["Dish soap"])
	}
}
