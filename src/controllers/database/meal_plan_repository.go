package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
)

// MealPlanRepositoryInterface persists household meal plan entries.
type MealPlanRepositoryInterface interface {
	Create(entry *database.MealPlanEntry) error
	GetByID(id, householdID uint) (*database.MealPlanEntry, error)
	ListByRange(householdID uint, from, to time.Time) ([]database.MealPlanEntry, error)
	Update(entry *database.MealPlanEntry) error
	Delete(id, householdID uint) error
}

var _ MealPlanRepositoryInterface = (*MealPlanRepository)(nil)

type MealPlanRepository struct {
	DB *gorm.DB
}

func NewMealPlanRepository(db *gorm.DB) *MealPlanRepository {
	return &MealPlanRepository{DB: db}
}

func (r *MealPlanRepository) Create(entry *database.MealPlanEntry) error {
	return r.DB.Create(entry).Error
}

func (r *MealPlanRepository) GetByID(id, householdID uint) (*database.MealPlanEntry, error) {
	var entry database.MealPlanEntry
	err := r.DB.Where("id = ? AND household_id = ?", id, householdID).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListByRange returns the entries whose date lies within [from, to], ordered by date.
func (r *MealPlanRepository) ListByRange(householdID uint, from, to time.Time) ([]database.MealPlanEntry, error) {
	var entries []database.MealPlanEntry
	err := r.DB.Where("household_id = ? AND date >= ? AND date <= ?", householdID, from, to).
		Order("date ASC, id ASC").Find(&entries).Error
	return entries, err
}

func (r *MealPlanRepository) Update(entry *database.MealPlanEntry) error {
	return r.DB.Save(entry).Error
}

func (r *MealPlanRepository) Delete(id, householdID uint) error {
	return r.DB.Where("id = ? AND household_id = ?", id, householdID).Delete(&database.MealPlanEntry{}).Error
}
