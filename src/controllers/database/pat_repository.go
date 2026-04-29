package database

import (
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"gorm.io/gorm"
)

type PATRepositoryInterface interface {
	CreatePAT(userID uint, name, tokenHash string, expiresAt *time.Time, scopes string) (*authentication.PersonalAccessToken, error)
	GetPATByTokenHash(tokenHash string) (*authentication.PersonalAccessToken, error)
	GetPATsByUserID(userID uint) ([]authentication.PersonalAccessToken, error)
	GetPATByID(patID uint) (*authentication.PersonalAccessToken, error)
	DeletePAT(patID uint, userID uint) error
	UpdateLastUsed(patID uint) error
}

var _ PATRepositoryInterface = (*PATRepository)(nil)

type PATRepository struct {
	DB *gorm.DB
}

func NewPATRepository(db *gorm.DB) *PATRepository {
	return &PATRepository{DB: db}
}

func (r *PATRepository) CreatePAT(userID uint, name, tokenHash string, expiresAt *time.Time, scopes string) (*authentication.PersonalAccessToken, error) {
	personalAccessToken := &authentication.PersonalAccessToken{
		UserID:    userID,
		Name:      name,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Scopes:    scopes,
	}
	if err := r.DB.Create(personalAccessToken).Error; err != nil {
		return nil, err
	}
	return personalAccessToken, nil
}

func (r *PATRepository) GetPATByTokenHash(tokenHash string) (*authentication.PersonalAccessToken, error) {
	var personalAccessToken authentication.PersonalAccessToken
	if err := r.DB.Where("token_hash = ?", tokenHash).First(&personalAccessToken).Error; err != nil {
		return nil, err
	}
	return &personalAccessToken, nil
}

func (r *PATRepository) GetPATsByUserID(userID uint) ([]authentication.PersonalAccessToken, error) {
	var pats []authentication.PersonalAccessToken
	if err := r.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&pats).Error; err != nil {
		return nil, err
	}
	return pats, nil
}

func (r *PATRepository) GetPATByID(patID uint) (*authentication.PersonalAccessToken, error) {
	var pat authentication.PersonalAccessToken
	if err := r.DB.First(&pat, patID).Error; err != nil {
		return nil, err
	}
	return &pat, nil
}

func (r *PATRepository) DeletePAT(patID uint, userID uint) error {
	result := r.DB.Where("id = ? AND user_id = ?", patID, userID).Delete(&authentication.PersonalAccessToken{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PATRepository) UpdateLastUsed(patID uint) error {
	now := time.Now()
	return r.DB.Model(&authentication.PersonalAccessToken{}).Where("id = ?", patID).Update("last_used_at", &now).Error
}
