package database

import (
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

type CalendarTokenRepositoryInterface interface {
	GetByToken(token string) (authentication.CalendarToken, error)
	DeleteByUserID(userID uint) error
	GetByUserID(userID uint) (authentication.CalendarToken, error)
	Create(ct *authentication.CalendarToken) error
	Update(ct *authentication.CalendarToken) error
}

var _ CalendarTokenRepositoryInterface = (*CalendarTokenRepository)(nil)

type CalendarTokenRepository struct {
	DB *gorm.DB
}

func NewCalendarTokenRepository(db *gorm.DB) *CalendarTokenRepository {
	return &CalendarTokenRepository{DB: db}
}

func (r *CalendarTokenRepository) GetByToken(token string) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where("token = ?", token).First(&ct).Error
	if err != nil {
		return ct, err
	}
	if ct.ExpiresAt.Before(time.Now()) {
		return ct, errors.ErrTokenExpired
	}
	return ct, nil
}

func (r *CalendarTokenRepository) DeleteByUserID(userID uint) error {
	return r.DB.Where(util.QueryUserId, userID).Delete(&authentication.CalendarToken{}).Error
}

func (r *CalendarTokenRepository) GetByUserID(userID uint) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where(util.QueryUserId, userID).First(&ct).Error
	return ct, err
}

func (r *CalendarTokenRepository) Create(ct *authentication.CalendarToken) error {
	return r.DB.Create(ct).Error
}

func (r *CalendarTokenRepository) Update(ct *authentication.CalendarToken) error {
	return r.DB.Save(ct).Error
}
