package database

import (
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
)

func TestSortByEffectiveExpiryZeroHandling(t *testing.T) {
	dated := dbModel.Product{ExpireAt: time.Now().Add(24 * time.Hour)}
	undated := dbModel.Product{}

	products := []dbModel.Product{undated, dated}
	SortByEffectiveExpiry(products)
	if !products[0].ExpireAt.Equal(dated.ExpireAt) || !products[1].ExpireAt.IsZero() {
		t.Error("SortByEffectiveExpiry() = dated product first, undated last")
	}

	bothUndated := []dbModel.Product{{}, {}}
	SortByEffectiveExpiry(bothUndated) // must not panic on all-zero expiries
}
