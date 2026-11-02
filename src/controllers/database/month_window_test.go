package database

import (
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

// The dashboard's waste-this-month number and the monthly waste report count
// the same month. Both measure it in the container's zone, so an anchor that
// sits in another calendar month names the same period for both.
func TestWasteThisMonthAndTheMonthlyReportCountTheSameMonth(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productRepo := NewProductRepository(db)
	notificationRepo := NewNotificationRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	// Half past midnight in a zone two hours ahead of UTC: the instant sits in
	// two different calendar months, so a window that reads the zone of the
	// value instead of the container's picks the wrong one.
	anchor := time.Date(2026, time.November, 1, 0, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	container := anchor.In(time.Local)

	archive := func(deletedAt time.Time) {
		t.Helper()
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.RemovalReason = dbModel.RemovalReasonWasted
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		if err := db.Delete(product).Error; err != nil {
			t.Fatalf("delete product: %v", err)
		}
		if err := db.Unscoped().Model(&dbModel.Product{}).Where("id = ?", product.ID).
			Update("deleted_at", deletedAt).Error; err != nil {
			t.Fatalf("pin the deletion: %v", err)
		}
	}

	archive(time.Date(container.Year(), container.Month(), 20, 12, 0, 0, 0, time.Local))
	archive(time.Date(container.Year(), container.Month()+1, 20, 12, 0, 0, 0, time.Local))

	count, err := productRepo.GetWasteThisMonth(user.ID, anchor)
	if err != nil {
		t.Fatalf("GetWasteThisMonth: %v", err)
	}
	if count != 1 {
		t.Errorf("GetWasteThisMonth = %d, want 1", count)
	}

	stats, err := notificationRepo.GetWasteStatsForHousehold(household.ID, anchor)
	if err != nil {
		t.Fatalf("GetWasteStatsForHousehold: %v", err)
	}
	if stats.DeletedCount != 1 {
		t.Errorf("report deleted count = %d, want 1", stats.DeletedCount)
	}
	wantMonth := time.Date(container.Year(), container.Month(), 1, 0, 0, 0, 0, time.Local)
	if !stats.Month.Equal(wantMonth) {
		t.Errorf("report month = %v, want %v", stats.Month, wantMonth)
	}
}
