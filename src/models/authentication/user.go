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
	MonthlyWasteReportEnabled bool   `json:"monthlyWasteReportEnabled" gorm:"default:false"`
	TelegramEnabled           bool   `json:"telegramEnabled" gorm:"default:false"`
	TelegramChatID            string `json:"-"`
	TelegramLinkToken         string `json:"-"`
	TelegramBotToken          string `json:"telegramBotToken"`
	TelegramBotUsername       string `json:"-"`
	TelegramLinked            bool   `json:"telegramLinked" gorm:"-"`
	TelegramBotConfigured     bool   `json:"telegramBotConfigured" gorm:"-"`
}

// User is the struct for the database definition and the JWT claims
// A single user can own many products
type User struct {
	gorm.Model
	ID                      uint       `gorm:"primaryKey,unique"`
	Username                string     `json:"username"`
	DisplayName             string     `json:"displayName"`
	MailAddress             string     `json:"mailAddress"`
	Password                string     `json:"-"`
	EmailVerifiedAt         *time.Time `json:"emailVerifiedAt,omitempty"`
	HouseholdID             uint       `gorm:"index"`
	Household               database.Household
	NotificationPreferences NotificationPreferences `gorm:"embedded"`
	FailedLoginAttempts     uint                    `gorm:"default:0" json:"-"`
	LockedUntil             gorm.DeletedAt          `json:"-"`
}

// EffectiveName returns DisplayName if set, otherwise falls back to Username.
func (u *User) EffectiveName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// IsValid is a simple validator function to check for valid properties
func (user *User) IsValid(skipPassword bool) error {
	return user.IsValidWithValidator(skipPassword, defaultPasswordValidator)
}

// IsValidWithValidator checks if the given user instance is valid using a custom validator
func (user *User) IsValidWithValidator(skipPassword bool, validator *PasswordValidator) error {
	if user.ID < 1 {
		return errors.ErrInvalidUserID
	}

	if user.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if !skipPassword {
		if err := validator.Validate(user.Password); err != nil {
			return err
		}
	}

	_, mailParseErr := mail.ParseAddress(user.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
