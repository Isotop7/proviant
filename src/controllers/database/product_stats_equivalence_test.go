package database

import (
	"sort"
	"strings"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// TestStatsQueriesMatchFullTableComputation pins every stats query to the
// full-table computation it replaced. The endpoint used to load every active
// row five times and every archived row twice; the replacements prune in SQL
// and must land on exactly the same numbers.
func TestStatsQueriesMatchFullTableComputation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	privateOwner := testutil.CreateTestUser(db, household.ID)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	otherUser := testutil.CreateTestUser(db, otherHousehold.ID)

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

	// Expired, critical, fresh, undated, opened-shelf-life-expired.
	create(inHousehold(database.Product{ProductName: "Expired", Categories: "en:dairy", ExpireAt: now.AddDate(0, 0, -2)}))
	location := testutil.CreateTestStorageLocation(db, household.ID)
	create(inHousehold(database.Product{ProductName: "Soon", Categories: "en:bakery", ExpireAt: now.AddDate(0, 0, 1),
		Quantity: 2, PackSize: "250g", StorageLocationID: &location.ID}))
	create(inHousehold(database.Product{ProductName: "Fresh", Categories: "en:produce", ExpireAt: now.AddDate(0, 0, 30)}))
	create(inHousehold(database.Product{ProductName: "NoDate"}))
	openedAt := now.AddDate(0, 0, -10)
	daysAfterOpening := 5
	create(inHousehold(database.Product{
		ProductName:      "OpenedExpired",
		Categories:       "en:meat",
		ExpireAt:         now.AddDate(0, 0, 30),
		OpenedAt:         &openedAt,
		DaysAfterOpening: &daysAfterOpening,
	}))

	// Another member's private product: invisible through the privacy scope.
	create(database.Product{
		ProductName: "PrivateOther",
		HouseholdID: household.ID,
		UserID:      privateOwner.ID,
		IsPrivate:   true,
		ExpireAt:    now.AddDate(0, 0, -3),
	})

	// Another household's product: must not appear in any count.
	create(database.Product{
		ProductName: "OtherHousehold",
		HouseholdID: otherHousehold.ID,
		UserID:      otherUser.ID,
		ExpireAt:    now.AddDate(0, 0, -3),
	})

	// Archived: two sharing a barcode, one without one, plus an active product
	// so "archived" and "unique archived" differ from the total.
	archived := []database.Product{
		inHousehold(database.Product{ProductName: "ArchA", Barcode: "111", Categories: "en:dairy"}),
		inHousehold(database.Product{ProductName: "ArchB", Barcode: "111", Categories: "en:dairy"}),
		inHousehold(database.Product{ProductName: "ArchNoBarcode", Categories: ""}),
	}
	for _, p := range archived {
		created := create(p)
		if err := db.Delete(&created).Error; err != nil {
			t.Fatalf("failed to archive product: %v", err)
		}
	}

	// --- Reference: the full-table computation the endpoint used to run. ---

	fullRows, err := repo.GetUserProductsBulk(user.ID, 0)
	if err != nil {
		t.Fatalf("GetUserProductsBulk() error = %v", err)
	}

	wantActive := len(fullRows)

	wantExpired := 0
	for i := range fullRows {
		eff := fullRows[i].EffectiveExpireAt()
		if eff.IsZero() {
			continue
		}
		if eff.Before(now) {
			wantExpired++
		}
	}

	wantCategories := map[string]int{}
	for i := range fullRows {
		raw := strings.TrimSpace(fullRows[i].Categories)
		if raw == "" {
			wantCategories["Uncategorized"]++
			continue
		}
		first := strings.SplitN(raw, ",", 2)[0]
		first = strings.TrimSpace(first)
		if idx := strings.Index(first, ":"); idx != -1 {
			first = strings.TrimSpace(first[idx+1:])
		}
		if first == "" {
			first = "Uncategorized"
		}
		wantCategories[first]++
	}

	wantTrend := []apiModel.StatsMonthlyCount{}
	// The month window is spelled out here instead of reusing the helper the
	// production code calls: a bug in the helper would otherwise cancel out on
	// both sides of this comparison. The trend buckets the calendar months of
	// the container's zone.
	local := now.In(time.Local)
	monthKeys := make([]string, 12)
	for i := range 12 {
		monthKeys[i] = time.Date(local.Year(), local.Month()+time.Month(i), 1, 0, 0, 0, 0, time.Local).Format(util.DefaultDateFormatMonthStr)
	}
	monthCounts := map[string]int{}
	for _, key := range monthKeys {
		monthCounts[key] = 0
	}
	for i := range fullRows {
		expiry := fullRows[i].EffectiveExpireAt()
		if expiry.IsZero() {
			continue
		}
		month := expiry.In(time.Local).Format(util.DefaultDateFormatMonthStr)
		if _, ok := monthCounts[month]; ok {
			monthCounts[month]++
		}
	}
	for _, key := range monthKeys {
		wantTrend = append(wantTrend, apiModel.StatsMonthlyCount{Month: key, Count: monthCounts[key]})
	}

	soonStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	soonEnd := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 999999999, now.Location())
	wantExpiringSoon := []apiModel.StatsExpiringProduct{}
	for i := range fullRows {
		expiry := fullRows[i].EffectiveExpireAt()
		if expiry.IsZero() {
			continue
		}
		if !expiry.Before(soonStart) && !expiry.After(soonEnd) {
			entry := apiModel.StatsExpiringProduct{
				ID:          fullRows[i].ID,
				ProductName: fullRows[i].ProductName,
				ExpireAt:    expiry.Format(util.DefaultDateFormatParseStr),
				Quantity:    fullRows[i].Quantity,
				PackSize:    fullRows[i].PackSize,
			}
			if fullRows[i].StorageLocation != nil {
				entry.StorageLocationID = &fullRows[i].StorageLocation.ID
				entry.StorageLocationName = fullRows[i].StorageLocation.Name
				entry.StorageLocationIcon = fullRows[i].StorageLocation.Icon
			}
			wantExpiringSoon = append(wantExpiringSoon, entry)
		}
	}
	sort.Slice(wantExpiringSoon, func(i, j int) bool {
		return wantExpiringSoon[i].ExpireAt < wantExpiringSoon[j].ExpireAt
	})

	archivedRows, err := repo.GetUserArchivedProductsBulk(user.ID, 0, 0)
	if err != nil {
		t.Fatalf("GetUserArchivedProductsBulk() error = %v", err)
	}
	wantArchived := len(archivedRows)
	wantUniqueArchived := map[string]int{}
	for i := range archivedRows {
		wantUniqueArchived[archivedRows[i].Barcode]++
	}

	// --- The replacement queries. ---

	if got, err := repo.GetActiveProductsCount(user.ID); err != nil || got != wantActive {
		t.Errorf("GetActiveProductsCount() = %d, %v; want %d", got, err, wantActive)
	}
	if got, err := repo.GetExpiredProductsCount(user.ID, now); err != nil || got != wantExpired {
		t.Errorf("GetExpiredProductsCount() = %d, %v; want %d", got, err, wantExpired)
	}
	if got, err := repo.GetArchivedProductsCount(user.ID); err != nil || got != wantArchived {
		t.Errorf("GetArchivedProductsCount() = %d, %v; want %d", got, err, wantArchived)
	}
	if got, err := repo.GetUniqueArchivedProductsCount(user.ID); err != nil || got != len(wantUniqueArchived) {
		t.Errorf("GetUniqueArchivedProductsCount() = %d, %v; want %d", got, err, len(wantUniqueArchived))
	}
	if got, err := repo.GetProductCategoryBreakdown(user.ID); err != nil || !categoryMapsEqual(got, wantCategories) {
		t.Errorf("GetProductCategoryBreakdown() = %v, %v; want %v", got, err, wantCategories)
	}
	if got, err := repo.GetExpiryTrend(user.ID, now); err != nil || !trendEqual(got, wantTrend) {
		t.Errorf("GetExpiryTrend() = %+v, %v; want %+v", got, err, wantTrend)
	}
	if got, err := repo.GetExpiringSoonProducts(user.ID, 7, now); err != nil || !expiringSoonEqual(got, wantExpiringSoon) {
		t.Errorf("GetExpiringSoonProducts() = %+v, %v; want %+v", got, err, wantExpiringSoon)
	}
	// The list carries the place and the pack count of every product, so a
	// household can act on it without opening the product.
	if got, err := repo.GetExpiringSoonProducts(user.ID, 7, now); err != nil {
		t.Fatalf("GetExpiringSoonProducts() error = %v", err)
	} else if len(got) != 1 {
		t.Fatalf("GetExpiringSoonProducts() = %+v, want the one product in the window", got)
	} else {
		entry := got[0]
		if entry.ID == 0 {
			t.Errorf("id = 0, want the product the row leads to")
		}
		if entry.StorageLocationName != location.Name || entry.StorageLocationIcon != location.Icon {
			t.Errorf("location = %q/%q, want %q/%q",
				entry.StorageLocationName, entry.StorageLocationIcon, location.Name, location.Icon)
		}
		if entry.Quantity != 2 || entry.PackSize != "250g" {
			t.Errorf("quantity/packSize = %d/%q, want 2/250g", entry.Quantity, entry.PackSize)
		}
	}
}

func categoryMapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func trendEqual(a, b []apiModel.StatsMonthlyCount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func expiringSoonEqual(a, b []apiModel.StatsExpiringProduct) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		// The location id is a pointer, so the two entries carry different
		// pointers to the same row: the values are what has to match.
		if a[i].ID != b[i].ID ||
			a[i].ProductName != b[i].ProductName ||
			a[i].ExpireAt != b[i].ExpireAt ||
			a[i].Quantity != b[i].Quantity ||
			a[i].PackSize != b[i].PackSize ||
			a[i].StorageLocationName != b[i].StorageLocationName ||
			a[i].StorageLocationIcon != b[i].StorageLocationIcon {
			return false
		}
	}
	return true
}
