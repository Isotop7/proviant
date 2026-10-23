package database

import (
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// The records written before the count model are not comparable with the ones
// after it: their figures were scaled by the net content of the product, so a
// litre of milk was recorded as thousands of euros. They stay out of the
// totals, and their number is reported so the gap can be explained.
func TestGetSavingsStatsExcludesTheLegacyRecords(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	testutil.CreateTestUser(db, household.ID)

	records := []dbModel.SavingsRecord{
		{HouseholdID: household.ID, ProductID: 1, ProductName: "Milk", EventType: "consumed", PriceEUR: 0.9, CO2Kg: 1.6, Amount: 1},
		{HouseholdID: household.ID, ProductID: 2, ProductName: "Yogurt", EventType: "wasted", PriceEUR: 1.2, CO2Kg: 0.3, Amount: 1},
		{HouseholdID: household.ID, ProductID: 3, ProductName: "Old Milk", EventType: "consumed", PriceEUR: 900, CO2Kg: 3200, Amount: 1000, Legacy: true},
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	repo := NewSavingsRepository(db)
	stats, err := repo.GetSavingsStats(household.ID)
	if err != nil {
		t.Fatalf("GetSavingsStats() error = %v", err)
	}
	if stats.SavedEURLifetime != 0.9 {
		t.Errorf("saved EUR = %v, want 0.9 (the legacy record is not counted)", stats.SavedEURLifetime)
	}
	if stats.SavedCO2KgLifetime != 1.6 {
		t.Errorf("saved CO₂ = %v, want 1.6 (the legacy record is not counted)", stats.SavedCO2KgLifetime)
	}
	if stats.WastedEURLifetime != 1.2 {
		t.Errorf("wasted EUR = %v, want 1.2", stats.WastedEURLifetime)
	}
	if stats.LegacyEvents != 1 {
		t.Errorf("legacy events = %d, want 1", stats.LegacyEvents)
	}
}
