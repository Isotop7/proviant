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
	if signup.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if len(signup.Password) < 8 {
		return errors.ErrPasswordTooShort
	}

	_, mailParseErr := mail.ParseAddress(signup.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
