package database

import (
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestResolveSortClause(t *testing.T) {
	cases := []struct {
		name       string
		sortValue  string
		orderValue string
		want       string
		wantErr    error
	}{
		{"empty falls back to defaults", "", "", "product_name ASC", nil},
		{"allowlisted column, lowercase direction", "expire_at", "asc", "expire_at ASC", nil},
		{"allowlisted column, uppercase direction", "created_at", "DESC", "created_at DESC", nil},
		{"unknown column is rejected", "category", "asc", "", errors.ErrDatabaseInvalidSortParameter},
		{"unknown direction is rejected", "created_at", "sideways", "", errors.ErrDatabaseInvalidSortParameter},
		{
			"statement injection in column is rejected",
			"created_at; DROP TABLE products;--",
			"asc",
			"",
			errors.ErrDatabaseInvalidSortParameter,
		},
		{
			"subquery in column is rejected",
			"(SELECT 1)",
			"asc",
			"",
			errors.ErrDatabaseInvalidSortParameter,
		},
		{
			"statement injection in direction is rejected",
			"created_at",
			"asc; DROP TABLE products;--",
			"",
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
				t.Errorf("got %q, want %q", got, tc.want)
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
