// Demo seed script - generates demo_seed.db for public demo instance
// Usage: go run seed.go
package main

import (
	"fmt"
	"os"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var models = []any{
	&dbModel.Household{},
	&dbModel.StorageLocation{},
	&authentication.User{},
	&authentication.RevokedToken{},
	&authentication.PersonalAccessToken{},
	&authentication.CalendarToken{},
	&dbModel.Product{},
	&dbModel.HouseholdApplication{},
	&dbModel.HouseholdInvitation{},
	&dbModel.OnboardingState{},
	&dbModel.OpenFoodFactsCache{},
	&dbModel.RecipeCache{},
	&dbModel.EmailVerification{},
	&dbModel.Webhook{},
	&dbModel.WebhookDeliveryLog{},
	&dbModel.WasteStreak{},
	&dbModel.ProductCategoryPrice{},
	&dbModel.SavingsRecord{},
	&dbModel.ExpiryScan{},
	&dbModel.AuditLog{},
	&dbModel.ActivityLog{},
	&dbModel.ShoppingListItem{},
	&dbModel.MailDigestUnsubscribeToken{},
}

func main() {
	os.Remove("demo_seed.db")

	db, err := gorm.Open(sqlite.Open("demo_seed.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}

	if err = db.AutoMigrate(models...); err != nil {
		panic("failed to migrate database: " + err.Error())
	}

	seedDemoData(db)

	fmt.Println("demo_seed.db created successfully")
}

func hashPassword(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}

func seedDemoData(db *gorm.DB) {
	now := time.Now()
	verifiedAt := now.Add(-time.Hour)

	household := dbModel.Household{
		Model:   gorm.Model{ID: 1},
		Name:    "Demo Household",
		AdminID: 1,
	}
	if err := db.Create(&household).Error; err != nil {
		panic(err)
	}

	locations := []dbModel.StorageLocation{
		{Model: gorm.Model{ID: 1}, HouseholdID: 1, Name: "Fridge", Icon: "🧊", SortOrder: 0},
		{Model: gorm.Model{ID: 2}, HouseholdID: 1, Name: "Freezer", Icon: "❄️", SortOrder: 1},
		{Model: gorm.Model{ID: 3}, HouseholdID: 1, Name: "Pantry", Icon: "🏪", SortOrder: 2},
	}
	for _, loc := range locations {
		if err := db.Create(&loc).Error; err != nil {
			panic(err)
		}
	}

	users := []authentication.User{
		{
			Model:           gorm.Model{ID: 1},
			Username:        "demo",
			DisplayName:     "Demo User",
			MailAddress:     "demo@proviant.app",
			Password:        hashPassword("demo"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:     1,
		},
		{
			Model:           gorm.Model{ID: 2},
			Username:        "alice",
			DisplayName:     "Alice Smith",
			MailAddress:     "alice@example.com",
			Password:        hashPassword("AliceDemo123!"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:     1,
		},
		{
			Model:           gorm.Model{ID: 3},
			Username:        "bob",
			DisplayName:     "Bob Johnson",
			MailAddress:     "bob@example.com",
			Password:        hashPassword("BobDemo123!"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:     1,
		},
	}
	for _, user := range users {
		if err := db.Create(&user).Error; err != nil {
			panic(err)
		}
	}

	products := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Milk", Barcode: "8710398018520", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 2), ScannedAt: now.AddDate(0, 0, -5), Amount: 1, Unit: "L", Categories: "en:dairy-products,en:milk", MinStockAmount: 3},
		{Model: gorm.Model{ID: 2}, ProductName: "Yogurt", Barcode: "8710398501466", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 5), ScannedAt: now.AddDate(0, 0, -3), Amount: 4, Unit: "pcs", Categories: "en:dairy-products,en:yoghurts"},
		{Model: gorm.Model{ID: 3}, ProductName: "Eggs", Barcode: "8710385123457", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 10), ScannedAt: now.AddDate(0, 0, -7), Amount: 12, Unit: "pcs", Categories: "en:eggs", MinStockAmount: 24},
		{Model: gorm.Model{ID: 4}, ProductName: "Butter", Barcode: "8710398512349", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 14), ScannedAt: now.AddDate(0, 0, -2), Amount: 1, Unit: "pkg", Categories: "en:dairy-products"},
		{Model: gorm.Model{ID: 5}, ProductName: "Chicken Breast", Barcode: "8710385123464", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, 1), ScannedAt: now.AddDate(0, 0, -3), Amount: 500, Unit: "g", Categories: "en:meats,en:poultry"},
		{Model: gorm.Model{ID: 6}, ProductName: "Ground Beef", Barcode: "8710385123471", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, 3), ScannedAt: now.AddDate(0, 0, -5), Amount: 400, Unit: "g", Categories: "en:meats,en:beef"},
		{Model: gorm.Model{ID: 7}, ProductName: "Salmon Fillet", Barcode: "8710385123488", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, -1), ScannedAt: now.AddDate(0, 0, -7), Amount: 300, Unit: "g", Categories: "en:fish"},
		{Model: gorm.Model{ID: 8}, ProductName: "Frozen Pizza", Barcode: "8076809513753", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -14), Amount: 2, Unit: "pcs", Categories: "en:frozen-foods"},
		{Model: gorm.Model{ID: 9}, ProductName: "Ice Cream", Barcode: "8076800195875", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 1, 15), ScannedAt: now.AddDate(0, 0, -30), Amount: 1, Unit: "L", Categories: "en:desserts"},
		{Model: gorm.Model{ID: 10}, ProductName: "Pasta", Barcode: "8076800195905", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(1, 0, 0), ScannedAt: now.AddDate(0, 0, -60), Amount: 3, Unit: "pkg", Categories: "en:pasta", MinStockAmount: 6},
		{Model: gorm.Model{ID: 11}, ProductName: "Rice", Barcode: "8410065012345", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -90), Amount: 2, Unit: "kg", Categories: "en:cereals,en:rice"},
		{Model: gorm.Model{ID: 12}, ProductName: "Tomato Sauce", Barcode: "8410065012352", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 2, 0), ScannedAt: now.AddDate(0, 0, -20), Amount: 2, Unit: "jars", Categories: "en:sauces"},
		{Model: gorm.Model{ID: 13}, ProductName: "Olive Oil", Barcode: "8410065012369", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(1, 0, 0), ScannedAt: now.AddDate(0, 0, -45), Amount: 1, Unit: "L", Categories: "en:oils"},
		{Model: gorm.Model{ID: 14}, ProductName: "Bread", Barcode: "3228027000016", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 1), ScannedAt: now.AddDate(0, 0, -2), Amount: 1, Unit: "loaf", Categories: "en:breads"},
		{Model: gorm.Model{ID: 15}, ProductName: "Cheese", Barcode: "3228027000023", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 7), ScannedAt: now.AddDate(0, 0, -4), Amount: 200, Unit: "g", Categories: "en:dairy-products,en:cheese"},
		{Model: gorm.Model{ID: 16}, ProductName: "Apple Juice", Barcode: "5449000000996", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -10), Amount: 2, Unit: "L", Categories: "en:soft-drinks"},
		{Model: gorm.Model{ID: 17}, ProductName: "Potatoes", Barcode: "5449000001238", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 20), ScannedAt: now.AddDate(0, 0, -15), Amount: 1, Unit: "kg", Categories: "en:vegetables,en:potatoes", MinStockAmount: 4},
		{Model: gorm.Model{ID: 18}, ProductName: "Onions", Barcode: "5449000001245", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 30), ScannedAt: now.AddDate(0, 0, -20), Amount: 500, Unit: "g", Categories: "en:vegetables"},
		{Model: gorm.Model{ID: 19}, ProductName: "Garlic", Barcode: "5449000001252", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -25), Amount: 3, Unit: "pcs", Categories: "en:vegetables"},
		{Model: gorm.Model{ID: 20}, ProductName: "Spinach", Barcode: "5449000001269", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 4), ScannedAt: now.AddDate(0, 0, -1), Amount: 200, Unit: "g", Categories: "en:vegetables,en:leafy-greens"},
		{Model: gorm.Model{ID: 21}, ProductName: "Ham", Barcode: "8410065012376", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 3), ScannedAt: now.AddDate(0, 0, -2), Amount: 100, Unit: "g", Categories: "en:meats,en:cured-meats"},
		{Model: gorm.Model{ID: 22}, ProductName: "Orange Juice", Barcode: "5449000001276", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 6), ScannedAt: now.AddDate(0, 0, -5), Amount: 1, Unit: "L", Categories: "en:soft-drinks"},
		{Model: gorm.Model{ID: 23}, ProductName: "Cereal", Barcode: "5000159484695", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 3, 0), ScannedAt: now.AddDate(0, 0, -40), Amount: 1, Unit: "box", Categories: "en:cereals"},
		{Model: gorm.Model{ID: 24}, ProductName: "Peanut Butter", Barcode: "5000159484701", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 4, 0), ScannedAt: now.AddDate(0, 0, -35), Amount: 1, Unit: "jar", Categories: "en:spreads"},
		{Model: gorm.Model{ID: 25}, ProductName: "Honey", Barcode: "5000159484718", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -50), Amount: 1, Unit: "jar", Categories: "en:spreads"},
		{Model: gorm.Model{ID: 26}, ProductName: "Coffee", Barcode: "5000159484725", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 2, 0), ScannedAt: now.AddDate(0, 0, -30), Amount: 200, Unit: "g", Categories: "en:beverages,en:coffee"},
		{Model: gorm.Model{ID: 27}, ProductName: "Tea", Barcode: "5000159484732", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -55), Amount: 50, Unit: "bags", Categories: "en:beverages,en:tea"},
		{Model: gorm.Model{ID: 28}, ProductName: "Sugar", Barcode: "5000159484749", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 12, 0), ScannedAt: now.AddDate(0, 0, -100), Amount: 1, Unit: "kg", Categories: "en:sweeteners"},
		{Model: gorm.Model{ID: 29}, ProductName: "Flour", Barcode: "5000159484756", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -70), Amount: 1, Unit: "kg", Categories: "en:cereals,en:flours"},
		{Model: gorm.Model{ID: 30}, ProductName: "Tofu", Barcode: "5000159484763", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 2), ScannedAt: now.AddDate(0, 0, -3), Amount: 400, Unit: "g", Categories: "en:plant-based-foods"},
	}
	for _, product := range products {
		if err := db.Create(&product).Error; err != nil {
			panic(err)
		}
	}

	seedConsumptionHistory(db, now, products)
	seedShoppingList(db, now)
	seedActivityAndStreak(db, now)
	seedPublicPageTokens(db, now)

	for i := 1; i <= 3; i++ {
		state := dbModel.OnboardingState{
			Model:  gorm.Model{ID: uint(i)},
			UserID: uint(i),
		}
		if err := db.Create(&state).Error; err != nil {
			panic(err)
		}
	}
}

