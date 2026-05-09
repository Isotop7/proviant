package database

import (
	"context"
	"time"

	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

type AuditLogRepository struct {
	DB *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{DB: db}
}

type AuditLogRepositoryInterface interface {
	Create(ctx context.Context, log *database.AuditLog) error
	GetAuditLogs(ctx context.Context, limit int) ([]database.AuditLog, error)
	GetAuditLogsByDate(ctx context.Context, limit int, date string) ([]database.AuditLog, error)
}

var _ AuditLogRepositoryInterface = (*AuditLogRepository)(nil)

func (r *AuditLogRepository) Create(ctx context.Context, log *database.AuditLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	return r.DB.WithContext(ctx).Create(log).Error
}

func (r *AuditLogRepository) GetAuditLogs(ctx context.Context, limit int) ([]database.AuditLog, error) {
	var logs []database.AuditLog
	err := r.DB.WithContext(ctx).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *AuditLogRepository) GetAuditLogsByDate(ctx context.Context, limit int, date string) ([]database.AuditLog, error) {
	var logs []database.AuditLog
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}
	start := parsedDate.UTC()
	end := start.Add(24 * time.Hour)
	err = r.DB.WithContext(ctx).
		Where("timestamp >= ? AND timestamp < ?", start, end).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
