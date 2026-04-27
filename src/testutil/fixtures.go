package testutil

import (
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

func CreateTestProduct(db *gorm.DB, householdID uint) *dbModel.Product {
	product := dbModel.Product{
		ProductName: "Test Product",
		Barcode:     "1234567890123",
		HouseholdID: householdID,
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