// historyEvent is one past consume/waste action. Each event materialises three
// rows that the real handlers would also have written: the soft-deleted Product
// (what the Consumption History tab reads), the SavingsRecord (what the waste
// analytics aggregations sum) and — via seedActivityAndStreak — an activity entry.
//
// Name, barcode and category are looked up from the demo product catalog so the
// history rows cannot drift from the live products. Price and CO2 are written
// literally instead of derived from ProductCategoryPrice so the demo numbers
// stay stable regardless of what the price table is seeded with.
type historyEvent struct {
	id         uint
	name       string
	amount     int
	unit       string
	priceEUR   float64
	co2Kg      float64
	location   uint
	monthsAgo  int
	daysOffset int
	wasted     bool
}

// seedConsumptionHistory writes ~6 months of consume/waste events. Without it
// /web/waste-analytics renders "€0.00 wasted" and the Consumption History tab
// shows its empty state, which makes both features look broken in a screenshot.
func seedConsumptionHistory(db *gorm.DB, now time.Time, catalog []dbModel.Product) {
	productByName := make(map[string]dbModel.Product, len(catalog))
	for _, p := range catalog {
		productByName[p.ProductName] = p
	}

	events := []historyEvent{
		{id: 31, name: "Yogurt", amount: 4, unit: "pcs", priceEUR: 4.80, co2Kg: 0.88, location: 1, monthsAgo: 5, daysOffset: -9},
		{id: 32, name: "Spinach", amount: 200, unit: "g", priceEUR: 1.50, co2Kg: 0.08, location: 1, monthsAgo: 5, daysOffset: -2, wasted: true},
		{id: 33, name: "Milk", amount: 1, unit: "L", priceEUR: 0.90, co2Kg: 1.60, location: 1, monthsAgo: 5, daysOffset: -14},
		{id: 34, name: "Bread", amount: 1, unit: "loaf", priceEUR: 2.20, co2Kg: 0.70, location: 3, monthsAgo: 4, daysOffset: -6},
		{id: 35, name: "Ground Beef", amount: 400, unit: "g", priceEUR: 7.00, co2Kg: 6.80, location: 2, monthsAgo: 4, daysOffset: -18, wasted: true},
		{id: 36, name: "Pasta", amount: 3, unit: "pkg", priceEUR: 5.40, co2Kg: 1.65, location: 3, monthsAgo: 4, daysOffset: -23},
		{id: 37, name: "Cheese", amount: 200, unit: "g", priceEUR: 3.50, co2Kg: 1.70, location: 1, monthsAgo: 3, daysOffset: -4},
		{id: 38, name: "Orange Juice", amount: 1, unit: "L", priceEUR: 1.20, co2Kg: 0.18, location: 1, monthsAgo: 3, daysOffset: -11, wasted: true},
		{id: 39, name: "Chicken Breast", amount: 500, unit: "g", priceEUR: 5.50, co2Kg: 3.45, location: 2, monthsAgo: 3, daysOffset: -20},
		{id: 40, name: "Rice", amount: 2, unit: "kg", priceEUR: 4.00, co2Kg: 5.40, location: 3, monthsAgo: 2, daysOffset: -7},
		{id: 41, name: "Tomato Sauce", amount: 2, unit: "jars", priceEUR: 5.60, co2Kg: 1.26, location: 3, monthsAgo: 2, daysOffset: -15, wasted: true},
		{id: 42, name: "Eggs", amount: 12, unit: "pcs", priceEUR: 5.00, co2Kg: 2.16, location: 1, monthsAgo: 2, daysOffset: -26},
		{id: 43, name: "Ham", amount: 100, unit: "g", priceEUR: 1.75, co2Kg: 1.70, location: 1, monthsAgo: 1, daysOffset: -5},
		{id: 44, name: "Potatoes", amount: 1, unit: "kg", priceEUR: 1.00, co2Kg: 0.35, location: 3, monthsAgo: 1, daysOffset: -12, wasted: true},
		{id: 45, name: "Ice Cream", amount: 1, unit: "L", priceEUR: 3.00, co2Kg: 0.84, location: 2, monthsAgo: 1, daysOffset: -19},
		{id: 46, name: "Milk", amount: 1, unit: "L", priceEUR: 0.90, co2Kg: 1.60, location: 1, monthsAgo: 0, daysOffset: -3},
		{id: 47, name: "Spinach", amount: 200, unit: "g", priceEUR: 1.50, co2Kg: 0.08, location: 1, monthsAgo: 0, daysOffset: -6, wasted: true},
		{id: 48, name: "Bread", amount: 1, unit: "loaf", priceEUR: 2.20, co2Kg: 0.70, location: 3, monthsAgo: 0, daysOffset: -10},
		{id: 49, name: "Yogurt", amount: 4, unit: "pcs", priceEUR: 4.80, co2Kg: 0.88, location: 1, monthsAgo: 0, daysOffset: -16},
		{id: 50, name: "Salmon Fillet", amount: 300, unit: "g", priceEUR: 6.00, co2Kg: 1.62, location: 2, monthsAgo: 0, daysOffset: -21, wasted: true},
		// Two more Milk consumptions inside the 90-day rate window, >7 days apart.
		// With the events above, /web/products/1/view's add-to-shopping-list button
		// answers with a real per-week rate instead of the min-stock fallback.
		{id: 51, name: "Milk", amount: 1, unit: "L", priceEUR: 0.90, co2Kg: 1.60, location: 1, monthsAgo: 0, daysOffset: -52},
		{id: 52, name: "Milk", amount: 1, unit: "L", priceEUR: 0.90, co2Kg: 1.60, location: 1, monthsAgo: 0, daysOffset: -24},
	}

	for i := range events {
		event := &events[i]
		at := now.AddDate(0, -event.monthsAgo, event.daysOffset)

		source, ok := productByName[event.name]
		if !ok {
			panic(fmt.Sprintf("history event %q has no matching demo product", event.name))
		}

		reason := dbModel.RemovalReasonConsumed
		if event.wasted {
			reason = dbModel.RemovalReasonWasted
		}

		product := dbModel.Product{
			Model: gorm.Model{
				ID:        event.id,
				CreatedAt: at.AddDate(0, 0, -21),
				UpdatedAt: at,
				DeletedAt: gorm.DeletedAt{Time: at, Valid: true},
			},
			ProductName:       event.name,
			Barcode:           source.Barcode,
			Categories:        source.Categories,
			HouseholdID:       1,
			StorageLocationID: uintPtr(event.location),
			ExpireAt:          at,
			ScannedAt:         at.AddDate(0, 0, -21),
			Amount:            event.amount,
			Unit:              event.unit,
			RemovalReason:     reason,
		}
		if err := db.Create(&product).Error; err != nil {
			panic(err)
		}

		// GORM's Create drops a soft-delete timestamp set on the embedded
		// gorm.Model, so the archive marker is written explicitly afterwards.
		if err := db.Model(&product).UpdateColumn("deleted_at", at).Error; err != nil {
			panic(err)
		}

		record := dbModel.SavingsRecord{
			Model:       gorm.Model{CreatedAt: at, UpdatedAt: at},
			HouseholdID: 1,
			ProductID:   event.id,
			ProductName: event.name,
			EventType:   reason,
			PriceEUR:    event.priceEUR,
			CO2Kg:       event.co2Kg,
			Amount:      event.amount,
		}
		if err := db.Create(&record).Error; err != nil {
			panic(err)
		}
	}
}

