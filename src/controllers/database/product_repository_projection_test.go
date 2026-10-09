package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

// TestProductProjections covers the narrow queries the products page pages from:
// they must carry the same visibility rules as the full-row queries (household,
// privacy, soft delete), stay narrow, and arrive in a deterministic order.
func TestProductProjections(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	privateOwner := testutil.CreateTestUser(db, household.ID)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	otherUser := testutil.CreateTestUser(db, otherHousehold.ID)

	locA := testutil.CreateTestStorageLocation(db, household.ID)
	locB := testutil.CreateTestStorageLocation(db, household.ID)

	now := time.Now()
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

	// Created in this order, so ascending ids are known.
	zebra := create(inHousehold(database.Product{
		ProductName: "Zebra", ExpireAt: now.AddDate(0, 0, 10), StorageLocationID: &locA.ID,
	}))
	apple := create(inHousehold(database.Product{
		ProductName: "Apple", ExpireAt: now.AddDate(0, 0, 5), StorageLocationID: &locB.ID,
	}))
	mango := create(inHousehold(database.Product{
		ProductName: "Mango", ExpireAt: now.AddDate(0, 0, 1),
	}))
	archived := create(inHousehold(database.Product{
		ProductName: "Archived", Barcode: "999", ExpireAt: now.AddDate(0, 0, -1),
		StorageLocationID: &locA.ID,
	}))
	if err := db.Delete(&archived).Error; err != nil {
		t.Fatalf("failed to archive product: %v", err)
	}
	create(database.Product{
		ProductName: "PrivateOther", HouseholdID: household.ID, UserID: privateOwner.ID,
		IsPrivate: true, ExpireAt: now,
	})
	create(database.Product{
		ProductName: "OtherHousehold", HouseholdID: otherHousehold.ID, UserID: otherUser.ID,
		ExpireAt: now,
	})

	t.Run("active projections are household-scoped, ordered and narrow", func(t *testing.T) {
		projections, err := repo.GetActiveProductProjections(user.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projections) != 3 {
			t.Fatalf("len(projections) = %d, want 3", len(projections))
		}
		wantIDs := []uint{zebra.ID, apple.ID, mango.ID}
		for i, want := range wantIDs {
			if projections[i].ID != want {
				t.Errorf("projections[%d].ID = %d, want %d (order must be stable across pages)",
					i, projections[i].ID, want)
			}
		}
		for _, p := range projections {
			if p.ProductName != "" || p.Barcode != "" || p.Categories != "" {
				t.Errorf("projection for id %d carried columns outside the projection: name=%q barcode=%q categories=%q",
					p.ID, p.ProductName, p.Barcode, p.Categories)
			}
			if p.ExpireAt.IsZero() {
				t.Errorf("projection for id %d lost expire_at", p.ID)
			}
		}
	})

	t.Run("location filter narrows the active projection", func(t *testing.T) {
		projections, err := repo.GetActiveProductProjections(user.ID, locA.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projections) != 1 || projections[0].ID != zebra.ID {
			t.Errorf("got %d projections (%+v), want only the locA product", len(projections), projections)
		}
	})

	t.Run("archived projections contain only archived rows", func(t *testing.T) {
		projections, err := repo.GetArchivedProductProjections(user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projections) != 1 || projections[0].ID != archived.ID {
			t.Fatalf("got %d projections (%+v), want only the archived product", len(projections), projections)
		}
		if projections[0].ProductName != "" {
			t.Errorf("projection for id %d was not narrowed", projections[0].ID)
		}
	})

	t.Run("archived hydration returns full rows with their location", func(t *testing.T) {
		rows, err := repo.GetUserArchivedProductsByIDs(user.ID, []uint{archived.ID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		if rows[0].ProductName != "Archived" {
			t.Errorf("ProductName = %q, want %q", rows[0].ProductName, "Archived")
		}
		if rows[0].StorageLocation == nil || rows[0].StorageLocation.ID != locA.ID {
			t.Errorf("StorageLocation = %+v, want the archived product's location", rows[0].StorageLocation)
		}
	})

	t.Run("archived hydration with no ids does not query", func(t *testing.T) {
		rows, err := repo.GetUserArchivedProductsByIDs(user.ID, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("len(rows) = %d, want 0", len(rows))
		}
	})

	t.Run("search projections honour visibility and the sort allowlist", func(t *testing.T) {
		ascending, err := repo.SearchProductProjections(ProductName, "a", "product_name", "asc", user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantOrder := []uint{apple.ID, mango.ID, zebra.ID}
		if len(ascending) != len(wantOrder) {
			t.Fatalf("len(ascending) = %d, want %d", len(ascending), len(wantOrder))
		}
		for i, want := range wantOrder {
			if ascending[i].ID != want {
				t.Errorf("ascending[%d].ID = %d, want %d", i, ascending[i].ID, want)
			}
		}

		descending, err := repo.SearchProductProjections(ProductName, "a", "product_name", "desc", user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, want := range []uint{zebra.ID, mango.ID, apple.ID} {
			if descending[i].ID != want {
				t.Errorf("descending[%d].ID = %d, want %d", i, descending[i].ID, want)
			}
		}

		if _, err := repo.SearchProductProjections(ProductName, "a", "product_name; DROP TABLE products;--", "asc", user.ID); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("err = %v, want ErrDatabaseInvalidSortParameter", err)
		}
	})
}
