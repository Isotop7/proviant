package database

import (
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

type TOTPRepositoryInterface interface {
	CreateRecoveryCodes(userID uint, codeHashes []string) error
	// ReplaceRecoveryCodes swaps the recovery codes of a user in one
	// transaction, so a failure never leaves the account without codes.
	ReplaceRecoveryCodes(userID uint, codeHashes []string) error
	GetRecoveryCodesByUserID(userID uint) ([]database.TwoFactorRecoveryCode, error)
	CountUnusedRecoveryCodes(userID uint) (int64, error)
	// ConsumeRecoveryCode atomically marks a recovery code as used only if it
	// is still pending. It returns true only for the caller that actually
	// consumed it, so a code cannot be used twice even under concurrency.
	ConsumeRecoveryCode(userID uint, codeHash string, usedAt time.Time) (bool, error)
}

var _ TOTPRepositoryInterface = (*TOTPRepository)(nil)

type TOTPRepository struct {
	DB *gorm.DB
}

func NewTOTPRepository(db *gorm.DB) *TOTPRepository {
	return &TOTPRepository{DB: db}
}

func (r *TOTPRepository) CreateRecoveryCodes(userID uint, codeHashes []string) error {
	if len(codeHashes) == 0 {
		return nil
	}

	codes := make([]database.TwoFactorRecoveryCode, 0, len(codeHashes))
	for _, codeHash := range codeHashes {
		codes = append(codes, database.TwoFactorRecoveryCode{UserID: userID, CodeHash: codeHash})
	}

	return r.DB.Create(&codes).Error
}

func (r *TOTPRepository) GetRecoveryCodesByUserID(userID uint) ([]database.TwoFactorRecoveryCode, error) {
	var codes []database.TwoFactorRecoveryCode
	if err := r.DB.Where(util.QueryUserId, userID).Order("created_at ASC").Find(&codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// ReplaceRecoveryCodes swaps the recovery codes of a user in one transaction.
// The codes of the previous enrolment are gone when the new ones are written,
// never in between.
func (r *TOTPRepository) ReplaceRecoveryCodes(userID uint, codeHashes []string) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// A recovery code is a single-use credential, not a record: it is
		// removed for good, so no hash of an issued code outlives it.
		if err := tx.Unscoped().
			Where(util.QueryUserId, userID).
			Delete(&database.TwoFactorRecoveryCode{}).Error; err != nil {
			return err
		}
		if len(codeHashes) == 0 {
			return nil
		}

		codes := make([]database.TwoFactorRecoveryCode, 0, len(codeHashes))
		for _, codeHash := range codeHashes {
			codes = append(codes, database.TwoFactorRecoveryCode{UserID: userID, CodeHash: codeHash})
		}
		return tx.Create(&codes).Error
	})
}

func (r *TOTPRepository) CountUnusedRecoveryCodes(userID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&database.TwoFactorRecoveryCode{}).
		Where(util.QueryUserId, userID).
		Where("used_at IS NULL").
		Count(&count).Error
	return count, err
}

// ConsumeRecoveryCode marks a recovery code as used only if it is still
// pending. The conditional update makes this safe under concurrent use: a
// second concurrent caller sees zero rows affected.
func (r *TOTPRepository) ConsumeRecoveryCode(userID uint, codeHash string, usedAt time.Time) (bool, error) {
	result := r.DB.Model(&database.TwoFactorRecoveryCode{}).
		Where(util.QueryUserId, userID).
		Where("code_hash = ?", codeHash).
		Where("used_at IS NULL").
		Update("used_at", usedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