// seedShoppingList gives the shopping list rows to show, including one checked so
// the strikethrough styling is visible, plus a note on one item.
func seedShoppingList(db *gorm.DB, now time.Time) {
	items := []dbModel.ShoppingListItem{
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -2)}, HouseholdID: 1, Name: "Oat milk", Category: "Dairy", Quantity: 2, Unit: "L", CreatedBy: 1},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -2)}, HouseholdID: 1, Name: "Free-range eggs", Category: "Dairy", Quantity: 12, Unit: "pcs", CreatedBy: 2},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -1)}, HouseholdID: 1, Name: "Penne pasta", Category: "Pantry", Quantity: 2, Unit: "pkg", CreatedBy: 1},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -1)}, HouseholdID: 1, Name: "Tomatoes", Category: "Produce", Quantity: 500, Unit: "g", Notes: "Vine-ripened if available", CreatedBy: 3},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -1)}, HouseholdID: 1, Name: "Coffee beans", Category: "Pantry", Quantity: 500, Unit: "g", Checked: true, CreatedBy: 1},
		{Model: gorm.Model{CreatedAt: now}, HouseholdID: 1, Name: "Dishwasher tablets", Category: "Household", Quantity: 1, Unit: "box", CreatedBy: 2},
	}
	for i := range items {
		if err := db.Create(&items[i]).Error; err != nil {
			panic(err)
		}
	}
}

