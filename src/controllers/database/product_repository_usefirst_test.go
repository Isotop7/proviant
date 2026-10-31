package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// TestGetUseFirstProducts covers the list that answers "what do we eat next":
// every active product of the household, ordered by the effective expiry, with
// the storage location and the pack count of each.
func TestGetUseFirstProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	privateOwner := testutil.CreateTestUser(db, household.ID)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	otherUser := testutil.CreateTestUser(db, otherHousehold.ID)

	fridge := testutil.CreateTestStorageLocation(db, household.ID)
	now := time.Now()
	openedAt := now.AddDate(0, 0, -10)
	daysAfterOpening := 5

	create := func(p database.Product) database.Product {
		t.Helper()
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("failed to create product: %v", err)
		}
		return p
	}
	inHousehold := func(p database.Product) database.Product {
		p.HouseholdID = household.ID
		p.UserID = user.ID
		return p
	}

	// Soonest first: a printed expiry in two days.
	soon := create(inHousehold(database.Product{
		ProductName: "Soon", ExpireAt: now.AddDate(0, 0, 2), Quantity: 3, PackSize: "250g",
		StorageLocationID: &fridge.ID,
	}))
	// The printed date is far away, the shelf life is not: the effective expiry
	// decides, so this one comes before "Later".
	opened := create(inHousehold(database.Product{
		ProductName: "Opened", ExpireAt: now.AddDate(0, 0, 30),
		OpenedAt: &openedAt, DaysAfterOpening: &daysAfterOpening, Quantity: 1,
	}))
	later := create(inHousehold(database.Product{
		ProductName: "Later", ExpireAt: now.AddDate(0, 0, 20), Quantity: 1,
	}))
	// Without a date: its own group at the end.
	undated := create(inHousehold(database.Product{ProductName: "Undated", Quantity: 2}))

	// Invisible rows: another member's private product and another household.
	create(database.Product{
		ProductName: "PrivateOther", HouseholdID: household.ID, UserID: privateOwner.ID,
		IsPrivate: true, ExpireAt: now.AddDate(0, 0, -3),
	})
	create(database.Product{
		ProductName: "OtherHousehold", HouseholdID: otherHousehold.ID, UserID: otherUser.ID,
		ExpireAt: now.AddDate(0, 0, -3),
	})

	t.Run("the list is ordered by the effective expiry", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantOrder := []uint{opened.ID, soon.ID, later.ID, undated.ID}
		if len(list) != len(wantOrder) {
			t.Fatalf("len(list) = %d, want %d (%+v)", len(list), len(wantOrder), list)
		}
		for i, want := range wantOrder {
			if list[i].ID != want {
				t.Errorf("list[%d].ID = %d, want %d", i, list[i].ID, want)
			}
		}
	})

	t.Run("the products without a date are the last group", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		last := list[len(list)-1]
		if last.ID != undated.ID {
			t.Fatalf("last product = %d, want the undated one (%d)", last.ID, undated.ID)
		}
		if last.ExpireAt != "" {
			t.Errorf("ExpireAt = %q, want empty for a product without a date", last.ExpireAt)
		}
		for _, entry := range list[:len(list)-1] {
			if entry.ExpireAt == "" {
				t.Errorf("product %d has no date but is not the last one", entry.ID)
			}
		}
	})

	t.Run("an opened product is ordered by its shelf life, not by its printed date", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if list[0].ID != opened.ID {
			t.Fatalf("first product = %d, want the opened one (%d)", list[0].ID, opened.ID)
		}
		wantExpiry := openedAt.AddDate(0, 0, daysAfterOpening).Format("2006-01-02")
		if list[0].ExpireAt != wantExpiry {
			t.Errorf("ExpireAt = %q, want the shelf-life date %q", list[0].ExpireAt, wantExpiry)
		}
	})

	t.Run("every entry carries the location and the pack count", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		byID := map[uint]int{}
		for i, entry := range list {
			byID[entry.ID] = i
		}
		entry := list[byID[soon.ID]]
		if entry.StorageLocationName != fridge.Name || entry.StorageLocationIcon != fridge.Icon {
			t.Errorf("location = %q/%q, want %q/%q",
				entry.StorageLocationName, entry.StorageLocationIcon, fridge.Name, fridge.Icon)
		}
		if entry.StorageLocationID == nil || *entry.StorageLocationID != fridge.ID {
			t.Errorf("StorageLocationID = %v, want %d", entry.StorageLocationID, fridge.ID)
		}
		if entry.Quantity != 3 {
			t.Errorf("Quantity = %d, want 3", entry.Quantity)
		}
		if entry.PackSize != "250g" {
			t.Errorf("PackSize = %q, want %q", entry.PackSize, "250g")
		}
		if entry.ProductName != "Soon" {
			t.Errorf("ProductName = %q, want %q", entry.ProductName, "Soon")
		}
	})

	t.Run("the limit cuts the head of the order", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("len(list) = %d, want 2", len(list))
		}
		if list[0].ID != opened.ID || list[1].ID != soon.ID {
			t.Errorf("list = %d/%d, want the two soonest products", list[0].ID, list[1].ID)
		}
	})

	t.Run("the rows of other households and private rows stay out", func(t *testing.T) {
		list, err := repo.GetUseFirstProducts(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, entry := range list {
			if entry.ProductName == "PrivateOther" || entry.ProductName == "OtherHousehold" {
				t.Errorf("the list carries %q", entry.ProductName)
			}
		}
	})
}

// TestEffectiveExpiryOrderingsAgree pins the invariant the products list
// depends on: a product holds the same position in the narrow projection the
// page pages from and in the full rows the repository hands out. The two
// orderings drift apart as soon as one of them stops sorting the way the other
// does, and a product then lands on two pages or on none.
func TestEffectiveExpiryOrderingsAgree(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	now := time.Now()
	openedAt := now.AddDate(0, 0, -2)
	daysAfterOpening := 3

	create := func(p database.Product) database.Product {
		t.Helper()
		p.HouseholdID = household.ID
		p.UserID = user.ID
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("failed to create product: %v", err)
		}
		return p
	}

	// Two products share an expiry date: they are a tie, and the tie order has
	// to be the same in both orderings.
	create(database.Product{ProductName: "TieA", ExpireAt: now.AddDate(0, 0, 4)})
	create(database.Product{ProductName: "TieB", ExpireAt: now.AddDate(0, 0, 4)})
	create(database.Product{ProductName: "Soon", ExpireAt: now.AddDate(0, 0, 1)})
	create(database.Product{ProductName: "Opened", ExpireAt: now.AddDate(0, 0, 30),
		OpenedAt: &openedAt, DaysAfterOpening: &daysAfterOpening})
	create(database.Product{ProductName: "Undated"})

	projections, err := repo.GetActiveProductProjections(user.ID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	SortByEffectiveExpiry(projections)

	rows, err := repo.GetUserProductsBulkSorted(user.ID, 0, 0, SortEffectiveExpiry, "asc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projections) != len(rows) {
		t.Fatalf("len(projections) = %d, len(rows) = %d", len(projections), len(rows))
	}
	for i := range projections {
		if projections[i].ID != rows[i].ID {
			t.Errorf("position %d: projection holds %d, the full rows hold %d",
				i, projections[i].ID, rows[i].ID)
		}
	}
}
