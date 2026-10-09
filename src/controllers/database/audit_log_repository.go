package database

import (
	"context"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
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
	GetAuditLogs(ctx context.Context, householdID uint, limit int) ([]database.AuditLog, error)
	GetAuditLogsByDate(ctx context.Context, householdID uint, limit int, date string) ([]database.AuditLog, error)
}

var _ AuditLogRepositoryInterface = (*AuditLogRepository)(nil)

// Create stamps the owning household onto the entry when the caller did not set
// one. Stamping here, once, covers every write site (login middleware, member
// management, password changes, admin actions) and keeps the attribution
// authoritative: it comes from the user row at the moment the action happened,
// not from whatever household that user belongs to later.
//
// The lookup is Unscoped because account_deleted is written after the target
// user is soft-deleted, and their household is still the correct bucket. A
// failed login for an unknown username has no user to attribute and stays NULL,
// which makes it invisible to every household on read — the safe default.
func (r *AuditLogRepository) Create(ctx context.Context, log *database.AuditLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.HouseholdID == nil && log.UserID != nil && *log.UserID != 0 {
		var user authentication.User
		err := r.DB.WithContext(ctx).Unscoped().First(&user, *log.UserID).Error
		if err == nil && user.HouseholdID != 0 {
			householdID := user.HouseholdID
			log.HouseholdID = &householdID
		}
	}
	return r.DB.WithContext(ctx).Create(log).Error
}

func (r *AuditLogRepository) GetAuditLogs(ctx context.Context, householdID uint, limit int) ([]database.AuditLog, error) {
	var logs []database.AuditLog
	err := r.DB.WithContext(ctx).
		Where("household_id = ?", householdID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *AuditLogRepository) GetAuditLogsByDate(ctx context.Context, householdID uint, limit int, date string) ([]database.AuditLog, error) {
	var logs []database.AuditLog
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}
	start := parsedDate.UTC()
	end := start.Add(24 * time.Hour)
	err = r.DB.WithContext(ctx).
		Where("household_id = ?", householdID).
		Where("timestamp >= ? AND timestamp < ?", start, end).
		Order("timestamp DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