// seedActivityAndStreak fills the dashboard's activity feed and waste-free streak
// tile, both of which are otherwise empty on a fresh demo database.
func seedActivityAndStreak(db *gorm.DB, now time.Time) {
	// The most recent waste event the activity feed below records is now-3d, so
	// the streak has to agree with it: three days without wasting anything.
	lastWasted := now.AddDate(0, 0, -3)

	streak := dbModel.WasteStreak{
		Model:           gorm.Model{CreatedAt: now.AddDate(0, -3, 0)},
		HouseholdID:     1,
		CurrentStreak:   3,
		LongestStreak:   19,
		LastCheckedDate: now,
		LastWastedDate:  &lastWasted,
	}
	if err := db.Create(&streak).Error; err != nil {
		panic(err)
	}

	activity := []dbModel.ActivityLog{
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -1)}, HouseholdID: 1, UserID: uintPtr(2), UserName: "Alice Smith", Action: dbModel.ActivityActionConsume, ProductID: 3, ProductName: "Eggs", Quantity: 12, Timestamp: now.AddDate(0, 0, -1)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -2)}, HouseholdID: 1, UserID: uintPtr(1), UserName: "Demo User", Action: dbModel.ActivityActionAdd, ProductID: 22, ProductName: "Orange Juice", Quantity: 1, Timestamp: now.AddDate(0, 0, -2)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -3)}, HouseholdID: 1, UserID: uintPtr(3), UserName: "Bob Johnson", Action: dbModel.ActivityActionWaste, ProductID: 20, ProductName: "Spinach", Quantity: 1, Timestamp: now.AddDate(0, 0, -3)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -5)}, HouseholdID: 1, UserID: uintPtr(1), UserName: "Demo User", Action: dbModel.ActivityActionCook, ProductID: 10, ProductName: "Pasta", Quantity: 3, Timestamp: now.AddDate(0, 0, -5)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -6)}, HouseholdID: 1, UserID: uintPtr(2), UserName: "Alice Smith", Action: dbModel.ActivityActionWaste, ProductID: 7, ProductName: "Salmon Fillet", Quantity: 1, Timestamp: now.AddDate(0, 0, -6)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -8)}, HouseholdID: 1, UserID: uintPtr(1), UserName: "Demo User", Action: dbModel.ActivityActionAmountChange, ProductID: 26, ProductName: "Coffee", Quantity: 200, Timestamp: now.AddDate(0, 0, -8)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -11)}, HouseholdID: 1, UserID: uintPtr(3), UserName: "Bob Johnson", Action: dbModel.ActivityActionConsume, ProductID: 18, ProductName: "Onions", Quantity: 500, Timestamp: now.AddDate(0, 0, -11)},
		{Model: gorm.Model{CreatedAt: now.AddDate(0, 0, -14)}, HouseholdID: 1, UserID: uintPtr(1), UserName: "Demo User", Action: dbModel.ActivityActionRestore, ProductID: 17, ProductName: "Potatoes", Quantity: 1, Timestamp: now.AddDate(0, 0, -14)},
	}
	for i := range activity {
		if err := db.Create(&activity[i]).Error; err != nil {
			panic(err)
		}
	}
}

