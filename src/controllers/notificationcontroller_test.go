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

type MockNotificationRepository struct{}

func (m *MockNotificationRepository) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]dbModel.Product, error) {
	return []dbModel.Product{}, nil
}

func (m *MockNotificationRepository) GetMaxNotificationThresholdDays() int {
	return 0
}

func (m *MockNotificationRepository) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	return []string{"test@example.com"}, nil
}

func (m *MockNotificationRepository) SetProductNotifiedAt(productID uint) error {
	return nil
}

func (m *MockNotificationRepository) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	return []models.NotificationRecipientInfo{
		{
			EmailAddress: "test@example.com",
			NtfyURL:      "https://ntfy.sh",
			NtfyTopic:    "test_topic",
			NtfyToken:    "test_token",
		},
	}, nil
}

func (m *MockNotificationRepository) CreateInvitation(householdID, inviterID uint, email string) (dbModel.HouseholdInvitation, error) {
	return dbModel.HouseholdInvitation{}, nil
}

func (m *MockNotificationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]dbModel.HouseholdInvitation, error) {
	return nil, nil
}

func (m *MockNotificationRepository) GetInvitationByToken(token string) (dbModel.HouseholdInvitation, error) {
	return dbModel.HouseholdInvitation{}, nil
}

func (m *MockNotificationRepository) AcceptInvitation(token, email string, userID uint) error {
	return nil
}

func (m *MockNotificationRepository) CancelInvitation(invitationID, userID uint) error {
	return nil
}

func (m *MockNotificationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]dbModel.HouseholdInvitation, error) {
	return nil, nil
}

func (m *MockNotificationRepository) MarkInvitationSent(invitationID uint) error {
	return nil
}

func (m *MockNotificationRepository) MarkInvitationSendFailed(invitationID uint) error {
	return nil
}

func (m *MockNotificationRepository) GetUserByID(userID uint) (authentication.User, error) {
	return authentication.User{}, nil
}

func (m *MockNotificationRepository) GetHouseholdByID(householdID uint) (dbModel.Household, error) {
	return dbModel.Household{}, nil
}

func (m *MockNotificationRepository) GetOnboardingState(userID uint) (dbModel.OnboardingState, error) {
	return dbModel.OnboardingState{}, nil
}

func (m *MockNotificationRepository) MarkNotificationsSetup(userID uint) error {
	return nil
}

func (m *MockNotificationRepository) MarkHouseholdStepDone(userID uint) error {
	return nil
}

func (m *MockNotificationRepository) MarkOnboardingComplete(userID uint) error {
	return nil
}

func (m *MockNotificationRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]dbModel.HouseholdWithMemberCount, error) {
	return nil, nil
}

func (m *MockNotificationRepository) GetHouseholdsWithMonthlyWasteReportEnabled() ([]models.HouseholdReportTarget, error) {
	return nil, nil
}

func (m *MockNotificationRepository) GetWasteStatsForHousehold(householdID uint, month time.Time) (models.WasteStats, error) {
	return models.WasteStats{}, nil
}

var _ dbController.NotificationRepositoryInterface = (*MockNotificationRepository)(nil)

func TestNotificationControllerInitialization(t *testing.T) {
	logger := zerolog.Nop()

	mockRepo := &MockNotificationRepository{}

	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepo,
	}

	if nc.NotificationRepo == nil {
		t.Error("Expected notification repository to be set, got nil")
	}
}

func TestNotificationControllerWithMockDB(t *testing.T) {
	logger := zerolog.Nop()

	mockRepo := &MockNotificationRepository{}

	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepo,
	}

	if nc.NotificationRepo == nil {
		t.Error("Expected notification repository to be set, got nil")
	}

	var _ = nc.NotificationRepo
}

func TestProductExpirationDetection(t *testing.T) {
	expiredProduct := dbModel.Product{
		Model: gorm.Model{
			ID: 1,
		},
		Barcode:     "1234567890123",
		ProductName: "Expired Product",
		ExpireAt:    time.Now().Add(-24 * time.Hour),
	}

	futureProduct := dbModel.Product{
		Model: gorm.Model{
			ID: 2,
		},
		Barcode:     "9876543210987",
		ProductName: "Future Product",
		ExpireAt:    time.Now().Add(24 * time.Hour),
	}

	if expiredProduct.ExpireAt.After(time.Now()) {
		t.Error("Expected expired product to have past expiration date")
	}

	if !futureProduct.ExpireAt.After(time.Now()) {
		t.Error("Expected future product to have future expiration date")
	}
}

func TestNotificationConfiguration(t *testing.T) {
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
