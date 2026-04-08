package authentication

import (
	"net/mail"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// NotificationPreferences contains user-specific notification settings
type NotificationPreferences struct {
	EmailEnabled              bool   `json:"emailEnabled" gorm:"default:true"`
	NtfyEnabled               bool   `json:"ntfyEnabled" gorm:"default:false"`
	NtfyURL                   string `json:"ntfyUrl,omitempty"`
	NtfyTopic                 string `json:"ntfyTopic,omitempty"`
	NtfyToken                 string `json:"ntfyToken,omitempty"`
	NotificationThresholdDays int    `json:"notificationThresholdDays" gorm:"default:0"`
}

// User is the struct for the database definition and the JWT claims
// A single user can own many products
type User struct {
	gorm.Model
	ID                      uint       `gorm:"primaryKey,unique"`
	Username                string     `json:"username"`
	MailAddress             string     `json:"mailAddress"`
	Password                string     `json:"-"`
	EmailVerifiedAt         *time.Time `json:"emailVerifiedAt,omitempty"`
	HouseholdID             uint       `gorm:"index"`
	Household               database.Household
	NotificationPreferences NotificationPreferences `gorm:"embedded"`
	FailedLoginAttempts     uint                    `gorm:"default:0" json:"-"`
	LockedUntil             gorm.DeletedAt          `json:"-"`
}

// IsValid is a simple validator function to check for valid properties
func (user *User) IsValid(skipPassword bool) error {
	if user.ID < 1 {
		return errors.ErrInvalidUserID
	}

	if user.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if !skipPassword {
		if len(user.Password) < 8 {
			return errors.ErrPasswordTooShort
		}
	}

	_, mailParseErr := mail.ParseAddress(user.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