// PublicPageTokens are the fixed query strings that reach the token-gated public
// pages (`/web/invite/accept`, `/web/verify-email`, `/web/unsubscribe`) in a state
// worth photographing. Those pages render an error card without a valid token, so
// ui-capture presets would otherwise only ever document the failure branch.
const (
	DemoInviteToken      = "demo-invite-token"
	DemoVerifyToken      = "demo-verify-token"
	DemoUnsubscribeToken = "demo-unsubscribe-token"
)

func seedPublicPageTokens(db *gorm.DB, now time.Time) {
	invitation := dbModel.HouseholdInvitation{
		Model:       gorm.Model{CreatedAt: now.AddDate(0, 0, -1)},
		HouseholdID: 1,
		InviterID:   1,
		Email:       "newcomer@example.com",
		Token:       DemoInviteToken,
		Status:      dbModel.InvitationStatusPending,
		ExpiresAt:   now.AddDate(0, 0, 6),
		SentAt:      &now,
	}
	if err := db.Create(&invitation).Error; err != nil {
		panic(err)
	}

	// A pending verification for a fourth, not-yet-verified account: the demo user
	// is already verified, so their token would render "already verified".
	pendingUser := authentication.User{
		Model:       gorm.Model{ID: 4, CreatedAt: now.AddDate(0, 0, -1)},
		Username:    "dana",
		DisplayName: "Dana Reed",
		MailAddress: "dana@example.com",
		Password:    hashPassword("DanaDemo123!"),
		HouseholdID: 1,
	}
	if err := db.Create(&pendingUser).Error; err != nil {
		panic(err)
	}

	verification := dbModel.EmailVerification{
		Model:     gorm.Model{CreatedAt: now.AddDate(0, 0, -1)},
		UserID:    4,
		Token:     DemoVerifyToken,
		ExpiresAt: now.AddDate(0, 0, 6),
		Status:    dbModel.EmailVerificationStatusPending,
	}
	if err := db.Create(&verification).Error; err != nil {
		panic(err)
	}

	unsubscribe := dbModel.MailDigestUnsubscribeToken{
		Model:           gorm.Model{CreatedAt: now.AddDate(0, 0, -3)},
		MailDigestToken: DemoUnsubscribeToken,
		UserID:          1,
	}
	if err := db.Create(&unsubscribe).Error; err != nil {
		panic(err)
	}
}

func uintPtr(v uint) *uint {
	return &v
}
