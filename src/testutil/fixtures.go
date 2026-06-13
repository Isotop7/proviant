package testutil

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

func CreateTestUser(db *gorm.DB, householdID uint) *authentication.User {
	user := authentication.User{
		Username:    "testuser",
		Password:    "testpassword123",
		MailAddress: "test@example.com",
		HouseholdID: householdID,
	}
	db.Create(&user)
	return &user
}

func CreateTestHousehold(db *gorm.DB, adminID uint) *dbModel.Household {
	household := dbModel.Household{
		Name:    "Test Household",
		AdminID: adminID,
	}
	db.Create(&household)
	return &household
}

func CreateTestProduct(db *gorm.DB, householdID uint, userID ...uint) *dbModel.Product {
	uid := uint(0)
	if len(userID) > 0 {
		uid = userID[0]
	}
	product := dbModel.Product{
		ProductName: "Test Product",
		Barcode:     "1234567890123",
		HouseholdID: householdID,
		UserID:      uid,
		ExpireAt:    time.Now().Add(7 * 24 * time.Hour),
	}
	db.Create(&product)
	return &product
}

func CreateTestStorageLocation(db *gorm.DB, householdID uint) *dbModel.StorageLocation {
	location := dbModel.StorageLocation{
		Name:        "Fridge",
		Icon:        "🧊",
		SortOrder:   0,
		HouseholdID: householdID,
	}
	db.Create(&location)
	return &location
}

func CreateTestWebhook(db *gorm.DB, userID uint) *dbModel.Webhook {
	webhook := dbModel.Webhook{
		UserID: userID,
		URL:    "https://example.com/webhook",
		Secret: "secret123",
		Events: "product.expired",
		Active: true,
	}
	db.Create(&webhook)
	return &webhook
}

func CreateTestInvitation(db *gorm.DB, householdID, inviterID uint, email string) *dbModel.HouseholdInvitation {
	invitation := dbModel.HouseholdInvitation{
		HouseholdID: householdID,
		InviterID:   inviterID,
		Email:       email,
		Token:       "test-token-" + email,
	}
	db.Create(&invitation)
	return &invitation
}

func CreateTestEmailVerification(db *gorm.DB, userID uint, token string) *dbModel.EmailVerification {
	verification := dbModel.EmailVerification{
		UserID:    userID,
		Token:     token,
		Status:    dbModel.EmailVerificationStatusPending,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	db.Create(&verification)
	return &verification
}

// SeedConsumedProduct creates a product, soft-deletes it with the given
// removalReason, and pins the deleted_at to a specific timestamp. It is
// shared by the consumption-service and consumption-handler test suites.
func SeedConsumedProduct(t *testing.T, db *gorm.DB, householdID, userID uint, isPrivate bool, barcode, name, unit string, amount int, deletedAt time.Time) {
	t.Helper()
	p := dbModel.Product{
		ProductName:   name,
		Barcode:       barcode,
		HouseholdID:   householdID,
		UserID:        userID,
		IsPrivate:     isPrivate,
		Amount:        amount,
		Unit:          unit,
		RemovalReason: dbModel.RemovalReasonConsumed,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if err := db.Delete(&p).Error; err != nil {
		t.Fatalf("seed delete: %v", err)
	}
	if err := db.Model(&dbModel.Product{}).Unscoped().
		Where("id = ?", p.ID).
		Update("deleted_at", deletedAt).Error; err != nil {
		t.Fatalf("seed update deleted_at: %v", err)
	}
}
