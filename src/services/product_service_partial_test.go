package services

import (
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"gorm.io/gorm"
)

func setupPartialService(t *testing.T, quantity int) (*gorm.DB, *ProductService, uint, uint, *dbModel.Product) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	svc := newTestProductService(db)
	household := testutil.CreateTestHousehold(db, 1)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)
	if err := db.Model(product).Updates(map[string]any{"quantity": quantity, "price_override": 2.5}).Error; err != nil {
		t.Fatalf("set product stock and price: %v", err)
	}
	return db, svc, household.ID, user.ID, product
}

// TestProductService_PartialRemovalAccounting verifies that activity and savings count only the removed packs.
func TestProductService_PartialRemovalAccounting(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		eventType string
		remove    func(*ProductService, uint, uint, int, *string, string) (int, error)
	}{
		{name: "consume", action: dbModel.ActivityActionConsume, eventType: "consumed", remove: func(s *ProductService, productID, userID uint, quantity int, mutationID *string, _ string) (int, error) {
			return s.ConsumeProductPartial(productID, userID, quantity, mutationID)
		}},
		{name: "waste", action: dbModel.ActivityActionWaste, eventType: "wasted", remove: (*ProductService).WasteProductPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, svc, householdID, userID, product := setupPartialService(t, 3)

			removed, err := tt.remove(svc, product.ID, userID, 2, nil, "")
			if err != nil {
				t.Fatalf("partial removal error = %v", err)
			}
			if removed != 2 {
				t.Errorf("removed = %d, want 2", removed)
			}
			var active dbModel.Product
			if err := db.First(&active, product.ID).Error; err != nil {
				t.Fatalf("reload active product: %v", err)
			}
			if active.Quantity != 1 || active.DeletedAt.Valid || active.RemovalReason != "" {
				t.Errorf("remaining stock = (Quantity %d, DeletedAt %v, reason %q), want (1, false, empty)", active.Quantity, active.DeletedAt.Valid, active.RemovalReason)
			}

			logs := waitForActivityLogs(t, db, householdID, 1)
			if logs[0].Action != tt.action || logs[0].Quantity != 2 {
				t.Errorf("activity = (Action %q, Quantity %d), want (%q, 2)", logs[0].Action, logs[0].Quantity, tt.action)
			}
			waitForSavingsEvents(t, db, householdID, 1)
			var record dbModel.SavingsRecord
			if err := db.Where("product_id = ?", product.ID).First(&record).Error; err != nil {
				t.Fatalf("load savings record: %v", err)
			}
			if record.EventType != tt.eventType || record.Amount != 2 || record.PriceEUR != 5 {
				t.Errorf("savings = (EventType %q, Amount %d, PriceEUR %v), want (%q, 2, 5)", record.EventType, record.Amount, record.PriceEUR, tt.eventType)
			}
			if tt.action == dbModel.ActivityActionWaste {
				var streak dbModel.WasteStreak
				if err := db.Where("household_id = ?", householdID).First(&streak).Error; err != nil {
					t.Fatalf("load waste streak: %v", err)
				}
				if streak.LastWastedDate == nil {
					t.Error("partial waste did not record a waste event")
				}
			}
		})
	}
}

// TestProductService_FullRemovalAccounting verifies that legacy whole-stock calls record their actual pack count.
func TestProductService_FullRemovalAccounting(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		eventType string
		reason    string
		remove    func(*ProductService, uint, uint, string) error
	}{
		{name: "consume", action: dbModel.ActivityActionConsume, eventType: "consumed", reason: dbModel.RemovalReasonConsumed, remove: func(s *ProductService, productID, userID uint, _ string) error {
			return s.ConsumeProduct(productID, userID)
		}},
		{name: "waste", action: dbModel.ActivityActionWaste, eventType: "wasted", reason: dbModel.RemovalReasonWasted, remove: (*ProductService).WasteProduct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, svc, householdID, userID, product := setupPartialService(t, 3)

			if err := tt.remove(svc, product.ID, userID, ""); err != nil {
				t.Fatalf("full removal error = %v", err)
			}
			var archived dbModel.Product
			if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
				t.Fatalf("reload archived product: %v", err)
			}
			if !archived.DeletedAt.Valid || archived.RemovalReason != tt.reason {
				t.Errorf("archive state = (DeletedAt %v, reason %q), want (true, %q)", archived.DeletedAt.Valid, archived.RemovalReason, tt.reason)
			}

			logs := waitForActivityLogs(t, db, householdID, 1)
			if logs[0].Action != tt.action || logs[0].Quantity != 3 {
				t.Errorf("activity = (Action %q, Quantity %d), want (%q, 3)", logs[0].Action, logs[0].Quantity, tt.action)
			}
			waitForSavingsEvents(t, db, householdID, 1)
			var record dbModel.SavingsRecord
			if err := db.Where("product_id = ?", product.ID).First(&record).Error; err != nil {
				t.Fatalf("load savings record: %v", err)
			}
			if record.EventType != tt.eventType || record.Amount != 3 || record.PriceEUR != 7.5 {
				t.Errorf("savings = (EventType %q, Amount %d, PriceEUR %v), want (%q, 3, 7.5)", record.EventType, record.Amount, record.PriceEUR, tt.eventType)
			}
		})
	}
}

