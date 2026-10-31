package database

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// assertConsecutiveMonths checks that the buckets name consecutive months,
// each one exactly once. A bucket list built with time.AddDate skips a month
// or names one twice whenever the anchor day does not exist in a neighbouring
// month, so the list itself is the thing under test.
func assertConsecutiveMonths(t *testing.T, keys []string) {
	t.Helper()
	for i := 1; i < len(keys); i++ {
		previous, err := time.Parse(util.DefaultDateFormatMonthStr, keys[i-1])
		if err != nil {
			t.Fatalf("month key %q: %v", keys[i-1], err)
		}
		current, err := time.Parse(util.DefaultDateFormatMonthStr, keys[i])
		if err != nil {
			t.Fatalf("month key %q: %v", keys[i], err)
		}
		want := previous.AddDate(0, 1, 0)
		if !current.Equal(want) {
			t.Errorf("month %q follows %q, want %q", keys[i], keys[i-1], want.Format(util.DefaultDateFormatMonthStr))
		}
	}
}

// The expiry trend buckets the next twelve months. The list has to name every
// month of the window exactly once, otherwise a product that expires in a
// skipped month vanishes from the trend.
func TestGetExpiryTrendBucketsTwelveConsecutiveMonths(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	// The 31st is the day the month arithmetic breaks on, so the anchor is
	// pinned to one: the assertion has to hold on every day of the year. The
	// anchor sits in the container's zone, because the trend buckets the
	// calendar months of that zone.
	anchor := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)

	// The product expires in the calendar month after the one the trend
	// starts in.
	monthStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.Local)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	product.ExpireAt = monthStart.AddDate(0, 1, 14)
	if err := db.Save(product).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	trend, err := repo.GetExpiryTrend(user.ID, anchor)
	if err != nil {
		t.Fatalf("GetExpiryTrend: %v", err)
	}
	if len(trend) != 12 {
		t.Fatalf("expiry trend has %d months, want 12", len(trend))
	}

	keys := make([]string, len(trend))
	for i := range trend {
		keys[i] = trend[i].Month
	}
	assertConsecutiveMonths(t, keys)

	if trend[0].Count != 0 {
		t.Errorf("first month count = %d, want 0", trend[0].Count)
	}
	if trend[1].Count != 1 {
		t.Errorf("second month count = %d, want 1", trend[1].Count)
	}

	// The anchor picks the window, not the wall clock: one calendar month
	// later the product moves into the first bucket.
	laterAnchor := time.Date(2026, time.November, 30, 12, 0, 0, 0, time.Local)
	later, err := repo.GetExpiryTrend(user.ID, laterAnchor)
	if err != nil {
		t.Fatalf("GetExpiryTrend: %v", err)
	}
	if later[0].Count != 1 {
		t.Errorf("first month of the later window = %d, want 1", later[0].Count)
	}
	if later[1].Count != 0 {
		t.Errorf("second month of the later window = %d, want 0", later[1].Count)
	}
}
