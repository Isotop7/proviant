package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

type NotificationRepository struct {
	DB *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{DB: db}
}

type NotificationRepositoryInterface interface {
	GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]database.Product, error)
	GetMaxNotificationThresholdDays() int
	GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error)
	GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error)
	SetProductNotifiedAt(productID uint) error
	CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error)
	GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error)
	GetInvitationByToken(token string) (database.HouseholdInvitation, error)
	AcceptInvitation(token, email string, userID uint) error
	CancelInvitation(invitationID, userID uint) error
	GetUserByID(userID uint) (authentication.User, error)
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error)
	MarkInvitationSent(invitationID uint) error
	MarkInvitationSendFailed(invitationID uint) error
	GetOnboardingState(userID uint) (database.OnboardingState, error)
	MarkNotificationsSetup(userID uint) error
	MarkHouseholdStepDone(userID uint) error
	MarkOnboardingComplete(userID uint) error
	GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error)
}

func (r *NotificationRepository) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]database.Product, error) {
	var notificationProducts []database.Product
	getError := r.DB.
		Where("expire_at > ?", time.Time{}).
		Where("expire_at <= ?", time.Now().AddDate(0, 0, maxLookAheadDays)).
		Where("notified_at < ?", time.Now().Add(-(sleepInterval))).
		Find(&notificationProducts)

	if getError.Error != nil {
		return []database.Product{}, getError.Error
	}
	return notificationProducts, nil
}

func (r *NotificationRepository) GetMaxNotificationThresholdDays() int {
	var maxThreshold int
	r.DB.Model(&authentication.User{}).
		Select("COALESCE(MAX(notification_threshold_days), 0)").
		Scan(&maxThreshold)
	return maxThreshold
}

func (r *NotificationRepository) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	var mailAddresses []string
	_, householdErr := r.GetHouseholdByID(householdID)
	if householdErr != nil {
		return mailAddresses, householdErr
	}

	var users []*authentication.User
	findErr := r.DB.Where("household_id = ?", householdID).Find(&users)
	if findErr != nil {
		return mailAddresses, findErr.Error
	}

	for idx := range users {
		mailAddresses = append(mailAddresses, users[idx].MailAddress)
	}
	return mailAddresses, nil
}

func (r *NotificationRepository) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	var preferences []models.NotificationRecipientInfo

	_, householdErr := r.GetHouseholdByID(householdID)
	if householdErr != nil {
		return preferences, householdErr
	}

	var users []*authentication.User
	findErr := r.DB.Where("household_id = ?", householdID).Find(&users)
	if findErr.Error != nil {
		return preferences, findErr.Error
	}

	for idx := range users {
		user := users[idx]
		preferences = append(preferences, models.NotificationRecipientInfo{
			EmailAddress:              user.MailAddress,
			NtfyURL:                   user.NotificationPreferences.NtfyURL,
			NtfyTopic:                 user.NotificationPreferences.NtfyTopic,
			NtfyToken:                 user.NotificationPreferences.NtfyToken,
			NotificationThresholdDays: user.NotificationPreferences.NotificationThresholdDays,
		})
	}

	return preferences, nil
}

func (r *NotificationRepository) SetProductNotifiedAt(productID uint) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	dbProduct.NotifiedAt = time.Now()
	saveResult := r.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

func (r *NotificationRepository) CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.CreateInvitation(householdID, inviterID, email)
}

func (r *NotificationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetInvitationsForHousehold(householdID, inviterID)
}

func (r *NotificationRepository) GetInvitationByToken(token string) (database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetInvitationByToken(token)
}

func (r *NotificationRepository) AcceptInvitation(token, email string, userID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.AcceptInvitation(token, email, userID)
}

func (r *NotificationRepository) CancelInvitation(invitationID, userID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.CancelInvitation(invitationID, userID)
}

func (r *NotificationRepository) GetUserByID(userID uint) (authentication.User, error) {
	userRepo := NewUserRepository(r.DB)
	return userRepo.GetUserByID(userID)
}

func (r *NotificationRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	householdRepo := NewHouseholdRepository(r.DB)
	return householdRepo.GetHouseholdByID(householdID)
}

func (r *NotificationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetPendingInvitationsNotSent(retryInterval)
}

func (r *NotificationRepository) MarkInvitationSent(invitationID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.MarkInvitationSent(invitationID)
}

func (r *NotificationRepository) MarkInvitationSendFailed(invitationID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.MarkInvitationSendFailed(invitationID)
}

func (r *NotificationRepository) GetOnboardingState(userID uint) (database.OnboardingState, error) {
	userRepo := NewUserRepository(r.DB)
	return userRepo.GetOnboardingState(userID)
}

func (r *NotificationRepository) MarkNotificationsSetup(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkNotificationsSetup(userID)
}

func (r *NotificationRepository) MarkHouseholdStepDone(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkHouseholdStepDone(userID)
}

func (r *NotificationRepository) MarkOnboardingComplete(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkOnboardingComplete(userID)
}

func (r *NotificationRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error) {
	householdRepo := NewHouseholdRepository(r.DB)
	return householdRepo.GetPublicHouseholds(excludeHouseholdID)
}

var _ NotificationRepositoryInterface = (*NotificationRepository)(nil)
