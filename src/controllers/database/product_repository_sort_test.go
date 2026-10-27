package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func TestResolveSortClause(t *testing.T) {
	cases := []struct {
		name       string
		sortValue  string
		orderValue string
		want       productSort
		wantErr    error
	}{
		{
			"empty falls back to defaults",
			"",
			"",
			productSort{clause: "product_name ASC"},
			nil,
		},
		{
			"allowlisted column, lowercase direction",
			"expire_at",
			"asc",
			productSort{clause: "expire_at ASC"},
			nil,
		},
		{
			"allowlisted column, uppercase direction",
			"created_at",
			"DESC",
			productSort{clause: "created_at DESC", descending: true},
			nil,
		},
		{
			"the effective expiry is ordered in Go, on the id order",
			SortEffectiveExpiry,
			"asc",
			productSort{clause: "id ASC", byEffectiveExpiry: true},
			nil,
		},
		{
			"the effective expiry honours the direction",
			SortEffectiveExpiry,
			"desc",
			productSort{clause: "id DESC", byEffectiveExpiry: true, descending: true},
			nil,
		},
		{"unknown column is rejected", "category", "asc", productSort{}, errors.ErrDatabaseInvalidSortParameter},
		{"unknown direction is rejected", "created_at", "sideways", productSort{}, errors.ErrDatabaseInvalidSortParameter},
		{
			"statement injection in column is rejected",
			"created_at; DROP TABLE products;--",
			"asc",
			productSort{},
			errors.ErrDatabaseInvalidSortParameter,
		},
		{
			"subquery in column is rejected",
			"(SELECT 1)",
			"asc",
			productSort{},
			errors.ErrDatabaseInvalidSortParameter,
		},
		{
			"statement injection in direction is rejected",
			"created_at",
			"asc; DROP TABLE products;--",
			productSort{},
			errors.ErrDatabaseInvalidSortParameter,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSortClause(tc.sortValue, tc.orderValue)
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestSearchProductsSortAllowlist proves the guard at the single Order() call
// site: a rejected sort never reaches SQL, and the table is still queryable
// afterwards.
func TestSearchProductsSortAllowlist(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID)
	product.ProductName = "Milk"
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("malicious sort is rejected before SQL", func(t *testing.T) {
		_, err := repo.SearchProducts(ProductName, "Milk", "product_name; DROP TABLE products;--", "asc", user.ID)
		if err != errors.ErrDatabaseInvalidSortParameter {
			t.Fatalf("err = %v, want ErrDatabaseInvalidSortParameter", err)
		}

		var count int64
		if err := db.Model(&database.Product{}).Count(&count).Error; err != nil {
			t.Fatalf("products table unusable after rejected sort: %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("allowlisted sort returns ordered rows", func(t *testing.T) {
		products, err := repo.SearchProducts(ProductName, "Milk", "created_at", "desc", user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 1 || products[0].ProductName != "Milk" {
			t.Errorf("got %d products (%+v), want the single match", len(products), products)
		}
	})
}

// TestGetUserProductsBulkSortedByEffectiveExpiry proves that the effective
// expiry orders the list and that the limit cuts the head of that order. The
// database cannot order by the effective expiry, so a limit applied before the
// sort would return an arbitrary subset of the pantry.
func TestGetUserProductsBulkSortedByEffectiveExpiry(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	now := time.Now()

	create := func(name string, expireAt time.Time) database.Product {
		t.Helper()
		product := database.Product{
			ProductName: name,
			HouseholdID: household.ID,
			UserID:      user.ID,
			ExpireAt:    expireAt,
		}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("failed to create product: %v", err)
		}
		return product
	}

	soonest := create("Soonest", now.AddDate(0, 0, 1))
	create("Middle", now.AddDate(0, 0, 5))
	create("Latest", now.AddDate(0, 0, 10))

	t.Run("the effective expiry orders the list", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkSorted(user.ID, 0, SortEffectiveExpiry, "asc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 3 {
			t.Fatalf("len(products) = %d, want 3", len(products))
		}
		if products[0].ID != soonest.ID {
			t.Errorf("products[0] = %d, want the soonest one (%d)", products[0].ID, soonest.ID)
		}
	})

	t.Run("the limit cuts the head of the order", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkSorted(user.ID, 1, SortEffectiveExpiry, "asc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 1 {
			t.Fatalf("len(products) = %d, want 1", len(products))
		}
		if products[0].ID != soonest.ID {
			t.Errorf("products[0] = %d, want the soonest one (%d)", products[0].ID, soonest.ID)
		}
	})

	t.Run("the direction is honoured", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkSorted(user.ID, 1, SortEffectiveExpiry, "desc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 1 {
			t.Fatalf("len(products) = %d, want 1", len(products))
		}
		if products[0].ProductName != "Latest" {
			t.Errorf("products[0] = %q, want the product that expires last", products[0].ProductName)
		}
	})

	t.Run("a column sort is left to the database", func(t *testing.T) {
		products, err := repo.GetUserProductsBulkSorted(user.ID, 0, "product_name", "asc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Latest", "Middle", "Soonest"}
		for i, name := range want {
			if products[i].ProductName != name {
				t.Errorf("products[%d] = %q, want %q", i, products[i].ProductName, name)
			}
		}
	})

	t.Run("an unknown sort is rejected", func(t *testing.T) {
		if _, err := repo.GetUserProductsBulkSorted(user.ID, 0, "category", "asc"); err != errors.ErrDatabaseInvalidSortParameter {
			t.Errorf("err = %v, want ErrDatabaseInvalidSortParameter", err)
		}
	})
}
