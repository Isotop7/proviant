package migrations

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The savings records written before the count model are flagged at the
// cutover, so the aggregates leave them out: their figures were scaled by the
// net content of the product rather than by a count of packs.
func TestMarkLegacySavingsRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	now := time.Now()
	record := database.SavingsRecord{
		Model:       gorm.Model{CreatedAt: now},
		HouseholdID: 1,
		ProductID:   1,
		ProductName: "Milk",
		EventType:   "consumed",
		PriceEUR:    900,
		CO2Kg:       3200,
		Amount:      1000,
	}
	if createErr := db.Create(&record).Error; createErr != nil {
		t.Fatalf("Failed to seed the record: %v", createErr)
	}

	logger := zerolog.Nop()
	if markErr := MarkLegacySavingsRecords(&logger, db); markErr != nil {
		t.Fatalf("MarkLegacySavingsRecords() error = %v", markErr)
	}

	var reloaded database.SavingsRecord
	if readErr := db.First(&reloaded, record.ID).Error; readErr != nil {
		t.Fatalf("Failed to read the record: %v", readErr)
	}
	if !reloaded.Legacy {
		t.Errorf("the record written before the count model is not flagged as legacy")
	}

	var cutover database.ModelCutover
	if readErr := db.Where("name = ?", util.CountModelCutover).First(&cutover).Error; readErr != nil {
		t.Fatalf("Failed to read the cutover: %v", readErr)
	}
	if cutover.CutoverAt.IsZero() {
		t.Errorf("the cutover has no timestamp")
	}

	// A record written after the cutover stays on the current scale.
	current := database.SavingsRecord{
		Model:       gorm.Model{CreatedAt: now.Add(time.Minute)},
		HouseholdID: 1,
		ProductID:   2,
		ProductName: "Yogurt",
		EventType:   "consumed",
		PriceEUR:    1.2,
		CO2Kg:       0.3,
		Amount:      1,
	}
	if createErr := db.Create(&current).Error; createErr != nil {
		t.Fatalf("Failed to seed the record: %v", createErr)
	}
	if markErr := MarkLegacySavingsRecords(&logger, db); markErr != nil {
		t.Fatalf("MarkLegacySavingsRecords() error = %v", markErr)
	}
	if readErr := db.First(&current, current.ID).Error; readErr != nil {
		t.Fatalf("Failed to read the record: %v", readErr)
	}
	if current.Legacy {
		t.Errorf("a record written after the cutover is flagged as legacy")
	}
}
