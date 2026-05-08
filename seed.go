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
		Model: gorm.Model{ID: 1},
		Name:  "Demo Household",
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
			Model:         gorm.Model{ID: 1},
			Username:      "demo",
			DisplayName:   "Demo User",
			MailAddress:   "demo@proviant.app",
			Password:      hashPassword("demo"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:   1,
		},
		{
			Model:         gorm.Model{ID: 2},
			Username:      "alice",
			DisplayName:   "Alice Smith",
			MailAddress:   "alice@example.com",
			Password:      hashPassword("AliceDemo123!"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:   1,
		},
		{
			Model:         gorm.Model{ID: 3},
			Username:      "bob",
			DisplayName:   "Bob Johnson",
			MailAddress:   "bob@example.com",
			Password:      hashPassword("BobDemo123!"),
			EmailVerifiedAt: &verifiedAt,
			HouseholdID:   1,
		},
	}
	for _, user := range users {
		if err := db.Create(&user).Error; err != nil {
			panic(err)
		}
	}

	products := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Milk", Barcode: "8710398018520", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 2), ScannedAt: now.AddDate(0, 0, -5), Amount: 1, Unit: "L"},
		{Model: gorm.Model{ID: 2}, ProductName: "Yogurt", Barcode: "8710398501466", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 5), ScannedAt: now.AddDate(0, 0, -3), Amount: 4, Unit: "pcs"},
		{Model: gorm.Model{ID: 3}, ProductName: "Eggs", Barcode: "8710385123457", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 10), ScannedAt: now.AddDate(0, 0, -7), Amount: 12, Unit: "pcs"},
		{Model: gorm.Model{ID: 4}, ProductName: "Butter", Barcode: "8710398512349", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 14), ScannedAt: now.AddDate(0, 0, -2), Amount: 1, Unit: "pkg"},
		{Model: gorm.Model{ID: 5}, ProductName: "Chicken Breast", Barcode: "8710385123464", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, 1), ScannedAt: now.AddDate(0, 0, -3), Amount: 500, Unit: "g"},
		{Model: gorm.Model{ID: 6}, ProductName: "Ground Beef", Barcode: "8710385123471", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, 3), ScannedAt: now.AddDate(0, 0, -5), Amount: 400, Unit: "g"},
		{Model: gorm.Model{ID: 7}, ProductName: "Salmon Fillet", Barcode: "8710385123488", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 0, -1), ScannedAt: now.AddDate(0, 0, -7), Amount: 300, Unit: "g", RemovalReason: "wasted"},
		{Model: gorm.Model{ID: 8}, ProductName: "Frozen Pizza", Barcode: "8076809513753", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -14), Amount: 2, Unit: "pcs"},
		{Model: gorm.Model{ID: 9}, ProductName: "Ice Cream", Barcode: "8076800195875", HouseholdID: 1, StorageLocationID: uintPtr(2), ExpireAt: now.AddDate(0, 1, 15), ScannedAt: now.AddDate(0, 0, -30), Amount: 1, Unit: "L"},
		{Model: gorm.Model{ID: 10}, ProductName: "Pasta", Barcode: "8076800195905", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(1, 0, 0), ScannedAt: now.AddDate(0, 0, -60), Amount: 3, Unit: "pkg"},
		{Model: gorm.Model{ID: 11}, ProductName: "Rice", Barcode: "8410065012345", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -90), Amount: 2, Unit: "kg"},
		{Model: gorm.Model{ID: 12}, ProductName: "Tomato Sauce", Barcode: "8410065012352", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 2, 0), ScannedAt: now.AddDate(0, 0, -20), Amount: 2, Unit: "jars"},
		{Model: gorm.Model{ID: 13}, ProductName: "Olive Oil", Barcode: "8410065012369", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(1, 0, 0), ScannedAt: now.AddDate(0, 0, -45), Amount: 1, Unit: "L"},
		{Model: gorm.Model{ID: 14}, ProductName: "Bread", Barcode: "3228027000016", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 1), ScannedAt: now.AddDate(0, 0, -2), Amount: 1, Unit: "loaf"},
		{Model: gorm.Model{ID: 15}, ProductName: "Cheese", Barcode: "3228027000023", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 7), ScannedAt: now.AddDate(0, 0, -4), Amount: 200, Unit: "g"},
		{Model: gorm.Model{ID: 16}, ProductName: "Apple Juice", Barcode: "5449000000996", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -10), Amount: 2, Unit: "L"},
		{Model: gorm.Model{ID: 17}, ProductName: "Potatoes", Barcode: "5449000001238", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 20), ScannedAt: now.AddDate(0, 0, -15), Amount: 1, Unit: "kg"},
		{Model: gorm.Model{ID: 18}, ProductName: "Onions", Barcode: "5449000001245", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 0, 30), ScannedAt: now.AddDate(0, 0, -20), Amount: 500, Unit: "g"},
		{Model: gorm.Model{ID: 19}, ProductName: "Garlic", Barcode: "5449000001252", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 1, 0), ScannedAt: now.AddDate(0, 0, -25), Amount: 3, Unit: "pcs"},
		{Model: gorm.Model{ID: 20}, ProductName: "Spinach", Barcode: "5449000001269", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 4), ScannedAt: now.AddDate(0, 0, -1), Amount: 200, Unit: "g"},
		{Model: gorm.Model{ID: 21}, ProductName: "Ham", Barcode: "8410065012376", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 3), ScannedAt: now.AddDate(0, 0, -2), Amount: 100, Unit: "g"},
		{Model: gorm.Model{ID: 22}, ProductName: "Orange Juice", Barcode: "5449000001276", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 6), ScannedAt: now.AddDate(0, 0, -5), Amount: 1, Unit: "L"},
		{Model: gorm.Model{ID: 23}, ProductName: "Cereal", Barcode: "5000159484695", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 3, 0), ScannedAt: now.AddDate(0, 0, -40), Amount: 1, Unit: "box"},
		{Model: gorm.Model{ID: 24}, ProductName: "Peanut Butter", Barcode: "5000159484701", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 4, 0), ScannedAt: now.AddDate(0, 0, -35), Amount: 1, Unit: "jar"},
		{Model: gorm.Model{ID: 25}, ProductName: "Honey", Barcode: "5000159484718", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -50), Amount: 1, Unit: "jar"},
		{Model: gorm.Model{ID: 26}, ProductName: "Coffee", Barcode: "5000159484725", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 2, 0), ScannedAt: now.AddDate(0, 0, -30), Amount: 200, Unit: "g"},
		{Model: gorm.Model{ID: 27}, ProductName: "Tea", Barcode: "5000159484732", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -55), Amount: 50, Unit: "bags"},
		{Model: gorm.Model{ID: 28}, ProductName: "Sugar", Barcode: "5000159484749", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 12, 0), ScannedAt: now.AddDate(0, 0, -100), Amount: 1, Unit: "kg"},
		{Model: gorm.Model{ID: 29}, ProductName: "Flour", Barcode: "5000159484756", HouseholdID: 1, StorageLocationID: uintPtr(3), ExpireAt: now.AddDate(0, 6, 0), ScannedAt: now.AddDate(0, 0, -70), Amount: 1, Unit: "kg"},
		{Model: gorm.Model{ID: 30}, ProductName: "Tofu", Barcode: "5000159484763", HouseholdID: 1, StorageLocationID: uintPtr(1), ExpireAt: now.AddDate(0, 0, 2), ScannedAt: now.AddDate(0, 0, -3), Amount: 400, Unit: "g"},
	}
	for _, product := range products {
		if err := db.Create(&product).Error; err != nil {
			panic(err)
		}
	}

	for i := 1; i <= 3; i++ {
		state := dbModel.OnboardingState{
			Model: gorm.Model{ID: uint(i)},
			UserID: uint(i),
		}
		if err := db.Create(&state).Error; err != nil {
			panic(err)
		}
	}
}

func uintPtr(v uint) *uint {
	return &v
}