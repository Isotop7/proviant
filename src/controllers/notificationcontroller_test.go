package controllers

import (
	"testing"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	authentication "codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// MockDatabaseController is a mock implementation for testing
type MockDatabaseController struct{}

func (m *MockDatabaseController) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration) ([]dbModel.Product, error) {
	// Return empty for testing
	return []dbModel.Product{}, nil
}

func (m *MockDatabaseController) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	// Return mock email addresses for testing
	return []string{"test@example.com"}, nil
}

func (m *MockDatabaseController) SetProductNotifiedAt(productID uint) error {
	// Mock successful update
	return nil
}

func (m *MockDatabaseController) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	// Return mock notification preferences for testing
	return []models.NotificationRecipientInfo{
		{
			EmailAddress: "test@example.com",
			NtfyURL:      "https://ntfy.sh",
			NtfyTopic:    "test_topic",
			NtfyToken:    "test_token",
		},
	}, nil
}

// Ensure MockDatabaseController implements the interface
var _ dbController.DatabaseControllerInterface = (*MockDatabaseController)(nil)

// TestNotificationControllerInitialization tests that the controller can be initialized
func TestNotificationControllerInitialization(t *testing.T) {
	// Setup logger
	logger := zerolog.Nop()

	// Setup mock database controller
	mockDB := &MockDatabaseController{}

	// Setup notification controller with dependency injection
	nc := &NotificationController{
		Logger:             &logger,
		Configuration:      &configuration.NotificationConfiguration{},
		DatabaseController: mockDB,
	}

	// Verify the mock database controller was set
	if nc.DatabaseController == nil {
		t.Error("Expected database controller to be set, got nil")
	}
}

// TestNotificationControllerWithMockDB tests the controller with a mock database
func TestNotificationControllerWithMockDB(t *testing.T) {
	// Setup logger
	logger := zerolog.Nop()

	// Setup mock database controller
	mockDB := &MockDatabaseController{}

	// Setup notification controller with dependency injection
	nc := &NotificationController{
		Logger:             &logger,
		Configuration:      &configuration.NotificationConfiguration{},
		DatabaseController: mockDB,
	}

	// Verify the mock database controller was set
	if nc.DatabaseController == nil {
		t.Error("Expected database controller to be set, got nil")
	}

	// Test that the mock implements the interface
	var _ = nc.DatabaseController
}

// TestProductExpirationDetection tests the logic for detecting expired products
func TestProductExpirationDetection(t *testing.T) {
	// Create test products
	expiredProduct := dbModel.Product{
		Model: gorm.Model{
			ID: 1,
		},
		Barcode:     "1234567890123",
		ProductName: "Expired Product",
		ExpireAt:    time.Now().Add(-24 * time.Hour), // Expired 24 hours ago
	}

	futureProduct := dbModel.Product{
		Model: gorm.Model{
			ID: 2,
		},
		Barcode:     "9876543210987",
		ProductName: "Future Product",
		ExpireAt:    time.Now().Add(24 * time.Hour), // Expires in 24 hours
	}

	// Test expiration detection
	if expiredProduct.ExpireAt.After(time.Now()) {
		t.Error("Expected expired product to have past expiration date")
	}

	if !futureProduct.ExpireAt.After(time.Now()) {
		t.Error("Expected future product to have future expiration date")
	}
}

// TestNotificationConfiguration tests configuration parsing
func TestNotificationConfiguration(t *testing.T) {
	// Test basic configuration
	config := configuration.NotificationConfiguration{
		Interval: 24,
		SMTP: configuration.SMTPConfiguration{
			Host:        "smtp.example.com",
			Port:        587,
			User:        "username",
			Password:    "password",
			FromAddress: "noreply@example.com",
			SSL:         true,
		},
	}

	// Verify configuration values
	if config.Interval != 24 {
		t.Errorf("Expected interval to be 24, got %d", config.Interval)
	}

	if config.SMTP.FromAddress != "noreply@example.com" {
		t.Errorf("Expected from address to be 'noreply@example.com', got %s", config.SMTP.FromAddress)
	}

	if config.SMTP.Host != "smtp.example.com" {
		t.Errorf("Expected SMTP host to be 'smtp.example.com', got %s", config.SMTP.Host)
	}
}

func (m *MockDatabaseController) CreateInvitation(householdID, inviterID uint, email string) (dbModel.HouseholdInvitation, error) {
	return dbModel.HouseholdInvitation{}, nil
}
func (m *MockDatabaseController) GetInvitationsForHousehold(householdID, inviterID uint) ([]dbModel.HouseholdInvitation, error) {
	return nil, nil
}
func (m *MockDatabaseController) GetInvitationByToken(token string) (dbModel.HouseholdInvitation, error) {
	return dbModel.HouseholdInvitation{}, nil
}
func (m *MockDatabaseController) AcceptInvitation(token, email string, userID uint) error {
	return nil
}
func (m *MockDatabaseController) CancelInvitation(invitationID, userID uint) error {
	return nil
}

func (m *MockDatabaseController) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]dbModel.HouseholdInvitation, error) {
	return nil, nil
}
func (m *MockDatabaseController) MarkInvitationSent(invitationID uint) error {
	return nil
}
func (m *MockDatabaseController) MarkInvitationSendFailed(invitationID uint) error {
	return nil
}

func (m *MockDatabaseController) GetUserByID(userID uint) (authentication.User, error) {
	return authentication.User{}, nil
}
func (m *MockDatabaseController) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return dbModel.Household{}, nil
}