// TestProductService_PartialRemovalReplay verifies that a replayed mutation writes no duplicate activity or savings.
func TestProductService_PartialRemovalReplay(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*ProductService, uint, uint, int, *string, string) (int, error)
	}{
		{name: "consume", remove: func(s *ProductService, productID, userID uint, quantity int, mutationID *string, _ string) (int, error) {
			return s.ConsumeProductPartial(productID, userID, quantity, mutationID)
		}},
		{name: "waste", remove: (*ProductService).WasteProductPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, svc, householdID, userID, product := setupPartialService(t, 3)
			mutationID := "offline-partial-action"

			if _, err := tt.remove(svc, product.ID, userID, 1, &mutationID, ""); err != nil {
				t.Fatalf("first removal error = %v", err)
			}
			waitForActivityLogs(t, db, householdID, 1)
			waitForSavingsEvents(t, db, householdID, 1)
			removed, err := tt.remove(svc, product.ID, userID, 1, &mutationID, "")
			if err != nil {
				t.Fatalf("replayed removal error = %v", err)
			}
			if removed != 0 {
				t.Errorf("removed after replay = %d, want 0", removed)
			}

			var activityCount int64
			if err := db.Model(&dbModel.ActivityLog{}).Where("household_id = ?", householdID).Count(&activityCount).Error; err != nil {
				t.Fatalf("count activity logs: %v", err)
			}
			var savingsCount int64
			if err := db.Model(&dbModel.SavingsRecord{}).Where("household_id = ?", householdID).Count(&savingsCount).Error; err != nil {
				t.Fatalf("count savings records: %v", err)
			}
			if activityCount != 1 || savingsCount != 1 {
				t.Errorf("records after replay = (activity %d, savings %d), want (1, 1)", activityCount, savingsCount)
			}
			var active dbModel.Product
			if err := db.First(&active, product.ID).Error; err != nil {
				t.Fatalf("reload active product: %v", err)
			}
			if active.Quantity != 2 {
				t.Errorf("Quantity after replay = %d, want 2", active.Quantity)
			}
		})
	}
}

// TestProductService_WasteRecordsItsCause verifies that the reason a waste
// happened reaches the ledger, the activity log and the archived product.
func TestProductService_WasteRecordsItsCause(t *testing.T) {
	db, svc, householdID, userID, product := setupPartialService(t, 2)

	if _, err := svc.WasteProductPartial(product.ID, userID, 0, nil, dbModel.WasteCauseOverbought); err != nil {
		t.Fatalf("WasteProductPartial() error = %v", err)
	}

	logs := waitForActivityLogs(t, db, householdID, 1)
	if logs[0].WasteCause != dbModel.WasteCauseOverbought {
		t.Errorf("activity cause = %q, want %q", logs[0].WasteCause, dbModel.WasteCauseOverbought)
	}

	waitForSavingsEvents(t, db, householdID, 1)
	var record dbModel.SavingsRecord
	if err := db.Where("product_id = ?", product.ID).First(&record).Error; err != nil {
		t.Fatalf("load savings record: %v", err)
	}
	if record.EventType != dbModel.SavingsEventTypeWasted {
		t.Errorf("savings event type = %q, want %q", record.EventType, dbModel.SavingsEventTypeWasted)
	}
	if record.WasteCause != dbModel.WasteCauseOverbought {
		t.Errorf("savings cause = %q, want %q", record.WasteCause, dbModel.WasteCauseOverbought)
	}

	var archived dbModel.Product
	if err := db.Unscoped().First(&archived, product.ID).Error; err != nil {
		t.Fatalf("load archived product: %v", err)
	}
	if archived.RemovalReason != dbModel.RemovalReasonWasted {
		t.Errorf("RemovalReason = %q, want %q", archived.RemovalReason, dbModel.RemovalReasonWasted)
	}
	if archived.WasteCause != dbModel.WasteCauseOverbought {
		t.Errorf("archived cause = %q, want %q", archived.WasteCause, dbModel.WasteCauseOverbought)
	}
}

// TestProductService_BulkWasteRecordsItsCause verifies that a bulk waste writes
// the reason to every product of the batch.
func TestProductService_BulkWasteRecordsItsCause(t *testing.T) {
	db, svc, householdID, userID, product := setupPartialService(t, 2)
	second := testutil.CreateTestProduct(db, householdID, userID)

	if _, err := svc.BulkWasteProducts([]uint{product.ID, second.ID}, userID, dbModel.WasteCauseSpoiled); err != nil {
		t.Fatalf("BulkWasteProducts() error = %v", err)
	}

	waitForSavingsEvents(t, db, householdID, 2)
	var records []dbModel.SavingsRecord
	if err := db.Where("household_id = ?", householdID).Find(&records).Error; err != nil {
		t.Fatalf("load savings records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("savings records = %d, want 2", len(records))
	}
	for _, record := range records {
		if record.WasteCause != dbModel.WasteCauseSpoiled {
			t.Errorf("savings cause = %q, want %q", record.WasteCause, dbModel.WasteCauseSpoiled)
		}
	}

	var archived []dbModel.Product
	if err := db.Unscoped().Where("household_id = ? AND deleted_at IS NOT NULL", householdID).Find(&archived).Error; err != nil {
		t.Fatalf("load archived products: %v", err)
	}
	if len(archived) != 2 {
		t.Fatalf("archived products = %d, want 2", len(archived))
	}
	for _, row := range archived {
		if row.WasteCause != dbModel.WasteCauseSpoiled {
			t.Errorf("archived cause = %q, want %q", row.WasteCause, dbModel.WasteCauseSpoiled)
		}
	}
}
