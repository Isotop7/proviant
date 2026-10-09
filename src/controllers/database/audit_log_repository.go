package database

import (
	"context"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type AuditLogRepository struct {
	DB     *gorm.DB
	Logger *zerolog.Logger
}

// NewAuditLogRepository wires the startup logger into the audit repository so
// Create can report entries it fails to attribute; logger may be nil (tests),
// in which case warnUnattributed is a no-op.
func NewAuditLogRepository(db *gorm.DB, logger *zerolog.Logger) *AuditLogRepository {
	return &AuditLogRepository{DB: db, Logger: logger}
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
// which makes it invisible to every household on read — the safe default. A
// failed attribution for a non-zero user id (user row gone, household unset)
// also stays NULL, but is logged at Warn: that entry is lost to every view and
// only the log line says why.
func (r *AuditLogRepository) Create(ctx context.Context, log *database.AuditLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.HouseholdID == nil && log.UserID != nil && *log.UserID != 0 {
		var user authentication.User
		err := r.DB.WithContext(ctx).Unscoped().First(&user, *log.UserID).Error
		switch {
		case err != nil:
			r.warnUnattributed(*log.UserID, err.Error())
		case user.HouseholdID == 0:
			r.warnUnattributed(*log.UserID, "user has no household")
		default:
			householdID := user.HouseholdID
			log.HouseholdID = &householdID
		}
	}
	return r.DB.WithContext(ctx).Create(log).Error
}

// warnUnattributed reports an audit entry that will stay invisible to every
// household. A missing Logger (tests, plain constructor) is a no-op.
func (r *AuditLogRepository) warnUnattributed(userID uint, reason string) {
	if r.Logger == nil {
		return
	}
	r.Logger.Warn().Msgf("audit log entry for user %d not attributed to a household (%s); it stays invisible to every household", userID, reason)
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
