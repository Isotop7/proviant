package authentication

import (
	"net/mail"

	"gitlab.com/Isotop7/expiro/errors"
)

// Signup is derived from User and Login and primarily used for registration
type Signup struct {
	Username    string `form:"username" json:"username" binding:"required"`
	Password    string `form:"password" json:"password" binding:"required"`
	MailAddress string `form:"mailAddress" json:"mailAddress" binding:"required"`
}

// IsValid checks if the given signup instance is valid
func (signup *Signup) IsValid() error {
	if len(signup.Username) == 0 {
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
