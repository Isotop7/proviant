package database

import (
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// StreakRepositoryInterface defines operations for household waste streaks.
type StreakRepositoryInterface interface {
	GetOrCreateStreakForHousehold(householdID uint) (*dbModel.WasteStreak, error)
	RecordWasteEvent(householdID uint) error
	UpdateStreak(streak *dbModel.WasteStreak) error
	GetAllStreaks() ([]dbModel.WasteStreak, error)
}

// StreakRepository implements StreakRepositoryInterface.
type StreakRepository struct {
	DB *gorm.DB
}

func NewStreakRepository(db *gorm.DB) *StreakRepository {
	return &StreakRepository{DB: db}
}

func (r *StreakRepository) GetOrCreateStreakForHousehold(householdID uint) (*dbModel.WasteStreak, error) {
	var streak dbModel.WasteStreak
	err := r.DB.Where("household_id = ?", householdID).First(&streak).Error
	if err == gorm.ErrRecordNotFound {
		streak = dbModel.WasteStreak{
			HouseholdID:     householdID,
			CurrentStreak:   0,
			LongestStreak:   0,
			LastCheckedDate: time.Now().UTC(),
		}
		if createErr := r.DB.Create(&streak).Error; createErr != nil {
			return nil, createErr
		}
		return &streak, nil
	}
	return &streak, err
}

func (r *StreakRepository) RecordWasteEvent(householdID uint) error {
	streak, err := r.GetOrCreateStreakForHousehold(householdID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	streak.LastWastedDate = &now
	return r.DB.Save(streak).Error
}

func (r *StreakRepository) UpdateStreak(streak *dbModel.WasteStreak) error {
	return r.DB.Save(streak).Error
}

func (r *StreakRepository) GetAllStreaks() ([]dbModel.WasteStreak, error) {
	var streaks []dbModel.WasteStreak
	return streaks, r.DB.Find(&streaks).Error
}
