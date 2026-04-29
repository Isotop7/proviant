package database

import (
	"fmt"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserRepositoryInterface interface {
	GetUserByUsername(username string) (authentication.User, error)
	GetUserByID(userID uint) (authentication.User, error)
	GetUserHouseholdByID(userID uint) (uint, error)
	UserExistsByUsername(user *authentication.User) bool
	UserExistsByMailAddress(user *authentication.User) bool
	CreateUser(user *authentication.User) error
	UpdateUser(userID uint, user *authentication.User) error
	UpdateAdminUserFields(userID uint, username, mailAddress string) error
	UpdateDisplayName(userID uint, displayName string) error
	UpdateUserPassword(userID uint, login *authentication.Login) error
	IsAccountLocked(userID uint, maxLoginAttempts int, lockoutDurationMins int) (bool, time.Duration)
	RecordFailedLoginAttempt(userID uint, maxLoginAttempts int, lockoutDurationMins int) error
	ResetFailedLoginAttempts(userID uint) error
	CreateEmailVerification(userID uint, token string, expiresAt time.Time) error
	GetEmailVerificationByToken(token string) (database.EmailVerification, error)
	UpdateUserEmailVerified(userID uint, verifiedAt time.Time) error
	UpdateEmailVerificationStatus(token string, status string) error
	GetOnboardingState(userID uint) (database.OnboardingState, error)
	MarkNotificationsSetup(userID uint) error
	UpdateUsername(userID uint, username string) error
	MarkProfileStepDone(userID uint) error
	MarkHouseholdStepDone(userID uint) error
	MarkOnboardingComplete(userID uint) error
	EnsureOnboardingState(userID uint) error
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetUsersByHouseholdID(householdID uint) ([]authentication.User, error)
	DeleteUser(userID uint) error
}

var _ UserRepositoryInterface = (*UserRepository)(nil)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{DB: db}
}

const (
	DefaultMaxLoginAttempts    = 10
	DefaultLockoutDurationMins = 15
)

func (r *UserRepository) GetUserByUsername(username string) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, "username = ?", username)
	return user, selectErr.Error
}

func (r *UserRepository) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user, selectErr.Error
}

func (r *UserRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user.HouseholdID, selectErr.Error
}

func (r *UserRepository) UserExistsByUsername(user *authentication.User) bool {
	var dbUser authentication.User
	selectErr := r.DB.First(&dbUser, "username = ?", user.Username)
	return selectErr.Error != gorm.ErrRecordNotFound
}

func (r *UserRepository) UserExistsByMailAddress(user *authentication.User) bool {
	var dbUser authentication.User
	selectErr := r.DB.First(&dbUser, "mail_address = ?", user.MailAddress)
	return selectErr.Error != gorm.ErrRecordNotFound
}

