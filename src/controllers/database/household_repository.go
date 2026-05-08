package database

import (
	"fmt"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
)

type HouseholdRepositoryInterface interface {
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetHouseholdMemberCount(householdID uint) (int64, error)
	GetHouseholdMembers(householdID uint) ([]authentication.User, error)
	LeaveHousehold(userID uint) error
	CreateAndSwitchHousehold(userID uint, name string) error
	ApplyForHousehold(applicantID, householdID uint) error
	GetPendingApplicationsForAdmin(adminUserID uint) ([]database.HouseholdApplication, error)
	ApproveApplication(applicationID, adminUserID uint) error
	RejectApplication(applicationID, adminUserID uint) error
	GetPendingApplicationsForApplicant(applicantUserID uint) ([]database.HouseholdApplication, error)
	CancelApplication(applicationID, applicantUserID uint) error
	UpdateHouseholdName(householdID, adminUserID uint, name string) error
	RemoveMemberFromHousehold(memberUserID, adminUserID uint) error
	GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error)
}

var _ HouseholdRepositoryInterface = (*HouseholdRepository)(nil)

type HouseholdRepository struct {
	DB *gorm.DB
}

func NewHouseholdRepository(db *gorm.DB) *HouseholdRepository {
	return &HouseholdRepository{DB: db}
}

func (r *HouseholdRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	selectErr := r.DB.First(&household, householdID)
	return household, selectErr.Error
}

func (r *HouseholdRepository) GetHouseholdMemberCount(householdID uint) (int64, error) {
	var count int64
	result := r.DB.Model(&authentication.User{}).Where(util.QueryHouseholdId, householdID).Count(&count)
	return count, result.Error
}

func (r *HouseholdRepository) GetHouseholdMembers(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where(util.QueryHouseholdId, householdID).Find(&users).Error
	return users, err
}

