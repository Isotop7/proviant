package database

import (
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InvitationRepository struct {
	DB *gorm.DB
}

func NewInvitationRepository(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{DB: db}
}

func (r *InvitationRepository) CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error) {
	var user authentication.User
	if err := r.DB.Where("id = ? AND household_id = ?", inviterID, householdID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return database.HouseholdInvitation{}, errors.ErrInvitationNotAuthorized
		}
		return database.HouseholdInvitation{}, err
	}

	var existingInvitation database.HouseholdInvitation
	err := r.DB.Where("household_id = ? AND email = ? AND status = ?", householdID, email, database.InvitationStatusPending).First(&existingInvitation).Error
	if err == nil {
		return database.HouseholdInvitation{}, errors.ErrDuplicateInvitation
	}
	if err != gorm.ErrRecordNotFound {
		return database.HouseholdInvitation{}, err
	}

	token := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	invitation := database.HouseholdInvitation{
		HouseholdID: householdID,
		InviterID:   inviterID,
		Email:       email,
		Token:       token,
		Status:      database.InvitationStatusPending,
		ExpiresAt:   expiresAt,
	}

	if err := r.DB.Create(&invitation).Error; err != nil {
		return database.HouseholdInvitation{}, err
	}

	return invitation, nil
}

func (r *InvitationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error) {
	var invitations []database.HouseholdInvitation
	err := r.DB.Where("household_id = ? AND inviter_id = ?", householdID, inviterID).Order("created_at DESC").Find(&invitations).Error
	return invitations, err
}

func (r *InvitationRepository) GetPendingInvitationsForHousehold(householdID uint) ([]database.HouseholdInvitation, error) {
	var invitations []database.HouseholdInvitation
	err := r.DB.
		Where("household_id = ? AND status = ?", householdID, database.InvitationStatusPending).
		Order("created_at DESC").
		Find(&invitations).Error
	return invitations, err
}

func (r *InvitationRepository) GetInvitationByToken(token string) (database.HouseholdInvitation, error) {
	var invitation database.HouseholdInvitation
	if err := r.DB.Where("token = ?", token).First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return database.HouseholdInvitation{}, errors.ErrInvitationNotFound
		}
		return database.HouseholdInvitation{}, err
	}
	return invitation, nil
}

func (r *InvitationRepository) AcceptInvitation(token, email string, userID uint) error {
	var invitation database.HouseholdInvitation
	if err := r.DB.Where("token = ?", token).First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotFound
		}
		return err
	}

	switch invitation.Status {
	case database.InvitationStatusAccepted:
		return errors.ErrInvitationAlreadyUsed
	case database.InvitationStatusCancelled:
		return errors.ErrInvitationCancelled
	}

	if time.Now().After(invitation.ExpiresAt) {
		r.DB.Model(&invitation).Update("status", database.InvitationStatusExpired)
		return errors.ErrInvitationExpired
	}

	if invitation.Email != email {
		return errors.ErrInvitationEmailMismatch
	}

	tx := r.DB.Begin()
	if err := tx.Model(&authentication.User{}).Where("id = ?", userID).Update("household_id", invitation.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&invitation).Update("status", database.InvitationStatusAccepted).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (r *InvitationRepository) CancelInvitation(invitationID, userID uint) error {
	var invitation database.HouseholdInvitation
	if err := r.DB.First(&invitation, invitationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotFound
		}
		return err
	}

	var user authentication.User
	if err := r.DB.Where("id = ? AND household_id = ?", userID, invitation.HouseholdID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotAuthorized
		}
		return err
	}

	return r.DB.Model(&invitation).Update("status", database.InvitationStatusCancelled).Error
}

func (r *InvitationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error) {
	var invitations []database.HouseholdInvitation
	query := r.DB.Where(
		"status = ? AND expires_at > ? AND (sent_at IS NULL OR sent_at < ?)",
		database.InvitationStatusPending,
		time.Now(),
		time.Now().Add(-retryInterval),
	)
	err := query.Order("created_at ASC").Find(&invitations).Error
	return invitations, err
}

func (r *InvitationRepository) MarkInvitationSent(invitationID uint) error {
	now := time.Now()
	return r.DB.Model(&database.HouseholdInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]any{
			"sent_at":       now,
			"send_attempts": gorm.Expr("send_attempts + 1"),
		}).Error
}

func (r *InvitationRepository) MarkInvitationSendFailed(invitationID uint) error {
	return r.DB.Model(&database.HouseholdInvitation{}).
		Where("id = ?", invitationID).
		Update("send_attempts", gorm.Expr("send_attempts + 1")).Error
}

var _ = (*InvitationRepository)(nil)