func (r *UserRepository) CreateUser(user *authentication.User) error {
	tx := r.DB.Begin()

	household := database.Household{
		Name: fmt.Sprintf("%s's Household", user.Username),
	}
	if err := tx.Create(&household).Error; err != nil {
		tx.Rollback()
		return err
	}

	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	user.HouseholdID = household.ID
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&household).Update("admin_id", user.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	onboardingState := database.OnboardingState{
		UserID: user.ID,
	}
	if err := tx.Create(&onboardingState).Error; err != nil {
		tx.Rollback()
		return err
	}

	defaultLocations := []database.StorageLocation{
		{HouseholdID: household.ID, Name: "Fridge", Icon: "🧊", SortOrder: 0},
		{HouseholdID: household.ID, Name: "Freezer", Icon: "❄️", SortOrder: 1},
		{HouseholdID: household.ID, Name: "Pantry", Icon: "🗄️", SortOrder: 2},
	}
	for i := range defaultLocations {
		if err := tx.Create(&defaultLocations[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (r *UserRepository) UpdateUser(userID uint, user *authentication.User) error {
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	var dbUser authentication.User
	getError := r.DB.First(&dbUser, userID)

	if getError.Error != nil {
		return getError.Error
	}
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	dbUser.DisplayName = user.DisplayName
	dbUser.MailAddress = user.MailAddress
	dbUser.NotificationPreferences = user.NotificationPreferences

	saveResult := r.DB.Save(&dbUser)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

// UpdateAdminUserFields allows admins to change login-credential fields (username, email).
func (r *UserRepository) UpdateAdminUserFields(userID uint, username, mailAddress string) error {
	return r.DB.Model(&authentication.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"username":     username,
			"mail_address": mailAddress,
		}).Error
}

func (r *UserRepository) UpdateDisplayName(userID uint, displayName string) error {
	return r.DB.Model(&authentication.User{}).
		Where("id = ?", userID).
		Update("display_name", displayName).Error
}

func (r *UserRepository) UpdateUserPassword(userID uint, login *authentication.Login) error {
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	var dbUser authentication.User
	getError := r.DB.First(&dbUser, userID)

	if getError.Error != nil {
		return getError.Error
	}
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	if dbUser.Username != login.Username {
		return errors.ErrMismatchedUsername
	}

	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(login.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	dbUser.Password = string(hashedPassword)

	saveResult := r.DB.Save(&dbUser)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

func (r *UserRepository) IsAccountLocked(userID uint, maxLoginAttempts int, lockoutDurationMins int) (bool, time.Duration) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return false, 0
	}
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		remaining := time.Until(user.LockedUntil.Time)
		return true, remaining
	}
	return false, 0
}

func (r *UserRepository) RecordFailedLoginAttempt(userID uint, maxLoginAttempts int, lockoutDurationMins int) error {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return err
	}
	user.FailedLoginAttempts++
	if maxLoginAttempts > 0 && user.FailedLoginAttempts >= uint(maxLoginAttempts) {
		lockoutUntil := time.Now().Add(time.Duration(lockoutDurationMins) * time.Minute)
		return r.DB.Model(&user).Updates(map[string]interface{}{
			"failed_login_attempts": user.FailedLoginAttempts,
			"locked_until":          lockoutUntil,
		}).Error
	}
	return r.DB.Model(&user).Update("failed_login_attempts", user.FailedLoginAttempts).Error
}

func (r *UserRepository) ResetFailedLoginAttempts(userID uint) error {
	return r.DB.Model(&authentication.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"failed_login_attempts": 0,
		"locked_until":          nil,
	}).Error
}

func (r *UserRepository) CreateEmailVerification(userID uint, token string, expiresAt time.Time) error {
	verification := database.EmailVerification{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		Status:    database.EmailVerificationStatusPending,
	}
	return r.DB.Create(&verification).Error
}

func (r *UserRepository) GetEmailVerificationByToken(token string) (database.EmailVerification, error) {
	var verification database.EmailVerification
	result := r.DB.Where("token = ?", token).First(&verification)
	return verification, result.Error
}

func (r *UserRepository) UpdateUserEmailVerified(userID uint, verifiedAt time.Time) error {
	return r.DB.Model(&authentication.User{}).Where("id = ?", userID).Update("email_verified_at", verifiedAt).Error
}

func (r *UserRepository) UpdateEmailVerificationStatus(token string, status string) error {
	return r.DB.Model(&database.EmailVerification{}).Where("token = ?", token).Update("status", status).Error
}

func (r *UserRepository) GetOnboardingState(userID uint) (database.OnboardingState, error) {
	var onboardingState database.OnboardingState
	err := r.DB.Where("user_id = ?", userID).First(&onboardingState).Error
	return onboardingState, err
}

func (r *UserRepository) MarkNotificationsSetup(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where("user_id = ?", userID).
		Update("notifications_setup", true).Error
}

func (r *UserRepository) UpdateUsername(userID uint, username string) error {
	return r.DB.Model(&authentication.User{}).
		Where("id = ?", userID).
		Update("username", username).Error
}

func (r *UserRepository) MarkProfileStepDone(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where("user_id = ?", userID).
		Update("profile_step_done", true).Error
}

func (r *UserRepository) MarkHouseholdStepDone(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where("user_id = ?", userID).
		Update("household_step_done", true).Error
}

func (r *UserRepository) MarkOnboardingComplete(userID uint) error {
	return r.DB.Model(&database.OnboardingState{}).
		Where("user_id = ?", userID).
		Update("onboarding_completed", true).Error
}

func (r *UserRepository) EnsureOnboardingState(userID uint) error {
	var existing database.OnboardingState
	findErr := r.DB.Where("user_id = ?", userID).First(&existing).Error
	if findErr == nil {
		return nil
	}
	if findErr != gorm.ErrRecordNotFound {
		return findErr
	}
	state := database.OnboardingState{UserID: userID}
	return r.DB.Create(&state).Error
}

func (r *UserRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	selectErr := r.DB.First(&household, householdID)
	return household, selectErr.Error
}

func (r *UserRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where("household_id = ?", householdID).Find(&users).Error
	return users, err
}

func (r *UserRepository) DeleteUser(userID uint) error {
	return r.DB.Delete(&authentication.User{}, userID).Error
}

var _ = (*UserRepository)(nil)
