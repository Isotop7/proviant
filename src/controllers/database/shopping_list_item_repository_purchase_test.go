package database

import (
	"testing"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// A purchase writes the product and the check together: a failure between
// the two writes leaves neither behind, so the item stays unchecked.
func TestPurchaseWithProductRollsBackWhenTheCheckFails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	item := database.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 2, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	failUpdatesOnTable(t, db, "shopping_list_items")

	repo := NewShoppingListItemRepository(db)
	product := database.Product{ProductName: "Milk", Quantity: 2, UserID: user.ID}
	if err := repo.PurchaseWithProduct(&item, &product); err == nil {
		t.Fatal("PurchaseWithProduct() error = nil, want the injected failure")
	}

	var products int64
	db.Model(&database.Product{}).Count(&products)
	if products != 0 {
		t.Errorf("products = %d, want 0 (the purchase rolled back)", products)
	}

	var stored database.ShoppingListItem
	if err := db.First(&stored, item.ID).Error; err != nil {
		t.Fatalf("load item: %v", err)
	}
	if stored.Checked {
		t.Error("item is checked, want unchecked after the rollback")
	}
	if stored.ProductID != nil {
		t.Errorf("productId = %v, want nil after the rollback", stored.ProductID)
	}
}

// The packs of a purchase belong to the product the item points at: a
// failure to check the item leaves the count of that product alone.
func TestPurchaseIntoProductRollsBackWhenTheCheckFails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.Quantity = 1
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	item := database.ShoppingListItem{HouseholdID: household.ID, ProductID: &product.ID, Name: "Milk", Quantity: 3, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	failUpdatesOnTable(t, db, "shopping_list_items")

	repo := NewShoppingListItemRepository(db)
	if err := repo.PurchaseIntoProduct(&item, product.ID, 3); err == nil {
		t.Fatal("PurchaseIntoProduct() error = nil, want the injected failure")
	}

	var stored database.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != 1 {
		t.Errorf("product quantity = %d, want 1 (the purchase rolled back)", stored.Quantity)
	}

	var storedItem database.ShoppingListItem
	if err := db.First(&storedItem, item.ID).Error; err != nil {
		t.Fatalf("load item: %v", err)
	}
	if storedItem.Checked {
		t.Error("item is checked, want unchecked after the rollback")
	}
}

// A product that is gone takes the packs nowhere: the purchase fails and the
// item stays unchecked. The caller read the product before the write, so the
// count update is the one that finds out.
func TestPurchaseIntoProductFailsWhenTheProductIsGone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	item := database.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 2, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	repo := NewShoppingListItemRepository(db)
	err := repo.PurchaseIntoProduct(&item, 9999, 2)
	if err != errors.ErrShoppingListItemProductGone {
		t.Fatalf("error = %v, want %v", err, errors.ErrShoppingListItemProductGone)
	}

	var stored database.ShoppingListItem
	if err := db.First(&stored, item.ID).Error; err != nil {
		t.Fatalf("load item: %v", err)
	}
	if stored.Checked {
		t.Error("item is checked, want unchecked when the packs went nowhere")
	}
}

// The item is deleted between the caller's read and the write: the purchase
// must not leave a product behind that no item points at, and the item must
// not be bought a second time.
func TestPurchaseWithProductRollsBackWhenTheItemIsGone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	item := database.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 2, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := db.Delete(&item).Error; err != nil {
		t.Fatalf("delete item: %v", err)
	}

	repo := NewShoppingListItemRepository(db)
	product := database.Product{ProductName: "Milk", Quantity: 2, UserID: user.ID}
	if err := repo.PurchaseWithProduct(&item, &product); err != errors.ErrShoppingListItemNotFound {
		t.Fatalf("error = %v, want %v", err, errors.ErrShoppingListItemNotFound)
	}

	var products int64
	db.Model(&database.Product{}).Count(&products)
	if products != 0 {
		t.Errorf("products = %d, want 0 (the purchase rolled back)", products)
	}
}

// The item is deleted between the caller's read and the write: the count of
// the product must not move either.
func TestPurchaseIntoProductRollsBackWhenTheItemIsGone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.Quantity = 1
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	item := database.ShoppingListItem{HouseholdID: household.ID, ProductID: &product.ID, Name: "Milk", Quantity: 3, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := db.Delete(&item).Error; err != nil {
		t.Fatalf("delete item: %v", err)
	}

	repo := NewShoppingListItemRepository(db)
	if err := repo.PurchaseIntoProduct(&item, product.ID, 3); err != errors.ErrShoppingListItemNotFound {
		t.Fatalf("error = %v, want %v", err, errors.ErrShoppingListItemNotFound)
	}

	var stored database.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.Quantity != 1 {
		t.Errorf("product quantity = %d, want 1 (the purchase rolled back)", stored.Quantity)
	}
}

// The purchase writes the product into the household of the item, so the
// link can never point at a product another household can see.
func TestPurchaseWithProductStampsTheHouseholdOfTheItem(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	item := database.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 1, CreatedBy: user.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	repo := NewShoppingListItemRepository(db)
	product := database.Product{ProductName: "Milk", Quantity: 1, UserID: user.ID}
	if err := repo.PurchaseWithProduct(&item, &product); err != nil {
		t.Fatalf("PurchaseWithProduct() error = %v", err)
	}

	var stored database.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.HouseholdID != household.ID {
		t.Errorf("householdId = %d, want %d", stored.HouseholdID, household.ID)
	}
}
