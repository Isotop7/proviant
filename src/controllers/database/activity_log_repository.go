package database

import (
	"context"
	"time"

	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

type ActivityLogRepository struct {
	DB *gorm.DB
}

func NewActivityLogRepository(db *gorm.DB) *ActivityLogRepository {
	return &ActivityLogRepository{DB: db}
}

type ActivityLogRepositoryInterface interface {
	Create(ctx context.Context, log *database.ActivityLog) error
	GetByHousehold(ctx context.Context, householdID uint, limit, offset int) ([]database.ActivityLog, error)
	GetByHouseholdCount(ctx context.Context, householdID uint) (int, error)
}

var _ ActivityLogRepositoryInterface = (*ActivityLogRepository)(nil)

func (r *ActivityLogRepository) Create(ctx context.Context, log *database.ActivityLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	return r.DB.WithContext(ctx).Create(log).Error
}

func (r *ActivityLogRepository) GetByHousehold(ctx context.Context, householdID uint, limit, offset int) ([]database.ActivityLog, error) {
	var logs []database.ActivityLog
	err := r.DB.WithContext(ctx).
		Where("household_id = ?", householdID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error
	return logs, err
}

func (r *ActivityLogRepository) GetByHouseholdCount(ctx context.Context, householdID uint) (int, error) {
	var count int64
	err := r.DB.Model(&database.ActivityLog{}).Where("household_id = ?", householdID).Count(&count).Error
	return int(count), err
}
