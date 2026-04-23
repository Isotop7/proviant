package database

import (
	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

type ExpiryScanRepository struct {
	DB *gorm.DB
}

func NewExpiryScanRepository(db *gorm.DB) *ExpiryScanRepository {
	return &ExpiryScanRepository{DB: db}
}

type ExpiryScanRepositoryInterface interface {
	Create(scan database.ExpiryScan) error
	GetByUser(userID uint, limit int) ([]database.ExpiryScan, error)
}

// Create inserts a new expiry scan record
func (r *ExpiryScanRepository) Create(scan database.ExpiryScan) error {
	return r.DB.Create(&scan).Error
}

// GetByUser returns recent scans for a user
func (r *ExpiryScanRepository) GetByUser(userID uint, limit int) ([]database.ExpiryScan, error) {
	var scans []database.ExpiryScan
	query := r.DB.Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&scans).Error
	return scans, err
}