// createSoloHousehold creates a new household named after the user, sets adminID,
// moves products if the user was the sole member of the old household, then moves the user.
func createSoloHousehold(tx *gorm.DB, user *authentication.User, oldHouseholdID uint, newName string) error {
	newHousehold := database.Household{Name: newName}
	if err := tx.Create(&newHousehold).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&newHousehold).Update("admin_id", user.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	var memberCount int64
	tx.Model(&authentication.User{}).Where(util.QueryHouseholdId, oldHouseholdID).Count(&memberCount)
	if memberCount == 1 {
		if err := tx.Model(&database.Product{}).Where(util.QueryHouseholdId, oldHouseholdID).Update("household_id", newHousehold.ID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Model(user).Update("household_id", newHousehold.ID).Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (r *HouseholdRepository) LeaveHousehold(userID uint) error {
	tx := r.DB.Begin()

	var user authentication.User
	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	oldHouseholdID := user.HouseholdID
	if err := createSoloHousehold(tx, &user, oldHouseholdID, fmt.Sprintf("%s's Household", user.Username)); err != nil {
		return err
	}

	return tx.Commit().Error
}

func (r *HouseholdRepository) CreateAndSwitchHousehold(userID uint, name string) error {
	tx := r.DB.Begin()

	var user authentication.User
	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	oldHouseholdID := user.HouseholdID
	if err := createSoloHousehold(tx, &user, oldHouseholdID, name); err != nil {
		return err
	}

	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	defaultLocations := []database.StorageLocation{
		{HouseholdID: user.HouseholdID, Name: "Fridge", Icon: "🧊", SortOrder: 0},
		{HouseholdID: user.HouseholdID, Name: "Freezer", Icon: "❄️", SortOrder: 1},
		{HouseholdID: user.HouseholdID, Name: "Pantry", Icon: "🗄️", SortOrder: 2},
	}
	for i := range defaultLocations {
		if err := tx.Create(&defaultLocations[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func (r *HouseholdRepository) ApplyForHousehold(applicantID, householdID uint) error {
	var household database.Household
	if err := r.DB.First(&household, householdID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrHouseholdNotFound
		}
		return err
	}

	var existing database.HouseholdApplication
	result := r.DB.Where("applicant_id = ? AND household_id = ? AND status = ?", applicantID, householdID, database.ApplicationStatusPending).First(&existing)
	if result.Error == nil {
		return errors.ErrApplicationAlreadyPending
	}
	if result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}

	application := database.HouseholdApplication{
		ApplicantID: applicantID,
		HouseholdID: householdID,
		Status:      database.ApplicationStatusPending,
	}
	return r.DB.Create(&application).Error
}

func (r *HouseholdRepository) GetPendingApplicationsForAdmin(adminUserID uint) ([]database.HouseholdApplication, error) {
	var user authentication.User
	if err := r.DB.First(&user, adminUserID).Error; err != nil {
		return nil, err
	}

	var household database.Household
	if err := r.DB.First(&household, user.HouseholdID).Error; err != nil {
		return nil, err
	}
	if household.AdminID != adminUserID {
		return nil, errors.ErrNotHouseholdAdmin
	}

	var applications []database.HouseholdApplication
	err := r.DB.Where("household_id = ? AND status = ?", household.ID, database.ApplicationStatusPending).Find(&applications).Error
	return applications, err
}

func (r *HouseholdRepository) fetchAndAuthorizeApplication(tx *gorm.DB, applicationID, adminUserID uint) (database.HouseholdApplication, error) {
	var application database.HouseholdApplication
	if err := tx.First(&application, applicationID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return application, errors.ErrApplicationNotFound
		}
		return application, err
	}
	var household database.Household
	if err := tx.First(&household, application.HouseholdID).Error; err != nil {
		tx.Rollback()
		return application, err
	}
	if household.AdminID != adminUserID {
		tx.Rollback()
		return application, errors.ErrNotHouseholdAdmin
	}
	return application, nil
}

func (r *HouseholdRepository) ApproveApplication(applicationID, adminUserID uint) error {
	tx := r.DB.Begin()
	application, err := r.fetchAndAuthorizeApplication(tx, applicationID, adminUserID)
	if err != nil {
		return err
	}
	if err := tx.Model(&authentication.User{}).Where(util.QueryId, application.ApplicantID).Update("household_id", application.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&application).Update("status", database.ApplicationStatusApproved).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *HouseholdRepository) RejectApplication(applicationID, adminUserID uint) error {
	tx := r.DB.Begin()
	application, err := r.fetchAndAuthorizeApplication(tx, applicationID, adminUserID)
	if err != nil {
		return err
	}
	if err := tx.Model(&application).Update("status", database.ApplicationStatusRejected).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *HouseholdRepository) GetPendingApplicationsForApplicant(applicantUserID uint) ([]database.HouseholdApplication, error) {
	var applications []database.HouseholdApplication
	err := r.DB.
		Where("applicant_id = ? AND status = ?", applicantUserID, database.ApplicationStatusPending).
		Find(&applications).Error
	return applications, err
}

func (r *HouseholdRepository) CancelApplication(applicationID, applicantUserID uint) error {
	var application database.HouseholdApplication
	if err := r.DB.First(&application, applicationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrApplicationNotFound
		}
		return err
	}
	if application.ApplicantID != applicantUserID {
		return errors.ErrNotApplicationApplicant
	}
	if application.Status != database.ApplicationStatusPending {
		return errors.ErrApplicationNotFound
	}
	return r.DB.Delete(&application).Error
}

func (r *HouseholdRepository) UpdateHouseholdName(householdID, adminUserID uint, name string) error {
	var household database.Household
	if err := r.DB.First(&household, householdID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrHouseholdNotFound
		}
		return err
	}
	if household.AdminID != adminUserID {
		return errors.ErrNotHouseholdAdmin
	}
	return r.DB.Model(&household).Update("name", name).Error
}

func (r *HouseholdRepository) RemoveMemberFromHousehold(memberUserID, adminUserID uint) error {
	tx := r.DB.Begin()

	var adminUser authentication.User
	if err := tx.First(&adminUser, adminUserID).Error; err != nil {
		tx.Rollback()
		return err
	}

	var household database.Household
	if err := tx.First(&household, adminUser.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if household.AdminID != adminUserID {
		tx.Rollback()
		return errors.ErrNotHouseholdAdmin
	}

	var memberUser authentication.User
	if err := tx.First(&memberUser, memberUserID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return errors.ErrMemberNotInHousehold
		}
		return err
	}
	if memberUser.HouseholdID != household.ID {
		tx.Rollback()
		return errors.ErrMemberNotInHousehold
	}
	if memberUserID == adminUserID {
		tx.Rollback()
		return errors.ErrCannotRemoveAdmin
	}

	if err := createSoloHousehold(tx, &memberUser, household.ID, fmt.Sprintf("%s's Household", memberUser.Username)); err != nil {
		return err
	}

	return tx.Commit().Error
}

func (r *HouseholdRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error) {
	var results []database.HouseholdWithMemberCount
	err := r.DB.Table("households").
		Select("households.*, COUNT(users.id) as member_count").
		Joins("LEFT JOIN users ON households.id = users.household_id AND users.deleted_at IS NULL").
		Where("households.deleted_at IS NULL AND households.id != ?", excludeHouseholdID).
		Group("households.id").
		Order("households.name").
		Find(&results).Error
	return results, err
}

var _ = (*HouseholdRepository)(nil)
