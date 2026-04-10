package authentication

import (
	"net/mail"

	"codeberg.org/isotop7/proviant/errors"
)

// Signup is derived from User and Login and primarily used for registration
type Signup struct {
	Username    string `form:"username" json:"username" binding:"required"`
	Password    string `form:"password" json:"password" binding:"required"`
	MailAddress string `form:"mailAddress" json:"mailAddress" binding:"required"`
	InviteToken string `form:"inviteToken" json:"inviteToken"`
}

// IsValid checks if the given signup instance is valid
func (signup *Signup) IsValid() error {
	return signup.IsValidWithValidator(defaultPasswordValidator)
}

// IsValidWithValidator checks if the given signup instance is valid using a custom validator
func (signup *Signup) IsValidWithValidator(validator *PasswordValidator) error {
	if signup.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if err := validator.Validate(signup.Password); err != nil {
		return err
	}

	_, mailParseErr := mail.ParseAddress(signup.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
