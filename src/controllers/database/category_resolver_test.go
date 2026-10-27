package database

import (
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func canonicalPriceRows() []dbModel.ProductCategoryPrice {
	return []dbModel.ProductCategoryPrice{
		{CategoryKey: "dairy products", DisplayName: "Dairy Products", AvgPriceEUR: 1.80, CO2KgPerKg: 3.20, WeightGrams: 500},
		{CategoryKey: "milk", DisplayName: "Milk", AvgPriceEUR: 0.90, CO2KgPerKg: 3.20, WeightGrams: 1000},
		{CategoryKey: "cheese", DisplayName: "Cheese", AvgPriceEUR: 3.50, CO2KgPerKg: 8.50, WeightGrams: 200},
		{CategoryKey: "fresh vegetables", DisplayName: "Fresh Vegetables", AvgPriceEUR: 1.50, CO2KgPerKg: 0.40, WeightGrams: 500},
		{CategoryKey: "rice", DisplayName: "Rice", AvgPriceEUR: 2.00, CO2KgPerKg: 2.70, WeightGrams: 1000},
	}
}

// The longest matching canonical key wins, so a product tagged with a
// generic and a specific category keeps the price the household already paid.
func TestCategoryResolverLongestMatchingKeyWins(t *testing.T) {
	resolver := NewCategoryResolver(canonicalPriceRows())

	tests := []struct {
		name       string
		categories string
		want       string
	}{
		{name: "generic and specific tag", categories: "dairy products, milk", want: "dairy products"},
		{name: "language tagged", categories: "en:dairy products,en:milk", want: "dairy products"},
		{name: "single key", categories: "milk", want: "milk"},
		{name: "language tagged single key", categories: "en:cheese", want: "cheese"},
		{name: "partial key", categories: "dairy", want: "dairy products"},
		{name: "dashed token", categories: "Fresh-Vegetables", want: "fresh vegetables"},
		{name: "no canonical category", categories: "en:snacks", want: ""},
		{name: "empty string", categories: "", want: ""},
		{name: "only separators", categories: ",,", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolver.ResolveKey(tc.categories)
			if !ok {
				if tc.want != "" {
					t.Fatalf("ResolveKey(%q) found no category, want %q", tc.categories, tc.want)
				}
				return
			}
			if got != tc.want {
				t.Errorf("ResolveKey(%q) = %q, want %q", tc.categories, got, tc.want)
			}
		})
	}
}

// The outcome must not depend on the order the canonical rows are read in.
func TestCategoryResolverIsIndependentOfRowOrder(t *testing.T) {
	rows := canonicalPriceRows()
	reversed := make([]dbModel.ProductCategoryPrice, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		reversed = append(reversed, rows[i])
	}

	forward := NewCategoryResolver(rows)
	backward := NewCategoryResolver(reversed)

	for _, categories := range []string{"dairy products, milk", "milk, dairy products", "rice, fresh vegetables", "cheese"} {
		forwardKey, forwardOK := forward.ResolveKey(categories)
		backwardKey, backwardOK := backward.ResolveKey(categories)
		if forwardOK != backwardOK {
			t.Fatalf("ResolveKey(%q) found a category in one row order only", categories)
		}
		if forwardKey != backwardKey {
			t.Errorf("ResolveKey(%q) = %q for one row order and %q for the other", categories, forwardKey, backwardKey)
		}
	}
}

// Two equally long keys match the same text: the lexicographically smaller
// one wins, every time.
func TestCategoryResolverBreaksTiesDeterministically(t *testing.T) {
	rows := []dbModel.ProductCategoryPrice{
		{CategoryKey: "zeta", DisplayName: "Zeta"},
		{CategoryKey: "alpha", DisplayName: "Alpha"},
	}
	resolver := NewCategoryResolver(rows)

	for range 5 {
		got, ok := resolver.ResolveKey("alpha")
		if !ok {
			t.Fatalf("ResolveKey(%q) found no category", "alpha")
		}
		if got != "alpha" {
			t.Fatalf("ResolveKey(%q) = %q, want alpha", "alpha", got)
		}
	}
}

// The resolver reads the canonical table once and answers every product from
// that snapshot.
func TestLoadCategoryResolverReadsTheCanonicalRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}

	resolver, err := LoadCategoryResolver(db)
	if err != nil {
		t.Fatalf("LoadCategoryResolver() error = %v", err)
	}
	if len(resolver.Keys()) != len(canonicalPriceRows()) {
		t.Errorf("Keys() returned %d keys, want %d", len(resolver.Keys()), len(canonicalPriceRows()))
	}

	canonical, ok := resolver.Canonical("milk")
	if !ok {
		t.Fatal("Canonical(milk) found nothing")
	}
	if canonical.AvgPriceEUR != 0.90 {
		t.Errorf("Canonical(milk).AvgPriceEUR = %v, want 0.90", canonical.AvgPriceEUR)
	}
}
